package download

// Package download implements the core download pipeline built on top of
// github.com/ytget/ytdlp (pure Go engine). It manages tasks lifecycle,
// concurrency limits, progress propagation to UI, and playlist chunk
// processing.
