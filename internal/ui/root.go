package ui

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"

	"github.com/ytget/yt-downloader/internal/compress"
	"github.com/ytget/yt-downloader/internal/config"
	"github.com/ytget/yt-downloader/internal/download"
	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/yt-downloader/internal/platform"
)

// UI constants
const (
	RootPlaylistQueryParam = "list="
	RootUIUpdateDebounce   = 100 * time.Millisecond
)

// Toast notification constants
const (
	RootToastWidth    = 300
	RootToastHeight   = 120
	RootToastMargin   = 20
	RootToastAutoHide = 5 * time.Second
)

// Playlist processing constants
const (
	RootPlaylistParseDelay = 500 * time.Millisecond
)

// StatusFilter represents different task status filters
// StatusFilter enumerates visible subsets of tasks in the UI.
// String() returns human-friendly names for tabs.
type StatusFilter int

const (
	FilterAll StatusFilter = iota
	FilterDownloading
	FilterPending
	FilterCompleted
	FilterErrors
)

// StatusFilterName returns the display name for a status filter
// String returns a localized-like English label for the filter tab.
func (sf StatusFilter) String() string {
	switch sf {
	case FilterAll:
		return "All"
	case FilterDownloading:
		return "Downloading"
	case FilterPending:
		return "Pending"
	case FilterCompleted:
		return "Completed"
	case FilterErrors:
		return "Errors"
	default:
		return "Unknown"
	}
}

// RootUI represents the main UI structure
type RootUI struct {
	window        fyne.Window
	urlEntry      *widget.Entry
	downloadBtn   *widget.Button
	taskList      *widget.List
	currentFilter StatusFilter
	tasks         binding.UntypedList
	filteredTasks []*model.DownloadTask
	downloadSvc   download.Downloader
	compressSvc   compress.Compressor
	settings      *config.Settings
	localization  *Localization

	// Playlist support
	playlistGroup *PlaylistGroup
	parserService *platform.YTDLPParserService

	// UI update debouncing
	lastUIUpdate  time.Time
	uiUpdateMutex sync.Mutex

	// Notification panel
	notificationContainer *fyne.Container
	notificationLabel     *widget.Label
	notificationSpinner   *widget.ProgressBarInfinite

	// Mobile UI enhancements
	mobileUI *MobileUI

	// Quick access buttons for mobile
	settingsBtn   *widget.Button
	languageBtn   *widget.Button
	languagePopup *widget.PopUp
	titleLabel    *widget.Label

	// Dialog for no app found scenario
	noAppDialog *widget.PopUp
}

// NewRootUI creates and initializes the main UI
func NewRootUI(window fyne.Window, app fyne.App, downloadSvc download.Downloader, compressSvc compress.Compressor) *RootUI {
	// Initialize settings
	settings := config.NewSettings(app)

	// Initialize localization with default language
	localization := NewLocalization()
	localization.SetLanguage("en") // Default to English, will be updated when settings are read

	ui := &RootUI{
		window:       window,
		tasks:        binding.NewUntypedList(),
		downloadSvc:  downloadSvc,
		compressSvc:  compressSvc,
		settings:     settings,
		localization: localization,

		// Initialize playlist services
		parserService: platform.NewYTDLPParserService(),

		// Initialize mobile UI
		mobileUI: NewMobileUI(app),
	}

	// Verify that all callbacks are properly initialized
	log.Printf("RootUI initialized with download service: %v", ui.downloadSvc != nil)

	// Set window title with version timestamp
	version := time.Now().Format("2006-01-02 15:04:05")
	window.SetTitle(fmt.Sprintf("%s v%s", localization.GetText(KeyAppTitle), version))

	// Set up callback for download updates
	ui.downloadSvc.SetUpdateCallback(ui.onTaskUpdate)

	ui.setupUI()
	return ui
}

