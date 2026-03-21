package tui

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"bubblecopy/internal/engine"
	"bubblecopy/internal/model"
)

type phase int

const (
	phaseSelect phase = iota
	phaseDryRun
	phaseRunning
	phaseResult
)

type focusPane int

const (
	focusGroups focusPane = iota
	focusTasks
)

const animationInterval = 120 * time.Millisecond

var (
	panelBackground      = lipgloss.AdaptiveColor{Light: "#FFF8F0", Dark: "#241D18"}
	runningBackground    = lipgloss.AdaptiveColor{Light: "#FFF4E8", Dark: "#2B211B"}
	focusLineBackground  = lipgloss.AdaptiveColor{Light: "#F8EDE1", Dark: "#30251F"}
	titleStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C8874A"))
	logoFrameStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#A97A5A")).Background(runningBackground).Padding(0, 2)
	logoArtStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C98E5A"))
	logoTitleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B86F4D"))
	logoSubtitleStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9E7E63"))
	panelStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#8C6A53")).Background(panelBackground).Padding(0, 1)
	runningPanelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#A76F53")).Background(runningBackground).Padding(0, 1)
	successStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#B3984B"))
	failedStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#B85E52"))
	skippedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#A88663"))
	plannedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#B88757"))
	pendingStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B786A"))
	runningSpinnerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C58D52"))
	copyOpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#A77A53"))
	moveOpStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#B06D52"))
	groupSelectedStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C29858"))
	groupPartialStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#B98757"))
	groupUnselectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8D7764"))

	focusColors = []lipgloss.Color{
		lipgloss.Color("#C99A62"),
		lipgloss.Color("#BD8358"),
		lipgloss.Color("#B56C54"),
		lipgloss.Color("#B89261"),
	}
	logoArtLines          = []string{"      .-~~~~-.", "   .-(  o  o  )-.", "  (   .-.__.-.   )", "   '-(  ____  )-'", "      '-.__.-'"}
	focusIconFrames       = []string{"◐", "◓", "◑", "◒"}
	runningIconFrames     = []string{"▗▞▖", "▝▚▘", "▖▙▗", "▘▛▝", "▗▟▖", "▝▜▘"}
	groupSelectedFrames   = []string{"◉", "◎", "◉", "◍"}
	groupPartialFrames    = []string{"◐", "◓", "◑", "◒"}
	groupUnselectedFrames = []string{"○", "◌", "○", "◌"}
	taskSelectedFrames    = []string{"◆", "◇", "◆", "◈"}
	taskUnselectedFrames  = []string{"•", "◦", "•", "◦"}
)

type animationTickMsg struct{}

type executionResultMsg struct {
	result engine.Result
	ok     bool
}

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
	animFrame   int

	resultStream   <-chan engine.Result
	runnableByTask map[int]bool
	runTotal       int
	runDone        int
	successCount   int
	failedCount    int
	skippedCount   int
	lastUpdate     string

	spinner  spinner.Model
	progress progress.Model
}

func Run(tasks []model.Task, workers int) ([]model.Task, error) {
	ui := newModel(tasks, workers)
	var opts []tea.ProgramOption
	if runtime.GOOS == "windows" {
		// Use a fresh console input handle on Windows. Bubble Tea's default stdin
		// path enables native mouse input and disables Quick Edit, which prevents
		// terminal text selection with the mouse.
		opts = append(opts, tea.WithInputTTY())
	}
	program := tea.NewProgram(ui, opts...)
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

	spin := spinner.New()
	spin.Spinner = spinner.MiniDot
	spin.Style = runningSpinnerStyle

	bar := progress.New(progress.WithGradient("#BA7A46", "#D3A867"), progress.WithoutPercentage())

	m := &uiModel{
		tasks:          cloned,
		selected:       make(map[int]bool, len(cloned)),
		focus:          focusGroups,
		phase:          phaseSelect,
		workers:        workers,
		groupCursor:    0,
		taskCursor:     0,
		runnableByTask: make(map[int]bool),
		spinner:        spin,
		progress:       bar,
	}
	m.refreshGroups()
	m.updateProgressWidth()
	return m
}

