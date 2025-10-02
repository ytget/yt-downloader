package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/ytdlp/v2"
)

// Timeout constants
const (
	DefaultParseTimeout = 60 * time.Second
)

// URL parameters and separators
const (
	PlaylistParam  = "list="
	ParamSeparator = "&"
)

// Default values
const (
	DefaultDuration     = "Unknown"
	DefaultPlaylistName = "Unknown Playlist"
)

// URL templates
const (
	YouTubeVideoURLTemplate = "https://www.youtube.com/watch?v=%s"
)

// Playlist title constants
const (
	MinPrefixLength = 10
	PlaylistSuffix  = " Playlist"
)

// Time formatting constants
const (
	SecondsPerHour   = 3600
	SecondsPerMinute = 60
	TimeFormat       = "%02d"
)

// YTDLPParserService handles parsing of YouTube playlists using library
type YTDLPParserService struct {
	timeout time.Duration
}

// NewYTDLPParserService creates a new parser service
func NewYTDLPParserService() *YTDLPParserService {
	return &YTDLPParserService{
		timeout: DefaultParseTimeout,
	}
}

// SetTimeout sets the timeout for parsing operations
func (y *YTDLPParserService) SetTimeout(timeout time.Duration) {
	y.timeout = timeout
}

// ParsePlaylist parses a YouTube playlist and returns video information
func (y *YTDLPParserService) ParsePlaylist(ctx context.Context, url string) (*model.Playlist, error) {
	// Validate URL
	if !y.isValidPlaylistURL(url) {
		return nil, fmt.Errorf("invalid playlist URL: %s", url)
	}

	// Extract playlist ID
	playlistID := y.extractPlaylistID(url)
	if playlistID == "" {
		return nil, fmt.Errorf("could not extract playlist ID from URL: %s", url)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, y.timeout)
	defer cancel()

	// Use library to fetch items
	d := ytdlp.New()
	items, err := d.GetPlaylistItemsAll(ctx, playlistID, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get playlist items: %v", err)
	}

	videos := make([]*model.PlaylistVideo, 0, len(items))
	for _, it := range items {
		videoURL := fmt.Sprintf(YouTubeVideoURLTemplate, it.VideoID)
		v := &model.PlaylistVideo{
			ID:        it.VideoID,
			Title:     it.Title,
			Duration:  DefaultDuration,
			URL:       videoURL,
			Status:    model.VideoStatusPending,
			Progress:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		videos = append(videos, v)
	}

	playlist := &model.Playlist{
		ID:          playlistID,
		Title:       y.extractPlaylistTitle(videos),
		URL:         url,
		Videos:      videos,
		Status:      model.PlaylistStatusReady,
		TotalVideos: len(videos),
		Downloaded:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return playlist, nil
}

// isValidPlaylistURL checks if the URL is a valid YouTube playlist URL
func (y *YTDLPParserService) isValidPlaylistURL(url string) bool {
	return strings.Contains(url, PlaylistParam)
}

// extractPlaylistID extracts the playlist ID from various URL formats
func (y *YTDLPParserService) extractPlaylistID(url string) string {
	if strings.Contains(url, PlaylistParam) {
		parts := strings.Split(url, PlaylistParam)
		if len(parts) > 1 {
			playlistPart := parts[1]
			if strings.Contains(playlistPart, ParamSeparator) {
				playlistPart = strings.Split(playlistPart, ParamSeparator)[0]
			}
			return playlistPart
		}
	}
	return ""
}

// formatDuration formats seconds into HH:MM:SS format
func (y *YTDLPParserService) formatDuration(seconds int) string {
	hours := seconds / SecondsPerHour
	minutes := (seconds % SecondsPerHour) / SecondsPerMinute
	secs := seconds % SecondsPerMinute
	if hours > 0 {
		return fmt.Sprintf(TimeFormat+":"+TimeFormat+":"+TimeFormat, hours, minutes, secs)
	}
	return fmt.Sprintf(TimeFormat+":"+TimeFormat, minutes, secs)
}

// parseDuration converts duration string to readable format (compat helper for tests)
func (y *YTDLPParserService) parseDuration(durationStr string) string {
	if durationStr == "" {
		return DefaultDuration
	}
	if seconds, err := strconv.Atoi(durationStr); err == nil {
		return y.formatDuration(seconds)
	}
	return durationStr
}

// parseYTDLPJSONOutput parses JSON-lines output (compat helper for tests)
func (y *YTDLPParserService) parseYTDLPJSONOutput(output string) ([]*model.PlaylistVideo, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var videos []*model.PlaylistVideo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var videoData map[string]interface{}
		if err := json.Unmarshal([]byte(line), &videoData); err != nil {
			continue
		}
		id, ok := videoData["id"].(string)
		if !ok || id == "" {
			continue
		}
		title, ok := videoData["title"].(string)
		if !ok || title == "" {
			continue
		}
		var duration string
		if durationStr, ok := videoData["duration_string"].(string); ok && durationStr != "" {
			duration = durationStr
		} else if durationFloat, ok := videoData["duration"].(float64); ok {
			duration = y.formatDuration(int(durationFloat))
		} else {
			duration = DefaultDuration
		}
		videoURL := fmt.Sprintf(YouTubeVideoURLTemplate, id)
		video := &model.PlaylistVideo{
			ID:        id,
			Title:     title,
			Duration:  duration,
			URL:       videoURL,
			Status:    model.VideoStatusPending,
			Progress:  0,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		videos = append(videos, video)
	}
	return videos, nil
}

// extractPlaylistTitle generates a title for the playlist based on videos
func (y *YTDLPParserService) extractPlaylistTitle(videos []*model.PlaylistVideo) string {
	if len(videos) == 0 {
		return DefaultPlaylistName
	}
	if len(videos) > 1 {
		firstTitle := videos[0].Title
		commonPrefix := y.findCommonPrefix(firstTitle, videos[1].Title)
		if len(commonPrefix) > MinPrefixLength {
			return strings.TrimSpace(commonPrefix) + PlaylistSuffix
		}
	}
	return videos[0].Title + PlaylistSuffix
}

// findCommonPrefix finds the common prefix between two strings
func (y *YTDLPParserService) findCommonPrefix(s1, s2 string) string {
	minLen := min(len(s1), len(s2))
	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}
	return s1[:minLen]
}
