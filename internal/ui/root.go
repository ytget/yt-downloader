package ui

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"github.com/romanitalian/yt-downloader/internal/config"
	"github.com/romanitalian/yt-downloader/internal/download"
	"github.com/romanitalian/yt-downloader/internal/model"
	"github.com/romanitalian/yt-downloader/internal/platform"
)

// RootUI represents the main UI structure
type RootUI struct {
	window       fyne.Window
	urlEntry     *widget.Entry
	downloadBtn  *widget.Button
	taskList     *widget.List
	tasks        binding.UntypedList
	downloadSvc  *download.Service
	settings     *config.Settings
	localization *Localization
}

// NewRootUI creates and initializes the main UI
func NewRootUI(window fyne.Window, app fyne.App) *RootUI {
	// Initialize settings
	settings := config.NewSettings(app)

	// Initialize localization
	localization := NewLocalization()
	localization.SetLanguage(settings.GetLanguage())

	// Get configured downloads directory
	downloadsDir := settings.GetDownloadDirectory()

	// Ensure directory exists
	platform.CreateDirectoryIfNotExists(downloadsDir)

	ui := &RootUI{
		window:       window,
		tasks:        binding.NewUntypedList(),
		downloadSvc:  download.NewService(downloadsDir, settings.GetMaxParallelDownloads()),
		settings:     settings,
		localization: localization,
	}

	// Set window title
	window.SetTitle(localization.GetText(KeyAppTitle))

	// Set up callback for download updates
	ui.downloadSvc.SetUpdateCallback(ui.onTaskUpdate)

	ui.setupUI()
	return ui
}

// setupUI creates and arranges all UI components
func (ui *RootUI) setupUI() {
	// Create menu
	ui.createMenu()

	// Create URL entry
	ui.urlEntry = widget.NewEntry()
	ui.urlEntry.SetPlaceHolder(ui.localization.GetText(KeyEnterURL))
	ui.urlEntry.Validator = ui.validateURL

	// Create download button
	ui.downloadBtn = widget.NewButton(ui.localization.GetText(KeyDownload), ui.onDownloadClick)

	// Create top panel
	topPanel := container.NewBorder(nil, nil, nil, ui.downloadBtn, ui.urlEntry)

	// Create task list
	ui.taskList = widget.NewList(
		func() int {
			return ui.tasks.Length()
		},
		func() fyne.CanvasObject { return ui.createTaskItem() },
		func(id widget.ListItemID, obj fyne.CanvasObject) { ui.updateTaskItem(id, obj) },
	)

	// Create main layout
	content := container.NewBorder(
		topPanel,    // top
		nil,         // bottom
		nil,         // left
		nil,         // right
		ui.taskList, // center
	)

	ui.window.SetContent(content)

	// Add some sample tasks for testing
	ui.addSampleTasks()
}

// createMenu creates the application menu
func (ui *RootUI) createMenu() {
	// Settings menu item
	settingsItem := fyne.NewMenuItem(ui.localization.GetText(KeySettings), ui.onShowSettings)

	// Language submenu
	languageMenu := fyne.NewMenu(ui.localization.GetText(KeyLanguage))

	availableLanguages := ui.localization.GetAvailableLanguages()
	for code, name := range availableLanguages {
		langCode := code // Capture for closure
		langItem := fyne.NewMenuItem(name, func() {
			ui.onLanguageChange(langCode)
		})

		// Mark current language
		if ui.localization.GetCurrentLanguage() == code {
			langItem.Checked = true
		}

		languageMenu.Items = append(languageMenu.Items, langItem)
	}

	// Create main menu
	mainMenu := fyne.NewMainMenu(
		fyne.NewMenu(ui.localization.GetText(KeyFile), settingsItem),
		languageMenu,
	)

	ui.window.SetMainMenu(mainMenu)
}

// onLanguageChange handles language change
func (ui *RootUI) onLanguageChange(langCode string) {
	// Update localization
	ui.localization.SetLanguage(langCode)

	// Save to settings
	ui.settings.SetLanguage(langCode)

	// Update UI texts
	ui.refreshUITexts()

	// Recreate menu to update checkmarks
	ui.createMenu()
}

