package engine

import (
	"path/filepath"
	"strings"
	"testing"

	"bubblecopy/internal/model"
)

func TestExecuteStreamEmitsAllResultsAndCloses(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	dst := filepath.Join(root, "dst.txt")
	missing := filepath.Join(root, "missing.txt")

	mustWriteFileTest(t, src, "hello")

	tasks := []model.Task{
		{
			Index:  0,
			Source: src,
			Target: dst,
			Op:     model.OpCopy,
			Group:  "g1",
		},
		{
			Index:  1,
			Source: missing,
			Target: filepath.Join(root, "missing-out.txt"),
			Op:     model.OpCopy,
			Group:  "g1",
		},
	}

	plan := DryRun(tasks, []int{0, 1})
	results := collectResults(ExecuteStream(tasks, plan, 2))

	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}

	if got := results[0].Status; got != model.StatusSuccess {
		t.Fatalf("task 0 status = %s, want %s", got, model.StatusSuccess)
	}
	if got := results[1].Status; got != model.StatusFailed {
		t.Fatalf("task 1 status = %s, want %s", got, model.StatusFailed)
	}
	if !strings.Contains(results[1].Message, "source not available") {
		t.Fatalf("task 1 message = %q, want source not available", results[1].Message)
	}
}

func TestExecuteStreamPreservesNonRunnableStatuses(t *testing.T) {
	root := t.TempDir()
	srcA := filepath.Join(root, "a.txt")
	srcB := filepath.Join(root, "b.txt")
	sameTarget := filepath.Join(root, "same.txt")

	mustWriteFileTest(t, srcA, "a")
	mustWriteFileTest(t, srcB, "b")

	tasks := []model.Task{
		{
			Index:  0,
			Source: srcA,
			Target: sameTarget,
			Op:     model.OpCopy,
			Group:  "g1",
		},
		{
			Index:  1,
			Source: srcB,
			Target: sameTarget,
			Op:     model.OpCopy,
			Group:  "g1",
		},
	}

	plan := DryRun(tasks, []int{0, 1})
	results := collectResults(ExecuteStream(tasks, plan, 4))

	if len(results) != 2 {
		t.Fatalf("result count = %d, want 2", len(results))
	}

	for _, idx := range []int{0, 1} {
		result := results[idx]
		if got := result.Status; got != model.StatusSkipped {
			t.Fatalf("task %d status = %s, want %s", idx, got, model.StatusSkipped)
		}
		if !strings.Contains(result.Message, "duplicate final destination") {
			t.Fatalf("task %d message = %q, want duplicate final destination", idx, result.Message)
		}
	}
}

func collectResults(stream <-chan Result) map[int]Result {
	byTask := make(map[int]Result)
	for result := range stream {
		byTask[result.TaskIndex] = result
	}
	return byTask
}
