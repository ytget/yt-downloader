#!/usr/bin/env bash
# Produce yt-downloader.app.zip for Homebrew cask consumption.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$ROOT/build/bin}"

shopt -s nullglob
apps=("$BIN_DIR"/*.app)
if [[ ${#apps[@]} -eq 0 ]]; then
  # Wails sometimes nests outputs; search one level.
  apps=("$BIN_DIR"/*/*.app)
fi
if [[ ${#apps[@]} -eq 0 ]]; then
  echo "No .app bundles found under $BIN_DIR" >&2
  exit 1
fi

# Prefer a single arm64/universal app named yt-downloader.app for the cask.
primary=""
for app in "${apps[@]}"; do
  base="$(basename "$app")"
  if [[ "$base" == "yt-downloader.app" ]]; then
    primary="$app"
    break
  fi
done
if [[ -z "$primary" ]]; then
  primary="${apps[0]}"
fi

# Ensure canonical name next to zip target dir.
stage="$BIN_DIR/yt-downloader.app"
if [[ "$primary" != "$stage" ]]; then
  rm -rf "$stage"
  cp -R "$primary" "$stage"
fi

zip_path="$BIN_DIR/yt-downloader.app.zip"
rm -f "$zip_path"
ditto -c -k --keepParent "$stage" "$zip_path"
echo "Created $zip_path"
ls -la "$zip_path"
