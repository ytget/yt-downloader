# Captions / transcripts

How yt-downloader obtains YouTube captions and writes `.srt` / `.txt`.

## Fallback chain (track discovery)

Implemented in `github.com/ytget/ytdlp/v2` (`Downloader.fetchPlayerResponse` / `DownloadCaptions`):

1. **ANDROID Innertube `/player`** (default client `ANDROID` / `20.10.38`)  
   Most reliable source of `captionTracks` for ordinary public videos.
2. **Watch-page `ytInitialPlayerResponse`**  
   Used only when ANDROID returns a player payload with **zero** caption tracks and the caller did not force a custom Innertube client.
3. **Timedtext download formats** (per selected track `baseUrl`):  
   `srv1` → `json3` → default URL → `srv3` → `vtt`.

## Expired timedtext URLs

Caption `baseUrl` values are signed and can expire. On HTTP **401 / 403 / 410** (or an explicit “timedtext expired” error), `DownloadCaptions` **refreshes player metadata once** and retries download with a new track URL.

## Language selection

Preference order for `preferredLang` (empty / `auto` = no preference):

1. Manual track matching language  
2. ASR (auto-generated) track matching language  
3. First manual track  
4. First any track  

App UI: **Transcript language** setting (`auto`, `en`, `ru`, `pt`, …). Main bar mode can be Video / Video+Transcript / Transcript only.

## Error classes (user-facing)

| Situation | Typical message |
|-----------|-----------------|
| No tracks + playable | No captions available for this video |
| Age gate | captions unavailable: video is age-restricted |
| Private / login | captions unavailable: video is private or requires sign-in |
| Region block | captions unavailable: video is region-blocked |
| Live | captions unavailable: live streams are not supported yet |
| Timedtext fail after retry | underlying HTTP / parse error |

The download service maps these into `TranscriptStatus` (`unavailable` vs `error`) and, for **Video + Transcript**, keeps media success with a **warning** (partial success).

## Outputs

- `{title}.{lang}.srt` — timed SubRip  
- `{title}.txt` — plain wrapped text  

Writes are atomic (temp + rename). Collision policy (skip / overwrite / rename) applies.

## Tests

- Unit/fixture parsers: `ytdlp/v2/youtube/captions` (XML srv1, json3, entities) — no live network.  
- App error classification: `internal/captions`  
- Optional live e2e: `go test -tags e2e ./internal/download` (`transcript_e2e_test.go`).
