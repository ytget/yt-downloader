package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

// Timeout constants
const (
	DefaultPlaylistParseTimeout = 30 * time.Second
)

// URL parameters
const (
	PlaylistURLParam       = "list="
	PlaylistParamSeparator = "&"
)

// Default values
const (
	DefaultPlaylistTitle = "Untitled Playlist"
	DefaultTitleSuffix   = " - Playlist"
	MaxTitleLength       = 50
	TitleTruncateSuffix  = "..."
)

// PlaylistParserService handles parsing of YouTube playlists
type PlaylistParserService struct {
	timeout time.Duration
}

// NewPlaylistParserService creates a new playlist parser service
func NewPlaylistParserService() *PlaylistParserService {
	return &PlaylistParserService{
		timeout: DefaultPlaylistParseTimeout,
	}
}

// ParsePlaylist parses a YouTube playlist URL and returns playlist information
func (p *PlaylistParserService) ParsePlaylist(ctx context.Context, url string) (*model.Playlist, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Validate URL format
	if !p.isValidPlaylistURL(url) {
		return nil, fmt.Errorf("invalid playlist URL format: %s", url)
	}

	// Create new playlist instance
	playlist := model.NewPlaylist(url)

	// Extract playlist ID from URL
	playlistID, err := p.extractPlaylistID(url)
	if err != nil {
		playlist.Error = err.Error()
		playlist.UpdateStatus(model.PlaylistStatusError)
		return playlist, err
	}
	playlist.ID = playlistID

	// Parse playlist using library-based parser
	y := NewYTDLPParserService()
	libPlaylist, err := y.ParsePlaylist(ctx, url)
	if err != nil {
		playlist.Error = err.Error()
		playlist.UpdateStatus(model.PlaylistStatusError)
		return playlist, err
	}

	// Add videos to playlist
	for _, video := range libPlaylist.Videos {
		playlist.AddVideo(video)
	}

	// Set playlist title
	if len(libPlaylist.Videos) > 0 {
		playlist.Title = p.extractPlaylistTitle(libPlaylist.Videos)
	} else {
		playlist.Title = fmt.Sprintf("Playlist %s", playlistID)
	}

	// Mark playlist as ready for download
	playlist.UpdateStatus(model.PlaylistStatusReady)

	return playlist, nil
}

// isValidPlaylistURL checks if the URL is a valid YouTube playlist URL
func (p *PlaylistParserService) isValidPlaylistURL(url string) bool {
	// Check for playlist parameter in URL
	return strings.Contains(url, PlaylistURLParam)
}

// extractPlaylistID extracts the playlist ID from a YouTube playlist URL
func (p *PlaylistParserService) extractPlaylistID(url string) (string, error) {
	// Extract playlist ID from URL
	// Support various formats:
	// - https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID&start_radio=1
	// - https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID
	// - https://www.youtube.com/playlist?list=PLAYLIST_ID

	// Find list parameter
	if !strings.Contains(url, PlaylistURLParam) {
		return "", fmt.Errorf("URL does not contain playlist parameter")
	}

	// Extract everything after list=
	parts := strings.Split(url, PlaylistURLParam)
	if len(parts) < 2 {
		return "", fmt.Errorf("could not extract playlist ID from URL")
	}

	playlistID := parts[1]

	// Remove any additional parameters (everything after &)
	if strings.Contains(playlistID, PlaylistParamSeparator) {
		playlistID = strings.Split(playlistID, PlaylistParamSeparator)[0]
	}

	if playlistID == "" {
		return "", fmt.Errorf("empty playlist ID")
	}

	return playlistID, nil
}

// parsePlaylistVideos parses the actual video list from the playlist
func (p *PlaylistParserService) parsePlaylistVideos(ctx context.Context, url string) ([]*model.PlaylistVideo, error) {
	y := NewYTDLPParserService()
	pl, err := y.ParsePlaylist(ctx, url)
	if err != nil {
		return nil, err
	}
	return pl.Videos, nil
}

// extractPlaylistTitle extracts a meaningful title for the playlist
func (p *PlaylistParserService) extractPlaylistTitle(videos []*model.PlaylistVideo) string {
	if len(videos) == 0 {
		return DefaultPlaylistTitle
	}

	// Try to find a common pattern in video titles
	// For now, just use the first video title with "Playlist" suffix
	firstTitle := videos[0].Title
	if len(firstTitle) > MaxTitleLength {
		firstTitle = firstTitle[:MaxTitleLength] + TitleTruncateSuffix
	}

	return firstTitle + DefaultTitleSuffix
}

// SetTimeout sets the timeout for playlist parsing
func (p *PlaylistParserService) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
}
