package ui

import (
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/romanitalian/yt-downloader/internal/model"
)

// PlaylistGroup represents a group of playlists with unified display
type PlaylistGroup struct {
	window       fyne.Window
	localization *Localization

	// Playlists data
	playlists        []*model.Playlist
	selectedPlaylist *model.Playlist
	individualVideos []*model.DownloadTask // Individual video downloads
	allVideos        []interface{}         // Unified list of all videos (PlaylistVideo + DownloadTask)

	// UI components
	container *fyne.Container
	list      *widget.List

	// Callbacks
	onDownloadPlaylist func(*model.Playlist)
	onCancelPlaylist   func(*model.Playlist)

	// TaskRow callbacks
	onStartPause func(taskID string)
	onReveal     func(filePath string)
	onOpen       func(filePath string)
	onCopyPath   func(filePath string)
	onRemove     func(taskID string)
}

// NewPlaylistGroup creates a new playlist group UI component
func NewPlaylistGroup(window fyne.Window, localization *Localization) *PlaylistGroup {
	pg := &PlaylistGroup{
		playlists:        make([]*model.Playlist, 0),
		selectedPlaylist: nil,
		individualVideos: make([]*model.DownloadTask, 0),
		allVideos:        make([]interface{}, 0),
		window:           window,
		localization:     localization,
	}

	pg.createUI()
	return pg
}

// createUI creates the user interface for the playlist group
func (pg *PlaylistGroup) createUI() {
	// Create List for videos instead of VBox
	pg.list = widget.NewList(
		func() int {
			return len(pg.allVideos)
		},
		func() fyne.CanvasObject {
			return pg.createVideoRow()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			pg.updateVideoRow(id, obj)
		},
	)

	// Wrap videos list in scroll container for long playlists
	scrollContainer := container.NewScroll(pg.list)

	// Main container - videos list with scroll only
	pg.container = container.NewBorder(
		nil,             // no top header
		nil,             // no bottom buttons
		nil,             // left
		nil,             // right
		scrollContainer, // center - scrollable videos list
	)
}

// createVideoRow creates a template video row widget
func (pg *PlaylistGroup) createVideoRow() fyne.CanvasObject {
	// Create a minimal task for the template - this will be replaced with real data
	dummyTask := &model.DownloadTask{
		ID:     "template",
		Status: model.TaskStatusPending,
		Title:  "",
	}

	// Use TaskRow for consistent display
	taskRow := NewTaskRow(dummyTask, pg.localization)

	// Set callbacks for playlist videos using stored callbacks
	taskRow.SetCallbacks(
		func(taskID string) {
			// Handle start/pause for playlist videos
			if pg.onStartPause != nil {
				pg.onStartPause(taskID)
			} else {
				log.Printf("Start/Pause requested for playlist video: %s", taskID)
			}
		},
		func(filePath string) {
			// Handle reveal for playlist videos
			if pg.onReveal != nil {
				pg.onReveal(filePath)
			} else {
				log.Printf("Reveal requested for playlist video: %s", filePath)
			}
		},
		func(filePath string) {
			// Handle open for playlist videos
			if pg.onOpen != nil {
				pg.onOpen(filePath)
			} else {
				log.Printf("Open requested for playlist video: %s", filePath)
			}
		},
		func(filePath string) {
			// Handle copy path for playlist videos
			if pg.onCopyPath != nil {
				pg.onCopyPath(filePath)
			} else {
				log.Printf("Copy path requested for playlist video: %s", filePath)
			}
		},
		func(taskID string) {
			// Handle remove for playlist videos
			if pg.onRemove != nil {
				pg.onRemove(taskID)
			} else {
				log.Printf("Remove requested for playlist video: %s", taskID)
			}
		},
	)

	return taskRow
}

// convertVideoStatusToTaskStatus converts VideoStatus to TaskStatus
func (pg *PlaylistGroup) convertVideoStatusToTaskStatus(videoStatus model.VideoStatus) model.TaskStatus {
	switch videoStatus {
	case model.VideoStatusPending:
		return model.TaskStatusPending
	case model.VideoStatusDownloading:
		return model.TaskStatusDownloading
	case model.VideoStatusPaused:
		return model.TaskStatusPaused
	case model.VideoStatusCompleted:
		return model.TaskStatusCompleted
	case model.VideoStatusError:
		return model.TaskStatusError
	case model.VideoStatusSkipped:
		return model.TaskStatusStopped
	default:
		return model.TaskStatusPending
	}
}

