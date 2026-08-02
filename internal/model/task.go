package model

import (
	"fmt"
	"strings"
	"time"
)

// DownloadMode controls media vs transcript outputs for a task.
type DownloadMode string

const (
	DownloadModeVideo              DownloadMode = "video"
	DownloadModeVideoAndTranscript DownloadMode = "video_and_transcript"
	DownloadModeTranscriptOnly     DownloadMode = "transcript_only"
)

// TranscriptStatus tracks caption download outcome (partial success friendly).
type TranscriptStatus string

const (
	TranscriptStatusNone        TranscriptStatus = "none"
	TranscriptStatusPending     TranscriptStatus = "pending"
	TranscriptStatusCompleted   TranscriptStatus = "completed"
	TranscriptStatusUnavailable TranscriptStatus = "unavailable"
	TranscriptStatusError       TranscriptStatus = "error"
)

// TaskPhase is a finer-grained pipeline stage for UI (orthogonal to Status).
type TaskPhase string

const (
	TaskPhaseQueued      TaskPhase = "queued"
	TaskPhaseResolving   TaskPhase = "resolving"
	TaskPhaseDownloading TaskPhase = "downloading"
	TaskPhaseCaptions    TaskPhase = "captions"
	TaskPhaseWriting     TaskPhase = "writing"
	TaskPhaseDone        TaskPhase = "done"
)

// DownloadTask represents a single download task
type DownloadTask struct {
	ID         string     `json:"id"`
	URL        string     `json:"url"`
	Status     TaskStatus `json:"status"`
	Phase      TaskPhase  `json:"phase,omitempty"`
	Progress   float64    `json:"progress"` // 0.0 to 1.0
	Percent    int        `json:"percent"`  // 0 to 100
	Speed      string     `json:"speed"`    // human readable speed (e.g., "1.2MB/s")
	ETASec     int        `json:"eta_sec"`  // ETA in seconds, -1 if unknown
	LastError  string     `json:"last_error,omitempty"`
	OutputPath string     `json:"output_path,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt time.Time  `json:"finished_at"`
	Title      string     `json:"title"`
	Duration   string     `json:"duration"`
	FileSize   int64      `json:"file_size"`

	// Mode and transcript outputs
	Mode              DownloadMode     `json:"mode"`
	TranscriptStatus  TranscriptStatus `json:"transcript_status"`
	TranscriptLang    string           `json:"transcript_lang,omitempty"`
	TranscriptSRTPath string           `json:"transcript_srt_path,omitempty"`
	TranscriptTXTPath string           `json:"transcript_txt_path,omitempty"`
	TranscriptError   string           `json:"transcript_error,omitempty"`
	Warning           string           `json:"warning,omitempty"`

	// Preferred multi-language audio track (language code or track id).
	AudioLanguage string `json:"audio_language,omitempty"`
}

// CompressionTask represents a single compression task
type CompressionTask struct {
	ID         string     `json:"id"`
	InputPath  string     `json:"input_path"`
	OutputPath string     `json:"output_path"`
	Status     TaskStatus `json:"status"`
	Progress   float64    `json:"progress"` // 0.0 to 1.0
	Percent    int        `json:"percent"`  // 0 to 100
	LastError  string     `json:"last_error,omitempty"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt time.Time  `json:"finished_at"`
}

// GetETAString returns ETA formatted as hh:mm:ss, or "—" if unknown
func (dt *DownloadTask) GetETAString() string {
	if dt.ETASec <= 0 {
		return "—"
	}

	hours := dt.ETASec / 3600
	minutes := (dt.ETASec % 3600) / 60
	seconds := dt.ETASec % 60

	if hours > 0 {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds))
		return b.String()
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%02d:%02d", minutes, seconds))
	return b.String()
}

// GetDisplayTitle returns title, filename, or URL in order of preference
func (dt *DownloadTask) GetDisplayTitle() string {
	// First priority: video title (non-URL)
	if dt.Title != "" && !strings.HasPrefix(dt.Title, "http") {
		return dt.Title
	}

	// Second priority: filename from OutputPath
	if dt.OutputPath != "" {
		// Extract just the filename without path (support both / and \ separators)
		parts := strings.FieldsFunc(dt.OutputPath, func(r rune) bool {
			return r == '/' || r == '\\'
		})
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			// Remove file extension for cleaner display
			if idx := strings.LastIndex(filename, "."); idx > 0 {
				filename = filename[:idx]
			}
			return filename
		}
	}

	// Fallback: URL (preserve full URL for tests; UI can compact if needed)
	if dt.URL == "" {
		return ""
	}
	return dt.URL
}

// GetDisplayTitleForMobile returns absolute file path for mobile devices, or title for desktop
func (dt *DownloadTask) GetDisplayTitleForMobile(isMobile bool) string {
	if isMobile {
		// For mobile: show absolute file path if available
		if dt.OutputPath != "" {
			return dt.OutputPath
		}
		// Fallback to title or URL
		if dt.Title != "" && !strings.HasPrefix(dt.Title, "http") {
			return dt.Title
		}
		return dt.URL
	}

	// For desktop: use regular display title
	return dt.GetDisplayTitle()
}
