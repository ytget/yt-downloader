package download

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lrstanley/go-ytdlp"
	"github.com/ytget/yt-downloader/internal/model"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Service handles download operations
type Service struct {
	tasks       map[string]*model.DownloadTask
	tasksMutex  sync.RWMutex
	maxParallel int
	activeCount int
	downloadDir string
	onUpdate    func(*model.DownloadTask) // callback for UI updates

	// Playlist support
	playlists           map[string]*model.Playlist
	playlistsMutex      sync.RWMutex
	playlistQueue       chan *model.Playlist
	maxPlaylistParallel int

	// Internal progress state for speed calculation (delta-based)
	progressState map[string]struct {
		lastBytes int64
		lastAt    time.Time
	}

	// stopModes remembers whether a stop request was a pause or a hard stop
	stopModes map[string]StopMode
}

// NewService creates a new download service
func NewService(downloadDir string, maxParallel int) Downloader {
	return &Service{
		tasks:       make(map[string]*model.DownloadTask),
		maxParallel: maxParallel,
		downloadDir: downloadDir,

		// Playlist support
		playlists:           make(map[string]*model.Playlist),
		playlistQueue:       make(chan *model.Playlist, 10),
		maxPlaylistParallel: 3, // Limit concurrent playlist downloads

		progressState: make(map[string]struct {
			lastBytes int64
			lastAt    time.Time
		}),

		stopModes: make(map[string]StopMode),
	}
}

// StopMode indicates intent behind cancellation
type StopMode int

const (
	StopModeNone StopMode = iota
	StopModePause
	StopModeStop
)

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
		Title:     "", // Leave empty; UI will fallback to URL until real title arrives
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

// GetTaskByVideoID returns a task by extracted YouTube video ID from its URL
// This is useful for playlist UI rows which use the plain YouTube ID as row identifier
func (s *Service) GetTaskByVideoID(videoID string) (*model.DownloadTask, bool) {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()

	for _, task := range s.tasks {
		if s.extractVideoID(task.URL) == videoID {
			return task, true
		}
	}
	return nil, false
}

// StopTask stops a task. If task is active it transitions to Stopping and ends as Stopped.
// If task is Pending or Paused it is transitioned to Stopped immediately.
func (s *Service) StopTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	log.Printf("StopTask called for task %s with status: %s", id, task.Status)

	// If task is pending, paused, or in error state, resolve to Stopped immediately
	if task.Status == model.TaskStatusPending ||
		task.Status == model.TaskStatusPaused ||
		task.Status == model.TaskStatusError {
		log.Printf("Task %s: converting from %s to Stopped immediately", id, task.Status)
		task.Status = model.TaskStatusStopped
		task.LastError = "" // Clear error when converting to stopped
		delete(s.stopModes, id)
		s.notifyUpdate(task)
		return nil
	}

	if !task.Status.IsActive() {
		log.Printf("Task %s: already in terminal state %s, no-op", id, task.Status)
		// Already in terminal state or not stoppable; treat as no-op
		return nil
	}

	log.Printf("Task %s: setting status to Stopping and marking as hard stop", id)
	// Set stopping status and mark mode as hard stop
	task.Status = model.TaskStatusStopping
	s.stopModes[id] = StopModeStop
	s.notifyUpdate(task)

	// The actual stopping will be handled in the task goroutine
	return nil
}

// PauseTask requests pausing a running task.
// Active tasks are transitioned to Stopping and will end as Paused.
func (s *Service) PauseTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	if !task.Status.IsActive() {
		// Not running; nothing to pause
		return nil
	}

	task.Status = model.TaskStatusStopping
	s.stopModes[id] = StopModePause
	s.notifyUpdate(task)
	return nil
}

// ResumeTask resumes a paused task.
// Paused tasks are transitioned to Pending and will start downloading again.
// yt-dlp will automatically continue from .part files thanks to the Continue() flag.
func (s *Service) ResumeTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status != model.TaskStatusPaused {
		return fmt.Errorf("task is not paused: %s", task.Status)
	}

	// Reset to pending state
	task.Status = model.TaskStatusPending
	task.StartedAt = time.Now() // Update start time for new attempt
	s.notifyUpdate(task)

	// Try to start if we have capacity
	if s.activeCount < s.maxParallel {
		go s.startTask(task)
	}

	return nil
}

