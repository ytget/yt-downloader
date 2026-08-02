package support

import (
	"strings"
	"testing"

	"github.com/ytget/yt-downloader/internal/applog"
	"github.com/ytget/yt-downloader/internal/config"
	"github.com/ytget/yt-downloader/internal/model"
)

func TestBuildTextRedactsAndIncludes(t *testing.T) {
	_ = applog.Init(applog.Config{Level: "info", FileEnabled: false, NoStderr: true})
	applog.Info("bundle-test-line")
	text := BuildText(Input{
		AppVersion: "1.2.3",
		WailsHint:  "v2",
		Settings: config.SettingsData{
			DownloadDirectory: "/Users/secret/Downloads",
			MaxParallel:       2,
			QualityPreset:     "medium",
			LogLevel:          "info",
		},
		Tasks: []*model.DownloadTask{
			{ID: "t1", Status: model.TaskStatusError, Title: "X", LastError: "boom", OutputPath: "/Users/secret/a.mp4"},
		},
		RedactPaths: true,
	})
	if !strings.Contains(text, "app_version=1.2.3") {
		t.Fatal(text)
	}
	if strings.Contains(text, "/Users/secret") {
		t.Fatalf("path not redacted: %s", text)
	}
	if !strings.Contains(text, "a.mp4") {
		t.Fatal("basename missing")
	}
	if !strings.Contains(text, "boom") {
		t.Fatal("error missing")
	}
	if !strings.Contains(text, "bundle-test-line") {
		t.Fatal("logs missing")
	}
}
