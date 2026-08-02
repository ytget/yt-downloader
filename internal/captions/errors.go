// Package captions maps caption/ytdlp errors into stable, user-facing messages
// and transcript status hints without requiring live YouTube.
package captions

import (
	"strings"

	"github.com/ytget/yt-downloader/internal/model"
)

// Classified is a normalized caption failure.
type Classified struct {
	Status  model.TranscriptStatus
	Message string
}

// ClassifyError turns raw download/caption errors into UI-friendly outcomes.
func ClassifyError(err error) Classified {
	if err == nil {
		return Classified{Status: model.TranscriptStatusNone, Message: ""}
	}
	msg := strings.TrimSpace(err.Error())
	low := strings.ToLower(msg)

	switch {
	case strings.Contains(low, "no caption"),
		strings.Contains(low, "no captions"),
		strings.Contains(low, "caption track contained no cues"):
		return Classified{
			Status:  model.TranscriptStatusUnavailable,
			Message: "No captions available for this video",
		}
	case strings.Contains(low, "age-restricted"), strings.Contains(low, "age restricted"):
		return Classified{
			Status:  model.TranscriptStatusUnavailable,
			Message: "Captions unavailable: video is age-restricted",
		}
	case strings.Contains(low, "private"), strings.Contains(low, "sign-in"), strings.Contains(low, "login_required"):
		return Classified{
			Status:  model.TranscriptStatusUnavailable,
			Message: "Captions unavailable: video is private or requires sign-in",
		}
	case strings.Contains(low, "region-blocked"), strings.Contains(low, "not available in your country"):
		return Classified{
			Status:  model.TranscriptStatusUnavailable,
			Message: "Captions unavailable: video is region-blocked",
		}
	case strings.Contains(low, "live stream"), strings.Contains(low, "live streams"):
		return Classified{
			Status:  model.TranscriptStatusUnavailable,
			Message: "Captions unavailable: live streams are not supported yet",
		}
	case strings.Contains(low, "timedtext expired"), strings.Contains(low, "http 403"):
		return Classified{
			Status:  model.TranscriptStatusError,
			Message: "Caption link expired; try again",
		}
	case strings.Contains(low, "no such host"), strings.Contains(low, "network is unreachable"),
		strings.Contains(low, "connection refused"), strings.Contains(low, "i/o timeout"),
		strings.Contains(low, "tls:"):
		return Classified{
			Status:  model.TranscriptStatusError,
			Message: "Network error while fetching captions",
		}
	default:
		// Keep original detail when unknown.
		if msg == "" {
			msg = "Transcript failed"
		}
		return Classified{Status: model.TranscriptStatusError, Message: msg}
	}
}