// RestartTask restarts a failed or stopped task
func (s *Service) RestartTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	// Can only restart failed, stopped, paused, or pending tasks
	if task.Status != model.TaskStatusError &&
		task.Status != model.TaskStatusStopped &&
		task.Status != model.TaskStatusPaused &&
		task.Status != model.TaskStatusPending {
		return fmt.Errorf("task cannot be restarted in status: %s", task.Status)
	}

	// Reset task state
	task.Status = model.TaskStatusPending
	task.Progress = 0.0
	task.Percent = 0
	task.LastError = ""
	task.Speed = ""
	task.ETASec = -1
	task.StartedAt = time.Now()
	task.FinishedAt = time.Time{}

	s.notifyUpdate(task)

	// Try to start if we have capacity
	if s.activeCount < s.maxParallel {
		go s.startTask(task)
	}

	return nil
}

// RemoveTask removes a task from the service
func (s *Service) RemoveTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	// Stop the task if it's active
	if task.Status.IsActive() {
		task.Status = model.TaskStatusStopping
		s.notifyUpdate(task)
	}

	// Remove from tasks map
	delete(s.tasks, id)

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

	log.Printf("Starting download for task %s: %s", task.ID, task.URL)

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
				log.Printf("Task %s requested to stop, canceling download", task.ID)
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
		Continue().
		NoCheckCertificates().
		Output(s.downloadDir + "/%(title)s.%(ext)s").
		NoOverwrites() // Prevent duplicate files

	// Add format selection for better compatibility
	dl.Format("best[ext=mp4]/best[ext=webm]/best")

	// Pre-check: if file with final name already exists, mark as completed without running yt-dlp
	// We need the final name; try to construct it from Title if known, otherwise we rely on progress callbacks later
	if task.OutputPath != "" {
		if fi, err := os.Stat(task.OutputPath); err == nil && fi.Size() > 0 {
			s.tasksMutex.Lock()
			task.Status = model.TaskStatusCompleted
			task.Progress = 1.0
			task.Percent = 100
			s.tasksMutex.Unlock()
			s.notifyUpdate(task)
			return
		}
	}

	// Setup progress callback with more frequent updates
	// Use 1s interval to provide stable deltas for speed calculation
	dl.ProgressFunc(1*time.Second, func(update ytdlp.ProgressUpdate) {
		s.updateTaskProgress(task, &update)
	})

	// Start download with retry logic
	result, err := s.downloadWithRetry(ctx, dl, task)

	// Update final status
	s.tasksMutex.Lock()
	if err != nil {
		// Check if this was a user-initiated stop/pause
		mode, wasStopped := s.stopModes[task.ID]
		if wasStopped {
			if mode == StopModePause {
				task.Status = model.TaskStatusPaused
				log.Printf("Task %s was paused by user", task.ID)
			} else if mode == StopModeStop {
				task.Status = model.TaskStatusStopped
				log.Printf("Task %s was stopped by user", task.ID)
			}
			delete(s.stopModes, task.ID)
		} else if ctx.Err() == context.Canceled {
			// Fallback: check if context was canceled (should not happen with our current logic)
			mode := s.stopModes[task.ID]
			if mode == StopModePause {
				task.Status = model.TaskStatusPaused
				log.Printf("Task %s was paused by user (context canceled)", task.ID)
			} else {
				task.Status = model.TaskStatusStopped
				log.Printf("Task %s was stopped by user (context canceled)", task.ID)
			}
			delete(s.stopModes, task.ID)
		} else {
			task.Status = model.TaskStatusError
			task.LastError = err.Error()
			log.Printf("Task %s failed with error: %v", task.ID, err)
		}
	} else {
		task.Status = model.TaskStatusCompleted
		task.Progress = 1.0
		task.Percent = 100

		// Set output path from result
		if result != nil {
			log.Printf("Task %s: result is not nil, trying to get extracted info", task.ID)
			info, err := result.GetExtractedInfo()
			if err != nil {
				log.Printf("Task %s: failed to get extracted info: %v", task.ID, err)
			} else if len(info) == 0 {
				log.Printf("Task %s: extracted info is empty", task.ID)
			} else {
				log.Printf("Task %s: got %d info items", task.ID, len(info))
				if info[0].Filename != nil {
					task.OutputPath = *info[0].Filename
					log.Printf("Task %s completed successfully: %s", task.ID, task.OutputPath)
				} else {
					log.Printf("Task %s: filename is nil in info[0]", task.ID)
				}
				// Update title from extracted info if available and current title is empty or looks like URL
				if info[0].Title != nil {
					extractedTitle := *info[0].Title
					if extractedTitle != "" && (task.Title == "" || strings.HasPrefix(task.Title, "http")) {
						task.Title = extractedTitle
					}
				}
			}
		} else {
			log.Printf("Task %s: result is nil, no output path available", task.ID)
		}

		// Fallback: if OutputPath is still empty, try to construct it from download template
		if task.OutputPath == "" {
			// Try to get info from the download result even if there was an error
			if result != nil {
				if info, err := result.GetExtractedInfo(); err == nil && len(info) > 0 {
					if info[0].Filename != nil && *info[0].Filename != "" {
						task.OutputPath = *info[0].Filename
						log.Printf("Task %s: set OutputPath from fallback info: %s", task.ID, task.OutputPath)
					}
					if info[0].Title != nil {
						extractedTitle := *info[0].Title
						if extractedTitle != "" && (task.Title == "" || strings.HasPrefix(task.Title, "http")) {
							task.Title = extractedTitle
						}
					}
				}
			}

			// If still empty, try to construct from URL and download template
			if task.OutputPath == "" {
				// Extract video ID from URL for fallback filename
				videoID := s.extractVideoID(task.URL)
				if videoID != "" {
					// Use a reasonable default extension
					fallbackPath := fmt.Sprintf("%s/%s.mp4", s.downloadDir, videoID)
					task.OutputPath = fallbackPath
					log.Printf("Task %s: set fallback OutputPath: %s", task.ID, task.OutputPath)
				} else {
					log.Printf("Task %s: could not construct fallback OutputPath", task.ID)
				}
			}
		}

		// Validate and clean the OutputPath
		if task.OutputPath != "" {
			// Ensure the path is absolute and clean
			if !filepath.IsAbs(task.OutputPath) {
				absPath, err := filepath.Abs(task.OutputPath)
				if err == nil {
					task.OutputPath = absPath
				}
			}

			// Clean the path
			task.OutputPath = filepath.Clean(task.OutputPath)

			log.Printf("Task %s: final OutputPath: %s", task.ID, task.OutputPath)
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

	// Update percentage - handle cases where TotalBytes might be 0
	if update.TotalBytes > 0 {
		percent := float64(update.DownloadedBytes) / float64(update.TotalBytes) * 100
		task.Percent = int(percent)
		task.Progress = percent / 100.0
	} else if update.DownloadedBytes > 0 {
		// If TotalBytes is 0 but we have downloaded bytes, show some progress
		// This can happen with live streams or when yt-dlp doesn't provide total size
		task.Percent = min(int(float64(update.DownloadedBytes)/1024/1024), 99) // Max 99% until complete
		task.Progress = float64(task.Percent) / 100.0
	}

	// Calculate speed using delta between updates for better stability
	now := time.Now()
	if state, ok := s.progressState[task.ID]; ok && !state.lastAt.IsZero() {
		deltaBytes := int64(update.DownloadedBytes) - state.lastBytes
		deltaTime := now.Sub(state.lastAt)
		if deltaBytes > 0 && deltaTime > 0 {
			bytesPerSecond := float64(deltaBytes) / deltaTime.Seconds()
			if bytesPerSecond > 0 {
				task.Speed = fmt.Sprintf("%.1fMB/s", bytesPerSecond/1024/1024)
			}
		}
	}
	// Update progress state snapshot
	s.progressState[task.ID] = struct {
		lastBytes int64
		lastAt    time.Time
	}{lastBytes: int64(update.DownloadedBytes), lastAt: now}

	// Calculate ETA
	eta := update.ETA()
	if eta > 0 {
		task.ETASec = int(eta.Seconds())
	} else if update.TotalBytes > 0 {
		if state, ok := s.progressState[task.ID]; ok && !state.lastAt.IsZero() {
			deltaBytes := int64(update.DownloadedBytes) - state.lastBytes
			deltaTime := now.Sub(state.lastAt)
			if deltaBytes > 0 && deltaTime > 0 {
				bytesPerSecond := float64(deltaBytes) / deltaTime.Seconds()
				remaining := int64(update.TotalBytes) - int64(update.DownloadedBytes)
				if bytesPerSecond > 0 && remaining > 0 {
					task.ETASec = int(float64(remaining) / bytesPerSecond)
				}
			}
		}
	}

	// Update title if available
	if update.Info != nil && update.Info.Title != nil && *update.Info.Title != "" && task.Title == "" {
		task.Title = *update.Info.Title
	}

	// Update OutputPath if available and not set yet
	if task.OutputPath == "" && update.Info != nil && update.Info.Filename != nil && *update.Info.Filename != "" {
		task.OutputPath = *update.Info.Filename
		log.Printf("Task %s: updated OutputPath during download: %s", task.ID, task.OutputPath)
	}

	// If file already exists (likely due to NoOverwrites and already downloaded), reflect 100% immediately
	if task.OutputPath != "" {
		if fi, err := os.Stat(task.OutputPath); err == nil {
			// Heuristic: if yt-dlp hasn't reported any bytes yet but the final file exists,
			// treat it as already downloaded and set 100% for UI consistency
			if update.DownloadedBytes == 0 {
				// If total size is known and file size >= total, definitely 100%
				if update.TotalBytes > 0 {
					if fi.Size() >= int64(update.TotalBytes) {
						task.Percent = 100
						task.Progress = 1.0
						task.Speed = ""
						task.ETASec = 0
					}
				} else {
					// Total unknown: the presence of final file strongly suggests skip
					task.Percent = 100
					task.Progress = 1.0
					task.Speed = ""
					task.ETASec = 0
				}
			}
		}
	}

	// Log progress for debugging
	log.Printf("Task %s progress: %d%%, downloaded: %d bytes, speed: %s",
		task.ID, task.Percent, update.DownloadedBytes, task.Speed)

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

// generateTaskID generates a unique task ID using UUID v7 for better uniqueness and time ordering
func generateTaskID() string {
	// Use UUID v7 which includes timestamp and is naturally ordered
	// This provides better uniqueness and allows for chronological sorting
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to timestamp if UUID generation fails
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task-" + id.String()
}

// extractVideoID extracts video ID from various YouTube URL formats
func (s *Service) extractVideoID(url string) string {
	// Simple extraction for common YouTube URL patterns
	if strings.Contains(url, "youtube.com/watch?v=") {
		parts := strings.Split(url, "v=")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "&")[0]
			// Clean video ID from any additional parameters
			videoID = strings.Split(videoID, "#")[0]
			videoID = strings.Split(videoID, "/")[0]
			if len(videoID) == 11 { // YouTube video IDs are 11 characters
				return videoID
			}
		}
	} else if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "?")[0]
			videoID = strings.Split(videoID, "#")[0]
			if len(videoID) == 11 {
				return videoID
			}
		}
	}

	// For other URLs, try to extract some identifier
	if strings.Contains(url, "vimeo.com/") {
		parts := strings.Split(url, "vimeo.com/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "/")[0]
			videoID = strings.Split(videoID, "?")[0]
			videoID = strings.Split(videoID, "#")[0]
			if videoID != "" {
				return "vimeo_" + videoID
			}
		}
	}

	// Generic fallback - use hash of URL
	hash := fmt.Sprintf("%x", len(url))
	return "video_" + hash[:8]
}

