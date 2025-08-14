package compress

import (
	"os"
	"strings"
	"testing"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestNewService(t *testing.T) {
	service := NewService()

	if service == nil {
		t.Error("Expected service to be created, got nil")
	}
}

func TestGenerateOutputPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/video.mp4", "/path/to/video-compressed.mp4"},
		{"/path/to/video.mkv", "/path/to/video-compressed.mp4"},
		{"video.avi", "video-compressed.mp4"},
		{"/no/ext/file", "/no/ext/file-compressed.mp4"},
	}

	for _, test := range tests {
		result := generateOutputPath(test.input)
		if result != test.expected {
			t.Errorf("generateOutputPath(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestBuildFFmpegArgs(t *testing.T) {
	service := NewService().(*Service)
	args := service.BuildFFmpegArgs("/input.mp4", "/output.mp4")

	expectedArgs := []string{
		"-y",
		"-i", "/input.mp4",
		"-c:v", VideoCodec,
		"-preset", VideoPreset,
		"-crf", VideoCRF,
		"-c:a", AudioCodec,
		"-b:a", AudioBitrate,
		"-movflags", FastStartFlag,
		"-progress", "pipe:2",
		"-nostats",
		"/output.mp4",
	}

	if len(args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(args))
	}

	for i, expected := range expectedArgs {
		if args[i] != expected {
			t.Errorf("Arg %d: expected %s, got %s", i, expected, args[i])
		}
	}
}

func TestStartCompression_NonExistentFile(t *testing.T) {
	service := NewService()

	_, err := service.StartCompression("/path/to/nonexistent/file.mp4")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected 'does not exist' error, got: %v", err)
	}
}

func TestStartCompression_WithExistingFile(t *testing.T) {
	service := NewService()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_video_*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	task, err := service.StartCompression(tempFile.Name())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if task == nil {
		t.Fatal("Expected task to be created, got nil")
	}

	if task.InputPath != tempFile.Name() {
		t.Errorf("Expected InputPath to be %s, got %s", tempFile.Name(), task.InputPath)
	}

	expectedOutput := generateOutputPath(tempFile.Name())
	if task.OutputPath != expectedOutput {
		t.Errorf("Expected OutputPath to be %s, got %s", expectedOutput, task.OutputPath)
	}

	if task.Status != model.TaskStatusPending && task.Status != model.TaskStatusStarting {
		t.Errorf("Expected status to be Pending or Starting, got %s", task.Status)
	}

	// Verify task is stored
	retrievedTask, exists := service.GetTask(task.ID)
	if !exists {
		t.Error("Task should exist in service")
	}

	if retrievedTask.ID != task.ID {
		t.Errorf("Retrieved task ID should be %s, got %s", task.ID, retrievedTask.ID)
	}
}

func TestStartCompression_DuplicateTask(t *testing.T) {
	service := NewService().(*Service)

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "test_video_*.mp4")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	// Start first compression
	task1, err := service.StartCompression(tempFile.Name())
	if err != nil {
		t.Fatalf("Expected no error for first compression, got: %v", err)
	}

	// Try to start second compression for same file
	// We need to wait a bit to ensure the first task is active
	// For testing, we'll manually set it to active status
	service.tasksMutex.Lock()
	task1.Status = model.TaskStatusDownloading
	service.tasksMutex.Unlock()

	_, err = service.StartCompression(tempFile.Name())
	if err == nil {
		t.Error("Expected error for duplicate compression, got nil")
	}

	if !strings.Contains(err.Error(), "already in progress") {
		t.Errorf("Expected 'already in progress' error, got: %v", err)
	}
}

func TestUpdateCallback(t *testing.T) {
	service := NewService().(*Service)

	updateCalled := false
	var updatedTask *model.CompressionTask

	service.SetUpdateCallback(func(task *model.CompressionTask) {
		updateCalled = true
		updatedTask = task
	})

	// Create a test task
	task := &model.CompressionTask{
		ID:         "test-id",
		InputPath:  "/test/input.mp4",
		OutputPath: "/test/output.mp4",
		Status:     model.TaskStatusDownloading,
	}

	service.notifyUpdate(task)

	if !updateCalled {
		t.Error("Expected update callback to be called")
	}

	if updatedTask != task {
		t.Error("Expected updated task to be the same as input task")
	}
}