// setupUI creates and arranges all UI components
func (ui *RootUI) setupUI() {
	// Create menu
	ui.createMenu()

	// Create URL entry with mobile optimizations
	ui.urlEntry = ui.mobileUI.CreateMobileEntry(ui.localization.GetText(KeyEnterURL))
	ui.urlEntry.Validator = ui.validateURL
	// Trigger download when user presses Enter in the URL field
	ui.urlEntry.OnSubmitted = func(string) {
		ui.onDownloadClick()
	}

	// Create download button with mobile optimizations
	ui.downloadBtn = ui.mobileUI.CreateMobileButton(ui.localization.GetText(KeyDownload), ui.onDownloadClick)

	// Create settings button with mobile optimizations
	ui.settingsBtn = ui.mobileUI.CreateMobileButton(IconSettings, ui.onShowSettings)
	ui.settingsBtn.Importance = widget.LowImportance

	// Create language button for mobile
	ui.languageBtn = ui.mobileUI.CreateMobileButton(IconLanguage, ui.onLanguageButtonClick)
	ui.languageBtn.Importance = widget.LowImportance

	// Create title label for mobile
	ui.titleLabel = widget.NewLabel(ui.localization.GetText(KeyAppTitle))
	ui.titleLabel.TextStyle = fyne.TextStyle{Bold: true}
	ui.titleLabel.Alignment = fyne.TextAlignCenter

	// Create logo
	var logoImage *canvas.Image
	if LogoResource != nil {
		logoImage = canvas.NewImageFromResource(LogoResource)
		logoImage.SetMinSize(fyne.NewSize(32, 32))
		logoImage.FillMode = canvas.ImageFillContain
	} else {
		// Fallback: try to load from file
		logo, err := LoadLogoResource()
		if err == nil {
			logoImage = canvas.NewImageFromResource(logo)
			logoImage.SetMinSize(fyne.NewSize(32, 32))
			logoImage.FillMode = canvas.ImageFillContain
		} else {
			// Fallback to text if logo loading fails
			logoImage = nil
		}
	}

	// Create top panel (URL row) with logo
	var topPanel *fyne.Container
	if ui.mobileUI.IsMobileDevice() {
		// For mobile, only show logo (if available) and download button
		if logoImage != nil {
			topPanel = container.NewBorder(nil, nil, logoImage, ui.downloadBtn, ui.urlEntry)
		} else {
			topPanel = container.NewBorder(nil, nil, nil, ui.downloadBtn, ui.urlEntry)
		}
	} else {
		// For desktop, show logo and settings button
		if logoImage != nil {
			topPanel = container.NewBorder(nil, nil, container.NewHBox(logoImage, ui.settingsBtn), ui.downloadBtn, ui.urlEntry)
		} else {
			topPanel = container.NewBorder(nil, nil, container.NewHBox(ui.settingsBtn), ui.downloadBtn, ui.urlEntry)
		}
	}

	// Create icon panel above URL input for mobile
	var iconPanel *fyne.Container
	if ui.mobileUI.IsMobileDevice() {
		iconPanel = ui.createMobileIconPanel()
	}

	// Create notification panel under URL input (hidden by default)
	ui.notificationLabel = widget.NewLabel("")
	ui.notificationLabel.Alignment = fyne.TextAlignLeading
	ui.notificationSpinner = widget.NewProgressBarInfinite()
	ui.notificationSpinner.Hide()
	ui.notificationContainer = container.NewHBox(ui.notificationSpinner, container.NewPadded(ui.notificationLabel))
	ui.notificationContainer.Hide()

	// Combine URL row and notification panel at the top
	// Add top spacer for mobile devices to avoid system status bar overlap
	var topCombined *fyne.Container
	if ui.mobileUI.IsMobileDevice() {
		// Add empty spacer at the top for mobile to avoid status bar overlap
		topSpacer := widget.NewLabel("")
		topSpacer.Resize(fyne.NewSize(0, 20)) // 20px top margin

		// Combine spacer, icon panel, URL panel, and notification panel
		var components []fyne.CanvasObject
		components = append(components, topSpacer)
		if iconPanel != nil {
			components = append(components, iconPanel)
		}
		components = append(components, topPanel, ui.notificationContainer)
		topCombined = container.NewVBox(components...)
	} else {
		topCombined = container.NewVBox(topPanel, ui.notificationContainer)
	}

	// Create task list (kept for individual video downloads)
	ui.taskList = widget.NewList(
		func() int {
			return len(ui.filteredTasks)
		},
		func() fyne.CanvasObject { return ui.createTaskItem() },
		func(id widget.ListItemID, obj fyne.CanvasObject) { ui.updateFilteredTaskItem(id, obj) },
	)

	// Initialize with all tasks
	ui.currentFilter = FilterAll

	// Create playlist group
	ui.playlistGroup = NewPlaylistGroup(ui.window, ui.localization)

	// Set playlist callbacks
	ui.playlistGroup.SetCallbacks(
		ui.onPlaylistDownload,
		ui.onPlaylistCancel,
	)

	// Set TaskRow callbacks for playlist videos
	ui.playlistGroup.SetTaskRowCallbacks(
		ui.onStartPauseTask,
		ui.onRevealFile,
		ui.onOpenFile,
		ui.onCopyPath,
		ui.onRemoveTask,
	)

	// Create main layout with simple list for mobile
	var content fyne.CanvasObject
	if ui.mobileUI.IsMobileDevice() {
		// Use simple task list for mobile (no complex containers)
		content = container.NewBorder(
			topCombined, // top
			nil,         // bottom
			nil,         // left
			nil,         // right
			ui.taskList, // center - simple list
		)
	} else {
		// Use standard layout for desktop
		content = container.NewBorder(
			topCombined,                  // top
			nil,                          // bottom
			nil,                          // left
			nil,                          // right
			ui.playlistGroup.Container(), // center - unified playlist view
		)
	}

	ui.window.SetContent(content)

	// UI setup completed
	log.Printf("UI setup completed successfully")
}

