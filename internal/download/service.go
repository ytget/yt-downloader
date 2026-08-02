package download

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ytget/yt-downloader/internal/applog"
	"github.com/ytget/yt-downloader/internal/captions"
	"github.com/ytget/yt-downloader/internal/fsutil"
	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/yt-downloader/internal/netutil"
	"github.com/ytget/yt-downloader/internal/platform"
	"github.com/ytget/yt-downloader/internal/transcript"
	"github.com/ytget/yt-downloader/internal/urlutil"
	"github.com/ytget/ytdlp/v2"
	"github.com/ytget/ytdlp/v2/types"
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
	onUpdate    func(*model.DownloadTask) // callback for UI updates

	// config contains settings used by workers. It is replaced as one snapshot
	// so workers never observe a partially applied settings update.
	config      ServiceConfig
	configMutex sync.RWMutex

	// Playlist support
	playlists           map[string]*model.Playlist
	playlistsMutex      sync.RWMutex
	playlistStateMutex  sync.RWMutex // protects mutable playlist and video state
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

	// Lifecycle
	shuttingDown    bool
	workerWG        sync.WaitGroup
	queueStore      *QueueStore
	persistSignal   chan struct{}
	persistStop     chan struct{}
	persistStopOnce sync.Once
	persistWG       sync.WaitGroup

	// Injectable engines (F1); default to ytdlp production impl.
	media    MediaEngine
	captions CaptionsEngine
}

// ServiceConfig contains runtime settings used by download workers.
type ServiceConfig struct {
	MaxParallel     int
	DownloadDir     string
	QualityPreset   string
	DefaultMode     model.DownloadMode
	TranscriptLang  string
	CollisionPolicy fsutil.CollisionPolicy
	DisableCaptions bool
	VideoOnlySafe   bool
}

func (s *Service) configSnapshot() ServiceConfig {
	s.configMutex.RLock()
	defer s.configMutex.RUnlock()
	return s.config
}

// ApplyConfig replaces all runtime download settings as one operation.
func (s *Service) ApplyConfig(cfg ServiceConfig) {
	if cfg.MaxParallel < 1 {
		cfg.MaxParallel = 1
	}
	if cfg.MaxParallel > 10 {
		cfg.MaxParallel = 10
	}
	cfg.QualityPreset = strings.ToLower(strings.TrimSpace(cfg.QualityPreset))
	if cfg.QualityPreset != "best" && cfg.QualityPreset != "medium" && cfg.QualityPreset != "audio" {
		cfg.QualityPreset = "best"
	}
	cfg.DefaultMode = normalizeMode(cfg.DefaultMode)
	cfg.TranscriptLang = strings.TrimSpace(strings.ToLower(cfg.TranscriptLang))
	if cfg.TranscriptLang == "auto" || cfg.TranscriptLang == "system" {
		cfg.TranscriptLang = ""
	}
	cfg.CollisionPolicy = fsutil.NormalizeCollisionPolicy(cfg.CollisionPolicy)

	s.tasksMutex.Lock()
	s.maxParallel = cfg.MaxParallel
	s.tasksMutex.Unlock()
	s.configMutex.Lock()
	s.config = cfg
	s.configMutex.Unlock()
}

// SmoothingState holds data for smoothing UI updates over 1-second intervals
type SmoothingState struct {
	// Raw measurements accumulated over 1 second
	speedMeasurements   []float64 // MB/s measurements
	percentMeasurements []int     // percent measurements

	// Timing
	timer *time.Timer

	// stopped prevents a fired timer callback from restarting after cleanup.
	stopped bool

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

	state.stopped = false
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
		state.stopped = true
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

	state.mutex.Lock()
	stopped := state.stopped
	state.mutex.Unlock()
	if stopped {
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

	// Restart timer if task is still downloading and smoothing was not stopped.
	s.tasksMutex.RLock()
	task, exists := s.tasks[taskID]
	stillDownloading := exists && task.Status == model.TaskStatusDownloading
	s.tasksMutex.RUnlock()

	state.mutex.Lock()
	defer state.mutex.Unlock()
	if state.stopped || !stillDownloading {
		return
	}
	if state.timer != nil {
		state.timer.Stop()
	}
	state.timer = time.AfterFunc(time.Second, func() {
		s.updateSmoothedUI(taskID)
	})
}

// NewService creates a new download service
func NewService(downloadDir string, maxParallel int) Downloader {
	store, err := NewQueueStore()
	if err != nil {
		applog.Warn("queue store unavailable", "err", err)
		store = nil
	}
	service := &Service{
		tasks:       make(map[string]*model.DownloadTask),
		maxParallel: maxParallel,
		config: ServiceConfig{
			MaxParallel:     maxParallel,
			DownloadDir:     downloadDir,
			QualityPreset:   "best",
			DefaultMode:     model.DownloadModeVideo,
			CollisionPolicy: fsutil.DefaultCollisionPolicy,
		},
		playlists:           make(map[string]*model.Playlist),
		playlistQueue:       make(chan *model.Playlist, 10),
		maxPlaylistParallel: 3,
		progressState: make(map[string]struct {
			lastBytes int64
			lastAt    time.Time
		}),
		smoothingState: make(map[string]*SmoothingState),
		stopModes:      make(map[string]StopMode),
		queueStore:     store,
		persistSignal:  make(chan struct{}, 1),
		persistStop:    make(chan struct{}),
		media:          NewYTDLPEngine(),
		captions:       NewYTDLPEngine(),
	}
	service.persistWG.Add(1)
	go service.persistenceLoop()
	return service
}

// persistenceLoop coalesces update notifications before snapshotting the queue.
func (s *Service) persistenceLoop() {
	defer s.persistWG.Done()
	for {
		select {
		case <-s.persistSignal:
			s.schedulePersist()
		case <-s.persistStop:
			return
		}
	}
}

// SetMediaEngine replaces the media backend (tests).
func (s *Service) SetMediaEngine(e MediaEngine) {
	if e == nil {
		return
	}
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	s.media = e
}

// SetCaptionsEngine replaces the captions backend (tests).
func (s *Service) SetCaptionsEngine(e CaptionsEngine) {
	if e == nil {
		return
	}
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	s.captions = e
}

// SetFeatureFlags applies kill switches.
func (s *Service) SetFeatureFlags(disableCaptions, videoOnlySafe bool) {
	s.configMutex.Lock()
	defer s.configMutex.Unlock()
	s.config.DisableCaptions = disableCaptions
	s.config.VideoOnlySafe = videoOnlySafe
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
	return s.AddTaskWithOptions(TaskOptions{URL: url})
}

// AddTaskWithOptions queues a task with explicit mode/language options.
func (s *Service) AddTaskWithOptions(opts TaskOptions) (*model.DownloadTask, error) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	url := strings.TrimSpace(opts.URL)
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	// Duplicate URL policy:
	// - Reject while a non-terminal task exists for the same URL (pending/active/paused).
	// - Allow intentional re-download when prior task is Completed/Stopped/Error.
	if existing := s.findOpenTaskByURLLocked(url); existing != nil {
		return nil, fmt.Errorf("task already exists for URL: %s (id=%s status=%s)", url, existing.ID, existing.Status)
	}

	cfg := s.configSnapshot()
	mode := opts.Mode
	if mode == "" {
		mode = cfg.DefaultMode
	}
	mode = normalizeMode(mode)
	// Kill switches: force video-only / disable captions paths.
	if cfg.VideoOnlySafe || cfg.DisableCaptions {
		if mode == model.DownloadModeVideoAndTranscript || mode == model.DownloadModeTranscriptOnly {
			if cfg.VideoOnlySafe {
				mode = model.DownloadModeVideo
			} else if mode == model.DownloadModeTranscriptOnly {
				return nil, fmt.Errorf("captions are disabled in settings")
			} else {
				mode = model.DownloadModeVideo
			}
		}
	}

	lang := strings.TrimSpace(opts.TranscriptLang)
	if lang == "" {
		lang = cfg.TranscriptLang
	}

	transcriptStatus := model.TranscriptStatusNone
	if mode == model.DownloadModeVideoAndTranscript || mode == model.DownloadModeTranscriptOnly {
		transcriptStatus = model.TranscriptStatusPending
	}

	task := &model.DownloadTask{
		ID:               generateTaskID(),
		URL:              url,
		Title:            "", // Leave empty; UI will fallback to URL until real title arrives
		Status:           model.TaskStatusPending,
		Phase:            model.TaskPhaseQueued,
		Progress:         0.0,
		Percent:          0,
		ETASec:           -1,
		StartedAt:        time.Now(),
		Mode:             mode,
		TranscriptStatus: transcriptStatus,
		TranscriptLang:   lang,
		AudioLanguage:    strings.TrimSpace(opts.AudioLanguage),
	}

	if s.shuttingDown {
		return nil, fmt.Errorf("service is shutting down")
	}
	s.tasks[task.ID] = task
	s.queueStartLocked(task)

	return task, nil
}

