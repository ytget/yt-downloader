package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/romanitalian/yt-downloader/internal/ui"
)

func main() {
	// Create new Fyne app
	myApp := app.NewWithID("com.romanitalian.yt-downloader")
	myWindow := myApp.NewWindow("YT Downloader")
	myWindow.Resize(fyne.NewSize(800, 600))

	// Create and setup UI
	ui.NewRootUI(myWindow, myApp)

	// Show and run
	myWindow.ShowAndRun()
}