// createMenu creates the application menu
func (ui *RootUI) createMenu() {
	// For mobile devices, we use icon buttons instead of menu
	if ui.mobileUI.IsMobileDevice() {
		// No menu for mobile - use icon buttons instead
		return
	}

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
	// Update window title with version timestamp
	version := time.Now().Format("2006-01-02 15:04:05")
	ui.window.SetTitle(fmt.Sprintf("%s v%s", ui.localization.GetText(KeyAppTitle), version))

	// Update UI elements
	ui.urlEntry.SetPlaceHolder(ui.localization.GetText(KeyEnterURL))
	ui.downloadBtn.SetText(ui.localization.GetText(KeyDownload))

	// Update mobile icon buttons and title
	if ui.mobileUI.IsMobileDevice() {
		ui.settingsBtn.SetText(IconSettings)
		ui.languageBtn.SetText(IconLanguage)
		ui.titleLabel.SetText(ui.localization.GetText(KeyAppTitle))
	}

	// Refresh task list to update button texts
	ui.taskList.Refresh()
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
	// Read settings before processing download
	ui.readAndApplySettings()

	urlText := strings.TrimSpace(ui.urlEntry.Text)
	if urlText == "" {
		// Also reflect in notification panel
		ui.showNotification(ui.localization.GetText(KeyPleaseEnterURL), false)
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyPleaseEnterURL)), ui.window.Canvas())
		return
	}

	if err := ui.validateURL(urlText); err != nil {
		// Also reflect in notification panel
		ui.showNotification(ui.localization.GetText(KeyInvalidURL)+": "+err.Error(), false)
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyInvalidURL)+": "+err.Error()), ui.window.Canvas())
		return
	}

	// Clean URL from any special characters that might cause display issues
	cleanURL := strings.ReplaceAll(urlText, "\n", "")
	cleanURL = strings.ReplaceAll(cleanURL, "\r", "")
	cleanURL = strings.ReplaceAll(cleanURL, "\t", " ")
	cleanURL = strings.TrimSpace(cleanURL)

	log.Printf("Processing URL: %s", cleanURL)

	// Check if this is a playlist URL
	if ui.isPlaylistURL(cleanURL) {
		log.Printf("Detected playlist URL, processing as playlist")
		ui.handlePlaylistURL(cleanURL)
		return
	}

	// Regular video download
	log.Printf("Adding download task for video URL: %s", cleanURL)

	// Add task to download service
	task, err := ui.downloadSvc.AddTask(cleanURL)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyAlreadyInQueue)), ui.window.Canvas())
		} else {
			widget.ShowPopUp(widget.NewLabel("Error: "+err.Error()), ui.window.Canvas())
		}
		return
	}

	log.Printf("Task added successfully: ID=%s, Status=%s, OutputPath=%s",
		task.ID, task.Status, task.OutputPath)

	// Add to UI task list
	_ = ui.tasks.Append(task)

	// Add to unified video list in PlaylistGroup
	ui.playlistGroup.AddIndividualVideo(task)

	// Update filtered tasks and refresh UI
	ui.updateFilteredTasks()
	ui.taskList.Refresh()

	// Single refresh of the entire UI to ensure proper display
	ui.window.Canvas().Refresh(ui.window.Content())

	ui.urlEntry.SetText("")

	widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyDownloadStarted)), ui.window.Canvas())
}

// showNotification displays a message in the notification panel under the URL input.
// When spinning is true, a spinner is shown to indicate background activity.
func (ui *RootUI) showNotification(message string, spinning bool) {
	if ui.notificationLabel == nil || ui.notificationContainer == nil || ui.notificationSpinner == nil {
		return
	}
	fyne.Do(func() {
		ui.notificationLabel.SetText(message)
		if spinning {
			ui.notificationSpinner.Show()
		} else {
			ui.notificationSpinner.Hide()
		}
		ui.notificationContainer.Show()
		ui.notificationContainer.Refresh()
	})
}

// hideNotification hides the notification panel.
//
//lint:ignore U1000 kept for future UI interactions (hide toast panel)
func (ui *RootUI) hideNotification() {
	if ui.notificationContainer == nil || ui.notificationSpinner == nil {
		return
	}
	fyne.Do(func() {
		ui.notificationSpinner.Hide()
		ui.notificationContainer.Hide()
	})
}

// onShowSettings shows the settings dialog
func (ui *RootUI) onShowSettings() {
	ShowSettingsDialog(ui.window, ui.settings, ui.localization, func() {
		// Settings changed callback - could restart download service if needed
		widget.ShowPopUp(widget.NewLabel("Settings saved"), ui.window.Canvas())
	})
}

// createTaskItem creates a new task item widget
func (ui *RootUI) createTaskItem() fyne.CanvasObject {
	// Create placeholder task row - will be updated in updateTaskItem
	dummyTask := &model.DownloadTask{
		ID:     "placeholder",
		Status: model.TaskStatusPending,
		Title:  "Loading...",
	}

	taskRow := NewTaskRow(dummyTask, ui.localization)

	// Set callbacks - these are initialized in the constructor

	taskRow.SetCallbacks(
		ui.onStartPauseTask,
		ui.onRevealFile,
		ui.onOpenFile,
		ui.onCopyPath,
		ui.onRemoveTask,
	)

	return taskRow
}

