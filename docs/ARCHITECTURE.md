# Architecture

## Stack

- **Shell:** [Wails v2](https://wails.io) desktop host
- **Backend:** Go (`main` package + `internal/*`)
- **Frontend:** Vite + TypeScript (`frontend/`)
- **Download engine:** `github.com/ytget/ytdlp/v2` (pure Go; may be temporarily `replace`d — see `go.mod`)

## Package map

| Package | Role |
|---------|------|
| `main` (`app.go`, `dto.go`) | Wails bindings, events, path guard, update check hook |
| `internal/download` | Task lifecycle, concurrency, playlists, queue persist, injectable engines |
| `internal/config` | JSON settings under user config dir |
| `internal/transcript` | Atomic `.srt` / `.txt` writers |
| `internal/captions` | Error classification for caption failures |
| `internal/fsutil` | Sanitization, free space, collision policy, atomic write |
| `internal/platform` | Open/reveal, downloads dir, playlist parse helpers |
| `internal/applog` | Leveled slog + ring buffer + optional file |
| `internal/netutil` | Retries, timeouts, network user messages |
| `internal/support` | Support bundle text |
| `internal/pathsafety` | Confine open/reveal to allowed roots |
| `internal/updatecheck` | Optional GitHub Releases comparison |
| `internal/model` | Task/playlist domain types |

## Download pipeline

```
AddTask → Pending → claim (Starting) → runTask worker
  → resolving (MediaEngine.Resolve)
  → downloading (MediaEngine.Download + progress)
  → captions (CaptionsEngine) [optional]
  → writing transcript files
  → Completed | Error | Paused | Stopped
```

Concurrency: `activeCount` ≤ `maxParallel`. Pause/stop set `Stopping` + `stopModes`; worker cancels context.

## Injectable engines (F1)

`MediaEngine` and `CaptionsEngine` abstract ytdlp. Production: `YTDLPEngine`. Tests inject stubs via `SetMediaEngine` / `SetCaptionsEngine`.

## Frontend

- Wails bindings under `frontend/wailsjs/`
- Events: `task:update`, `tasks:changed`, `notification`, `download:complete`, `queue:restore`
- Progress ticks update the task list incrementally (no full shell rebuild)

## Feature flags

Persisted in settings:

- `disable_captions` — skip captions; block transcript-only
- `video_only_safe_mode` — force video mode
- `check_updates` — allow background GitHub release check on startup
