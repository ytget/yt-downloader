package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ytget/yt-downloader/internal/applog"
	"github.com/ytget/yt-downloader/internal/config"
	"github.com/ytget/yt-downloader/internal/download"
	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/yt-downloader/internal/pathsafety"
	"github.com/ytget/yt-downloader/internal/platform"
	"github.com/ytget/yt-downloader/internal/support"
	"github.com/ytget/yt-downloader/internal/updatecheck"
	"github.com/ytget/yt-downloader/internal/urlutil"
	"github.com/ytget/ytdlp/v2"
)

const (
	playlistParseDelay    = 500 * time.Millisecond
	eventTaskUpdate       = "task:update"
	eventTasksChanged     = "tasks:changed"
	eventPlaylistUpdate   = "playlist:update"
	eventNotification     = "notification"
	eventDownloadComplete = "download:complete"
	eventQueueRestore     = "queue:restore"
)

// App is the Wails backend binding surface.
type App struct {
	ctx         context.Context
	version     string
	settings    *config.Settings
	downloadSvc download.Downloader
	parser      *platform.YTDLPParserService

	mu              sync.RWMutex
	tasks           map[string]*model.DownloadTask
	pendingRestore  []*model.DownloadTask
	settingsWarning string
	shutdownOnce    sync.Once
	pathGuard       *pathsafety.Guard
}

// NotificationPayload is emitted to the frontend toast/status area.
type NotificationPayload struct {
	Message  string `json:"message"`
	Spinning bool   `json:"spinning"`
	Level    string `json:"level"` // info | error | success
}

// ActionResult is a simple frontend-facing result.
type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NewApp creates a new App application struct.
func NewApp(version string) *App {
	return &App{
		version: version,
		tasks:   make(map[string]*model.DownloadTask),
		parser:  platform.NewYTDLPParserService(),
	}
}

// startup is called when the app starts. The context is saved so we can call the runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	settings, err := config.NewSettings()
	if err != nil {
		fmt.Printf("failed to load settings: %v\n", err)
		applog.Error("settings load failed", "err", err)
		a.settingsWarning = "Using defaults; could not open settings store: " + err.Error()
	}
	a.settings = settings
	if a.settings != nil && a.settings.LoadWarning() != "" {
		a.settingsWarning = a.settings.LoadWarning()
	}
	a.initLogging()
	if a.settingsWarning != "" {
		applog.Warn("settings warning", "msg", a.settingsWarning)
	}

	downloadsDir := ""
	if a.settings != nil {
		downloadsDir = a.settings.GetDownloadDirectory()
	}
	if downloadsDir == "" {
		downloadsDir = filepath.Join(os.TempDir(), "downloads")
	}
	if err := platform.CreateDirectoryIfNotExists(downloadsDir); err != nil {
		applog.Warn("ensure downloads dir failed", "err", err, "dir", downloadsDir)
	}

	maxParallel := config.DefaultMaxParallel
	if a.settings != nil {
		maxParallel = a.settings.GetMaxParallelDownloads()
	}
	svc := download.NewService(downloadsDir, maxParallel)
	svc.SetUpdateCallback(a.onTaskUpdate)
	a.downloadSvc = svc
	a.rebuildPathGuard(downloadsDir)
	if a.settings != nil {
		a.applySettingsToService()
	}

	// Optional non-blocking update check (off unless settings enable it).
	if a.settings != nil && a.settings.GetCheckUpdates() {
		go a.runUpdateCheck()
	}

	// Load persisted queue for optional restore (user confirms in UI).
	if saved, err := a.downloadSvc.LoadPersistedQueue(); err != nil {
		applog.Warn("queue load failed", "err", err)
	} else if len(saved) > 0 {
		a.mu.Lock()
		a.pendingRestore = saved
		a.mu.Unlock()
		incomplete := 0
		for _, t := range saved {
			if t != nil && !t.Status.IsFinished() {
				incomplete++
			}
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, eventQueueRestore, map[string]any{
				"total":      len(saved),
				"incomplete": incomplete,
			})
		}
	}

	applog.Info("app ready", "version", a.version, "dir", downloadsDir)
	if a.settingsWarning != "" {
		a.emitNotification(NotificationPayload{Message: a.settingsWarning, Level: "warn"})
	}
}

