package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ytget/yt-downloader/internal/fsutil"
	"github.com/ytget/yt-downloader/internal/platform"
)

// Quality presets for downloads
type QualityPreset string

const (
	QualityBest   QualityPreset = "best"
	QualityMedium QualityPreset = "medium"
	QualityAudio  QualityPreset = "audio"
)

// Download modes
type DownloadMode string

const (
	ModeVideo              DownloadMode = "video"
	ModeVideoAndTranscript DownloadMode = "video_and_transcript"
	ModeTranscriptOnly     DownloadMode = "transcript_only"
)

// Default values
const (
	DefaultMaxParallel        = 2
	DefaultQualityPreset      = QualityMedium
	DefaultFilenameTemplate   = "%(title)s.%(ext)s"
	DefaultLanguage           = "lt"
	DefaultAutoRevealComplete = true
	DefaultDownloadMode       = ModeVideo
	DefaultTranscriptLang     = "auto"
	DefaultCollisionPolicy    = fsutil.DefaultCollisionPolicy
	DefaultLogLevel           = "info"
	DefaultLogToFile          = true
	DefaultCheckUpdates       = false
	DefaultDisableCaptions    = false
	DefaultVideoOnlySafe      = false
)

// Validation limits
const (
	MinParallelDownloads = 1
	MaxParallelDownloads = 10
)

// Fallback values
const (
	FallbackDownloadDir = "/tmp/downloads"
	AppConfigDirName    = "yt-downloader"
	SettingsFileName    = "settings.json"
)

// Language codes
const (
	LanguageSystem = "system"
	LanguageLT     = "lt"
	LanguageEN     = "en"
	LanguageRU     = "ru"
	LanguagePT     = "pt"
)

// SettingsData is the persisted settings payload.
type SettingsData struct {
	DownloadDirectory  string                 `json:"download_directory"`
	MaxParallel        int                    `json:"max_parallel_downloads"`
	QualityPreset      QualityPreset          `json:"quality_preset"`
	FilenameTemplate   string                 `json:"filename_template"`
	Language           string                 `json:"app_language"`
	AutoRevealComplete bool                   `json:"auto_reveal_on_complete"`
	DownloadMode       DownloadMode           `json:"download_mode"`
	TranscriptLang     string                 `json:"transcript_language"`
	CollisionPolicy    fsutil.CollisionPolicy `json:"collision_policy"`
	LogLevel           string                 `json:"log_level"`
	LogToFile          bool                   `json:"log_to_file"`
	// Feature flags / kill switches (G4)
	DisableCaptions bool `json:"disable_captions"`
	VideoOnlySafe   bool `json:"video_only_safe_mode"`
	// Optional GitHub releases check (G3); off by default (no surprise network).
	CheckUpdates bool `json:"check_updates"`
}

// Settings manages application configuration with JSON file persistence.
type Settings struct {
	mu          sync.RWMutex
	data        SettingsData
	filePath    string
	loadWarning string
}

// NewSettings creates a settings manager and loads existing values if present.
func NewSettings() (*Settings, error) {
	path, err := defaultSettingsPath()
	if err != nil {
		return nil, err
	}

	s := &Settings{
		filePath: path,
		data: SettingsData{
			MaxParallel:        DefaultMaxParallel,
			QualityPreset:      DefaultQualityPreset,
			FilenameTemplate:   DefaultFilenameTemplate,
			Language:           DefaultLanguage,
			AutoRevealComplete: DefaultAutoRevealComplete,
			DownloadMode:       DefaultDownloadMode,
			TranscriptLang:     DefaultTranscriptLang,
			CollisionPolicy:    DefaultCollisionPolicy,
			LogLevel:           DefaultLogLevel,
			LogToFile:          DefaultLogToFile,
			DisableCaptions:    DefaultDisableCaptions,
			VideoOnlySafe:      DefaultVideoOnlySafe,
			CheckUpdates:       DefaultCheckUpdates,
		},
	}

	if err := s.load(); err != nil && !os.IsNotExist(err) {
		// Corrupt/unreadable settings: quarantine and continue with defaults (D2).
		backup := path + ".corrupt"
		_ = os.Rename(path, backup)
		s.loadWarning = fmt.Sprintf("settings reset to defaults (previous file moved to %s): %v", filepath.Base(backup), err)
	}

	// Ensure download directory has a usable default.
	if s.data.DownloadDirectory == "" {
		s.data.DownloadDirectory = s.systemDownloadDir()
	}

	return s, nil
}

