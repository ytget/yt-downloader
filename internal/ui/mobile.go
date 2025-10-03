package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MobileUI provides mobile-specific UI enhancements
type MobileUI struct {
	app fyne.App
}

// NewMobileUI creates a new mobile UI helper
func NewMobileUI(app fyne.App) *MobileUI {
	return &MobileUI{app: app}
}

// IsMobileDevice checks if the app is running on a mobile device
func (m *MobileUI) IsMobileDevice() bool {
	return fyne.CurrentDevice().IsMobile()
}

// CreateAdaptiveContainer creates a container that adapts to mobile orientation
func (m *MobileUI) CreateAdaptiveContainer(columns int, objects ...fyne.CanvasObject) *fyne.Container {
	return container.NewAdaptiveGrid(columns, objects...)
}

// CreateMobileOptimizedLayout creates a layout optimized for mobile devices
func (m *MobileUI) CreateMobileOptimizedLayout() *fyne.Container {
	if !m.IsMobileDevice() {
		// For desktop, use regular layout
		return container.NewVBox()
	}

	// For mobile, use adaptive grid that changes based on orientation
	return container.NewAdaptiveGrid(1) // Single column for mobile
}

// SetupMobileKeyboardHandling configures virtual keyboard behavior
func (m *MobileUI) SetupMobileKeyboardHandling(entry *widget.Entry) {
	if !m.IsMobileDevice() {
		return
	}

	// Note: Fyne doesn't have OnFocusGained/OnFocusLost callbacks
	// Keyboard handling is managed automatically by the platform
	// This method is kept for future enhancements when such callbacks become available
}

// CreateMobileButton creates a button optimized for mobile touch
func (m *MobileUI) CreateMobileButton(text string, onTapped func()) *widget.Button {
	btn := widget.NewButton(text, onTapped)

	// For mobile devices, set minimum size for touch targets
	if m.IsMobileDevice() {
		btn.Resize(fyne.NewSize(MobileButtonWidth, MobileRowButtonHeight))
	}

	return btn
}

// CreateMobileEntry creates an entry field optimized for mobile
func (m *MobileUI) CreateMobileEntry(placeholder string) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(placeholder)

	// For mobile devices, we'll rely on layout to provide proper sizing
	// The actual sizing will be handled by the container layout

	// Setup mobile keyboard handling (temporarily disabled for debugging)
	// m.SetupMobileKeyboardHandling(entry)

	return entry
}

// GetMobileSpacing returns appropriate spacing for mobile devices
func (m *MobileUI) GetMobileSpacing() float32 {
	if m.IsMobileDevice() {
		return 16 // Larger spacing for mobile
	}
	return 8 // Standard spacing for desktop
}

// GetMobilePadding returns appropriate padding for mobile devices
func (m *MobileUI) GetMobilePadding() float32 {
	if m.IsMobileDevice() {
		return 20 // Larger padding for mobile
	}
	return 10 // Standard padding for desktop
}

// GetDeviceOrientation returns the current device orientation
func (m *MobileUI) GetDeviceOrientation() fyne.DeviceOrientation {
	return fyne.CurrentDevice().Orientation()
}

// IsLandscape returns true if device is in landscape orientation
func (m *MobileUI) IsLandscape() bool {
	orientation := m.GetDeviceOrientation()
	return orientation == fyne.OrientationHorizontalLeft || orientation == fyne.OrientationHorizontalRight
}

// IsPortrait returns true if device is in portrait orientation
func (m *MobileUI) IsPortrait() bool {
	orientation := m.GetDeviceOrientation()
	return orientation == fyne.OrientationVertical || orientation == fyne.OrientationVerticalUpsideDown
}

// CreateOrientationAwareContainer creates a container that adapts to orientation changes
func (m *MobileUI) CreateOrientationAwareContainer(portraitLayout, landscapeLayout fyne.CanvasObject) *fyne.Container {
	if !m.IsMobileDevice() {
		// For desktop, always use portrait layout
		return container.NewVBox(portraitLayout)
	}

	// For mobile, choose layout based on current orientation
	if m.IsLandscape() {
		return container.NewVBox(landscapeLayout)
	}
	return container.NewVBox(portraitLayout)
}
