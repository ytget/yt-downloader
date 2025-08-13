package ui

import "time"

// UI-wide constants to avoid magic numbers/strings scattered across the codebase.

// Icons (emojis/symbols)
const (
	IconSettings = "⚙"
	IconPlay     = "▶"
	IconPause    = "⏸"
	IconFolder   = "📁"
	IconFile     = "📄"
	IconCopy     = "📋"
	IconClose    = "×"
	IconError    = "❌"
)

// Text fragments
const (
	MiddleDotSeparator  = " · "
	DashPlaceholder     = "—"
	ProgressLabelFormat = "%d%%"
)

// Layout sizing (TaskRow / lists)
const (
	StatusLabelWidth  float32 = 84
	SpeedLabelWidth   float32 = 100
	PercentLabelWidth float32 = 48

	RowMinWidth  float32 = 400
	RowMinHeight float32 = 80
	RowDefaultH  float32 = 72
)

// Toast notification sizing and behavior
const (
	ToastWidth    float32 = 300
	ToastHeight   float32 = 120
	ToastMargin   float32 = 20
	ToastAutoHide         = 5 * time.Second
)

// Tooltip behavior
const (
	TooltipAutoHide = 1500 * time.Millisecond
)

// Debounce durations
const (
	UIUpdateDebounce = 100 * time.Millisecond
)

// URLs / parsing
const (
	PlaylistQueryParam = "list="
)

// Delays
const (
	AutoStartDelay = 500 * time.Millisecond
)