// shutdown is invoked on app exit (idempotent).
func (a *App) shutdown(ctx context.Context) {
	a.shutdownOnce.Do(func() {
		applog.Info("shutdown begin")
		if a.downloadSvc != nil {
			shCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			if err := a.downloadSvc.Shutdown(shCtx); err != nil {
				applog.Warn("shutdown wait", "err", err)
			}
		}
		_ = applog.Close()
	})
}

// beforeClose starts graceful stop; always allow the window to close.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	go a.shutdown(ctx)
	return false
}

func (a *App) initLogging() {
	level := config.DefaultLogLevel
	toFile := config.DefaultLogToFile
	if a.settings != nil {
		level = a.settings.GetLogLevel()
		toFile = a.settings.GetLogToFile()
	}
	logDir, _ := applog.DefaultLogDir()
	if err := applog.Init(applog.Config{
		Level:       level,
		FileEnabled: toFile,
		Dir:         logDir,
	}); err != nil {
		fmt.Printf("logging init failed: %v\n", err)
		_ = applog.Init(applog.Config{Level: level, FileEnabled: false})
	}
}

// GetVersion returns the app version string.
func (a *App) GetVersion() string {
	return a.version
}

// GetSettings returns current settings.
func (a *App) GetSettings() config.SettingsData {
	if a.settings == nil {
		return config.SettingsData{}
	}
	snap := a.settings.Snapshot()
	if snap.DownloadDirectory == "" {
		snap.DownloadDirectory = a.settings.GetDownloadDirectory()
	}
	if snap.MaxParallel <= 0 {
		snap.MaxParallel = a.settings.GetMaxParallelDownloads()
	}
	if snap.QualityPreset == "" {
		snap.QualityPreset = a.settings.GetQualityPreset()
	}
	if snap.Language == "" {
		snap.Language = a.settings.GetLanguage()
	}
	if snap.FilenameTemplate == "" {
		snap.FilenameTemplate = a.settings.GetFilenameTemplate()
	}
	if snap.DownloadMode == "" {
		snap.DownloadMode = a.settings.GetDownloadMode()
	}
	if snap.TranscriptLang == "" {
		snap.TranscriptLang = a.settings.GetTranscriptLanguage()
	}
	if snap.CollisionPolicy == "" {
		snap.CollisionPolicy = a.settings.GetCollisionPolicy()
	}
	if snap.LogLevel == "" {
		snap.LogLevel = a.settings.GetLogLevel()
	}
	// LogToFile is bool; always reflect getter so older snapshots stay consistent.
	snap.LogToFile = a.settings.GetLogToFile()
	snap.DisableCaptions = a.settings.GetDisableCaptions()
	snap.VideoOnlySafe = a.settings.GetVideoOnlySafe()
	snap.CheckUpdates = a.settings.GetCheckUpdates()
	return snap
}

// SaveSettings persists settings and applies them to the download service.
func (a *App) SaveSettings(data config.SettingsData) ActionResult {
	if a.settings == nil {
		return ActionResult{OK: false, Error: "settings not initialized"}
	}
	if err := a.settings.Apply(data); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	a.applySettingsToService()
	a.initLogging()
	applog.Info("settings saved", "log_level", a.settings.GetLogLevel(), "log_to_file", a.settings.GetLogToFile())
	return ActionResult{OK: true, Message: "Settings saved"}
}

// SelectDownloadDirectory opens a native folder picker.
func (a *App) SelectDownloadDirectory() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("app not ready")
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select download directory",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// GetTasks returns all known download tasks.
func (a *App) GetTasks() []TaskDTO {
	if a.downloadSvc == nil {
		return []TaskDTO{}
	}
	return toTaskDTOs(a.downloadSvc.GetAllTasks())
}

// GetPlaylists returns all playlists tracked by the download service.
func (a *App) GetPlaylists() []PlaylistDTO {
	if a.downloadSvc == nil {
		return []PlaylistDTO{}
	}
	return toPlaylistDTOs(a.downloadSvc.GetAllPlaylists())
}

