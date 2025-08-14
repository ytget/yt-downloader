package platform

import (
	"context"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestNewYTDLPParserService(t *testing.T) {
	tests := []struct {
		name            string
		expectedTimeout time.Duration
	}{
		{
			name:            "should create service with default timeout",
			expectedTimeout: DefaultParseTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()

			if service == nil {
				t.Fatal("service should not be nil")
			}

			if service.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, service.timeout)
			}
		})
	}
}

func TestSetTimeout(t *testing.T) {
	tests := []struct {
		name            string
		initialTimeout  time.Duration
		newTimeout      time.Duration
		expectedTimeout time.Duration
	}{
		{
			name:            "should set new timeout",
			initialTimeout:  DefaultParseTimeout,
			newTimeout:      30 * time.Second,
			expectedTimeout: 30 * time.Second,
		},
		{
			name:            "should set zero timeout",
			initialTimeout:  DefaultParseTimeout,
			newTimeout:      0,
			expectedTimeout: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &YTDLPParserService{timeout: tt.initialTimeout}
			service.SetTimeout(tt.newTimeout)

			if service.timeout != tt.expectedTimeout {
				t.Errorf("expected timeout %v, got %v", tt.expectedTimeout, service.timeout)
			}
		})
	}
}

func TestIsValidPlaylistURL(t *testing.T) {
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
			service := NewYTDLPParserService()
			result := service.isValidPlaylistURL(tt.url)

			if result != tt.expected {
				t.Errorf("expected %v, got %v for URL: %s", tt.expected, result, tt.url)
			}
		})
	}
}

func TestExtractPlaylistID(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "extract playlist ID from watch URL",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID",
			expected: "PLAYLIST_ID",
		},
		{
			name:     "extract playlist ID from playlist URL",
			url:      "https://www.youtube.com/playlist?list=PLAYLIST_ID",
			expected: "PLAYLIST_ID",
		},
		{
			name:     "extract playlist ID with additional parameters",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID&index=1&t=30",
			expected: "PLAYLIST_ID",
		},
		{
			name:     "extract playlist ID with multiple list parameters",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID&list=PLAYLIST_ID&list=OTHER_ID",
			expected: "PLAYLIST_ID",
		},
		{
			name:     "URL without playlist parameter",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID",
			expected: "",
		},
		{
			name:     "URL with empty playlist parameter",
			url:      "https://www.youtube.com/watch?v=VIDEO_ID&list=",
			expected: "",
		},
		{
			name:     "empty URL",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			result := service.extractPlaylistID(tt.url)

			if result != tt.expected {
				t.Errorf("expected %q, got %q for URL: %s", tt.expected, result, tt.url)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		seconds  int
		expected string
	}{
		{
			name:     "zero seconds",
			seconds:  0,
			expected: "00:00",
		},
		{
			name:     "less than one minute",
			seconds:  30,
			expected: "00:30",
		},
		{
			name:     "exactly one minute",
			seconds:  60,
			expected: "01:00",
		},
		{
			name:     "more than one minute",
			seconds:  125,
			expected: "02:05",
		},
		{
			name:     "exactly one hour",
			seconds:  3600,
			expected: "01:00:00",
		},
		{
			name:     "more than one hour",
			seconds:  7325,
			expected: "02:02:05",
		},
		{
			name:     "large duration",
			seconds:  123456,
			expected: "34:17:36",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			result := service.formatDuration(tt.seconds)

			if result != tt.expected {
				t.Errorf("expected %q, got %q for %d seconds", tt.expected, result, tt.seconds)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		durationStr string
		expected    string
	}{
		{
			name:        "empty string",
			durationStr: "",
			expected:    DefaultDuration,
		},
		{
			name:        "valid seconds as string",
			durationStr: "125",
			expected:    "02:05",
		},
		{
			name:        "zero seconds",
			durationStr: "0",
			expected:    "00:00",
		},
		{
			name:        "large number",
			durationStr: "7325",
			expected:    "02:02:05",
		},
		{
			name:        "invalid number",
			durationStr: "abc",
			expected:    "abc",
		},
		{
			name:        "mixed string",
			durationStr: "123abc",
			expected:    "123abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			result := service.parseDuration(tt.durationStr)

			if result != tt.expected {
				t.Errorf("expected %q, got %q for duration string: %q", tt.expected, result, tt.durationStr)
			}
		})
	}
}

func TestFindCommonPrefix(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected string
	}{
		{
			name:     "identical strings",
			s1:       "hello world",
			s2:       "hello world",
			expected: "hello world",
		},
		{
			name:     "common prefix",
			s1:       "hello world",
			s2:       "hello there",
			expected: "hello ",
		},
		{
			name:     "no common prefix",
			s1:       "hello world",
			s2:       "goodbye world",
			expected: "",
		},
		{
			name:     "first string is prefix of second",
			s1:       "hello",
			s2:       "hello world",
			expected: "hello",
		},
		{
			name:     "second string is prefix of first",
			s1:       "hello world",
			s2:       "hello",
			expected: "hello",
		},
		{
			name:     "empty first string",
			s1:       "",
			s2:       "hello world",
			expected: "",
		},
		{
			name:     "empty second string",
			s1:       "hello world",
			s2:       "",
			expected: "",
		},
		{
			name:     "both empty strings",
			s1:       "",
			s2:       "",
			expected: "",
		},
		{
			name:     "single character difference",
			s1:       "hello world",
			s2:       "hello world!",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			result := service.findCommonPrefix(tt.s1, tt.s2)

			if result != tt.expected {
				t.Errorf("expected %q, got %q for s1=%q, s2=%q", tt.expected, result, tt.s1, tt.s2)
			}
		})
	}
}

