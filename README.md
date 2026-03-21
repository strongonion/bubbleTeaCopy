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
