package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bubblecopy/internal/model"
)

func TestFindDefaultConfigPrefersTasksCSV(t *testing.T) {
	dir := t.TempDir()
	tasksCSV := filepath.Join(dir, "tasks.csv")
	tasksExampleCSV := filepath.Join(dir, "tasks.example.csv")

	if err := os.WriteFile(tasksCSV, []byte("a"), 0o644); err != nil {
		t.Fatalf("write tasks.csv: %v", err)
	}
	if err := os.WriteFile(tasksExampleCSV, []byte("b"), 0o644); err != nil {
		t.Fatalf("write tasks.example.csv: %v", err)
	}

	got, ok := findDefaultConfig([]string{dir})
	if !ok {
		t.Fatal("expected to find a default config file")
	}
	if got != tasksCSV {
		t.Fatalf("expected %q, got %q", tasksCSV, got)
	}
}

func TestFindDefaultConfigChecksLaterDirs(t *testing.T) {
	firstDir := t.TempDir()
	secondDir := t.TempDir()
	expected := filepath.Join(secondDir, "tasks.example.csv")

	if err := os.WriteFile(expected, []byte("a"), 0o644); err != nil {
		t.Fatalf("write tasks.example.csv: %v", err)
	}

	got, ok := findDefaultConfig([]string{firstDir, secondDir})
	if !ok {
		t.Fatal("expected to find a default config file")
	}
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestFindDefaultConfigReturnsFalseWhenMissing(t *testing.T) {
	dir := t.TempDir()

	if got, ok := findDefaultConfig([]string{dir}); ok {
		t.Fatalf("expected no config file, got %q", got)
	}
}

func TestRunFallsBackToTUIWhenDefaultWebFails(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	webCalls := 0
	tuiCalls := 0

	code := run([]string{"-config", "tasks.csv"}, dependencies{
		stdout: &stdout,
		stderr: &stderr,
		loadCSV: func(path string) ([]model.Task, error) {
			return []model.Task{{Index: 0, Status: model.StatusSuccess}}, nil
		},
		runWeb: func(tasks []model.Task, workers int, listenAddr string, disableBrowser bool) error {
			webCalls++
			return os.ErrInvalid
		},
		runTUI: func(tasks []model.Task, workers int) ([]model.Task, error) {
			tuiCalls++
			return tasks, nil
		},
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if webCalls != 1 {
		t.Fatalf("webCalls = %d, want 1", webCalls)
	}
	if tuiCalls != 1 {
		t.Fatalf("tuiCalls = %d, want 1", tuiCalls)
	}
	if !strings.Contains(stderr.String(), "回退到 TUI") {
		t.Fatalf("stderr = %q, want fallback message", stderr.String())
	}
	if !strings.Contains(stdout.String(), "汇总: 成功=1") {
		t.Fatalf("stdout = %q, want summary", stdout.String())
	}
}

func TestRunExplicitWebDoesNotFallback(t *testing.T) {
	var stderr bytes.Buffer
	tuiCalls := 0

	code := run([]string{"-config", "tasks.csv", "-ui", "web"}, dependencies{
		stderr: &stderr,
		loadCSV: func(path string) ([]model.Task, error) {
			return []model.Task{{Index: 0}}, nil
		},
		runWeb: func(tasks []model.Task, workers int, listenAddr string, disableBrowser bool) error {
			return os.ErrPermission
		},
		runTUI: func(tasks []model.Task, workers int) ([]model.Task, error) {
			tuiCalls++
			return tasks, nil
		},
	})

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if tuiCalls != 0 {
		t.Fatalf("tuiCalls = %d, want 0", tuiCalls)
	}
	if !strings.Contains(stderr.String(), "Web UI 启动失败") {
		t.Fatalf("stderr = %q, want web failure message", stderr.String())
	}
}

func TestRunPassesNoBrowserToWeb(t *testing.T) {
	called := false

	code := run([]string{"-config", "tasks.csv", "-no-browser"}, dependencies{
		loadCSV: func(path string) ([]model.Task, error) {
			return []model.Task{{Index: 0}}, nil
		},
		runWeb: func(tasks []model.Task, workers int, listenAddr string, disableBrowser bool) error {
			called = true
			if !disableBrowser {
				t.Fatalf("disableBrowser = false, want true")
			}
			if listenAddr != "127.0.0.1:0" {
				t.Fatalf("listenAddr = %q, want default listen address", listenAddr)
			}
			return nil
		},
		runTUI: func(tasks []model.Task, workers int) ([]model.Task, error) {
			t.Fatalf("runTUI should not be called")
			return nil, nil
		},
	})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !called {
		t.Fatal("expected runWeb to be called")
	}
}
