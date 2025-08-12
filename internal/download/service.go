package download

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lrstanley/go-ytdlp"
	"github.com/romanitalian/yt-downloader/internal/model"
)

// Service handles download operations
type Service struct {
	tasks       map[string]*model.DownloadTask
	tasksMutex  sync.RWMutex
	maxParallel int
	activeCount int
	downloadDir string
	onUpdate    func(*model.DownloadTask) // callback for UI updates
}

// NewService creates a new download service
func NewService(downloadDir string, maxParallel int) *Service {
	return &Service{
		tasks:       make(map[string]*model.DownloadTask),
		maxParallel: maxParallel,
		downloadDir: downloadDir,
	}
}

// SetUpdateCallback sets the callback function for task updates
func (s *Service) SetUpdateCallback(callback func(*model.DownloadTask)) {
	s.onUpdate = callback
}

// AddTask adds a new download task
func (s *Service) AddTask(url string) (*model.DownloadTask, error) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	// Check for duplicate URLs
	for _, task := range s.tasks {
		if task.URL == url && !task.Status.IsFinished() {
			return nil, fmt.Errorf("task already exists for URL: %s", url)
		}
	}

	task := &model.DownloadTask{
		ID:        generateTaskID(),
		URL:       url,
		Status:    model.TaskStatusPending,
		Progress:  0.0,
		Percent:   0,
		ETASec:    -1,
		StartedAt: time.Now(),
	}

	s.tasks[task.ID] = task

	// Try to start task if we have capacity
	if s.activeCount < s.maxParallel {
		go s.startTask(task)
	}

	return task, nil
}

// GetTask returns a task by ID
func (s *Service) GetTask(id string) (*model.DownloadTask, bool) {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()
	task, exists := s.tasks[id]
	return task, exists
}

// GetAllTasks returns all tasks
func (s *Service) GetAllTasks() []*model.DownloadTask {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()

	tasks := make([]*model.DownloadTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// StopTask stops a running task
func (s *Service) StopTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	if !task.Status.IsActive() {
		return fmt.Errorf("task is not active: %s", task.Status)
	}

	// Set stopping status
	task.Status = model.TaskStatusStopping
	s.notifyUpdate(task)

	// The actual stopping will be handled in the task goroutine
	return nil
}

// startTask starts downloading a task
func (s *Service) startTask(task *model.DownloadTask) {
	s.tasksMutex.Lock()
	s.activeCount++
	task.Status = model.TaskStatusStarting
	s.tasksMutex.Unlock()

	s.notifyUpdate(task)

	defer func() {
		s.tasksMutex.Lock()
		s.activeCount--
		s.tasksMutex.Unlock()

		// Try to start next pending task
		s.startNextPendingTask()
	}()

	// Update status to downloading
	s.tasksMutex.Lock()
	task.Status = model.TaskStatusDownloading
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Monitor for stop requests
	go func() {
		for {
			s.tasksMutex.RLock()
			status := task.Status
			s.tasksMutex.RUnlock()

			if status == model.TaskStatusStopping {
				cancel()
				return
			}
			if status.IsFinished() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Configure yt-dlp
	dl := ytdlp.New().
		ForceOverwrites().
		RestrictFilenames().
		Output(s.downloadDir + "/%(title)s.%(ext)s")

	// Setup progress callback
	dl.ProgressFunc(500*time.Millisecond, func(update ytdlp.ProgressUpdate) {
		s.updateTaskProgress(task, &update)
	})

	// Start download with retry logic
	result, err := s.downloadWithRetry(ctx, dl, task)

	// Update final status
	s.tasksMutex.Lock()
	if err != nil {
		if ctx.Err() == context.Canceled {
			task.Status = model.TaskStatusStopped
		} else {
			task.Status = model.TaskStatusError
			task.LastError = err.Error()
		}
	} else {
		task.Status = model.TaskStatusCompleted
		task.Progress = 1.0
		task.Percent = 100

		// Set output path from result
		if result != nil {
			info, err := result.GetExtractedInfo()
			if err == nil && len(info) > 0 {
				// Get the first downloaded file
				if info[0].Filename != nil {
					task.OutputPath = *info[0].Filename
				}
			}
		}
	}
	task.FinishedAt = time.Now()
	s.tasksMutex.Unlock()

	s.notifyUpdate(task)
}

// downloadWithRetry attempts download with retry logic
func (s *Service) downloadWithRetry(ctx context.Context, dl *ytdlp.Command, task *model.DownloadTask) (*ytdlp.Result, error) {
	maxRetries := 1
	var lastErr error
	var result *ytdlp.Result

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Backoff delay
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			log.Printf("Retrying download for task %s, attempt %d", task.ID, attempt+1)
		}

		// Attempt download
		res, err := dl.Run(ctx, task.URL)
		if err == nil {
			return res, nil
		}

		lastErr = err
		result = res // Keep last result even if there was an error
		log.Printf("Download attempt %d failed for task %s: %v", attempt+1, task.ID, err)

		// Check if we should retry
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	return result, lastErr
}

// updateTaskProgress updates task progress from yt-dlp info
func (s *Service) updateTaskProgress(task *model.DownloadTask, update *ytdlp.ProgressUpdate) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	// Update percentage
	if update.TotalBytes > 0 {
		percent := float64(update.DownloadedBytes) / float64(update.TotalBytes) * 100
		task.Percent = int(percent)
		task.Progress = percent / 100.0
	}

	// Calculate speed
	if !update.Started.IsZero() {
		elapsed := time.Since(update.Started)
		if elapsed.Seconds() > 0 {
			bytesPerSecond := float64(update.DownloadedBytes) / elapsed.Seconds()
			task.Speed = fmt.Sprintf("%.1fMB/s", bytesPerSecond/1024/1024)
		}
	}

	// Calculate ETA
	eta := update.ETA()
	if eta > 0 {
		task.ETASec = int(eta.Seconds())
	}

	// Update title if available
	if update.Info != nil && update.Info.Title != nil && *update.Info.Title != "" && task.Title == "" {
		task.Title = *update.Info.Title
	}

	s.notifyUpdate(task)
}

// startNextPendingTask starts the next pending task if we have capacity
func (s *Service) startNextPendingTask() {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	if s.activeCount >= s.maxParallel {
		return
	}

	// Find next pending task
	for _, task := range s.tasks {
		if task.Status == model.TaskStatusPending {
			go s.startTask(task)
			return
		}
	}
}

// notifyUpdate calls the update callback if set
func (s *Service) notifyUpdate(task *model.DownloadTask) {
	if s.onUpdate != nil {
		s.onUpdate(task)
	}
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}
