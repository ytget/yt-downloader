package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

// TouchableWidget represents a widget that can handle touch events
type TouchableWidget interface {
	fyne.CanvasObject
	OnTouchDown(*mobile.TouchEvent)
	OnTouchUp(*mobile.TouchEvent)
	OnTouchCancel(*mobile.TouchEvent)
}

// TouchHandler provides mobile touch event handling
type TouchHandler struct {
	widget fyne.CanvasObject
	onTap  func()
}

// NewTouchHandler creates a new touch handler for a widget
func NewTouchHandler(widget fyne.CanvasObject, onTap func()) *TouchHandler {
	return &TouchHandler{
		widget: widget,
		onTap:  onTap,
	}
}

// TouchDown handles touch down events
func (th *TouchHandler) TouchDown(event *mobile.TouchEvent) {
	// Store touch start position for potential tap detection
	// This is a simple implementation - could be enhanced for gestures
}

// TouchUp handles touch up events
func (th *TouchHandler) TouchUp(event *mobile.TouchEvent) {
	// Simple tap detection - if touch up happens quickly after touch down, it's a tap
	if th.onTap != nil {
		th.onTap()
	}
}

// TouchCancel handles touch cancel events
func (th *TouchHandler) TouchCancel(event *mobile.TouchEvent) {
	// Touch was cancelled, do nothing
}

// MobileList provides a list widget optimized for mobile touch interaction
type MobileList struct {
	*widget.List
	touchHandler *TouchHandler
}

// NewMobileList creates a new mobile-optimized list
func NewMobileList(length func() int, createItem func() fyne.CanvasObject, updateItem func(widget.ListItemID, fyne.CanvasObject)) *MobileList {
	list := widget.NewList(length, createItem, updateItem)

	ml := &MobileList{
		List: list,
	}

	// Wrap the list to add touch handling
	ml.touchHandler = NewTouchHandler(list, func() {
		// Handle tap on list - could be used for pull-to-refresh or other gestures
	})

	return ml
}

// TouchDown handles touch down events on the mobile list
func (ml *MobileList) TouchDown(event *mobile.TouchEvent) {
	if ml.touchHandler != nil {
		ml.touchHandler.TouchDown(event)
	}
}

// TouchUp handles touch up events on the mobile list
func (ml *MobileList) TouchUp(event *mobile.TouchEvent) {
	if ml.touchHandler != nil {
		ml.touchHandler.TouchUp(event)
	}
}

// TouchCancel handles touch cancel events on the mobile list
func (ml *MobileList) TouchCancel(event *mobile.TouchEvent) {
	if ml.touchHandler != nil {
		ml.touchHandler.TouchCancel(event)
	}
}

// MobileButton provides a button widget optimized for mobile touch
type MobileButton struct {
	*widget.Button
	touchHandler *TouchHandler
}

// NewMobileButton creates a new mobile-optimized button
func NewMobileButton(text string, onTapped func()) *MobileButton {
	btn := widget.NewButton(text, onTapped)

	mb := &MobileButton{
		Button: btn,
	}

	// Add touch handling for better mobile UX
	mb.touchHandler = NewTouchHandler(btn, onTapped)

	return mb
}

// TouchDown handles touch down events on the mobile button
func (mb *MobileButton) TouchDown(event *mobile.TouchEvent) {
	if mb.touchHandler != nil {
		mb.touchHandler.TouchDown(event)
	}
}

// TouchUp handles touch up events on the mobile button
func (mb *MobileButton) TouchUp(event *mobile.TouchEvent) {
	if mb.touchHandler != nil {
		mb.touchHandler.TouchUp(event)
	}
}

// TouchCancel handles touch cancel events on the mobile button
func (mb *MobileButton) TouchCancel(event *mobile.TouchEvent) {
	if mb.touchHandler != nil {
		mb.touchHandler.TouchCancel(event)
	}
}