func TestGenerateTaskID(t *testing.T) {
	// Test multiple ID generation to ensure uniqueness
	ids := make(map[string]bool)
	const numIDs = 1000

	for i := 0; i < numIDs; i++ {
		id := generateTaskID()

		// Check prefix
		if !strings.HasPrefix(id, "compress-") {
			t.Errorf("Expected ID to start with 'compress-', got: %s", id)
		}

		// Check UUID format (compress- + 36 chars for UUID)
		if len(id) != len("compress-")+36 {
			t.Errorf("Expected ID length %d, got %d for ID: %s", len("compress-")+36, len(id), id)
		}

		// Check UUID part format (8-4-4-4-12)
		uuidPart := strings.TrimPrefix(id, "compress-")
		if !isValidUUID(uuidPart) {
			t.Errorf("Invalid UUID format in ID: %s", id)
		}

		// Check uniqueness
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}

	// Verify we got the expected number of unique IDs
	if len(ids) != numIDs {
		t.Errorf("Expected %d unique IDs, got %d", numIDs, len(ids))
	}
}

// isValidUUID checks if a string is a valid UUID v4 format
func isValidUUID(uuid string) bool {
	if len(uuid) != 36 {
		return false
	}

	// Check UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		return false
	}

	if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		return false
	}

	// Check that all parts are hexadecimal
	for _, part := range parts {
		for _, char := range part {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				return false
			}
		}
	}

	return true
}

func TestGenerateTaskID_Uniqueness(t *testing.T) {
	// Test that IDs are unique even when generated rapidly
	const numIDs = 100
	ids := make(map[string]bool)

	for i := 0; i < numIDs; i++ {
		id := generateTaskID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateTaskID_Format(t *testing.T) {
	id := generateTaskID()

	// Check prefix
	if !strings.HasPrefix(id, "compress-") {
		t.Errorf("Expected ID to start with 'compress-', got: %s", id)
	}

	// Check total length
	expectedLength := len("compress-") + 36 // UUID length
	if len(id) != expectedLength {
		t.Errorf("Expected ID length %d, got %d", expectedLength, len(id))
	}

	// Check UUID part
	uuidPart := strings.TrimPrefix(id, "compress-")
	if !isValidUUID(uuidPart) {
		t.Errorf("Invalid UUID format: %s", uuidPart)
	}
}

func TestGenerateTaskID_ConsistentPrefix(t *testing.T) {
	// Test that all generated IDs have the same prefix
	const numIDs = 100
	expectedPrefix := "compress-"

	for i := 0; i < numIDs; i++ {
		id := generateTaskID()
		if !strings.HasPrefix(id, expectedPrefix) {
			t.Errorf("ID %d: Expected prefix '%s', got: %s", i, expectedPrefix, id)
		}
	}
}

func TestGenerateTaskID_UUIDVersion(t *testing.T) {
	// Test that generated UUIDs are valid and follow proper format
	id := generateTaskID()
	uuidPart := strings.TrimPrefix(id, "compress-")

	// UUID v7 has specific version and variant bits
	// Version 7: first 4 bits of time_hi_and_version should be 0111 (7)
	// Variant: first 2 bits of clock_seq_hi_and_reserved should be 10

	// Extract version bits (position 14-15 in hex string)
	if len(uuidPart) < 15 {
		t.Fatalf("UUID too short: %s", uuidPart)
	}

	// Check version (7th character should be '7')
	if uuidPart[14] != '7' {
		t.Errorf("Expected UUID v7, version character should be '7', got: %c", uuidPart[14])
	}

	// Check variant (9th character should be '8', '9', 'a', or 'b')
	variantChar := uuidPart[19]
	if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
		t.Errorf("Expected valid UUID variant, got: %c", variantChar)
	}
}

func TestGenerateTaskID_StressTest(t *testing.T) {
	// Stress test to ensure no collisions under high load
	const numIDs = 10000
	ids := make(map[string]bool)

	t.Logf("Generating %d unique IDs...", numIDs)

	for i := 0; i < numIDs; i++ {
		id := generateTaskID()
		if ids[id] {
			t.Errorf("Collision detected at iteration %d: %s", i, id)
		}
		ids[id] = true

		// Progress indicator for long tests
		if i%1000 == 0 {
			t.Logf("Generated %d IDs...", i)
		}
	}

	t.Logf("Successfully generated %d unique IDs", len(ids))
}
