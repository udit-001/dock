#!/usr/bin/env bash
# scripts/gen-winres.sh — generate Windows PE resource metadata (.syso) via
# go-winres, so the Dock .exe carries legitimate company/product/version
# PE metadata (no Mark-of-the-Web / AV false-positive heuristics on the binary
# resource side). Mirrors the pattern used by the managed fleet (Pharos/etc).
#
# Usage:  ./scripts/gen-winres.sh [0.0.1]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WINRES_JSON="$ROOT/winres/winres.json"
OUT_DIR="$ROOT/cmd/app-store"

GOPATH_BIN="$(go env GOPATH)/bin"
if ! command -v go-winres >/dev/null 2>&1 && [ -x "$GOPATH_BIN/go-winres" ]; then
  export PATH="$GOPATH_BIN:$PATH"
fi
if ! command -v go-winres >/dev/null 2>&1; then
  echo "  Installing go-winres..."
  go install github.com/tc-hib/go-winres@latest
  export PATH="$GOPATH_BIN:$PATH"
fi

if [[ $# -ge 1 ]]; then
  VERSION="$1"
  QUAD="$(echo "$VERSION" | awk -F. '{printf "%d.%d.%d.0", $1+0, $2+0, $3+0}')"
  cat > "$WINRES_JSON" <<EOF
{
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": { "file_version": "$QUAD", "product_version": "$QUAD" },
        "info": {
          "0409": {
            "CompanyName": "udit-001",
            "FileDescription": "Dock",
            "FileVersion": "$VERSION",
            "InternalName": "app-store",
            "LegalCopyright": "MIT License",
            "OriginalFilename": "app-store.exe",
            "ProductName": "Dock",
            "ProductVersion": "$VERSION"
          }
        }
      }
    }
  },
  "RT_GROUP_ICON": { "#1": { "0000": "appstore.ico" } },
  "RT_MANIFEST": {
    "#1": {
      "0409": {
        "identity": { "name": "app-store", "version": "$QUAD" },
        "description": "Desktop manager for the self-hosted Go fleet",
        "execution-level": "as invoker",
        "dpi-awareness": "PerMonitorV2",
        "use-common-controls-v6": true
      }
    }
  }
}
EOF
  echo "  Patched winres/winres.json -> v$VERSION"
fi

mkdir -p "$OUT_DIR"
echo "  Generating .syso files into $OUT_DIR..."
go-winres make --arch amd64,arm64 --in "$WINRES_JSON" --out "$OUT_DIR/rsrc"
echo "  ✓ Windows PE resources generated"