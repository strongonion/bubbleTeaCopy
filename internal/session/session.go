package session

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"bubblecopy/internal/engine"
	"bubblecopy/internal/model"
)

type Phase string

const (
	PhaseSelect  Phase = "select"
	PhaseDryRun  Phase = "dry-run"
	PhaseRunning Phase = "running"
	PhaseResult  Phase = "result"
)

var (
	ErrBusy         = errors.New("execution is running")
	ErrInvalidPhase = errors.New("action is not allowed in the current phase")
	ErrNoSelection  = errors.New("no tasks selected")
)

type Executor func(tasks []model.Task, plan engine.Plan, workers int) <-chan engine.Result

type TaskView struct {
	Index       int             `json:"index"`
	Group       string          `json:"group"`
	Source      string          `json:"source"`
	Target      string          `json:"target"`
	SourcePath  string          `json:"sourcePath"`
	TargetPath  string          `json:"targetPath"`
	Op          model.Operation `json:"op"`
	ClearTarget bool            `json:"clearTarget"`
	Status      model.Status    `json:"status"`
	Message     string          `json:"message"`
	Selected    bool            `json:"selected"`
}

type Snapshot struct {
	Phase            Phase             `json:"phase"`
	Message          string            `json:"message"`
	Workers          int               `json:"workers"`
	SelectedCount    int               `json:"selectedCount"`
	RunTotal         int               `json:"runTotal"`
	RunDone          int               `json:"runDone"`
	SuccessCount     int               `json:"successCount"`
	FailedCount      int               `json:"failedCount"`
	SkippedCount     int               `json:"skippedCount"`
	LastUpdate       string            `json:"lastUpdate"`
	ExecutionPercent float64           `json:"executionPercent"`
	Groups           []model.GroupView `json:"groups"`
	Tasks            []TaskView        `json:"tasks"`
}

type Session struct {
	mu sync.RWMutex

	tasks          []model.Task
	selected       map[int]bool
	workers        int
	phase          Phase
	plan           engine.Plan
	message        string
	runnableByTask map[int]bool
	runTotal       int
	runDone        int
	successCount   int
	failedCount    int
	skippedCount   int
	lastUpdate     string
	executor       Executor
}

func New(tasks []model.Task, workers int) *Session {
	return NewWithExecutor(tasks, workers, engine.ExecuteStream)
}

func NewWithExecutor(tasks []model.Task, workers int, executor Executor) *Session {
	cloned := cloneTasks(tasks)
	for i := range cloned {
		cloned[i].Index = i
		if cloned[i].Status == "" {
			cloned[i].Status = model.StatusPending
		}
	}
	if workers < 1 {
		workers = 1
	}
	if executor == nil {
		executor = engine.ExecuteStream
	}

	return &Session{
		tasks:          cloned,
		selected:       make(map[int]bool, len(cloned)),
		workers:        workers,
		phase:          PhaseSelect,
		runnableByTask: make(map[int]bool),
		executor:       executor,
	}
}

func (s *Session) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.snapshotLocked()
}

func (s *Session) Tasks() []model.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneTasks(s.tasks)
}

func (s *Session) ReplaceSelection(indexes []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.phase != PhaseSelect {
		s.message = "selection is locked outside the choose phase"
		return ErrInvalidPhase
	}

	selected := make(map[int]bool, len(indexes))
	for _, idx := range indexes {
		if idx < 0 || idx >= len(s.tasks) {
			return fmt.Errorf("invalid task index: %d", idx)
		}
		selected[idx] = true
	}

	s.selected = selected
	return nil
}

func (s *Session) DryRun() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.phase == PhaseRunning {
		s.message = "execution is running..."
		return ErrBusy
	}

	selected := sortedSelectedIndexes(s.selected)
	if len(selected) == 0 {
		s.message = "no tasks selected"
		return ErrNoSelection
	}

	s.plan = engine.DryRun(s.tasks, selected)
	s.resetExecutionLocked()

	for i := range s.tasks {
		s.tasks[i].Status = model.StatusPending
		s.tasks[i].Message = ""
	}

	planned := 0
	skipped := 0
	failed := 0
	for _, taskIndex := range s.plan.Order {
		item := s.plan.ByTask[taskIndex]
		s.tasks[taskIndex].Status = item.Status
		s.tasks[taskIndex].Message = item.Message
		switch item.Status {
		case model.StatusPlanned:
			planned++
		case model.StatusSkipped:
			skipped++
		case model.StatusFailed:
			failed++
		}
	}

	s.phase = PhaseDryRun
	s.message = fmt.Sprintf("dry-run complete: planned=%d skipped=%d failed=%d. press execute to continue", planned, skipped, failed)
	return nil
}