// StartDownload starts a single video or playlist download from a URL.
// Kept for compatibility; uses current settings mode.
func (a *App) StartDownload(rawURL string) ActionResult {
	return a.StartDownloadRequest(DownloadRequest{URL: rawURL})
}

// ProbeMedia resolves title before download. Audio track probing is unavailable
// until the ytdlp audio-selection API is published.
func (a *App) ProbeMedia(rawURL string) MediaProbeResult {
	cleanURL := sanitizeURL(rawURL)
	if cleanURL == "" {
		return MediaProbeResult{OK: false, Error: "Please enter a URL"}
	}
	if err := validateURL(cleanURL); err != nil {
		return MediaProbeResult{OK: false, Error: "Invalid URL: " + err.Error()}
	}
	if urlutil.IsPlaylist(cleanURL) {
		return MediaProbeResult{OK: true, AudioTracks: []AudioTrackDTO{}, Title: "Playlist"}
	}
	_, ffmpegErr := exec.LookPath("ffmpeg")
	ffmpegFound := ffmpegErr == nil

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	info, err := ytdlp.New().GetVideoInfo(ctx, cleanURL)
	if err != nil {
		applog.Warn("probe media failed", "err", err)
		return MediaProbeResult{OK: false, Error: err.Error(), FFmpegFound: ffmpegFound}
	}
	title := ""
	vid := ""
	if info != nil {
		title = info.Title
		vid = info.ID
	}
	return MediaProbeResult{
		OK:          true,
		Title:       title,
		VideoID:     vid,
		AudioTracks: []AudioTrackDTO{},
		FFmpegFound: ffmpegFound,
	}
}

// StartDownloadRequest starts work with explicit mode/language overrides.
func (a *App) StartDownloadRequest(req DownloadRequest) ActionResult {
	a.applySettingsToService()

	cleanURL := sanitizeURL(req.URL)
	if cleanURL == "" {
		return ActionResult{OK: false, Error: "Please enter a URL"}
	}
	if err := validateURL(cleanURL); err != nil {
		return ActionResult{OK: false, Error: "Invalid URL: " + err.Error()}
	}

	mode := model.DownloadMode(strings.TrimSpace(req.Mode))
	if mode == "" && a.settings != nil {
		mode = model.DownloadMode(a.settings.GetDownloadMode())
	}
	lang := strings.TrimSpace(req.TranscriptLang)
	if lang == "" && a.settings != nil {
		lang = a.resolveTranscriptLang(a.settings.GetTranscriptLanguage())
	} else {
		lang = a.resolveTranscriptLang(lang)
	}

	if urlutil.IsPlaylist(cleanURL) {
		// Keep service defaults in sync and stamp mode/lang onto the playlist itself
		// so each item uses the selection from this request (not a later settings change).
		a.downloadSvc.SetDefaultMode(mode)
		a.downloadSvc.SetTranscriptLanguage(lang)
		go a.handlePlaylistURL(cleanURL, mode, lang)
		a.emitNotification(NotificationPayload{
			Message:  "Starting playlist parsing in background...",
			Spinning: true,
			Level:    "info",
		})
		return ActionResult{OK: true, Message: "Parsing playlist"}
	}

	task, err := a.downloadSvc.AddTaskWithOptions(download.TaskOptions{
		URL:            cleanURL,
		Mode:           mode,
		TranscriptLang: lang,
		AudioLanguage:  strings.TrimSpace(req.AudioLanguage),
	})
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return ActionResult{OK: false, Error: "Already in queue"}
		}
		return ActionResult{OK: false, Error: err.Error()}
	}

	a.trackTask(task)
	a.emitTasksChanged()
	msg := "Download started"
	switch mode {
	case model.DownloadModeTranscriptOnly:
		msg = "Transcript download started"
	case model.DownloadModeVideoAndTranscript:
		msg = "Video + transcript download started"
	}
	return ActionResult{OK: true, Message: msg}
}

