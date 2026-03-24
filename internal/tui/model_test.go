package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"bubblecopy/internal/engine"
	"bubblecopy/internal/model"
	tea "github.com/charmbracelet/bubbletea"
)

func TestBuildGroupsPreservesFirstAppearanceOrder(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "group-b"},
		{Index: 1, Group: "group-a"},
		{Index: 2, Group: "group-b"},
		{Index: 3, Group: ""},
	}

	groups := buildGroups(tasks, map[int]bool{})
	if len(groups) != 3 {
		t.Fatalf("groups 长度 = %d, 期望 3", len(groups))
	}
	if groups[0].Name != "group-b" {
		t.Fatalf("groups[0] = %q, 期望 group-b", groups[0].Name)
	}
	if groups[1].Name != "group-a" {
		t.Fatalf("groups[1] = %q, 期望 group-a", groups[1].Name)
	}
	if groups[2].Name != model.DefaultGroup {
		t.Fatalf("groups[2] = %q, 期望 %q", groups[2].Name, model.DefaultGroup)
	}
}

func TestGroupSpaceTogglesWholeGroup(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "docs"},
		{Index: 1, Group: "docs"},
		{Index: 2, Group: "media"},
	}

	ui := newModel(tasks, 2)
	ui.focus = focusGroups
	ui.groupCursor = 0
	ui.toggleSelection()

	if !ui.selected[0] || !ui.selected[1] {
		t.Fatalf("期望 docs 分组下全部任务被选中")
	}
	if ui.selected[2] {
		t.Fatalf("期望 media 分组任务保持不变")
	}

	ui.toggleSelection()
	if ui.selected[0] || ui.selected[1] {
		t.Fatalf("期望 docs 分组被取消选择")
	}
}

func TestMoveCursorWrapsGroups(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "docs"},
		{Index: 1, Group: "media"},
	}

	ui := newModel(tasks, 1)
	ui.focus = focusGroups
	ui.groupCursor = 0

	ui.moveCursor(-1)
	if ui.groupCursor != 1 {
		t.Fatalf("groupCursor = %d, 期望 1", ui.groupCursor)
	}

	ui.moveCursor(1)
	if ui.groupCursor != 0 {
		t.Fatalf("groupCursor = %d, 期望 0", ui.groupCursor)
	}
}

func TestMoveCursorWrapsTasks(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "docs"},
		{Index: 1, Group: "docs"},
		{Index: 2, Group: "docs"},
	}

	ui := newModel(tasks, 1)
	ui.focus = focusTasks
	ui.groupCursor = 0
	ui.taskCursor = 0

	ui.moveCursor(-1)
	if ui.taskCursor != 2 {
		t.Fatalf("taskCursor = %d, 期望 2", ui.taskCursor)
	}

	ui.moveCursor(1)
	if ui.taskCursor != 0 {
		t.Fatalf("taskCursor = %d, 期望 0", ui.taskCursor)
	}
}

func TestAnimatedFocusIconFrameChangesAfterTick(t *testing.T) {
	ui := newModel([]model.Task{{Index: 0, Group: "g1"}}, 1)
	first := ui.focusIconFrame()
	ui.Update(animationTickMsg{})
	second := ui.focusIconFrame()

	if first == second {
		t.Fatalf("focus icon 帧未变化: %q", first)
	}
}

func TestCtrlCDoesNotQuit(t *testing.T) {
	ui := newModel([]model.Task{{Index: 0, Group: "g1"}}, 1)

	_, cmd := ui.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("ctrl+c 不应退出，以便终端复制选中文本")
		}
	}
}

func TestQStillQuits(t *testing.T) {
	ui := newModel([]model.Task{{Index: 0, Group: "g1"}}, 1)

	_, cmd := ui.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("q 应继续触发退出")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q 返回的不是退出消息")
	}
}

func TestRenderStatusKeepsTextLabel(t *testing.T) {
	got := renderStatus(model.StatusSuccess)
	if !strings.Contains(got, string(model.StatusSuccess)) {
		t.Fatalf("renderStatus(%q) 未包含状态文本: %q", model.StatusSuccess, got)
	}
}

func TestFormatLatestTaskUsesAbsolutePaths(t *testing.T) {
	srcAbs := filepath.Clean(filepath.Join(t.TempDir(), "源", "文件.txt"))
	targetAbs := filepath.Clean(filepath.Join(t.TempDir(), "目标", "文件.txt"))

	ui := newModel([]model.Task{
		{Index: 0, Source: ".\\ignored-source", Target: ".\\ignored-target", Op: model.OpCopy, Group: "g1"},
	}, 1)
	ui.plan = engine.Plan{
		ByTask: map[int]engine.PlanItem{
			0: {
				TaskIndex: 0,
				SourceAbs: srcAbs,
				FinalPath: targetAbs,
			},
		},
	}

	got := ui.formatLatestTask(engine.Result{TaskIndex: 0, Status: model.StatusSuccess})
	if !strings.Contains(got, srcAbs) {
		t.Fatalf("formatLatestTask() 未包含完整源路径: %q", got)
	}
	if !strings.Contains(got, targetAbs) {
		t.Fatalf("formatLatestTask() 未包含完整目标路径: %q", got)
	}
}

