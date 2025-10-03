package ui

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
)

const (
	AppIcon = "yt-downloader.png"
)

// LogoResource represents the embedded logo resource
var LogoResource *fyne.StaticResource

// LoadLogoResource loads the logo from file path
func LoadLogoResource() (fyne.Resource, error) {
	// Try to load from current directory first
	resource, err := fyne.LoadResourceFromPath(AppIcon)
	if err == nil {
		return resource, nil
	}

	// If not found, try to load from the executable directory
	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}

	execDir := filepath.Dir(execPath)
	logoPath := filepath.Join(execDir, AppIcon)
	return fyne.LoadResourceFromPath(logoPath)
}

// InitLogoResource initializes the embedded logo resource
func InitLogoResource() error {
	resource, err := fyne.LoadResourceFromPath(AppIcon)
	if err != nil {
		return err
	}

	LogoResource = &fyne.StaticResource{
		StaticName:    AppIcon,
		StaticContent: resource.Content(),
	}

	return nil
}
