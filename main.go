package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/ytget/yt-downloader/internal/compress"
	"github.com/ytget/yt-downloader/internal/download"
	"github.com/ytget/yt-downloader/internal/platform"
	"github.com/ytget/yt-downloader/internal/ui"
)

// Version is set during build via -ldflags "-X main.version=X.Y.Z"
var version = "dev"

const (
	AppID   = "com.ytget.yt-downloader"
	AppName = "YT Downloader"
	AppIcon = "yt-downloader.png"

	WindowWidth  = 800
	WindowHeight = 600
)

func main() {
	// Log version information
	fmt.Printf("YT Downloader v%s starting...\n", version)

	// Initialize logo resource
	if err := ui.InitLogoResource(); err != nil {
		fmt.Printf("Warning: failed to initialize logo: %v\n", err)
	}

	// Create new Fyne app
	myApp := app.NewWithID(AppID)

	// Apply compact theme
	myApp.Settings().SetTheme(ui.NewCompactTheme())

	windowTitle := fmt.Sprintf("%s v%s", AppName, version)
	myWindow := myApp.NewWindow(windowTitle)
	myWindow.Resize(fyne.NewSize(WindowWidth, WindowHeight))

	// Initialize services with default values
	// Settings will be read when Download button is clicked
	defaultDownloadsDir, err := platform.GetHomeDownloadsDir()
	if err != nil {
		fmt.Printf("failed to get downloads dir: %v\n", err)
		defaultDownloadsDir = "/tmp/downloads"
	}
	if err := platform.CreateDirectoryIfNotExists(defaultDownloadsDir); err != nil {
		fmt.Printf("failed to ensure downloads dir: %v\n", err)
	}

	downloadSvc := download.NewService(defaultDownloadsDir, 2) // Default values
	downloadSvc.SetQualityPreset("best")                       // Default quality

	compressSvc := compress.NewService()

	// Create and setup UI
	ui.NewRootUI(myWindow, myApp, downloadSvc, compressSvc)

	// Show and run
	myWindow.ShowAndRun()
}