// findOpenTaskByURLLocked returns a non-terminal task for url (caller holds tasksMutex).
func (s *Service) findOpenTaskByURLLocked(url string) *model.DownloadTask {
	for _, task := range s.tasks {
		if task.URL != url {
			continue
		}
		// Paused is not IsFinished but still "owns" the URL until removed/restarted.
		if !task.Status.IsFinished() {
			return task
		}
	}
	return nil
}

func normalizeMode(mode model.DownloadMode) model.DownloadMode {
	switch mode {
	case model.DownloadModeVideo, model.DownloadModeVideoAndTranscript, model.DownloadModeTranscriptOnly:
		return mode
	default:
		return model.DownloadModeVideo
	}
}

// transitionTaskLocked applies the common status/phase bookkeeping. The caller
// must hold tasksMutex; workers own the live task objects.
func transitionTaskLocked(task *model.DownloadTask, status model.TaskStatus, phase model.TaskPhase) {
	if task == nil {
		return
	}
	task.Status = status
	if phase != "" {
		task.Phase = phase
	}
	if status.IsFinished() {
		task.FinishedAt = time.Now()
	}
}

// cloneTask creates a detached task snapshot for callers outside the service.
// Download workers retain ownership of the live task objects.
func cloneTask(task *model.DownloadTask) *model.DownloadTask {
	if task == nil {
		return nil
	}
	copy := *task
	return &copy
}

// GetTask returns a detached task snapshot by ID.
func (s *Service) GetTask(id string) (*model.DownloadTask, bool) {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()
	task, exists := s.tasks[id]
	if !exists {
		return nil, false
	}
	return cloneTask(task), true
}

// GetAllTasks returns detached task snapshots.
func (s *Service) GetAllTasks() []*model.DownloadTask {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()

	tasks := make([]*model.DownloadTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		if snapshot := cloneTask(task); snapshot != nil {
			tasks = append(tasks, snapshot)
		}
	}
	return tasks
}

// GetTaskByVideoID returns a detached task snapshot by extracted video ID.
// This is useful for playlist UI rows which use the plain YouTube ID as row identifier.
func (s *Service) GetTaskByVideoID(videoID string) (*model.DownloadTask, bool) {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()

	for _, task := range s.tasks {
		if s.extractVideoID(task.URL) == videoID {
			return cloneTask(task), true
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

	applog.WithTask(id).Info("stop requested", "status", string(task.Status))

	// If task is pending, paused, or in error state, resolve to Stopped immediately
	if task.Status == model.TaskStatusPending ||
		task.Status == model.TaskStatusPaused ||
		task.Status == model.TaskStatusError {
		applog.WithTask(id).Info("stop immediate", "from", string(task.Status))
		transitionTaskLocked(task, model.TaskStatusStopped, model.TaskPhaseDone)
		task.LastError = "" // Clear error when converting to stopped
		delete(s.stopModes, id)
		s.notifyUpdate(task)
		return nil
	}

	if !task.Status.IsActive() {
		applog.WithTask(id).Debug("stop no-op", "status", string(task.Status))
		// Already in terminal state or not stoppable; treat as no-op
		return nil
	}

	applog.WithTask(id).Info("stopping active task")
	// Set stopping status and mark mode as hard stop
	transitionTaskLocked(task, model.TaskStatusStopping, "")
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

	transitionTaskLocked(task, model.TaskStatusStopping, "")
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

	// Keep OutputPath so the engine can resume from partial/.part files.
	task.Status = model.TaskStatusPending
	task.LastError = ""
	task.Warning = ""
	task.StartedAt = time.Now()
	task.FinishedAt = time.Time{}
	delete(s.stopModes, id)
	s.notifyUpdate(task)
	s.queueStartLocked(task)

	return nil
}

// RestartTask restarts a failed, stopped, paused, or pending task from scratch.
// Unlike ResumeTask, restart clears media/transcript outputs and warnings.
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

	s.resetTaskForRestartLocked(task)
	s.notifyUpdate(task)
	s.queueStartLocked(task)

	return nil
}

// resetTaskForRestartLocked clears progress and outputs for a clean re-download.
func (s *Service) resetTaskForRestartLocked(task *model.DownloadTask) {
	task.Status = model.TaskStatusPending
	task.Phase = model.TaskPhaseQueued
	task.Progress = 0.0
	task.Percent = 0
	task.LastError = ""
	task.Warning = ""
	task.Speed = ""
	task.ETASec = -1
	task.StartedAt = time.Now()
	task.FinishedAt = time.Time{}
	task.OutputPath = ""
	task.FileSize = 0
	task.TranscriptSRTPath = ""
	task.TranscriptTXTPath = ""
	task.TranscriptError = ""

	mode := normalizeMode(task.Mode)
	task.Mode = mode
	switch mode {
	case model.DownloadModeVideoAndTranscript, model.DownloadModeTranscriptOnly:
		task.TranscriptStatus = model.TranscriptStatusPending
	default:
		task.TranscriptStatus = model.TranscriptStatusNone
	}

	delete(s.stopModes, task.ID)
	delete(s.progressState, task.ID)
}

// RetryTranscript re-fetches captions for an existing task without re-downloading media.
// Allowed when the task is terminal (Completed/Error/Stopped/Paused) and not actively running.
func (s *Service) RetryTranscript(id string) error {
	s.tasksMutex.Lock()
	task, exists := s.tasks[id]
	if !exists {
		s.tasksMutex.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}
	if task.Status.IsActive() {
		s.tasksMutex.Unlock()
		return fmt.Errorf("task is still active")
	}
	// Promote video-only tasks so partial success UI applies after captions land.
	if task.Mode == model.DownloadModeVideo {
		task.Mode = model.DownloadModeVideoAndTranscript
	}
	if task.Mode != model.DownloadModeVideoAndTranscript && task.Mode != model.DownloadModeTranscriptOnly {
		s.tasksMutex.Unlock()
		return fmt.Errorf("task mode does not support transcripts")
	}
	baseName := "video"
	if task.OutputPath != "" {
		base := filepath.Base(task.OutputPath)
		baseName = strings.TrimSuffix(base, filepath.Ext(base))
	} else if title := strings.TrimSpace(task.Title); title != "" && !strings.HasPrefix(title, "http") {
		baseName = s.sanitizeFilename(title)
	}
	task.TranscriptStatus = model.TranscriptStatusPending
	task.TranscriptError = ""
	task.TranscriptSRTPath = ""
	task.TranscriptTXTPath = ""
	task.Warning = ""
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		s.fetchAndWriteTranscript(ctx, task, baseName)

		s.tasksMutex.Lock()
		defer s.tasksMutex.Unlock()
		switch task.TranscriptStatus {
		case model.TranscriptStatusCompleted:
			task.Warning = ""
			if task.Status == model.TaskStatusError && task.OutputPath != "" {
				task.Status = model.TaskStatusCompleted
				task.LastError = ""
			}
			if task.Mode == model.DownloadModeTranscriptOnly && task.OutputPath == "" {
				task.OutputPath = task.TranscriptTXTPath
			}
			if task.Status != model.TaskStatusCompleted && task.OutputPath != "" {
				task.Status = model.TaskStatusCompleted
			}
		default:
			if task.Status == model.TaskStatusCompleted {
				if task.TranscriptError != "" {
					task.Warning = "Video downloaded, but transcript failed: " + task.TranscriptError
				} else {
					task.Warning = "Video downloaded, but no captions were available"
				}
			}
		}
		s.notifyUpdate(task)
	}()
	return nil
}

