package download

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ytget/yt-downloader/internal/applog"
	"github.com/ytget/yt-downloader/internal/model"
)

const (
	queueFileName     = "queue.json"
	queueSaveDebounce = 400 * time.Millisecond
	maxPersistedTasks = 200
)

// persistedQueue is the on-disk queue/history snapshot.
type persistedQueue struct {
	Version int                   `json:"version"`
	SavedAt time.Time             `json:"saved_at"`
	Tasks   []*model.DownloadTask `json:"tasks"`
}

// QueueStore loads/saves download queue snapshots under the app config dir.
type QueueStore struct {
	mu      sync.Mutex
	path    string
	timer   *time.Timer
	pending []*model.DownloadTask
}

// NewQueueStore creates a store at the default config path.
func NewQueueStore() (*QueueStore, error) {
	path, err := defaultQueuePath()
	if err != nil {
		return nil, err
	}
	return &QueueStore{path: path}, nil
}

// NewQueueStoreAt is for tests.
func NewQueueStoreAt(path string) *QueueStore {
	return &QueueStore{path: path}
}

func defaultQueuePath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfgDir, "yt-downloader")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, queueFileName), nil
}

// Path returns the queue file path.
func (q *QueueStore) Path() string {
	if q == nil {
		return ""
	}
	return q.path
}

// ScheduleSave debounces a full snapshot write.
func (q *QueueStore) ScheduleSave(tasks []*model.DownloadTask) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = cloneTasks(tasks)
	if q.timer != nil {
		q.timer.Stop()
	}
	q.timer = time.AfterFunc(queueSaveDebounce, func() {
		q.mu.Lock()
		snap := q.pending
		q.pending = nil
		q.mu.Unlock()
		if err := q.Save(snap); err != nil {
			applog.Warn("queue save failed", "err", err)
		}
	})
}

// Save writes tasks immediately.
func (q *QueueStore) Save(tasks []*model.DownloadTask) error {
	if q == nil {
		return nil
	}
	// Cap history size (keep newest by StartedAt roughly via input order).
	if len(tasks) > maxPersistedTasks {
		tasks = tasks[:maxPersistedTasks]
	}
	payload := persistedQueue{
		Version: 1,
		SavedAt: time.Now().UTC(),
		Tasks:   cloneTasks(tasks),
	}
	// Normalize in-flight states for disk: active → Paused so restore is safe.
	for _, t := range payload.Tasks {
		if t == nil {
			continue
		}
		if t.Status.IsActive() {
			t.Status = model.TaskStatusPaused
			t.Speed = ""
		}
		if t.Status == model.TaskStatusPending {
			// keep pending
		}
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(q.path), 0o755); err != nil {
		return err
	}
	tmp := q.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, q.path)
}

// Load reads the queue file. Missing file → empty, nil error.
func (q *QueueStore) Load() ([]*model.DownloadTask, error) {
	if q == nil {
		return nil, nil
	}
	raw, err := os.ReadFile(q.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var payload persistedQueue
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return cloneTasks(payload.Tasks), nil
}

// Clear removes the queue file.
func (q *QueueStore) Clear() error {
	if q == nil {
		return nil
	}
	err := os.Remove(q.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func cloneTasks(in []*model.DownloadTask) []*model.DownloadTask {
	if len(in) == 0 {
		return nil
	}
	out := make([]*model.DownloadTask, 0, len(in))
	for _, t := range in {
		if t == nil {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}
	return out
}
