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
	"bubblecopy/internal/web"
)

var defaultConfigNames = []string{"tasks.csv", "tasks.example.csv"}

const usageLine = "用法: bubblecopy [-config <tasks.csv>] [-workers 4] [-ui web|tui] [-listen 127.0.0.1:0] [-no-browser]"

type options struct {
	configPath string
	workers    int
	ui         string
	listen     string
	noBrowser  bool
	uiExplicit bool
}

type dependencies struct {
	stdout  io.Writer
	stderr  io.Writer
	loadCSV func(path string) ([]model.Task, error)
	runTUI  func(tasks []model.Task, workers int) ([]model.Task, error)
	runWeb  func(tasks []model.Task, workers int, listenAddr string, disableBrowser bool) error
}

func main() {
	code := run(os.Args[1:], dependencies{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		loadCSV: config.LoadCSV,
		runTUI:  tui.Run,
		runWeb:  web.Run,
	})
	os.Exit(code)
}

func run(args []string, deps dependencies) int {
	if deps.stdout == nil {
		deps.stdout = io.Discard
	}
	if deps.stderr == nil {
		deps.stderr = io.Discard
	}
	if deps.loadCSV == nil {
		deps.loadCSV = config.LoadCSV
	}
	if deps.runTUI == nil {
		deps.runTUI = tui.Run
	}
	if deps.runWeb == nil {
		deps.runWeb = web.Run
	}

	opts, err := parseOptions(args, deps.stderr)
	if err != nil {
		return 2
	}

	resolvedConfigPath, err := resolveConfigPath(opts.configPath)
	if err != nil {
		fmt.Fprintf(deps.stderr, "%v\n", err)
		fmt.Fprintln(deps.stderr, usageLine)
		return 2
	}

	tasks, err := deps.loadCSV(resolvedConfigPath)
	if err != nil {
		fmt.Fprintf(deps.stderr, "加载配置失败: %v\n", err)
		return 1
	}

	switch opts.ui {
	case "tui":
		return runTUIWithSummary(deps, tasks, opts.workers)
	case "web":
		err := deps.runWeb(tasks, opts.workers, opts.listen, opts.noBrowser)
		if err == nil {
			return 0
		}
		if opts.uiExplicit {
			fmt.Fprintf(deps.stderr, "Web UI 启动失败: %v\n", err)
			return 1
		}
		fmt.Fprintf(deps.stderr, "Web UI 启动失败，回退到 TUI: %v\n", err)
		return runTUIWithSummary(deps, tasks, opts.workers)
	default:
		fmt.Fprintf(deps.stderr, "无效的 -ui 值 %q，期望 web 或 tui\n", opts.ui)
		fmt.Fprintln(deps.stderr, usageLine)
		return 2
	}
}

func runTUIWithSummary(deps dependencies, tasks []model.Task, workers int) int {
	finalTasks, err := deps.runTUI(tasks, workers)
	if err != nil {
		fmt.Fprintf(deps.stderr, "TUI 运行错误: %v\n", err)
		return 1
	}

	printSummary(deps.stdout, finalTasks)
	return 0
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("bubblecopy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	configPath := fs.String("config", "", "任务 CSV 文件路径；留空时自动读取当前目录或程序目录中的 tasks.csv / tasks.example.csv")
	workers := fs.Int("workers", 4, "并发 worker 数量")
	ui := fs.String("ui", "web", "界面模式：web 或 tui")
	listen := fs.String("listen", "127.0.0.1:0", "Web 模式监听地址")
	noBrowser := fs.Bool("no-browser", false, "Web 模式下不自动打开浏览器")

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		fmt.Fprintln(stderr, usageLine)
		return options{}, err
	}

	uiExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "ui" {
			uiExplicit = true
		}
	})

	return options{
		configPath: *configPath,
		workers:    *workers,
		ui:         strings.ToLower(strings.TrimSpace(*ui)),
		listen:     strings.TrimSpace(*listen),
		noBrowser:  *noBrowser,
		uiExplicit: uiExplicit,
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
	fmt.Fprintf(out, "汇总: 成功=%d 失败=%d 跳过=%d\n", success, failed, skipped)
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