// TogglePause resumes or pauses a task.
func (a *App) TogglePause(taskID string) ActionResult {
	task, ok := a.resolveTask(taskID)
	if !ok {
		return ActionResult{OK: false, Error: "Task not found"}
	}

	switch task.Status {
	case model.TaskStatusPending, model.TaskStatusError, model.TaskStatusStopped:
		if err := a.downloadSvc.RestartTask(task.ID); err != nil {
			return ActionResult{OK: false, Error: err.Error()}
		}
	case model.TaskStatusPaused:
		if err := a.downloadSvc.ResumeTask(task.ID); err != nil {
			return ActionResult{OK: false, Error: err.Error()}
		}
	case model.TaskStatusDownloading, model.TaskStatusStarting:
		if err := a.downloadSvc.PauseTask(task.ID); err != nil {
			return ActionResult{OK: false, Error: err.Error()}
		}
	default:
		return ActionResult{OK: false, Error: "Cannot start/pause task in current state"}
	}
	return ActionResult{OK: true}
}

// RemoveTask removes a task from the queue.
func (a *App) RemoveTask(taskID string) ActionResult {
	task, ok := a.resolveTask(taskID)
	if !ok {
		// Still try direct remove.
		if err := a.downloadSvc.RemoveTask(taskID); err != nil {
			return ActionResult{OK: false, Error: err.Error()}
		}
	} else if err := a.downloadSvc.RemoveTask(task.ID); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}

	a.mu.Lock()
	delete(a.tasks, taskID)
	if task != nil {
		delete(a.tasks, task.ID)
	}
	a.mu.Unlock()

	a.emitTasksChanged()
	return ActionResult{OK: true}
}

// OpenFile opens a downloaded file with the default application.
func (a *App) OpenFile(filePath string) ActionResult {
	safe, err := a.confinePath(filePath)
	if err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	if err := platform.OpenFileWithDefaultApp(safe); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	return ActionResult{OK: true}
}

// RevealFile reveals a file in the system file manager.
func (a *App) RevealFile(filePath string) ActionResult {
	safe, err := a.confinePath(filePath)
	if err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	if err := platform.OpenFileInManager(safe); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	return ActionResult{OK: true}
}

func (a *App) confinePath(filePath string) (string, error) {
	if filePath == "" || strings.HasPrefix(filePath, "http") {
		return "", fmt.Errorf("no valid file path")
	}
	if a.pathGuard == nil {
		return "", fmt.Errorf("path guard not ready")
	}
	return a.pathGuard.Check(filePath)
}

func (a *App) rebuildPathGuard(downloadDir string) {
	roots := []string{downloadDir}
	if a.settings != nil {
		if p := a.settings.FilePath(); p != "" {
			roots = append(roots, filepath.Dir(p))
		}
	}
	if logDir, err := applog.DefaultLogDir(); err == nil {
		roots = append(roots, logDir)
	}
	a.pathGuard = pathsafety.New(roots...)
}

func (a *App) runUpdateCheck() {
	res := updatecheck.CheckLatest(context.Background(), a.version)
	if res.Error != "" {
		applog.Debug("update check", "err", res.Error)
		return
	}
	if res.UpdateAvailable {
		msg := fmt.Sprintf("Update available: v%s (you have v%s)", res.Latest, res.Current)
		if res.HTMLURL != "" {
			msg += " — " + res.HTMLURL
		}
		a.emitNotification(NotificationPayload{Message: msg, Level: "info"})
	}
}

// CheckForUpdates runs a manual GitHub releases check (always allowed when invoked).
func (a *App) CheckForUpdates() updatecheck.Result {
	return updatecheck.CheckLatest(context.Background(), a.version)
}

// CopyToClipboard copies text (e.g. a file path) to the system clipboard.
func (a *App) CopyToClipboard(text string) ActionResult {
	if strings.TrimSpace(text) == "" {
		return ActionResult{OK: false, Error: "Nothing to copy"}
	}
	if a.ctx == nil {
		return ActionResult{OK: false, Error: "app not ready"}
	}
	if err := runtime.ClipboardSetText(a.ctx, text); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	return ActionResult{OK: true, Message: "Copied"}
}

