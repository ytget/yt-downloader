package ui

// Package ui provides user interface components

// Localization manages UI text translations
type Localization struct {
	currentLanguage string
	texts           map[string]map[string]string
}

// Text keys for localization
const (
	KeyAppTitle          = "app_title"
	KeyDownload          = "download"
	KeyStop              = "stop"
	KeyOpen              = "open"
	KeyCompress          = "compress"
	KeySettings          = "settings"
	KeyFile              = "file"
	KeyLanguage          = "language"
	KeyDownloadDirectory = "download_directory"
	KeyMaxParallel       = "max_parallel"
	KeyQualityPreset     = "quality_preset"
	KeyFilenameTemplate  = "filename_template"
	KeySave              = "save"
	KeyCancel            = "cancel"
	KeyBrowse            = "browse"
	KeyEnterURL          = "enter_url"
	KeySettingsSaved     = "settings_saved"
	KeyDownloadStarted   = "download_started"
	KeyDownloadCompleted = "download_completed"
	KeyErrorStoppingTask = "error_stopping_task"
	KeyErrorOpeningFile  = "error_opening_file"
	KeyStoppingDownload  = "stopping_download"
	KeyInvalidURL        = "invalid_url"
	KeyPleaseEnterURL    = "please_enter_url"
	KeyAlreadyInQueue    = "already_in_queue"
	KeyTaskAdded         = "task_added"
)

// NewLocalization creates a new localization manager
func NewLocalization() *Localization {
	l := &Localization{
		currentLanguage: "en",
		texts:           make(map[string]map[string]string),
	}

	l.initializeTexts()
	return l
}

// SetLanguage sets the current language
func (l *Localization) SetLanguage(lang string) {
	if lang == "system" {
		// Use system locale - simplified to English for now
		lang = "en"
	}

	if _, exists := l.texts[lang]; exists {
		l.currentLanguage = lang
	}
}

// GetText returns localized text for the given key
func (l *Localization) GetText(key string) string {
	if texts, exists := l.texts[l.currentLanguage]; exists {
		if text, found := texts[key]; found {
			return text
		}
	}

	// Fallback to English
	if texts, exists := l.texts["en"]; exists {
		if text, found := texts[key]; found {
			return text
		}
	}

	// Final fallback - return key itself
	return key
}

// GetCurrentLanguage returns the current language code
func (l *Localization) GetCurrentLanguage() string {
	return l.currentLanguage
}

// GetAvailableLanguages returns map of available languages with their display names
func (l *Localization) GetAvailableLanguages() map[string]string {
	return map[string]string{
		"en": "English",
		"ru": "Русский",
		"pt": "Português",
	}
}

// initializeTexts initializes all text translations
func (l *Localization) initializeTexts() {
	// English texts
	l.texts["en"] = map[string]string{
		KeyAppTitle:          "YT Downloader",
		KeyDownload:          "Download",
		KeyStop:              "Stop",
		KeyOpen:              "Open",
		KeyCompress:          "Compress",
		KeySettings:          "Settings",
		KeyFile:              "File",
		KeyLanguage:          "Language",
		KeyDownloadDirectory: "Download Directory",
		KeyMaxParallel:       "Max Parallel Downloads",
		KeyQualityPreset:     "Quality Preset",
		KeyFilenameTemplate:  "Filename Template",
		KeySave:              "Save",
		KeyCancel:            "Cancel",
		KeyBrowse:            "Browse",
		KeyEnterURL:          "Enter YouTube URL (https://youtube.com/watch?v=...)",
		KeySettingsSaved:     "Settings saved successfully!",
		KeyDownloadStarted:   "Download started",
		KeyDownloadCompleted: "Download completed",
		KeyErrorStoppingTask: "Error stopping task",
		KeyErrorOpeningFile:  "Error opening file",
		KeyStoppingDownload:  "Stopping download...",
		KeyInvalidURL:        "Invalid URL",
		KeyPleaseEnterURL:    "Please enter a URL",
		KeyAlreadyInQueue:    "Already in queue",
		KeyTaskAdded:         "Task added to queue",
	}

	// Russian texts
	l.texts["ru"] = map[string]string{
		KeyAppTitle:          "YT Загрузчик",
		KeyDownload:          "Скачать",
		KeyStop:              "Стоп",
		KeyOpen:              "Открыть",
		KeyCompress:          "Сжать",
		KeySettings:          "Настройки",
		KeyFile:              "Файл",
		KeyLanguage:          "Язык",
		KeyDownloadDirectory: "Папка загрузки",
		KeyMaxParallel:       "Макс. параллельных",
		KeyQualityPreset:     "Предустановка качества",
		KeyFilenameTemplate:  "Шаблон имени файла",
		KeySave:              "Сохранить",
		KeyCancel:            "Отмена",
		KeyBrowse:            "Обзор",
		KeyEnterURL:          "Введите URL YouTube (https://youtube.com/watch?v=...)",
		KeySettingsSaved:     "Настройки успешно сохранены!",
		KeyDownloadStarted:   "Загрузка начата",
		KeyDownloadCompleted: "Загрузка завершена",
		KeyErrorStoppingTask: "Ошибка остановки задачи",
		KeyErrorOpeningFile:  "Ошибка открытия файла",
		KeyStoppingDownload:  "Остановка загрузки...",
		KeyInvalidURL:        "Неверный URL",
		KeyPleaseEnterURL:    "Пожалуйста, введите URL",
		KeyAlreadyInQueue:    "Уже в очереди",
		KeyTaskAdded:         "Задача добавлена в очередь",
	}

	// Portuguese texts
	l.texts["pt"] = map[string]string{
		KeyAppTitle:          "YT Downloader",
		KeyDownload:          "Baixar",
		KeyStop:              "Parar",
		KeyOpen:              "Abrir",
		KeyCompress:          "Comprimir",
		KeySettings:          "Configurações",
		KeyFile:              "Arquivo",
		KeyLanguage:          "Idioma",
		KeyDownloadDirectory: "Diretório de Download",
		KeyMaxParallel:       "Max Downloads Paralelos",
		KeyQualityPreset:     "Predefinição de Qualidade",
		KeyFilenameTemplate:  "Modelo de Nome de Arquivo",
		KeySave:              "Salvar",
		KeyCancel:            "Cancelar",
		KeyBrowse:            "Navegar",
		KeyEnterURL:          "Digite URL do YouTube (https://youtube.com/watch?v=...)",
		KeySettingsSaved:     "Configurações salvas com sucesso!",
		KeyDownloadStarted:   "Download iniciado",
		KeyDownloadCompleted: "Download concluído",
		KeyErrorStoppingTask: "Erro ao parar tarefa",
		KeyErrorOpeningFile:  "Erro ao abrir arquivo",
		KeyStoppingDownload:  "Parando download...",
		KeyInvalidURL:        "URL inválida",
		KeyPleaseEnterURL:    "Por favor, digite uma URL",
		KeyAlreadyInQueue:    "Já na fila",
		KeyTaskAdded:         "Tarefa adicionada à fila",
	}
}