// updateVideoRow updates a video row with actual data
func (pg *PlaylistGroup) updateVideoRow(id widget.ListItemID, obj fyne.CanvasObject) {
	if id >= len(pg.allVideos) {
		log.Printf("Warning: updateVideoRow called with invalid ID %d, total videos: %d", id, len(pg.allVideos))
		return
	}

	videoItem := pg.allVideos[id]
	taskRow, ok := obj.(*TaskRow)
	if !ok {
		log.Printf("Warning: expected TaskRow but got %T", obj)
		return
	}

	log.Printf("Updating video row %d with item: %T, ID: %v", id, videoItem, videoItem)

	// Update the task row based on video type
	if video, ok := videoItem.(*model.PlaylistVideo); ok {
		// Convert PlaylistVideo to DownloadTask for TaskRow
		task := &model.DownloadTask{
			ID:         video.ID,
			Title:      video.Title,
			Duration:   video.Duration,
			Status:     pg.convertVideoStatusToTaskStatus(video.Status), // Convert VideoStatus to TaskStatus
			Progress:   video.Progress,
			Percent:    int(video.Progress * 100),
			URL:        video.URL,        // Add URL for fallback display
			OutputPath: video.OutputPath, // Add OutputPath for reveal/open functionality
			FileSize:   video.FileSize,   // Add FileSize for display
			// Propagate runtime telemetry so TaskRow can render speed/ETA
			Speed:  video.Speed,
			ETASec: video.ETASec,
		}

		log.Printf("Updating TaskRow for PlaylistVideo %s: Status=%s, OutputPath=%s, FileSize=%d",
			video.ID, video.Status, video.OutputPath, video.FileSize)

		// Update in UI thread to avoid Fyne call thread errors
		fyne.Do(func() {
			taskRow.UpdateTask(task)
		})
	} else if task, ok := videoItem.(*model.DownloadTask); ok {
		// Individual video task - update directly in UI thread
		log.Printf("Updating TaskRow for DownloadTask %s: Status=%s, OutputPath=%s, FileSize=%d",
			task.ID, task.Status, task.OutputPath, task.FileSize)

		fyne.Do(func() {
			taskRow.UpdateTask(task)
		})
	} else {
		log.Printf("Warning: unknown video item type: %T", videoItem)
	}
}

// Container returns the main container of the playlist group
func (pg *PlaylistGroup) Container() *fyne.Container {
	return pg.container
}

// SetCallbacks sets the callback functions for playlist actions (kept for compatibility)
func (pg *PlaylistGroup) SetCallbacks(
	onDownloadPlaylist func(*model.Playlist),
	onCancelPlaylist func(*model.Playlist),
) {
	pg.onDownloadPlaylist = onDownloadPlaylist
	pg.onCancelPlaylist = onCancelPlaylist
}

// SetTaskRowCallbacks sets the callback functions for TaskRow actions
func (pg *PlaylistGroup) SetTaskRowCallbacks(
	onStartPause func(taskID string),
	onReveal func(filePath string),
	onOpen func(filePath string),
	onCopyPath func(filePath string),
	onRemove func(taskID string),
) {
	pg.onStartPause = onStartPause
	pg.onReveal = onReveal
	pg.onOpen = onOpen
	pg.onCopyPath = onCopyPath
	pg.onRemove = onRemove
}

// UpdatePlaylistProgress updates the progress display for a playlist
func (pg *PlaylistGroup) UpdatePlaylistProgress(playlistID string, progress float64) {
	for _, playlist := range pg.playlists {
		if playlist.ID == playlistID {
			playlist.UpdateVideoProgress("", progress)
			pg.refreshVideosDisplay()
			break
		}
	}
}

// GetSelectedPlaylist returns the currently selected playlist
func (pg *PlaylistGroup) GetSelectedPlaylist() *model.Playlist {
	return pg.selectedPlaylist
}

// ClearPlaylists clears all playlists from the list
func (pg *PlaylistGroup) ClearPlaylists() {
	pg.playlists = make([]*model.Playlist, 0)
	pg.selectedPlaylist = nil
	pg.refreshVideosDisplay()
}

