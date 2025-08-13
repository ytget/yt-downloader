package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/romanitalian/yt-downloader/internal/model"
)

// YTDLPParserService handles parsing of YouTube playlists using yt-dlp
type YTDLPParserService struct {
	timeout time.Duration
}

// NewYTDLPParserService creates a new yt-dlp parser service
func NewYTDLPParserService() *YTDLPParserService {
	return &YTDLPParserService{
		timeout: 60 * time.Second,
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

	// Important: do NOT force-convert to playlist URL for Mix/autoplay lists (RD*, RDE*),
	// as youtube playlist endpoint can be "unviewable". Pass the original URL with list=.
	videos, err := y.parsePlaylistVideos(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playlist videos: %w", err)
	}

	// Create playlist
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
	return strings.Contains(url, "list=")
}

// extractPlaylistID extracts the playlist ID from various URL formats
func (y *YTDLPParserService) extractPlaylistID(url string) string {
	// Handle different URL formats:
	// https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID
	// https://www.youtube.com/playlist?list=PLAYLIST_ID

	if strings.Contains(url, "list=") {
		parts := strings.Split(url, "list=")
		if len(parts) > 1 {
			playlistPart := parts[1]
			// Remove additional parameters after playlist ID
			if strings.Contains(playlistPart, "&") {
				playlistPart = strings.Split(playlistPart, "&")[0]
			}
			return playlistPart
		}
	}

	return ""
}

// parsePlaylistVideos uses yt-dlp to extract video information from playlist
func (y *YTDLPParserService) parsePlaylistVideos(ctx context.Context, url string) ([]*model.PlaylistVideo, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, y.timeout)
	defer cancel()

	// Build yt-dlp command - use JSON output for better parsing
	// --flat-playlist: Don't extract video info, just list videos
	// --dump-json: Output video info as JSON
	cmd := exec.CommandContext(ctx, "yt-dlp",
		"--flat-playlist",
		"--dump-json",
		url)

	// Execute command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp command failed: %v; output: %s", err, strings.TrimSpace(string(output)))
	}

	// Parse JSON output
	return y.parseYTDLPJSONOutput(string(output))
}

// parseYTDLPJSONOutput parses the JSON output from yt-dlp command
func (y *YTDLPParserService) parseYTDLPJSONOutput(output string) ([]*model.PlaylistVideo, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var videos []*model.PlaylistVideo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse each line as JSON
		var videoData map[string]interface{}
		if err := json.Unmarshal([]byte(line), &videoData); err != nil {
			// Skip invalid JSON lines
			continue
		}

		// Extract required fields
		id, ok := videoData["id"].(string)
		if !ok || id == "" {
			continue
		}

		title, ok := videoData["title"].(string)
		if !ok || title == "" {
			continue
		}

		// Get duration from duration_string if available, otherwise from duration
		var duration string
		if durationStr, ok := videoData["duration_string"].(string); ok && durationStr != "" {
			duration = durationStr
		} else if durationFloat, ok := videoData["duration"].(float64); ok {
			duration = y.formatDuration(int(durationFloat))
		} else {
			duration = "Unknown"
		}

		// Create video URL
		videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", id)

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

// parseDuration converts duration string to readable format
func (y *YTDLPParserService) parseDuration(durationStr string) string {
	// yt-dlp returns duration in seconds
	if durationStr == "" {
		return "Unknown"
	}

	// Try to parse as seconds
	if seconds, err := strconv.Atoi(durationStr); err == nil {
		return y.formatDuration(seconds)
	}

	// If parsing failed, return as-is
	return durationStr
}

// formatDuration formats seconds into HH:MM:SS format
func (y *YTDLPParserService) formatDuration(seconds int) string {
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

// extractPlaylistTitle generates a title for the playlist based on videos
func (y *YTDLPParserService) extractPlaylistTitle(videos []*model.PlaylistVideo) string {
	if len(videos) == 0 {
		return "Unknown Playlist"
	}

	// Try to extract common prefix from video titles
	if len(videos) > 1 {
		firstTitle := videos[0].Title
		commonPrefix := y.findCommonPrefix(firstTitle, videos[1].Title)

		if len(commonPrefix) > 10 { // Only use if prefix is meaningful
			return strings.TrimSpace(commonPrefix) + " Playlist"
		}
	}

	// Fallback: use first video title + "Playlist"
	return videos[0].Title + " Playlist"
}

// findCommonPrefix finds the common prefix between two strings
func (y *YTDLPParserService) findCommonPrefix(s1, s2 string) string {
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[:i]
		}
	}

	return s1[:minLen]
}
