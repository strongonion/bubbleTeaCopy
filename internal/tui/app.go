package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"bubblecopy/internal/model"
	"bubblecopy/internal/session"
)

type UI struct {
	app        *tview.Application
	sess       *session.Session
	flex       *tview.Flex
	header     *tview.TextView
	groupsList *tview.List
	tasksTable *tview.Table
	footer     *tview.TextView

	btnSelectAll   *tview.Button
	btnUnselectAll *tview.Button
	btnDryRun      *tview.Button
	btnExecute     *tview.Button
	btnQuit        *tview.Button

	currentGroupIndex int
}

func Run(tasks []model.Task, workers int) ([]model.Task, error) {
	sess := session.New(tasks, workers)
	ui := &UI{
		app:        tview.NewApplication(),
		sess:       sess,
		header:     tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter),
		groupsList: tview.NewList().ShowSecondaryText(false),
		tasksTable: tview.NewTable().SetSelectable(true, false).SetBorders(false),
		footer:     tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft),
	}

	ui.groupsList.SetBorder(true).SetTitle("Groups").SetTitleAlign(tview.AlignLeft)
	ui.tasksTable.SetBorder(true).SetTitle("Tasks").SetTitleAlign(tview.AlignLeft)

	ui.btnSelectAll = tview.NewButton("Select All").SetSelectedFunc(ui.selectAll)
	ui.btnUnselectAll = tview.NewButton("Unselect All").SetSelectedFunc(ui.unselectAll)
	ui.btnDryRun = tview.NewButton("Dry Run").SetSelectedFunc(ui.doDryRun)
	ui.btnExecute = tview.NewButton("Execute").SetSelectedFunc(ui.doExecute)
	ui.btnQuit = tview.NewButton("Quit").SetSelectedFunc(func() { ui.app.Stop() })

	buttonsFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.btnSelectAll, 0, 1, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(ui.btnUnselectAll, 0, 1, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(ui.btnDryRun, 0, 1, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(ui.btnExecute, 0, 1, false).
		AddItem(tview.NewBox(), 1, 0, false).
		AddItem(ui.btnQuit, 0, 1, false)

	mainFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(ui.groupsList, 0, 1, true).
		AddItem(ui.tasksTable, 0, 2, false)

	ui.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.header, 2, 0, false).
		AddItem(mainFlex, 0, 1, true).
		AddItem(ui.footer, 2, 0, false).
		AddItem(buttonsFlex, 3, 0, false)

	ui.app.EnableMouse(true)
	ui.app.SetRoot(ui.flex, true)

	ui.setupKeyboard()
	ui.refresh()

	if err := ui.app.Run(); err != nil {
		return nil, err
	}

	return ui.sess.Tasks(), nil
}

func (ui *UI) setupKeyboard() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		snap := ui.sess.Snapshot()
		
		switch event.Key() {
		case tcell.KeyEsc:
			ui.app.Stop()
			return nil
		case tcell.KeyCtrlC:
			ui.app.Stop()
			return nil
		case tcell.KeyTab:
			// Tab cycles through main panels and buttons
			focus := ui.app.GetFocus()
			if focus == ui.groupsList {
				ui.app.SetFocus(ui.tasksTable)
			} else if focus == ui.tasksTable {
				ui.app.SetFocus(ui.btnSelectAll)
			} else {
				ui.app.SetFocus(ui.groupsList)
			}
			return nil
		}

		switch event.Rune() {
		case 'q', 'Q':
			ui.app.Stop()
			return nil
		case ' ':
			if snap.Phase == session.PhaseSelect {
				focus := ui.app.GetFocus()
				if focus == ui.groupsList || focus == ui.tasksTable {
					ui.toggleSelection()
					ui.refresh()
					return nil
				}
			}
		}

		return event
	})

	ui.groupsList.SetChangedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		ui.currentGroupIndex = index
		ui.refreshTasksTable()
	})
	
	// Mouse double clicks or enter on groups list
	ui.groupsList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		snap := ui.sess.Snapshot()
		if snap.Phase == session.PhaseSelect {
			ui.toggleGroupSelection(index)
			ui.refresh()
		}
	})

	// Enter or double click on tasks table
	ui.tasksTable.SetSelectedFunc(func(row int, column int) {
		snap := ui.sess.Snapshot()
		if snap.Phase == session.PhaseSelect {
			ui.toggleTaskSelection(row)
			ui.refresh()
		}
	})
}