// AddPlaylist adds a new playlist to the list
func (pg *PlaylistGroup) AddPlaylist(playlist *model.Playlist) {
	log.Printf("PlaylistGroup.AddPlaylist called with playlist: %s, %d videos", playlist.Title, len(playlist.Videos))

	pg.playlists = append(pg.playlists, playlist)
	pg.selectedPlaylist = playlist

	log.Printf("Playlist added to internal list, total playlists: %d", len(pg.playlists))

	// Update UI
	log.Printf("Refreshing videos display...")
	pg.refreshVideosDisplay()

	log.Printf("PlaylistGroup.AddPlaylist completed")
}

// AddIndividualVideo adds an individual video download task
func (pg *PlaylistGroup) AddIndividualVideo(task *model.DownloadTask) {
	pg.individualVideos = append(pg.individualVideos, task)
	pg.refreshVideosDisplay()
}

// refreshVideosDisplay rebuilds the videos display with all videos (playlist + individual)
func (pg *PlaylistGroup) refreshVideosDisplay() {
	log.Printf("Refreshing videos display...")

	// Clear existing content
	pg.allVideos = make([]interface{}, 0)

	// Combine all videos into one list
	// Add playlist videos if available
	if pg.selectedPlaylist != nil && len(pg.selectedPlaylist.Videos) > 0 {
		log.Printf("Adding %d playlist videos to display", len(pg.selectedPlaylist.Videos))
		for _, video := range pg.selectedPlaylist.Videos {
			log.Printf("Adding playlist video: ID=%s, Status=%s, OutputPath=%s", video.ID, video.Status, video.OutputPath)
			pg.allVideos = append(pg.allVideos, video)
		}
	}

	// Add individual videos if available
	log.Printf("Adding %d individual videos to display", len(pg.individualVideos))
	for _, task := range pg.individualVideos {
		log.Printf("Adding individual video: ID=%s, Status=%s, OutputPath=%s", task.ID, task.Status, task.OutputPath)
		pg.allVideos = append(pg.allVideos, task)
	}

	log.Printf("Total videos in display: %d", len(pg.allVideos))

	// Refresh the list to update UI
	pg.list.Refresh()
}

// UpdateVideoStatus updates the status of a specific video
func (pg *PlaylistGroup) UpdateVideoStatus(videoID string, status interface{}) {
	updated := false

	// Update playlist videos
	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.ID == videoID {
				if newStatus, ok := status.(model.VideoStatus); ok {
					if video.Status != newStatus {
						video.Status = newStatus
						updated = true
					}
				}
				break
			}
		}
	}

	// Update individual videos
	for _, task := range pg.individualVideos {
		if task.ID == videoID {
			if newStatus, ok := status.(model.TaskStatus); ok {
				if task.Status != newStatus {
					task.Status = newStatus
					updated = true
				}
			}
			break
		}
	}

	// Only refresh UI if something actually changed
	if updated {
		fyne.Do(func() {
			pg.list.Refresh()
		})
	}
}

// UpdateVideoProgress updates the progress of a specific video
func (pg *PlaylistGroup) UpdateVideoProgress(videoID string, progress float64) {
	log.Printf("STEP[PlaylistGroup] UpdateVideoProgress: id=%s progress=%.2f", videoID, progress)
	updated := false

	// Update playlist videos
	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.ID == videoID && video.Progress != progress {
				video.Progress = progress
				// Keep UI responsive by refreshing when value changes
				updated = true
				log.Printf("STEP[PlaylistGroup] playlist video updated: id=%s progress=%.2f", videoID, progress)
				break
			}
		}
	}

	// Update individual videos
	for _, task := range pg.individualVideos {
		if task.ID == videoID {
			// Even если значение уже совпадает (тот же указатель), нам нужна перерисовка строки
			task.Progress = progress
			task.Percent = int(progress * 100)
			updated = true
			log.Printf("STEP[PlaylistGroup] individual task updated: id=%s progress=%.2f percent=%d", videoID, progress, task.Percent)
			break
		}
	}

	// Only refresh UI if something actually changed
	if updated {
		fyne.Do(func() {
			log.Printf("STEP[PlaylistGroup] list refreshed after progress update")
			pg.list.Refresh()
		})
	}
}

