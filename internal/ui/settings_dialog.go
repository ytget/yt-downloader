package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"

	"fyne.io/fyne/v2/widget"

	"github.com/ytget/yt-downloader/internal/config"
)

// ShowSettingsDialog shows the application settings dialog
func ShowSettingsDialog(window fyne.Window, settings *config.Settings, localization *Localization, onSettingsChanged func()) {
	// Download directory selection
	downloadDirLabel := widget.NewLabel(localization.GetText(KeyDownloadDirectory) + ":")
	downloadDirEntry := widget.NewEntry()
	downloadDirEntry.SetText(settings.GetDownloadDirectory())

	browseBtn := widget.NewButton(localization.GetText(KeyBrowse), func() {
		dialog.ShowFolderOpen(func(folder fyne.ListableURI, err error) {
			if err == nil && folder != nil {
				downloadDirEntry.SetText(folder.Path())
			}
		}, window)
	})

	downloadDirContainer := container.NewBorder(nil, nil, nil, browseBtn, downloadDirEntry)

	// Quality preset selection
	qualityLabel := widget.NewLabel(localization.GetText(KeyQualityPreset) + ":")
	qualitySelect := widget.NewSelect([]string{"Best", "Medium", "Audio Only"}, nil)

	// Set current value
	switch settings.GetQualityPreset() {
	case config.QualityBest:
		qualitySelect.SetSelected("Best")
	case config.QualityMedium:
		qualitySelect.SetSelected("Medium")
	case config.QualityAudio:
		qualitySelect.SetSelected("Audio Only")
	}

	// Max parallel downloads
	parallelLabel := widget.NewLabel(localization.GetText(KeyMaxParallel) + ":")
	parallelEntry := widget.NewEntry()
	parallelEntry.SetText(strconv.Itoa(settings.GetMaxParallelDownloads()))
	parallelEntry.Validator = func(s string) error {
		if _, err := strconv.Atoi(s); err != nil {
			return err
		}
		return nil
	}

	// Auto reveal setting
	autoRevealCheck := widget.NewCheck("Auto-reveal completed downloads", nil)
	autoRevealCheck.SetChecked(settings.GetAutoRevealOnComplete())

	// Create form
	form := container.NewVBox(
		downloadDirLabel,
		downloadDirContainer,
		widget.NewSeparator(),
		qualityLabel,
		qualitySelect,
		widget.NewSeparator(),
		parallelLabel,
		parallelEntry,
		widget.NewSeparator(),
		autoRevealCheck,
	)

	// Save function
	saveSettings := func() {
		// Save download directory
		settings.SetDownloadDirectory(downloadDirEntry.Text)

		// Save quality preset
		switch qualitySelect.Selected {
		case "Best":
			settings.SetQualityPreset(config.QualityBest)
		case "Medium":
			settings.SetQualityPreset(config.QualityMedium)
		case "Audio Only":
			settings.SetQualityPreset(config.QualityAudio)
		}

		// Save max parallel downloads
		if parallel, err := strconv.Atoi(parallelEntry.Text); err == nil && parallel > 0 {
			settings.SetMaxParallelDownloads(parallel)
		}

		// Save auto reveal setting
		settings.SetAutoRevealOnComplete(autoRevealCheck.Checked)

		// Notify parent about changes
		if onSettingsChanged != nil {
			onSettingsChanged()
		}
	}

	// Show dialog
	dlg := dialog.NewCustomConfirm(localization.GetText(KeySettings), localization.GetText(KeySave), localization.GetText(KeyCancel), form, func(confirmed bool) {
		if confirmed {
			saveSettings()
		}
	}, window)
	dlg.Resize(fyne.NewSize(500, 400))
	dlg.Show()
}
