package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/romanitalian/yt-downloader/internal/config"
)

// SettingsDialog represents the settings configuration dialog
type SettingsDialog struct {
	settings *config.Settings
	window   fyne.Window
	dialog   *dialog.ConfirmDialog

	// UI components
	downloadDirEntry *widget.Entry
	maxParallelEntry *widget.Entry
	qualitySelect    *widget.Select
	filenameEntry    *widget.Entry
	languageSelect   *widget.Select
}

// NewSettingsDialog creates a new settings dialog
func NewSettingsDialog(settings *config.Settings, window fyne.Window) *SettingsDialog {
	sd := &SettingsDialog{
		settings: settings,
		window:   window,
	}

	sd.createUI()
	return sd
}

// Show displays the settings dialog
func (sd *SettingsDialog) Show() {
	sd.loadCurrentSettings()
	sd.dialog.Show()
}

// createUI creates the settings dialog UI
func (sd *SettingsDialog) createUI() {
	// Download directory selection
	sd.downloadDirEntry = widget.NewEntry()
	sd.downloadDirEntry.SetPlaceHolder("Download directory path")

	browseDirBtn := widget.NewButton("Browse", sd.onBrowseDirectory)
	downloadDirRow := container.NewBorder(nil, nil, nil, browseDirBtn, sd.downloadDirEntry)

	// Max parallel downloads
	sd.maxParallelEntry = widget.NewEntry()
	sd.maxParallelEntry.SetPlaceHolder("1-10")

	// Quality preset selection
	qualityOptions := []string{}
	for _, preset := range sd.settings.GetQualityPresetOptions() {
		qualityOptions = append(qualityOptions, string(preset))
	}
	sd.qualitySelect = widget.NewSelect(qualityOptions, nil)

	// Filename template
	sd.filenameEntry = widget.NewEntry()
	sd.filenameEntry.SetPlaceHolder("%(title)s.%(ext)s")

	// Language selection
	languageOptions := []string{}
	languageLabels := sd.settings.GetLanguageOptions()
	for code := range languageLabels {
		languageOptions = append(languageOptions, code)
	}
	sd.languageSelect = widget.NewSelect(languageOptions, nil)
	// Custom formatting for language display
	sd.languageSelect.PlaceHolder = "Select language"

	// Create form
	form := container.NewVBox(
		widget.NewLabel("Download Settings"),
		widget.NewSeparator(),

		widget.NewLabel("Download Directory:"),
		downloadDirRow,

		widget.NewLabel("Max Parallel Downloads:"),
		sd.maxParallelEntry,

		widget.NewLabel("Quality Preset:"),
		sd.qualitySelect,

		widget.NewLabel("Filename Template:"),
		sd.filenameEntry,

		widget.NewSeparator(),
		widget.NewLabel("Interface Settings"),
		widget.NewSeparator(),

		widget.NewLabel("Language:"),
		sd.languageSelect,
	)

	// Create dialog with buttons
	sd.dialog = dialog.NewCustomConfirm(
		"Settings",
		"Save",
		"Cancel",
		form,
		sd.onSave,
		sd.window,
	)

	sd.dialog.Resize(fyne.NewSize(500, 400))
}

// loadCurrentSettings loads current settings into the UI
func (sd *SettingsDialog) loadCurrentSettings() {
	sd.downloadDirEntry.SetText(sd.settings.GetDownloadDirectory())
	sd.maxParallelEntry.SetText(strconv.Itoa(sd.settings.GetMaxParallelDownloads()))
	sd.qualitySelect.SetSelected(string(sd.settings.GetQualityPreset()))
	sd.filenameEntry.SetText(sd.settings.GetFilenameTemplate())
	sd.languageSelect.SetSelected(sd.settings.GetLanguage())
}

// onBrowseDirectory handles directory browsing
func (sd *SettingsDialog) onBrowseDirectory() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		sd.downloadDirEntry.SetText(uri.Path())
	}, sd.window)
}

// onSave handles saving the settings
func (sd *SettingsDialog) onSave(confirmed bool) {
	if !confirmed {
		return
	}

	// Validate and save download directory
	downloadDir := sd.downloadDirEntry.Text
	if downloadDir != "" {
		sd.settings.SetDownloadDirectory(downloadDir)
	}

	// Validate and save max parallel downloads
	maxParallelStr := sd.maxParallelEntry.Text
	if maxParallelStr != "" {
		if maxParallel, err := strconv.Atoi(maxParallelStr); err == nil {
			sd.settings.SetMaxParallelDownloads(maxParallel)
		}
	}

	// Save quality preset
	if sd.qualitySelect.Selected != "" {
		sd.settings.SetQualityPreset(config.QualityPreset(sd.qualitySelect.Selected))
	}

	// Save filename template
	if sd.filenameEntry.Text != "" {
		sd.settings.SetFilenameTemplate(sd.filenameEntry.Text)
	}

	// Save language
	if sd.languageSelect.Selected != "" {
		sd.settings.SetLanguage(sd.languageSelect.Selected)
	}

	// Show confirmation
	dialog.ShowInformation("Settings", "Settings saved successfully!", sd.window)
}
