# Stability task list

Source of truth for making yt-downloader a stable product application.

**How to use**
- Mark items `[x]` only when fully done (code + tests/docs as needed).
- Keep status notes short under each item when useful.
- Prefer smallest vertical slice that ships value.
- Do **not** start ASR/Whisper, mobile, or unrelated features unless a task explicitly says so.

**Status legend**
- `[ ]` not started
- `[~]` in progress
- `[x]` done

---

## Phase A — Distribution foundation (P0)

- [~] **A1. Publish and pin ytdlp captions support**
  - Publish tagged `ytdlp/v2` including captions API used by this app.
  - Remove `replace github.com/ytget/ytdlp/v2 => ../../ytdlp-src` from `go.mod`.
  - Pin exact module version; verify clean checkout builds without local ytdlp path.
  - **Progress:** Captions API committed; fork release `bangoo2bangoo/ytdlp@v2.1.0`; PR https://github.com/ytget/ytdlp/pull/18. App now pins the fork release instead of a local path; `go test ./...` passes. Audio-track probing remains disabled because that API is not in the published fork.
  - **Blocked:** no push rights on `ytget/ytdlp`. After upstream merge+tag on proxy, drop the fork `replace` and pin `github.com/ytget/ytdlp/v2@v2.1.0` only.
- [x] **A2. Release / CI hygiene**
  - Single version source (`main.version`, `wails.json`, release tags).
  - CI builds on macOS, Windows, Linux via Wails.
  - CI runs `go test`, frontend typecheck/build, and packaging smoke.
  - **Done:** `VERSION` file + `make sync-version` → `wails.json` `productVersion`; ldflags `-X main.version` via overridable `VERSION=?=`. CI `verify` (vet/test/tsc/frontend build) + matrix `package-smoke` (ubuntu/windows/macos). Release workflow passes tag version into Make.
- [x] **A3. Signing and packaging**
  - macOS sign + notarize app bundle.
  - Windows code signing for release artifacts.
  - Linux artifact + WebKitGTK dependency notes.
  - **Done:** `docs/PACKAGING.md`; `scripts/sign_macos.sh` / `sign_windows.sh`; `make sign-mac` / `sign-windows`; release workflow optional sign/notarize when cert secrets are set (unsigned publish if absent). Linux WebKitGTK build/runtime notes in packaging doc + README.
  - **Note:** actual Developer ID / Authenticode certs are org secrets — not creatable in-repo.
- [x] **A4. Homebrew / distribution metadata**
  - Cask points at real release artifacts.
  - Correct zap/config paths for Wails settings location.
  - Clarify GUI cask vs any CLI formula.
  - **Done:** cask `version 1.0.0` → `yt-downloader.app.zip`; zap includes Application Support settings + Wails caches; CLI formula deprecated with odie; release packages app.zip; docs in PACKAGING.md/README. Sha256 `:no_check` until first Wails release asset is published to the tap.

---

## Phase B — Core correctness (P0/P1)

- [x] **B1. Task lifecycle hardening**
  - Pause/resume guarantees with partial files.
  - Remove-while-downloading cancels work and frees slots.
  - Restart clears media + transcript fields/warnings correctly.
  - Clear duplicate-URL policy (active vs finished; intentional re-download).
  - **Done:** claim-before-start (`queueStartLocked`); resume keeps `OutputPath`; restart clears media/transcript/warning; remove records `StopModeStop` and worker `finishWorker` frees the slot without mutating the worker-owned task; duplicate URL rejects non-terminal (incl. paused), allows after Completed/Stopped/Error; invariants in `internal/download/doc.go` + unit tests.
- [x] **B2. Disk and path safety**
  - Writable download dir checks on settings save and task start.
  - Free-space check before media download.
  - Atomic writes for `.srt` / `.txt` (temp + rename).
  - Strong filename sanitization (Windows reserved names, length, unicode).
  - Configurable collision policy: skip / overwrite / rename.
  - **Done:** `internal/fsutil` (writable probe, free-space, sanitize, collision, atomic write); settings validate dir + `collision_policy`; service checks before download; transcript atomic writes + collision; UI setting for policy.
- [x] **B3. Concurrency cleanup**
  - Document service invariants (`activeCount`, stop modes, timers).
  - Stress tests for pause/remove/add storms.
  - Ensure timers/goroutines cannot leak after terminal task states.
  - **Done:** invariants in `doc.go` (B1); `SmoothingState.stopped` blocks timer restart after stop; early abort + `finalizeAbort` on remove/pause; `TestPauseRemoveAddStorm` + timer stop test.
