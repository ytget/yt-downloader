package ui

import (
	"fmt"
	"image/color"
	"log"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/ytget/yt-downloader/internal/model"
)

// File size formatting constants
const (
	FileSizeUnit  = 1024
	FileSizeUnits = "KMGTPE"
)

// Progress calculation constants
const (
	MaxProgressPercent  = 100
	MinProgressPercent  = 1
	RoundingCoefficient = 0.5
)

// Dialog size constants
const (
	TaskRowDialogWidth  = 500
	TaskRowDialogHeight = 400
)

// formatFileSize formats file size in bytes to human readable format
func formatFileSize(bytes int64) string {
	if bytes < FileSizeUnit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(FileSizeUnit), 0
	for n := bytes / FileSizeUnit; n >= FileSizeUnit; n /= FileSizeUnit {
		div *= FileSizeUnit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), FileSizeUnits[exp])
}

// TaskRow represents a compact task row widget
type TaskRow struct {
	widget.BaseWidget

	task         *model.DownloadTask
	localization *Localization

	// UI components
	titleLabel    *widget.Label
	statusLabel   *widget.Label
	progressLabel *widget.Label
	speedEtaLabel *widget.Label

	// Action buttons
	startPauseBtn *widget.Button
	openBtn       *widget.Button // reveal in file manager
	playBtn       *widget.Button // open file with default app (player)
	copyBtn       *widget.Button

	// Callbacks
	onStartPause func(taskID string)
	onReveal     func(filePath string)
	onOpen       func(filePath string)
	onCopyPath   func(filePath string)
	onRemove     func(taskID string)
}

// NewTaskRow creates a new task row widget
func NewTaskRow(task *model.DownloadTask, localization *Localization) *TaskRow {
	if task == nil {
		log.Printf("Warning: NewTaskRow called with nil task")
		// Create a dummy task to prevent crashes
		task = &model.DownloadTask{
			ID:     "dummy",
			Status: model.TaskStatusPending,
			Title:  "Dummy Task",
		}
	}

	tr := &TaskRow{
		task:         task,
		localization: localization,
	}
	tr.ExtendBaseWidget(tr)
	tr.createUI()
	tr.updateFromTask()
	return tr
}

// SetCallbacks sets the action callbacks
func (tr *TaskRow) SetCallbacks(
	onStartPause func(taskID string),
	onReveal func(filePath string),
	onOpen func(filePath string),
	onCopyPath func(filePath string),
	onRemove func(taskID string),
) {
	// Log callback status for debugging
	if onStartPause == nil {
		log.Printf("Warning: onStartPause callback is nil for task %s", tr.task.ID)
	}
	if onReveal == nil {
		log.Printf("Warning: onReveal callback is nil for task %s", tr.task.ID)
	}
	if onOpen == nil {
		log.Printf("Warning: onOpen callback is nil for task %s", tr.task.ID)
	}
	if onCopyPath == nil {
		log.Printf("Warning: onCopyPath callback is nil for task %s", tr.task.ID)
	}
	if onRemove == nil {
		log.Printf("Warning: onRemove callback is nil for task %s", tr.task.ID)
	}

	tr.onStartPause = onStartPause
	tr.onReveal = onReveal
	tr.onOpen = onOpen
	tr.onCopyPath = onCopyPath
	tr.onRemove = onRemove
}

// UpdateTask updates the row with new task data
func (tr *TaskRow) UpdateTask(task *model.DownloadTask) {
	if task == nil {
		log.Printf("Warning: UpdateTask called with nil task for existing task %s", tr.task.ID)
		return
	}

	// Clean task data to prevent display issues
	if task.URL != "" {
		task.URL = strings.ReplaceAll(task.URL, "\n", "")
		task.URL = strings.ReplaceAll(task.URL, "\r", "")
		task.URL = strings.ReplaceAll(task.URL, "\t", " ")
		task.URL = strings.TrimSpace(task.URL)
	}

	if task.Title != "" {
		task.Title = strings.ReplaceAll(task.Title, "\n", " ")
		task.Title = strings.ReplaceAll(task.Title, "\r", " ")
		task.Title = strings.ReplaceAll(task.Title, "\t", " ")
		task.Title = strings.TrimSpace(task.Title)
	}

	log.Printf("Updating TaskRow for task %s: Status=%s, OutputPath=%s",
		task.ID, task.Status, task.OutputPath)

	tr.task = task
	tr.updateFromTask()
	tr.Refresh()

	// Force refresh to ensure proper display
	tr.Refresh()
}