// refreshUITexts updates all UI texts with current language
func (ui *RootUI) refreshUITexts() {
	// Update window title
	ui.window.SetTitle(ui.localization.GetText(KeyAppTitle))

	// Update UI elements
	ui.urlEntry.SetPlaceHolder(ui.localization.GetText(KeyEnterURL))
	ui.downloadBtn.SetText(ui.localization.GetText(KeyDownload))

	// Refresh task list to update button texts
	ui.taskList.Refresh()
}

// onShowSettings shows the settings dialog
func (ui *RootUI) onShowSettings() {
	settingsDialog := NewSettingsDialog(ui.settings, ui.window)
	settingsDialog.Show()
}

// validateURL validates the entered URL
func (ui *RootUI) validateURL(input string) error {
	if strings.TrimSpace(input) == "" {
		return nil // Empty is allowed
	}

	parsedURL, err := url.Parse(input)
	if err != nil {
		return err
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must start with http:// or https://")
	}

	return nil
}

// onDownloadClick handles the download button click
func (ui *RootUI) onDownloadClick() {
	urlText := strings.TrimSpace(ui.urlEntry.Text)
	if urlText == "" {
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyPleaseEnterURL)), ui.window.Canvas())
		return
	}

	if err := ui.validateURL(urlText); err != nil {
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyInvalidURL)+": "+err.Error()), ui.window.Canvas())
		return
	}

	// Add task to download service
	task, err := ui.downloadSvc.AddTask(urlText)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyAlreadyInQueue)), ui.window.Canvas())
		} else {
			widget.ShowPopUp(widget.NewLabel("Error: "+err.Error()), ui.window.Canvas())
		}
		return
	}

	// Add to UI task list
	ui.tasks.Append(task)
	ui.urlEntry.SetText("")

	widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyDownloadStarted)), ui.window.Canvas())
}

// createTaskItem creates a new task item widget
func (ui *RootUI) createTaskItem() fyne.CanvasObject {
	titleLabel := widget.NewLabel("Title")
	statusLabel := widget.NewLabel("Status")
	progressBar := widget.NewProgressBar()
	progressLabel := widget.NewLabel("0%")
	etaLabel := widget.NewLabel("—")
	stopBtn := widget.NewButton(ui.localization.GetText(KeyStop), func() {
		// Will be set in updateTaskItem
	})
	stopBtn.Hide() // Hidden by default

	openBtn := widget.NewButton(ui.localization.GetText(KeyOpen), func() {
		// Will be set in updateTaskItem
	})
	openBtn.Hide() // Hidden by default

	// Arrange widgets
	topRow := container.NewHBox(titleLabel, widget.NewSeparator(), statusLabel, stopBtn, openBtn)
	progressRow := container.NewBorder(nil, nil, progressLabel, etaLabel, progressBar)

	return container.NewVBox(topRow, progressRow)
}

// updateTaskItem updates a task item with current data
func (ui *RootUI) updateTaskItem(id widget.ListItemID, item fyne.CanvasObject) {
	taskData, err := ui.tasks.GetValue(id)
	if err != nil {
		return
	}

	task, ok := taskData.(*model.DownloadTask)
	if !ok {
		return
	}

	// Get container and its children
	vbox := item.(*fyne.Container)
	topRow := vbox.Objects[0].(*fyne.Container)
	progressRow := vbox.Objects[1].(*fyne.Container)

	// Update labels and controls
	titleLabel := topRow.Objects[0].(*widget.Label)
	statusLabel := topRow.Objects[2].(*widget.Label)
	stopBtn := topRow.Objects[3].(*widget.Button)
	openBtn := topRow.Objects[4].(*widget.Button)

	// For border container: left (progressLabel), center (progressBar), right (etaLabel)
	var progressLabel *widget.Label
	var progressBar *widget.ProgressBar
	var etaLabel *widget.Label

	// Find widgets by type in progressRow
	for _, obj := range progressRow.Objects {
		switch v := obj.(type) {
		case *widget.Label:
			if progressLabel == nil {
				progressLabel = v // First label is progress
			} else {
				etaLabel = v // Second label is ETA
			}
		case *widget.ProgressBar:
			progressBar = v
		}
	}

	// Update UI elements - these should be safe in list callback, but wrap just in case
	titleLabel.SetText(task.GetDisplayTitle())
	statusLabel.SetText(task.Status.String())
	progressLabel.SetText(fmt.Sprintf("%d%%", task.Percent))
	etaLabel.SetText(task.GetETAString())
	progressBar.SetValue(task.Progress)

	// Configure stop button
	if task.Status.IsActive() {
		stopBtn.Show()
		stopBtn.OnTapped = func() {
			ui.onStopTask(task.ID)
		}
		if task.Status == model.TaskStatusStopping {
			stopBtn.Disable()
		} else {
			stopBtn.Enable()
		}
	} else {
		stopBtn.Hide()
	}

	// Configure open button
	if task.Status == model.TaskStatusCompleted && task.OutputPath != "" {
		openBtn.Show()
		openBtn.OnTapped = func() {
			ui.onOpenFile(task.OutputPath)
		}
	} else {
		openBtn.Hide()
	}

	// Color coding for status
	switch task.Status {
	case model.TaskStatusError:
		statusLabel.Importance = widget.DangerImportance
	case model.TaskStatusCompleted:
		statusLabel.Importance = widget.SuccessImportance
	default:
		statusLabel.Importance = widget.MediumImportance
	}
}