// LoadWarning returns a non-empty message when settings were recovered from failure.
func (s *Settings) LoadWarning() string {
	if s == nil {
		return ""
	}
	return s.loadWarning
}

func defaultSettingsPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfgDir, AppConfigDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, SettingsFileName), nil
}

func (s *Settings) systemDownloadDir() string {
	dir, err := platform.GetHomeDownloadsDir()
	if err != nil || dir == "" {
		return FallbackDownloadDir
	}
	return dir
}

func (s *Settings) load() error {
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}
	// Use a map first so we can detect missing keys vs explicit false.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return err
	}
	var data SettingsData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if data.LogLevel == "" {
		data.LogLevel = DefaultLogLevel
	} else {
		data.LogLevel = normalizeLogLevel(data.LogLevel)
	}
	if _, ok := probe["log_to_file"]; !ok {
		data.LogToFile = DefaultLogToFile
	}
	data = s.normalize(data)
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()
	return nil
}

func (s *Settings) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, raw, 0o644)
}

// Snapshot returns a copy of current settings.
func (s *Settings) Snapshot() SettingsData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// normalize applies defaults and validates enum-like settings in one place.
// It deliberately does not touch the filesystem, so it is safe during load.
func (s *Settings) normalize(data SettingsData) SettingsData {
	if data.MaxParallel < MinParallelDownloads {
		data.MaxParallel = DefaultMaxParallel
	}
	if data.MaxParallel > MaxParallelDownloads {
		data.MaxParallel = MaxParallelDownloads
	}
	if data.FilenameTemplate == "" {
		data.FilenameTemplate = DefaultFilenameTemplate
	}
	if data.QualityPreset != QualityBest && data.QualityPreset != QualityMedium && data.QualityPreset != QualityAudio {
		data.QualityPreset = DefaultQualityPreset
	}
	if data.Language == "" {
		data.Language = DefaultLanguage
	}
	if data.DownloadDirectory == "" {
		data.DownloadDirectory = s.systemDownloadDir()
	}
	switch data.DownloadMode {
	case ModeVideo, ModeVideoAndTranscript, ModeTranscriptOnly:
	default:
		data.DownloadMode = DefaultDownloadMode
	}
	if strings.TrimSpace(data.TranscriptLang) == "" {
		data.TranscriptLang = DefaultTranscriptLang
	}
	data.TranscriptLang = strings.TrimSpace(strings.ToLower(data.TranscriptLang))
	data.CollisionPolicy = fsutil.NormalizeCollisionPolicy(data.CollisionPolicy)
	data.LogLevel = normalizeLogLevel(data.LogLevel)
	return data
}

