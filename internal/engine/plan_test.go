package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bubblecopy/internal/model"
)

func TestDryRunSkipsDirectoryToExistingFile(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "src")
	targetFile := filepath.Join(root, "target.txt")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("创建 source 目录失败: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("写入 target 文件失败: %v", err)
	}

	tasks := []model.Task{
		{
			Index:  0,
			Source: sourceDir,
			Target: targetFile,
			Op:     model.OpCopy,
			Group:  "g1",
		},
	}

	plan := DryRun(tasks, []int{0})
	item := plan.ByTask[0]
	if item.ShouldRun {
		t.Fatalf("期望 should_run=false")
	}
	if item.Status != model.StatusSkipped {
		t.Fatalf("status = %s, 期望 %s", item.Status, model.StatusSkipped)
	}
	if !strings.Contains(item.Message, "directory source") {
		t.Fatalf("消息不符合预期: %s", item.Message)
	}
}

func TestDryRunBlocksDuplicateFinalDestination(t *testing.T) {
	root := t.TempDir()
	srcA := filepath.Join(root, "a.txt")
	srcB := filepath.Join(root, "b.txt")
	target := filepath.Join(root, "same.txt")

	mustWriteFile(t, srcA, "a")
	mustWriteFile(t, srcB, "b")

	tasks := []model.Task{
		{
			Index:  0,
			Source: srcA,
			Target: target,
			Op:     model.OpCopy,
			Group:  "g1",
		},
		{
			Index:  1,
			Source: srcB,
			Target: target,
			Op:     model.OpCopy,
			Group:  "g1",
		},
	}

	plan := DryRun(tasks, []int{0, 1})
	for _, idx := range []int{0, 1} {
		item := plan.ByTask[idx]
		if item.ShouldRun {
			t.Fatalf("任务 %d 应该被阻塞", idx)
		}
		if item.Status != model.StatusSkipped {
			t.Fatalf("任务 %d status = %s，期望 skipped", idx, item.Status)
		}
		if !strings.Contains(item.Message, "duplicate final destination") {
			t.Fatalf("任务 %d 消息不符合预期: %s", idx, item.Message)
		}
	}
}

func TestDryRunBlocksDuplicateClearTargetDirectory(t *testing.T) {
	root := t.TempDir()
	srcA := filepath.Join(root, "a.txt")
	srcB := filepath.Join(root, "b.txt")
	targetDir := filepath.Join(root, "out")

	mustWriteFile(t, srcA, "a")
	mustWriteFile(t, srcB, "b")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("创建 target 目录失败: %v", err)
	}

	tasks := []model.Task{
		{
			Index:       0,
			Source:      srcA,
			Target:      targetDir,
			Op:          model.OpCopy,
			ClearTarget: true,
			Group:       "g1",
		},
		{
			Index:       1,
			Source:      srcB,
			Target:      targetDir,
			Op:          model.OpCopy,
			ClearTarget: true,
			Group:       "g1",
		},
	}

	plan := DryRun(tasks, []int{0, 1})
	for _, idx := range []int{0, 1} {
		item := plan.ByTask[idx]
		if item.ShouldRun {
			t.Fatalf("任务 %d 应该被阻塞", idx)
		}
		if item.Status != model.StatusSkipped {
			t.Fatalf("任务 %d status = %s，期望 skipped", idx, item.Status)
		}
		if !strings.Contains(item.Message, "duplicate clear_target") {
			t.Fatalf("任务 %d 消息不符合预期: %s", idx, item.Message)
		}
	}
}

func TestDryRunDirectoryToExistingDirWithClearTargetUsesTargetAsFinalPath(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "src")
	targetDir := filepath.Join(root, "dst")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("创建 source 目录失败: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("创建 target 目录失败: %v", err)
	}

	tasks := []model.Task{
		{
			Index:       0,
			Source:      sourceDir,
			Target:      targetDir,
			Op:          model.OpMove,
			ClearTarget: true,
			Group:       "g1",
		},
	}

	plan := DryRun(tasks, []int{0})
	item := plan.ByTask[0]
	if !item.ShouldRun {
		t.Fatalf("期望 should_run=true, message=%q", item.Message)
	}
	if got, want := item.FinalPath, targetDir; got != want {
		t.Fatalf("finalPath = %q, 期望 %q", got, want)
	}
}

func TestDryRunDirectoryToExistingDirWithoutClearTargetKeepsSubdirFinalPath(t *testing.T) {
	root := t.TempDir()
	sourceDir := filepath.Join(root, "src")
	targetDir := filepath.Join(root, "dst")

	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("创建 source 目录失败: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("创建 target 目录失败: %v", err)
	}

	tasks := []model.Task{
		{
			Index:       0,
			Source:      sourceDir,
			Target:      targetDir,
			Op:          model.OpMove,
			ClearTarget: false,
			Group:       "g1",
		},
	}

	plan := DryRun(tasks, []int{0})
	item := plan.ByTask[0]
	if !item.ShouldRun {
		t.Fatalf("期望 should_run=true, message=%q", item.Message)
	}
	if got, want := item.FinalPath, filepath.Join(targetDir, filepath.Base(sourceDir)); got != want {
		t.Fatalf("finalPath = %q, 期望 %q", got, want)
	}
}

func mustWriteFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建父目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
}