func (s *Session) StartExecution() error {
	return s.StartExecutionWithExecutor(nil)
}

func (s *Session) StartExecutionWithExecutor(executor Executor) error {
	s.mu.Lock()
	if s.phase == PhaseRunning {
		s.message = "execution is running..."
		s.mu.Unlock()
		return ErrBusy
	}
	if s.phase != PhaseDryRun || len(s.plan.Order) == 0 {
		s.message = "no dry-run plan available"
		s.mu.Unlock()
		return ErrInvalidPhase
	}

	if executor == nil {
		executor = s.executor
	}
	if executor == nil {
		executor = engine.ExecuteStream
	}

	s.resetExecutionLocked()
	for _, taskIndex := range s.plan.Order {
		item, ok := s.plan.ByTask[taskIndex]
		if ok && item.ShouldRun {
			s.runnableByTask[taskIndex] = true
			s.runTotal++
		}
	}

	if s.runTotal == 0 {
		s.phase = PhaseResult
		s.summarizePlanOnlyStatusesLocked()
		s.message = fmt.Sprintf("no runnable tasks in plan: success=%d failed=%d skipped=%d", s.successCount, s.failedCount, s.skippedCount)
		s.mu.Unlock()
		return nil
	}

	tasks := cloneTasks(s.tasks)
	plan := s.plan
	workers := s.workers

	s.phase = PhaseRunning
	s.lastUpdate = "waiting for results..."
	s.message = fmt.Sprintf("running %d task(s)...", s.runTotal)
	s.mu.Unlock()

	go s.execute(tasks, plan, workers, executor)
	return nil
}

func (s *Session) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.phase == PhaseRunning {
		s.message = "execution is running..."
		return ErrBusy
	}

	s.plan = engine.Plan{}
	s.resetExecutionLocked()
	for i := range s.tasks {
		s.tasks[i].Status = model.StatusPending
		s.tasks[i].Message = ""
	}
	s.phase = PhaseSelect
	s.message = ""
	return nil
}

func (s *Session) execute(tasks []model.Task, plan engine.Plan, workers int, executor Executor) {
	for result := range executor(tasks, plan, workers) {
		s.mu.Lock()
		s.applyExecutionResultLocked(result)
		s.mu.Unlock()
	}

	s.mu.Lock()
	if s.phase == PhaseRunning {
		s.phase = PhaseResult
		s.message = fmt.Sprintf("execution complete: success=%d failed=%d skipped=%d", s.successCount, s.failedCount, s.skippedCount)
	}
	s.mu.Unlock()
}

func (s *Session) snapshotLocked() Snapshot {
	tasks := make([]TaskView, len(s.tasks))
	selectedCount := 0
	for i, task := range s.tasks {
		selected := s.selected[i]
		if selected {
			selectedCount++
		}
		tasks[i] = TaskView{
			Index:       task.Index,
			Group:       normalizedGroupName(task.Group),
			Source:      task.Source,
			Target:      task.Target,
			SourcePath:  s.taskSourcePathLocked(i),
			TargetPath:  s.taskTargetPathLocked(i),
			Op:          task.Op,
			ClearTarget: task.ClearTarget,
			Status:      task.Status,
			Message:     task.Message,
			Selected:    selected,
		}
	}

	return Snapshot{
		Phase:            s.phase,
		Message:          s.message,
		Workers:          s.workers,
		SelectedCount:    selectedCount,
		RunTotal:         s.runTotal,
		RunDone:          s.runDone,
		SuccessCount:     s.successCount,
		FailedCount:      s.failedCount,
		SkippedCount:     s.skippedCount,
		LastUpdate:       s.lastUpdate,
		ExecutionPercent: executionPercent(s.runDone, s.runTotal),
		Groups:           cloneGroups(BuildGroups(s.tasks, s.selected)),
		Tasks:            tasks,
	}
}

