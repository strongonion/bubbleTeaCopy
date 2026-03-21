# bubblecopy

Bubble Tea TUI tool for grouped file/folder copy and move tasks from CSV.

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

## TUI keys

- `Tab`: switch focus between left group pane and right task pane
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
