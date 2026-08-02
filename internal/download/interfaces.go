package download

import (
	"context"

	"github.com/ytget/yt-downloader/internal/fsutil"
	"github.com/ytget/yt-downloader/internal/model"
)

// TaskOptions configures a new download/transcript task.
type TaskOptions struct {
	URL            string
	Mode           model.DownloadMode
	TranscriptLang string // empty = auto
	AudioLanguage  string // multi-language audio track preference
}

// Downloader defines the interface for the download service.
type Downloader interface {
	SetUpdateCallback(func(*model.DownloadTask))
	AddTask(url string) (*model.DownloadTask, error)
	AddTaskWithOptions(opts TaskOptions) (*model.DownloadTask, error)
	GetTask(id string) (*model.DownloadTask, bool)
	GetAllTasks() []*model.DownloadTask
	GetTaskByVideoID(videoID string) (*model.DownloadTask, bool)
	StopTask(id string) error
	PauseTask(id string) error
	ResumeTask(id string) error
	RestartTask(id string) error
	RetryTranscript(id string) error
	RemoveTask(id string) error
	AddPlaylist(playlist *model.Playlist) error
	GetPlaylist(id string) (*model.Playlist, bool)
	GetAllPlaylists() []*model.Playlist
	DownloadPlaylist(playlist *model.Playlist) error
	CancelPlaylist(playlistID string) error
	RetryFailedPlaylistItems(playlistID string) error
	SetMaxPlaylistParallel(max int)

	// ApplyConfig updates all runtime settings in one operation.
	ApplyConfig(config ServiceConfig)

	// SetQualityPreset configures quality selection for downloads (best/medium/audio)
	SetQualityPreset(preset string)

	// SetMaxParallelDownloads sets the maximum number of parallel downloads
	SetMaxParallelDownloads(max int)

	// SetDownloadDirectory sets the download directory
	SetDownloadDirectory(dir string)

	// SetCollisionPolicy sets skip|overwrite|rename for existing output files
	SetCollisionPolicy(policy fsutil.CollisionPolicy)

	// SetDefaultMode sets default download mode for new tasks
	SetDefaultMode(mode model.DownloadMode)

	// SetTranscriptLanguage sets preferred transcript language ("" = auto)
	SetTranscriptLanguage(lang string)

	// SetFeatureFlags applies captions kill switch and video-only safe mode.
	SetFeatureFlags(disableCaptions, videoOnlySafe bool)

	// SetMediaEngine / SetCaptionsEngine inject backends for tests.
	SetMediaEngine(e MediaEngine)
	SetCaptionsEngine(e CaptionsEngine)

	// Batch / lifecycle
	PauseAll() int
	ClearCompleted() int
	RetryFailed() int
	Shutdown(ctx context.Context) error
	PersistNow() error
	LoadPersistedQueue() ([]*model.DownloadTask, error)
	ImportTasks(tasks []*model.DownloadTask, startPending bool) int
	ClearPersistedQueue() error
}
