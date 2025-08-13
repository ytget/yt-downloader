package download

import (
	"testing"
	"time"

	"github.com/romanitalian/yt-downloader/internal/model"
)

func TestNewService(t *testing.T) {
	service := NewService("/tmp", 2)

	if service.downloadDir != "/tmp" {
		t.Errorf("Expected downloadDir to be '/tmp', got '%s'", service.downloadDir)
	}

	if service.maxParallel != 2 {
		t.Errorf("Expected maxParallel to be 2, got %d", service.maxParallel)
	}

	if len(service.tasks) != 0 {
		t.Errorf("Expected empty tasks map, got %d items", len(service.tasks))
	}
}

func TestAddTask(t *testing.T) {
	service := NewService("/tmp", 1)

	// Add first task
	task1, err := service.AddTask("https://youtube.com/watch?v=test1")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task1.URL != "https://youtube.com/watch?v=test1" {
		t.Errorf("Expected URL to be 'https://youtube.com/watch?v=test1', got '%s'", task1.URL)
	}

	if task1.Status != model.TaskStatusPending && task1.Status != model.TaskStatusStarting {
		t.Errorf("Expected status to be Pending or Starting, got %s", task1.Status)
	}

	// Try to add duplicate task (should fail)
	_, err = service.AddTask("https://youtube.com/watch?v=test1")
	if err == nil {
		t.Error("Expected error for duplicate URL, got nil")
	}

	// Add different task (should succeed)
	task2, err := service.AddTask("https://youtube.com/watch?v=test2")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if task2.URL != "https://youtube.com/watch?v=test2" {
		t.Errorf("Expected URL to be 'https://youtube.com/watch?v=test2', got '%s'", task2.URL)
	}
}

func TestGetTask(t *testing.T) {
	service := NewService("/tmp", 1)

	// Add a task
	task, err := service.AddTask("https://youtube.com/watch?v=test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Get existing task
	retrievedTask, exists := service.GetTask(task.ID)
	if !exists {
		t.Error("Expected task to exist")
	}

	if retrievedTask.ID != task.ID {
		t.Errorf("Expected task ID to be '%s', got '%s'", task.ID, retrievedTask.ID)
	}

	// Get non-existing task
	_, exists = service.GetTask("non-existing-id")
	if exists {
		t.Error("Expected task to not exist")
	}
}

func TestGetAllTasks(t *testing.T) {
	service := NewService("/tmp", 2)

	// Initially empty
	tasks := service.GetAllTasks()
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}

	// Add some tasks with different URLs
	task1, err1 := service.AddTask("https://youtube.com/watch?v=test1")
	if err1 != nil {
		t.Fatalf("Failed to add first task: %v", err1)
	}

	task2, err2 := service.AddTask("https://youtube.com/watch?v=test2")
	if err2 != nil {
		t.Fatalf("Failed to add second task: %v", err2)
	}

	// Wait a bit for tasks to be processed and check multiple times
	maxAttempts := 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		tasks = service.GetAllTasks()
		if len(tasks) == 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d after waiting", len(tasks))
		return
	}

	// Verify task IDs are present
	foundTask1 := false
	foundTask2 := false
	for _, task := range tasks {
		if task.ID == task1.ID {
			foundTask1 = true
		}
		if task.ID == task2.ID {
			foundTask2 = true
		}
	}

	if !foundTask1 {
		t.Error("Task 1 not found in results")
	}
	if !foundTask2 {
		t.Error("Task 2 not found in results")
	}
}

func TestUpdateCallback(t *testing.T) {
	service := NewService("/tmp", 1)

	updateCalled := false
	var updatedTask *model.DownloadTask

	service.SetUpdateCallback(func(task *model.DownloadTask) {
		updateCalled = true
		updatedTask = task
	})

	// Create a test task
	task := &model.DownloadTask{
		ID:     "test-id",
		URL:    "https://youtube.com/watch?v=test",
		Status: model.TaskStatusDownloading,
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
	time.Sleep(1 * time.Millisecond)
	id2 := generateTaskID()

	if id1 == id2 {
		t.Error("Expected different task IDs")
	}

	if id1 == "" || id2 == "" {
		t.Error("Expected non-empty task IDs")
	}
}
