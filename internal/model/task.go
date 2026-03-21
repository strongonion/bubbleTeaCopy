package model

type Operation string

const (
	OpCopy Operation = "copy"
	OpMove Operation = "move"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusPlanned Status = "planned"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
	StatusSkipped Status = "skipped"
)

const DefaultGroup = "ungrouped"

type Task struct {
	Index       int
	Source      string
	Target      string
	Op          Operation
	ClearTarget bool
	Group       string
	Status      Status
	Message     string
}

type GroupView struct {
	Name          string
	TaskIndexes   []int
	SelectedCount int
}