// updateFilteredTaskItem updates a filtered task item with current data
func (ui *RootUI) updateFilteredTaskItem(id widget.ListItemID, item fyne.CanvasObject) {
	if id >= len(ui.filteredTasks) {
		return
	}

	task := ui.filteredTasks[id]
	if task == nil {
		return
	}

	// Cast to TaskRow and update
	if taskRow, ok := item.(*TaskRow); ok {
		// IMPORTANT: Re-set callbacks every time we update the task
		// This ensures callbacks are properly connected to real tasks

		// Set callbacks - these are initialized in the constructor

		taskRow.SetCallbacks(
			ui.onStartPauseTask,
			ui.onRevealFile,
			ui.onOpenFile,
			ui.onCopyPath,
			ui.onRemoveTask,
		)

		// Update the task data
		taskRow.UpdateTask(task)

		log.Printf("Updated TaskRow for task %s with callbacks, OutputPath: %s, Status: %s",
			task.ID, task.OutputPath, task.Status)

		// Force refresh of the task row to ensure proper display
		taskRow.Refresh()
	}
}

// onFilterChanged handles filter changes from status tabs
//
//lint:ignore U1000 reserved for future tabbed filters
func (ui *RootUI) onFilterChanged(filter StatusFilter) {
	ui.currentFilter = filter
	ui.updateFilteredTasks()
	ui.taskList.Refresh()
}

// updateFilteredTasks updates the filtered tasks list based on current filter
func (ui *RootUI) updateFilteredTasks() {
	ui.filteredTasks = nil

	// Get all tasks from binding
	allTasks := ui.getAllTasks()

	// Filter tasks based on current status filter
	for _, task := range allTasks {
		if ui.shouldShowTask(task) {
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

			ui.filteredTasks = append(ui.filteredTasks, task)
		}
	}
}

// shouldShowTask returns whether a task should be shown based on current filter
func (ui *RootUI) shouldShowTask(task *model.DownloadTask) bool {
	switch ui.currentFilter {
	case FilterAll:
		return true
	case FilterDownloading:
		return task.Status == model.TaskStatusDownloading || task.Status == model.TaskStatusStarting
	case FilterPending:
		return task.Status == model.TaskStatusPending
	case FilterCompleted:
		return task.Status == model.TaskStatusCompleted
	case FilterErrors:
		return task.Status == model.TaskStatusError
	default:
		return true
	}
}

// getAllTasks converts binding list to task slice
func (ui *RootUI) getAllTasks() []*model.DownloadTask {
	var tasks []*model.DownloadTask

	length := ui.tasks.Length()
	for i := 0; i < length; i++ {
		item, err := ui.tasks.GetValue(i)
		if err != nil {
			continue
		}

		if task, ok := item.(*model.DownloadTask); ok {
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

			tasks = append(tasks, task)
		}
	}

	return tasks
}

// onStartPauseTask handles start/pause button click
func (ui *RootUI) onStartPauseTask(taskID string) {
	log.Printf("onStartPauseTask called for task %s", taskID)

	task, ok := ui.downloadSvc.GetTask(taskID)
	if !ok {
		// Fallback to lookup by YouTube video ID for playlist rows
		if t2, ok := ui.downloadSvc.GetTaskByVideoID(taskID); ok {
			log.Printf("Mapped videoID %s to internal task %s", taskID, t2.ID)
			task = t2
			taskID = t2.ID
		} else {
			log.Printf("Task %s not found", taskID)
			widget.ShowPopUp(widget.NewLabel("Task not found"), ui.window.Canvas())
			return
		}
	}

	log.Printf("Task %s status: %s, OutputPath: %s", taskID, task.Status, task.OutputPath)

	switch task.Status {
	case model.TaskStatusPending, model.TaskStatusError, model.TaskStatusStopped:
		// Start/Restart the task
		log.Printf("Starting task %s", taskID)
		err := ui.downloadSvc.RestartTask(taskID)
		if err != nil {
			log.Printf("Error starting task %s: %v", taskID, err)
			widget.ShowPopUp(widget.NewLabel("Error starting task: "+err.Error()), ui.window.Canvas())
		}
	case model.TaskStatusPaused:
		// Resume the paused task
		log.Printf("Resuming task %s", taskID)
		err := ui.downloadSvc.ResumeTask(taskID)
		if err != nil {
			log.Printf("Error resuming task %s: %v", taskID, err)
			widget.ShowPopUp(widget.NewLabel("Error resuming task: "+err.Error()), ui.window.Canvas())
		}
	case model.TaskStatusDownloading, model.TaskStatusStarting:
		// Pause the task
		log.Printf("Pausing task %s", taskID)
		err := ui.downloadSvc.PauseTask(taskID)
		if err != nil {
			log.Printf("Error pausing task %s: %v", taskID, err)
			widget.ShowPopUp(widget.NewLabel("Error pausing task: "+err.Error()), ui.window.Canvas())
		}
		// No manual status change; wait for service update
	default:
		log.Printf("Cannot start/pause task %s in status: %s", taskID, task.Status)
		widget.ShowPopUp(widget.NewLabel("Cannot start/pause task in current state"), ui.window.Canvas())
	}
}

