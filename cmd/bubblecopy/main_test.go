package main

import (
	"os"
	"path/filepath"
	"testing"
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
