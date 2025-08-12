package model

import (
	"fmt"
	"strings"
	"time"
)

// DownloadTask represents a single download task
type DownloadTask struct {
	ID         string
	URL        string
	Status     TaskStatus
	Progress   float64   // 0.0 to 1.0
	Percent    int       // 0 to 100
	Speed      string    // human readable speed (e.g., "1.2MB/s")
	ETASec     int       // ETA in seconds, -1 if unknown
	LastError  string    // last error message if any
	OutputPath string    // path to downloaded file
	StartedAt  time.Time // when download started
	FinishedAt time.Time // when download finished
	Title      string    // video title
	Duration   string    // video duration
	FileSize   int64     // file size in bytes
}

// CompressionTask represents a single compression task
type CompressionTask struct {
	ID         string
	InputPath  string
	OutputPath string
	Status     TaskStatus
	Progress   float64 // 0.0 to 1.0
	Percent    int     // 0 to 100
	LastError  string  // last error message if any
	StartedAt  time.Time
	FinishedAt time.Time
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

// GetDisplayTitle returns title or URL if title is empty
func (dt *DownloadTask) GetDisplayTitle() string {
	if dt.Title != "" {
		return dt.Title
	}
	return dt.URL
}