// GetClipboardText returns the current system clipboard text (for Paste URL).
func (a *App) GetClipboardText() ActionResult {
	if a.ctx == nil {
		return ActionResult{OK: false, Error: "app not ready"}
	}
	text, err := runtime.ClipboardGetText(a.ctx)
	if err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ActionResult{OK: false, Error: "Clipboard is empty"}
	}
	return ActionResult{OK: true, Message: text}
}

// CopyLogs copies recent application logs to the clipboard for support.
func (a *App) CopyLogs() ActionResult {
	text := applog.RecentText()
	if strings.TrimSpace(text) == "" {
		text = "(no log lines yet)\n"
	}
	header := fmt.Sprintf("YT Downloader v%s logs\n---\n", a.version)
	if path := applog.LogFilePath(); path != "" {
		header = fmt.Sprintf("YT Downloader v%s logs\nlog_file=%s\n---\n", a.version, path)
	}
	return a.CopyToClipboard(header + text)
}

// GetLogPath returns the active log file path (empty if file logging is off).
func (a *App) GetLogPath() string {
	return applog.LogFilePath()
}

// RevealLogFile reveals the log file or log directory in the system file manager.
func (a *App) RevealLogFile() ActionResult {
	path := applog.LogFilePath()
	if path == "" {
		dir, err := applog.DefaultLogDir()
		if err != nil {
			return ActionResult{OK: false, Error: err.Error()}
		}
		_ = os.MkdirAll(dir, 0o755)
		path = dir
	}
	return a.RevealFile(path)
}

// CopySupportBundle builds a sanitized diagnostics bundle and copies it to the clipboard.
func (a *App) CopySupportBundle() ActionResult {
	var settings config.SettingsData
	if a.settings != nil {
		settings = a.GetSettings()
	}
	var tasks []*model.DownloadTask
	if a.downloadSvc != nil {
		tasks = a.downloadSvc.GetAllTasks()
	}
	text := support.BuildText(support.Input{
		AppVersion:  a.version,
		WailsHint:   "wails/v2",
		Settings:    settings,
		Tasks:       tasks,
		RedactPaths: true,
	})
	if path, err := support.WriteFile(text); err == nil {
		applog.Info("support bundle written", "path", path)
	}
	return a.CopyToClipboard(text)
}

// OpenDownloadFolder reveals the configured download directory.
func (a *App) OpenDownloadFolder() ActionResult {
	dir := ""
	if a.settings != nil {
		dir = a.settings.GetDownloadDirectory()
	}
	if dir == "" {
		return ActionResult{OK: false, Error: "No download directory"}
	}
	_ = os.MkdirAll(dir, 0o755)
	return a.RevealFile(dir)
}

// ResetSettings restores factory defaults.
func (a *App) ResetSettings() ActionResult {
	if a.settings == nil {
		return ActionResult{OK: false, Error: "settings not initialized"}
	}
	if err := a.settings.ResetDefaults(); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	a.applySettingsToService()
	a.initLogging()
	return ActionResult{OK: true, Message: "Settings reset"}
}

// GetQualityHelp returns honesty notes for quality presets.
func (a *App) GetQualityHelp() map[string]string {
	return map[string]string{
		"best":   config.QualityPresetHelp(config.QualityBest),
		"medium": config.QualityPresetHelp(config.QualityMedium),
		"audio":  config.QualityPresetHelp(config.QualityAudio),
	}
}

// PauseAll pauses every active download.
func (a *App) PauseAll() ActionResult {
	if a.downloadSvc == nil {
		return ActionResult{OK: false, Error: "not ready"}
	}
	n := a.downloadSvc.PauseAll()
	a.emitTasksChanged()
	return ActionResult{OK: true, Message: fmt.Sprintf("Pausing %d", n)}
}

// ClearCompleted removes finished successful tasks from the list.
func (a *App) ClearCompleted() ActionResult {
	if a.downloadSvc == nil {
		return ActionResult{OK: false, Error: "not ready"}
	}
	n := a.downloadSvc.ClearCompleted()
	a.emitTasksChanged()
	return ActionResult{OK: true, Message: fmt.Sprintf("Cleared %d", n)}
}