// onRevealFile handles revealing a file in the system file manager
func (ui *RootUI) onRevealFile(filePath string) {
	log.Printf("onRevealFile called for path: %s", filePath)

	if filePath == "" {
		log.Printf("Error: onRevealFile called with empty filePath")
		widget.ShowPopUp(widget.NewLabel("Error: No file path provided"), ui.window.Canvas())
		return
	}

	// Check if this is actually a file path, not a URL
	if strings.HasPrefix(filePath, "http") {
		log.Printf("Error: onRevealFile called with URL instead of file path: %s", filePath)
		widget.ShowPopUp(widget.NewLabel("Error: Cannot reveal URL as file"), ui.window.Canvas())
		return
	}

	err := platform.OpenFileInManager(filePath)
	if err != nil {
		log.Printf("Error revealing file %s: %v", filePath, err)
		widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyErrorOpeningFile)+": "+err.Error()), ui.window.Canvas())
		return
	}

	log.Printf("File revealed successfully: %s", filePath)
}

// onOpenFile handles opening a downloaded file with the default application
func (ui *RootUI) onOpenFile(filePath string) {
	log.Printf("onOpenFile called for path: %s", filePath)

	if filePath == "" {
		log.Printf("Error: onOpenFile called with empty filePath")
		widget.ShowPopUp(widget.NewLabel("Error: No file path provided"), ui.window.Canvas())
		return
	}

	// Check if this is actually a file path, not a URL
	if strings.HasPrefix(filePath, "http") {
		log.Printf("Error: onOpenFile called with URL instead of file path: %s", filePath)
		widget.ShowPopUp(widget.NewLabel("Error: Cannot open URL as file"), ui.window.Canvas())
		return
	}

	err := platform.OpenFileWithDefaultApp(filePath)
	if err != nil {
		log.Printf("Error opening file %s: %v", filePath, err)

		// Check if it's a "no suitable app found" error
		if strings.Contains(err.Error(), "no suitable app found") {
			ui.showNoAppFoundDialog(filePath)
		} else {
			widget.ShowPopUp(widget.NewLabel(ui.localization.GetText(KeyErrorOpeningFile)+": "+err.Error()), ui.window.Canvas())
		}
		return
	}

	log.Printf("File opened successfully: %s", filePath)
}

// showNoAppFoundDialog shows a dialog with alternative actions when no suitable app is found
func (ui *RootUI) showNoAppFoundDialog(filePath string) {
	log.Printf("Showing no app found dialog for: %s", filePath)

	// Create dialog content
	titleLabel := widget.NewLabel("No Media Player Found")
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	messageLabel := widget.NewLabel("No media player found on this device.\nYou can view the file path or copy it to open manually.")
	messageLabel.Wrapping = fyne.TextWrapWord

	// Create action buttons
	openInFilesBtn := widget.NewButton("📁 Show File Path", func() {
		log.Printf("User chose to show file path: %s", filePath)
		// Show file path in a popup instead of trying to open file manager
		pathLabel := widget.NewLabel(filePath)
		pathLabel.Wrapping = fyne.TextWrapWord
		pathLabel.TextStyle = fyne.TextStyle{Monospace: true}

		copyBtn := widget.NewButton("📋 Copy Path", func() {
			ui.onCopyPath(filePath)
		})
		copyBtn.Importance = widget.HighImportance

		closeBtn := widget.NewButton("❌ Close", func() {
			// Close the popup
		})

		content := container.NewVBox(
			widget.NewLabel("File location:"),
			pathLabel,
			container.NewHBox(copyBtn, closeBtn),
		)

		popup := widget.NewModalPopUp(content, ui.window.Canvas())
		popup.Resize(fyne.NewSize(400, 200))
		popup.Show()

		// Close the main dialog
		if ui.noAppDialog != nil {
			ui.noAppDialog.Hide()
		}
	})
	openInFilesBtn.Importance = widget.HighImportance

	copyPathBtn := widget.NewButton("📋 Copy Path", func() {
		log.Printf("User chose to copy path: %s", filePath)
		ui.onCopyPath(filePath)
		if ui.noAppDialog != nil {
			ui.noAppDialog.Hide()
		}
	})
	copyPathBtn.Importance = widget.MediumImportance

	cancelBtn := widget.NewButton("❌ Cancel", func() {
		log.Printf("User cancelled no app dialog")
		if ui.noAppDialog != nil {
			ui.noAppDialog.Hide()
		}
	})
	cancelBtn.Importance = widget.LowImportance

	// Layout the dialog content
	header := container.NewBorder(nil, nil, titleLabel, nil)
	actions := container.NewHBox(openInFilesBtn, copyPathBtn, cancelBtn)
	content := container.NewVBox(
		header,
		messageLabel,
		widget.NewSeparator(),
		actions,
	)

	// Create and show the dialog
	ui.noAppDialog = widget.NewModalPopUp(content, ui.window.Canvas())
	ui.noAppDialog.Resize(fyne.NewSize(350, 200))
	ui.noAppDialog.Show()
}

// onCopyPath handles copying file path to clipboard
func (ui *RootUI) onCopyPath(filePath string) {
	log.Printf("onCopyPath called for path: %s", filePath)

	if filePath == "" {
		log.Printf("Error: onCopyPath called with empty filePath")
		widget.ShowPopUp(widget.NewLabel("Error: No file path provided"), ui.window.Canvas())
		return
	}

	// Check if this is actually a file path, not a URL
	if strings.HasPrefix(filePath, "http") {
		log.Printf("Error: onCopyPath called with URL instead of file path: %s", filePath)
		widget.ShowPopUp(widget.NewLabel("Error: Cannot copy URL as file path"), ui.window.Canvas())
		return
	}

	clipboard := fyne.CurrentApp().Clipboard()
	clipboard.SetContent(filePath)
	widget.ShowPopUp(widget.NewLabel("Path copied to clipboard"), ui.window.Canvas())
}

