package config

import (
	"fyne.io/fyne/v2"
	"github.com/ytget/yt-downloader/internal/platform"
)

// Quality presets for downloads
type QualityPreset string

const (
	QualityBest   QualityPreset = "best"
	QualityMedium QualityPreset = "medium"
	QualityAudio  QualityPreset = "audio"
)

// Settings keys for Fyne preferences
const (
	KeyDownloadDir        = "download_directory"
	KeyMaxParallel        = "max_parallel_downloads"
	KeyQualityPreset      = "quality_preset"
	KeyFilenameTemplate   = "filename_template"
	KeyLanguage           = "app_language"
	KeyAutoRevealComplete = "auto_reveal_on_complete"
)

// Default values
const (
	DefaultMaxParallel        = 2
	DefaultQualityPreset      = QualityMedium
	DefaultFilenameTemplate   = "%(title)s.%(ext)s"
	DefaultLanguage           = "system"
	DefaultAutoRevealComplete = true
)

// Settings manages application configuration
type Settings struct {
	app fyne.App
}

// NewSettings creates a new settings manager
func NewSettings(app fyne.App) *Settings {
	return &Settings{app: app}
}

// GetDownloadDirectory returns the configured download directory
func (s *Settings) GetDownloadDirectory() string {
	dir := s.app.Preferences().String(KeyDownloadDir)
	if dir == "" {
		// Use system default Downloads directory
		defaultDir, err := platform.GetHomeDownloadsDir()
		if err != nil {
			defaultDir = "/tmp/downloads"
		}
		s.SetDownloadDirectory(defaultDir)
		return defaultDir
	}
	return dir
}

// SetDownloadDirectory sets the download directory
func (s *Settings) SetDownloadDirectory(dir string) {
	s.app.Preferences().SetString(KeyDownloadDir, dir)
}

// GetMaxParallelDownloads returns the maximum number of parallel downloads
func (s *Settings) GetMaxParallelDownloads() int {
	value := s.app.Preferences().Int(KeyMaxParallel)
	if value <= 0 {
		s.SetMaxParallelDownloads(DefaultMaxParallel)
		return DefaultMaxParallel
	}
	return value
}

// SetMaxParallelDownloads sets the maximum number of parallel downloads
func (s *Settings) SetMaxParallelDownloads(count int) {
	if count < 1 {
		count = 1
	}
	if count > 10 {
		count = 10
	}
	s.app.Preferences().SetInt(KeyMaxParallel, count)
}

// GetQualityPreset returns the configured quality preset
func (s *Settings) GetQualityPreset() QualityPreset {
	preset := s.app.Preferences().String(KeyQualityPreset)
	if preset == "" {
		s.SetQualityPreset(DefaultQualityPreset)
		return DefaultQualityPreset
	}
	return QualityPreset(preset)
}

// SetQualityPreset sets the quality preset
func (s *Settings) SetQualityPreset(preset QualityPreset) {
	s.app.Preferences().SetString(KeyQualityPreset, string(preset))
}

// GetFilenameTemplate returns the filename template
func (s *Settings) GetFilenameTemplate() string {
	template := s.app.Preferences().String(KeyFilenameTemplate)
	if template == "" {
		s.SetFilenameTemplate(DefaultFilenameTemplate)
		return DefaultFilenameTemplate
	}
	return template
}

// SetFilenameTemplate sets the filename template
func (s *Settings) SetFilenameTemplate(template string) {
	if template == "" {
		template = DefaultFilenameTemplate
	}
	s.app.Preferences().SetString(KeyFilenameTemplate, template)
}

// GetLanguage returns the configured language
func (s *Settings) GetLanguage() string {
	lang := s.app.Preferences().String(KeyLanguage)
	if lang == "" {
		s.SetLanguage(DefaultLanguage)
		return DefaultLanguage
	}
	return lang
}

// SetLanguage sets the application language
func (s *Settings) SetLanguage(lang string) {
	s.app.Preferences().SetString(KeyLanguage, lang)
}

// GetQualityPresetOptions returns available quality preset options
func (s *Settings) GetQualityPresetOptions() []QualityPreset {
	return []QualityPreset{QualityBest, QualityMedium, QualityAudio}
}

// GetAutoRevealOnComplete returns whether to auto-reveal completed downloads
func (s *Settings) GetAutoRevealOnComplete() bool {
	return s.app.Preferences().BoolWithFallback(KeyAutoRevealComplete, DefaultAutoRevealComplete)
}

// SetAutoRevealOnComplete sets whether to auto-reveal completed downloads
func (s *Settings) SetAutoRevealOnComplete(autoReveal bool) {
	s.app.Preferences().SetBool(KeyAutoRevealComplete, autoReveal)
}

// GetLanguageOptions returns available language options
func (s *Settings) GetLanguageOptions() map[string]string {
	return map[string]string{
		"system": "System Default",
		"en":     "English",
		"ru":     "Русский",
		"pt":     "Português",
	}
}