// RetryFailed restarts all tasks in Error state.
func (a *App) RetryFailed() ActionResult {
	if a.downloadSvc == nil {
		return ActionResult{OK: false, Error: "not ready"}
	}
	n := a.downloadSvc.RetryFailed()
	a.emitTasksChanged()
	return ActionResult{OK: true, Message: fmt.Sprintf("Retrying %d", n)}
}

// ConfirmRestoreQueue imports the pending disk snapshot into the live queue.
func (a *App) ConfirmRestoreQueue() ActionResult {
	a.mu.Lock()
	saved := a.pendingRestore
	a.pendingRestore = nil
	a.mu.Unlock()
	if len(saved) == 0 {
		return ActionResult{OK: true, Message: "Nothing to restore"}
	}
	n := a.downloadSvc.ImportTasks(saved, false)
	a.emitTasksChanged()
	return ActionResult{OK: true, Message: fmt.Sprintf("Restored %d tasks (paused/history)", n)}
}

// DiscardRestoreQueue drops the pending snapshot (and clears disk).
func (a *App) DiscardRestoreQueue() ActionResult {
	a.mu.Lock()
	a.pendingRestore = nil
	a.mu.Unlock()
	if a.downloadSvc != nil {
		_ = a.downloadSvc.ClearPersistedQueue()
	}
	return ActionResult{OK: true, Message: "Discarded saved queue"}
}

// HasPendingRestore reports whether a restore prompt should stay visible.
func (a *App) HasPendingRestore() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.pendingRestore) > 0
}

// RestoreInfo is shown in the queue restore banner.
type RestoreInfo struct {
	Pending    bool `json:"pending"`
	Total      int  `json:"total"`
	Incomplete int  `json:"incomplete"`
}

// GetRestoreInfo returns pending queue restore details (safe if UI missed the event).
func (a *App) GetRestoreInfo() RestoreInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	info := RestoreInfo{Pending: len(a.pendingRestore) > 0, Total: len(a.pendingRestore)}
	for _, t := range a.pendingRestore {
		if t != nil && !t.Status.IsFinished() {
			info.Incomplete++
		}
	}
	return info
}

// RetryTranscript re-downloads captions for an existing task (keeps media file).
func (a *App) RetryTranscript(taskID string) ActionResult {
	task, ok := a.resolveTask(taskID)
	if !ok {
		return ActionResult{OK: false, Error: "Task not found"}
	}
	if err := a.downloadSvc.RetryTranscript(task.ID); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	a.emitTasksChanged()
	return ActionResult{OK: true, Message: "Retrying transcript"}
}

// CancelPlaylist cancels an in-progress playlist download.
func (a *App) CancelPlaylist(playlistID string) ActionResult {
	if err := a.downloadSvc.CancelPlaylist(playlistID); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	a.emitPlaylistsChanged()
	a.emitTasksChanged()
	return ActionResult{OK: true}
}

// RetryFailedPlaylistItems re-queues failed/skipped playlist videos.
func (a *App) RetryFailedPlaylistItems(playlistID string) ActionResult {
	if err := a.downloadSvc.RetryFailedPlaylistItems(playlistID); err != nil {
		return ActionResult{OK: false, Error: err.Error()}
	}
	a.emitPlaylistsChanged()
	return ActionResult{OK: true, Message: "Retrying failed playlist items"}
}

func (a *App) applySettingsToService() {
	if a.settings == nil || a.downloadSvc == nil {
		return
	}
	dir := a.settings.GetDownloadDirectory()
	if err := platform.CreateDirectoryIfNotExists(dir); err != nil {
		applog.Warn("ensure downloads dir failed", "err", err, "dir", dir)
	}
	mode := model.DownloadMode(a.settings.GetDownloadMode())
	if a.settings.GetVideoOnlySafe() {
		mode = model.DownloadModeVideo
	}
	a.downloadSvc.ApplyConfig(download.ServiceConfig{
		MaxParallel:     a.settings.GetMaxParallelDownloads(),
		DownloadDir:     dir,
		QualityPreset:   string(a.settings.GetQualityPreset()),
		DefaultMode:     mode,
		TranscriptLang:  a.resolveTranscriptLang(a.settings.GetTranscriptLanguage()),
		CollisionPolicy: a.settings.GetCollisionPolicy(),
		DisableCaptions: a.settings.GetDisableCaptions(),
		VideoOnlySafe:   a.settings.GetVideoOnlySafe(),
	})
	a.rebuildPathGuard(dir)
}

