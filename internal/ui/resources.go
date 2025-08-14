package ui

import "fyne.io/fyne/v2"

// LogoResource represents the embedded logo resource
var LogoResource = &fyne.StaticResource{
	StaticName:    "yt-downloader-logo.png",
	StaticContent: []byte{
		// This is a placeholder - we'll use LoadResourceFromPath instead
		// to avoid embedding large binary data
	},
}

// LoadLogoResource loads the logo from file path
func LoadLogoResource() (fyne.Resource, error) {
	return fyne.LoadResourceFromPath("yt-downloader.png")
}
