package download

// Package download implements the core download pipeline built on top of yt-dlp
// (via github.com/lrstanley/go-ytdlp). It manages tasks lifecycle, concurrency
// limits, progress propagation to UI, and playlist chunk processing.