// resolveTranscriptLang maps settings/UI values to caption language codes.
// "auto"/"system"/"" use app language when possible.
func (a *App) resolveTranscriptLang(lang string) string {
	lang = strings.TrimSpace(strings.ToLower(lang))
	switch lang {
	case "", "auto", "system":
		if a.settings != nil {
			appLang := strings.TrimSpace(strings.ToLower(a.settings.GetLanguage()))
			if appLang != "" && appLang != "system" {
				return appLang
			}
		}
		return ""
	default:
		return lang
	}
}

func (a *App) onTaskUpdate(task *model.DownloadTask) {
	if task == nil {
		return
	}
	cleanTask(task)
	a.trackTask(task)

	dto := toTaskDTO(task)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventTaskUpdate, dto)
		runtime.EventsEmit(a.ctx, eventTasksChanged)
	}

	if task.Status == model.TaskStatusCompleted {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, eventDownloadComplete, dto)
		}
		if a.settings != nil && a.settings.GetAutoRevealOnComplete() && task.OutputPath != "" {
			if safe, err := a.confinePath(task.OutputPath); err == nil {
				_ = platform.OpenFileInManager(safe)
			}
		}
	}
}

func (a *App) handlePlaylistURL(rawURL string, mode model.DownloadMode, lang string) {
	playlist, err := a.parser.ParsePlaylist(context.Background(), rawURL)
	if err != nil {
		a.emitNotification(NotificationPayload{
			Message: "Failed to parse playlist: " + err.Error(),
			Level:   "error",
		})
		return
	}

	playlist.Mode = mode
	playlist.TranscriptLang = lang
	if a.settings != nil {
		// When collision policy is skip, playlist items treat existing files as done.
		playlist.SkipExisting = a.settings.GetCollisionPolicy() == "skip"
	}

	a.emitNotification(NotificationPayload{
		Message: fmt.Sprintf("Playlist parsed: %s (%d)", playlist.Title, playlist.TotalVideos),
		Level:   "success",
	})
	a.emitPlaylistsChanged()

	time.Sleep(playlistParseDelay)

	if err := a.downloadSvc.AddPlaylist(playlist); err != nil {
		a.emitNotification(NotificationPayload{
			Message: "Failed to queue playlist: " + err.Error(),
			Level:   "error",
		})
		return
	}
	if err := a.downloadSvc.DownloadPlaylist(playlist); err != nil {
		a.emitNotification(NotificationPayload{
			Message: "Failed to start playlist download: " + err.Error(),
			Level:   "error",
		})
		return
	}

	a.emitPlaylistsChanged()
	a.emitTasksChanged()
	a.emitNotification(NotificationPayload{
		Message: fmt.Sprintf("Downloading playlist: %s", playlist.Title),
		Level:   "info",
	})
}

func (a *App) resolveTask(taskID string) (*model.DownloadTask, bool) {
	if task, ok := a.downloadSvc.GetTask(taskID); ok {
		return task, true
	}
	if task, ok := a.downloadSvc.GetTaskByVideoID(taskID); ok {
		return task, true
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	task, ok := a.tasks[taskID]
	return task, ok
}

func (a *App) trackTask(task *model.DownloadTask) {
	if task == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := *task
	a.tasks[task.ID] = &cp
}

func (a *App) emitTasksChanged() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, eventTasksChanged)
}

func (a *App) emitPlaylistsChanged() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, eventPlaylistUpdate, a.GetPlaylists())
}

func (a *App) emitNotification(n NotificationPayload) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, eventNotification, n)
}

func sanitizeURL(raw string) string { return urlutil.Normalize(raw) }

func validateURL(raw string) error { return urlutil.Validate(raw) }

func cleanTask(task *model.DownloadTask) {
	if task.URL != "" {
		task.URL = sanitizeURL(task.URL)
	}
	if task.Title != "" {
		task.Title = strings.Join(strings.Fields(task.Title), " ")
	}
}
