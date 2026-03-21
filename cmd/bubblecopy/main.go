package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"bubblecopy/internal/config"
	"bubblecopy/internal/model"
	"bubblecopy/internal/tui"
)

func main() {
	configPath := flag.String("config", "", "任务 CSV 文件路径")
	workers := flag.Int("workers", 4, "并发 worker 数量")
	flag.Parse()

	if strings.TrimSpace(*configPath) == "" {
		fmt.Fprintln(os.Stderr, "用法: bubblecopy -config <tasks.csv> [-workers 4]")
		os.Exit(2)
	}

	tasks, err := config.LoadCSV(*configPath)
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