func (m *uiModel) Init() tea.Cmd {
	return animationTickCmd()
}

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch incoming := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = incoming.Width
		m.height = incoming.Height
		m.updateProgressWidth()
		return m, nil
	case animationTickMsg:
		if m.phase != phaseSelect && m.phase != phaseRunning {
			return m, nil
		}
		m.animFrame = (m.animFrame + 1) % len(focusColors)
		return m, animationTickCmd()
	case spinner.TickMsg:
		if m.phase != phaseRunning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(incoming)
		return m, cmd
	case progress.FrameMsg:
		if m.phase != phaseRunning {
			return m, nil
		}
		updated, cmd := m.progress.Update(incoming)
		next, ok := updated.(progress.Model)
		if ok {
			m.progress = next
		}
		return m, cmd
	case executionResultMsg:
		if m.phase != phaseRunning {
			return m, nil
		}
		if !incoming.ok {
			m.finishExecution()
			return m, nil
		}
		m.applyExecutionResult(incoming.result)
		cmds := []tea.Cmd{waitForExecutionResult(m.resultStream)}
		if cmd := m.progress.SetPercent(m.executionPercent()); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		switch incoming.String() {
		case "q", "esc":
			return m, tea.Quit
		case "ctrl+c":
			// Keep Ctrl+C available for terminal copy when the user selects text.
			return m, nil
		case "left":
			m.focus = focusGroups
			return m, nil
		case "right":
			m.focus = focusTasks
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
				return m, nil
			case phaseDryRun:
				return m, m.startExecution()
			case phaseRunning:
				m.message = "execution is running..."
				return m, nil
			case phaseResult:
				m.message = "execution finished, press q to quit"
				return m, nil
			}
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

	parts := make([]string, 0, 4)
	parts = append(parts, m.renderLogo())
	if m.phase == phaseRunning {
		parts = append(parts, m.renderRunningHeader())
	}
	parts = append(parts, body, m.renderFooter())
	return strings.Join(parts, "\n")
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

func (m *uiModel) updateProgressWidth() {
	width := 40
	if m.width > 0 {
		width = m.width - 24
	}
	if width < 16 {
		width = 16
	}
	if width > 80 {
		width = 80
	}
	m.progress.Width = width
}

func (m *uiModel) moveCursor(delta int) {
	if m.focus == focusGroups {
		if len(m.groups) == 0 {
			return
		}
		m.groupCursor = wrapIndex(m.groupCursor, delta, len(m.groups))
		m.clampTaskCursor()
		return
	}

	current, ok := m.currentGroup()
	if !ok || len(current.TaskIndexes) == 0 {
		return
	}
	m.taskCursor = wrapIndex(m.taskCursor, delta, len(current.TaskIndexes))
}