func TestExtractPlaylistTitle(t *testing.T) {
	tests := []struct {
		name     string
		videos   []*model.PlaylistVideo
		expected string
	}{
		{
			name:     "empty videos list",
			videos:   []*model.PlaylistVideo{},
			expected: DefaultPlaylistName,
		},
		{
			name: "single video",
			videos: []*model.PlaylistVideo{
				{Title: "Test Video"},
			},
			expected: "Test Video" + PlaylistSuffix,
		},
		{
			name: "two videos with common prefix longer than minimum",
			videos: []*model.PlaylistVideo{
				{Title: "Rammstein - Ohne Dich Official Video"},
				{Title: "Rammstein - Sonne Official Video"},
			},
			expected: "Rammstein -" + PlaylistSuffix,
		},
		{
			name: "two videos with common prefix shorter than minimum",
			videos: []*model.PlaylistVideo{
				{Title: "Test Video 1"},
				{Title: "Test Video 2"},
			},
			expected: "Test Video" + PlaylistSuffix,
		},
		{
			name: "multiple videos with common prefix",
			videos: []*model.PlaylistVideo{
				{Title: "Artist - Song 1 Official Video"},
				{Title: "Artist - Song 2 Official Video"},
				{Title: "Artist - Song 3 Official Video"},
			},
			expected: "Artist - Song" + PlaylistSuffix,
		},
		{
			name: "videos with no common prefix",
			videos: []*model.PlaylistVideo{
				{Title: "First Video"},
				{Title: "Second Video"},
			},
			expected: "First Video" + PlaylistSuffix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			result := service.extractPlaylistTitle(tt.videos)

			if result != tt.expected {
				t.Errorf("expected %q, got %q for videos: %v", tt.expected, result, tt.videos)
			}
		})
	}
}

func TestParseYTDLPJSONOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int // expected number of videos
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single valid video",
			output:   `{"id":"VIDEO_ID","title":"Test Video","duration":125}`,
			expected: 1,
		},
		{
			name: "multiple valid videos",
			output: `{"id":"VIDEO_ID_1","title":"Test Video 1","duration":125}
{"id":"VIDEO_ID_2","title":"Test Video 2","duration":180}`,
			expected: 2,
		},
		{
			name:     "video with duration_string",
			output:   `{"id":"VIDEO_ID","title":"Test Video","duration_string":"2:05"}`,
			expected: 1,
		},
		{
			name:     "video with missing id",
			output:   `{"title":"Test Video","duration":125}`,
			expected: 0,
		},
		{
			name:     "video with missing title",
			output:   `{"id":"VIDEO_ID","duration":125}`,
			expected: 0,
		},
		{
			name:     "video with empty id",
			output:   `{"id":"","title":"Test Video","duration":125}`,
			expected: 0,
		},
		{
			name:     "video with empty title",
			output:   `{"id":"VIDEO_ID","title":"","duration":125}`,
			expected: 0,
		},
		{
			name: "invalid JSON line",
			output: `{"id":"VIDEO_ID","title":"Test Video","duration":125}
invalid json line
{"id":"VIDEO_ID_2","title":"Test Video 2","duration":180}`,
			expected: 2,
		},
		{
			name: "mixed valid and invalid videos",
			output: `{"id":"VIDEO_ID_1","title":"Test Video 1","duration":125}
{"id":"","title":"Test Video 2","duration":180}
{"id":"VIDEO_ID_3","title":"","duration":200}
{"id":"VIDEO_ID_4","title":"Test Video 4","duration":300}`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			result, err := service.parseYTDLPJSONOutput(tt.output)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(result) != tt.expected {
				t.Errorf("expected %d videos, got %d", tt.expected, len(result))
			}

			// Additional validation for valid videos
			for i, video := range result {
				if video.ID == "" {
					t.Errorf("video %d has empty ID", i)
				}
				if video.Title == "" {
					t.Errorf("video %d has empty title", i)
				}
				if video.URL == "" {
					t.Errorf("video %d has empty URL", i)
				}
				if video.Status != model.VideoStatusPending {
					t.Errorf("video %d has wrong status, expected %s, got %s", i, model.VideoStatusPending, video.Status)
				}
			}
		})
	}
}

func TestParsePlaylist_Integration(t *testing.T) {
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
			errorMsg:    "invalid playlist URL",
		},
		{
			name:        "URL with empty playlist ID",
			url:         "https://www.youtube.com/watch?v=VIDEO_ID&list=",
			expectError: true,
			errorMsg:    "could not extract playlist ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewYTDLPParserService()
			ctx := context.Background()

			result, err := service.ParsePlaylist(ctx, tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
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

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			contains(s[1:], substr))))
}
