package captions

import (
	"errors"
	"strings"
	"testing"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		in     string
		status model.TranscriptStatus
		sub    string
	}{
		{"no caption tracks available", model.TranscriptStatusUnavailable, "No captions available"},
		{"captions unavailable: video is age-restricted", model.TranscriptStatusUnavailable, "age-restricted"},
		{"captions unavailable: video is private or requires sign-in", model.TranscriptStatusUnavailable, "private"},
		{"captions unavailable: video is region-blocked", model.TranscriptStatusUnavailable, "region-blocked"},
		{"captions unavailable: live streams are not supported yet", model.TranscriptStatusUnavailable, "live"},
		{"captions: timedtext expired or forbidden (http 403)", model.TranscriptStatusError, "expired"},
		{"Get \"https://x\": dial tcp: lookup x: no such host", model.TranscriptStatusError, "Network"},
		{"write srt: permission denied", model.TranscriptStatusError, "permission denied"},
	}
	for _, tc := range cases {
		got := ClassifyError(errors.New(tc.in))
		if got.Status != tc.status {
			t.Fatalf("%q: status=%s want %s", tc.in, got.Status, tc.status)
		}
		if tc.sub != "" && !strings.Contains(got.Message, tc.sub) {
			t.Fatalf("%q: message %q missing %q", tc.in, got.Message, tc.sub)
		}
	}
	if ClassifyError(nil).Status != model.TranscriptStatusNone {
		t.Fatal("nil error")
	}
}
