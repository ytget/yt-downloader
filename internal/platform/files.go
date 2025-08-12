package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenFileInManager opens the file in the system file manager and highlights it
func OpenFileInManager(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	switch runtime.GOOS {
	case "darwin": // macOS
		return openFileInFinderMacOS(absPath)
	case "windows":
		return openFileInExplorerWindows(absPath)
	case "linux":
		return openFileInManagerLinux(absPath)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// openFileInFinderMacOS opens file in Finder on macOS with selection
func openFileInFinderMacOS(filePath string) error {
	cmd := exec.Command("open", "-R", filePath)
	return cmd.Run()
}

// openFileInExplorerWindows opens file in Explorer on Windows with selection
func openFileInExplorerWindows(filePath string) error {
	cmd := exec.Command("explorer", "/select,", filePath)
	return cmd.Run()
}

// openFileInManagerLinux opens directory containing file on Linux
// Note: File selection is not standardized on Linux, so we open the parent directory
func openFileInManagerLinux(filePath string) error {
	dir := filepath.Dir(filePath)

	// Try xdg-open first (most common)
	cmd := exec.Command("xdg-open", dir)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback to common file managers
	fileManagers := []string{"nautilus", "dolphin", "thunar", "nemo", "pcmanfm"}

	for _, fm := range fileManagers {
		if _, err := exec.LookPath(fm); err == nil {
			cmd := exec.Command(fm, dir)
			return cmd.Run()
		}
	}

	return fmt.Errorf("no suitable file manager found")
}

// CreateDirectoryIfNotExists creates directory if it doesn't exist
func CreateDirectoryIfNotExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}

// GetHomeDownloadsDir returns the standard Downloads directory for the user
func GetHomeDownloadsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	downloadsDir := filepath.Join(homeDir, "Downloads")
	return downloadsDir, nil
}
