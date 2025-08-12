package config

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestNewSettings(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

	if settings.app != app {
		t.Error("Settings app reference should match provided app")
	}
}

func TestDownloadDirectory(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

	// Test default value
	dir := settings.GetDownloadDirectory()
	if dir == "" {
		t.Error("Download directory should not be empty")
	}

	// Test setting custom value
	customDir := "/custom/downloads"
	settings.SetDownloadDirectory(customDir)

	retrievedDir := settings.GetDownloadDirectory()
	if retrievedDir != customDir {
		t.Errorf("Expected download directory %s, got %s", customDir, retrievedDir)
	}
}

func TestMaxParallelDownloads(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

	// Test default value
	maxParallel := settings.GetMaxParallelDownloads()
	if maxParallel != DefaultMaxParallel {
		t.Errorf("Expected default max parallel %d, got %d", DefaultMaxParallel, maxParallel)
	}

	// Test setting custom value
	settings.SetMaxParallelDownloads(5)

	retrievedMax := settings.GetMaxParallelDownloads()
	if retrievedMax != 5 {
		t.Errorf("Expected max parallel 5, got %d", retrievedMax)
	}

	// Test boundary values
	settings.SetMaxParallelDownloads(0) // Should be clamped to 1
	if settings.GetMaxParallelDownloads() != 1 {
		t.Error("Max parallel should be clamped to minimum 1")
	}

	settings.SetMaxParallelDownloads(15) // Should be clamped to 10
	if settings.GetMaxParallelDownloads() != 10 {
		t.Error("Max parallel should be clamped to maximum 10")
	}
}

func TestQualityPreset(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

	// Test default value
	preset := settings.GetQualityPreset()
	if preset != DefaultQualityPreset {
		t.Errorf("Expected default quality preset %s, got %s", DefaultQualityPreset, preset)
	}

	// Test setting custom value
	settings.SetQualityPreset(QualityBest)

	retrievedPreset := settings.GetQualityPreset()
	if retrievedPreset != QualityBest {
		t.Errorf("Expected quality preset %s, got %s", QualityBest, retrievedPreset)
	}
}

func TestFilenameTemplate(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

	// Test default value
	template := settings.GetFilenameTemplate()
	if template != DefaultFilenameTemplate {
		t.Errorf("Expected default template %s, got %s", DefaultFilenameTemplate, template)
	}

	// Test setting custom value
	customTemplate := "%(uploader)s - %(title)s.%(ext)s"
	settings.SetFilenameTemplate(customTemplate)

	retrievedTemplate := settings.GetFilenameTemplate()
	if retrievedTemplate != customTemplate {
		t.Errorf("Expected template %s, got %s", customTemplate, retrievedTemplate)
	}

	// Test empty template defaults back
	settings.SetFilenameTemplate("")
	retrievedTemplate = settings.GetFilenameTemplate()
	if retrievedTemplate != DefaultFilenameTemplate {
		t.Errorf("Empty template should default to %s, got %s", DefaultFilenameTemplate, retrievedTemplate)
	}
}

func TestLanguage(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

	// Test default value
	lang := settings.GetLanguage()
	if lang != DefaultLanguage {
		t.Errorf("Expected default language %s, got %s", DefaultLanguage, lang)
	}

	// Test setting custom value
	settings.SetLanguage("en")

	retrievedLang := settings.GetLanguage()
	if retrievedLang != "en" {
		t.Errorf("Expected language 'en', got %s", retrievedLang)
	}
}

func TestGetQualityPresetOptions(t *testing.T) {
	app := test.NewApp()
	settings := NewSettings(app)

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
	app := test.NewApp()
	settings := NewSettings(app)

	options := settings.GetLanguageOptions()

	expectedLangs := []string{"system", "en", "ru", "pt"}
	for _, lang := range expectedLangs {
		if _, exists := options[lang]; !exists {
			t.Errorf("Expected language option '%s' to exist", lang)
		}
	}

	if len(options) != len(expectedLangs) {
		t.Errorf("Expected %d language options, got %d", len(expectedLangs), len(options))
	}
}
