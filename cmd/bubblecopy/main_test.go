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

func TestParseOptionsDefaults(t *testing.T) {
	var stdout bytes.Buffer
	opts, err := parseOptions(nil, &stdout)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}

	if opts.configPath != "" {
		t.Fatalf("configPath = %q, want empty", opts.configPath)
	}
	if opts.workers != 4 {
		t.Fatalf("workers = %d, want 4", opts.workers)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestParseOptionsRejectsUnknownFlag(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"-unknown"}, &stderr)
	if err == nil {
		t.Fatal("expected parseOptions to fail for unknown flag")
	}
	if !strings.Contains(stderr.String(), usageLine) {
		t.Fatalf("stderr = %q, want usage line", stderr.String())
	}
}

func TestResolveConfigPathReturnsExplicitPath(t *testing.T) {
	got, err := resolveConfigPath("custom.csv")
	if err != nil {
		t.Fatalf("resolveConfigPath returned error: %v", err)
	}
	if got != "custom.csv" {
		t.Fatalf("got %q, want %q", got, "custom.csv")
	}
}

func TestResolveConfigPathFindsTasksCSVInWorkingDir(t *testing.T) {
	dir := t.TempDir()
	withWorkingDir(t, dir)

	expected := filepath.Join(dir, "tasks.csv")
	if err := os.WriteFile(expected, []byte("source,target,op,clear_target,group\n"), 0o644); err != nil {
		t.Fatalf("write tasks.csv: %v", err)
	}

	got, err := resolveConfigPath("")
	if err != nil {
		t.Fatalf("resolveConfigPath returned error: %v", err)
	}
	if got != expected {
		t.Fatalf("got %q, want %q", got, expected)
	}
}

func TestRunReturnsUsageOnFlagError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"-unknown"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), usageLine) {
		t.Fatalf("stderr = %q, want usage line", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunReturnsLoadCSVErrorForMissingConfig(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	missingPath := filepath.Join(t.TempDir(), "missing.csv")
	code := run([]string{"-config", missingPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Failed to load CSV:") {
		t.Fatalf("stderr = %q, want load failure message", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestPrintSummaryCountsStatuses(t *testing.T) {
	var out bytes.Buffer

	printSummary(&out, []model.Task{
		{Status: model.StatusSuccess},
		{Status: model.StatusSuccess},
		{Status: model.StatusFailed},
		{Status: model.StatusSkipped},
	})

	got := out.String()
	want := "Result: Success=2 Failed=1 Skipped=1\n"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
}

func withWorkingDir(t *testing.T, dir string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("restore wd to %q: %v", oldDir, err)
		}
	})
}
