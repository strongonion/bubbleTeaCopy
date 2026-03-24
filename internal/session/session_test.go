package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bubblecopy/internal/model"
)

func TestBuildGroupsPreservesFirstAppearanceOrder(t *testing.T) {
	tasks := []model.Task{
		{Index: 0, Group: "group-b"},
		{Index: 1, Group: "group-a"},
		{Index: 2, Group: "group-b"},
		{Index: 3, Group: ""},
	}

	groups := BuildGroups(tasks, map[int]bool{})
	if len(groups) != 3 {
		t.Fatalf("groups len = %d, want 3", len(groups))
	}
	if groups[0].Name != "group-b" {
		t.Fatalf("groups[0] = %q, want group-b", groups[0].Name)
	}
	if groups[1].Name != "group-a" {
		t.Fatalf("groups[1] = %q, want group-a", groups[1].Name)
	}
	if groups[2].Name != model.DefaultGroup {
		t.Fatalf("groups[2] = %q, want %q", groups[2].Name, model.DefaultGroup)
	}
}

func TestDryRunResetKeepsSelectionAndClearsStatuses(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	dst := filepath.Join(root, "dst.txt")
	mustWriteFile(t, src, "hello")

	sess := New([]model.Task{
		{Index: 0, Source: src, Target: dst, Op: model.OpCopy, Group: "docs"},
	}, 2)

	if err := sess.ReplaceSelection([]int{0}); err != nil {
		t.Fatalf("ReplaceSelection() error = %v", err)
	}
	if err := sess.DryRun(); err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	beforeReset := sess.Snapshot()
	if beforeReset.Phase != PhaseDryRun {
		t.Fatalf("phase before reset = %s, want %s", beforeReset.Phase, PhaseDryRun)
	}
	if beforeReset.Tasks[0].Status != model.StatusPlanned {
		t.Fatalf("task status before reset = %s, want %s", beforeReset.Tasks[0].Status, model.StatusPlanned)
	}

	if err := sess.Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	got := sess.Snapshot()
	if got.Phase != PhaseSelect {
		t.Fatalf("phase after reset = %s, want %s", got.Phase, PhaseSelect)
	}
	if got.SelectedCount != 1 {
		t.Fatalf("SelectedCount = %d, want 1", got.SelectedCount)
	}
	if !got.Tasks[0].Selected {
		t.Fatalf("task selection should be preserved after reset")
	}
	if got.Tasks[0].Status != model.StatusPending {
		t.Fatalf("task status after reset = %s, want %s", got.Tasks[0].Status, model.StatusPending)
	}
	if got.Tasks[0].Message != "" {
		t.Fatalf("task message after reset = %q, want empty", got.Tasks[0].Message)
	}
}

func TestStartExecutionUpdatesCountsAndFinishes(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	dst := filepath.Join(root, "dst.txt")
	missing := filepath.Join(root, "missing.txt")
	mustWriteFile(t, src, "hello")

	sess := New([]model.Task{
		{Index: 0, Source: src, Target: dst, Op: model.OpCopy, Group: "docs"},
		{Index: 1, Source: missing, Target: filepath.Join(root, "missing-out.txt"), Op: model.OpCopy, Group: "docs"},
	}, 2)

	if err := sess.ReplaceSelection([]int{0, 1}); err != nil {
		t.Fatalf("ReplaceSelection() error = %v", err)
	}
	if err := sess.DryRun(); err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}
	if err := sess.StartExecution(); err != nil {
		t.Fatalf("StartExecution() error = %v", err)
	}

	got := waitForPhase(t, sess, PhaseResult)
	if got.RunTotal != 1 {
		t.Fatalf("RunTotal = %d, want 1", got.RunTotal)
	}
	if got.RunDone != 1 {
		t.Fatalf("RunDone = %d, want 1", got.RunDone)
	}
	if got.SuccessCount != 1 {
		t.Fatalf("SuccessCount = %d, want 1", got.SuccessCount)
	}
	if got.FailedCount != 1 {
		t.Fatalf("FailedCount = %d, want 1", got.FailedCount)
	}
	if got.ExecutionPercent != 1 {
		t.Fatalf("ExecutionPercent = %.2f, want 1.00", got.ExecutionPercent)
	}
	if !strings.Contains(got.LastUpdate, dst) {
		t.Fatalf("LastUpdate = %q, want to contain %q", got.LastUpdate, dst)
	}
}

func waitForPhase(t *testing.T, sess *Session, want Phase) Snapshot {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := sess.Snapshot()
		if snapshot.Phase == want {
			return snapshot
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("session did not reach phase %s before timeout", want)
	return Snapshot{}
}

func mustWriteFile(t *testing.T, path string, data string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