// RemoveTask removes a task from the service.
// If the task is downloading, it is cancelled first so the worker frees its slot.
func (s *Service) RemoveTask(id string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	// Cancel in-flight work without mutating the live task after it may have
	// been handed to a worker. The worker watches stopModes and cancels its
	// context; avoiding the write here keeps detached update snapshots safe.
	if task.Status.IsActive() {
		s.stopModes[id] = StopModeStop
	}

	delete(s.tasks, id)
	delete(s.progressState, id)
	// Keep stopModes until worker observes cancel so pause vs stop intent is preserved
	// for the still-running goroutine; cleaned in cleanupTaskRuntimeLocked.
	s.stopSmoothingTimerLocked(id)

	return nil
}

// queueStartLocked starts task when capacity allows (caller holds tasksMutex).
func (s *Service) queueStartLocked(task *model.DownloadTask) {
	if task == nil || task.Status != model.TaskStatusPending {
		return
	}
	if s.activeCount >= s.maxParallel {
		return
	}
	if _, ok := s.tasks[task.ID]; !ok {
		return
	}
	// Claim before spawning to prevent double-start races.
	transitionTaskLocked(task, model.TaskStatusStarting, model.TaskPhaseResolving)
	s.activeCount++
	s.workerWG.Add(1)
	go func(t *model.DownloadTask) {
		defer s.workerWG.Done()
		s.runTask(t)
	}(task)
}

// startTask is kept as a thin wrapper for older call sites/tests.
func (s *Service) startTask(task *model.DownloadTask) {
	s.tasksMutex.Lock()
	if _, ok := s.tasks[task.ID]; !ok {
		s.tasksMutex.Unlock()
		return
	}
	if task.Status != model.TaskStatusPending && task.Status != model.TaskStatusStarting {
		s.tasksMutex.Unlock()
		return
	}
	if task.Status == model.TaskStatusPending {
		if s.activeCount >= s.maxParallel {
			s.tasksMutex.Unlock()
			return
		}
		task.Status = model.TaskStatusStarting
		s.activeCount++
	}
	// If already Starting, activeCount was claimed by queueStartLocked.
	s.tasksMutex.Unlock()
	// startTask is sometimes called inline from tests; still track WaitGroup when claimed here.
	s.runTask(task)
}

// shouldAbort reports whether the worker should exit without further I/O.
func (s *Service) shouldAbort(task *model.DownloadTask) bool {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()
	if _, ok := s.tasks[task.ID]; !ok {
		return true
	}
	return task.Status == model.TaskStatusStopping || task.Status.IsFinished() || task.Status == model.TaskStatusPaused
}

// finalizeAbort applies pause/stop terminal state when exiting before I/O completes.
func (s *Service) finalizeAbort(task *model.DownloadTask) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	if _, ok := s.tasks[task.ID]; !ok {
		return
	}
	mode := s.stopModes[task.ID]
	switch {
	case mode == StopModePause || task.Status == model.TaskStatusPaused:
		transitionTaskLocked(task, model.TaskStatusPaused, model.TaskPhaseDone)
	case task.Status == model.TaskStatusStopping || mode == StopModeStop:
		transitionTaskLocked(task, model.TaskStatusStopped, model.TaskPhaseDone)
	case task.Status.IsFinished():
		// already terminal
	default:
		// Removed-from-map races handled above; keep status if still pending/starting.
	}
	if !task.Status.IsFinished() {
		task.FinishedAt = time.Now()
	}
	delete(s.stopModes, task.ID)
	s.notifyUpdate(task)
}

// finishWorker releases the concurrency slot and starts the next queued task.
func (s *Service) finishWorker(task *model.DownloadTask) {
	s.tasksMutex.Lock()
	s.activeCount--
	if s.activeCount < 0 {
		s.activeCount = 0
	}
	s.cleanupTaskRuntimeLocked(task.ID)
	s.tasksMutex.Unlock()
	s.startNextPendingTask()
}

// cleanupTaskRuntimeLocked drops per-task runtime maps (caller holds tasksMutex).
func (s *Service) cleanupTaskRuntimeLocked(taskID string) {
	// Only clear stopModes when task is no longer active in the map or is terminal.
	if task, ok := s.tasks[taskID]; ok {
		if task.Status.IsActive() {
			return
		}
	}
	delete(s.stopModes, taskID)
	delete(s.progressState, taskID)
	s.stopSmoothingTimerLocked(taskID)
}

