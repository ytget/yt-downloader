# Network behavior

YT Downloader only opens network connections for documented purposes. There is no analytics, ad, or telemetry channel.

## Always (during downloads)

| Destination | Purpose |
|-------------|---------|
| YouTube / Google video hosts | Resolve player metadata, progressive media, timedtext captions, playlist HTML/API as used by `ytdlp` |
| DNS | Resolve those hosts |

No third-party CDN or crash reporter is bundled for phone-home.

## Optional

| Trigger | Destination | Purpose |
|---------|-------------|---------|
| Settings **Check for updates** on startup, or **Check for updates** action | `api.github.com` (`/repos/ytget/yt-downloader/releases/latest`) | Compare SemVer tags |

Update checks are **off by default**.

## Local only

- Settings, queue, logs under the user config directory
- Open/reveal use the OS file manager / default app (no upload)