- [x] **B4. Playlist correctness**
  - Apply selected mode/language per playlist item (not only service defaults).
  - Cancel playlist cancels in-flight item downloads.
  - Playlist progress/status visible and accurate in UI.
  - Retry failed playlist items; skip existing outputs option.
  - **Done:** playlist stores Mode/TranscriptLang/SkipExisting/Cancelled; items use `AddTaskWithOptions`; processPlaylist waits on item completion; CancelPlaylist stops linked tasks; RetryFailedPlaylistItems API; ticker leak fixed; progress/speed mirrored onto playlist videos.

---

## Phase C — Captions / transcripts reliability (P1)

- [x] **C1. Captions client fallback matrix**
  - Documented fallback chain (e.g. ANDROID → watch-page JSON → other).
  - Retry on expired timedtext URLs (refresh player metadata).
  - Better errors for private/age-gated/live/region-blocked/no-captions.
  - **Done:** `docs/CAPTIONS.md`; ytdlp `v2.1.1` refresh-once on expired timedtext + playability classification; app `internal/captions.ClassifyError`.
- [x] **C2. Fixture-based caption tests**
  - Recorded player JSON + timedtext samples (manual, ASR, multi-lang, none).
  - Unit/contract tests do not require live YouTube for core parsing paths.
  - Keep one optional live e2e behind build tag.
  - **Done:** ytdlp `youtube/captions/testdata` fixtures + `fixtures_test.go`; app classifier unit tests; optional live e2e via `-tags e2e`.
- [x] **C3. Transcript UX completeness**
  - Explicit language picker (plus Auto).
  - Retry transcript-only for an existing completed media task.
  - Open/reveal/copy paths for video, SRT, TXT.
  - Partial-success always obvious in UI.
  - **Done:** main-bar + settings caption language; `RetryTranscript` / `CopyToClipboard`; reveal SRT/TXT; partial badge + warning styling.

---

## Phase D — Operability (P1)

- [x] **D1. Structured logging**
  - Leveled logger; task-id correlation.
  - No full Innertube/player JSON dumps in normal runs.
  - Optional log file + “Copy logs” / support bundle action.
  - **Done:** `internal/applog` (slog + ring buffer + optional `app.log`); task_id via `WithTask`; attr truncation / bulk-JSON omit; settings `log_level` / `log_to_file`; UI Copy/Reveal logs; download service correlated logs. Support bundle remains D4.
- [x] **D2. Crash and shutdown safety**
  - Recover panics in worker goroutines; mark task error.
  - Graceful shutdown cancels contexts and does not corrupt outputs.
  - Settings load failure falls back to defaults instead of hard-fatal when possible.
  - **Done:** worker `recover` → task Error; `Shutdown` pauses in-flight + `workerWG`; Wails `OnBeforeClose`/`OnShutdown`; corrupt `settings.json` quarantined to `.corrupt` + defaults + warning.
- [x] **D3. Network resilience**
  - Retries/backoff for transient media and caption HTTP failures.
  - Explicit timeouts per phase (resolve / media / captions).
  - User-friendly offline/DNS/TLS messages.
  - **Done:** `internal/netutil` retry/backoff + classification; resolve/captions timeouts; media limited retries; friendlier DNS/TLS/offline/timeout messages in `parseYouTubeError`.
- [x] **D4. Support bundle**
  - App/OS/Wails versions, recent task errors, sanitized settings.
  - Redact secrets/tokens; optional path redaction.
  - **Done:** `internal/support` text bundle + file write; `CopySupportBundle` API/UI; path basenames when redacting.

---

## Phase E — Product durability (P2)

- [x] **E1. Persist queue and history**
  - Survive app restart (incomplete + completed history).
  - Restore policy with user confirmation.
  - **Done:** `queue.json` debounced persist; active→Paused on disk; restore banner Confirm/Discard APIs.
- [x] **E2. Frontend performance**
  - Avoid full-list DOM rerender on every progress tick.
  - Incremental row updates; virtualize long lists.
  - Preserve scroll position during updates.
  - **Done:** shell vs task-list split; `task:update` patches list only; scrollTop preserved; force rebuild on filter/batch.
- [x] **E3. Settings polish**
  - Validate inputs; open download folder; reset defaults.
  - Mode default persistence clearly reflected in UI.
  - **Done:** open folder, reset defaults, parallel/dir validation; main-bar mode/lang auto-save to settings.
