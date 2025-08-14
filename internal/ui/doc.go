package ui

// Package ui contains the Fyne-based desktop user interface for the application.
// It wires user interactions to the download service and renders tasks, playlists,
// notifications, and settings. All UI strings are localized via Localization.

// UI Design Principles for Stability and Readability:

// Current Implementation (Stable):
// - Fixed-width right cluster: Status/speed/percentage/buttons are anchored to prevent layout shifts
// - Elastic center only: Left header expands, right side remains fixed
// - Header constraints: 1-2 lines max with ellipsis, no URL/ID noise
// - Compact progress: Text-based percentages instead of progress bars for consistent height
// - Single status source: Status shown once (top-right), no duplicates
// - Compact metrics: "speed left · ETA if available" and percentages right for stable alignment
// - No long emojis/inserts in title and right fields to prevent width jumps
// - Stable row height with 2-line header buffer

// Best Practices for Fyne UI Layout:

// - Anchor edges, make center elastic: Right "utility cluster" always fixed and width-limited
// - Limit growing text by lines/width, use ellipsis/tooltip for long values
// - Prefer text progress/icons over heavy progress bars in dense lists
// - Separate information by semantic zones: "status in one place", "numbers aligned", "header - name only"
// - Use GridWrap/fixed containers for stable column geometry
// - Implement proper error boundaries and loading states
// - Use consistent spacing and padding throughout the interface
// - Leverage Fyne's built-in responsive design features
