package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ytget/ytdlp/v2/types"
)

func TestWriteSRTAndTXT(t *testing.T) {
	dir := t.TempDir()
	cues := []types.CaptionCue{
		{StartMs: 0, DurationMs: 1500, Text: "Hello world"},
		{StartMs: 1600, DurationMs: 1000, Text: "Second line"},
	}
	srt := filepath.Join(dir, "a.en.srt")
	txt := filepath.Join(dir, "a.txt")
	if err := WriteSRT(srt, cues); err != nil {
		t.Fatal(err)
	}
	if err := WriteTXT(txt, cues); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(srt)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "00:00:00,000 --> 00:00:01,500") {
		t.Fatalf("bad srt: %s", s)
	}
	if !strings.Contains(s, "Hello world") {
		t.Fatalf("missing text: %s", s)
	}
	traw, err := os.ReadFile(txt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(traw), "Hello world Second line") {
		t.Fatalf("bad txt: %s", traw)
	}
}

func TestBuildPaths(t *testing.T) {
	srt, txt := BuildPaths("/tmp", "My Video.mp4", "EN")
	if !strings.HasSuffix(srt, "My_Video.en.srt") {
		t.Fatalf("srt=%s", srt)
	}
	if !strings.HasSuffix(txt, "My_Video.txt") {
		t.Fatalf("txt=%s", txt)
	}
}