// generateUserFriendlyPath generates a user-friendly file path with original title
func (s *Service) generateUserFriendlyPath(task *model.DownloadTask) string {
	if task.Title == "" || task.OutputPath == "" {
		return ""
	}

	// Get the directory and extension from the original OutputPath
	dir := filepath.Dir(task.OutputPath)
	ext := filepath.Ext(task.OutputPath)
	if ext == "" {
		ext = ".mp4" // Default extension
	}

	// Clean the title to make it safe for filenames
	safeTitle := s.sanitizeFilename(task.Title)

	// Create the new path
	newPath := filepath.Join(dir, safeTitle+ext)

	// Check if the file already exists with this name
	if _, err := os.Stat(newPath); err == nil {
		// File exists, add a number suffix
		counter := 1
		for {
			newPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", safeTitle, counter, ext))
			if _, err := os.Stat(newPath); err != nil {
				break
			}
			counter++
		}
	}

	return newPath
}

// sanitizeFilename removes or replaces characters that are not safe for filenames
func (s *Service) sanitizeFilename(filename string) string {
	// Replace unsafe characters
	unsafe := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename

	for _, char := range unsafe {
		result = strings.ReplaceAll(result, char, "_")
	}

	// Replace multiple spaces with single space
	result = strings.Join(strings.Fields(result), " ")

	// Replace spaces with underscores
	result = strings.ReplaceAll(result, " ", "_")

	// Limit length to avoid filesystem issues
	if len(result) > 200 {
		result = result[:200]
	}

	return result
}