// createUI creates the UI components
func (tr *TaskRow) createUI() {
	// Create labels
	tr.titleLabel = widget.NewLabel("")
	tr.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	// Allow wrapping for readability; truncate with ellipsis if overflows
	tr.titleLabel.Wrapping = fyne.TextWrapWord
	tr.titleLabel.Truncation = fyne.TextTruncateEllipsis

	// Ensure proper text display
	tr.titleLabel.Alignment = fyne.TextAlignLeading

	tr.statusLabel = widget.NewLabel("")
	tr.statusLabel.Alignment = fyne.TextAlignTrailing
	tr.progressLabel = widget.NewLabel("")
	tr.progressLabel.Alignment = fyne.TextAlignTrailing
	tr.speedEtaLabel = widget.NewLabel("")
	tr.speedEtaLabel.Alignment = fyne.TextAlignLeading
	tr.speedEtaLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Create action buttons
	tr.startPauseBtn = widget.NewButton(tr.localization.GetText(KeyPause), func() {
		// Get current task state dynamically
		currentTask := tr.task
		log.Printf("Start/Pause button clicked for task %s", currentTask.ID)
		if tr.onStartPause != nil {
			tr.onStartPause(currentTask.ID)
		} else {
			log.Printf("onStartPause callback is nil for task %s", currentTask.ID)
		}
	})
	tr.startPauseBtn.Importance = widget.MediumImportance

	// open -> reveal in file manager (Finder/Explorer) and highlight file
	tr.openBtn = widget.NewButton("open", func() {
		// Get current task state dynamically - not from closure!
		currentTask := tr.task
		log.Printf("Open button clicked for task %s, OutputPath: %s", currentTask.ID, currentTask.OutputPath)

		if tr.onReveal == nil {
			log.Printf("onReveal callback is nil for task %s", currentTask.ID)
			return
		}

		if currentTask.OutputPath == "" {
			log.Printf("No output path available for task %s (status: %s)", currentTask.ID, currentTask.Status)
			// Show user-friendly message
			widget.ShowPopUp(widget.NewLabel("File path not available yet. Wait for download to complete."), fyne.CurrentApp().Driver().CanvasForObject(tr.openBtn))
			return
		}

		// Check if this is actually a file path, not a URL
		if strings.HasPrefix(currentTask.OutputPath, "http") {
			log.Printf("Cannot open URL as file: %s", currentTask.OutputPath)
			widget.ShowPopUp(widget.NewLabel("Cannot reveal URL as file. Wait for download to complete."), fyne.CurrentApp().Driver().CanvasForObject(tr.openBtn))
			return
		}

		// Additional validation: check if the path looks like a real file path
		if !strings.Contains(currentTask.OutputPath, "/") && !strings.Contains(currentTask.OutputPath, "\\") {
			log.Printf("OutputPath does not contain path separators: %s", currentTask.OutputPath)
			widget.ShowPopUp(widget.NewLabel("File path is incomplete. Wait for download to complete."), fyne.CurrentApp().Driver().CanvasForObject(tr.openBtn))
			return
		}

		tr.onReveal(currentTask.OutputPath)
	})
	tr.openBtn.Importance = widget.MediumImportance

	// play -> open with default app (player)
	tr.playBtn = widget.NewButton("play", func() {
		currentTask := tr.task
		if currentTask.OutputPath != "" && !strings.HasPrefix(currentTask.OutputPath, "http") &&
			(strings.Contains(currentTask.OutputPath, "/") || strings.Contains(currentTask.OutputPath, "\\")) {
			if tr.onOpen != nil {
				tr.onOpen(currentTask.OutputPath)
			}
		} else {
			widget.ShowPopUp(widget.NewLabel("File path not available"), fyne.CurrentApp().Driver().CanvasForObject(tr.startPauseBtn))
		}
	})
	tr.playBtn.Importance = widget.MediumImportance

	tr.copyBtn = widget.NewButton("path", func() {
		currentTask := tr.task
		if tr.onCopyPath != nil {
			if currentTask.OutputPath != "" && !strings.HasPrefix(currentTask.OutputPath, "http") &&
				(strings.Contains(currentTask.OutputPath, "/") || strings.Contains(currentTask.OutputPath, "\\")) {
				tr.onCopyPath(currentTask.OutputPath)
			} else {
				widget.ShowPopUp(widget.NewLabel("Copy path not available"), fyne.CurrentApp().Driver().CanvasForObject(tr.copyBtn))
			}
		} else {
			log.Printf("onCopyPath callback is nil for task %s", currentTask.ID)
		}
	})
	tr.copyBtn.Importance = widget.MediumImportance

}

