package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/mobile"
)

// GestureType represents different types of gestures
type GestureType int

const (
	GestureTap GestureType = iota
	GestureSwipeLeft
	GestureSwipeRight
	GestureSwipeUp
	GestureSwipeDown
	GestureLongPress
	GesturePullToRefresh
)

// GestureHandler handles mobile gestures
type GestureHandler struct {
	onGesture func(GestureType)

	// Touch tracking
	touchStartTime time.Time
	touchStartPos  fyne.Position
	touchEndPos    fyne.Position

	// Gesture thresholds
	swipeThreshold    float32
	longPressDuration time.Duration
}

// Gesture thresholds constants
const (
	DefaultSwipeThreshold    float32 = 50.0
	DefaultLongPressDuration         = 500 * time.Millisecond
)

// NewGestureHandler creates a new gesture handler
func NewGestureHandler(onGesture func(GestureType)) *GestureHandler {
	return &GestureHandler{
		onGesture:         onGesture,
		swipeThreshold:    DefaultSwipeThreshold,
		longPressDuration: DefaultLongPressDuration,
	}
}

// TouchDown handles touch down events for gesture detection
func (gh *GestureHandler) TouchDown(event *mobile.TouchEvent) {
	gh.touchStartTime = time.Now()
	gh.touchStartPos = event.Position
}

// TouchUp handles touch up events for gesture detection
func (gh *GestureHandler) TouchUp(event *mobile.TouchEvent) {
	gh.touchEndPos = event.Position
	duration := time.Since(gh.touchStartTime)

	// Calculate movement distance
	dx := gh.touchEndPos.X - gh.touchStartPos.X
	dy := gh.touchEndPos.Y - gh.touchStartPos.Y
	distance := float32(dx*dx + dy*dy)

	// Detect gesture type
	if duration < gh.longPressDuration && distance < gh.swipeThreshold {
		// Quick tap
		gh.triggerGesture(GestureTap)
	} else if duration >= gh.longPressDuration {
		// Long press
		gh.triggerGesture(GestureLongPress)
	} else if distance >= gh.swipeThreshold {
		// Swipe gesture
		gh.detectSwipeDirection(dx, dy)
	}
}

// TouchCancel handles touch cancel events
func (gh *GestureHandler) TouchCancel(event *mobile.TouchEvent) {
	// Reset tracking
	gh.touchStartTime = time.Time{}
}

// detectSwipeDirection determines the direction of a swipe gesture
func (gh *GestureHandler) detectSwipeDirection(dx, dy float32) {
	absDx := dx
	if absDx < 0 {
		absDx = -absDx
	}
	absDy := dy
	if absDy < 0 {
		absDy = -absDy
	}

	// Determine primary direction
	if absDx > absDy {
		// Horizontal swipe
		if dx > 0 {
			gh.triggerGesture(GestureSwipeRight)
		} else {
			gh.triggerGesture(GestureSwipeLeft)
		}
	} else {
		// Vertical swipe
		if dy > 0 {
			gh.triggerGesture(GestureSwipeDown)
		} else {
			gh.triggerGesture(GestureSwipeUp)
		}
	}
}

// triggerGesture triggers a gesture callback
func (gh *GestureHandler) triggerGesture(gesture GestureType) {
	if gh.onGesture != nil {
		gh.onGesture(gesture)
	}
}

// SwipeableWidget represents a widget that can handle swipe gestures
type SwipeableWidget struct {
	fyne.CanvasObject
	gestureHandler *GestureHandler
}

// NewSwipeableWidget creates a new swipeable widget
func NewSwipeableWidget(widget fyne.CanvasObject, onGesture func(GestureType)) *SwipeableWidget {
	return &SwipeableWidget{
		CanvasObject:   widget,
		gestureHandler: NewGestureHandler(onGesture),
	}
}

// TouchDown handles touch down events
func (sw *SwipeableWidget) TouchDown(event *mobile.TouchEvent) {
	if sw.gestureHandler != nil {
		sw.gestureHandler.TouchDown(event)
	}
}

// TouchUp handles touch up events
func (sw *SwipeableWidget) TouchUp(event *mobile.TouchEvent) {
	if sw.gestureHandler != nil {
		sw.gestureHandler.TouchUp(event)
	}
}

// TouchCancel handles touch cancel events
func (sw *SwipeableWidget) TouchCancel(event *mobile.TouchEvent) {
	if sw.gestureHandler != nil {
		sw.gestureHandler.TouchCancel(event)
	}
}

// PullToRefreshWidget provides pull-to-refresh functionality
type PullToRefreshWidget struct {
	*fyne.Container
	gestureHandler *GestureHandler
	refreshFunc    func()
	isRefreshing   bool
}

// NewPullToRefreshWidget creates a new pull-to-refresh widget
func NewPullToRefreshWidget(content fyne.CanvasObject, refreshFunc func()) *PullToRefreshWidget {
	ptr := &PullToRefreshWidget{
		Container:   fyne.NewContainer(content),
		refreshFunc: refreshFunc,
	}

	ptr.gestureHandler = NewGestureHandler(ptr.handleGesture)

	return ptr
}

// handleGesture handles gestures for pull-to-refresh
func (ptr *PullToRefreshWidget) handleGesture(gesture GestureType) {
	if gesture == GestureSwipeDown && !ptr.isRefreshing {
		ptr.triggerRefresh()
	}
}

// triggerRefresh triggers the refresh action
func (ptr *PullToRefreshWidget) triggerRefresh() {
	if ptr.refreshFunc != nil && !ptr.isRefreshing {
		ptr.isRefreshing = true
		ptr.refreshFunc()
		// Reset refreshing state after a delay
		go func() {
			time.Sleep(2 * time.Second)
			ptr.isRefreshing = false
		}()
	}
}

// TouchDown handles touch down events
func (ptr *PullToRefreshWidget) TouchDown(event *mobile.TouchEvent) {
	if ptr.gestureHandler != nil {
		ptr.gestureHandler.TouchDown(event)
	}
}

// TouchUp handles touch up events
func (ptr *PullToRefreshWidget) TouchUp(event *mobile.TouchEvent) {
	if ptr.gestureHandler != nil {
		ptr.gestureHandler.TouchUp(event)
	}
}

// TouchCancel handles touch cancel events
func (ptr *PullToRefreshWidget) TouchCancel(event *mobile.TouchEvent) {
	if ptr.gestureHandler != nil {
		ptr.gestureHandler.TouchCancel(event)
	}
}
