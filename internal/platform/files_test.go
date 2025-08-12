package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDirectoryIfNotExists(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	testDir := filepath.Join(tempDir, "test_dir")

	// Directory should not exist initially
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Fatalf("Test directory already exists: %s", testDir)
	}

	// Create directory
	err := CreateDirectoryIfNotExists(testDir)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Directory should now exist
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Fatalf("Directory was not created: %s", testDir)
	}

	// Second call should not fail
	err = CreateDirectoryIfNotExists(testDir)
	if err != nil {
		t.Fatalf("Failed to handle existing directory: %v", err)
	}
}

func TestGetHomeDownloadsDir(t *testing.T) {
	downloadsDir, err := GetHomeDownloadsDir()
	if err != nil {
		t.Fatalf("Failed to get downloads directory: %v", err)
	}

	if downloadsDir == "" {
		t.Fatal("Downloads directory is empty")
	}

	// Should end with "Downloads"
	if filepath.Base(downloadsDir) != "Downloads" {
		t.Errorf("Expected directory to end with 'Downloads', got: %s", downloadsDir)
	}
}

func TestOpenFileInManager_NonExistentFile(t *testing.T) {
	nonExistentFile := "/path/to/nonexistent/file.txt"

	err := OpenFileInManager(nonExistentFile)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	if err.Error() != "file does not exist: "+nonExistentFile {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestOpenFileInManager_WithExistingFile(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_file_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// This test just verifies the function doesn't panic and handles the file path
	// We can't really test the actual opening without user interaction
	err = OpenFileInManager(tempFile.Name())

	// On CI or headless systems, this might fail, which is expected
	// We're mainly testing that the function handles the path correctly
	if err != nil {
		t.Logf("OpenFileInManager failed (expected on headless systems): %v", err)
	}
}
