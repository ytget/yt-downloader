# Packaging, signing, and runtime dependencies

## Versioning

- Canonical version string: root `VERSION` file (e.g. `1.0.0`).
- `make sync-version` copies it into `wails.json` → `info.productVersion` (macOS bundle / installer metadata).
- Native binary embeds the same value via `-ldflags "-X main.version=..."`.
- GitHub Releases should use tags `vX.Y.Z` matching `VERSION` (CI strips the leading `v`).

Override for a one-off build:

```bash
make build VERSION=1.2.3
```

## Artifacts

| Platform | Typical output under `build/bin/` | Notes |
|----------|-------------------------------------|--------|
| macOS    | `yt-downloader.app` + `yt-downloader.app.zip` | Zip is what the Homebrew cask downloads |
| Windows  | `yt-downloader.exe` (+ versioned copy) | Optional code signing |
| Linux    | `yt-downloader` (+ versioned copy)  | Requires WebKitGTK at **runtime** |

Release workflow (`.github/workflows/release.yaml`) builds all three OS targets on `v*` tags and attaches `build/bin/**` to the GitHub Release.

## macOS — sign and notarize

### Local (Developer ID)

```bash
# After make build-mac-arm64 / build-mac-amd64
codesign --force --deep --options runtime \
  --sign "Developer ID Application: YOUR NAME (TEAMID)" \
  build/bin/yt-downloader.app

# Zip for notary (preserve symlinks)
ditto -c -k --keepParent build/bin/yt-downloader.app build/bin/yt-downloader.app.zip

xcrun notarytool submit build/bin/yt-downloader.app.zip \
  --apple-id "$APPLE_ID" \
  --team-id "$APPLE_TEAM_ID" \
  --password "$APPLE_APP_SPECIFIC_PASSWORD" \
  --wait

xcrun stapler staple build/bin/yt-downloader.app
```

Or use an App Store Connect API key:

```bash
xcrun notarytool submit build/bin/yt-downloader.app.zip \
  --key "$API_KEY_PATH" \
  --key-id "$API_KEY_ID" \
  --issuer "$API_ISSUER_ID" \
  --wait
```

### CI secrets (optional)

When present, the release workflow signs/notarizes macOS bundles:

| Secret | Purpose |
|--------|---------|
| `APPLE_CERTIFICATE_P12_BASE64` | Base64-encoded Developer ID `.p12` |
| `APPLE_CERTIFICATE_PASSWORD` | Password for the `.p12` |
| `APPLE_TEAM_ID` | Team ID |
| `APPLE_ID` | Apple ID email (password auth) |
| `APPLE_APP_SPECIFIC_PASSWORD` | App-specific password |
| **or** `APPLE_API_KEY_P8_BASE64` + `APPLE_API_KEY_ID` + `APPLE_API_ISSUER_ID` | notarytool API key auth |

If certificate secrets are missing, macOS artifacts are still uploaded **unsigned** (fine for dev tags; not Gatekeeper-friendly).

## Windows — code signing

### Local

```bash
signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 \
  /f path\to\cert.pfx /p "%CERT_PASSWORD%" \
  build\bin\yt-downloader.exe
```

### CI secrets (optional)

| Secret | Purpose |
|--------|---------|
| `WINDOWS_CERTIFICATE_PFX_BASE64` | Base64-encoded `.pfx` |
| `WINDOWS_CERTIFICATE_PASSWORD` | PFX password |

When missing, the `.exe` is published unsigned.

## Linux — build and runtime dependencies

### Build (Debian/Ubuntu)

```bash
sudo apt-get update
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
# Older distros may only ship 4.0:
# sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.0-dev
```

Fedora:

```bash
sudo dnf install gtk3-devel webkit2gtk4.1-devel
```

Arch:

```bash
sudo pacman -S gtk3 webkit2gtk-4.1
```

### Runtime

End users need matching WebKitGTK/GTK shared libraries. The release binary is not fully static.

Document for users:

- Ubuntu 22.04+: `libgtk-3-0` and `libwebkit2gtk-4.1-0` (package names may vary slightly by release).
- If the app fails to start with webview/loader errors, install the WebKitGTK package for your distribution.

Optional: ship a `.deb` later; until then, GitHub Release binary + dependency notes are the distribution path alongside Homebrew cask on macOS.

## Homebrew distribution

### GUI cask (supported)

In-repo formula: `Formula/yt-downloader.rb` (cask).

- Asset URL: `https://github.com/ytget/yt-downloader/releases/download/vX.Y.Z/yt-downloader.app.zip`
- Release workflow builds macOS apps and runs `scripts/package_macos_release.sh` to create that zip.
- Settings path (zap): `~/Library/Application Support/yt-downloader/` (`settings.json` via `os.UserConfigDir()`).

On each release:

1. Publish GitHub Release with `yt-downloader.app.zip`.
2. `shasum -a 256 yt-downloader.app.zip`
3. Bump `version` + `sha256` in the cask (repo + [ytget/homebrew-yt-downloader](https://github.com/ytget/homebrew-yt-downloader) tap).

Install:

```bash
brew tap ytget/yt-downloader
brew install --cask yt-downloader
```

### CLI formula (deprecated)

`Formula/yt-downloader-cli.rb` intentionally fails install. The Wails app is GUI-only; do not advertise a CLI bottle.

## Smoke checks before publishing

```bash
make deps
make test
make frontend-typecheck
make build VERSION=$(tr -d '\n' < VERSION)
```

On each OS CI runner, `package-smoke` in `build.yaml` already exercises Wails packaging.
