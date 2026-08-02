package applog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAndRecentText(t *testing.T) {
	dir := t.TempDir()
	if err := Init(Config{
		Level:       "debug",
		FileEnabled: true,
		Dir:         dir,
		NoStderr:    true,
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	Info("hello", "k", "v")
	WithTask("task-1").Warn("stopped", "status", "Stopping")

	text := RecentText()
	if !strings.Contains(text, "hello") {
		t.Fatalf("ring missing hello: %q", text)
	}
	if !strings.Contains(text, "task_id=task-1") && !strings.Contains(text, "task-1") {
		t.Fatalf("ring missing task id: %q", text)
	}
	if !strings.Contains(text, "stopped") {
		t.Fatalf("ring missing stopped: %q", text)
	}

	path := LogFilePath()
	if path == "" {
		t.Fatal("expected log file path")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "hello") {
		t.Fatalf("file missing hello: %s", raw)
	}
}

func TestTruncateLargeAttrs(t *testing.T) {
	dir := t.TempDir()
	if err := Init(Config{Level: "info", FileEnabled: false, Dir: dir, NoStderr: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	big := strings.Repeat("x", MaxAttrValueLen+100)
	Info("payload", "body", big)
	text := RecentText()
	if strings.Contains(text, big) {
		t.Fatal("expected large attr to be truncated")
	}
	if !strings.Contains(text, "truncated") {
		t.Fatalf("expected truncation marker: %q", text)
	}
}

func TestOmitBulkJSON(t *testing.T) {
	if err := Init(Config{Level: "info", FileEnabled: false, NoStderr: true}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	blob := `{"captions":{"playerCaptionsTracklistRenderer":{` + strings.Repeat(`"a":{},`, 20) + `"z":1}}}`
	Info("meta", "player", blob)
	text := RecentText()
	if strings.Contains(text, "playerCaptionsTracklistRenderer") {
		t.Fatalf("bulk json should be omitted: %q", text)
	}
	if !strings.Contains(text, "omitted json") {
		t.Fatalf("expected omit marker: %q", text)
	}
}

func TestParseAndNormalizeLevel(t *testing.T) {
	if NormalizeLevel("warning") != "warn" {
		t.Fatal(NormalizeLevel("warning"))
	}
	if NormalizeLevel("nope") != DefaultLevel {
		t.Fatal(NormalizeLevel("nope"))
	}
	if NormalizeLevel("DEBUG") != "debug" {
		t.Fatal(NormalizeLevel("DEBUG"))
	}
	SetLevel("error")
	if Level() != "error" {
		t.Fatalf("level=%s", Level())
	}
	SetLevel("info")
	if Level() != "info" {
		t.Fatalf("level=%s", Level())
	}
}

func TestDefaultLogDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))
	dir, err := DefaultLogDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dir, "yt-downloader") || !strings.HasSuffix(dir, "logs") {
		t.Fatalf("unexpected dir %s", dir)
	}
}
