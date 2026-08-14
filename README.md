# Dock

Desktop manager for a fleet of self-hosted Go apps (Pharos, Harbor, Waypoint). It reads the embedded manifest, checks GitHub releases directly, and installs/updates each app — verifying the sha256, swapping the binary atomically, and restarting daemons around the change.

## Quick start

```bash
go install github.com/udit-001/dock@latest
dock
```

- **Windows** needs a Go toolchain with gcc (mingw-w64) — Fyne/GLFW requires CGO.
- **Linux / macOS** can also download a binary from [Releases](https://github.com/udit-001/dock/releases).

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


