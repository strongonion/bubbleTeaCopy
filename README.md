# bubblecopy

[中文说明](README.zh-CN.md)

Local batch copy and move tool for grouped file/folder tasks from CSV. It now provides a pure terminal TUI based on tview, optimized for mouse and keyboard interactions.

## CSV format

Required headers:

```csv
source,target,op,clear_target,group
```

Rules:
- `op`: `copy` or `move`
- `clear_target`: `true` or `false`
- `group`: empty value becomes `ungrouped`
- CSV encoding: `UTF-8` recommended; `GB18030/GBK` also supported

## Run

```bash
go run ./cmd/bubblecopy -config ./tasks.example.csv -workers 4
```

By default, the app starts the tview terminal UI.

If `-config` is omitted, the app automatically looks for `tasks.csv` first, then `tasks.example.csv`, in the current directory and the executable directory.
For packaged builds, keep the CSV next to the compiled binary, or launch the app from a directory that already contains the CSV.

## UI

- The terminal TUI provides a dual-pane layout with grouped tasks on the left and task details on the right.
- You can use the mouse to click groups, click tasks, or click the buttons at the bottom (`Select All`, `Unselect All`, `Dry Run`, `Execute`, `Quit`).
- You can also use the keyboard: `Tab` to cycle focus, `Space` or `Enter` to select, and `Enter` on buttons to trigger actions.

## Release

GitHub Actions will automatically build and publish GitHub Releases for:
- Windows `amd64` and `arm64`
- macOS `amd64` and `arm64`
- Linux `amd64` and `arm64`

The workflow is triggered when you push a version tag such as:

```bash
git tag v1.0.0
git push origin v1.0.0
```

Each release asset includes the compiled binary, `tasks.example.csv`, and both README files.

## TUI interactions

- The interface is split into a Groups pane and a Tasks pane, with action buttons at the bottom.
- You can interact entirely using the **Mouse**:
  - Click on a group to select/unselect all its tasks.
  - Click on a task to toggle its selection.
  - Click the buttons (`Select All`, `Unselect All`, `Dry Run`, `Execute`, `Quit`) to control the flow.
- Or use the **Keyboard**:
  - `Tab` / `Left` / `Right`: switch focus between panes and buttons
  - `Up/Down`: move cursor
  - `Space` or `Enter`: toggle selection in lists
  - `Enter`: trigger focused button
  - `q` or `Esc`: quit