// Playlist methods

// AddPlaylist adds a new playlist for downloading
func (s *Service) AddPlaylist(playlist *model.Playlist) error {
	s.playlistsMutex.Lock()
	defer s.playlistsMutex.Unlock()

	// Check for duplicate playlists
	if _, exists := s.playlists[playlist.ID]; exists {
		return fmt.Errorf("playlist already exists: %s", playlist.ID)
	}

	s.playlists[playlist.ID] = playlist

	// Add to download queue
	select {
	case s.playlistQueue <- playlist:
		// Successfully queued
	default:
		// Queue is full, start processing immediately
		go s.processPlaylist(playlist)
	}

	return nil
}

// GetPlaylist returns a playlist by ID
func (s *Service) GetPlaylist(id string) (*model.Playlist, bool) {
	s.playlistsMutex.RLock()
	defer s.playlistsMutex.RUnlock()
	playlist, exists := s.playlists[id]
	return playlist, exists
}

// GetAllPlaylists returns all playlists
func (s *Service) GetAllPlaylists() []*model.Playlist {
	s.playlistsMutex.RLock()
	defer s.playlistsMutex.RUnlock()

	playlists := make([]*model.Playlist, 0, len(s.playlists))
	for _, playlist := range s.playlists {
		playlists = append(playlists, playlist)
	}
	return playlists
}