func TestRenderTasksShowsFullPaths(t *testing.T) {
	srcAbs := filepath.Clean(filepath.Join(t.TempDir(), "源", "很长的文件名.txt"))
	targetAbs := filepath.Clean(filepath.Join(t.TempDir(), "目标", "很长的文件名.txt"))

	ui := newModel([]model.Task{
		{Index: 0, Source: srcAbs, Target: targetAbs, Op: model.OpCopy, Group: "g1"},
	}, 1)
	ui.width = 240

	rendered := ui.renderTasks()
	if !strings.Contains(rendered, srcAbs) {
		t.Fatalf("renderTasks() 未显示完整源路径: %q", rendered)
	}
	if !strings.Contains(rendered, targetAbs) {
		t.Fatalf("renderTasks() 未显示完整目标路径: %q", rendered)
	}
}

func TestExecutionPhaseTransitionsDryRunToRunningToResult(t *testing.T) {
	ui := newModel([]model.Task{
		{Index: 0, Source: "src.txt", Target: "dst.txt", Op: model.OpCopy, Group: "g1"},
	}, 1)
	ui.phase = phaseDryRun
	ui.plan = engine.Plan{
		Order: []int{0},
		ByTask: map[int]engine.PlanItem{
			0: {
				TaskIndex: 0,
				ShouldRun: true,
				Status:    model.StatusPlanned,
			},
		},
	}

	stream := make(chan engine.Result)
	cmd := ui.startExecutionWithStream(func() <-chan engine.Result {
		return stream
	})
	if cmd == nil {
		t.Fatalf("startExecutionWithStream() 应返回非空命令")
	}
	if ui.phase != phaseRunning {
		t.Fatalf("phase = %v, 期望 phaseRunning", ui.phase)
	}

	close(stream)
	ui.Update(executionResultMsg{ok: false})
	if ui.phase != phaseResult {
		t.Fatalf("phase = %v, 期望 phaseResult", ui.phase)
	}
}

func TestExecutionResultUpdatesCountsAndProgress(t *testing.T) {
	ui := newModel([]model.Task{
		{Index: 0, Source: "src.txt", Target: "dst.txt", Op: model.OpCopy, Group: "g1"},
	}, 1)
	ui.phase = phaseRunning
	ui.resultStream = make(chan engine.Result)
	ui.runTotal = 1
	ui.runnableByTask[0] = true

	_, cmd := ui.Update(executionResultMsg{
		ok: true,
		result: engine.Result{
			TaskIndex: 0,
			Status:    model.StatusSuccess,
			Message:   "done",
		},
	})

	if cmd == nil {
		t.Fatalf("Update() 应返回后续等待命令")
	}
	if ui.tasks[0].Status != model.StatusSuccess {
		t.Fatalf("task status = %s, 期望 %s", ui.tasks[0].Status, model.StatusSuccess)
	}
	if ui.runDone != 1 {
		t.Fatalf("runDone = %d, 期望 1", ui.runDone)
	}
	if ui.successCount != 1 {
		t.Fatalf("successCount = %d, 期望 1", ui.successCount)
	}
	if got := ui.executionPercent(); got != 1 {
		t.Fatalf("executionPercent = %.2f, 期望 1.00", got)
	}
}

func TestAnimationTickStopsAfterResultPhase(t *testing.T) {
	ui := newModel([]model.Task{{Index: 0, Group: "g1"}}, 1)
	ui.phase = phaseSelect

	_, cmd := ui.Update(animationTickMsg{})
	if cmd == nil {
		t.Fatalf("选择阶段动画 tick 应继续调度")
	}
	if ui.animFrame == 0 {
		t.Fatalf("animFrame 未推进")
	}

	before := ui.animFrame
	ui.phase = phaseResult
	_, cmd = ui.Update(animationTickMsg{})
	if cmd != nil {
		t.Fatalf("结果阶段动画 tick 应停止")
	}
	if ui.animFrame != before {
		t.Fatalf("结果阶段不应推进 animFrame, got=%d want=%d", ui.animFrame, before)
	}
}

func TestRenderTasksClipsToPanelHeight(t *testing.T) {
	tasks := make([]model.Task, 12)
	for i := range tasks {
		tasks[i] = model.Task{
			Index:  i,
			Group:  "g1",
			Source: fmt.Sprintf("src-%02d.txt", i),
			Target: fmt.Sprintf("dst-%02d.txt", i),
			Op:     model.OpCopy,
		}
	}

	ui := newModel(tasks, 1)
	ui.focus = focusTasks
	ui.groupCursor = 0
	ui.taskCursor = 8
	ui.width = 80
	ui.height = 20

	rendered := ui.renderTasks()
	lines := strings.Split(rendered, "\n")
	max := ui.panelContentHeight()
	if max > 0 && len(lines) > max {
		t.Fatalf("renderTasks() 行数 = %d, 期望 <= %d", len(lines), max)
	}
	if !strings.Contains(rendered, "src-08.txt") {
		t.Fatalf("renderTasks() 未显示焦点任务内容: %q", rendered)
	}
}
