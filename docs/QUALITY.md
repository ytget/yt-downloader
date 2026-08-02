# Quality presets

The download engine uses **progressive** YouTube streams (combined video+audio), not separate DASH audio/video muxing.

| Preset | Selector | Meaning |
|--------|----------|---------|
| **best** | `best` + prefer `mp4` | Highest progressive quality available |
| **medium** | `height<=480` + prefer `mp4` | Cap around 480p when such a progressive format exists |
| **audio** | _unavailable_ | True audio-only is **not supported** yet. The UI disables this option. Legacy settings that still say `audio` fall back to the smallest progressive stream (`worst`) server-side. |

When captions/transcript modes run, media quality selection is unchanged; captions are fetched separately.

## Audio language (multi-track)

Some YouTube videos expose **multiple spoken audio languages** as adaptive audio streams.

1. Before download, the app probes available audio tracks.
2. If more than one track exists, a picker lets you choose language.
3. Download uses adaptive **video-only + selected audio**, then **ffmpeg** muxes them (`-c copy`).
4. Requires `ffmpeg` on `PATH`. If missing, the UI warns; progressive fallback may ignore the language choice.

Transcript-only mode skips the audio picker.
