package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"bubblecopy/internal/model"
)

type PlanItem struct {
	TaskIndex    int
	SourceAbs    string
	TargetAbs    string
	FinalPath    string
	SourceIsDir  bool
	ClearTarget  bool
	TargetExists bool
	TargetIsDir  bool
	ShouldRun    bool
	Status       model.Status
	Message      string
}

type Plan struct {
	Order  []int
	ByTask map[int]PlanItem
}

func DryRun(tasks []model.Task, selected []int) Plan {
	plan := Plan{
		Order:  append([]int(nil), selected...),
		ByTask: make(map[int]PlanItem, len(selected)),
	}

	for _, taskIndex := range selected {
		if taskIndex < 0 || taskIndex >= len(tasks) {
			plan.ByTask[taskIndex] = PlanItem{
				TaskIndex: taskIndex,
				Status:    model.StatusFailed,
				Message:   "invalid task index",
			}
			continue
		}

		plan.ByTask[taskIndex] = buildPlanItem(tasks[taskIndex], taskIndex)
	}

	blockDuplicateFinalDestinations(&plan)
	blockDuplicateClearDirectory(&plan)
	return plan
}

func buildPlanItem(task model.Task, taskIndex int) PlanItem {
	item := PlanItem{
		TaskIndex:   taskIndex,
		ClearTarget: task.ClearTarget,
		Status:      model.StatusFailed,
	}

	sourceAbs, err := filepath.Abs(task.Source)
	if err != nil {
		item.Message = fmt.Sprintf("source path error: %v", err)
		return item
	}
	sourceAbs = filepath.Clean(sourceAbs)
	item.SourceAbs = sourceAbs

	sourceInfo, err := os.Stat(sourceAbs)
	if err != nil {
		item.Message = fmt.Sprintf("source not available: %v", err)
		return item
	}
	item.SourceIsDir = sourceInfo.IsDir()

	targetAbs, err := filepath.Abs(task.Target)
	if err != nil {
		item.Message = fmt.Sprintf("target path error: %v", err)
		return item
	}
	targetAbs = filepath.Clean(targetAbs)
	item.TargetAbs = targetAbs

	targetInfo, err := os.Stat(targetAbs)
	if err == nil {
		item.TargetExists = true
		item.TargetIsDir = targetInfo.IsDir()
	} else if !os.IsNotExist(err) {
		item.Message = fmt.Sprintf("target stat failed: %v", err)
		return item
	}

	finalPath := targetAbs
	if item.TargetExists {
		if item.TargetIsDir {
			if item.SourceIsDir && task.ClearTarget {
				finalPath = targetAbs
			} else {
				finalPath = filepath.Join(targetAbs, filepath.Base(sourceAbs))
			}
		} else if item.SourceIsDir {
			item.Status = model.StatusSkipped
			item.Message = "skipped: directory source cannot target an existing file"
			return item
		}
	}

	finalPath, err = filepath.Abs(finalPath)
	if err != nil {
		item.Message = fmt.Sprintf("final path error: %v", err)
		return item
	}
	item.FinalPath = filepath.Clean(finalPath)
	item.Status = model.StatusPlanned
	item.ShouldRun = true
	item.Message = "planned"
	return item
}

func blockDuplicateFinalDestinations(plan *Plan) {
	byFinalPath := make(map[string][]int)
	for _, taskIndex := range plan.Order {
		item := plan.ByTask[taskIndex]
		if !item.ShouldRun {
			continue
		}
		key := normalizePath(item.FinalPath)
		byFinalPath[key] = append(byFinalPath[key], taskIndex)
	}

	for _, conflicts := range byFinalPath {
		if len(conflicts) < 2 {
			continue
		}
		for _, taskIndex := range conflicts {
			item := plan.ByTask[taskIndex]
			item.ShouldRun = false
			item.Status = model.StatusSkipped
			item.Message = "blocked: duplicate final destination among selected tasks"
			plan.ByTask[taskIndex] = item
		}
	}
}

func blockDuplicateClearDirectory(plan *Plan) {
	byTargetDir := make(map[string][]int)
	for _, taskIndex := range plan.Order {
		item := plan.ByTask[taskIndex]
		if !item.ShouldRun || !item.ClearTarget || !item.TargetExists || !item.TargetIsDir {
			continue
		}
		key := normalizePath(item.TargetAbs)
		byTargetDir[key] = append(byTargetDir[key], taskIndex)
	}

	for _, conflicts := range byTargetDir {
		if len(conflicts) < 2 {
			continue
		}
		for _, taskIndex := range conflicts {
			item := plan.ByTask[taskIndex]
			item.ShouldRun = false
			item.Status = model.StatusSkipped
			item.Message = "blocked: duplicate clear_target on same directory"
			plan.ByTask[taskIndex] = item
		}
	}
}

func normalizePath(path string) string {
	p := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}
