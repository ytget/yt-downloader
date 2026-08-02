package transcript

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ytget/yt-downloader/internal/fsutil"
	"github.com/ytget/ytdlp/v2/types"
)

// WriteSRT writes cues to path in SubRip format.
func WriteSRT(path string, cues []types.CaptionCue) error {
	var b strings.Builder
	idx := 1
	for _, cue := range cues {
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			continue
		}
		end := cue.EndMs()
		if end <= cue.StartMs {
			end = cue.StartMs + 1000
		}
		b.WriteString(fmt.Sprintf("%d\n", idx))
		b.WriteString(formatSRTTimestamp(cue.StartMs))
		b.WriteString(" --> ")
		b.WriteString(formatSRTTimestamp(end))
		b.WriteString("\n")
		b.WriteString(text)
		b.WriteString("\n\n")
		idx++
	}
	if idx == 1 {
		return fmt.Errorf("no caption cues to write")
	}
	return fsutil.AtomicWriteFile(path, []byte(b.String()), 0o644)
}

// WriteTXT writes plain full-text transcript.
func WriteTXT(path string, cues []types.CaptionCue) error {
	var b strings.Builder
	for i, cue := range cues {
		text := strings.TrimSpace(cue.Text)
		if text == "" {
			continue
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(text)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return fmt.Errorf("no caption text to write")
	}
	// Keep paragraphs reasonably readable: break long runs.
	out = wrapPlainText(out, 100)
	return fsutil.AtomicWriteFile(path, []byte(out+"\n"), 0o644)
}

func formatSRTTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func wrapPlainText(s string, width int) string {
	if width <= 0 || utf8.RuneCountInString(s) <= width {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return s
	}
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		wl := utf8.RuneCountInString(w)
		if lineLen == 0 {
			b.WriteString(w)
			lineLen = wl
			continue
		}
		if lineLen+1+wl > width {
			b.WriteByte('\n')
			b.WriteString(w)
			lineLen = wl
			continue
		}
		b.WriteByte(' ')
		b.WriteString(w)
		lineLen += 1 + wl
		_ = i
	}
	return b.String()
}

// BuildPaths returns srt/txt paths for a base title and language.
func BuildPaths(dir, baseName, lang string) (srtPath, txtPath string) {
	baseName = fsutil.SanitizeFilename(baseName)
	lang = strings.TrimSpace(strings.ToLower(lang))
	lang = fsutil.SanitizeFilename(lang)
	if lang == "" || lang == "download" {
		lang = "und"
	}
	// strip extension from base if present
	if ext := filepath.Ext(baseName); ext != "" {
		baseName = strings.TrimSuffix(baseName, ext)
	}
	srtPath = filepath.Join(dir, fmt.Sprintf("%s.%s.srt", baseName, lang))
	txtPath = filepath.Join(dir, fmt.Sprintf("%s.txt", baseName))
	return srtPath, txtPath
}
