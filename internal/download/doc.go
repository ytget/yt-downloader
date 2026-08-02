// Package download implements the core download pipeline built on top of
// github.com/ytget/ytdlp/v2 (pure Go engine). It manages task lifecycle,
// concurrency limits, progress propagation to UI, captions/transcripts, and
// playlist chunk processing.
//
// # Service invariants
//
//   - activeCount is the number of runTask workers currently executing. It is
//     incremented only when a Pending task is claimed (Status → Starting) and
//     decremented exactly once in finishWorker.
//   - A task may be claimed only via queueStartLocked / startTask while it is
//     still present in the tasks map and Status is Pending (or already Starting
//     for the startTask wrapper path).
//   - stopModes records user intent when Status becomes Stopping:
//     StopModePause → terminal Paused (keep OutputPath for resume/.part);
//     StopModeStop  → terminal Stopped (hard stop / remove).
//   - RemoveTask deletes the task from the map immediately and, if active,
//     records StopModeStop so the worker cancels and frees its slot. The
//     worker-owned task is not mutated after removal; this keeps update
//     snapshots race-free.
//   - ResumeTask keeps media paths; RestartTask clears media + transcript
//     outputs/warnings and resets transcript status from Mode.
//   - Duplicate URL policy: reject while any non-terminal task (including
//     Paused) exists for the same URL; allow re-add after Completed/Stopped/Error.
//   - Smoothing timers live in smoothingState and must be stopped on remove /
//     worker finish to avoid leaks after terminal states.
package download
