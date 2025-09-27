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
	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/yt-downloader/internal/platform"
	"github.com/ytget/ytdlp"
	"github.com/ytget/ytdlp/types"
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

	// Quality preset: "best" | "medium" | "audio"
	qualityPreset string

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

	// Smoothing state for UI updates (1 second intervals)
	smoothingState map[string]*SmoothingState
	smoothingMutex sync.RWMutex

	// stopModes remembers whether a stop request was a pause or a hard stop
	stopModes map[string]StopMode
}

// SmoothingState holds data for smoothing UI updates over 1-second intervals
type SmoothingState struct {
	// Raw measurements accumulated over 1 second
	speedMeasurements   []float64 // MB/s measurements
	percentMeasurements []int     // percent measurements

	// Timing
	lastUpdate time.Time
	timer      *time.Timer

	// Current smoothed values
	smoothedSpeed   string // formatted speed string
	smoothedPercent int    // smoothed percent

	// Mutex for thread safety
	mutex sync.Mutex
}

// addMeasurement adds a new speed and percent measurement
func (ss *SmoothingState) addMeasurement(speedMBps float64, percent int) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	ss.speedMeasurements = append(ss.speedMeasurements, speedMBps)
	ss.percentMeasurements = append(ss.percentMeasurements, percent)

	// Keep only last 10 measurements to prevent memory growth
	if len(ss.speedMeasurements) > 10 {
		ss.speedMeasurements = ss.speedMeasurements[1:]
		ss.percentMeasurements = ss.percentMeasurements[1:]
	}
}

// calculateSmoothedValues calculates and returns smoothed speed and percent
func (ss *SmoothingState) calculateSmoothedValues() (string, int) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	if len(ss.speedMeasurements) == 0 {
		return "", 0
	}

	// Calculate average speed
	var totalSpeed float64
	for _, speed := range ss.speedMeasurements {
		totalSpeed += speed
	}
	avgSpeed := totalSpeed / float64(len(ss.speedMeasurements))

	// Calculate average percent
	var totalPercent int
	for _, percent := range ss.percentMeasurements {
		totalPercent += percent
	}
	avgPercent := totalPercent / len(ss.percentMeasurements)

	// Format speed string
	var speedStr string
	if avgSpeed > 0 {
		speedStr = fmt.Sprintf("%.1fMB/s", avgSpeed)
	}

	// Store smoothed values
	ss.smoothedSpeed = speedStr
	ss.smoothedPercent = avgPercent

	return speedStr, avgPercent
}

// clearMeasurements clears accumulated measurements
func (ss *SmoothingState) clearMeasurements() {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	ss.speedMeasurements = ss.speedMeasurements[:0]
	ss.percentMeasurements = ss.percentMeasurements[:0]
}

// startSmoothingTimer starts the 1-second smoothing timer for a task
func (s *Service) startSmoothingTimer(taskID string) {
	s.smoothingMutex.Lock()
	defer s.smoothingMutex.Unlock()

	// Create smoothing state if it doesn't exist
	if s.smoothingState[taskID] == nil {
		s.smoothingState[taskID] = &SmoothingState{
			speedMeasurements:   make([]float64, 0, 10),
			percentMeasurements: make([]int, 0, 10),
		}
	}

	state := s.smoothingState[taskID]
	state.mutex.Lock()
	defer state.mutex.Unlock()

	// Stop existing timer if any
	if state.timer != nil {
		state.timer.Stop()
	}

	// Start new timer
	state.timer = time.AfterFunc(time.Second, func() {
		s.updateSmoothedUI(taskID)
	})
}

// stopSmoothingTimer stops the smoothing timer for a task
func (s *Service) stopSmoothingTimer(taskID string) {
	s.smoothingMutex.Lock()
	defer s.smoothingMutex.Unlock()

	if state, ok := s.smoothingState[taskID]; ok {
		state.mutex.Lock()
		if state.timer != nil {
			state.timer.Stop()
			state.timer = nil
		}
		state.mutex.Unlock()
	}
}