// DownloadPlaylist starts downloading a playlist with chunked approach
func (s *Service) DownloadPlaylist(playlist *model.Playlist) error {
	if !playlist.IsReadyForDownload() {
		return fmt.Errorf("playlist is not ready for download")
	}

	// Update playlist status
	playlist.UpdateStatus(model.PlaylistStatusDownloading)

	// Start processing the playlist
	go s.processPlaylist(playlist)

	return nil
}

// processPlaylist processes a playlist by downloading videos in chunks
func (s *Service) processPlaylist(playlist *model.Playlist) {
	// Get pending videos
	pendingVideos := playlist.GetPendingVideos()
	if len(pendingVideos) == 0 {
		playlist.UpdateStatus(model.PlaylistStatusCompleted)
		return
	}

	// Process videos in chunks
	chunkSize := s.maxPlaylistParallel
	for i := 0; i < len(pendingVideos); i += chunkSize {
		end := min(i+chunkSize, len(pendingVideos))
		chunk := pendingVideos[i:end]

		// Download chunk of videos
		var wg sync.WaitGroup
		for _, video := range chunk {
			wg.Add(1)
			go func(v *model.PlaylistVideo) {
				defer wg.Done()
				s.downloadPlaylistVideo(playlist, v)
			}(video)
		}

		// Wait for current chunk to complete
		wg.Wait()

		// Check if playlist was cancelled
		if playlist.Status == model.PlaylistStatusError {
			return
		}
	}

	// Mark playlist as completed
	playlist.UpdateStatus(model.PlaylistStatusCompleted)
}

// downloadPlaylistVideo downloads a single video from a playlist
func (s *Service) downloadPlaylistVideo(playlist *model.Playlist, video *model.PlaylistVideo) {
	// Update video status
	playlist.UpdateVideoStatus(video.ID, model.VideoStatusDownloading)

	// Create download task for this video
	task, err := s.AddTask(video.URL)
	if err != nil {
		playlist.UpdateVideoStatus(video.ID, model.VideoStatusError)
		video.Error = err.Error()
		return
	}

	// Monitor task progress
	go func() {
		for {
			select {
			case <-time.After(100 * time.Millisecond):
				// Update video progress
				if task.Status.IsFinished() {
					if task.Status == model.TaskStatusCompleted {
						// Update video with task info including OutputPath
						playlist.UpdateVideoOutputPath(video.ID, task.OutputPath, task.FileSize)
						playlist.UpdateVideoStatus(video.ID, model.VideoStatusCompleted)
						playlist.UpdateVideoProgress(video.ID, 100.0)
					} else {
						playlist.UpdateVideoStatus(video.ID, model.VideoStatusError)
						video.Error = task.LastError
					}
					return
				}

				// Update progress
				playlist.UpdateVideoProgress(video.ID, task.Progress)
			}
		}
	}()
}

// CancelPlaylist cancels a playlist download
func (s *Service) CancelPlaylist(playlistID string) error {
	s.playlistsMutex.Lock()
	defer s.playlistsMutex.Unlock()

	playlist, exists := s.playlists[playlistID]
	if !exists {
		return fmt.Errorf("playlist not found: %s", playlistID)
	}

	// Update status
	playlist.UpdateStatus(model.PlaylistStatusError)

	// Cancel all downloading videos
	for _, video := range playlist.Videos {
		if video.Status == model.VideoStatusDownloading {
			playlist.UpdateVideoStatus(video.ID, model.VideoStatusSkipped)
		}
	}

	return nil
}

// SetMaxPlaylistParallel sets the maximum number of concurrent playlist downloads
func (s *Service) SetMaxPlaylistParallel(max int) {
	s.maxPlaylistParallel = max
}
