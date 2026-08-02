# User guide

## What the app does

YT Downloader is a desktop app for downloading YouTube **videos** and **playlists**, optionally saving **captions** as `.srt` and `.txt`.

## Download modes

| Mode | Media | Captions |
|------|-------|----------|
| **Video** | Progressive video+audio file | No |
| **Video + Transcript** | Media file | Yes (`.srt` + `.txt`) |
| **Transcript only** | No media | Yes (fails if no captions) |

Modes can be chosen in the main bar (saved as the default) or in Settings.

## Captions limitations

- Only **YouTube-hosted captions** (manual or ASR tracks exposed by the player). There is **no** local speech-to-text / Whisper fallback.
- Language **Auto** prefers the app language when set, otherwise the first suitable track.
- Timedtext URLs can expire; the engine refreshes player metadata once and retries.
- Private, age-gated, live, or region-blocked videos may fail with a clear error.
- Partial success: media can complete while captions fail — the row shows a **Partial** badge and warning.

See also [CAPTIONS.md](./CAPTIONS.md) and [QUALITY.md](./QUALITY.md).

## Quality presets

- **Best** — highest progressive stream (or best video-only when merging multi-language audio).
- **Medium** — progressive up to ~480p when available.
- **Audio only** — not available yet as a standalone preset; disabled in the UI.

## Audio language

If a video has **multiple audio languages**, the app asks you to pick one before downloading.
Selecting a non-default track downloads separate video/audio streams and merges them with **ffmpeg** (must be installed and on your `PATH`).

## File locations

| What | Typical location |
|------|------------------|
| Downloads | Your chosen folder (default: system Downloads) |
| Settings | macOS: `~/Library/Application Support/yt-downloader/settings.json`<br>Linux: `~/.config/yt-downloader/settings.json`<br>Windows: `%AppData%\yt-downloader\settings.json` |
| Queue snapshot | Same config dir: `queue.json` |
| Logs | Config dir `logs/app.log` (if file logging enabled) |
| Support bundles | Written under the logs directory when copied |

## Queue restore

On startup, if a previous session left a saved queue, the app asks whether to **Restore** (paused + history) or **Discard**.

## Batch actions

- **Pause all** — pause active downloads (keeps partial files when possible).
- **Retry failed** — re-queue tasks in Error.
- **Clear completed** — remove Completed rows from the list (does not delete files).

## Troubleshooting

| Symptom | Try |
|---------|-----|
| Network / DNS / TLS errors | Check connectivity; retry; see logs |
| No captions | Switch language; use Video-only; check video has CC on YouTube |
| Cannot open/reveal file | Path must stay under download/config/log folders (security confine) |
| Corrupt settings | App resets to defaults and keeps `settings.json.corrupt` |
| Need help diagnosing | Settings → **Copy support bundle** (paths redacted) |

## Privacy / network

Outbound traffic is for YouTube media/captions (and playlist parse) only, unless you enable **Check for updates** (GitHub Releases API) or run a manual update check. Details: [NETWORK.md](./NETWORK.md).