// UpdateVideoSpeed updates the speed and ETA for a specific video by internal ID
func (pg *PlaylistGroup) UpdateVideoSpeed(videoID string, speed string, etaSec int) {
	updated := false

	// Update playlist videos
	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.ID == videoID {
				if video.Speed != speed || video.ETASec != etaSec {
					video.Speed = speed
					video.ETASec = etaSec
					updated = true
				}
				break
			}
		}
	}

	// Update individual videos
	for _, task := range pg.individualVideos {
		if task.ID == videoID {
			if task.Speed != speed || task.ETASec != etaSec {
				task.Speed = speed
				task.ETASec = etaSec
				updated = true
			}
			break
		}
	}

	if updated {
		fyne.Do(func() { pg.list.Refresh() })
	}
}

// UpdateVideoSpeedByURL updates speed and ETA using video URL mapping
func (pg *PlaylistGroup) UpdateVideoSpeedByURL(videoURL string, speed string, etaSec int) {
	updated := false

	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.URL == videoURL {
				if video.Speed != speed || video.ETASec != etaSec {
					video.Speed = speed
					video.ETASec = etaSec
					updated = true
				}
				break
			}
		}
	}

	if updated {
		fyne.Do(func() { pg.list.Refresh() })
	}
}

// UpdateVideoOutputPath updates the output path of a specific video
func (pg *PlaylistGroup) UpdateVideoOutputPath(videoID string, outputPath string, fileSize int64) {
	updated := false

	// Update playlist videos
	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.ID == videoID {
				if video.OutputPath != outputPath || video.FileSize != fileSize {
					video.OutputPath = outputPath
					video.FileSize = fileSize
					updated = true
				}
				break
			}
		}
	}

	// Update individual videos
	for _, task := range pg.individualVideos {
		if task.ID == videoID {
			if task.OutputPath != outputPath || task.FileSize != fileSize {
				task.OutputPath = outputPath
				task.FileSize = fileSize
				updated = true
			}
			break
		}
	}

	// Only refresh UI if something actually changed
	if updated {
		fyne.Do(func() {
			// Refresh the list to update UI
			pg.list.Refresh()

			// Also refresh the specific video row to update buttons
			pg.refreshVideosDisplay()
		})
	}
}

// UpdateVideoProgressByURL updates progress of a playlist video identified by its URL
func (pg *PlaylistGroup) UpdateVideoProgressByURL(videoURL string, progress float64) {
	updated := false

	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.URL == videoURL && video.Progress != progress {
				video.Progress = progress
				updated = true
				break
			}
		}
	}

	if updated {
		fyne.Do(func() { pg.list.Refresh() })
	}
}

// UpdateVideoStatusByURL updates status of a playlist video identified by its URL
func (pg *PlaylistGroup) UpdateVideoStatusByURL(videoURL string, status interface{}) {
	updated := false

	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.URL == videoURL {
				if newStatus, ok := status.(model.VideoStatus); ok {
					if video.Status != newStatus {
						video.Status = newStatus
						updated = true
					}
				} else if newTaskStatus, ok := status.(model.TaskStatus); ok {
					mapped := model.VideoStatusPending
					switch newTaskStatus {
					case model.TaskStatusDownloading, model.TaskStatusStarting:
						mapped = model.VideoStatusDownloading
					case model.TaskStatusPaused:
						mapped = model.VideoStatusPaused
					case model.TaskStatusCompleted:
						mapped = model.VideoStatusCompleted
					case model.TaskStatusError:
						mapped = model.VideoStatusError
					case model.TaskStatusStopped:
						mapped = model.VideoStatusSkipped
					default:
						mapped = model.VideoStatusPending
					}
					if video.Status != mapped {
						video.Status = mapped
						updated = true
					}
				}
				break
			}
		}
	}

	if updated {
		fyne.Do(func() { pg.list.Refresh() })
	}
}

// UpdateVideoOutputPathByURL updates output path of a playlist video identified by its URL
func (pg *PlaylistGroup) UpdateVideoOutputPathByURL(videoURL string, outputPath string, fileSize int64) {
	updated := false

	if pg.selectedPlaylist != nil {
		for _, video := range pg.selectedPlaylist.Videos {
			if video.URL == videoURL {
				if video.OutputPath != outputPath || video.FileSize != fileSize {
					video.OutputPath = outputPath
					video.FileSize = fileSize
					updated = true
				}
				break
			}
		}
	}

	if updated {
		fyne.Do(func() {
			pg.list.Refresh()
			pg.refreshVideosDisplay()
		})
	}
}
