package download

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ytget/yt-downloader/internal/model"
)

// TestPauseRemoveAddStorm hammers lifecycle APIs without relying on live YouTube.
// Workers exit quickly because tasks are removed before/during runTask.
func TestPauseRemoveAddStorm(t *testing.T) {
	svc := NewService(t.TempDir(), 4).(*Service)

	const n = 80
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("storm-%d", i)
			task := &model.DownloadTask{
				ID:     id,
				URL:    "https://example.invalid/watch?v=" + id,
				Status: model.TaskStatusPending,
				Mode:   model.DownloadModeVideo,
			}
			svc.tasksMutex.Lock()
			svc.tasks[id] = task
			svc.queueStartLocked(task)
			svc.tasksMutex.Unlock()

			switch i % 3 {
			case 0:
				_ = svc.PauseTask(id)
				_ = svc.RemoveTask(id)
			case 1:
				_ = svc.StopTask(id)
				_ = svc.RemoveTask(id)
			default:
				_ = svc.RemoveTask(id)
			}
		}(i)
	}
	wg.Wait()

	// Allow workers to observe removal and finishWorker.
	deadline := time.Now().Add(3 * time.Second)
	for {
		svc.tasksMutex.RLock()
		active := svc.activeCount
		left := len(svc.tasks)
		stops := len(svc.stopModes)
		svc.tasksMutex.RUnlock()

		svc.smoothingMutex.RLock()
		smooth := len(svc.smoothingState)
		svc.smoothingMutex.RUnlock()

		if active == 0 && left == 0 && stops == 0 && smooth == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("leaked state after storm: active=%d tasks=%d stopModes=%d smoothing=%d",
				active, left, stops, smooth)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSmoothingTimerDoesNotRestartAfterStop(t *testing.T) {
	svc := NewService(t.TempDir(), 0).(*Service)
	id := "smooth-stop"
	task := &model.DownloadTask{
		ID:     id,
		URL:    "https://example.invalid/x",
		Status: model.TaskStatusDownloading,
	}
	svc.tasks[id] = task
	svc.startSmoothingTimer(id)
	svc.stopSmoothingTimer(id)

	// Manually invoke callback path as if a timer fired late.
	svc.updateSmoothedUI(id)

	svc.smoothingMutex.RLock()
	state := svc.smoothingState[id]
	svc.smoothingMutex.RUnlock()
	if state == nil {
		// deleted path is also fine
		return
	}
	state.mutex.Lock()
	defer state.mutex.Unlock()
	if state.timer != nil {
		t.Fatal("timer should not restart after stop")
	}
	if !state.stopped {
		t.Fatal("expected stopped flag")
	}
}
