package config

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestSettings(t *testing.T) *Settings {
	t.Helper()
	dir := t.TempDir()
	s := &Settings{
		filePath: filepath.Join(dir, SettingsFileName),
		data: SettingsData{
			MaxParallel:        DefaultMaxParallel,
			QualityPreset:      DefaultQualityPreset,
			FilenameTemplate:   DefaultFilenameTemplate,
			Language:           DefaultLanguage,
			AutoRevealComplete: DefaultAutoRevealComplete,
			DownloadDirectory:  filepath.Join(dir, "downloads"),
		},
	}
	return s
}

func TestNewSettings(t *testing.T) {
	// Isolate config dir
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	settings, err := NewSettings()
	if err != nil {
		t.Fatalf("NewSettings failed: %v", err)
	}
	if settings == nil {
		t.Fatal("settings should not be nil")
	}
}

func TestDownloadDirectory(t *testing.T) {
	settings := newTestSettings(t)

	dir := settings.GetDownloadDirectory()
	if dir == "" {
		t.Error("Download directory should not be empty")
	}

	customDir := filepath.Join(t.TempDir(), "custom")
	if err := settings.SetDownloadDirectory(customDir); err != nil {
		t.Fatalf("SetDownloadDirectory failed: %v", err)
	}

	if got := settings.GetDownloadDirectory(); got != customDir {
		t.Errorf("Expected download directory %s, got %s", customDir, got)
	}
}

func TestApplyRejectsUnwritableDir(t *testing.T) {
	settings := newTestSettings(t)
	// Point at a path that cannot be created as a directory: existing file.
	filePath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	data := settings.Snapshot()
	data.DownloadDirectory = filePath
	if err := settings.Apply(data); err == nil {
		t.Fatal("expected Apply to reject file path as download directory")
	}
}

func TestCollisionPolicyDefaultAndSet(t *testing.T) {
	settings := newTestSettings(t)
	if got := settings.GetCollisionPolicy(); got != DefaultCollisionPolicy {
		t.Fatalf("default policy %q want %q", got, DefaultCollisionPolicy)
	}
	if err := settings.SetCollisionPolicy("overwrite"); err != nil {
		t.Fatal(err)
	}
	if settings.GetCollisionPolicy() != "overwrite" {
		t.Fatal("expected overwrite")
	}
	// invalid normalizes
	_ = settings.SetCollisionPolicy("nope")
	if settings.GetCollisionPolicy() != DefaultCollisionPolicy {
		t.Fatalf("invalid should normalize to default, got %s", settings.GetCollisionPolicy())
	}
}

func TestMaxParallelDownloads(t *testing.T) {
	settings := newTestSettings(t)

	if maxParallel := settings.GetMaxParallelDownloads(); maxParallel != DefaultMaxParallel {
		t.Errorf("Expected default max parallel %d, got %d", DefaultMaxParallel, maxParallel)
	}

	if err := settings.SetMaxParallelDownloads(5); err != nil {
		t.Fatalf("SetMaxParallelDownloads failed: %v", err)
	}
	if got := settings.GetMaxParallelDownloads(); got != 5 {
		t.Errorf("Expected max parallel 5, got %d", got)
	}

	if err := settings.SetMaxParallelDownloads(0); err != nil {
		t.Fatalf("SetMaxParallelDownloads failed: %v", err)
	}
	if settings.GetMaxParallelDownloads() != 1 {
		t.Error("Max parallel should be clamped to minimum 1")
	}

	if err := settings.SetMaxParallelDownloads(15); err != nil {
		t.Fatalf("SetMaxParallelDownloads failed: %v", err)
	}
	if settings.GetMaxParallelDownloads() != 10 {
		t.Error("Max parallel should be clamped to maximum 10")
	}
}

func TestQualityPreset(t *testing.T) {
	settings := newTestSettings(t)

	if preset := settings.GetQualityPreset(); preset != DefaultQualityPreset {
		t.Errorf("Expected default quality preset %s, got %s", DefaultQualityPreset, preset)
	}

	if err := settings.SetQualityPreset(QualityBest); err != nil {
		t.Fatalf("SetQualityPreset failed: %v", err)
	}
	if got := settings.GetQualityPreset(); got != QualityBest {
		t.Errorf("Expected quality preset %s, got %s", QualityBest, got)
	}
}

func TestFilenameTemplate(t *testing.T) {
	settings := newTestSettings(t)

	if template := settings.GetFilenameTemplate(); template != DefaultFilenameTemplate {
		t.Errorf("Expected default template %s, got %s", DefaultFilenameTemplate, template)
	}

	customTemplate := "%(uploader)s - %(title)s.%(ext)s"
	if err := settings.SetFilenameTemplate(customTemplate); err != nil {
		t.Fatalf("SetFilenameTemplate failed: %v", err)
	}
	if got := settings.GetFilenameTemplate(); got != customTemplate {
		t.Errorf("Expected template %s, got %s", customTemplate, got)
	}

	if err := settings.SetFilenameTemplate(""); err != nil {
		t.Fatalf("SetFilenameTemplate failed: %v", err)
	}
	if got := settings.GetFilenameTemplate(); got != DefaultFilenameTemplate {
		t.Errorf("Empty template should default to %s, got %s", DefaultFilenameTemplate, got)
	}
}

