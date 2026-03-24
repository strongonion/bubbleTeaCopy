# bubblecopy
![alt text](image-2.png)

[中文说明](README.zh-CN.md)

Local batch copy and move tool for grouped file/folder tasks from CSV. It now starts a browser-based visual UI by default and still keeps the Bubble Tea TUI as a compatibility mode.

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

By default, the app starts a local web UI on `127.0.0.1` with a random free port and opens your default browser automatically.

If `-config` is omitted, the app automatically looks for `tasks.csv` first, then `tasks.example.csv`, in the current directory and the executable directory.
For packaged builds, keep the CSV next to the compiled binary, or launch the app from a directory that already contains the CSV.

Useful flags:

```bash
# keep the browser UI but do not auto-open the browser
go run ./cmd/bubblecopy -config ./tasks.example.csv -no-browser

# choose a fixed listening address for the local web UI
go run ./cmd/bubblecopy -config ./tasks.example.csv -listen 127.0.0.1:8080

# use the legacy Bubble Tea terminal UI
go run ./cmd/bubblecopy -config ./tasks.example.csv -ui tui
```

## Web UI

- Default mode is a local single-user web UI served from the current process.
- The browser flow covers task browsing, grouped selection, dry-run, execution, progress, latest update, and final result counts.
- The UI only visualizes and controls the CSV already loaded on startup. Browser upload/edit/save is intentionally not included in v1.

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

## TUI keys

The following keys apply when you explicitly start `-ui tui`:

- `Left/Right`: switch focus between left group pane and right task pane
- `Up/Down` or `j/k`: move cursor
- `Space` on group: select/unselect all tasks in that group
- `Space` on task: toggle one task
- `Enter` (first): dry-run
- `Enter` (second): execute
- `q`: quit

## Animated execution view

- The interface uses a warm high-saturation palette (amber/orange/coral) across title, borders, focus, and status labels.
- Selection phase animates focus and Unicode icons in both group and task panes to keep navigation visually clear.
- Execution phase shows a live spinner, a warm gradient progress bar, a dynamic running icon, completed count (`done/total`), and rolling success/failed/skipped stats.
- The `Current` line shows the latest updated task in real time.
- Result phase stops animation and keeps a static final summary.
