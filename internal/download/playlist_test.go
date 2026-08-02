package download

import (
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestPlaylistCancelStopsInFlightTasks(t *testing.T) {
	svc := NewService(t.TempDir(), 2).(*Service)

	pl := model.NewPlaylist("https://youtube.com/playlist?list=PLtest")
	pl.ID = "pl-1"
	pl.Title = "Test"
	pl.Status = model.PlaylistStatusDownloading
	pl.Mode = model.DownloadModeVideo
	for i := 0; i < 3; i++ {
		pl.AddVideo(&model.PlaylistVideo{
			ID:     "v" + string(rune('a'+i)),
			URL:    "https://example.invalid/watch?v=pl" + string(rune('a'+i)),
			Status: model.VideoStatusPending,
		})
	}
	// Register without queueing processPlaylist workers.
	svc.playlistsMutex.Lock()
	svc.playlists[pl.ID] = pl
	svc.playlistsMutex.Unlock()

	// Seed one in-flight task linked to first video (simulates mid-download).
	task := &model.DownloadTask{
		ID:     "task-pl-1",
		URL:    pl.Videos[0].URL,
		Status: model.TaskStatusDownloading,
		Mode:   model.DownloadModeVideo,
	}
	svc.tasksMutex.Lock()
	svc.tasks[task.ID] = task
	svc.activeCount = 1
	svc.tasksMutex.Unlock()
	pl.Videos[0].Status = model.VideoStatusDownloading
	pl.Videos[0].TaskID = task.ID

	if err := svc.CancelPlaylist(pl.ID); err != nil {
		t.Fatal(err)
	}
	if !pl.Cancelled || pl.Status != model.PlaylistStatusCancelled {
		t.Fatalf("playlist not cancelled: %+v", pl)
	}
	// StopTask should have moved active task toward Stopping/Stopped.
	if task.Status != model.TaskStatusStopping && task.Status != model.TaskStatusStopped {
		t.Fatalf("expected in-flight task stopped, got %s", task.Status)
	}
	if pl.Videos[0].Status != model.VideoStatusSkipped {
		t.Fatalf("video status=%s want skipped", pl.Videos[0].Status)
	}
}

func TestPlaylistModeAppliedToItems(t *testing.T) {
	svc := NewService(t.TempDir(), 0).(*Service) // no auto-start
	pl := model.NewPlaylist("https://youtube.com/playlist?list=PLmode")
	pl.ID = "pl-mode"
	pl.Status = model.PlaylistStatusDownloading
	pl.Mode = model.DownloadModeTranscriptOnly
	pl.TranscriptLang = "ru"
	pl.AddVideo(&model.PlaylistVideo{
		ID:     "vid1",
		URL:    "https://example.invalid/watch?v=mode1",
		Status: model.VideoStatusPending,
	})

	// downloadPlaylistVideo will AddTaskWithOptions then wait; with maxParallel 0
	// task stays Pending forever — run in goroutine and cancel quickly after add.
	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.downloadPlaylistVideo(pl, pl.Videos[0])
	}()

	// Wait until task is created with correct mode.
	deadline := time.Now().Add(2 * time.Second)
	var task *model.DownloadTask
	for time.Now().Before(deadline) {
		task = svc.findTaskByURL(pl.Videos[0].URL)
		if task != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if task == nil {
		t.Fatal("task was not created")
	}
	if task.Mode != model.DownloadModeTranscriptOnly {
		t.Fatalf("mode=%s want transcript_only", task.Mode)
	}
	if task.TranscriptLang != "ru" {
		t.Fatalf("lang=%s want ru", task.TranscriptLang)
	}

	svc.playlistStateMutex.Lock()
	pl.Cancelled = true
	svc.playlistStateMutex.Unlock()
	_ = svc.StopTask(task.ID)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("downloadPlaylistVideo did not exit after cancel")
	}
}

func TestRetryFailedPlaylistItems(t *testing.T) {
	svc := NewService(t.TempDir(), 0).(*Service)
	pl := model.NewPlaylist("https://youtube.com/playlist?list=PLretry")
	pl.ID = "pl-retry"
	pl.Status = model.PlaylistStatusCompleted
	pl.AddVideo(&model.PlaylistVideo{ID: "ok", URL: "https://x/ok", Status: model.VideoStatusCompleted})
	pl.AddVideo(&model.PlaylistVideo{ID: "bad", URL: "https://x/bad", Status: model.VideoStatusError, Error: "boom"})
	svc.playlists[pl.ID] = pl

	if err := svc.RetryFailedPlaylistItems(pl.ID); err != nil {
		t.Fatal(err)
	}
	if pl.Videos[1].Status != model.VideoStatusPending {
		t.Fatalf("expected pending after retry, got %s", pl.Videos[1].Status)
	}
	if pl.Status != model.PlaylistStatusDownloading {
		t.Fatalf("status=%s", pl.Status)
	}
	// Stop background processPlaylist from hanging on network-less waits.
	_ = svc.CancelPlaylist(pl.ID)
	time.Sleep(30 * time.Millisecond)
}