func TestLanguage(t *testing.T) {
	settings := newTestSettings(t)

	if lang := settings.GetLanguage(); lang != DefaultLanguage {
		t.Errorf("Expected default language %s, got %s", DefaultLanguage, lang)
	}

	if err := settings.SetLanguage("en"); err != nil {
		t.Fatalf("SetLanguage failed: %v", err)
	}
	if got := settings.GetLanguage(); got != "en" {
		t.Errorf("Expected language 'en', got %s", got)
	}
}

func TestGetQualityPresetOptions(t *testing.T) {
	settings := newTestSettings(t)
	options := settings.GetQualityPresetOptions()
	expectedOptions := []QualityPreset{QualityBest, QualityMedium, QualityAudio}

	if len(options) != len(expectedOptions) {
		t.Fatalf("Expected %d quality options, got %d", len(expectedOptions), len(options))
	}
	for i, expected := range expectedOptions {
		if options[i] != expected {
			t.Errorf("Quality option %d: expected %s, got %s", i, expected, options[i])
		}
	}
}

func TestGetLanguageOptions(t *testing.T) {
	settings := newTestSettings(t)
	options := settings.GetLanguageOptions()

	expectedLangs := []string{"system", "lt", "en", "ru", "pt"}
	for _, lang := range expectedLangs {
		if _, exists := options[lang]; !exists {
			t.Errorf("Expected language option '%s' to exist", lang)
		}
	}
	if len(options) != len(expectedLangs) {
		t.Errorf("Expected %d language options, got %d", len(expectedLangs), len(options))
	}
	if DefaultLanguage != LanguageLT {
		t.Fatalf("DefaultLanguage=%q want %q", DefaultLanguage, LanguageLT)
	}
}

func TestCorruptSettingsFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	// Point NewSettings at isolated config dir.
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "cfg"))
	path, err := defaultSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewSettings()
	if err != nil {
		t.Fatalf("expected defaults on corrupt file, got err %v", err)
	}
	if s.LoadWarning() == "" {
		t.Fatal("expected load warning")
	}
	if s.GetMaxParallelDownloads() != DefaultMaxParallel {
		t.Fatal(s.GetMaxParallelDownloads())
	}
	if _, err := os.Stat(path + ".corrupt"); err != nil {
		t.Fatalf("expected quarantine file: %v", err)
	}
}

func TestLogSettingsDefaultsAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SettingsFileName)
	// Legacy file without log fields should default log_to_file=true.
	legacy := `{"download_directory":` + `"` + filepath.Join(dir, "dl") + `","max_parallel_downloads":2}`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Settings{filePath: path}
	if err := s.load(); err != nil {
		t.Fatal(err)
	}
	if s.GetLogLevel() != DefaultLogLevel {
		t.Fatalf("log level %s", s.GetLogLevel())
	}
	if !s.GetLogToFile() {
		t.Fatal("expected default log_to_file true for legacy settings")
	}

	if err := s.SetLogLevel("debug"); err != nil {
		t.Fatal(err)
	}
	if s.GetLogLevel() != "debug" {
		t.Fatal(s.GetLogLevel())
	}
	if err := s.SetLogToFile(false); err != nil {
		t.Fatal(err)
	}
	if s.GetLogToFile() {
		t.Fatal("expected false")
	}
}

func TestNormalizeDefaultsAndEnums(t *testing.T) {
	settings := newTestSettings(t)
	data := SettingsData{DownloadDirectory: filepath.Join(t.TempDir(), "dl"), MaxParallel: 99, QualityPreset: "invalid", DownloadMode: "invalid", TranscriptLang: " EN "}
	if err := settings.Apply(data); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	snap := settings.Snapshot()
	if snap.MaxParallel != MaxParallelDownloads || snap.QualityPreset != DefaultQualityPreset || snap.DownloadMode != DefaultDownloadMode || snap.TranscriptLang != "en" {
		t.Fatalf("unexpected normalized settings: %+v", snap)
	}
}

func TestApplyAndPersist(t *testing.T) {
	settings := newTestSettings(t)
	data := SettingsData{
		DownloadDirectory:  filepath.Join(t.TempDir(), "dl"),
		MaxParallel:        3,
		QualityPreset:      QualityAudio,
		FilenameTemplate:   "x.%(ext)s",
		Language:           "ru",
		AutoRevealComplete: false,
	}
	if err := settings.Apply(data); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	raw, err := os.ReadFile(settings.filePath)
	if err != nil {
		t.Fatalf("settings file missing: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("settings file empty")
	}

	snap := settings.Snapshot()
	if snap.Language != "ru" || snap.MaxParallel != 3 || snap.QualityPreset != QualityAudio {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}