// stopSmoothingTimerLocked stops smoothing without taking smoothingMutex around tasksMutex incorrectly.
// Safe to call with or without tasksMutex; takes smoothingMutex.
func (s *Service) stopSmoothingTimerLocked(taskID string) {
	s.stopSmoothingTimer(taskID)
	s.smoothingMutex.Lock()
	delete(s.smoothingState, taskID)
	s.smoothingMutex.Unlock()
}

// runTask performs download/captions work for a claimed task (activeCount already incremented).
func (s *Service) runTask(task *model.DownloadTask) {
	s.tasksMutex.Lock()
	mode := normalizeMode(task.Mode)
	task.Mode = mode
	_, stillTracked := s.tasks[task.ID]
	s.tasksMutex.Unlock()

	if !stillTracked {
		// Removed after claim — free slot without network work.
		s.finishWorker(task)
		return
	}

	s.notifyUpdate(task)

	defer s.finishWorker(task)
	defer func() {
		if r := recover(); r != nil {
			applog.WithTask(task.ID).Error("worker panic", "panic", fmt.Sprint(r))
			s.finishTaskError(task, fmt.Sprintf("internal error: %v", r))
		}
	}()

	s.tasksMutex.Lock()
	// Re-check under lock before marking Downloading (remove/pause race).
	if _, ok := s.tasks[task.ID]; !ok || task.Status == model.TaskStatusStopping {
		s.tasksMutex.Unlock()
		s.finalizeAbort(task)
		return
	}
	task.Status = model.TaskStatusDownloading
	task.Phase = model.TaskPhaseResolving
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	s.startSmoothingTimer(task.ID)
	defer s.stopSmoothingTimer(task.ID)

	applog.WithTask(task.ID).Info("task start", "mode", string(mode), "url", task.URL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			s.tasksMutex.RLock()
			status := task.Status
			_, stopped := s.stopModes[task.ID]
			shutting := s.shuttingDown
			s.tasksMutex.RUnlock()
			if status == model.TaskStatusStopping || stopped || shutting {
				cancel()
				return
			}
			if status.IsFinished() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	needMedia := mode == model.DownloadModeVideo || mode == model.DownloadModeVideoAndTranscript
	needTranscript := mode == model.DownloadModeVideoAndTranscript || mode == model.DownloadModeTranscriptOnly

	// Path safety before any network I/O that writes outputs.
	cfg := s.configSnapshot()
	outDir := cfg.DownloadDir
	policy := cfg.CollisionPolicy
	if err := fsutil.EnsureWritableDir(outDir); err != nil {
		s.finishTaskError(task, err.Error())
		return
	}
	if needMedia {
		if err := fsutil.CheckFreeSpace(outDir, fsutil.DefaultMinFreeBytes); err != nil {
			s.finishTaskError(task, err.Error())
			return
		}
	}
	if s.shouldAbort(task) {
		s.finalizeAbort(task)
		return
	}

	var (
		info       *ytdlp.VideoInfo
		outputPath string
		mediaErr   error
		baseName   = "video"
	)

	if needMedia {
		quality, ext := s.formatForPreset()
		s.tasksMutex.RLock()
		mediaEng := s.media
		s.tasksMutex.RUnlock()
		if mediaEng == nil {
			mediaEng = NewYTDLPEngine()
		}

		s.setPhase(task, model.TaskPhaseResolving)
		resolveCtx, resolveCancel := netutil.WithTimeout(ctx, netutil.DefaultResolveTimeout)
		resErr := netutil.Do(resolveCtx, netutil.DefaultRetry(), func(c context.Context) error {
			resolvedInfo, err := mediaEng.Resolve(c, MediaRequest{
				URL:           task.URL,
				Quality:       quality,
				Ext:           ext,
				AudioLanguage: task.AudioLanguage,
			})
			if err != nil {
				return err
			}
			info = resolvedInfo
			return nil
		})
		resolveCancel()
		if resErr != nil {
			if s.applyStopOrError(task, ctx, resErr) {
				return
			}
			s.finishTaskError(task, s.parseYouTubeError(resErr))
			return
		}
		if info != nil && strings.TrimSpace(info.Title) != "" {
			baseName = s.sanitizeFilename(info.Title)
			s.tasksMutex.Lock()
			task.Title = info.Title
			s.tasksMutex.Unlock()
			s.notifyUpdate(task)
		}

		extGuess := "mp4"
		if info != nil {
			if g := s.guessExtFromFormats(info.Formats); g != "" {
				extGuess = g
			}
		}
		desiredPath := filepath.Join(outDir, baseName+"."+extGuess)
		resolved, collErr := fsutil.ResolveCollision(desiredPath, policy)
		if collErr != nil {
			s.finishTaskError(task, collErr.Error())
			return
		}
		outputPath = resolved.Path

		if resolved.Skip {
			// Existing non-empty file — treat as already downloaded.
			s.tasksMutex.Lock()
			task.OutputPath = outputPath
			task.Progress = 1.0
			task.Percent = 100
			s.tasksMutex.Unlock()
		} else {
			s.setPhase(task, model.TaskPhaseDownloading)
			s.tasksMutex.Lock()
			task.OutputPath = outputPath
			s.tasksMutex.Unlock()
			s.notifyUpdate(task)

			// Long media transfer: retry only a couple times on transient start failures.
			mediaErr = netutil.Do(ctx, netutil.RetryConfig{Attempts: 2, Base: time.Second, MaxDelay: 5 * time.Second}, func(c context.Context) error {
				var err error
				info, err = mediaEng.Download(c, MediaRequest{
					URL:           task.URL,
					Quality:       quality,
					Ext:           ext,
					OutputPath:    outputPath,
					AudioLanguage: task.AudioLanguage,
					OnProgress: func(p ytdlp.Progress) {
						s.updateTaskProgressFromNew(task, p)
					},
				})
				return err
			})
			if mediaErr != nil {
				if s.applyStopOrError(task, ctx, mediaErr) {
					return
				}
				s.finishTaskError(task, s.parseYouTubeError(mediaErr))
				return
			}
		}

		s.tasksMutex.Lock()
		if task.OutputPath != "" {
			if !filepath.IsAbs(task.OutputPath) {
				if abs, e := filepath.Abs(task.OutputPath); e == nil {
					task.OutputPath = abs
				}
			}
			task.OutputPath = filepath.Clean(task.OutputPath)
		}
		if info != nil && info.Title != "" {
			task.Title = info.Title
			baseName = s.sanitizeFilename(info.Title)
		}
		task.Progress = 1.0
		task.Percent = 100
		s.tasksMutex.Unlock()
		if task.OutputPath != "" {
			_ = platform.NotifyMediaScanner(task.OutputPath)
		}
		s.notifyUpdate(task)
	}

	if needTranscript {
		s.tasksMutex.RLock()
		cfg := s.configSnapshot()
		capsDisabled := cfg.DisableCaptions || cfg.VideoOnlySafe
		s.tasksMutex.RUnlock()
		if capsDisabled {
			s.tasksMutex.Lock()
			task.TranscriptStatus = model.TranscriptStatusUnavailable
			task.TranscriptError = "captions disabled by feature flag"
			if mode == model.DownloadModeTranscriptOnly {
				s.tasksMutex.Unlock()
				s.finishTaskError(task, task.TranscriptError)
				return
			}
			task.Warning = "Captions skipped (disabled in settings)"
			s.tasksMutex.Unlock()
			s.notifyUpdate(task)
		} else {
			s.setPhase(task, model.TaskPhaseCaptions)
			capsCtx, capsCancel := netutil.WithTimeout(ctx, netutil.DefaultCaptionsTimeout)
			s.fetchAndWriteTranscript(capsCtx, task, baseName)
			capsCancel()
		}
		// transcript-only with no captions is a hard error
		if mode == model.DownloadModeTranscriptOnly &&
			task.TranscriptStatus != model.TranscriptStatusCompleted {
			msg := task.TranscriptError
			if msg == "" {
				msg = "No captions available for this video"
			}
			s.finishTaskError(task, msg)
			return
		}
	}

	// Final success (possibly partial if transcript failed alongside media)
	s.tasksMutex.Lock()
	transitionTaskLocked(task, model.TaskStatusCompleted, model.TaskPhaseDone)
	task.Progress = 1.0
	task.Percent = 100
	task.FinishedAt = time.Now()
	if needTranscript && task.TranscriptStatus != model.TranscriptStatusCompleted && task.Warning == "" {
		if task.TranscriptError != "" {
			task.Warning = "Video downloaded, but transcript failed: " + task.TranscriptError
		} else if task.TranscriptStatus == model.TranscriptStatusUnavailable {
			task.Warning = "Video downloaded, but no captions were available"
		}
	}
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)
}

