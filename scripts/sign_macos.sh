#!/usr/bin/env bash
# Sign (+ optional notarize) macOS .app bundles under build/bin.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="${BIN_DIR:-$ROOT/build/bin}"
IDENTITY="${APPLE_CODESIGN_IDENTITY:-}"
NOTARIZE="${NOTARIZE:-1}"

if [[ -z "$IDENTITY" ]]; then
  # Prefer Developer ID Application identity when exactly one is present.
  IDENTITY="$(security find-identity -v -p codesigning 2>/dev/null | awk -F'\"' '/Developer ID Application/{print $2; exit}')" || true
fi

if [[ -z "$IDENTITY" ]]; then
  echo "No codesign identity (set APPLE_CODESIGN_IDENTITY). Skipping sign."
  exit 0
fi

shopt -s nullglob
apps=("$BIN_DIR"/*.app)
if [[ ${#apps[@]} -eq 0 ]]; then
  echo "No .app bundles in $BIN_DIR"
  exit 1
fi

for app in "${apps[@]}"; do
  echo "Signing $app with $IDENTITY"
  codesign --force --deep --options runtime --sign "$IDENTITY" "$app"
  codesign --verify --verbose=2 "$app"
done

if [[ "$NOTARIZE" != "1" ]]; then
  echo "NOTARIZE!=1; skipping notarization"
  exit 0
fi

for app in "${apps[@]}"; do
  zip="${app}.zip"
  echo "Zipping $app -> $zip"
  rm -f "$zip"
  ditto -c -k --keepParent "$app" "$zip"

  if [[ -n "${APPLE_API_KEY_PATH:-}" && -n "${APPLE_API_KEY_ID:-}" && -n "${APPLE_API_ISSUER_ID:-}" ]]; then
    xcrun notarytool submit "$zip" \
      --key "$APPLE_API_KEY_PATH" \
      --key-id "$APPLE_API_KEY_ID" \
      --issuer "$APPLE_API_ISSUER_ID" \
      --wait
  elif [[ -n "${APPLE_ID:-}" && -n "${APPLE_TEAM_ID:-}" && -n "${APPLE_APP_SPECIFIC_PASSWORD:-}" ]]; then
    xcrun notarytool submit "$zip" \
      --apple-id "$APPLE_ID" \
      --team-id "$APPLE_TEAM_ID" \
      --password "$APPLE_APP_SPECIFIC_PASSWORD" \
      --wait
  else
    echo "No notarization credentials; signed but not notarized."
    continue
  fi

  xcrun stapler staple "$app"
  echo "Notarized and stapled: $app"
done
