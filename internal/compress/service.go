package compress

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/romanitalian/yt-downloader/internal/model"
)

// FFmpeg constants for compression settings
const (
	// Video codec settings
	VideoCodec  = "libx264"
	VideoPreset = "medium"
	VideoCRF    = "23"

	// Audio codec settings
	AudioCodec   = "aac"
	AudioBitrate = "128k"

	// Container flags
	FastStartFlag = "+faststart"

	// Output suffix
	CompressedSuffix = "-compressed"
)

// Service handles video compression operations
type Service struct {
	tasks      map[string]*model.CompressionTask
	tasksMutex sync.RWMutex
	onUpdate   func(*model.CompressionTask) // callback for UI updates
}

// NewService creates a new compression service
func NewService() *Service {
	return &Service{
		tasks: make(map[string]*model.CompressionTask),
	}
}

// SetUpdateCallback sets the callback function for task updates
func (s *Service) SetUpdateCallback(callback func(*model.CompressionTask)) {
	s.onUpdate = callback
}

// StartCompression starts compressing a video file
func (s *Service) StartCompression(inputPath string) (*model.CompressionTask, error) {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	// Check if compression is already in progress for this file
	for _, task := range s.tasks {
		if task.InputPath == inputPath && task.Status.IsActive() {
			return nil, fmt.Errorf("compression already in progress for file: %s", inputPath)
		}
	}

	// Check if input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file does not exist: %s", inputPath)
	}

	// Generate output path
	outputPath := generateOutputPath(inputPath)

	task := &model.CompressionTask{
		ID:         generateTaskID(),
		InputPath:  inputPath,
		OutputPath: outputPath,
		Status:     model.TaskStatusPending,
		Progress:   0.0,
		Percent:    0,
		StartedAt:  time.Now(),
	}

	s.tasks[task.ID] = task

	// Start compression in background
	go s.startCompression(task)

	return task, nil
}

// StopCompression stops a running compression task
func (s *Service) StopCompression(taskID string) error {
	s.tasksMutex.Lock()
	defer s.tasksMutex.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("compression task not found: %s", taskID)
	}

	if !task.Status.IsActive() {
		return fmt.Errorf("compression task is not active: %s", task.Status)
	}

	// Set stopping status
	task.Status = model.TaskStatusStopping
	s.notifyUpdate(task)

	return nil
}

// GetTask returns a compression task by ID
func (s *Service) GetTask(taskID string) (*model.CompressionTask, bool) {
	s.tasksMutex.RLock()
	defer s.tasksMutex.RUnlock()
	task, exists := s.tasks[taskID]
	return task, exists
}

// startCompression performs the actual compression
func (s *Service) startCompression(task *model.CompressionTask) {
	// Update status to starting
	s.tasksMutex.Lock()
	task.Status = model.TaskStatusStarting
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	// Get duration of input file for progress calculation
	duration, err := s.getVideoDuration(task.InputPath)
	if err != nil {
		log.Printf("Failed to get video duration for %s: %v", task.InputPath, err)
		s.setTaskError(task, err)
		return
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Monitor for stop requests
	go func() {
		for {
			s.tasksMutex.RLock()
			status := task.Status
			s.tasksMutex.RUnlock()

			if status == model.TaskStatusStopping {
				cancel()
				return
			}
			if status.IsFinished() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Update status to downloading
	s.tasksMutex.Lock()
	task.Status = model.TaskStatusDownloading
	s.tasksMutex.Unlock()
	s.notifyUpdate(task)

	// Build ffmpeg command
	args := s.buildFFmpegArgs(task.InputPath, task.OutputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	// Setup progress monitoring
	stderr, err := cmd.StderrPipe()
	if err != nil {
		s.setTaskError(task, fmt.Errorf("failed to create stderr pipe: %w", err))
		return
	}

	// Start ffmpeg process
	if err := cmd.Start(); err != nil {
		s.setTaskError(task, fmt.Errorf("failed to start ffmpeg: %w", err))
		return
	}

	// Monitor progress
	go s.monitorProgress(stderr, task, duration)

	// Wait for completion
	err = cmd.Wait()

	// Handle result
	s.tasksMutex.Lock()
	if ctx.Err() == context.Canceled {
		task.Status = model.TaskStatusStopped
		// Remove partial output file
		os.Remove(task.OutputPath)
	} else if err != nil {
		task.Status = model.TaskStatusError
		task.LastError = err.Error()
		// Remove partial output file
		os.Remove(task.OutputPath)
	} else {
		task.Status = model.TaskStatusCompleted
		task.Progress = 1.0
		task.Percent = 100
	}
	task.FinishedAt = time.Now()
	s.tasksMutex.Unlock()

	s.notifyUpdate(task)
}

// buildFFmpegArgs builds the ffmpeg command arguments
func (s *Service) buildFFmpegArgs(inputPath, outputPath string) []string {
	return []string{
		"-y",            // Overwrite output file
		"-i", inputPath, // Input file
		"-c:v", VideoCodec, // Video codec
		"-preset", VideoPreset, // Encoding preset
		"-crf", VideoCRF, // Constant rate factor
		"-c:a", AudioCodec, // Audio codec
		"-b:a", AudioBitrate, // Audio bitrate
		"-movflags", FastStartFlag, // MP4 optimization
		"-progress", "pipe:2", // Progress to stderr
		"-nostats", // No stats output
		outputPath, // Output file
	}
}

// getVideoDuration gets the duration of a video file using ffprobe
func (s *Service) getVideoDuration(filePath string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", filePath)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run ffprobe: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// monitorProgress monitors ffmpeg progress output
func (s *Service) monitorProgress(stderr io.ReadCloser, task *model.CompressionTask, totalDuration float64) {
	defer stderr.Close()
	scanner := bufio.NewScanner(stderr)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse progress line: out_time_us=123456
		if strings.HasPrefix(line, "out_time_us=") {
			timeStr := strings.TrimPrefix(line, "out_time_us=")
			timeMicroseconds, err := strconv.ParseInt(timeStr, 10, 64)
			if err != nil {
				continue
			}

			// Convert to seconds
			timeSeconds := float64(timeMicroseconds) / 1000000.0

			// Calculate progress percentage
			if totalDuration > 0 {
				progress := timeSeconds / totalDuration
				if progress > 1.0 {
					progress = 1.0
				}

				s.tasksMutex.Lock()
				task.Progress = progress
				task.Percent = int(progress * 100)
				s.tasksMutex.Unlock()

				s.notifyUpdate(task)
			}
		}
	}
}

// setTaskError sets an error state for a task
func (s *Service) setTaskError(task *model.CompressionTask, err error) {
	s.tasksMutex.Lock()
	task.Status = model.TaskStatusError
	task.LastError = err.Error()
	task.FinishedAt = time.Now()
	s.tasksMutex.Unlock()

	s.notifyUpdate(task)
}

// notifyUpdate calls the update callback if set
func (s *Service) notifyUpdate(task *model.CompressionTask) {
	if s.onUpdate != nil {
		s.onUpdate(task)
	}
}

// generateOutputPath generates the output path for compressed file
func generateOutputPath(inputPath string) string {
	ext := filepath.Ext(inputPath)
	baseName := strings.TrimSuffix(inputPath, ext)
	return baseName + CompressedSuffix + ".mp4"
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("compress-%d", time.Now().UnixNano())
}