func (s *Service) setPhase(task *model.DownloadTask, phase model.TaskPhase) {
	s.tasksMutex.Lock()
	if _, ok := s.tasks[task.ID]; ok {
		task.Phase = phase
	}
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)
}

func (s *Service) formatForPreset() (quality, ext string) {
	switch s.configSnapshot().QualityPreset {
	case "medium":
		// Progressive A/V up to 480p when the selector matches.
		return "height<=480", "mp4"
	case "audio":
		// True audio-only is not supported by the progressive engine yet.
		// Map to smallest progressive stream and surface honesty via docs/UI.
		return "worst", "mp4"
	default:
		return "best", "mp4"
	}
}

func (s *Service) fetchAndWriteTranscript(ctx context.Context, task *model.DownloadTask, baseName string) {
	s.tasksMutex.Lock()
	task.TranscriptStatus = model.TranscriptStatusPending
	langPref := task.TranscriptLang
	if langPref == "" {
		langPref = s.configSnapshot().TranscriptLang
	}
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	s.tasksMutex.RLock()
	capsEng := s.captions
	s.tasksMutex.RUnlock()
	if capsEng == nil {
		capsEng = NewYTDLPEngine()
	}
	// Resolve title for transcript-only / missing title cases.
	if info, err := capsEng.GetVideoInfo(ctx, task.URL); err == nil && info != nil && strings.TrimSpace(info.Title) != "" {
		s.tasksMutex.Lock()
		task.Title = info.Title
		s.tasksMutex.Unlock()
		baseName = s.sanitizeFilename(info.Title)
		s.notifyUpdate(task)
	}

	var caps *types.Captions
	var track types.CaptionTrack
	err := netutil.Do(ctx, netutil.DefaultRetry(), func(c context.Context) error {
		var e error
		caps, track, e = capsEng.DownloadCaptions(c, task.URL, langPref)
		return e
	})
	if err != nil {
		cls := captions.ClassifyError(err)
		if msg := netutil.UserMessage(err); netutil.IsTransient(err) && msg != "" {
			cls.Message = msg
		}
		s.tasksMutex.Lock()
		task.TranscriptStatus = cls.Status
		task.TranscriptError = cls.Message
		s.tasksMutex.Unlock()
		s.notifyUpdate(task)
		return
	}
	if caps == nil || len(caps.Cues) == 0 {
		s.tasksMutex.Lock()
		task.TranscriptStatus = model.TranscriptStatusUnavailable
		task.TranscriptError = "caption track contained no cues"
		s.tasksMutex.Unlock()
		s.notifyUpdate(task)
		return
	}

	if strings.TrimSpace(baseName) == "" || baseName == "video" {
		if title := strings.TrimSpace(task.Title); title != "" && !strings.HasPrefix(title, "http") {
			baseName = s.sanitizeFilename(title)
		} else {
			// Best-effort title from player via list tracks path is already used when media ran.
			baseName = "transcript"
		}
	}

	lang := track.LanguageCode
	if lang == "" {
		lang = caps.LanguageCode
	}
	if lang == "" {
		lang = "und"
	}
	cfg := s.configSnapshot()
	outDir := cfg.DownloadDir
	policy := cfg.CollisionPolicy
	srtPath, txtPath := transcript.BuildPaths(outDir, baseName, lang)
	srtRes, err := fsutil.ResolveCollision(srtPath, policy)
	if err != nil {
		s.tasksMutex.Lock()
		task.TranscriptStatus = model.TranscriptStatusError
		task.TranscriptError = "srt path: " + err.Error()
		s.tasksMutex.Unlock()
		s.notifyUpdate(task)
		return
	}
	txtRes, err := fsutil.ResolveCollision(txtPath, policy)
	if err != nil {
		s.tasksMutex.Lock()
		task.TranscriptStatus = model.TranscriptStatusError
		task.TranscriptError = "txt path: " + err.Error()
		s.tasksMutex.Unlock()
		s.notifyUpdate(task)
		return
	}
	srtPath, txtPath = srtRes.Path, txtRes.Path
	s.setPhase(task, model.TaskPhaseWriting)
	if !srtRes.Skip {
		if err := transcript.WriteSRT(srtPath, caps.Cues); err != nil {
			s.tasksMutex.Lock()
			task.TranscriptStatus = model.TranscriptStatusError
			task.TranscriptError = "write srt: " + err.Error()
			s.tasksMutex.Unlock()
			s.notifyUpdate(task)
			return
		}
	}
	if !txtRes.Skip {
		if err := transcript.WriteTXT(txtPath, caps.Cues); err != nil {
			s.tasksMutex.Lock()
			task.TranscriptStatus = model.TranscriptStatusError
			task.TranscriptError = "write txt: " + err.Error()
			task.TranscriptSRTPath = srtPath
			s.tasksMutex.Unlock()
			s.notifyUpdate(task)
			return
		}
	}

	s.tasksMutex.Lock()
	task.TranscriptStatus = model.TranscriptStatusCompleted
	task.TranscriptLang = lang
	task.TranscriptSRTPath = srtPath
	task.TranscriptTXTPath = txtPath
	task.TranscriptError = ""
	if task.Title == "" || strings.HasPrefix(task.Title, "http") {
		task.Title = baseName
	}
	// For transcript-only, expose txt as primary output path for open/reveal defaults.
	if task.Mode == model.DownloadModeTranscriptOnly && task.OutputPath == "" {
		task.OutputPath = txtPath
	}
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)
}

