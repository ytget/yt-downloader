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
