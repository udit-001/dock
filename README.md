# Dock

Desktop manager for a fleet of self-hosted Go apps (Pharos, Harbor, Waypoint). It reads the embedded manifest, checks GitHub releases directly, and installs/updates each app — verifying the sha256, swapping the binary atomically, and restarting daemons around the change.

## Quick start

Prebuilt binaries (no Go toolchain, no C compiler):

Linux / macOS:

```bash
curl -fsSL https://cdn.jsdelivr.net/gh/udit-001/dock@main/scripts/install.sh | bash
```

Windows (PowerShell):

```powershell
powershell -ExecutionPolicy Bypass -Command "irm https://cdn.jsdelivr.net/gh/udit-001/dock@main/scripts/install.ps1 | iex"
```

Both installers download the latest tagged release, verify its sha256 against
`checksums.txt`, and install the binary to a user-writable location (`~/.local/bin`
on Unix, `%LOCALAPPDATA%\Programs\Dock` on Windows).

Power users can still build from source:

```bash
go install github.com/udit-001/dock@latest
dock
```

- **Windows**: `go install` needs a Go toolchain with gcc (mingw-w64) — Fyne/GLFW requires CGO; the prebuilt binary avoids this.

## How it works

- Fleet manifest lives in `internal/appdata/manifest.yaml` (single source of truth)
- Metadata comes from the GitHub releases API (latest version, per-platform assets, sha256)
- Updates: download → verify sha256 → atomic swap → restart daemon

## Using

- **Install / Update** — install a missing app or upgrade to the latest
- **Update all** — apply every pending update at once
- **Launch** — open an app's dashboard in a standalone browser window
- **Check for updates** — refresh metadata; the button shows progress while it runs

## Development

```bash
make deps     # local X11 dev symlinks for Fyne/GLFW
make run      # build & open the desktop app
make test     # unit tests (fakes at the seams, no display/network)
make gen      # regenerate apps.json from the manifest
```

## Releases

Built per-OS with CGO on tag push (`git tag v0.x.x && git push --tags`). Windows: `go install`; Linux/macOS: tar.gz archives with `checksums.txt`.