// updateSmoothedUI updates UI with smoothed values and restarts timer
func (s *Service) updateSmoothedUI(taskID string) {
	s.smoothingMutex.RLock()
	state, exists := s.smoothingState[taskID]
	s.smoothingMutex.RUnlock()

	if !exists {
		return
	}

	// Calculate smoothed values
	speedStr, percent := state.calculateSmoothedValues()

	// Update task with smoothed values
	s.tasksMutex.Lock()
	if task, ok := s.tasks[taskID]; ok {
		if speedStr != "" {
			task.Speed = speedStr
		}
		if percent > 0 {
			task.Percent = percent
			task.Progress = float64(percent) / 100.0
		}
		s.notifyUpdate(task)
	}
	s.tasksMutex.Unlock()

	// Clear measurements for next interval
	state.clearMeasurements()

	// Restart timer if task is still downloading
	s.tasksMutex.RLock()
	task, exists := s.tasks[taskID]
	s.tasksMutex.RUnlock()

	if exists && task.Status == model.TaskStatusDownloading {
		state.mutex.Lock()
		if state.timer != nil {
			state.timer.Stop()
		}
		state.timer = time.AfterFunc(time.Second, func() {
			s.updateSmoothedUI(taskID)
		})
		state.mutex.Unlock()
	}
}