// updateFromTask updates UI components based on task state
func (tr *TaskRow) updateFromTask() {
	if tr.task == nil {
		log.Printf("Warning: updateFromTask called with nil task")
		return
	}

	// Clean task data to prevent display issues
	if tr.task.URL != "" {
		tr.task.URL = strings.ReplaceAll(tr.task.URL, "\n", "")
		tr.task.URL = strings.ReplaceAll(tr.task.URL, "\r", "")
		tr.task.URL = strings.ReplaceAll(tr.task.URL, "\t", " ")
		tr.task.URL = strings.TrimSpace(tr.task.URL)
	}

	if tr.task.Title != "" {
		tr.task.Title = strings.ReplaceAll(tr.task.Title, "\n", " ")
		tr.task.Title = strings.ReplaceAll(tr.task.Title, "\r", " ")
		tr.task.Title = strings.ReplaceAll(tr.task.Title, "\t", " ")
		tr.task.Title = strings.TrimSpace(tr.task.Title)
	}

	log.Printf("TaskRow updateFromTask: id=%s status=%s percent=%d progress=%.2f output=%s",
		tr.task.ID, tr.task.Status, tr.task.Percent, tr.task.Progress, tr.task.OutputPath)

	// Update labels
	titleText := tr.task.GetDisplayTitle()

	// Keep title compact: no URL/ID/filename/size/extension/duration in title.

	// Do not append time to title to keep it clean

	// Do not append error to title; errors are reflected in status/tooltip

	// Do not append progress/speed/ETA/ID into title

	// Clean title text from any special characters that might cause display issues
	cleanTitleText := strings.ReplaceAll(titleText, "\n", " ")
	cleanTitleText = strings.ReplaceAll(cleanTitleText, "\r", " ")
	cleanTitleText = strings.ReplaceAll(cleanTitleText, "\t", " ")
	cleanTitleText = strings.TrimSpace(cleanTitleText)

	tr.titleLabel.SetText(cleanTitleText)

	// Update status label color and text
	switch tr.task.Status {
	case model.TaskStatusError:
		tr.statusLabel.Importance = widget.DangerImportance
		tr.statusLabel.SetText(IconError + " " + tr.task.Status.String())
	case model.TaskStatusCompleted:
		tr.statusLabel.Importance = widget.SuccessImportance
		tr.statusLabel.SetText(tr.task.Status.String())
	case model.TaskStatusDownloading:
		tr.statusLabel.Importance = widget.HighImportance
		tr.statusLabel.SetText(IconPlay + " " + tr.task.Status.String())
	case model.TaskStatusPaused:
		tr.statusLabel.Importance = widget.MediumImportance
		tr.statusLabel.SetText("⏸ " + tr.task.Status.String())
	case model.TaskStatusPending:
		tr.statusLabel.Importance = widget.MediumImportance
		tr.statusLabel.SetText("⏳ " + tr.task.Status.String())
	case model.TaskStatusStopped:
		tr.statusLabel.Importance = widget.MediumImportance
		tr.statusLabel.SetText("⏹ " + tr.task.Status.String())
	default:
		tr.statusLabel.Importance = widget.MediumImportance
		tr.statusLabel.SetText(tr.task.Status.String())
	}

	// Update progress with icon (robust fallback logic)
	effectivePercent := tr.task.Percent
	if tr.task.Status == model.TaskStatusCompleted {
		// Do not show redundant 100% label when completed; keep bar filled.
		effectivePercent = MaxProgressPercent
	} else if effectivePercent <= 0 && tr.task.Progress > 0 {
		effectivePercent = int(tr.task.Progress * MaxProgressPercent)
	}
	// Если прогресс дробный (например 0.69), переводим в проценты округлением вниз, но не позволяем оставаться 0 при progress>0
	if effectivePercent == 0 && tr.task.Progress > 0 {
		effectivePercent = int(tr.task.Progress*MaxProgressPercent + RoundingCoefficient)
		if effectivePercent == 0 {
			effectivePercent = MinProgressPercent
		}
	}
	if effectivePercent < 0 {
		effectivePercent = 0
	}
	if effectivePercent > MaxProgressPercent {
		effectivePercent = MaxProgressPercent
	}
	if tr.task.Status == model.TaskStatusCompleted {
		tr.progressLabel.SetText("")
	} else {
		tr.progressLabel.SetText(fmt.Sprintf("%d%%", effectivePercent))
	}
	log.Printf("TaskRow set progress label: id=%s percent=%d (status=%s)", tr.task.ID, effectivePercent, tr.task.Status)

	// Update speed and ETA
	speedEtaText := ""
	if tr.task.Status == model.TaskStatusDownloading {
		if tr.task.Speed != "" {
			speedEtaText = tr.task.Speed
		}
		if tr.task.ETASec > 0 {
			if speedEtaText != "" {
				speedEtaText += MiddleDotSeparator
			}
			speedEtaText += tr.task.GetETAString()
		}
		if speedEtaText == "" {
			speedEtaText = DashPlaceholder
		}
	} else if tr.task.Status == model.TaskStatusCompleted {
		speedEtaText = ""
	} else if tr.task.Status == model.TaskStatusError {
		speedEtaText = "Error"
	}
	tr.speedEtaLabel.SetText(speedEtaText)

	// No progress bar — color handling removed

	// Update button states and text
	tr.updateButtons()
}

