package model

// TaskStatus represents the status of a download or compression task
type TaskStatus string

const (
	// TaskStatusPending means the task is queued but not started
	TaskStatusPending TaskStatus = "Pending"

	// TaskStatusStarting means the task is in the process of starting
	TaskStatusStarting TaskStatus = "Starting"

	// TaskStatusDownloading means the download is in progress
	TaskStatusDownloading TaskStatus = "Downloading"

	// TaskStatusStopping means the task is in the process of stopping
	TaskStatusStopping TaskStatus = "Stopping"

	// TaskStatusStopped means the task was stopped by user
	TaskStatusStopped TaskStatus = "Stopped"

	// TaskStatusCompleted means the task finished successfully
	TaskStatusCompleted TaskStatus = "Completed"

	// TaskStatusError means the task failed with an error
	TaskStatusError TaskStatus = "Error"
)

// String returns the string representation of TaskStatus
func (ts TaskStatus) String() string {
	return string(ts)
}

// IsActive returns true if the task is in an active state
func (ts TaskStatus) IsActive() bool {
	return ts == TaskStatusStarting || ts == TaskStatusDownloading || ts == TaskStatusStopping
}

// IsFinished returns true if the task is in a finished state (completed, stopped, or error)
func (ts TaskStatus) IsFinished() bool {
	return ts == TaskStatusCompleted || ts == TaskStatusStopped || ts == TaskStatusError
}