// NewService creates a new download service
func NewService(downloadDir string, maxParallel int) Downloader {
	return &Service{
		tasks:         make(map[string]*model.DownloadTask),
		maxParallel:   maxParallel,
		downloadDir:   downloadDir,
		qualityPreset: "best",

		// Playlist support
		playlists:           make(map[string]*model.Playlist),
		playlistQueue:       make(chan *model.Playlist, 10),
		maxPlaylistParallel: 3, // Limit concurrent playlist downloads

		progressState: make(map[string]struct {
			lastBytes int64
			lastAt    time.Time
		}),

		smoothingState: make(map[string]*SmoothingState),

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
// The native Go engine will automatically continue from .part files thanks to the Continue() flag.
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

	// Start smoothing timer for this task
	s.startSmoothingTimer(task.ID)

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

	// Configure new ytdlp downloader (pure Go)
	quality := "best"
	ext := ""
	switch s.qualityPreset {
	case "best":
		quality, ext = "best", ""
	case "medium":
		quality, ext = "height<=480", ""
	case "audio":
		quality, ext = "best", "" // MVP: keep progressive best; audio-only later
	}

	d := ytdlp.New().WithFormat(quality, ext)

	// Resolve metadata and select format to compute output path
	_, info, resErr := d.ResolveURL(ctx, task.URL)
	if resErr != nil {
		// Handle ResolveURL error immediately
		log.Printf("ResolveURL failed for task %s: %v", task.ID, resErr)

		s.tasksMutex.Lock()
		task.Status = model.TaskStatusError
		task.LastError = s.parseYouTubeError(resErr)
		task.FinishedAt = time.Now()
		s.tasksMutex.Unlock()
		s.notifyUpdate(task)
		return
	}

	// Compute output file path
	outputPath := s.downloadDir
	if info != nil {
		base := strings.TrimSpace(info.Title)
		if base == "" {
			base = "video"
		}
		base = s.sanitizeFilename(base)
		extGuess := s.guessExtFromFormats(info.Formats)
		if extGuess == "" {
			extGuess = "mp4"
		}
		outputPath = filepath.Join(s.downloadDir, base+"."+extGuess)
	}

	// If file already exists and has size, short-circuit
	if fi, statErr := os.Stat(outputPath); statErr == nil && fi.Size() > 0 {
		s.tasksMutex.Lock()
		task.Status = model.TaskStatusCompleted
		task.Progress = 1.0
		task.Percent = 100
		task.OutputPath = outputPath
		s.tasksMutex.Unlock()
		s.notifyUpdate(task)
		return
	}

	// Set progress callback and output path
	d = d.WithOutputPath(outputPath).WithProgress(func(p ytdlp.Progress) {
		s.updateTaskProgressFromNew(task, p)
	})

	// Expose expected output early (for UI actions like copy path)
	s.tasksMutex.Lock()
	task.OutputPath = outputPath
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	// Start download
	info, err := d.Download(ctx, task.URL)

	// Update final status
	s.tasksMutex.Lock()
	if err != nil {
		mode, wasStopped := s.stopModes[task.ID]
		if wasStopped {
			if mode == StopModePause {
				task.Status = model.TaskStatusPaused
			} else {
				task.Status = model.TaskStatusStopped
			}
			delete(s.stopModes, task.ID)
		} else if ctx.Err() == context.Canceled {
			if mode := s.stopModes[task.ID]; mode == StopModePause {
				task.Status = model.TaskStatusPaused
			} else {
				task.Status = model.TaskStatusStopped
			}
			delete(s.stopModes, task.ID)
		} else {
			task.Status = model.TaskStatusError
			task.LastError = s.parseYouTubeError(err)
		}
	} else {
		task.Status = model.TaskStatusCompleted
		task.Progress = 1.0
		task.Percent = 100
		// Derive final output path: if WithOutputPath was a directory, library created file inside
		if task.OutputPath == "" {
			// Attempt to build absolute path using info.Title and default ext; real path captured during progress if possible
			if info != nil && strings.TrimSpace(info.Title) != "" {
				// Leave empty; progress callback may have set OutputPath via probing; we will resolve on disk scan below
			}
		}
		// Resolve absolute/clean path if present
		if task.OutputPath != "" {
			if !filepath.IsAbs(task.OutputPath) {
				if abs, e := filepath.Abs(task.OutputPath); e == nil {
					task.OutputPath = abs
				}
			}
			task.OutputPath = filepath.Clean(task.OutputPath)
		}
		// Update title from info
		if info != nil && info.Title != "" && (task.Title == "" || strings.HasPrefix(task.Title, "http")) {
			task.Title = info.Title
		}

		// Notify Android media scanner about the new file
		// This makes downloaded videos appear in the Gallery app
		if task.OutputPath != "" {
			platform.NotifyMediaScanner(task.OutputPath)
		}
	}
	task.FinishedAt = time.Now()
	s.tasksMutex.Unlock()

	// Stop smoothing timer for this task
	s.stopSmoothingTimer(task.ID)

	s.notifyUpdate(task)
}

// Replace old progress updater with one that accepts new Progress
func (s *Service) updateTaskProgressFromNew(task *model.DownloadTask, p ytdlp.Progress) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	// Calculate current percent
	var currentPercent int
	if p.TotalSize > 0 {
		percent := float64(p.DownloadedSize) / float64(p.TotalSize) * 100
		currentPercent = int(percent)
		task.Progress = percent / 100.0
	} else if p.DownloadedSize > 0 {
		currentPercent = min(int(float64(p.DownloadedSize)/1024/1024), 99)
		task.Progress = float64(currentPercent) / 100.0
	}

	// Calculate current speed using delta snapshots
	now := time.Now()
	var currentSpeedMBps float64
	if state, ok := s.progressState[task.ID]; ok && !state.lastAt.IsZero() {
		deltaBytes := p.DownloadedSize - state.lastBytes
		deltaTime := now.Sub(state.lastAt)
		if deltaBytes > 0 && deltaTime > 0 {
			bps := float64(deltaBytes) / deltaTime.Seconds()
			if bps > 0 {
				currentSpeedMBps = bps / 1024 / 1024
			}
		}
	}
	s.progressState[task.ID] = struct {
		lastBytes int64
		lastAt    time.Time
	}{lastBytes: p.DownloadedSize, lastAt: now}

	// Add measurements to smoothing state
	s.smoothingMutex.RLock()
	if smoothingState, exists := s.smoothingState[task.ID]; exists {
		smoothingState.addMeasurement(currentSpeedMBps, currentPercent)
	}
	s.smoothingMutex.RUnlock()

	// Calculate ETA (not smoothed, as it's less critical)
	if p.TotalSize > 0 {
		if state, ok := s.progressState[task.ID]; ok && !state.lastAt.IsZero() {
			deltaBytes := p.DownloadedSize - state.lastBytes
			deltaTime := now.Sub(state.lastAt)
			if deltaBytes > 0 && deltaTime > 0 {
				bps := float64(deltaBytes) / deltaTime.Seconds()
				remaining := int64(p.TotalSize) - p.DownloadedSize
				if bps > 0 && remaining > 0 {
					task.ETASec = int(float64(remaining) / bps)
				}
			}
		}
	}

	// Don't call notifyUpdate here - it will be called by the smoothing timer
}

