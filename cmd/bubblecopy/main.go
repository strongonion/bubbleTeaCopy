package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bubblecopy/internal/config"
	"bubblecopy/internal/model"
	"bubblecopy/internal/tui"
)

var defaultConfigNames = []string{"tasks.csv", "tasks.example.csv"}

const usageLine = "Usage: bubblecopy [-config <tasks.csv>] [-workers 4]"

type options struct {
	configPath string
	workers    int
}

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	opts, err := parseOptions(args, stderr)
	if err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		fmt.Fprintln(stderr, usageLine)
		return 2
	}

	tasks, err := config.LoadCSV(resolvedConfigPath)
	if err != nil {
		fmt.Fprintf(stderr, "Failed to load CSV: %v\n", err)
		return 1
	}

	finalTasks, err := tui.Run(tasks, opts.workers)
	if err != nil {
		fmt.Fprintf(stderr, "TUI Error: %v\n", err)
		return 1
	}

	printSummary(stdout, finalTasks)
	return 0
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("bubblecopy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	configPath := fs.String("config", "", "Path to tasks.csv. If empty, uses tasks.csv or tasks.example.csv in current directory.")
	workers := fs.Int("workers", 4, "Number of worker goroutines")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		fmt.Fprintln(stderr, usageLine)
		return options{}, err
	}

	return options{
		configPath: *configPath,
		workers:    *workers,
	}, nil
}

func printSummary(out io.Writer, tasks []model.Task) {
	var success, failed, skipped int
	for _, task := range tasks {
		switch task.Status {
		case model.StatusSuccess:
			success++
		case model.StatusFailed:
			failed++
		case model.StatusSkipped:
			skipped++
		}
	}
	fmt.Fprintf(out, "Result: Success=%d Failed=%d Skipped=%d\n", success, failed, skipped)
}

func resolveConfigPath(raw string) (string, error) {
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		return trimmed, nil
	}

	if configPath, ok := findDefaultConfig(defaultConfigSearchDirs()); ok {
		return configPath, nil
	}

	return "", fmt.Errorf("no config specified, and %s not found in default locations", strings.Join(defaultConfigNames, " / "))
}

func defaultConfigSearchDirs() []string {
	dirs := make([]string, 0, 2)

	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, cwd)
	}

	if exePath, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exePath))
	}

	return dirs
}

func findDefaultConfig(dirs []string) (string, bool) {
	for _, dir := range dirs {
		for _, name := range defaultConfigNames {
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if err == nil && !info.IsDir() {
				return path, true
			}
		}
	}

	return "", false
}
