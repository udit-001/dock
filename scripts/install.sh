#!/usr/bin/env bash
# Dock installer for Linux/macOS.
#
# Downloads the latest tagged release from GitHub, verifies the archive's
# sha256 against the release checksums.txt, then installs the dock binary into
# ~/.local/bin (override with INSTALL_DIR). No Go toolchain, no C compiler,
# no sudo.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/udit-001/dock/main/scripts/install.sh | bash
#   INSTALL_DIR=/opt/dock curl -fsSL https://raw.githubusercontent.com/udit-001/dock/main/scripts/install.sh | bash
#
# To uninstall: rm "$INSTALL_DIR/dock"

set -euo pipefail

REPO="udit-001/dock"
RELEASE_BASE="https://github.com/${REPO}/releases"
DOWNLOAD_BASE="${RELEASE_BASE}/download"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

# Resolve the latest tag via the /releases/latest redirect (no GitHub API, no jq).
TAG="$(curl -fsSL -o /dev/null -w '%{url_effective}' "${RELEASE_BASE}/latest")"
TAG="${TAG##*/}"

# OS / arch detection.
GOOS=""
case "$(uname -s)" in
  Linux)  GOOS="linux" ;;
  Darwin) GOOS="darwin" ;;
  *)      echo "error: unsupported OS '$(uname -s)'" >&2; exit 1 ;;
esac

GOARCH=""
case "$(uname -m)" in
  x86_64|amd64) GOARCH="amd64" ;;
  arm64|aarch64) GOARCH="arm64" ;;
  *)            echo "error: unsupported architecture '$(uname -m)'" >&2; exit 1 ;;
esac

# Published matrix today: linux/amd64, darwin/arm64, windows/amd64.
if [[ "${GOOS}/${GOARCH}" != "linux/amd64" && "${GOOS}/${GOARCH}" != "darwin/arm64" ]]; then
  echo "error: no prebuilt binary for ${GOOS}/${GOARCH}" >&2
  echo "       fall back to: go install ${REPO}@latest" >&2
  exit 1
fi

ASSET="dock_${TAG}_${GOOS}_${GOARCH}.tar.gz"

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

echo "Downloading ${ASSET}"
curl -fsSL "${DOWNLOAD_BASE}/${TAG}/${ASSET}" -o "${TMP}/${ASSET}"
curl -fsSL "${DOWNLOAD_BASE}/${TAG}/checksums.txt" -o "${TMP}/checksums.txt"

# sha256 verification against checksums.txt.
EXPECTED="$(awk -v asset=" ${ASSET}$" '$0 ~ asset {print $1}' "${TMP}/checksums.txt")"
if [[ -z "${EXPECTED}" ]]; then
  echo "error: no checksum entry for ${ASSET}" >&2
  exit 1
fi

sha256() { # portable: sha256sum on Linux, shasum on macOS
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}
ACTUAL="$(sha256 "${TMP}/${ASSET}")"

if [[ "${ACTUAL}" != "${EXPECTED}" ]]; then
  echo "error: sha256 mismatch" >&2
  echo "  expected ${EXPECTED}" >&2
  echo "  got      ${ACTUAL}" >&2
  exit 1
fi

# Install.
mkdir -p "${INSTALL_DIR}"
tar -xzf "${TMP}/${ASSET}" -C "${TMP}" dock
cp "${TMP}/dock" "${INSTALL_DIR}/dock"
chmod +x "${INSTALL_DIR}/dock"

echo "Installed dock ${TAG} -> ${INSTALL_DIR}/dock"
if [[ ":${PATH}:" != *":${INSTALL_DIR}:"* ]]; then
  echo "note: ${INSTALL_DIR} is not on your PATH."
  echo "      add it with: export PATH=\"${INSTALL_DIR}:\$PATH\""
fi