- [x] **E4. Job model clarity**
  - Phases: resolving → downloading → captions → writing.
  - Batch actions: pause all, clear completed, retry failed.
  - **Done:** `TaskPhase` on model/DTO/UI badge; Pause all / Clear completed / Retry failed.
- [x] **E5. Quality preset honesty**
  - Implement real audio-only **or** hide/disable until ready.
  - Document what best/medium/audio actually mean.
  - **Done:** audio disabled in UI; `docs/QUALITY.md`; backend maps legacy audio→`worst` progressive; help strings via `GetQualityHelp`.

---

## Phase F — Quality engineering (P3)

- [x] **F1. Injectable ytdlp interfaces**
  - Abstract media + captions dependencies for unit tests.
  - **Done:** `MediaEngine`/`CaptionsEngine` + `YTDLPEngine`; service injection; stub unit tests.
- [x] **F2. CI quality gates**
  - `go test ./...`, frontend `tsc`/build, `go vet`, `govulncheck`, npm audit policy.
  - **Done:** `make vet|govulncheck|npm-audit|security`; CI verify job runs govulncheck + npm audit high+.
- [x] **F3. Security hygiene**
  - Path open/reveal confined sensibly.
  - Dependency updates process.
  - No unexpected third-party network calls.
  - **Done:** `internal/pathsafety` on open/reveal/auto-reveal; `docs/DEPENDENCIES.md` + `docs/NETWORK.md`.

---

## Phase G — Docs and maintenance (P4)

- [x] **G1. User documentation**
  - Modes, captions limitations, file locations, troubleshooting.
  - **Done:** `docs/USER_GUIDE.md` + README doc index.
- [x] **G2. Developer/release documentation**
  - Architecture, ytdlp bump process, release checklist, CHANGELOG/SemVer.
  - **Done:** `docs/ARCHITECTURE.md`, `docs/RELEASING.md`, `CHANGELOG.md`.
- [x] **G3. Optional auto-update**
  - GitHub releases check or platform updater (only after signed releases exist).
  - **Done:** `internal/updatecheck` + settings `check_updates` (default off) + manual Check now; notice only (no silent install).
- [x] **G4. Feature flags / kill switches**
  - Disable captions globally, force client, video-only safe mode.
  - **Done:** `disable_captions`, `video_only_safe_mode` settings → service flags (transcript-only blocked when captions off).

---

## Explicitly deferred (do not implement unless requested)

- ASR / Whisper speech-to-text fallback
- Burn/embed subtitles into MP4
- Cloud accounts / remote queues
- Mobile clients
- Browser extension

---

## Progress log

| Date | Item | Note |
|------|------|------|
| 2026-08-02 | — | Task list created from stability planning pass. |
| 2026-08-02 | A1 | Captions API in ytdlp; fork v2.1.0 + PR #18; app uses fork replace; upstream tag blocked on ytget push access. |
| 2026-08-02 | A2 | VERSION file + sync-version; multi-OS package-smoke CI; release tag → ldflags/productVersion. |
| 2026-08-02 | A3 | Packaging/signing docs + scripts; optional CI signing hooks; Linux WebKitGTK notes. |
| 2026-08-02 | A4 | Cask → app.zip + Wails zap paths; CLI formula deprecated; release packages Homebrew zip. |
| 2026-08-02 | B1 | Lifecycle: claim/start, remove cancels+frees slot, restart clears outputs, dup-URL policy + tests. |
| 2026-08-02 | B2 | fsutil path safety; collision policy setting; atomic srt/txt; free-space + writable checks. |
| 2026-08-02 | B3 | Smoothing stop flag; abort/finalize on remove; pause/remove/add storm test. |
| 2026-08-02 | B4 | Playlist mode/lang per item; cancel stops tasks; wait-for-completion; retry API. |
| 2026-08-02 | C1–C3 | Captions fallback/docs/retry; fixtures; transcript lang picker, retry CC, partial UX. |
| 2026-08-02 | D1 | Structured applog + file/ring buffer; Copy/Reveal logs; task_id correlation. |
| 2026-08-02 | D2–D4 | Panic/shutdown safety; netutil retries/timeouts; support bundle. |
| 2026-08-02 | E1–E5 | Queue persist/restore; FE incremental list; settings polish; phases/batch; quality honesty. |
| 2026-08-02 | F1–F3 | Injectable engines; CI govulncheck/npm audit; path confine + dep/network docs. |
| 2026-08-02 | G1–G4 | User/dev docs; optional update check; captions/video-only kill switches. |