// showMoreMenu removed: actions are separate buttons now

// updateButtons updates button states based on task status
func (tr *TaskRow) updateButtons() {
	if tr.task == nil {
		log.Printf("Warning: updateButtons called with nil task")
		return
	}

	log.Printf("Updating buttons for task %s: Status=%s, OutputPath=%s",
		tr.task.ID, tr.task.Status, tr.task.OutputPath)

	// Start/Pause button visibility, enabled state, and text by status
	switch tr.task.Status {
	case model.TaskStatusPending:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Enable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyPause))
	case model.TaskStatusStarting, model.TaskStatusDownloading:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Enable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyPause))
	case model.TaskStatusPaused:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Enable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyContinue))
	case model.TaskStatusStopped:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Enable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyPause))
	case model.TaskStatusError:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Enable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyPause))
	case model.TaskStatusCompleted:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Disable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyPause))
	default:
		tr.startPauseBtn.Show()
		tr.startPauseBtn.Enable()
		tr.startPauseBtn.SetText(tr.localization.GetText(KeyPause))
	}

	// Open (reveal) and Play buttons - only enable when OutputPath is a real file path
	tr.openBtn.Show()
	tr.playBtn.Show()

	// Check if OutputPath is valid and points to a real file
	if tr.task.OutputPath != "" && !strings.HasPrefix(tr.task.OutputPath, "http") {
		// Additional validation: check if the path looks like a real file path
		if strings.Contains(tr.task.OutputPath, "/") || strings.Contains(tr.task.OutputPath, "\\") {
			log.Printf("Enabling Open/Play buttons for task %s: OutputPath=%s", tr.task.ID, tr.task.OutputPath)
			tr.openBtn.Enable()
			tr.playBtn.Enable()
		} else {
			// Disable if it's just a filename without path
			log.Printf("Disabling Open/Play buttons for task %s: OutputPath=%s (no path separators)", tr.task.ID, tr.task.OutputPath)
			tr.openBtn.Disable()
			tr.playBtn.Disable()
		}
	} else {
		log.Printf("Disabling Open/Play buttons for task %s: OutputPath=%s (empty or URL)", tr.task.ID, tr.task.OutputPath)
		tr.openBtn.Disable()
		tr.playBtn.Disable()
	}

	// Copy Path button availability mirrors reveal/open
	if tr.task.OutputPath != "" && !strings.HasPrefix(tr.task.OutputPath, "http") &&
		(strings.Contains(tr.task.OutputPath, "/") || strings.Contains(tr.task.OutputPath, "\\")) {
		tr.copyBtn.Show()
		tr.copyBtn.Enable()
	} else {
		tr.copyBtn.Show()
		tr.copyBtn.Disable()
	}

	// No remove button anymore
}

// CreateRenderer creates the widget renderer
func (tr *TaskRow) CreateRenderer() fyne.WidgetRenderer {
	return &taskRowRenderer{taskRow: tr}
}

// taskRowRenderer renders the task row widget
type taskRowRenderer struct {
	taskRow *TaskRow
	layout  *fyne.Container
}

// Layout arranges the components
func (r *taskRowRenderer) Layout(size fyne.Size) {
	if r.layout == nil {
		r.createLayout()
	}
	if r.layout != nil {
		// Ensure minimum size to prevent layout issues
		if size.Width < RowMinWidth {
			size.Width = RowMinWidth
		}
		if size.Height < RowMinHeight {
			size.Height = RowMinHeight
		}
		r.layout.Resize(size)
	}
}

// MinSize returns the minimum size
func (r *taskRowRenderer) MinSize() fyne.Size {
	if r.layout != nil {
		return r.layout.MinSize()
	}
	// Ensure minimum size is reasonable to prevent layout issues
	return fyne.NewSize(RowMinWidth, RowMinHeight)
}

