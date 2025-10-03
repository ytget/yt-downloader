package ui

// Package ui provides user interface components

// Localization manages UI text translations
type Localization struct {
	currentLanguage string
	texts           map[string]map[string]string
}

// Text keys for localization
const (
	// Actions
	KeyAppTitle          = "app_title"
	KeyDownload          = "download"
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
	KeyErrorStartingTask = "error_starting_task"
	KeyErrorOpeningFile  = "error_opening_file"
	KeyErrorCopyingPath  = "error_copying_path"
	KeyErrorRemovingTask = "error_removing_task"
	KeyInvalidURL        = "invalid_url"
	KeyPleaseEnterURL    = "please_enter_url"
	KeyAlreadyInQueue    = "already_in_queue"
	KeyTaskAdded         = "task_added"
	KeyPause             = "pause"
	KeyContinue          = "continue"
	KeyPlay              = "play"

	// Notification panel
	KeyParsingStarted = "parsing_started"
	KeyParsingFailed  = "parsing_failed"
	KeyPlaylistParsed = "playlist_parsed"

	// Tooltips
	KeyTooltipStartPause = "tooltip_start_pause"
	KeyTooltipReveal     = "tooltip_reveal"
	KeyTooltipOpen       = "tooltip_open"
	KeyTooltipCopyPath   = "tooltip_copy_path"
	KeyTooltipRemove     = "tooltip_remove"
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
		KeyOpen:              "Open",
		KeyCompress:          "Compress",
		KeyFile:              "File",
		KeyLanguage:          "Language",
		KeyDownloadDirectory: "Download Directory",
		KeyMaxParallel:       "Max Parallel Downloads",
		KeyQualityPreset:     "Quality Preset",
		KeyFilenameTemplate:  "Filename Template",
		KeySave:              "Save",
		KeyCancel:            "Cancel",
		KeyEnterURL:          "Enter YouTube URL (https://youtube.com/watch?v=...)",
		KeySettingsSaved:     "Settings saved successfully!",
		KeyDownloadStarted:   "Download started",
		KeyDownloadCompleted: "Download completed",
		KeyErrorStartingTask: "Error starting task",
		KeyErrorOpeningFile:  "Error opening file",
		KeyErrorCopyingPath:  "Error copying path",
		KeyErrorRemovingTask: "Error removing task",
		KeyInvalidURL:        "Invalid URL",
		KeyPleaseEnterURL:    "Please enter a URL",
		KeyAlreadyInQueue:    "Already in queue",
		KeyTaskAdded:         "Task added to queue",
		KeyPause:             "Pause",
		KeyContinue:          "Continue",
		KeyPlay:              "Play",
		KeyParsingStarted:    "Starting playlist parsing in background...",
		KeyParsingFailed:     "Failed to parse playlist",
		KeyPlaylistParsed:    "Playlist parsed",

		// Tooltips
		KeyTooltipStartPause: "Start / Pause",
		KeyTooltipReveal:     "Reveal in Finder/Explorer",
		KeyTooltipOpen:       "Open with default app",
		KeyTooltipCopyPath:   "Copy file path",
		KeyTooltipRemove:     "Remove task",
	}

	// Russian texts
	l.texts["ru"] = map[string]string{
		KeyAppTitle:          "YT Загрузчик",
		KeyDownload:          "Скачать",
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
		KeyEnterURL:          "Введите URL YouTube (https://youtube.com/watch?v=...)",
		KeySettingsSaved:     "Настройки успешно сохранены!",
		KeyDownloadStarted:   "Загрузка начата",
		KeyDownloadCompleted: "Загрузка завершена",
		KeyErrorStartingTask: "Ошибка запуска задачи",
		KeyErrorOpeningFile:  "Ошибка открытия файла",
		KeyErrorCopyingPath:  "Ошибка копирования пути",
		KeyErrorRemovingTask: "Ошибка удаления задачи",
		KeyInvalidURL:        "Неверный URL",
		KeyPleaseEnterURL:    "Пожалуйста, введите URL",
		KeyAlreadyInQueue:    "Уже в очереди",
		KeyTaskAdded:         "Задача добавлена в очередь",
		KeyPause:             "Пауза",
		KeyContinue:          "Продолжить",
		KeyPlay:              "Воспроизвести",
		KeyParsingStarted:    "Запуск парсинга плейлиста в фоне...",
		KeyParsingFailed:     "Не удалось распарсить плейлист",
		KeyPlaylistParsed:    "Плейлист распарсен",

		// Tooltips
		KeyTooltipStartPause: "Старт / Пауза",
		KeyTooltipReveal:     "Показать в проводнике",
		KeyTooltipOpen:       "Открыть файл",
		KeyTooltipCopyPath:   "Копировать путь к файлу",
		KeyTooltipRemove:     "Удалить задачу",
	}

	// Portuguese texts
	l.texts["pt"] = map[string]string{
		KeyAppTitle:          "YT Downloader",
		KeyDownload:          "Baixar",
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
		KeyEnterURL:          "Digite URL do YouTube (https://youtube.com/watch?v=...)",
		KeySettingsSaved:     "Configurações salvas com sucesso!",
		KeyDownloadStarted:   "Download iniciado",
		KeyDownloadCompleted: "Download concluído",
		KeyErrorStartingTask: "Erro ao iniciar tarefa",
		KeyErrorOpeningFile:  "Erro ao abrir arquivo",
		KeyErrorCopyingPath:  "Erro ao copiar caminho",
		KeyErrorRemovingTask: "Erro ao remover tarefa",
		KeyInvalidURL:        "URL inválida",
		KeyPleaseEnterURL:    "Por favor, digite uma URL",
		KeyAlreadyInQueue:    "Já na fila",
		KeyTaskAdded:         "Tarefa adicionada à fila",
		KeyPause:             "Pausar",
		KeyContinue:          "Continuar",
		KeyPlay:              "Reproduzir",
		KeyParsingStarted:    "Iniciando análise da playlist em segundo plano...",
		KeyParsingFailed:     "Falha ao analisar a playlist",
		KeyPlaylistParsed:    "Playlist analisada",

		// Tooltips
		KeyTooltipStartPause: "Iniciar / Pausar",
		KeyTooltipReveal:     "Mostrar no Finder/Explorer",
		KeyTooltipOpen:       "Abrir arquivo",
		KeyTooltipCopyPath:   "Copiar caminho do arquivo",
		KeyTooltipRemove:     "Remover tarefa",
	}
}