// onRemoveTask handles removing a task from the list
func (ui *RootUI) onRemoveTask(taskID string) {
	log.Printf("onRemoveTask called for task %s", taskID)

	err := ui.downloadSvc.RemoveTask(taskID)
	if err != nil {
		log.Printf("Error removing task %s: %v", taskID, err)
		widget.ShowPopUp(widget.NewLabel("Error removing task: "+err.Error()), ui.window.Canvas())
		return
	}

	log.Printf("Task %s removed from download service", taskID)

	// Remove from UI binding list
	length := ui.tasks.Length()
	for i := 0; i < length; i++ {
		item, err := ui.tasks.GetValue(i)
		if err != nil {
			continue
		}

		if task, ok := item.(*model.DownloadTask); ok && task.ID == taskID {
			// Create new list without this item
			newTasks := binding.NewUntypedList()
			for j := 0; j < length; j++ {
				if j != i {
					item, err := ui.tasks.GetValue(j)
					if err == nil {
						_ = newTasks.Append(item)
					}
				}
			}
			ui.tasks = newTasks
			ui.updateFilteredTasks()
			ui.taskList.Refresh()
			log.Printf("Task %s removed from UI list", taskID)
			break
		}
	}
}

// debouncedUIUpdate prevents excessive UI updates by limiting frequency
func (ui *RootUI) debouncedUIUpdate() {
	ui.uiUpdateMutex.Lock()
	defer ui.uiUpdateMutex.Unlock()

	now := time.Now()
	if now.Sub(ui.lastUIUpdate) < RootUIUpdateDebounce {
		return // Skip update if too soon
	}

	ui.lastUIUpdate = now
}

// onTaskUpdate handles task updates from the download service
func (ui *RootUI) onTaskUpdate(task *model.DownloadTask) {
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

	log.Printf("Task update received: id=%s status=%s percent=%d progress=%.2f output=%s",
		task.ID, task.Status, task.Percent, task.Progress, task.OutputPath)

	// Check if task just completed for notification
	wasCompleted := false

	// Update task in the list
	// Find and update the task in our binding list
	log.Printf("Updating binding list entry for id=%s", task.ID)
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
				log.Printf("Task %s completed, OutputPath: %s", task.ID, task.OutputPath)
			}
			if err := ui.tasks.SetValue(i, task); err != nil {
				log.Printf("failed to update binding for id=%s: %v", task.ID, err)
			}
			log.Printf("Binding updated for id=%s (status=%s percent=%d progress=%.2f)",
				task.ID, task.Status, task.Percent, task.Progress)
			break
		}
	}

	// Send notification for completed downloads
	if wasCompleted {
		ui.sendCompletionNotification(task)

		// Auto-reveal if enabled
		if ui.settings.GetAutoRevealOnComplete() && task.OutputPath != "" {
			log.Printf("Auto-revealing completed task %s: %s", task.ID, task.OutputPath)
			ui.onRevealFile(task.OutputPath)
		} else if ui.settings.GetAutoRevealOnComplete() && task.OutputPath == "" {
			log.Printf("Auto-reveal enabled but no OutputPath for completed task %s", task.ID)
		}
	}

	// Update filtered tasks
	log.Printf("Updating filtered tasks")
	ui.updateFilteredTasks()

	// Force direct update of TaskRow binding to avoid stale references
	fyne.Do(func() {
		length := ui.taskList.Length()
		for i := 0; i < length; i++ {
			ui.taskList.RefreshItem(i)
		}
	})

	// Update PlaylistGroup if this task is displayed there
	// Only update if values actually changed to prevent excessive UI updates
	log.Printf("Updating playlist group: id=%s progress=%.2f status=%s", task.ID, task.Progress, task.Status)
	ui.playlistGroup.UpdateVideoProgress(task.ID, task.Progress)
	ui.playlistGroup.UpdateVideoStatus(task.ID, task.Status)
	// Propagate runtime telemetry to playlist rows so speed/ETA are visible
	ui.playlistGroup.UpdateVideoSpeed(task.ID, task.Speed, task.ETASec)
	// Additionally try to match playlist items by URL, since playlist videos use
	// YouTube video IDs while download tasks have generated IDs.
	if task.URL != "" {
		ui.playlistGroup.UpdateVideoProgressByURL(task.URL, task.Progress)
		ui.playlistGroup.UpdateVideoStatusByURL(task.URL, task.Status)
		ui.playlistGroup.UpdateVideoSpeedByURL(task.URL, task.Speed, task.ETASec)
	}

	// Only update OutputPath if it's not empty and different from current
	if task.OutputPath != "" {
		log.Printf("Updating playlist output path: id=%s path=%s size=%d", task.ID, task.OutputPath, task.FileSize)
		ui.playlistGroup.UpdateVideoOutputPath(task.ID, task.OutputPath, task.FileSize)
		if task.URL != "" {
			ui.playlistGroup.UpdateVideoOutputPathByURL(task.URL, task.OutputPath, task.FileSize)
		}
	}

	// Use debounced UI update to prevent excessive refreshes
	log.Printf("Debounced UI update")
	ui.debouncedUIUpdate()

	// Refresh the list to update UI - must be done in UI thread
	fyne.Do(func() {
		log.Printf("Refreshing list and specific item")
		ui.taskList.Refresh()
		// Also refresh individual task rows if possible
		for i, filteredTask := range ui.filteredTasks {
			if filteredTask.ID == task.ID {
				// Force refresh of this specific item
				ui.taskList.RefreshItem(i)
				log.Printf("Refreshed item index=%d id=%s", i, task.ID)
				break
			}
		}

		// Single refresh of the entire UI to ensure proper display
		ui.window.Canvas().Refresh(ui.window.Content())
		log.Printf("Canvas refreshed")
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

		// Show in-app toast notification with action button
		ui.showToastNotification(task)
	}
}

