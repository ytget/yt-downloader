package compress

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/romanitalian/yt-downloader/internal/model"
)

func TestNewService(t *testing.T) {
	service := NewService()

	if len(service.tasks) != 0 {
		t.Errorf("Expected empty tasks map, got %d items", len(service.tasks))
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
	service := NewService()
	args := service.buildFFmpegArgs("/input.mp4", "/output.mp4")

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
	service := NewService()

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
	service := NewService()

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
	id1 := generateTaskID()
	time.Sleep(1 * time.Millisecond) // Ensure different timestamp
	id2 := generateTaskID()

	if id1 == id2 {
		t.Error("Expected different task IDs")
	}

	if !strings.HasPrefix(id1, "compress-") {
		t.Errorf("Expected ID to start with 'compress-', got: %s", id1)
	}

	if !strings.HasPrefix(id2, "compress-") {
		t.Errorf("Expected ID to start with 'compress-', got: %s", id2)
	}
}