func (ui *UI) toggleSelection() {
	focus := ui.app.GetFocus()
	if focus == ui.groupsList {
		ui.toggleGroupSelection(ui.groupsList.GetCurrentItem())
	} else if focus == ui.tasksTable {
		row, _ := ui.tasksTable.GetSelection()
		ui.toggleTaskSelection(row)
	}
}

func (ui *UI) selectAll() {
	snap := ui.sess.Snapshot()
	if snap.Phase != session.PhaseSelect {
		return
	}
	all := make([]int, len(snap.Tasks))
	for i, t := range snap.Tasks {
		all[i] = t.Index
	}
	ui.sess.ReplaceSelection(all)
	ui.refresh()
}

func (ui *UI) unselectAll() {
	snap := ui.sess.Snapshot()
	if snap.Phase != session.PhaseSelect {
		return
	}
	ui.sess.ReplaceSelection([]int{})
	ui.refresh()
}

func (ui *UI) doDryRun() {
	snap := ui.sess.Snapshot()
	if snap.Phase == session.PhaseSelect {
		if err := ui.sess.DryRun(); err == nil {
			ui.refresh()
		}
	}
}

func (ui *UI) doExecute() {
	snap := ui.sess.Snapshot()
	if snap.Phase == session.PhaseDryRun {
		if err := ui.sess.StartExecution(); err == nil {
			ui.startPolling()
			ui.refresh()
		}
	}
}

func (ui *UI) toggleGroupSelection(groupIndex int) {
	snap := ui.sess.Snapshot()
	if groupIndex < 0 || groupIndex >= len(snap.Groups) {
		return
	}
	group := snap.Groups[groupIndex]
	allSelected := group.SelectedCount == len(group.TaskIndexes)
	
	newSelected := make([]int, 0)
	for _, task := range snap.Tasks {
		if task.Selected {
			newSelected = append(newSelected, task.Index)
		}
	}
	
	if allSelected {
		// Deselect all in group
		filtered := []int{}
		for _, idx := range newSelected {
			inGroup := false
			for _, gIdx := range group.TaskIndexes {
				if idx == gIdx {
					inGroup = true
					break
				}
			}
			if !inGroup {
				filtered = append(filtered, idx)
			}
		}
		newSelected = filtered
	} else {
		// Select all in group
		for _, gIdx := range group.TaskIndexes {
			found := false
			for _, idx := range newSelected {
				if idx == gIdx {
					found = true
					break
				}
			}
			if !found {
				newSelected = append(newSelected, gIdx)
			}
		}
	}
	ui.sess.ReplaceSelection(newSelected)
}

func (ui *UI) toggleTaskSelection(row int) {
	snap := ui.sess.Snapshot()
	if ui.currentGroupIndex < 0 || ui.currentGroupIndex >= len(snap.Groups) {
		return
	}
	group := snap.Groups[ui.currentGroupIndex]
	if row < 0 || row >= len(group.TaskIndexes) {
		return
	}
	
	taskIndex := group.TaskIndexes[row]
	
	newSelected := make([]int, 0)
	found := false
	for _, task := range snap.Tasks {
		if task.Selected {
			if task.Index == taskIndex {
				found = true
			} else {
				newSelected = append(newSelected, task.Index)
			}
		}
	}
	if !found {
		newSelected = append(newSelected, taskIndex)
	}
	
	ui.sess.ReplaceSelection(newSelected)
}