func (s *Service) finishTaskError(task *model.DownloadTask, msg string) {
	s.tasksMutex.Lock()
	transitionTaskLocked(task, model.TaskStatusError, model.TaskPhaseDone)
	task.LastError = msg
	s.tasksMutex.Unlock()
	applog.WithTask(task.ID).Error("task error", "err", msg)
	s.notifyUpdate(task)
}

// applyStopOrError returns true if task was stopped/paused (caller should return).
func (s *Service) applyStopOrError(task *model.DownloadTask, ctx context.Context, err error) bool {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	mode, wasStopped := s.stopModes[task.ID]
	if wasStopped || ctx.Err() == context.Canceled {
		if mode == StopModePause || s.stopModes[task.ID] == StopModePause {
			transitionTaskLocked(task, model.TaskStatusPaused, model.TaskPhaseDone)
		} else {
			transitionTaskLocked(task, model.TaskStatusStopped, model.TaskPhaseDone)
		}
		delete(s.stopModes, task.ID)
		s.notifyUpdate(task)
		return true
	}
	_ = err
	return false
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

	for _, task := range s.tasks {
		if task.Status == model.TaskStatusPending {
			s.queueStartLocked(task)
			if s.activeCount >= s.maxParallel {
				return
			}
		}
	}
}

// notifyUpdate publishes a detached snapshot so callbacks cannot mutate worker state.
// Callers may hold tasksMutex; do not lock here.
func (s *Service) notifyUpdate(task *model.DownloadTask) {
	snapshot := cloneTask(task)
	if s.onUpdate != nil && snapshot != nil {
		s.onUpdate(snapshot)
	}
	// Signal a single persistence worker; callers may hold tasksMutex.
	select {
	case s.persistSignal <- struct{}{}:
	default:
	}
}

func (s *Service) schedulePersist() {
	if s.queueStore == nil {
		return
	}
	// Snapshot under lock without holding lock during debounce callback prep.
	s.tasksMutex.RLock()
	snap := make([]*model.DownloadTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		if t == nil {
			continue
		}
		cp := *t
		snap = append(snap, &cp)
	}
	s.tasksMutex.RUnlock()
	s.queueStore.ScheduleSave(snap)
}

// PersistNow flushes the queue snapshot immediately (shutdown/tests).
func (s *Service) PersistNow() error {
	if s.queueStore == nil {
		return nil
	}
	s.tasksMutex.RLock()
	snap := make([]*model.DownloadTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		if t == nil {
			continue
		}
		cp := *t
		snap = append(snap, &cp)
	}
	s.tasksMutex.RUnlock()
	return s.queueStore.Save(snap)
}

// LoadPersistedQueue reads disk snapshot without importing into the live map.
func (s *Service) LoadPersistedQueue() ([]*model.DownloadTask, error) {
	if s.queueStore == nil {
		return nil, nil
	}
	return s.queueStore.Load()
}

// ImportTasks restores tasks into the live queue. Incomplete tasks stay Paused
// unless startPending is true (Pending only).
func (s *Service) ImportTasks(tasks []*model.DownloadTask, startPending bool) int {
	if len(tasks) == 0 {
		return 0
	}
	n := 0
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	if s.shuttingDown {
		return 0
	}
	for _, t := range tasks {
		if t == nil || t.ID == "" || t.URL == "" {
			continue
		}
		if _, exists := s.tasks[t.ID]; exists {
			continue
		}
		if s.findOpenTaskByURLLocked(t.URL) != nil && !t.Status.IsFinished() {
			continue
		}
		cp := *t
		// Never resume mid-download automatically.
		if cp.Status.IsActive() {
			cp.Status = model.TaskStatusPaused
		}
		if cp.Phase == "" {
			cp.Phase = model.TaskPhaseQueued
		}
		s.tasks[cp.ID] = &cp
		n++
		if startPending && cp.Status == model.TaskStatusPending {
			s.queueStartLocked(&cp)
		}
	}
	return n
}

// ClearPersistedQueue deletes the on-disk snapshot.
func (s *Service) ClearPersistedQueue() error {
	if s.queueStore == nil {
		return nil
	}
	return s.queueStore.Clear()
}

// PauseAll requests pause on every active task.
func (s *Service) PauseAll() int {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	n := 0
	for id, task := range s.tasks {
		if task.Status == model.TaskStatusDownloading || task.Status == model.TaskStatusStarting {
			task.Status = model.TaskStatusStopping
			s.stopModes[id] = StopModePause
			n++
			s.notifyUpdate(task)
		}
	}
	return n
}

// ClearCompleted removes Completed tasks from the queue.
func (s *Service) ClearCompleted() int {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	n := 0
	for id, task := range s.tasks {
		if task.Status == model.TaskStatusCompleted {
			delete(s.tasks, id)
			delete(s.progressState, id)
			s.stopSmoothingTimerLocked(id)
			n++
		}
	}
	return n
}

// RetryFailed re-queues Error tasks for another attempt.
func (s *Service) RetryFailed() int {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()
	n := 0
	for _, task := range s.tasks {
		if task.Status != model.TaskStatusError {
			continue
		}
		s.resetTaskForRestartLocked(task)
		s.queueStartLocked(task)
		n++
	}
	return n
}

