package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/ytget/yt-downloader/internal/compress"
	"github.com/ytget/yt-downloader/internal/config"
	"github.com/ytget/yt-downloader/internal/download"
	"github.com/ytget/yt-downloader/internal/platform"
	"github.com/ytget/yt-downloader/internal/ui"
)

const (
	AppID   = "com.ytget.yt-downloader"
	AppName = "YT Downloader"
	AppIcon = "yt-downloader.png"

	WindowWidth  = 800
	WindowHeight = 600
)

func main() {
	// Create new Fyne app
	myApp := app.NewWithID(AppID)

	// Apply compact theme
	myApp.Settings().SetTheme(ui.NewCompactTheme())

	myWindow := myApp.NewWindow(AppName)
	myWindow.Resize(fyne.NewSize(WindowWidth, WindowHeight))

	// Initialize services
	settings := config.NewSettings(myApp)
	downloadsDir := settings.GetDownloadDirectory()
	platform.CreateDirectoryIfNotExists(downloadsDir)

	downloadSvc := download.NewService(downloadsDir, settings.GetMaxParallelDownloads())
	compressSvc := compress.NewService()

	// Create and setup UI
	ui.NewRootUI(myWindow, myApp, downloadSvc, compressSvc)

	// Show and run
	myWindow.ShowAndRun()
}