func (s *Session) resetExecutionLocked() {
	s.runnableByTask = make(map[int]bool)
	s.runTotal = 0
	s.runDone = 0
	s.successCount = 0
	s.failedCount = 0
	s.skippedCount = 0
	s.lastUpdate = ""
}

func (s *Session) summarizePlanOnlyStatusesLocked() {
	for _, taskIndex := range s.plan.Order {
		item, ok := s.plan.ByTask[taskIndex]
		if !ok {
			s.failedCount++
			continue
		}
		switch item.Status {
		case model.StatusSuccess:
			s.successCount++
		case model.StatusFailed:
			s.failedCount++
		case model.StatusSkipped:
			s.skippedCount++
		}
	}
}

func (s *Session) applyExecutionResultLocked(result engine.Result) {
	if result.TaskIndex >= 0 && result.TaskIndex < len(s.tasks) {
		s.tasks[result.TaskIndex].Status = result.Status
		s.tasks[result.TaskIndex].Message = result.Message
	}

	switch result.Status {
	case model.StatusSuccess:
		s.successCount++
	case model.StatusFailed:
		s.failedCount++
	case model.StatusSkipped:
		s.skippedCount++
	}

	if s.runnableByTask[result.TaskIndex] {
		delete(s.runnableByTask, result.TaskIndex)
		s.runDone++
	}

	s.lastUpdate = s.formatLatestTaskLocked(result)
}

func (s *Session) formatLatestTaskLocked(result engine.Result) string {
	if result.TaskIndex < 0 || result.TaskIndex >= len(s.tasks) {
		if result.Message == "" {
			return fmt.Sprintf("task %d", result.TaskIndex)
		}
		return result.Message
	}

	task := s.tasks[result.TaskIndex]
	return fmt.Sprintf(
		"%s %s %s -> %s",
		strings.ToUpper(string(result.Status)),
		task.Op,
		s.taskSourcePathLocked(result.TaskIndex),
		s.taskTargetPathLocked(result.TaskIndex),
	)
}

func (s *Session) taskSourcePathLocked(taskIndex int) string {
	if item, ok := s.plan.ByTask[taskIndex]; ok && item.SourceAbs != "" {
		return item.SourceAbs
	}
	if taskIndex < 0 || taskIndex >= len(s.tasks) {
		return ""
	}
	return absoluteDisplayPath(s.tasks[taskIndex].Source)
}

func (s *Session) taskTargetPathLocked(taskIndex int) string {
	if item, ok := s.plan.ByTask[taskIndex]; ok {
		switch {
		case item.FinalPath != "":
			return item.FinalPath
		case item.TargetAbs != "":
			return item.TargetAbs
		}
	}
	if taskIndex < 0 || taskIndex >= len(s.tasks) {
		return ""
	}
	return absoluteDisplayPath(s.tasks[taskIndex].Target)
}

func BuildGroups(tasks []model.Task, selected map[int]bool) []model.GroupView {
	type groupRef struct {
		index int
	}

	seen := make(map[string]groupRef)
	var groups []model.GroupView

	for taskIndex, task := range tasks {
		name := normalizedGroupName(task.Group)

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

func normalizedGroupName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.DefaultGroup
	}
	return name
}

func cloneTasks(tasks []model.Task) []model.Task {
	cloned := make([]model.Task, len(tasks))
	copy(cloned, tasks)
	return cloned
}

func cloneGroups(groups []model.GroupView) []model.GroupView {
	cloned := make([]model.GroupView, len(groups))
	for i, group := range groups {
		taskIndexes := make([]int, len(group.TaskIndexes))
		copy(taskIndexes, group.TaskIndexes)
		cloned[i] = model.GroupView{
			Name:          group.Name,
			TaskIndexes:   taskIndexes,
			SelectedCount: group.SelectedCount,
		}
	}
	return cloned
}

func sortedSelectedIndexes(selected map[int]bool) []int {
	indexes := make([]int, 0, len(selected))
	for taskIndex, ok := range selected {
		if ok {
			indexes = append(indexes, taskIndex)
		}
	}
	sort.Ints(indexes)
	return indexes
}

func executionPercent(done, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(done) / float64(total)
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

func normalizePath(path string) string {
	p := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}