// showToastNotification shows an in-app toast notification with action buttons
func (ui *RootUI) showToastNotification(task *model.DownloadTask) {
	// Create notification content
	titleLabel := widget.NewLabel(ui.localization.GetText(KeyDownloadCompleted))
	titleLabel.TextStyle = fyne.TextStyle{Bold: true}

	messageLabel := widget.NewLabel(task.GetDisplayTitle())
	messageLabel.Truncation = fyne.TextTruncateEllipsis

	// Create action buttons
	revealBtn := widget.NewButton("Reveal", func() {
		if task.OutputPath != "" {
			ui.onRevealFile(task.OutputPath)
		} else {
			log.Printf("Toast notification: Cannot reveal file for task %s - no OutputPath", task.ID)
			widget.ShowPopUp(widget.NewLabel("File path not available"), ui.window.Canvas())
		}
	})
	revealBtn.Importance = widget.HighImportance

	openBtn := widget.NewButton(ui.localization.GetText(KeyOpen), func() {
		if task.OutputPath != "" {
			ui.onOpenFile(task.OutputPath)
		} else {
			log.Printf("Toast notification: Cannot open file for task %s - no OutputPath", task.ID)
			widget.ShowPopUp(widget.NewLabel("File path not available"), ui.window.Canvas())
		}
	})
	openBtn.Importance = widget.MediumImportance

	// Create close button
	var toastPopup *widget.PopUp
	closeBtn := widget.NewButton(IconClose, func() {
		if toastPopup != nil {
			toastPopup.Hide()
		}
	})
	closeBtn.Importance = widget.LowImportance

	// Layout the toast content
	header := container.NewBorder(nil, nil, titleLabel, closeBtn)
	actions := container.NewHBox(revealBtn, openBtn)
	content := container.NewVBox(
		header,
		messageLabel,
		actions,
	)

	// Create and position the popup
	toastPopup = widget.NewModalPopUp(content, ui.window.Canvas())

	// Position in top-right corner
	canvasSize := ui.window.Canvas().Size()
	toastSize := fyne.NewSize(RootToastWidth, RootToastHeight)
	toastPos := fyne.NewPos(canvasSize.Width-toastSize.Width-RootToastMargin, RootToastMargin)

	toastPopup.Resize(toastSize)
	toastPopup.Move(toastPos)
	toastPopup.Show()

	// Auto-hide after configured time
	go func() {
		time.Sleep(RootToastAutoHide)
		if toastPopup != nil {
			toastPopup.Hide()
		}
	}()
}

// generateTaskID removed as unused

// Playlist methods

// onPlaylistDownload handles playlist download requests
func (ui *RootUI) onPlaylistDownload(playlist *model.Playlist) {
	// Add playlist to download service
	err := ui.downloadSvc.AddPlaylist(playlist)
	if err != nil {
		log.Printf("Failed to add playlist to download service: %v", err)
		return
	}

	// Start downloading the playlist
	err = ui.downloadSvc.DownloadPlaylist(playlist)
	if err != nil {
		log.Printf("Failed to start playlist download: %v", err)
		return
	}

	log.Printf("Started downloading playlist: %s with %d videos", playlist.Title, playlist.TotalVideos)
}

// onPlaylistCancel handles playlist cancellation requests
func (ui *RootUI) onPlaylistCancel(playlist *model.Playlist) {
	if playlist == nil {
		return
	}

	// Cancel playlist download
	err := ui.downloadSvc.CancelPlaylist(playlist.ID)
	if err != nil {
		log.Printf("Failed to cancel playlist: %v", err)
		return
	}

	log.Printf("Cancelled playlist download: %s", playlist.Title)
}

// isPlaylistURL checks if the URL is a playlist URL
func (ui *RootUI) isPlaylistURL(url string) bool {
	return strings.Contains(url, RootPlaylistQueryParam)
}

