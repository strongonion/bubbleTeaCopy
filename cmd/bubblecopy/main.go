package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bubblecopy/internal/config"
	"bubblecopy/internal/model"
	"bubblecopy/internal/tui"
)

var defaultConfigNames = []string{"tasks.csv", "tasks.example.csv"}

func main() {
	configPath := flag.String("config", "", "任务 CSV 文件路径；留空时自动读取当前目录或程序目录中的 tasks.csv / tasks.example.csv")
	workers := flag.Int("workers", 4, "并发 worker 数量")
	flag.Parse()

	resolvedConfigPath, err := resolveConfigPath(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		fmt.Fprintln(os.Stderr, "用法: bubblecopy [-config <tasks.csv>] [-workers 4]")
		os.Exit(2)
	}

	tasks, err := config.LoadCSV(resolvedConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	finalTasks, err := tui.Run(tasks, *workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TUI 运行错误: %v\n", err)
		os.Exit(1)
	}

	printSummary(finalTasks)
}

func printSummary(tasks []model.Task) {
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
	fmt.Printf("汇总: 成功=%d 失败=%d 跳过=%d\n", success, failed, skipped)
}

func resolveConfigPath(raw string) (string, error) {
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		return trimmed, nil
	}

	if configPath, ok := findDefaultConfig(defaultConfigSearchDirs()); ok {
		return configPath, nil
	}

	return "", fmt.Errorf("未指定 -config，且未在当前目录或程序目录找到默认任务文件：%s", strings.Join(defaultConfigNames, " / "))
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
