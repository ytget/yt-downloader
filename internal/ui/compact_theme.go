package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// CompactTheme defines a compact theme for the UI with reduced padding and font sizes
type CompactTheme struct{}

// NewCompactTheme creates a new compact theme
func NewCompactTheme() fyne.Theme {
	return &CompactTheme{}
}

// Color returns theme colors
func (t *CompactTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameSuccess:
		return color.RGBA{R: 46, G: 160, B: 67, A: 255} // Green for completed
	case theme.ColorNameError:
		return color.RGBA{R: 183, G: 28, B: 28, A: 255} // Red for errors
	case theme.ColorNameWarning:
		return color.RGBA{R: 255, G: 193, B: 7, A: 255} // Amber for warnings
	case theme.ColorNamePrimary:
		return color.RGBA{R: 25, G: 118, B: 210, A: 255} // Blue for primary actions
	case theme.ColorNameBackground:
		if variant == theme.VariantDark {
			return color.RGBA{R: 18, G: 18, B: 18, A: 255} // Dark gray
		}
		return color.RGBA{R: 250, G: 250, B: 250, A: 255} // Light gray
	case theme.ColorNameForeground:
		if variant == theme.VariantDark {
			return color.RGBA{R: 255, G: 255, B: 255, A: 255} // White text
		}
		return color.RGBA{R: 33, G: 33, B: 33, A: 255} // Dark text
	}

	// Use default colors for everything else
	return theme.DefaultTheme().Color(name, variant)
}

// Font returns theme fonts
func (t *CompactTheme) Font(style fyne.TextStyle) fyne.Resource {
	// Use default theme fonts
	return theme.DefaultTheme().Font(style)
}

// Icon returns theme icons
func (t *CompactTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	// Use default theme icons
	return theme.DefaultTheme().Icon(name)
}

// Size returns theme sizes with compact adjustments
func (t *CompactTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 3 // Reduced from default 4
	case theme.SizeNameInnerPadding:
		return 6 // Reduced from default 8
	case theme.SizeNameLineSpacing:
		return 2 // Reduced from default 4
	case theme.SizeNameScrollBar:
		return 12 // Reduced from default 16
	case theme.SizeNameScrollBarSmall:
		return 3 // Reduced from default 3
	case theme.SizeNameSeparatorThickness:
		return 1 // Keep default 1
	case theme.SizeNameText:
		return 13 // Reduced from default 14
	case theme.SizeNameHeadingText:
		return 16 // Reduced from default 18
	case theme.SizeNameSubHeadingText:
		return 13 // Reduced from default 16
	case theme.SizeNameCaptionText:
		return 10 // Reduced from default 11
	case theme.SizeNameInputBorder:
		return 1 // Keep default 1
	case theme.SizeNameInputRadius:
		return 3 // Reduced from default 5
	case theme.SizeNameSelectionRadius:
		return 2 // Reduced from default 3
	}

	// Use default theme for everything else
	return theme.DefaultTheme().Size(name)
}