// onStopTask handles stopping a download task
func (ui *RootUI) onStopTask(taskID string) {
	err := ui.downloadSvc.StopTask(taskID)
	if err != nil {
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyErrorStoppingTask)+": "+err.Error()), ui.window.Canvas())
		return
	}

	widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyStoppingDownload)), ui.window.Canvas())
}

// onOpenFile handles opening a downloaded file in the system file manager
func (ui *RootUI) onOpenFile(filePath string) {
	err := platform.OpenFileInManager(filePath)
	if err != nil {
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyErrorOpeningFile)+": "+err.Error()), ui.window.Canvas())
		return
	}
}

// onTaskUpdate handles task updates from the download service
func (ui *RootUI) onTaskUpdate(task *model.DownloadTask) {
	// Check if task just completed for notification
	wasCompleted := false

	// Update task in the list
	// Find and update the task in our binding list
	length := ui.tasks.Length()
	for i := 0; i < length; i++ {
		item, err := ui.tasks.GetValue(i)
		if err != nil {
			continue
		}

		if existingTask, ok := item.(*model.DownloadTask); ok && existingTask.ID == task.ID {
			// Check if status changed to completed
			if existingTask.Status != model.TaskStatusCompleted && task.Status == model.TaskStatusCompleted {
				wasCompleted = true
			}
			ui.tasks.SetValue(i, task)
			break
		}
	}

	// Send notification for completed downloads
	if wasCompleted {
		ui.sendCompletionNotification(task)
	}

	// Refresh the list to update UI - must be done in UI thread
	fyne.Do(func() {
		ui.taskList.Refresh()
	})
}

// sendCompletionNotification sends a system notification for completed downloads
func (ui *RootUI) sendCompletionNotification(task *model.DownloadTask) {
	if task.Status == model.TaskStatusCompleted {
		title := ui.localization.GetText(KeyDownloadCompleted)
		message := task.GetDisplayTitle()

		// Use Fyne's SendNotification
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   title,
			Content: message,
		})
	}
}

// addSampleTasks adds some sample tasks for testing
func (ui *RootUI) addSampleTasks() {
	sampleTasks := []*model.DownloadTask{
		{
			ID:       "sample-1",
			URL:      "https://youtube.com/watch?v=example1",
			Title:    "Sample Video 1",
			Status:   model.TaskStatusCompleted,
			Progress: 1.0,
			Percent:  100,
		},
		{
			ID:       "sample-2",
			URL:      "https://youtube.com/watch?v=example2",
			Title:    "Sample Video 2 (Downloading)",
			Status:   model.TaskStatusDownloading,
			Progress: 0.45,
			Percent:  45,
			Speed:    "1.2MB/s",
			ETASec:   123,
		},
		{
			ID:        "sample-3",
			URL:       "https://youtube.com/watch?v=example3",
			Title:     "Sample Video 3 (Error)",
			Status:    model.TaskStatusError,
			LastError: "Network timeout",
		},
	}

	for _, task := range sampleTasks {
		ui.tasks.Append(task)
	}
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	// Simple ID generation for now
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
