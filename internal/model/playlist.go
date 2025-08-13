package model

import (
	"time"
)

// PlaylistStatus represents the current status of a playlist
type PlaylistStatus string

const (
	PlaylistStatusParsing     PlaylistStatus = "parsing"
	PlaylistStatusReady       PlaylistStatus = "ready"
	PlaylistStatusDownloading PlaylistStatus = "downloading"
	PlaylistStatusCompleted   PlaylistStatus = "completed"
	PlaylistStatusError       PlaylistStatus = "error"
)

// VideoStatus represents the status of a single video in playlist
type VideoStatus string

const (
	VideoStatusPending     VideoStatus = "pending"
	VideoStatusDownloading VideoStatus = "downloading"
	VideoStatusCompleted   VideoStatus = "completed"
	VideoStatusError       VideoStatus = "error"
	VideoStatusSkipped     VideoStatus = "skipped"
	// Paused is used when user paused a download (mapped from task pause state)
	VideoStatusPaused VideoStatus = "paused"
)

// PlaylistVideo represents a single video in a playlist
type PlaylistVideo struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	Duration   string      `json:"duration"`
	URL        string      `json:"url"`
	Status     VideoStatus `json:"status"`
	Progress   float64     `json:"progress"`
	Error      string      `json:"error,omitempty"`
	OutputPath string      `json:"output_path,omitempty"` // Path to downloaded file
	FileSize   int64       `json:"file_size,omitempty"`   // File size in bytes
	// Runtime telemetry (mirrors DownloadTask fields)
	Speed     string    `json:"speed,omitempty"`   // human readable speed (e.g., "1.2MB/s")
	ETASec    int       `json:"eta_sec,omitempty"` // ETA in seconds, -1 if unknown
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Playlist represents a YouTube playlist with its videos
type Playlist struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	URL         string           `json:"url"`
	Videos      []*PlaylistVideo `json:"videos"`
	Status      PlaylistStatus   `json:"status"`
	TotalVideos int              `json:"total_videos"`
	Downloaded  int              `json:"downloaded"`
	Error       string           `json:"error,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// NewPlaylist creates a new playlist instance
func NewPlaylist(url string) *Playlist {
	now := time.Now()
	return &Playlist{
		URL:       url,
		Status:    PlaylistStatusParsing,
		Videos:    make([]*PlaylistVideo, 0),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddVideo adds a video to the playlist
func (p *Playlist) AddVideo(video *PlaylistVideo) {
	p.Videos = append(p.Videos, video)
	p.TotalVideos = len(p.Videos)
	p.UpdatedAt = time.Now()
}

// RemoveVideo removes a video from the playlist by ID
func (p *Playlist) RemoveVideo(videoID string) {
	for i, video := range p.Videos {
		if video.ID == videoID {
			p.Videos = append(p.Videos[:i], p.Videos[i+1:]...)
			p.TotalVideos = len(p.Videos)
			p.UpdatedAt = time.Now()
			break
		}
	}
}

// UpdateStatus updates the playlist status
func (p *Playlist) UpdateStatus(status PlaylistStatus) {
	p.Status = status
	p.UpdatedAt = time.Now()
}

// UpdateVideoStatus updates the status of a specific video
func (p *Playlist) UpdateVideoStatus(videoID string, status VideoStatus) {
	for _, video := range p.Videos {
		if video.ID == videoID {
			video.Status = status
			video.UpdatedAt = time.Now()
			break
		}
	}
}

// UpdateVideoProgress updates the progress of a specific video
func (p *Playlist) UpdateVideoProgress(videoID string, progress float64) {
	for _, video := range p.Videos {
		if video.ID == videoID {
			video.Progress = progress
			video.UpdatedAt = time.Now()
			break
		}
	}
}

// UpdateVideoOutputPath updates the output path and file size of a specific video
func (p *Playlist) UpdateVideoOutputPath(videoID string, outputPath string, fileSize int64) {
	for _, video := range p.Videos {
		if video.ID == videoID {
			video.OutputPath = outputPath
			video.FileSize = fileSize
			video.UpdatedAt = time.Now()
			break
		}
	}
}

// GetPendingVideos returns all videos with pending status
func (p *Playlist) GetPendingVideos() []*PlaylistVideo {
	var pending []*PlaylistVideo
	for _, video := range p.Videos {
		if video.Status == VideoStatusPending {
			pending = append(pending, video)
		}
	}
	return pending
}

// GetDownloadingVideos returns all videos currently downloading
func (p *Playlist) GetDownloadingVideos() []*PlaylistVideo {
	var downloading []*PlaylistVideo
	for _, video := range p.Videos {
		if video.Status == VideoStatusDownloading {
			downloading = append(downloading, video)
		}
	}
	return downloading
}

// GetCompletedVideos returns all completed videos
func (p *Playlist) GetCompletedVideos() []*PlaylistVideo {
	var completed []*PlaylistVideo
	for _, video := range p.Videos {
		if video.Status == VideoStatusCompleted {
			completed = append(completed, video)
		}
	}
	return completed
}

// GetDownloadProgress returns overall download progress as percentage
func (p *Playlist) GetDownloadProgress() float64 {
	if p.TotalVideos == 0 {
		return 0
	}

	completed := len(p.GetCompletedVideos())
	return float64(completed) / float64(p.TotalVideos) * 100
}

// IsReadyForDownload checks if playlist is ready to start downloading
func (p *Playlist) IsReadyForDownload() bool {
	return p.Status == PlaylistStatusReady && p.TotalVideos > 0
}

// HasErrors checks if any video has errors
func (p *Playlist) HasErrors() bool {
	for _, video := range p.Videos {
		if video.Status == VideoStatusError {
			return true
		}
	}
	return false
}