// Shutdown gracefully pauses in-flight work and waits for workers.
func (s *Service) Shutdown(ctx context.Context) error {
	s.tasksMutex.Lock()
	s.shuttingDown = true
	for id, task := range s.tasks {
		if task.Status.IsActive() {
			task.Status = model.TaskStatusStopping
			s.stopModes[id] = StopModePause
			s.notifyUpdate(task)
		}
	}
	s.tasksMutex.Unlock()

	done := make(chan struct{})
	go func() {
		s.workerWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		s.persistStopOnce.Do(func() { close(s.persistStop) })
		s.persistWG.Wait()
		return s.PersistNow()
	case <-ctx.Done():
		s.persistStopOnce.Do(func() { close(s.persistStop) })
		s.persistWG.Wait()
		_ = s.PersistNow()
		return ctx.Err()
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

// extractVideoID delegates URL classification to the shared URL utility.
func (s *Service) extractVideoID(rawURL string) string {
	return urlutil.VideoID(rawURL)
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

// sanitizeFilename removes or replaces characters that are not safe for filenames.
func (s *Service) sanitizeFilename(filename string) string {
	return fsutil.SanitizeFilename(filename)
}

// parseYouTubeError converts YouTube-specific errors to user-friendly messages
func (s *Service) parseYouTubeError(err error) string {
	if err == nil {
		return ""
	}
	if msg := netutil.UserMessage(err); netutil.IsTransient(err) || msg != err.Error() {
		// Prefer network-oriented message when classification applied.
		if netutil.IsTransient(err) {
			return msg
		}
		switch {
		case strings.Contains(strings.ToLower(msg), "dns"),
			strings.Contains(strings.ToLower(msg), "tls"),
			strings.Contains(strings.ToLower(msg), "offline"),
			strings.Contains(strings.ToLower(msg), "timeout"),
			strings.Contains(strings.ToLower(msg), "rate limited"),
			strings.Contains(strings.ToLower(msg), "unavailable"):
			return msg
		}
	}

	errStr := strings.ToLower(err.Error())

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

	// Authentication issues
	if strings.Contains(errStr, "auth") || strings.Contains(errStr, "login") {
		return "Authentication required - video may be private"
	}

	return err.Error()
}

// Playlist state helpers keep mutable playlist objects synchronized independently
// from the map lock. Playlist workers and cancellation can run concurrently.
func (s *Service) withPlaylistState(fn func()) {
	s.playlistStateMutex.Lock()
	defer s.playlistStateMutex.Unlock()
	fn()
}

func (s *Service) playlistState(fn func()) {
	s.playlistStateMutex.RLock()
	defer s.playlistStateMutex.RUnlock()
	fn()
}

func (s *Service) playlistCancelled(playlist *model.Playlist) bool {
	cancelled := false
	s.playlistState(func() { cancelled = playlist.Cancelled })
	return cancelled
}

func (s *Service) playlistIsCancelled(playlist *model.Playlist) bool {
	cancelled := false
	s.playlistState(func() {
		cancelled = playlist.Cancelled || playlist.Status == model.PlaylistStatusCancelled
	})
	return cancelled
}

func (s *Service) clonePlaylist(playlist *model.Playlist) *model.Playlist {
	if playlist == nil {
		return nil
	}
	var snapshot model.Playlist
	s.playlistState(func() {
		snapshot = *playlist
		snapshot.Videos = make([]*model.PlaylistVideo, 0, len(playlist.Videos))
		for _, video := range playlist.Videos {
			if video == nil {
				continue
			}
			copy := *video
			snapshot.Videos = append(snapshot.Videos, &copy)
		}
	})
	return &snapshot
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
	playlist, exists := s.playlists[id]
	s.playlistsMutex.RUnlock()
	if !exists {
		return nil, false
	}
	return s.clonePlaylist(playlist), true
}

// GetAllPlaylists returns all playlists
func (s *Service) GetAllPlaylists() []*model.Playlist {
	s.playlistsMutex.RLock()
	defer s.playlistsMutex.RUnlock()

	playlists := make([]*model.Playlist, 0, len(s.playlists))
	for _, playlist := range s.playlists {
		if snapshot := s.clonePlaylist(playlist); snapshot != nil {
			playlists = append(playlists, snapshot)
		}
	}
	return playlists
}

// DownloadPlaylist starts downloading a playlist with chunked approach
func (s *Service) DownloadPlaylist(playlist *model.Playlist) error {
	ready := false
	s.playlistState(func() { ready = playlist.IsReadyForDownload() })
	if !ready {
		return fmt.Errorf("playlist is not ready for download")
	}

	s.withPlaylistState(func() { playlist.UpdateStatus(model.PlaylistStatusDownloading) })
	go s.processPlaylist(playlist)
	return nil
}

// processPlaylist processes a playlist by downloading videos in chunks.
// Each chunk waits until item tasks finish (or are cancelled).
func (s *Service) processPlaylist(playlist *model.Playlist) {
	var pendingVideos []*model.PlaylistVideo
	s.playlistState(func() { pendingVideos = playlist.GetPendingVideos() })
	if len(pendingVideos) == 0 {
		status := model.PlaylistStatusCompleted
		if s.playlistCancelled(playlist) {
			status = model.PlaylistStatusCancelled
		}
		s.withPlaylistState(func() { playlist.UpdateStatus(status) })
		return
	}

	chunkSize := s.maxPlaylistParallel
	if chunkSize < 1 {
		chunkSize = 1
	}
	for i := 0; i < len(pendingVideos); i += chunkSize {
		if s.playlistIsCancelled(playlist) {
			s.withPlaylistState(func() { playlist.UpdateStatus(model.PlaylistStatusCancelled) })
			return
		}

		end := min(i+chunkSize, len(pendingVideos))
		chunk := pendingVideos[i:end]

		var wg sync.WaitGroup
		for _, video := range chunk {
			wg.Add(1)
			go func(v *model.PlaylistVideo) {
				defer wg.Done()
				s.downloadPlaylistVideo(playlist, v)
			}(video)
		}
		wg.Wait()

		if s.playlistCancelled(playlist) {
			s.withPlaylistState(func() { playlist.UpdateStatus(model.PlaylistStatusCancelled) })
			return
		}
	}

	// Refresh aggregate counters for UI.
	s.withPlaylistState(func() {
		playlist.Downloaded = len(playlist.GetCompletedVideos())
	})
	if s.playlistCancelled(playlist) {
		s.withPlaylistState(func() { playlist.UpdateStatus(model.PlaylistStatusCancelled) })
		return
	}
	s.withPlaylistState(func() { playlist.UpdateStatus(model.PlaylistStatusCompleted) })
}

// downloadPlaylistVideo downloads a single video from a playlist and blocks until done.
func (s *Service) downloadPlaylistVideo(playlist *model.Playlist, video *model.PlaylistVideo) {
	if s.playlistCancelled(playlist) {
		s.withPlaylistState(func() { playlist.UpdateVideoStatus(video.ID, model.VideoStatusSkipped) })
		return
	}

	s.withPlaylistState(func() { playlist.UpdateVideoStatus(video.ID, model.VideoStatusDownloading) })

	var mode model.DownloadMode
	var lang string
	s.playlistState(func() {
		mode = playlist.Mode
		lang = playlist.TranscriptLang
	})
	if mode == "" {
		s.tasksMutex.RLock()
		mode = s.configSnapshot().DefaultMode
		s.tasksMutex.RUnlock()
	}
	if lang == "" {
		s.tasksMutex.RLock()
		lang = s.configSnapshot().TranscriptLang
		s.tasksMutex.RUnlock()
	}

	// Optional skip-existing: if collision policy is skip and a completed-looking
	// output already exists, AddTask+engine short-circuit still works; we also
	// honor explicit playlist.SkipExisting by mapping to skip collision for the
	// duration of this item when service policy is rename (best-effort).
	task, err := s.AddTaskWithOptions(TaskOptions{
		URL:            video.URL,
		Mode:           mode,
		TranscriptLang: lang,
	})
	if err != nil {
		// Duplicate open task: try to attach to existing by URL.
		if existing := s.findTaskByURL(video.URL); existing != nil {
			task = existing
		} else {
			s.withPlaylistState(func() {
				playlist.UpdateVideoStatus(video.ID, model.VideoStatusError)
				video.Error = err.Error()
			})
			return
		}
	}
	s.withPlaylistState(func() { video.TaskID = task.ID })

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if s.playlistCancelled(playlist) {
			_ = s.StopTask(task.ID)
			s.withPlaylistState(func() { playlist.UpdateVideoStatus(video.ID, model.VideoStatusSkipped) })
			return
		}

		// Refresh task pointer from service map (status mutations are on same ptr).
		cur, ok := s.GetTask(task.ID)
		if !ok {
			// Removed externally.
			s.withPlaylistState(func() { playlist.UpdateVideoStatus(video.ID, model.VideoStatusSkipped) })
			return
		}
		s.withPlaylistState(func() {
			video.Speed = cur.Speed
			video.ETASec = cur.ETASec
			playlist.UpdateVideoProgress(video.ID, cur.Progress*100)
		})

		switch cur.Status {
		case model.TaskStatusCompleted:
			s.withPlaylistState(func() {
				playlist.UpdateVideoOutputPath(video.ID, cur.OutputPath, cur.FileSize)
				playlist.UpdateVideoStatus(video.ID, model.VideoStatusCompleted)
				playlist.UpdateVideoProgress(video.ID, 100)
				playlist.Downloaded = len(playlist.GetCompletedVideos())
			})
			return
		case model.TaskStatusError:
			s.withPlaylistState(func() {
				playlist.UpdateVideoStatus(video.ID, model.VideoStatusError)
				video.Error = cur.LastError
			})
			return
		case model.TaskStatusStopped:
			s.withPlaylistState(func() { playlist.UpdateVideoStatus(video.ID, model.VideoStatusSkipped) })
			return
		case model.TaskStatusPaused:
			s.withPlaylistState(func() { playlist.UpdateVideoStatus(video.ID, model.VideoStatusPaused) })
			// Keep waiting — user may resume the task.
		}
	}
}

func (s *Service) findTaskByURL(url string) *model.DownloadTask {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()
	for _, t := range s.tasks {
		if t.URL == url {
			return t
		}
	}
	return nil
}

// CancelPlaylist cancels a playlist download and stops in-flight item tasks.
func (s *Service) CancelPlaylist(playlistID string) error {
	s.playlistsMutex.Lock()
	playlist, exists := s.playlists[playlistID]
	if !exists {
		s.playlistsMutex.Unlock()
		return fmt.Errorf("playlist not found: %s", playlistID)
	}
	s.withPlaylistState(func() {
		playlist.Cancelled = true
		playlist.UpdateStatus(model.PlaylistStatusCancelled)
	})
	// Snapshot task ids under playlist state lock, then unlock before StopTask
	// (StopTask needs tasksMutex).
	var taskIDs []string
	s.withPlaylistState(func() {
		for _, video := range playlist.Videos {
			if video.Status == model.VideoStatusDownloading || video.Status == model.VideoStatusPending || video.Status == model.VideoStatusPaused {
				if video.TaskID != "" {
					taskIDs = append(taskIDs, video.TaskID)
				}
				if video.Status != model.VideoStatusCompleted {
					playlist.UpdateVideoStatus(video.ID, model.VideoStatusSkipped)
				}
			}
		}
	})
	s.playlistsMutex.Unlock()

	for _, id := range taskIDs {
		_ = s.StopTask(id)
	}
	// Also stop any open tasks matching remaining video URLs (TaskID not set yet).
	s.playlistsMutex.RLock()
	pl, ok := s.playlists[playlistID]
	s.playlistsMutex.RUnlock()
	if ok {
		var urls []string
		s.playlistState(func() {
			for _, video := range pl.Videos {
				if video.TaskID == "" {
					urls = append(urls, video.URL)
				}
			}
		})
		for _, url := range urls {
			if t := s.findTaskByURL(url); t != nil && !t.Status.IsFinished() {
				_ = s.StopTask(t.ID)
			}
		}
	}
	return nil
}

// RetryFailedPlaylistItems re-queues errored (and optionally skipped) items.
func (s *Service) RetryFailedPlaylistItems(playlistID string) error {
	s.playlistsMutex.Lock()
	playlist, exists := s.playlists[playlistID]
	if !exists {
		s.playlistsMutex.Unlock()
		return fmt.Errorf("playlist not found: %s", playlistID)
	}
	s.withPlaylistState(func() {
		playlist.Cancelled = false
		for _, video := range playlist.Videos {
			if video.Status == model.VideoStatusError || video.Status == model.VideoStatusSkipped {
				video.Status = model.VideoStatusPending
				video.Error = ""
				video.Progress = 0
				video.TaskID = ""
				video.UpdatedAt = time.Now()
			}
		}
		playlist.UpdateStatus(model.PlaylistStatusDownloading)
	})
	s.playlistsMutex.Unlock()

	go s.processPlaylist(playlist)
	return nil
}

// SetMaxPlaylistParallel sets the maximum number of concurrent playlist downloads.
func (s *Service) SetMaxPlaylistParallel(max int) {
	if max < 1 {
		max = 1
	}
	s.maxPlaylistParallel = max
}

// SetQualityPreset sets quality preset for downloads.
func (s *Service) SetQualityPreset(preset string) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	if preset != "best" && preset != "medium" && preset != "audio" {
		preset = "best"
	}
	s.configMutex.Lock()
	s.config.QualityPreset = preset
	s.configMutex.Unlock()
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

// SetDownloadDirectory sets the download directory.
func (s *Service) SetDownloadDirectory(dir string) {
	s.configMutex.Lock()
	s.config.DownloadDir = dir
	s.configMutex.Unlock()
}

// SetCollisionPolicy sets skip|overwrite|rename behavior for existing outputs.
func (s *Service) SetCollisionPolicy(policy fsutil.CollisionPolicy) {
	s.configMutex.Lock()
	s.config.CollisionPolicy = fsutil.NormalizeCollisionPolicy(policy)
	s.configMutex.Unlock()
}

// SetDefaultMode sets the default download mode for new tasks.
func (s *Service) SetDefaultMode(mode model.DownloadMode) {
	s.configMutex.Lock()
	s.config.DefaultMode = normalizeMode(mode)
	s.configMutex.Unlock()
}

// SetTranscriptLanguage sets preferred caption language ("" or "auto" = automatic).
func (s *Service) SetTranscriptLanguage(lang string) {
	lang = strings.TrimSpace(strings.ToLower(lang))
	if lang == "auto" || lang == "system" {
		lang = ""
	}
	s.configMutex.Lock()
	s.config.TranscriptLang = lang
	s.configMutex.Unlock()
}
