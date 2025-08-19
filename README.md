# yt-downloader

Lightweight cross‑platform desktop app to download YouTube videos and playlists with a clean Fyne UI and robust yt-dlp integration.

<div align="center">
    <img src="./yt-downloader.png" alt="yt-downloader" width="250" height="250" />
</div>

### Quick links
- [Quick start](#quick-start)
- [Usage](#usage)
- [Screenshots](#screenshots)
- [Configuration](#configuration-in-app-settings)
- [Architecture overview](#architecture-overview)
- [yt-dlp flags](#playlist-parsing-and-yt-dlp-flags)
- [Development](#development-makefile-driven)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

### Table of contents
- [What is this?](#what-is-this)
- [Purpose](#purpose)
- [Features](#features)
- [Screenshots](#screenshots)
- [Supported platforms](#supported-platforms)
- [Architecture overview](#architecture-overview)
- [Requirements](#requirements)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Usage](#usage)
- [Configuration (in-app Settings)](#configuration-in-app-settings)
- [Playlist parsing and yt-dlp flags](#playlist-parsing-and-yt-dlp-flags)
- [Development (Makefile driven)](#development-makefile-driven)
- [Troubleshooting](#troubleshooting)
- [Diagnostics and issue reporting](#diagnostics-and-issue-reporting)
- [FAQ](#faq)
- [Legal note](#legal-note)
- [Roadmap (high level)](#roadmap-high-level)
- [Contributing](#contributing)
- [Acknowledgements](#acknowledgements)
- [License](#license)

### What is this?
yt-downloader is a GUI application written in Go using the Fyne toolkit. It downloads single videos and entire playlists from YouTube, shows live progress, and stores files in your Downloads folder by default.

### Purpose
- Provide a simple, reliable, cross‑platform YouTube downloader with a friendly UI.
- Handle large playlists quickly using yt-dlp's flat JSON mode.
- Offer sensible defaults with options for power users (quality presets, filename template, parallelism, language, auto-reveal on complete).

### Features
- Best‑effort resilient downloads (continue, no-overwrite behavior).
- Playlist parsing via external `yt-dlp` CLI with `--flat-playlist` + `--dump-json` for speed and stability.
- Live progress, speed, ETA; per-item Start/Pause/Stop, Open/Reveal, Copy path, Remove.
- Parallel downloads with configurable limit.
- File naming template and quality presets (best/medium/audio).
- Notifications on completion with quick actions.
- Localization: System, English, Русский, Português.
- Cross‑platform (macOS, Linux, Windows) using Fyne.

### Screenshots

<div align="center">
  <img src="./screenshots/screen-01-playlist-downloading.jpeg" alt="Playlist downloading UI" width="720" />
  <br/>
  <img src="./screenshots/screen-02-settings.jpeg" alt="Settings dialog" width="720" />
  <br/>
  <sub>Screens may vary slightly depending on OS and theme.</sub>
</div>

### Supported platforms
- macOS 12+
- Linux (X11/Wayland)
- Windows 10/11

### Architecture overview
- UI: Fyne (`internal/ui`) with a unified playlist view and task rows.
- Downloads: `github.com/lrstanley/go-ytdlp` wrapper (uses yt-dlp under the hood) for single videos.
- Playlists: external `yt-dlp` process (CLI) for fast list extraction; JSON lines are parsed to `Playlist`/`PlaylistVideo`.
- Settings: stored via Fyne preferences; sane defaults with runtime changes.

### Requirements
- Go (matching `go.mod`, currently 1.24.x).
- yt-dlp installed and available in PATH (required for playlist parsing; also used by the downloader library under the hood).
  - macOS: `brew install yt-dlp`
  - Linux (Debian/Ubuntu): `sudo apt install yt-dlp` (or install latest from the project)
  - Windows: install from the official project and add to PATH
  - Optional but recommended: `ffmpeg` installed in PATH for muxing/format conversions.

### Installation
- Clone the repo and ensure `yt-dlp` is installed.
- The app runs without additional system services. All dependencies are Go modules; UI uses Fyne.
- Install binary into your Go bin:

```
make install
```

- Alternatively, download `yt-dlp` locally into `./bin` via:

```
make deps
```

### Quick start
1) Install `yt-dlp`.
2) Run the app:

```
make run
```

3) Paste a YouTube URL (single video or a playlist with `list=`) and press Download.

### Usage
- Single video: paste the video URL and click Download.
- Playlist: paste a URL containing `list=`; the app parses the list in background and then starts downloads (auto-start can apply).
- Each item exposes actions: Start/Pause, Stop, Reveal in Finder/Explorer, Open, Copy path, Remove.

### Configuration (in-app Settings)
- Download directory: defaults to the system Downloads folder.
- Max parallel downloads: bounded to a safe range.
- Quality preset: best, medium, audio.
- Filename template: defaults to `%(title)s.%(ext)s`.
- Language: System/English/Русский/Português.
- Auto reveal on complete: open file location automatically after download.

### Playlist parsing and yt-dlp flags
For playlists, the app runs the external `yt-dlp` process with:
- `--flat-playlist` — list items only (fast for big playlists)
- `--dump-json` — each item is printed as a JSON line and parsed

For downloads (via go-ytdlp -> yt-dlp):
- Continue partial downloads, avoid overwriting final files, prefer best MP4/WebM, and stream frequent progress updates for the UI.

### Development (Makefile driven)
Common tasks are exposed via the Makefile. See all commands:

```
make help
```

Key targets:
- `run`: Run application (entrypoint `cmd/yt-downloader/main.go`).
- `build`: Build binary to `bin/yt-downloader`.
- `test`: Run tests.
- `lint`: Run golangci-lint.
- `format`: Apply goimports formatting.
- `deps`: Download/tidy modules.
- `deps-update`: Update modules.
- `clean`: Remove build artifacts.
- `docker-run` / `docker-stop`: Run/stop via docker-compose.
 - `debug`: Run app and tee logs to `debug.log` (useful for bug reports).

Aliases:
- `r` -> run, `t` -> test, `l` -> lint, `f` -> format, `dr` -> docker-run, `ds` -> docker-stop.

### Troubleshooting
- "yt-dlp not found": ensure `yt-dlp` is installed and in PATH.
- Playlist returns 0 items: verify the URL contains `list=`; some Mix/autoplay lists are special but still supported when `list=` is present.
- Progress not showing 100%: for rare cases with unknown total size, completion will still flip the status to Completed.
- macOS Gatekeeper: you may need to allow the app/network access depending on your environment.

### Diagnostics and issue reporting
When reporting a bug, please include:
- Your OS and version, Go version, and `yt-dlp --version`.
- Example URL(s) that reproduce the issue (video or playlist with `list=`).
- Full app logs captured from a terminal session.

How to capture logs:

```
make debug
# or manually
make run 2>&1 | tee debug.log
```

Before filing an issue, please:
- Ensure you are on the latest `yt-dlp` (update it if needed).
- Search existing issues to avoid duplicates.
- Provide clear steps to reproduce and expected vs actual behavior.

### Roadmap (high level)
- More granular quality/format selection.
- Persistent task history and resume between sessions.
- Export/import playlist queues.
- Theming and additional locales.

### FAQ
- Why do I need `yt-dlp` installed? — The app delegates extraction/downloading to `yt-dlp` for maximum site compatibility and performance.
- How do I update `yt-dlp`? — Use your package manager (e.g. `brew upgrade yt-dlp`) or run `make deps` to refresh the local copy in `./bin`.
- Where are files saved? — By default to the system Downloads folder; you can change it in Settings.

### Legal note
This tool is intended for downloading content you have the rights to access. Respect platform Terms of Service and local laws.

### Contributing
Contributions are welcome! Please:
- Open issues for bugs and feature requests.
- Submit pull requests with focused changes and clear descriptions.
- Keep code readable and follow Go best practices.
- Prefer explicit error handling; avoid unnecessary reflection/generics; do not use `panic`.

### Acknowledgements
- `yt-dlp` — the heart of extraction.
- `Fyne` — cross‑platform UI.
- `go-ytdlp` — Go wrapper around yt-dlp used for downloads.
- Inspiration from [`youtube-dl`](https://github.com/ytdl-org/youtube-dl) project and its excellent documentation structure.

### License
MIT License

Copyright (c) 2025 yt-downloader contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Build artifacts and release structure

This project supports two build paths:
- Local builds (native toolchain)
- Cross-platform builds via Docker using fyne-cross

### Output directories
- `bin/`: native binaries built by `make build` (current host only).
- `fyne-cross/dist/<target>/`: release-ready packages produced by fyne-cross (e.g. `linux-amd64/dist.tar.xz`, `windows-amd64/dist.zip`, `android-*/dist.apk`).
- `fyne-cross/bin/<target>/`: raw binaries produced by fyne-cross (if available).
- `dist/`: aggregated artifacts for publishing; populated by `make collect-artifacts`.
- `dist/darwin-local/`: zipped local macOS `.app` bundles.

### Common tasks
- Cross-build (Docker required):
  - Linux amd64: `make build-linux-amd64`
  - Windows amd64: `make build-windows-amd64`
  - Android (all ABIs): `make build-android`
- Local macOS packaging (on macOS):
  - macOS `.app`: `make package-darwin`
- Aggregate all outputs into `dist/`:
  - `make collect-artifacts`

Version embedding: all builds inject version with `-ldflags -X main.version=<value>` (auto-populated from `git describe` in Makefile).

### CI (GitHub Actions) recommendations
- Build using the same targets as above (Linux/Windows/Android on `ubuntu-latest`; macOS/iOS on `macos-latest` if needed).
- Upload artifacts from `fyne-cross/dist/**` (and optionally `dist/**` if you aggregate locally in workflow).
- Attach the same artifacts to GitHub Releases triggered by tags `v*`.

Example artifact globs:
- `fyne-cross/dist/**`
- `dist/**` (if `make collect-artifacts` is used in CI)
