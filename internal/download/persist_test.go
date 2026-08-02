package download

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

func TestQueueStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	store := NewQueueStoreAt(path)
	tasks := []*model.DownloadTask{
		{
			ID:        "a",
			URL:       "https://youtu.be/x",
			Status:    model.TaskStatusDownloading,
			Title:     "Vid",
			Mode:      model.DownloadModeVideo,
			StartedAt: time.Now(),
		},
		{
			ID:     "b",
			URL:    "https://youtu.be/y",
			Status: model.TaskStatusCompleted,
			Title:  "Done",
		},
	}
	if err := store.Save(tasks); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Status != model.TaskStatusPaused {
		t.Fatalf("active should persist as paused, got %s", got[0].Status)
	}
	if got[1].Status != model.TaskStatusCompleted {
		t.Fatal(got[1].Status)
	}
}
