#!/usr/bin/env bash
# Sign Windows exe with signtool when a PFX is available.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$ROOT/build/bin}"
PFX_PATH="${WINDOWS_CERTIFICATE_PFX_PATH:-}"
PFX_PASSWORD="${WINDOWS_CERTIFICATE_PASSWORD:-}"

if [[ -z "$PFX_PATH" || ! -f "$PFX_PATH" ]]; then
  echo "No WINDOWS_CERTIFICATE_PFX_PATH; skipping Windows signing."
  exit 0
fi

if ! command -v signtool >/dev/null 2>&1; then
  # Common VS / WinSDK location on GHA windows-latest is on PATH via setup.
  echo "signtool not found on PATH; skipping Windows signing."
  exit 0
fi

shopt -s nullglob
exes=("$BIN_DIR"/*.exe)
if [[ ${#exes[@]} -eq 0 ]]; then
  echo "No .exe files in $BIN_DIR"
  exit 1
fi

for exe in "${exes[@]}"; do
  echo "Signing $exe"
  if [[ -n "$PFX_PASSWORD" ]]; then
    signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 \
      /f "$PFX_PATH" /p "$PFX_PASSWORD" "$exe"
  else
    signtool sign /fd SHA256 /tr http://timestamp.digicert.com /td SHA256 \
      /f "$PFX_PATH" "$exe"
  fi
  signtool verify /pa "$exe"
done
