package compress

import (
	"github.com/ytget/yt-downloader/internal/model"
)

// Compressor defines the interface for the compression service.
type Compressor interface {
	SetUpdateCallback(func(*model.CompressionTask))
	StartCompression(inputPath string) (*model.CompressionTask, error)
	StopCompression(taskID string) error
	GetTask(taskID string) (*model.CompressionTask, bool)
}
