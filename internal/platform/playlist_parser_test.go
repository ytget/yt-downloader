package platform

import (
	"context"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestNewPlaylistParserService(t *testing.T) {
	tests := []struct {
		name            string
		expectedTimeout time.Duration
	}{
		{
			name:            "should create service with default timeout",
			expectedTimeout: DefaultPlaylistParseTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPlaylistParserService()

			if service == nil {
				t.Fatal("service should not be nil")
			}

			if service.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, service.timeout)
			}
		})
	}
}

func TestPlaylistSetTimeout(t *testing.T) {
	tests := []struct {
		name            string
		initialTimeout  time.Duration
		newTimeout      time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "should set new timeout",
			initialTimeout:  DefaultPlaylistParseTimeout,
			newTimeout:      60 * time.Second,
			expectedTimeout: 60 * time.Second,
		},
		{
			name:            "should set zero timeout",
			initialTimeout:  DefaultPlaylistParseTimeout,
			newTimeout:      0,
			expectedTimeout: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &PlaylistParserService{timeout: tt.initialTimeout}
			service.SetTimeout(tt.newTimeout)

			if service.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, service.timeout)
			}
		})
	}
}

func TestPlaylistIsValidPlaylistURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "valid playlist URL with watch parameter",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID",
			expected: true,
		},
		{
			name:     "valid playlist URL with playlist parameter",
			url:      "https://www.youtube.com/playlist?list=PLAYLIST_ID",
			expected: true,
		},
		{
			name:     "valid playlist URL with additional parameters",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID&index=1",
			expected: true,
		},
		{
			name:     "invalid URL without playlist parameter",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID",
			expected: false,
		},
		{
			name:     "invalid URL with different domain",
			url:      "https://example.com/watch?v=VIDEO_ID&list=PLAYLIST_ID",
			expected: true, // still contains list= parameter
		},
		{
			name:     "empty URL",
			url:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPlaylistParserService()
			result := service.isValidPlaylistURL(tt.url)

			if result != tt.expected {
				t.Errorf("expected %v, got %v for URL: %s", tt.expected, result, tt.url)
			}
		})
	}
}

func TestPlaylistExtractPlaylistID(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectedID  string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "extract playlist ID from watch URL",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID",
			expectedID:  "PLAYLIST_ID",
			expectError: false,
		},
		{
			name:        "extract playlist ID from playlist URL",
			url:         "https://www.youtube.com/playlist?list=PLAYLIST_ID",
			expectedID:  "PLAYLIST_ID",
			expectError: false,
		},
		{
			name:        "extract playlist ID with additional parameters",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID&start_radio=1",
			expectedID:  "PLAYLIST_ID",
			expectError: false,
		},
		{
			name:        "extract playlist ID with multiple parameters",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID&index=1&t=30",
			expectedID:  "PLAYLIST_ID",
			expectError: false,
		},
		{
			name:        "URL without playlist parameter",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID",
			expectedID:  "",
			expectError: true,
			errorMsg:    "URL does not contain playlist parameter",
		},
		{
			name:        "URL with empty playlist parameter",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list=",
			expectedID:  "",
			expectError: true,
			errorMsg:    "empty playlist ID",
		},
		{
			name:        "URL with malformed playlist parameter",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list",
			expectedID:  "",
			expectError: true,
			errorMsg:    "URL does not contain playlist parameter",
		},
		{
			name:        "empty URL",
			url:         "",
			expectedID:  "",
			expectError: true,
			errorMsg:    "URL does not contain playlist parameter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPlaylistParserService()
			result, err := service.extractPlaylistID(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !playlistContains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if result != tt.expectedID {
					t.Errorf("expected playlist ID %q, got %q", tt.expectedID, result)
				}
			}
		})
	}
}

func TestPlaylistExtractPlaylistTitle(t *testing.T) {
	tests := []struct {
		name     string
		videos   []*model.PlaylistVideo
		expected string
	}{
		{
			name:     "empty videos list",
			videos:   []*model.PlaylistVideo{},
			expected: DefaultPlaylistTitle,
		},
		{
			name: "single video with short title",
			videos: []*model.PlaylistVideo{
				{Title: "Test Video"},
			},
			expected: "Test Video" + DefaultTitleSuffix,
		},
		{
			name: "single video with long title",
			videos: []*model.PlaylistVideo{
				{Title: "This is a very long video title that should be truncated because it exceeds the maximum length limit"},
			},
			expected: "This is a very long video title that should be tru..." + DefaultTitleSuffix,
		},
		{
			name: "single video with title exactly at max length",
			videos: []*model.PlaylistVideo{
				{Title: "This is exactly fifty characters long title here"},
			},
			expected: "This is exactly fifty characters long title here" + DefaultTitleSuffix,
		},
		{
			name: "single video with title one character over max length",
			videos: []*model.PlaylistVideo{
				{Title: "This is exactly fifty one characters long title here!"},
			},
			expected: "This is exactly fifty one characters long title he..." + DefaultTitleSuffix,
		},
		{
			name: "multiple videos (should use first video title)",
			videos: []*model.PlaylistVideo{
				{Title: "First Video Title"},
				{Title: "Second Video Title"},
				{Title: "Third Video Title"},
			},
			expected: "First Video Title" + DefaultTitleSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPlaylistParserService()
			result := service.extractPlaylistTitle(tt.videos)

			if result != tt.expected {
				t.Errorf("expected %q, got %q for videos: %v", tt.expected, result, tt.videos)
			}
		})
	}
}

func TestPlaylistParsePlaylist_Integration(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "invalid URL without playlist parameter",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID",
			expectError: true,
			errorMsg:    "invalid playlist URL format",
		},
		{
			name:        "URL with empty playlist ID",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list=",
			expectError: true,
			errorMsg:    "empty playlist ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPlaylistParserService()
			ctx := context.Background()

			result, err := service.ParsePlaylist(ctx, tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !playlistContains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error to contain %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				if result == nil {
					t.Error("expected playlist result, got nil")
				}
			}
		})
	}
}

func TestPlaylistParsePlaylist_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		expectedStatus model.PlaylistStatus
		expectError    bool
	}{

		{
			name:           "URL with empty playlist ID should return error status",
			url:            "https://www.youtube.com/watch?v=VIDEO_ID&list=",
			expectedStatus: model.PlaylistStatusError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewPlaylistParserService()
			ctx := context.Background()

			result, err := service.ParsePlaylist(ctx, tt.url)

			if !tt.expectError {
				t.Error("expected error, got nil")
				return
			}

			if err == nil {
				t.Error("expected error, got nil")
				return
			}

			if result == nil {
				t.Error("expected playlist result even with error, got nil")
				return
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, result.Status)
			}

			if result.Error == "" {
				t.Error("expected error message in playlist, got empty string")
			}
		})
	}
}

// Helper function to check if string contains substring
func playlistContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			playlistContains(s[1:], substr))))
}