// handlePlaylistURL handles playlist URL processing
func (ui *RootUI) handlePlaylistURL(url string) {
	// Clean URL from any special characters
	cleanURL := strings.ReplaceAll(url, "\n", "")
	cleanURL = strings.ReplaceAll(cleanURL, "\r", "")
	cleanURL = strings.ReplaceAll(cleanURL, "\t", " ")
	cleanURL = strings.TrimSpace(cleanURL)

	log.Printf("Processing playlist URL: %s", cleanURL)

	// Show notification: parsing started (without spinner)
	fyne.Do(func() { ui.showNotification(ui.localization.GetText(KeyParsingStarted), false) })

	// Parse playlist in background
	go func() {
		log.Printf("Starting playlist parsing in background...")
		playlist, err := ui.parserService.ParsePlaylist(context.Background(), cleanURL)

		// Update UI in main thread
		fyne.Do(func() {
			if err != nil {
				log.Printf("Playlist parsing failed: %v", err)
				ui.showNotification(ui.localization.GetText(KeyParsingFailed)+": "+err.Error(), false)
				return
			}

			log.Printf("Playlist parsed successfully: %s with %d videos", playlist.Title, playlist.TotalVideos)
			log.Printf("Playlist videos: %+v", playlist.Videos)

			// Add playlist to playlist group
			log.Printf("Adding playlist to UI...")
			ui.playlistGroup.AddPlaylist(playlist)

			// Clear URL entry
			ui.urlEntry.SetText("")

			// Show success message in notification panel
			ui.showNotification(fmt.Sprintf("%s: %s (%d)", ui.localization.GetText(KeyPlaylistParsed), playlist.Title, playlist.TotalVideos), false)

			log.Printf("Playlist added to UI successfully")

			// Auto-start downloading the playlist
			log.Printf("Auto-starting playlist download...")
			go func() {
				// Small delay to ensure UI is updated
				time.Sleep(RootPlaylistParseDelay)

				// Start downloading the playlist
				err := ui.downloadSvc.AddPlaylist(playlist)
				if err != nil {
					log.Printf("Failed to add playlist to download service: %v", err)
					return
				}

				err = ui.downloadSvc.DownloadPlaylist(playlist)
				if err != nil {
					log.Printf("Failed to start playlist download: %v", err)
					return
				}

				log.Printf("Auto-started downloading playlist: %s with %d videos", playlist.Title, playlist.TotalVideos)
			}()
		})
	}()
}

// readAndApplySettings reads current settings and applies them to download service
func (ui *RootUI) readAndApplySettings() {
	// Update localization language
	ui.localization.SetLanguage(ui.settings.GetLanguage())

	// Get configured downloads directory
	downloadsDir := ui.settings.GetDownloadDirectory()

	// Ensure directory exists
	if err := platform.CreateDirectoryIfNotExists(downloadsDir); err != nil {
		log.Printf("failed to ensure downloads dir: %v", err)
	}

	// Update download service settings
	ui.downloadSvc.SetMaxParallelDownloads(ui.settings.GetMaxParallelDownloads())
	ui.downloadSvc.SetDownloadDirectory(downloadsDir)

	// Update quality preset
	switch ui.settings.GetQualityPreset() {
	case config.QualityBest:
		ui.downloadSvc.SetQualityPreset("best")
	case config.QualityMedium:
		ui.downloadSvc.SetQualityPreset("medium")
	case config.QualityAudio:
		ui.downloadSvc.SetQualityPreset("audio")
	default:
		ui.downloadSvc.SetQualityPreset("best")
	}

	log.Printf("Settings applied: dir=%s, maxParallel=%d, quality=%s",
		downloadsDir, ui.settings.GetMaxParallelDownloads(), ui.settings.GetQualityPreset())
}

// createMobileIconPanel creates a panel with quick access icons for mobile
func (ui *RootUI) createMobileIconPanel() *fyne.Container {
	// Create horizontal container with title in center and icons on sides
	leftIcons := container.NewHBox(ui.settingsBtn, ui.languageBtn)
	rightSpacer := widget.NewLabel("") // Empty spacer for balance

	// Create main container with title in center
	mainContainer := container.NewBorder(
		nil, nil, // top, bottom
		leftIcons,     // left - icons
		rightSpacer,   // right - empty spacer
		ui.titleLabel, // center - title
	)

	// Add some padding
	return container.NewPadded(mainContainer)
}

// onLanguageButtonClick handles language button click on mobile
func (ui *RootUI) onLanguageButtonClick() {
	// Create language selection popup
	availableLanguages := ui.localization.GetAvailableLanguages()

	var languageItems []fyne.CanvasObject
	for code, name := range availableLanguages {
		langCode := code // Capture for closure

		// Create button for each language
		langBtn := widget.NewButton(name, func() {
			ui.onLanguageChange(langCode)
			if ui.languagePopup != nil {
				ui.languagePopup.Hide()
			}
		})

		// Mark current language
		if ui.localization.GetCurrentLanguage() == langCode {
			langBtn.Importance = widget.HighImportance
		} else {
			langBtn.Importance = widget.MediumImportance
		}

		languageItems = append(languageItems, langBtn)
	}

	// Create popup content
	popupContent := container.NewVBox(languageItems...)

	// Create and show popup
	ui.languagePopup = widget.NewModalPopUp(popupContent, ui.window.Canvas())
	ui.languagePopup.Resize(fyne.NewSize(200, float32(len(languageItems)*50)))
	ui.languagePopup.Show()
}