// downloadWithRetry is a compatibility shim; the new downloader returns VideoInfo.
func (s *Service) downloadWithRetry(ctx context.Context, d *ytdlp.Downloader, task *model.DownloadTask) (*ytdlp.VideoInfo, error) {
	return d.Download(ctx, task.URL)
}

// updateTaskProgress is kept for compatibility; delegates to the new handler.
func (s *Service) updateTaskProgress(task *model.DownloadTask, p ytdlp.Progress) {
	s.updateTaskProgressFromNew(task, p)
}

// guessExtFromFormats returns a preferred extension based on available formats.
func (s *Service) guessExtFromFormats(list []types.Format) string {
	for _, f := range list {
		if strings.Contains(strings.ToLower(f.MimeType), "video/mp4") {
			return "mp4"
		}
	}
	for _, f := range list {
		if strings.Contains(strings.ToLower(f.MimeType), "video/webm") {
			return "webm"
		}
	}
	return ""
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
//
//lint:ignore U1000 reserved for future rename feature
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
//
//lint:ignore U1000 reserved for future rename feature
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

// parseYouTubeError converts YouTube-specific errors to user-friendly messages
func (s *Service) parseYouTubeError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()
	errStr = strings.ToLower(errStr)

	// Age restriction
	if strings.Contains(errStr, "age restricted") || strings.Contains(errStr, "age-restricted") {
		return "Video is age-restricted and cannot be downloaded"
	}

	// Geographic restriction
	if strings.Contains(errStr, "geo") && strings.Contains(errStr, "block") {
		return "Video is not available in your region"
	}

	// Private video
	if strings.Contains(errStr, "private") {
		return "Video is private and cannot be downloaded"
	}

	// Deleted video
	if strings.Contains(errStr, "deleted") || strings.Contains(errStr, "unavailable") {
		return "Video has been deleted or is unavailable"
	}

	// Copyright issues
	if strings.Contains(errStr, "copyright") || strings.Contains(errStr, "blocked") {
		return "Video is blocked due to copyright restrictions"
	}

	// Network issues
	if strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") {
		return "Network error - please check your connection"
	}

	// Authentication issues
	if strings.Contains(errStr, "auth") || strings.Contains(errStr, "login") {
		return "Authentication required - video may be private"
	}

	// Return original error if no specific pattern matches
	return err.Error()
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
		for range time.NewTicker(100 * time.Millisecond).C {
			if task.Status.IsFinished() {
				if task.Status == model.TaskStatusCompleted {
					playlist.UpdateVideoOutputPath(video.ID, task.OutputPath, task.FileSize)
					playlist.UpdateVideoStatus(video.ID, model.VideoStatusCompleted)
					playlist.UpdateVideoProgress(video.ID, 100.0)
				} else {
					playlist.UpdateVideoStatus(video.ID, model.VideoStatusError)
					video.Error = task.LastError
				}
				return
			}
			playlist.UpdateVideoProgress(video.ID, task.Progress)
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

// SetQualityPreset sets quality preset for downloads
func (s *Service) SetQualityPreset(preset string) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	switch preset {
	case "best", "medium", "audio":
		s.qualityPreset = preset
	default:
		s.qualityPreset = "best"
	}
}

// SetMaxParallelDownloads sets the maximum number of parallel downloads
func (s *Service) SetMaxParallelDownloads(max int) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	if max < 1 {
		max = 1
	}
	if max > 10 {
		max = 10
	}

	s.maxParallel = max
}

// SetDownloadDirectory sets the download directory
func (s *Service) SetDownloadDirectory(dir string) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	s.downloadDir = dir
}
