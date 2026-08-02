package download

import (
	"strings"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/ytdlp/v2"
)

func TestNewService(t *testing.T) {
	service := NewService("/tmp", 2).(*Service)

	if service.configSnapshot().DownloadDir != "/tmp" {
		t.Errorf("Expected download directory to be '/tmp', got '%s'", service.configSnapshot().DownloadDir)
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
	// Public reads must not expose the live worker-owned object.
	retrievedTask.Title = "mutated outside service"
	live, _ := service.GetTask(task.ID)
	if live.Title == retrievedTask.Title {
		t.Fatal("GetTask returned the live task instead of a snapshot")
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

	// Wait longer for tasks to be processed and check multiple times
	maxAttempts := 20
	for attempt := 0; attempt < maxAttempts; attempt++ {
		tasks = service.GetAllTasks()
		if len(tasks) == 2 {
			break
		}
		time.Sleep(100 * time.Millisecond) // Increased wait time
	}

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d after waiting", len(tasks))
		// Log current tasks for debugging
		for i, task := range tasks {
			t.Logf("Task %d: ID=%s, URL=%s, Status=%s", i, task.ID, task.URL, task.Status)
		}
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
	service := NewService("/tmp", 1).(*Service)

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

	if updatedTask == task || updatedTask.ID != task.ID {
		t.Error("Expected callback to receive an equivalent detached snapshot")
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()

	if id1 == id2 {
		t.Error("Expected different task IDs")
	}

	if id1 == "" || id2 == "" {
		t.Error("Expected non-empty task IDs")
	}

	// Check prefix
	if !strings.HasPrefix(id1, "task-") {
		t.Errorf("Expected ID to start with 'task-', got: %s", id1)
	}

	if !strings.HasPrefix(id2, "task-") {
		t.Errorf("Expected ID to start with 'task-', got: %s", id2)
	}

	// Check UUID format (task- + 36 chars for UUID)
	if len(id1) != len("task-")+36 {
		t.Errorf("Expected ID length %d, got %d for ID: %s", len("task-")+36, len(id1), id1)
	}

	if len(id2) != len("task-")+36 {
		t.Errorf("Expected ID length %d, got %d for ID: %s", len("task-")+36, len(id2), id2)
	}
}

func TestSmoothingState(t *testing.T) {
	// Test SmoothingState methods
	state := &SmoothingState{
		speedMeasurements:   make([]float64, 0, 10),
		percentMeasurements: make([]int, 0, 10),
	}

	// Add some test measurements
	state.addMeasurement(1.5, 25) // 1.5 MB/s, 25%
	state.addMeasurement(2.0, 30) // 2.0 MB/s, 30%
	state.addMeasurement(1.8, 35) // 1.8 MB/s, 35%

	// Calculate smoothed values
	speedStr, percent := state.calculateSmoothedValues()

	// Check speed (average of 1.5, 2.0, 1.8 = 1.77)
	expectedSpeed := "1.8MB/s"
	if speedStr != expectedSpeed {
		t.Errorf("Expected speed '%s', got '%s'", expectedSpeed, speedStr)
	}

	// Check percent (average of 25, 30, 35 = 30)
	expectedPercent := 30
	if percent != expectedPercent {
		t.Errorf("Expected percent %d, got %d", expectedPercent, percent)
	}

	// Test clearing measurements
	state.clearMeasurements()
	if len(state.speedMeasurements) != 0 {
		t.Errorf("Expected empty speed measurements after clear, got %d", len(state.speedMeasurements))
	}
	if len(state.percentMeasurements) != 0 {
		t.Errorf("Expected empty percent measurements after clear, got %d", len(state.percentMeasurements))
	}
}

func TestSmoothingIntegration(t *testing.T) {
	// maxParallel 0 avoids background runTask cleaning runtime state mid-test.
	service := NewService("/tmp", 0).(*Service)

	task := &model.DownloadTask{
		ID:     "task-smooth-1",
		URL:    "https://youtube.com/watch?v=test",
		Status: model.TaskStatusDownloading,
	}
	service.tasks[task.ID] = task

	// Start smoothing timer
	service.startSmoothingTimer(task.ID)

	// Simulate progress updates
	progress := ytdlp.Progress{
		TotalSize:      1000000, // 1MB
		DownloadedSize: 100000,  // 100KB
	}

	// Update progress multiple times to accumulate measurements
	for i := 0; i < 5; i++ {
		service.updateTaskProgressFromNew(task, progress)
		progress.DownloadedSize += 50000  // Add 50KB each time
		time.Sleep(10 * time.Millisecond) // Small delay
	}

	// Wait for smoothing timer to trigger
	time.Sleep(1100 * time.Millisecond)

	// Check that smoothing state exists
	service.smoothingMutex.RLock()
	state, exists := service.smoothingState[task.ID]
	service.smoothingMutex.RUnlock()

	if !exists {
		t.Error("Expected smoothing state to exist for task")
	}

	if state == nil {
		t.Error("Expected smoothing state to be non-nil")
	}

	// Stop smoothing timer
	service.stopSmoothingTimer(task.ID)
}

func TestDuplicateURLPolicy(t *testing.T) {
	service := NewService(t.TempDir(), 1).(*Service)
	url := "https://youtube.com/watch?v=dup-policy"

	task, err := service.AddTask(url)
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Active/pending duplicate rejected.
	if _, err := service.AddTask(url); err == nil {
		t.Fatal("expected duplicate error for open task")
	}

	// Mark completed (terminal) — re-download allowed.
	service.tasksMutex.Lock()
	task.Status = model.TaskStatusCompleted
	service.tasksMutex.Unlock()

	task2, err := service.AddTask(url)
	if err != nil {
		t.Fatalf("re-download after completed should be allowed: %v", err)
	}
	if task2.ID == task.ID {
		t.Fatal("expected a new task id for intentional re-download")
	}

	// Paused still owns URL.
	service.tasksMutex.Lock()
	task2.Status = model.TaskStatusPaused
	service.tasksMutex.Unlock()
	if _, err := service.AddTask(url); err == nil {
		t.Fatal("expected duplicate error while paused task exists")
	}
}

func TestRestartClearsMediaAndTranscriptFields(t *testing.T) {
	// maxParallel 0 prevents queueStartLocked from claiming/running network work.
	service := NewService(t.TempDir(), 0).(*Service)

	task := &model.DownloadTask{
		ID:                "task-restart-1",
		URL:               "https://youtube.com/watch?v=restart",
		Status:            model.TaskStatusError,
		Progress:          0.8,
		Percent:           80,
		LastError:         "boom",
		Warning:           "partial captions",
		OutputPath:        "/tmp/video.mp4",
		FileSize:          1234,
		Mode:              model.DownloadModeVideoAndTranscript,
		TranscriptStatus:  model.TranscriptStatusError,
		TranscriptSRTPath: "/tmp/video.en.srt",
		TranscriptTXTPath: "/tmp/video.en.txt",
		TranscriptError:   "no caps",
		Speed:             "1MB/s",
		ETASec:            9,
	}
	service.tasks[task.ID] = task

	if err := service.RestartTask(task.ID); err != nil {
		t.Fatalf("restart: %v", err)
	}

	got, ok := service.GetTask(task.ID)
	if !ok {
		t.Fatal("task missing after restart")
	}
	if got.Status != model.TaskStatusPending {
		t.Fatalf("expected Pending (not started with maxParallel=0), got %s", got.Status)
	}
	if got.OutputPath != "" || got.FileSize != 0 {
		t.Fatalf("media fields not cleared: path=%q size=%d", got.OutputPath, got.FileSize)
	}
	if got.TranscriptSRTPath != "" || got.TranscriptTXTPath != "" || got.TranscriptError != "" {
		t.Fatalf("transcript paths/error not cleared: %+v", got)
	}
	if got.Warning != "" || got.LastError != "" {
		t.Fatalf("warning/error not cleared: warn=%q err=%q", got.Warning, got.LastError)
	}
	if got.TranscriptStatus != model.TranscriptStatusPending {
		t.Fatalf("expected transcript pending, got %s", got.TranscriptStatus)
	}
	if got.Progress != 0 || got.Percent != 0 {
		t.Fatalf("progress not reset: %v %d", got.Progress, got.Percent)
	}
}

func TestRemoveWhileDownloadingCancelsAndFreesSlot(t *testing.T) {
	service := NewService(t.TempDir(), 1).(*Service)

	// Seed a fake in-flight task holding the only slot.
	task := &model.DownloadTask{
		ID:     "task-remove-inflight",
		URL:    "https://youtube.com/watch?v=remove-me",
		Status: model.TaskStatusDownloading,
	}
	service.tasksMutex.Lock()
	service.tasks[task.ID] = task
	service.activeCount = 1
	service.tasksMutex.Unlock()

	if err := service.RemoveTask(task.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if _, ok := service.GetTask(task.ID); ok {
		t.Fatal("task should be gone from map")
	}
	// Removal is signalled through stopModes so the worker can observe it
	// without racing a detached update snapshot.
	if task.Status != model.TaskStatusDownloading {
		t.Fatalf("remove should not mutate the worker-owned task, got %s", task.Status)
	}
	service.tasksMutex.RLock()
	mode := service.stopModes[task.ID]
	service.tasksMutex.RUnlock()
	if mode != StopModeStop {
		t.Fatalf("expected StopModeStop, got %v", mode)
	}

	// Simulate worker exit after cancel observes Stopping.
	service.finishWorker(task)

	service.tasksMutex.RLock()
	active := service.activeCount
	_, stopLeft := service.stopModes[task.ID]
	service.tasksMutex.RUnlock()
	if active != 0 {
		t.Fatalf("expected activeCount 0 after worker finish, got %d", active)
	}
	if stopLeft {
		t.Fatal("stopModes entry should be cleaned after worker finish")
	}

	// Slot free: a new pending task should be claimable.
	next := &model.DownloadTask{
		ID:     "task-next",
		URL:    "https://youtube.com/watch?v=next",
		Status: model.TaskStatusPending,
	}
	service.tasksMutex.Lock()
	service.tasks[next.ID] = next
	service.queueStartLocked(next)
	claimed := next.Status == model.TaskStatusStarting
	active = service.activeCount
	service.tasksMutex.Unlock()
	if !claimed || active != 1 {
		t.Fatalf("expected next task claimed with active=1, status=%s active=%d", next.Status, active)
	}
	// Prevent runTask network work from the claim goroutine affecting the test process:
	// mark removed so runTask exits immediately.
	service.tasksMutex.Lock()
	delete(service.tasks, next.ID)
	service.tasksMutex.Unlock()
	time.Sleep(30 * time.Millisecond)
}

func TestResumeKeepsOutputPathForPartial(t *testing.T) {
	service := NewService(t.TempDir(), 0).(*Service)
	task := &model.DownloadTask{
		ID:         "task-resume-1",
		URL:        "https://youtube.com/watch?v=resume",
		Status:     model.TaskStatusPaused,
		OutputPath: "/tmp/partial-video.mp4.part",
		Percent:    40,
	}
	service.tasks[task.ID] = task
	service.maxParallel = 0 // do not auto-start

	if err := service.ResumeTask(task.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if task.OutputPath != "/tmp/partial-video.mp4.part" {
		t.Fatalf("resume must keep partial output path, got %q", task.OutputPath)
	}
	if task.Status != model.TaskStatusPending {
		t.Fatalf("expected Pending after resume, got %s", task.Status)
	}
}

func TestPauseSetsStopModePause(t *testing.T) {
	service := NewService(t.TempDir(), 1).(*Service)
	task := &model.DownloadTask{
		ID:     "task-pause-1",
		URL:    "https://youtube.com/watch?v=pause",
		Status: model.TaskStatusDownloading,
	}
	service.tasks[task.ID] = task

	if err := service.PauseTask(task.ID); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if task.Status != model.TaskStatusStopping {
		t.Fatalf("expected Stopping, got %s", task.Status)
	}
	if service.stopModes[task.ID] != StopModePause {
		t.Fatalf("expected StopModePause")
	}
}

func TestRetryTranscriptRejectsActive(t *testing.T) {
	svc := NewService(t.TempDir(), 0).(*Service)
	task := &model.DownloadTask{
		ID:     "t-retry-active",
		URL:    "https://example.invalid/v",
		Status: model.TaskStatusDownloading,
		Mode:   model.DownloadModeVideoAndTranscript,
	}
	svc.tasks[task.ID] = task
	if err := svc.RetryTranscript(task.ID); err == nil {
		t.Fatal("expected error for active task")
	}
}

func TestRetryTranscriptQueuesForCompletedMedia(t *testing.T) {
	svc := NewService(t.TempDir(), 0).(*Service)
	task := &model.DownloadTask{
		ID:               "t-retry-ok",
		URL:              "https://example.invalid/v2",
		Status:           model.TaskStatusCompleted,
		Mode:             model.DownloadModeVideo,
		OutputPath:       "/tmp/video.mp4",
		Title:            "Hello",
		TranscriptStatus: model.TranscriptStatusUnavailable,
		Warning:          "no caps",
	}
	svc.tasks[task.ID] = task
	if err := svc.RetryTranscript(task.ID); err != nil {
		t.Fatal(err)
	}
	updated, ok := svc.GetTask(task.ID)
	if !ok {
		t.Fatal("task was removed")
	}
	if updated.Mode != model.DownloadModeVideoAndTranscript {
		t.Fatalf("mode promoted: %s", updated.Mode)
	}
	if updated.TranscriptStatus != model.TranscriptStatusPending {
		t.Fatalf("status=%s", updated.TranscriptStatus)
	}
	// Allow background goroutine to fail network quickly without panicking.
	time.Sleep(50 * time.Millisecond)
}