// Apply replaces settings with the provided values and persists them.
func (s *Settings) Apply(data SettingsData) error {
	data = s.normalize(data)
	if err := fsutil.EnsureWritableDir(data.DownloadDirectory); err != nil {
		return fmt.Errorf("download directory: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = data
	return s.saveLocked()
}

// GetDownloadDirectory returns the configured download directory.
func (s *Settings) GetDownloadDirectory() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	defaultDir := s.systemDownloadDir()
	if s.data.DownloadDirectory != "" && s.data.DownloadDirectory != defaultDir {
		return s.data.DownloadDirectory
	}
	if s.data.DownloadDirectory == "" {
		return defaultDir
	}
	return s.data.DownloadDirectory
}

// SetDownloadDirectory sets the download directory.
func (s *Settings) SetDownloadDirectory(dir string) error {
	if err := fsutil.EnsureWritableDir(dir); err != nil {
		return fmt.Errorf("download directory: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DownloadDirectory = dir
	return s.saveLocked()
}

// ResetDownloadDirectory resets download directory to system default.
func (s *Settings) ResetDownloadDirectory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DownloadDirectory = s.systemDownloadDir()
	return s.saveLocked()
}

// GetMaxParallelDownloads returns the maximum number of parallel downloads.
func (s *Settings) GetMaxParallelDownloads() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.MaxParallel <= 0 {
		return DefaultMaxParallel
	}
	return s.data.MaxParallel
}

// SetMaxParallelDownloads sets the maximum number of parallel downloads.
func (s *Settings) SetMaxParallelDownloads(count int) error {
	if count < MinParallelDownloads {
		count = MinParallelDownloads
	}
	if count > MaxParallelDownloads {
		count = MaxParallelDownloads
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.MaxParallel = count
	return s.saveLocked()
}

// GetQualityPreset returns the configured quality preset.
func (s *Settings) GetQualityPreset() QualityPreset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.QualityPreset == "" {
		return DefaultQualityPreset
	}
	return s.data.QualityPreset
}

// SetQualityPreset sets the quality preset.
func (s *Settings) SetQualityPreset(preset QualityPreset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.QualityPreset = preset
	return s.saveLocked()
}

// GetFilenameTemplate returns the filename template.
func (s *Settings) GetFilenameTemplate() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.FilenameTemplate == "" {
		return DefaultFilenameTemplate
	}
	return s.data.FilenameTemplate
}

// SetFilenameTemplate sets the filename template.
func (s *Settings) SetFilenameTemplate(template string) error {
	if template == "" {
		template = DefaultFilenameTemplate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.FilenameTemplate = template
	return s.saveLocked()
}

// GetLanguage returns the configured language.
func (s *Settings) GetLanguage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.Language == "" {
		return DefaultLanguage
	}
	return s.data.Language
}

// SetLanguage sets the application language.
func (s *Settings) SetLanguage(lang string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Language = lang
	return s.saveLocked()
}

// GetQualityPresetOptions returns available quality preset options.
func (s *Settings) GetQualityPresetOptions() []QualityPreset {
	return []QualityPreset{QualityBest, QualityMedium, QualityAudio}
}

// GetAutoRevealOnComplete returns whether to auto-reveal completed downloads.
func (s *Settings) GetAutoRevealOnComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.AutoRevealComplete
}

// SetAutoRevealOnComplete sets whether to auto-reveal completed downloads.
func (s *Settings) SetAutoRevealOnComplete(autoReveal bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.AutoRevealComplete = autoReveal
	return s.saveLocked()
}

// GetLanguageOptions returns available language options.
func (s *Settings) GetLanguageOptions() map[string]string {
	return map[string]string{
		LanguageSystem: "System Default",
		LanguageLT:     "Lietuvių",
		LanguageEN:     "English",
		LanguageRU:     "Русский",
		LanguagePT:     "Português",
	}
}

// GetDownloadMode returns the default download mode.
func (s *Settings) GetDownloadMode() DownloadMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch s.data.DownloadMode {
	case ModeVideo, ModeVideoAndTranscript, ModeTranscriptOnly:
		return s.data.DownloadMode
	default:
		return DefaultDownloadMode
	}
}

// SetDownloadMode sets the default download mode.
func (s *Settings) SetDownloadMode(mode DownloadMode) error {
	switch mode {
	case ModeVideo, ModeVideoAndTranscript, ModeTranscriptOnly:
	default:
		mode = DefaultDownloadMode
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DownloadMode = mode
	return s.saveLocked()
}

// GetTranscriptLanguage returns preferred transcript language ("auto" default).
func (s *Settings) GetTranscriptLanguage() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data.TranscriptLang == "" {
		return DefaultTranscriptLang
	}
	return s.data.TranscriptLang
}

