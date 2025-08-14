package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/ytget/yt-downloader/internal/ui"
)

func main() {
	// Create new Fyne app
	myApp := app.NewWithID("com.ytget.yt-downloader")

	// Apply compact theme
	myApp.Settings().SetTheme(ui.NewCompactTheme())

	myWindow := myApp.NewWindow("YT Downloader")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Create and setup UI
	ui.NewRootUI(myWindow, myApp)

	// Show and run
	myWindow.ShowAndRun()
}
