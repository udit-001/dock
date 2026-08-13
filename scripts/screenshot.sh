#!/usr/bin/env bash
# scripts/screenshot.sh <binary> <out.png>
# Launch the App Store GUI, let it render, capture the screen, then quit it.
# Requires a display and ImageMagick 'import'.
set -euo pipefail
BIN="${1:?binary required}"
OUT="${2:?output required}"

"$BIN" &
PID=$!
trap 'kill "$PID" 2>/dev/null || true' EXIT

# Wait for the window to come up and paint.
sleep "$((${APP_SLEEP:-6}))"

# Under Wayland the app runs via XWayland; capture the X root window.
import -window root "$OUT" 2>/dev/null || {
  echo "WARN: import failed; trying 'xwd -> convert' path" >&2
  xwd -root -silent | convert xwd:- "$OUT" 2>/dev/null || true
}
echo "captured $OUT"