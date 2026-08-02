//go:build e2e

package download

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestE2E_TranscriptOnly_RayDalio(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir, 1).(*Service)
	done := make(chan *model.DownloadTask, 1)
	svc.SetUpdateCallback(func(task *model.DownloadTask) {
		if task.Status == model.TaskStatusCompleted || task.Status == model.TaskStatusError {
			select {
			case done <- task:
			default:
			}
		}
	})

	_, err := svc.AddTaskWithOptions(TaskOptions{
		URL:            "https://www.youtube.com/watch?v=VoJQuQRih0s",
		Mode:           model.DownloadModeTranscriptOnly,
		TranscriptLang: "en",
	})
	if err != nil {
		t.Fatal(err)
	}

	select {
	case task := <-done:
		if task.Status != model.TaskStatusCompleted {
			t.Fatalf("status=%s err=%q terr=%q", task.Status, task.LastError, task.TranscriptError)
		}
		if task.TranscriptStatus != model.TranscriptStatusCompleted {
			t.Fatalf("tstatus=%s terr=%q", task.TranscriptStatus, task.TranscriptError)
		}
		if task.TranscriptSRTPath == "" || task.TranscriptTXTPath == "" {
			t.Fatalf("missing paths srt=%q txt=%q", task.TranscriptSRTPath, task.TranscriptTXTPath)
		}
		for _, p := range []string{task.TranscriptSRTPath, task.TranscriptTXTPath} {
			st, err := os.Stat(p)
			if err != nil || st.Size() == 0 {
				t.Fatalf("bad file %s: %v", p, err)
			}
		}
		t.Logf("ok srt=%s txt=%s lang=%s", filepath.Base(task.TranscriptSRTPath), filepath.Base(task.TranscriptTXTPath), task.TranscriptLang)
	case <-time.After(90 * time.Second):
		t.Fatal("timeout")
	}
}