func wrapIndex(current, delta, length int) int {
	if length <= 0 {
		return 0
	}
	next := (current + delta) % length
	if next < 0 {
		next += length
	}
	return next
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

func (m *uiModel) startExecution() tea.Cmd {
	if len(m.plan.Order) == 0 {
		m.message = "no dry-run plan available"
		return nil
	}
	return m.startExecutionWithStream(func() <-chan engine.Result {
		return engine.ExecuteStream(m.tasks, m.plan, m.workers)
	})
}

func (m *uiModel) startExecutionWithStream(streamFactory func() <-chan engine.Result) tea.Cmd {
	m.successCount = 0
	m.failedCount = 0
	m.skippedCount = 0
	m.runDone = 0
	m.runTotal = 0
	m.lastUpdate = ""
	clear(m.runnableByTask)

	for _, taskIndex := range m.plan.Order {
		item, ok := m.plan.ByTask[taskIndex]
		if ok && item.ShouldRun {
			m.runnableByTask[taskIndex] = true
			m.runTotal++
		}
	}

	if m.runTotal == 0 {
		m.phase = phaseResult
		m.resultStream = nil
		m.summarizePlanOnlyStatuses()
		m.message = fmt.Sprintf("no runnable tasks in plan: success=%d failed=%d skipped=%d", m.successCount, m.failedCount, m.skippedCount)
		return nil
	}

	m.phase = phaseRunning
	if streamFactory == nil {
		m.phase = phaseResult
		m.message = "execution stream unavailable"
		return nil
	}
	m.resultStream = streamFactory()
	m.lastUpdate = "waiting for results..."
	m.message = fmt.Sprintf("running %d task(s)...", m.runTotal)

	cmds := []tea.Cmd{
		waitForExecutionResult(m.resultStream),
		m.spinner.Tick,
		animationTickCmd(),
	}
	if cmd := m.progress.SetPercent(0); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
}

func (m *uiModel) summarizePlanOnlyStatuses() {
	for _, taskIndex := range m.plan.Order {
		item, ok := m.plan.ByTask[taskIndex]
		if !ok {
			m.failedCount++
			continue
		}
		switch item.Status {
		case model.StatusSuccess:
			m.successCount++
		case model.StatusFailed:
			m.failedCount++
		case model.StatusSkipped:
			m.skippedCount++
		}
	}
}

func (m *uiModel) finishExecution() {
	m.phase = phaseResult
	m.resultStream = nil
	m.message = fmt.Sprintf("execution complete: success=%d failed=%d skipped=%d", m.successCount, m.failedCount, m.skippedCount)
}

func (m *uiModel) applyExecutionResult(result engine.Result) {
	if result.TaskIndex >= 0 && result.TaskIndex < len(m.tasks) {
		m.tasks[result.TaskIndex].Status = result.Status
		m.tasks[result.TaskIndex].Message = result.Message
	}

	switch result.Status {
	case model.StatusSuccess:
		m.successCount++
	case model.StatusFailed:
		m.failedCount++
	case model.StatusSkipped:
		m.skippedCount++
	}

	if m.runnableByTask[result.TaskIndex] {
		delete(m.runnableByTask, result.TaskIndex)
		m.runDone++
	}

	m.lastUpdate = m.formatLatestTask(result)
}

func (m *uiModel) formatLatestTask(result engine.Result) string {
	if result.TaskIndex < 0 || result.TaskIndex >= len(m.tasks) {
		if result.Message == "" {
			return fmt.Sprintf("task %d", result.TaskIndex)
		}
		return result.Message
	}

	task := m.tasks[result.TaskIndex]
	return fmt.Sprintf(
		"%s %s %s -> %s",
		strings.ToUpper(string(result.Status)),
		task.Op,
		m.taskSourcePath(result.TaskIndex),
		m.taskTargetPath(result.TaskIndex),
	)
}

func (m *uiModel) executionPercent() float64 {
	if m.runTotal <= 0 {
		return 0
	}
	return float64(m.runDone) / float64(m.runTotal)
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
		focused := m.focus == focusGroups && groupIndex == m.groupCursor
		cursor := " "
		if focused {
			cursor = m.animatedFocusIcon()
		}
		selection := m.renderGroupSelectionMark(group, focused)
		line := fmt.Sprintf("%s %s %s (%d/%d)", cursor, selection, group.Name, group.SelectedCount, len(group.TaskIndexes))
		if focused {
			line = m.animatedLine(line)
		}
		lines = append(lines, line)
	}
	if len(m.groups) == 0 {
		lines = append(lines, "(no groups)")
	}
	return strings.Join(lines, "\n")
}

