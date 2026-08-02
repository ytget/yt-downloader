package main

import (
	"strings"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

// TaskDTO is a frontend-safe download task (no time.Time for Wails bindings).
type TaskDTO struct {
	ID                string  `json:"id"`
	URL               string  `json:"url"`
	Status            string  `json:"status"`
	Phase             string  `json:"phase,omitempty"`
	Progress          float64 `json:"progress"`
	Percent           int     `json:"percent"`
	Speed             string  `json:"speed"`
	ETASec            int     `json:"eta_sec"`
	LastError         string  `json:"last_error,omitempty"`
	OutputPath        string  `json:"output_path,omitempty"`
	StartedAt         string  `json:"started_at,omitempty"`
	FinishedAt        string  `json:"finished_at,omitempty"`
	Title             string  `json:"title"`
	Duration          string  `json:"duration"`
	FileSize          int64   `json:"file_size"`
	Mode              string  `json:"mode"`
	TranscriptStatus  string  `json:"transcript_status"`
	TranscriptLang    string  `json:"transcript_lang,omitempty"`
	TranscriptSRTPath string  `json:"transcript_srt_path,omitempty"`
	TranscriptTXTPath string  `json:"transcript_txt_path,omitempty"`
	TranscriptError   string  `json:"transcript_error,omitempty"`
	Warning           string  `json:"warning,omitempty"`
	AudioLanguage     string  `json:"audio_language,omitempty"`
}

// DownloadRequest is the frontend payload for starting work.
type DownloadRequest struct {
	URL            string `json:"url"`
	Mode           string `json:"mode,omitempty"`
	TranscriptLang string `json:"transcript_language,omitempty"`
	AudioLanguage  string `json:"audio_language,omitempty"`
}

// AudioTrackDTO is a selectable multi-language audio option.
type AudioTrackDTO struct {
	LanguageCode string `json:"language_code"`
	DisplayName  string `json:"display_name"`
	TrackID      string `json:"track_id,omitempty"`
	IsDefault    bool   `json:"is_default"`
}

// MediaProbeResult is returned before download so the UI can pick audio language.
type MediaProbeResult struct {
	OK          bool            `json:"ok"`
	Title       string          `json:"title,omitempty"`
	VideoID     string          `json:"video_id,omitempty"`
	AudioTracks []AudioTrackDTO `json:"audio_tracks"`
	Error       string          `json:"error,omitempty"`
	NeedsFFmpeg bool            `json:"needs_ffmpeg"`
	FFmpegFound bool            `json:"ffmpeg_found"`
}

// PlaylistVideoDTO is a frontend-safe playlist video.
type PlaylistVideoDTO struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Duration   string  `json:"duration"`
	URL        string  `json:"url"`
	Status     string  `json:"status"`
	Progress   float64 `json:"progress"`
	Error      string  `json:"error,omitempty"`
	OutputPath string  `json:"output_path,omitempty"`
	FileSize   int64   `json:"file_size,omitempty"`
	Speed      string  `json:"speed,omitempty"`
	ETASec     int     `json:"eta_sec,omitempty"`
	CreatedAt  string  `json:"created_at,omitempty"`
	UpdatedAt  string  `json:"updated_at,omitempty"`
}

// PlaylistDTO is a frontend-safe playlist.
type PlaylistDTO struct {
	ID          string             `json:"id"`
	Title       string             `json:"title"`
	URL         string             `json:"url"`
	Videos      []PlaylistVideoDTO `json:"videos"`
	Status      string             `json:"status"`
	TotalVideos int                `json:"total_videos"`
	Downloaded  int                `json:"downloaded"`
	Error       string             `json:"error,omitempty"`
	CreatedAt   string             `json:"created_at,omitempty"`
	UpdatedAt   string             `json:"updated_at,omitempty"`
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func toTaskDTO(task *model.DownloadTask) TaskDTO {
	if task == nil {
		return TaskDTO{}
	}
	title := task.Title
	if title == "" || strings.HasPrefix(title, "http") {
		title = task.GetDisplayTitle()
	}
	return TaskDTO{
		ID:                task.ID,
		URL:               task.URL,
		Status:            string(task.Status),
		Phase:             string(task.Phase),
		Progress:          task.Progress,
		Percent:           task.Percent,
		Speed:             task.Speed,
		ETASec:            task.ETASec,
		LastError:         task.LastError,
		OutputPath:        task.OutputPath,
		StartedAt:         formatTime(task.StartedAt),
		FinishedAt:        formatTime(task.FinishedAt),
		Title:             title,
		Duration:          task.Duration,
		FileSize:          task.FileSize,
		Mode:              string(task.Mode),
		TranscriptStatus:  string(task.TranscriptStatus),
		TranscriptLang:    task.TranscriptLang,
		TranscriptSRTPath: task.TranscriptSRTPath,
		TranscriptTXTPath: task.TranscriptTXTPath,
		TranscriptError:   task.TranscriptError,
		Warning:           task.Warning,
		AudioLanguage:     task.AudioLanguage,
	}
}

func toTaskDTOs(tasks []*model.DownloadTask) []TaskDTO {
	if len(tasks) == 0 {
		return []TaskDTO{}
	}
	out := make([]TaskDTO, 0, len(tasks))
	for _, task := range tasks {
		if task == nil {
			continue
		}
		out = append(out, toTaskDTO(task))
	}
	return out
}

func toPlaylistVideoDTO(video *model.PlaylistVideo) PlaylistVideoDTO {
	if video == nil {
		return PlaylistVideoDTO{}
	}
	return PlaylistVideoDTO{
		ID:         video.ID,
		Title:      video.Title,
		Duration:   video.Duration,
		URL:        video.URL,
		Status:     string(video.Status),
		Progress:   video.Progress,
		Error:      video.Error,
		OutputPath: video.OutputPath,
		FileSize:   video.FileSize,
		Speed:      video.Speed,
		ETASec:     video.ETASec,
		CreatedAt:  formatTime(video.CreatedAt),
		UpdatedAt:  formatTime(video.UpdatedAt),
	}
}

func toPlaylistDTO(playlist *model.Playlist) PlaylistDTO {
	if playlist == nil {
		return PlaylistDTO{}
	}
	videos := make([]PlaylistVideoDTO, 0, len(playlist.Videos))
	for _, video := range playlist.Videos {
		if video == nil {
			continue
		}
		videos = append(videos, toPlaylistVideoDTO(video))
	}
	return PlaylistDTO{
		ID:          playlist.ID,
		Title:       playlist.Title,
		URL:         playlist.URL,
		Videos:      videos,
		Status:      string(playlist.Status),
		TotalVideos: playlist.TotalVideos,
		Downloaded:  playlist.Downloaded,
		Error:       playlist.Error,
		CreatedAt:   formatTime(playlist.CreatedAt),
		UpdatedAt:   formatTime(playlist.UpdatedAt),
	}
}

func toPlaylistDTOs(playlists []*model.Playlist) []PlaylistDTO {
	if len(playlists) == 0 {
		return []PlaylistDTO{}
	}
	out := make([]PlaylistDTO, 0, len(playlists))
	for _, playlist := range playlists {
		if playlist == nil {
			continue
		}
		out = append(out, toPlaylistDTO(playlist))
	}
	return out
}
