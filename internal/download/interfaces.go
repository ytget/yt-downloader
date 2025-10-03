package download

import (
	"github.com/ytget/yt-downloader/internal/model"
)

// Downloader defines the interface for the download service.
type Downloader interface {
	SetUpdateCallback(func(*model.DownloadTask))
	AddTask(url string) (*model.DownloadTask, error)
	GetTask(id string) (*model.DownloadTask, bool)
	GetAllTasks() []*model.DownloadTask
	GetTaskByVideoID(videoID string) (*model.DownloadTask, bool)
	StopTask(id string) error
	PauseTask(id string) error
	ResumeTask(id string) error
	RestartTask(id string) error
	RemoveTask(id string) error
	AddPlaylist(playlist *model.Playlist) error
	GetPlaylist(id string) (*model.Playlist, bool)
	GetAllPlaylists() []*model.Playlist
	DownloadPlaylist(playlist *model.Playlist) error
	CancelPlaylist(playlistID string) error
	SetMaxPlaylistParallel(max int)

	// SetQualityPreset configures quality selection for downloads (best/medium/audio)
	SetQualityPreset(preset string)

	// SetMaxParallelDownloads sets the maximum number of parallel downloads
	SetMaxParallelDownloads(max int)

	// SetDownloadDirectory sets the download directory
	SetDownloadDirectory(dir string)
}