func (m *uiModel) renderGroupSelectionMark(group model.GroupView, focused bool) string {
	switch {
	case len(group.TaskIndexes) == 0:
		if focused {
			return groupUnselectedStyle.Render(m.frameValue(groupUnselectedFrames))
		}
		return groupUnselectedStyle.Render("○")
	case group.SelectedCount == 0:
		if focused {
			return groupUnselectedStyle.Render(m.frameValue(groupUnselectedFrames))
		}
		return groupUnselectedStyle.Render("○")
	case group.SelectedCount == len(group.TaskIndexes):
		if focused {
			return groupSelectedStyle.Render(m.frameValue(groupSelectedFrames))
		}
		return groupSelectedStyle.Render("◉")
	default:
		if focused {
			return groupPartialStyle.Render(m.frameValue(groupPartialFrames))
		}
		return groupPartialStyle.Render("◐")
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

	contentWidth := m.taskPanelContentWidth()
	for row, taskIndex := range group.TaskIndexes {
		task := m.tasks[taskIndex]
		focused := m.focus == focusTasks && row == m.taskCursor
		cursor := " "
		if focused {
			cursor = m.animatedFocusIcon()
		}

		mark := m.renderTaskSelectionMark(m.selected[taskIndex], focused)

		op := m.renderOperation(task.Op)
		status := renderStatus(task.Status)

		line := fmt.Sprintf("%s %s %s %s", cursor, mark, op, status)
		if focused {
			line = m.animatedLine(line)
		}
		lines = append(lines, line)
		lines = append(lines, wrapLabeledText("    from: ", m.taskSourcePath(taskIndex), contentWidth)...)
		lines = append(lines, wrapLabeledText("    to:   ", m.taskTargetPath(taskIndex), contentWidth)...)

		if task.Message != "" && m.phase != phaseSelect {
			lines = append(lines, wrapLabeledText("    ", task.Message, contentWidth)...)
		}
	}

	return strings.Join(lines, "\n")
}

func renderStatus(status model.Status) string {
	if status == "" {
		status = model.StatusPending
	}
	icon := renderStatusIcon(status)
	value := fmt.Sprintf("%s %s", icon, status)

	switch status {
	case model.StatusPending:
		return pendingStyle.Render(value)
	case model.StatusSuccess:
		return successStyle.Render(value)
	case model.StatusFailed:
		return failedStyle.Render(value)
	case model.StatusSkipped:
		return skippedStyle.Render(value)
	case model.StatusPlanned:
		return plannedStyle.Render(value)
	default:
		return pendingStyle.Render(value)
	}
}

func renderStatusIcon(status model.Status) string {
	switch status {
	case model.StatusPending:
		return "◌"
	case model.StatusPlanned:
		return "◆"
	case model.StatusSuccess:
		return "✔"
	case model.StatusFailed:
		return "✖"
	case model.StatusSkipped:
		return "➟"
	default:
		return "•"
	}
}

func renderOpIcon(op model.Operation) string {
	switch op {
	case model.OpCopy:
		return "⧉"
	case model.OpMove:
		return "➜"
	default:
		return "•"
	}
}

func (m *uiModel) renderOperation(op model.Operation) string {
	value := fmt.Sprintf("%s %s", renderOpIcon(op), op)
	switch op {
	case model.OpCopy:
		return copyOpStyle.Render(value)
	case model.OpMove:
		return moveOpStyle.Render(value)
	default:
		return value
	}
}

func (m *uiModel) renderTaskSelectionMark(selected bool, focused bool) string {
	if selected {
		if focused {
			return groupSelectedStyle.Render(m.frameValue(taskSelectedFrames))
		}
		return groupSelectedStyle.Render("◆")
	}
	if focused {
		return groupUnselectedStyle.Render(m.frameValue(taskUnselectedFrames))
	}
	return groupUnselectedStyle.Render("•")
}

func (m *uiModel) frameValue(frames []string) string {
	if len(frames) == 0 {
		return ""
	}
	return frames[m.animFrame%len(frames)]
}

func (m *uiModel) focusColor() lipgloss.Color {
	if len(focusColors) == 0 {
		return lipgloss.Color("220")
	}
	return focusColors[m.animFrame%len(focusColors)]
}

func (m *uiModel) focusIconFrame() string {
	return m.frameValue(focusIconFrames)
}

func (m *uiModel) animatedFocusIcon() string {
	return lipgloss.NewStyle().Bold(true).Foreground(m.focusColor()).Render(m.focusIconFrame())
}

func (m *uiModel) animatedRunIcon() string {
	return lipgloss.NewStyle().Bold(true).Foreground(m.focusColor()).Render(m.frameValue(runningIconFrames))
}

func (m *uiModel) renderLogo() string {
	art := logoArtStyle.Render(strings.Join(logoArtLines, "\n"))
	title := logoTitleStyle.Render("BUBBLECOPY")
	subtitle := logoSubtitleStyle.Render("batch copy / move")
	content := lipgloss.JoinVertical(lipgloss.Center, art, title, subtitle)

	frameWidth := m.runningPanelWidth()
	if frameWidth > 72 {
		frameWidth = 72
	}
	if frameWidth < 34 {
		frameWidth = 34
	}

	return lipgloss.NewStyle().
		Width(m.runningPanelWidth()).
		Align(lipgloss.Center).
		Render(logoFrameStyle.Width(frameWidth).Align(lipgloss.Center).Render(content))
}

func (m *uiModel) animatedLine(line string) string {
	return lipgloss.NewStyle().
		Foreground(m.focusColor()).
		Background(focusLineBackground).
		Render(line)
}

func (m *uiModel) renderRunningHeader() string {
	percent := m.executionPercent()
	head := fmt.Sprintf("%s %s %d/%d  success=%d failed=%d skipped=%d",
		m.spinner.View(),
		m.animatedRunIcon(),
		m.runDone,
		m.runTotal,
		m.successCount,
		m.failedCount,
		m.skippedCount,
	)
	progressLine := fmt.Sprintf("%s %3.0f%%", m.progress.View(), percent*100)

	lines := []string{head, progressLine}
	if m.lastUpdate != "" {
		lines = append(lines, wrapLabeledText("Current: ", m.lastUpdate, m.runningPanelContentWidth())...)
	}
	return runningPanelStyle.Width(m.runningPanelWidth()).Render(strings.Join(lines, "\n"))
}

func (m *uiModel) runningPanelWidth() int {
	if m.width <= 0 {
		return 122
	}
	width := m.width - 2
	if width < 40 {
		width = 40
	}
	return width
}

func (m *uiModel) runningPanelContentWidth() int {
	return panelContentWidth(m.runningPanelWidth())
}

func (m *uiModel) taskPanelContentWidth() int {
	_, rightWidth := m.columnWidths()
	return panelContentWidth(rightWidth)
}

func (m *uiModel) renderFooter() string {
	var help string
	switch m.phase {
	case phaseSelect:
		help = "left/right:focus  up/down:move  space:select  enter:dry-run  q/esc:quit  mouse-select+ctrl+c:copy"
	case phaseDryRun:
		help = "dry-run ready. enter:execute  q/esc:quit  mouse-select+ctrl+c:copy"
	case phaseRunning:
		help = "execution running. q/esc:quit  mouse-select+ctrl+c:copy"
	case phaseResult:
		help = "execution complete. q/esc:quit  mouse-select+ctrl+c:copy"
	}
	if m.message == "" {
		return help
	}
	return help + "\n" + m.message
}

func panelContentWidth(panelWidth int) int {
	width := panelWidth - 6
	if width < 20 {
		return 20
	}
	return width
}

func (m *uiModel) taskSourcePath(taskIndex int) string {
	if item, ok := m.plan.ByTask[taskIndex]; ok && item.SourceAbs != "" {
		return item.SourceAbs
	}
	if taskIndex < 0 || taskIndex >= len(m.tasks) {
		return ""
	}
	return absoluteDisplayPath(m.tasks[taskIndex].Source)
}

func (m *uiModel) taskTargetPath(taskIndex int) string {
	if item, ok := m.plan.ByTask[taskIndex]; ok {
		switch {
		case item.FinalPath != "":
			return item.FinalPath
		case item.TargetAbs != "":
			return item.TargetAbs
		}
	}
	if taskIndex < 0 || taskIndex >= len(m.tasks) {
		return ""
	}
	return absoluteDisplayPath(m.tasks[taskIndex].Target)
}

func absoluteDisplayPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(abs)
}

func wrapLabeledText(prefix, value string, max int) []string {
	if value == "" {
		return []string{prefix}
	}

	prefixWidth := utf8.RuneCountInString(prefix)
	if max <= prefixWidth+1 {
		return []string{prefix + value}
	}

	continuation := strings.Repeat(" ", prefixWidth)
	rawLines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))

	for rawLineIndex, rawLine := range rawLines {
		currentPrefix := prefix
		if rawLineIndex > 0 {
			currentPrefix = continuation
		}

		runes := []rune(rawLine)
		if len(runes) == 0 {
			lines = append(lines, currentPrefix)
			continue
		}

		for len(runes) > 0 {
			room := max - utf8.RuneCountInString(currentPrefix)
			if room <= 0 || len(runes) <= room {
				lines = append(lines, currentPrefix+string(runes))
				break
			}

			lines = append(lines, currentPrefix+string(runes[:room]))
			runes = runes[room:]
			currentPrefix = continuation
		}
	}

	return lines
}

func animationTickCmd() tea.Cmd {
	return tea.Tick(animationInterval, func(time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

func waitForExecutionResult(stream <-chan engine.Result) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-stream
		return executionResultMsg{
			result: result,
			ok:     ok,
		}
	}
}
