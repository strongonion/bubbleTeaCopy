package engine

import (
	"fmt"
	"sync"

	"bubblecopy/internal/model"
)

type Result struct {
	TaskIndex int
	Status    model.Status
	Message   string
}

type job struct {
	task model.Task
	item PlanItem
}

func Execute(tasks []model.Task, plan Plan, workers int) []Result {
	if workers < 1 {
		workers = 1
	}

	resultByTask := make(map[int]Result, len(plan.Order))
	var runnable []job

	for _, taskIndex := range plan.Order {
		item, ok := plan.ByTask[taskIndex]
		if !ok {
			resultByTask[taskIndex] = Result{
				TaskIndex: taskIndex,
				Status:    model.StatusFailed,
				Message:   "missing plan item",
			}
			continue
		}

		if !item.ShouldRun {
			resultByTask[taskIndex] = Result{
				TaskIndex: taskIndex,
				Status:    item.Status,
				Message:   item.Message,
			}
			continue
		}

		if taskIndex < 0 || taskIndex >= len(tasks) {
			resultByTask[taskIndex] = Result{
				TaskIndex: taskIndex,
				Status:    model.StatusFailed,
				Message:   "task index out of range",
			}
			continue
		}

		runnable = append(runnable, job{task: tasks[taskIndex], item: item})
	}

	jobs := make(chan job)
	results := make(chan Result, len(runnable))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for current := range jobs {
				results <- runJob(current)
			}
		}()
	}

	go func() {
		for _, current := range runnable {
			jobs <- current
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	for result := range results {
		resultByTask[result.TaskIndex] = result
	}

	ordered := make([]Result, 0, len(plan.Order))
	for _, taskIndex := range plan.Order {
		result, ok := resultByTask[taskIndex]
		if !ok {
			result = Result{
				TaskIndex: taskIndex,
				Status:    model.StatusFailed,
				Message:   "result missing",
			}
		}
		ordered = append(ordered, result)
	}
	return ordered
}

func runJob(current job) Result {
	taskIndex := current.item.TaskIndex

	if current.task.ClearTarget {
		if err := clearTarget(current.item.TargetAbs); err != nil {
			return Result{
				TaskIndex: taskIndex,
				Status:    model.StatusFailed,
				Message:   fmt.Sprintf("clear_target failed: %v", err),
			}
		}
	}

	var err error
	switch current.task.Op {
	case model.OpCopy:
		err = copyPath(current.item.SourceAbs, current.item.FinalPath, current.item.SourceIsDir)
	case model.OpMove:
		err = movePath(current.item.SourceAbs, current.item.FinalPath, current.item.SourceIsDir)
	default:
		return Result{
			TaskIndex: taskIndex,
			Status:    model.StatusFailed,
			Message:   fmt.Sprintf("unsupported op: %s", current.task.Op),
		}
	}
	if err != nil {
		return Result{
			TaskIndex: taskIndex,
			Status:    model.StatusFailed,
			Message:   err.Error(),
		}
	}

	return Result{
		TaskIndex: taskIndex,
		Status:    model.StatusSuccess,
		Message:   fmt.Sprintf("completed: %s", current.item.FinalPath),
	}
}