// SetTranscriptLanguage sets preferred transcript language.
func (s *Settings) SetTranscriptLanguage(lang string) error {
	lang = strings.TrimSpace(lang)
	if lang == "" {
		lang = DefaultTranscriptLang
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.TranscriptLang = lang
	return s.saveLocked()
}

// GetCollisionPolicy returns file collision policy for downloads.
func (s *Settings) GetCollisionPolicy() fsutil.CollisionPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fsutil.NormalizeCollisionPolicy(s.data.CollisionPolicy)
}

// SetCollisionPolicy sets file collision policy (skip|overwrite|rename).
func (s *Settings) SetCollisionPolicy(policy fsutil.CollisionPolicy) error {
	policy = fsutil.NormalizeCollisionPolicy(policy)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.CollisionPolicy = policy
	return s.saveLocked()
}

func normalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return "debug"
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return DefaultLogLevel
	}
}

// GetLogLevel returns the configured log level.
func (s *Settings) GetLogLevel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return normalizeLogLevel(s.data.LogLevel)
}

// SetLogLevel sets debug|info|warn|error.
func (s *Settings) SetLogLevel(level string) error {
	level = normalizeLogLevel(level)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.LogLevel = level
	return s.saveLocked()
}

// GetLogToFile returns whether file logging is enabled.
func (s *Settings) GetLogToFile() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.LogToFile
}

// SetLogToFile enables or disables writing logs to the app log file.
func (s *Settings) SetLogToFile(enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.LogToFile = enabled
	return s.saveLocked()
}

// ResetDefaults restores factory defaults (keeps settings file path).
func (s *Settings) ResetDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = SettingsData{
		DownloadDirectory:  s.systemDownloadDir(),
		MaxParallel:        DefaultMaxParallel,
		QualityPreset:      DefaultQualityPreset,
		FilenameTemplate:   DefaultFilenameTemplate,
		Language:           DefaultLanguage,
		AutoRevealComplete: DefaultAutoRevealComplete,
		DownloadMode:       DefaultDownloadMode,
		TranscriptLang:     DefaultTranscriptLang,
		CollisionPolicy:    DefaultCollisionPolicy,
		LogLevel:           DefaultLogLevel,
		LogToFile:          DefaultLogToFile,
		DisableCaptions:    DefaultDisableCaptions,
		VideoOnlySafe:      DefaultVideoOnlySafe,
		CheckUpdates:       DefaultCheckUpdates,
	}
	return s.saveLocked()
}

// FilePath returns the settings.json path.
func (s *Settings) FilePath() string {
	if s == nil {
		return ""
	}
	return s.filePath
}

// GetDisableCaptions returns the captions kill switch.
func (s *Settings) GetDisableCaptions() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DisableCaptions
}

// GetVideoOnlySafe returns video-only safe mode.
func (s *Settings) GetVideoOnlySafe() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.VideoOnlySafe
}

// GetCheckUpdates returns whether optional GitHub update checks are enabled.
func (s *Settings) GetCheckUpdates() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.CheckUpdates
}

// SetFeatureFlags updates kill switches and update check preference.
func (s *Settings) SetFeatureFlags(disableCaptions, videoOnlySafe, checkUpdates bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DisableCaptions = disableCaptions
	s.data.VideoOnlySafe = videoOnlySafe
	s.data.CheckUpdates = checkUpdates
	return s.saveLocked()
}

// QualityPresetHelp returns a short honesty note for UI/docs.
func QualityPresetHelp(preset QualityPreset) string {
	switch preset {
	case QualityBest:
		return "Highest progressive video+audio stream available"
	case QualityMedium:
		return "Progressive video up to 480p when available"
	case QualityAudio:
		return "Not supported yet (engine downloads progressive A/V only)"
	default:
		return ""
	}
}