func (ui *UI) refresh() {
	snap := ui.sess.Snapshot()

	// Update Header
	headerText := fmt.Sprintf("[yellow]BUBBLECOPY[white] - batch copy/move")
	if snap.Phase == session.PhaseRunning {
		headerText += fmt.Sprintf(" | Running: %d/%d (%.1f%%) | Success: %d, Failed: %d", 
			snap.RunDone, snap.RunTotal, snap.ExecutionPercent*100, snap.SuccessCount, snap.FailedCount)
	} else if snap.Phase == session.PhaseResult {
		headerText += fmt.Sprintf(" | Complete | Success: %d, Failed: %d", snap.SuccessCount, snap.FailedCount)
	}
	ui.header.SetText(headerText)

	// Update Footer
	var help string
	switch snap.Phase {
	case session.PhaseSelect:
		help = "Click or use Tab/Space/Enter. double-click/Enter to toggle selection."
	case session.PhaseDryRun:
		help = "Dry-run ready. Click Execute to start."
	case session.PhaseRunning:
		help = "Execution running..."
	case session.PhaseResult:
		help = "Execution complete."
	}
	if snap.Message != "" {
		help += "\n" + snap.Message
	}
	ui.footer.SetText(help)

	// Update Button visibility/styles
	ui.btnSelectAll.SetDisabled(snap.Phase != session.PhaseSelect)
	ui.btnUnselectAll.SetDisabled(snap.Phase != session.PhaseSelect)
	ui.btnDryRun.SetDisabled(snap.Phase != session.PhaseSelect)
	ui.btnExecute.SetDisabled(snap.Phase != session.PhaseDryRun)

	// Update Groups List
	oldGroupIdx := ui.groupsList.GetCurrentItem()
	ui.groupsList.Clear()
	for _, g := range snap.Groups {
		mark := "[ ]"
		if len(g.TaskIndexes) > 0 {
			if g.SelectedCount == len(g.TaskIndexes) {
				mark = "[X]"
			} else if g.SelectedCount > 0 {
				mark = "[-]"
			}
		}
		title := fmt.Sprintf("%s %s (%d/%d)", mark, g.Name, g.SelectedCount, len(g.TaskIndexes))
		ui.groupsList.AddItem(title, "", 0, nil)
	}
	if oldGroupIdx >= 0 && oldGroupIdx < ui.groupsList.GetItemCount() {
		ui.groupsList.SetCurrentItem(oldGroupIdx)
	}

	// Update Tasks Table
	ui.refreshTasksTable()
	ui.app.Draw()
}

func (ui *UI) refreshTasksTable() {
	snap := ui.sess.Snapshot()
	ui.tasksTable.Clear()

	if ui.currentGroupIndex < 0 || ui.currentGroupIndex >= len(snap.Groups) {
		return
	}
	
	group := snap.Groups[ui.currentGroupIndex]
	for row, taskIdx := range group.TaskIndexes {
		task := snap.Tasks[taskIdx]
		
		mark := "[ ]"
		if task.Selected {
			mark = "[X]"
		}
		
		statusColor := "white"
		switch task.Status {
		case model.StatusSuccess:
			statusColor = "green"
		case model.StatusFailed:
			statusColor = "red"
		case model.StatusSkipped:
			statusColor = "yellow"
		case model.StatusPlanned:
			statusColor = "blue"
		}
		
		ui.tasksTable.SetCell(row, 0, tview.NewTableCell(mark).SetTextColor(tcell.ColorYellow))
		ui.tasksTable.SetCell(row, 1, tview.NewTableCell(string(task.Op)).SetTextColor(tcell.ColorDarkCyan))
		ui.tasksTable.SetCell(row, 2, tview.NewTableCell(task.SourcePath).SetTextColor(tcell.ColorWhite))
		ui.tasksTable.SetCell(row, 3, tview.NewTableCell("->").SetTextColor(tcell.ColorGray))
		ui.tasksTable.SetCell(row, 4, tview.NewTableCell(task.TargetPath).SetTextColor(tcell.ColorWhite))
		ui.tasksTable.SetCell(row, 5, tview.NewTableCell(fmt.Sprintf("[%s]%s[white]", statusColor, task.Status)))
		ui.tasksTable.SetCell(row, 6, tview.NewTableCell(task.Message).SetTextColor(tcell.ColorGray))
	}
}

func (ui *UI) startPolling() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			<-ticker.C
			snap := ui.sess.Snapshot()
			ui.app.QueueUpdateDraw(func() {
				ui.refresh()
			})
			if snap.Phase == session.PhaseResult {
				return
			}
		}
	}()
}
