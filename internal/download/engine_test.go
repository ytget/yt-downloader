package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
	"github.com/ytget/ytdlp/v2"
	"github.com/ytget/ytdlp/v2/types"
)

type stubEngine struct {
	resolveInfo *ytdlp.VideoInfo
	resolveErr  error
	dlInfo      *ytdlp.VideoInfo
	dlErr       error
	caps        *types.Captions
	track       types.CaptionTrack
	capsErr     error
	calls       []string
}

func (s *stubEngine) Resolve(ctx context.Context, req MediaRequest) (*ytdlp.VideoInfo, error) {
	s.calls = append(s.calls, "resolve")
	return s.resolveInfo, s.resolveErr
}

func (s *stubEngine) Download(ctx context.Context, req MediaRequest) (*ytdlp.VideoInfo, error) {
	s.calls = append(s.calls, "download")
	if req.OutputPath != "" && s.dlErr == nil {
		_ = os.WriteFile(req.OutputPath, []byte("stub-media"), 0o644)
	}
	return s.dlInfo, s.dlErr
}

func (s *stubEngine) GetVideoInfo(ctx context.Context, videoURL string) (*ytdlp.VideoInfo, error) {
	s.calls = append(s.calls, "info")
	return s.resolveInfo, s.resolveErr
}

func (s *stubEngine) DownloadCaptions(ctx context.Context, videoURL, preferredLang string) (*types.Captions, types.CaptionTrack, error) {
	s.calls = append(s.calls, "caps")
	return s.caps, s.track, s.capsErr
}

func TestServiceUsesInjectedEngine(t *testing.T) {
	dir := t.TempDir()
	stub := &stubEngine{
		resolveInfo: &ytdlp.VideoInfo{
			Title:   "Stub Title",
			Formats: []types.Format{{MimeType: "video/mp4"}},
		},
		dlInfo: &ytdlp.VideoInfo{Title: "Stub Title"},
	}
	svc := NewService(dir, 1).(*Service)
	svc.SetMediaEngine(stub)
	svc.SetCaptionsEngine(stub)

	task, err := svc.AddTaskWithOptions(TaskOptions{
		URL:  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Mode: model.DownloadModeVideo,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := svc.GetTask(task.ID)
		if ok && got.Status.IsFinished() {
			if got.Status != model.TaskStatusCompleted {
				t.Fatalf("status=%s err=%s", got.Status, got.LastError)
			}
			if got.Title != "Stub Title" {
				t.Fatalf("title=%q", got.Title)
			}
			if _, err := os.Stat(got.OutputPath); err != nil {
				t.Fatalf("output missing: %v", err)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, _ := svc.GetTask(task.ID)
	if got == nil || !got.Status.IsFinished() {
		t.Fatal("task did not finish")
	}
	if len(stub.calls) < 2 {
		t.Fatalf("expected resolve+download, calls=%v", stub.calls)
	}
}

func TestDisableCaptionsFlag(t *testing.T) {
	dir := t.TempDir()
	stub := &stubEngine{
		resolveInfo: &ytdlp.VideoInfo{Title: "X", Formats: []types.Format{{MimeType: "video/mp4"}}},
		dlInfo:      &ytdlp.VideoInfo{Title: "X"},
	}
	svc := NewService(dir, 1).(*Service)
	svc.SetMediaEngine(stub)
	svc.SetCaptionsEngine(stub)
	svc.SetFeatureFlags(true, false)

	_, err := svc.AddTaskWithOptions(TaskOptions{
		URL:  "https://www.youtube.com/watch?v=aaaaaaaaaaa",
		Mode: model.DownloadModeTranscriptOnly,
	})
	if err == nil {
		t.Fatal("expected captions disabled error")
	}

	task, err := svc.AddTaskWithOptions(TaskOptions{
		URL:  "https://www.youtube.com/watch?v=bbbbbbbbbbb",
		Mode: model.DownloadModeVideoAndTranscript,
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, ok := svc.GetTask(task.ID)
		if ok && got.Status.IsFinished() {
			if got.Status != model.TaskStatusCompleted {
				t.Fatalf("status=%s err=%q", got.Status, got.LastError)
			}
			// Caps engine should not be required; may still not call caps.
			_ = filepath.Base(got.OutputPath)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timeout")
}
