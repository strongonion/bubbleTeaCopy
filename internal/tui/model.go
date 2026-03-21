package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"bubblecopy/internal/engine"
	"bubblecopy/internal/model"
)

type phase int

const (
	phaseSelect phase = iota
	phaseDryRun
	phaseResult
)

type focusPane int

const (
	focusGroups focusPane = iota
	focusTasks
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	panelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	cursorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	failedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	skippedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

type uiModel struct {
	tasks       []model.Task
	groups      []model.GroupView
	selected    map[int]bool
	groupCursor int
	taskCursor  int
	focus       focusPane
	phase       phase
	workers     int
	plan        engine.Plan
	message     string
	width       int
	height      int
}

func Run(tasks []model.Task, workers int) ([]model.Task, error) {
	ui := newModel(tasks, workers)
	program := tea.NewProgram(ui)
	finalModel, err := program.Run()
	if err != nil {
		return nil, err
	}

	finalUI, ok := finalModel.(*uiModel)
	if !ok {
		return nil, fmt.Errorf("unexpected model type")
	}
	return finalUI.tasks, nil
}

func newModel(tasks []model.Task, workers int) *uiModel {
	cloned := make([]model.Task, len(tasks))
	copy(cloned, tasks)
	for i := range cloned {
		cloned[i].Index = i
	}
	if workers < 1 {
		workers = 1
	}

	m := &uiModel{
		tasks:       cloned,
		selected:    make(map[int]bool, len(cloned)),
		focus:       focusGroups,
		phase:       phaseSelect,
		workers:     workers,
		groupCursor: 0,
		taskCursor:  0,
	}
	m.refreshGroups()
	return m
}

func (m *uiModel) Init() tea.Cmd {
	return nil
}

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch incoming := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = incoming.Width
		m.height = incoming.Height
		return m, nil
	case tea.KeyMsg:
		switch incoming.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.focus == focusGroups {
				m.focus = focusTasks
			} else {
				m.focus = focusGroups
			}
			return m, nil
		case "up", "k":
			m.moveCursor(-1)
			return m, nil
		case "down", "j":
			m.moveCursor(1)
			return m, nil
		case " ":
			if m.phase == phaseSelect {
				m.toggleSelection()
			}
			return m, nil
		case "enter":
			switch m.phase {
			case phaseSelect:
				m.runDryRun()
			case phaseDryRun:
				m.runExecution()
			case phaseResult:
				m.message = "execution finished, press q to quit"
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *uiModel) View() string {
	if len(m.tasks) == 0 {
		return "No tasks loaded. Press q to quit."
	}

	leftWidth, rightWidth := m.columnWidths()
	groupsPanel := panelStyle.Width(leftWidth).Render(m.renderGroups())
	tasksPanel := panelStyle.Width(rightWidth).Render(m.renderTasks())

	body := lipgloss.JoinHorizontal(lipgloss.Top, groupsPanel, tasksPanel)
	return body + "\n" + m.renderFooter()
}

func (m *uiModel) columnWidths() (int, int) {
	if m.width <= 0 {
		return 34, 88
	}
	left := m.width / 3
	if left < 24 {
		left = 24
	}
	right := m.width - left - 4
	if right < 40 {
		right = 40
	}
	return left, right
}

func (m *uiModel) moveCursor(delta int) {
	if m.focus == focusGroups {
		if len(m.groups) == 0 {
			return
		}
		m.groupCursor += delta
		if m.groupCursor < 0 {
			m.groupCursor = 0
		}
		if m.groupCursor >= len(m.groups) {
			m.groupCursor = len(m.groups) - 1
		}
		m.clampTaskCursor()
		return
	}

	current, ok := m.currentGroup()
	if !ok {
		return
	}
	m.taskCursor += delta
	if m.taskCursor < 0 {
		m.taskCursor = 0
	}
	if m.taskCursor >= len(current.TaskIndexes) {
		m.taskCursor = len(current.TaskIndexes) - 1
	}
}

func (m *uiModel) toggleSelection() {
	if m.focus == focusGroups {
		m.toggleCurrentGroupSelection()
		return
	}
	taskIndex, ok := m.currentTaskIndex()
	if !ok {
		return
	}
	m.selected[taskIndex] = !m.selected[taskIndex]
	m.refreshGroups()
}

func (m *uiModel) toggleCurrentGroupSelection() {
	group, ok := m.currentGroup()
	if !ok || len(group.TaskIndexes) == 0 {
		return
	}

	allSelected := true
	for _, taskIndex := range group.TaskIndexes {
		if !m.selected[taskIndex] {
			allSelected = false
			break
		}
	}

	nextState := !allSelected
	for _, taskIndex := range group.TaskIndexes {
		m.selected[taskIndex] = nextState
	}
	m.refreshGroups()
}

func (m *uiModel) runDryRun() {
	selected := m.selectedIndexes()
	if len(selected) == 0 {
		m.message = "no tasks selected"
		return
	}

	plan := engine.DryRun(m.tasks, selected)
	m.plan = plan
	for i := range m.tasks {
		m.tasks[i].Status = model.StatusPending
		m.tasks[i].Message = ""
	}
	planned := 0
	skipped := 0
	failed := 0

	for _, taskIndex := range plan.Order {
		item := plan.ByTask[taskIndex]
		m.tasks[taskIndex].Status = item.Status
		m.tasks[taskIndex].Message = item.Message
		switch item.Status {
		case model.StatusPlanned:
			planned++
		case model.StatusSkipped:
			skipped++
		case model.StatusFailed:
			failed++
		}
	}

	m.phase = phaseDryRun
	m.message = fmt.Sprintf("dry-run complete: planned=%d skipped=%d failed=%d. press enter to execute", planned, skipped, failed)
}

func (m *uiModel) runExecution() {
	if len(m.plan.Order) == 0 {
		m.message = "no dry-run plan available"
		return
	}

	results := engine.Execute(m.tasks, m.plan, m.workers)
	success := 0
	failed := 0
	skipped := 0

	for _, result := range results {
		if result.TaskIndex < 0 || result.TaskIndex >= len(m.tasks) {
			continue
		}
		m.tasks[result.TaskIndex].Status = result.Status
		m.tasks[result.TaskIndex].Message = result.Message
		switch result.Status {
		case model.StatusSuccess:
			success++
		case model.StatusFailed:
			failed++
		case model.StatusSkipped:
			skipped++
		}
	}

	m.phase = phaseResult
	m.message = fmt.Sprintf("execution complete: success=%d failed=%d skipped=%d", success, failed, skipped)
}

func (m *uiModel) selectedIndexes() []int {
	var indexes []int
	for taskIndex := range m.selected {
		if m.selected[taskIndex] {
			indexes = append(indexes, taskIndex)
		}
	}
	sort.Ints(indexes)
	return indexes
}

func (m *uiModel) refreshGroups() {
	m.groups = buildGroups(m.tasks, m.selected)
	if len(m.groups) == 0 {
		m.groupCursor = 0
		m.taskCursor = 0
		return
	}
	if m.groupCursor < 0 {
		m.groupCursor = 0
	}
	if m.groupCursor >= len(m.groups) {
		m.groupCursor = len(m.groups) - 1
	}
	m.clampTaskCursor()
}

func buildGroups(tasks []model.Task, selected map[int]bool) []model.GroupView {
	type groupRef struct {
		index int
	}
	seen := make(map[string]groupRef)
	var groups []model.GroupView

	for taskIndex, task := range tasks {
		name := strings.TrimSpace(task.Group)
		if name == "" {
			name = model.DefaultGroup
		}

		ref, ok := seen[name]
		if !ok {
			ref = groupRef{index: len(groups)}
			seen[name] = ref
			groups = append(groups, model.GroupView{Name: name})
		}

		groups[ref.index].TaskIndexes = append(groups[ref.index].TaskIndexes, taskIndex)
		if selected[taskIndex] {
			groups[ref.index].SelectedCount++
		}
	}

	return groups
}

func (m *uiModel) clampTaskCursor() {
	group, ok := m.currentGroup()
	if !ok || len(group.TaskIndexes) == 0 {
		m.taskCursor = 0
		return
	}
	if m.taskCursor < 0 {
		m.taskCursor = 0
	}
	if m.taskCursor >= len(group.TaskIndexes) {
		m.taskCursor = len(group.TaskIndexes) - 1
	}
}

func (m *uiModel) currentGroup() (model.GroupView, bool) {
	if len(m.groups) == 0 || m.groupCursor < 0 || m.groupCursor >= len(m.groups) {
		return model.GroupView{}, false
	}
	return m.groups[m.groupCursor], true
}

func (m *uiModel) currentTaskIndex() (int, bool) {
	group, ok := m.currentGroup()
	if !ok || len(group.TaskIndexes) == 0 {
		return 0, false
	}
	if m.taskCursor < 0 || m.taskCursor >= len(group.TaskIndexes) {
		return 0, false
	}
	return group.TaskIndexes[m.taskCursor], true
}

func (m *uiModel) renderGroups() string {
	var lines []string
	lines = append(lines, titleStyle.Render("Groups"))
	lines = append(lines, "")

	for groupIndex, group := range m.groups {
		cursor := " "
		if m.focus == focusGroups && groupIndex == m.groupCursor {
			cursor = cursorStyle.Render(">")
		}
		selection := groupSelectionMark(group)
		line := fmt.Sprintf("%s %s %s (%d/%d)", cursor, selection, group.Name, group.SelectedCount, len(group.TaskIndexes))
		lines = append(lines, line)
	}
	if len(m.groups) == 0 {
		lines = append(lines, "(no groups)")
	}
	return strings.Join(lines, "\n")
}

func groupSelectionMark(group model.GroupView) string {
	switch {
	case len(group.TaskIndexes) == 0:
		return "[ ]"
	case group.SelectedCount == 0:
		return "[ ]"
	case group.SelectedCount == len(group.TaskIndexes):
		return "[x]"
	default:
		return "[~]"
	}
}

func (m *uiModel) renderTasks() string {
	group, ok := m.currentGroup()
	title := "Tasks"
	if ok {
		title = fmt.Sprintf("Tasks - %s", group.Name)
	}

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	if !ok || len(group.TaskIndexes) == 0 {
		lines = append(lines, "(no tasks)")
		return strings.Join(lines, "\n")
	}

	for row, taskIndex := range group.TaskIndexes {
		task := m.tasks[taskIndex]
		cursor := " "
		if m.focus == focusTasks && row == m.taskCursor {
			cursor = cursorStyle.Render(">")
		}

		mark := "[ ]"
		if m.selected[taskIndex] {
			mark = "[x]"
		}

		status := renderStatus(task.Status)
		source := filepath.Base(task.Source)
		if source == "." || source == string(filepath.Separator) || source == "" {
			source = task.Source
		}

		line := fmt.Sprintf("%s %s %-4s %-8s %s -> %s", cursor, mark, task.Op, status, source, task.Target)
		lines = append(lines, truncate(line, 120))

		if task.Message != "" && m.phase != phaseSelect {
			lines = append(lines, "    "+truncate(task.Message, 110))
		}
	}

	return strings.Join(lines, "\n")
}

func renderStatus(status model.Status) string {
	switch status {
	case model.StatusSuccess:
		return successStyle.Render(string(status))
	case model.StatusFailed:
		return failedStyle.Render(string(status))
	case model.StatusSkipped:
		return skippedStyle.Render(string(status))
	default:
		return string(status)
	}
}

func (m *uiModel) renderFooter() string {
	var help string
	switch m.phase {
	case phaseSelect:
		help = "tab:focus  up/down:move  space:select  enter:dry-run  q:quit"
	case phaseDryRun:
		help = "dry-run ready. enter:execute  q:quit"
	case phaseResult:
		help = "execution complete. q:quit"
	}
	if m.message == "" {
		return help
	}
	return help + "\n" + m.message
}

func truncate(value string, max int) string {
	if max <= 0 {
		return value
	}
	if utf8.RuneCountInString(value) <= max {
		return value
	}

	runes := []rune(value)
	if max == 1 {
		return string(runes[:1])
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
