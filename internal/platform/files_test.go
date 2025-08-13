package platform

import (
	"os"
	"path/filepath"
	"strings"
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
	// Create a temporary directory with no similar files
	tempDir := t.TempDir()
	nonExistentFile := filepath.Join(tempDir, "nonexistent.txt")

	err := OpenFileInManager(nonExistentFile)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// Check that error contains the expected message
	if !strings.Contains(err.Error(), "file does not exist:") {
		t.Errorf("Error message should contain 'file does not exist:', got: %v", err)
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

func TestFindFileWithFallback_ExistingFile(t *testing.T) {
	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_file_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Function should find the existing file
	foundPath, err := FindFileWithFallback(tempFile.Name())
	if err != nil {
		t.Fatalf("Failed to find existing file: %v", err)
	}

	if foundPath != tempFile.Name() {
		t.Errorf("Expected path %s, got %s", tempFile.Name(), foundPath)
	}
}

func TestFindFileWithFallback_SimilarFileName(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a file with a similar name
	originalName := "test_video.mp4"
	similarName := "-test_video.mp4"

	originalPath := filepath.Join(tempDir, originalName)
	similarPath := filepath.Join(tempDir, similarName)

	// Create the similar file
	tempFile, err := os.Create(similarPath)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(similarPath)

	// Function should find the similar file
	foundPath, err := FindFileWithFallback(originalPath)
	if err != nil {
		t.Fatalf("Failed to find similar file: %v", err)
	}

	if foundPath != similarPath {
		t.Errorf("Expected path %s, got %s", similarPath, foundPath)
	}
}

func TestFindFileWithFallback_NoSimilarFile(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Create a file with a name that won't be considered a downloaded file
	// (too short and no descriptive elements)
	differentName := "a.mp4"
	differentPath := filepath.Join(tempDir, differentName)

	tempFile, err := os.Create(differentPath)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(differentPath)

	// Function should not find a similar file
	originalPath := filepath.Join(tempDir, "test_video.mp4")
	_, err = FindFileWithFallback(originalPath)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	expectedError := "file not found: " + originalPath
	if err.Error() != expectedError {
		t.Errorf("Expected error message %s, got %v", expectedError, err)
	}
}

func TestIsSimilarFileName(t *testing.T) {
	tests := []struct {
		name1, name2 string
		expected     bool
	}{
		{"test", "test", true},
		{"test", "-test", true},
		{"test", "test-", true},
		{"test", "_test", true},
		{"test", "test_", true},
		{"test", " test", true},
		{"test", "test ", true},
		{"test", "other", false},
		{"test_video", "test_video_long", true},
		{"test_video_long", "test_video", true},
		{"test_video_very_long_name", "test_video", false}, // too different
	}

	for _, tt := range tests {
		t.Run(tt.name1+"_"+tt.name2, func(t *testing.T) {
			result := isSimilarFileName(tt.name1, tt.name2)
			if result != tt.expected {
				t.Errorf("isSimilarFileName(%q, %q) = %v, expected %v",
					tt.name1, tt.name2, result, tt.expected)
			}
		})
	}
}

func TestGetDescriptiveScore(t *testing.T) {
	tests := []struct {
		filename string
		expected int
	}{
		{"short.mp4", 0},                                // short name (len=9 < 10)
		{"medium_name.mp4", 2},                          // medium name with underscore
		{"long descriptive name.mp4", 5},                // long name with spaces
		{"Rammstein_-_Ohne_Dich_Official_Video.mp4", 7}, // contains video word
		{"2025-06-30 10.43.15.mp4", 5},                  // timestamp-like name
		{"a.mp4", 0},                                    // very short name
		{"music_mix.mp4", 4},                            // contains music word
		{"artist-song.mp4", 4},                          // contains hyphens
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := getDescriptiveScore(tt.filename)
			if result != tt.expected {
				t.Errorf("getDescriptiveScore(%q) = %d, expected %d",
					tt.filename, result, tt.expected)
			}
		})
	}
}