// Refresh refreshes the renderer
func (r *taskRowRenderer) Refresh() {
	if r.layout == nil {
		r.createLayout()
	}

	if r.taskRow.task != nil {
		// Also update textual progress label to reflect current percent
		effectivePercent := r.taskRow.task.Percent
		if r.taskRow.task.Status == model.TaskStatusCompleted {
			effectivePercent = MaxProgressPercent
		} else if effectivePercent <= 0 && r.taskRow.task.Progress > 0 {
			effectivePercent = int(r.taskRow.task.Progress * MaxProgressPercent)
		}
		if effectivePercent < 0 {
			effectivePercent = 0
		}
		if effectivePercent > MaxProgressPercent {
			effectivePercent = MaxProgressPercent
		}
		if effectivePercent == 0 && r.taskRow.task.Progress > 0 {
			effectivePercent = int(r.taskRow.task.Progress*MaxProgressPercent + RoundingCoefficient)
			if effectivePercent == 0 {
				effectivePercent = MinProgressPercent
			}
		}
		r.taskRow.progressLabel.SetText(fmt.Sprintf(ProgressLabelFormat, effectivePercent))
		log.Printf("TaskRowRenderer refresh: id=%s percent=%d progress=%.2f status=%s",
			r.taskRow.task.ID, effectivePercent, r.taskRow.task.Progress, r.taskRow.task.Status)
	}

	if r.layout != nil {
		r.layout.Refresh()
		// Force refresh of the entire layout to ensure proper display
		r.layout.Resize(r.layout.Size())
	}
}

// Objects returns the container objects
func (r *taskRowRenderer) Objects() []fyne.CanvasObject {
	if r.layout == nil {
		r.createLayout()
	}
	return []fyne.CanvasObject{r.layout}
}

// Destroy cleans up the renderer
func (r *taskRowRenderer) Destroy() {}

// createLayout creates the main layout
func (r *taskRowRenderer) createLayout() {
	tr := r.taskRow

	// Left side: file title (more prominent)
	leftSide := tr.titleLabel

	// Right side: vertical compact info aligned to the right with fixed widths
	// Helper to fix width using a transparent rectangle underneath
	fixedWidth := func(w float32, obj fyne.CanvasObject) fyne.CanvasObject {
		spacer := canvas.NewRectangle(color.RGBA{0, 0, 0, 0})
		spacer.SetMinSize(fyne.NewSize(w, obj.MinSize().Height))
		return container.NewStack(spacer, obj)
	}

	const statusWidth float32 = StatusLabelWidth
	const speedWidth float32 = SpeedLabelWidth
	const percentWidth float32 = PercentLabelWidth

	// Order: status (row1), speed then percent (row2)
	rightSide := container.NewVBox(
		fixedWidth(statusWidth, tr.statusLabel),
		container.NewHBox(
			fixedWidth(speedWidth, tr.speedEtaLabel),
			fixedWidth(percentWidth, tr.progressLabel),
		),
	)

	// Action buttons row - ensure buttons have enough space
	// Wrap into a container to ensure mouse events are captured correctly for tooltips
	actionRow := container.NewHBox(
		r.taskRow.startPauseBtn, // pause
		r.taskRow.openBtn,       // open (reveal)
		r.taskRow.playBtn,       // play (open with default app)
		r.taskRow.copyBtn,       // path (copy)
	)

	// Ensure buttons are properly sized and clickable
	// The buttons will get their natural size from the layout

	// No progress bar: we'll use only textual percent; keep a thin separator below
	separator := widget.NewSeparator()

	// Build layout so that action buttons are pinned to the right edge,
	// the compact info (status, percent, speed/ETA) stays near the buttons,
	// and the title occupies the remaining space on the left with wrapping.
	//
	// Build a right cluster with the info stacked left and the buttons pinned to the right.
	// Border layout here guarantees action buttons are flush to the row's right edge with no extra gap.
	rightCluster := container.NewBorder(nil, nil, nil, actionRow, rightSide)

	// Border with center expandable title and right cluster pinned.
	mainContent := container.NewBorder(nil, nil, nil, rightCluster, leftSide)

	r.layout = container.NewVBox(
		mainContent,
		separator,
	)

	// Ensure layout has reasonable sizing; give room for two text lines
	r.layout.Resize(fyne.NewSize(RowMinWidth, RowDefaultH))

	// Force refresh of the layout
	r.layout.Refresh()
}
