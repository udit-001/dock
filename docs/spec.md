# Dock — Spec

> Tracker: APPS (lific) · Language decision: **Go (+ Fyne GUI)**
> Status: spec, ready-for-agent

## Problem Statement

The user maintains several self-hosted, single-binary Go tools — **Pharos** (learning CLI + web dashboard), **Harbor** (HTML page library), **Cresto/income-tracker** (income/file-return tracker), **Waypoint** (job tracker) — each released to GitHub under `udit-001/*` via GoReleaser with a version injected into `internal/version` and a Windows PE resource attached through `go-winres`. Each ships a background daemon alongside the CLI.

Today there is no single way to see what is installed, whether an upgrade is available, or to actually install/update those tools from one place. The user wants a minimal **desktop app** that lists the whole fleet, shows each app's name, description, icon, latest version and current installed version, and performs installs/upgrades — which for daemon-backed apps must stop the daemon, swap the binary, and restart it. Publishing on Windows is painful (winget approval is slow; downloaded binaries get the Mark-of-the-Web SmartScreen warning), so the manager itself must be **installed by local compilation** (`go install`) so it carries no MoW and lands fast.

## Solution

A **manifest-driven app manager**:

1. A hand-maintained `manifest.yaml` in *this* repo declares the fleet: each app's GitHub repo, a clean display name, a PNG icon, and the daemon stop/start/status control commands.
2. A **GitHub Action** (scheduled + manual + on-demand) reads the manifest, calls the GitHub API for each repo (`repos/{owner}/{repo}/releases/latest` and `repos/{owner}/{repo}`), resolves the per-platform binary asset + its sha256 from the uploaded `checksums.txt`, fetches the repo description, and writes a generated **`apps.json`** into the repo.
3. `apps.json` + the PNG icons are served to the desktop app through **jsDelivr** (`https://cdn.jsdelivr.net/gh/udit-001/dock@<branch>/apps.json`).
4. A **Go + Fyne desktop app** fetches `apps.json`, lists each app (icon, clean name, description, latest version), probes its own managed install directory to detect the **installed version** (via `<binary> version`), semver-compares to show **Up-to-date / Upgrade available / Not installed**, and offers per-app **Install / Update** plus **Update All** and **Check now**. Updates stop the daemon, download + sha256-verify the asset, atomically swap the binary, and restart the daemon.

The whole fleet stays **Go**; the manager is installed via `go install github.com/udit-001/dock@latest` (locally compiled → no Mark-of-the-Web, no winget dependency).

## User Stories

1. As a user, I want to open the app-store and see every managed tool (Pharos, Harbor, Cresto, Waypoint) with its icon, clean name, and GitHub description, so that I get one overview of my whole fleet.
2. As a user, I want each app row to show its **latest available version** and **currently installed version** (or "Not installed"), so that I can tell at a glance what needs updating.
3. As a user, I want a **Check now** action that re-fetches the latest manifest from jsDelivr, so that I don't have to wait for the scheduler.
4. As a user, I want a **per-app Install** action on apps that aren't installed, so that I can add a new tool from the manager.
5. As a user, I want a **per-app Update** action on installed apps with an upgrade available, so that I can update one tool without touching the others.
6. As a user, I want **Update All** to install/upgrade every app that has an available newer version in one action, so that I can sync the whole fleet at once.
7. As a user, when an installed app has a background daemon, I want Update to **stop the daemon before swapping and restart it after**, so that the service comes back up on the new binary without manual intervention.
8. As a user, I want every download **verified against the sha256 in the manifest's checksums**, so that a corrupted or tampered binary is never installed.
9. As a user, I want the binary **replaced atomically** (temp file + rename), so that a crash mid-update never leaves a corrupt install.
10. As a user, I want to click an app's row/icon to open its GitHub page, so that I can read full docs.
11. As a user, I want to see whether an installed app's daemon is currently **running or stopped**, so that I know the live state of each service.
12. As a user, I want a **per-app Uninstall/Stop** capability where the product supports it, so that I can remove a tool I no longer need.
13. As a user, I want the manager to be installable with `go install github.com/udit-001/dock@latest` and launched as a real desktop window with proper Windows PE metadata + icon (winres), so that there is no Mark-of-the-Web warning and no winget wait.
14. As a maintainer, I want the fleet declared in one `manifest.yaml` so that adding/removing an app is a one-line change.
15. As a maintainer, I want the GitHub Action to regenerate `apps.json` **without me having to remember to**, so that users always fetch fresh latest versions.
16. As a maintainer, I want the generated `apps.json` committed to the repo so that jsDelivr serves a versioned, cache-able snapshot.

## Implementation Decisions

### Language & GUI
- **Go**, not Rust. The deciding factor is distribution: `go install path@latest` satisfies "install easily / locally compiled / no Mark-of-the-Web". Rust has no equivalent ergonomic path for a personal local tool (crates publishing + heavy compile).
- **Guard against the GUI eroding the "install easily" story:** the GUI must be embeddable into a single `go install`-able binary with all assets (`//go:embed`) — no external runtime, no Node toolchain at install time.
- GUI framework default: **Fyne** (pure Go, single static binary, native list + progressbar + buttons, `fyne package` for app icon + PE resources, local build → no MoW). Fallback if a materially prettier UI is required: **Wails** (webview frontend) at the cost of a Node toolchain + WebView2 — only choose it if Fyne's look is unacceptable. Rust alternatives (`egui/eframe`, **Tauri**) rejected on the `go install` criterion.

### Repo layout
- `manifest.yaml` — source of truth, hand-maintained fleet declaration.
- `.github/workflows/update-manifest.yml` — generator Action.
- `scripts/generate_manifest` (Go) — Action entrypoint that turns `manifest.yaml` + GitHub API into `apps.json`.
- `apps.json` — committed, generated artifact, served via jsDelivr.
- `assets/icons/<id>.png` — app icons, served via jsDelivr.
- `cmd/app-store` + `internal/...` — the Fyne desktop app.
- `internal/version` + `scripts/gen-winres.sh` + `.goreleaser.yaml` — mirror the exact release/winres/ldflags infra the managed fleet already uses.

### Manifest schema (`manifest.yaml`)
```yaml
apps:
  - id: pharos
    repo: udit-001/pharos        # owner/repo used for GitHub API + release download
    display_name: Pharos          # optional clean name; falls back to repo name
    icon: assets/icons/pharos.png # repo file, served via jsDelivr @<branch>
    binary: pharos                # stripped binary name in the managed install dir
    # assets are resolved by the Action from GoReleaser's name_template:
    #   {{ProjectName}}_{{Version}}_{{Os}}_{{Arch}}  (+ .exe / .zip / .tar.gz)
    daemon:                       # optional; how the manager controls the service
      stop:  [pharos, daemon, stop]
      start: [pharos, daemon, start]
      status: [pharos, daemon, status]
```

### Generated `apps.json` schema
```json
{
  "generated_at": "2026-…",
  "apps": [
    {
      "id": "pharos",
      "display_name": "Pharos",
      "repo": "udit-001/pharos",
      "homepage": "https://github.com/udit-001/pharos",
      "description": "…(from GitHub API)…",
      "icon": "https://cdn.jsdelivr.net/gh/udit-001/dock@main/assets/icons/pharos.png",
      "daemon": { "has_daemon": true, "stop": […], "start": […], "status": […] },
      "latest_version": "v0.3.0",
      "published_at": "…",
      "prerelease": false,
      "assets": {
        "windows/amd64": { "url": "https://github.com/…/releases/download/v0.3.0/pharos_0.3.0_windows_amd64.exe", "sha256": "…", "size": 12345 },
        "linux/amd64":    { "url": "…", "sha256": "…", "size": 12345 },
        "darwin/arm64":   { "url": "…", "sha256": "…", "size": 12345 }
      }
    }
  ]
}
```

### Manifest → apps.json generator (the Action)
- Triggers: `schedule` (e.g. daily) + `workflow_dispatch` (manual Check now backing) + an optional `repository_dispatch` so a managed repo's release can ping it.
- For each app: `GET /repos/{owner}/{repo}/releases/latest` → `tag_name`, `published_at`, `assets[{name, browser_download_url, size}]`, `prerelease`; `GET /repos/{owner}/{repo}` → `description`.
- **Asset resolution:** reconstruct the GoReleaser name per platform from the app's `project_name` + `latest_version` + `os/arch`, pick the matching asset (Windows `binary`/`zip` → `.exe`/`.zip`; others `tar.gz`). If an expected platform asset is absent, omit that platform entry rather than failing the whole run.
- **Checksum:** download `checksums.txt` (sha256 lines `hash  filename`) and join each asset's sha256 into `apps.json`. This is the seam where download integrity is guaranteed end-to-end.
- Commit `apps.json` back to the repo with a bot user so jsDelivr re-serves a fresh snapshot.

### jsDelivr
- Serves only metadata: `apps.json` + `assets/icons/*.png`. Binaries are GitHub release URLs (jsDelivr is a repo-content CDN, not a release-asset CDN).
- URL form `https://cdn.jsdelivr.net/gh/udit-001/dock@<branch>/…` (default branch `main`). In the manager, the branch is configurable for testing.

### Desktop app (Go + Fyne)
- Tabs/rows: app icon, display name, description, latest vs installed version, daemon status chip, and Install/Update/Uninstall/Open actions; a Check-now button and an Update-All button at the top.
- **Installed-version seam:** the manager installs binaries into the OS-standard user bin dir (`~/.local/bin` on Linux/macOS honouring `XDG_BIN_HOME`, `%LOCALAPPDATA%\Programs` on Windows), and detects installed apps across the managed bin root, `$GOBIN`/`$GOPATH/bin`, and `$PATH`. Detection = execute `<binary> version` (or `--version`), parse `vX.Y.Z`, semver-compare with the latest. Until every app exposes a `version`/`daemon` subcommand, add it to those apps as a prerequisite (see Fleet gaps below).
- **Download seam:** keep the HTTP client behind an interface with a progress callback so the Fyne progress bar can subscribe without the transport knowing about the GUI.
- **Verify:** sha256 of the downloaded bytes must equal the manifest `sha256` before install.
- **Atomic swap:** write to `<binary>.new`, then `os.Rename`/replace over the live path. On Windows rename-over-running-file requires the daemon stopped and the target not locked.
- **Upgrade flow (daemon apps):** run `daemon stop` → wait for exit / confirm stopped → download+verify+swap → run `daemon start`.
- Failures leave the previous binary intact and surface a per-app error message.
- **Config:** a local config file records managed apps + their install paths so detection is deterministic across launches.

### Windows, winres, Mark-of-the-Web
- The manager ships exactly like the fleet: `scripts/gen-winres.sh` (tc-hib/go-winres) attaches product/company/version + DPI manifest in CI `before` hooks, mirroring each managed repo's `.goreleaser.yaml`. Locally built via `go install`, the binary carries no Zone.Identifier ADS → no SmartScreen/MoW prompt.

## Testing Decisions

- **What makes a good test:** exercise external behaviour at the highest seam that does not require a real network or a real display. GUI paint logic is kept thin; state/decision logic lives in testable packages.
- **Highest seam:** a fake `apps.json` (in-memory or fixture) + a fake install-dir + a fake HTTP client/fake daemon executor. Assert decisions: "upgrade available" / "up-to-date" / "not installed", whether an update stops+starts a daemon around a verified+atomic swap, and that a mismatched sha256 aborts before any file is replaced.
- **Modules tested:**
  - `internal/fleet` — semver comparison and upgrade decision (pure function).
  - `internal/store` — installed-version detection + atomic swap lifecycle against a temp dir and a fake executor.
  - `internal/updater` — download→verify→swap→(stop/start) orchestration with an injected HTTP client and an injected daemon controller.
  - `scripts/generate_manifest` — manifest→apps.json mapping + asset resolution + checksums join against recorded GitHub API fixtures.
- **Prior art:** the managed repos (Pharos/Harbor/Waypoint) already have `internal/version` + goreleaser + checksums as unit/CI tested patterns; mirror their conventions.
- GUI layer is smoke-tested manually (or via `fyne` screenshot tests where cheap); no full UI automation required.

## Out of Scope

- Publishing the manager via **winget** / scoops / brew taps (open decision — winget approval is slow, so first ship is `go install`; taps can be added later).
- Code-signing the manager (no MoW is achieved via local build; signing only matters for distributed signed installers, out of scope).
- Rebuilding the managed apps' features — this manager only installs/updates/runs them.
- Auto-update of the **manager** itself (detect newer manager release and prompt; nicety, deferred).
- A system tray / auto-start for the manager (the apps have daemons; the manager is on-demand).
- Cross-platform parity beyond what is cheap — Windows is the primary target; darwin/linux are best-effort.

## Further Notes

- **Fleet prerequisite gaps:** the manager needs each managed app to (a) expose a parseable version via `version`/`--version`, and (b) expose deterministic `daemon stop/start/status` subcommands. The generator can surface "missing asset for platform X" and the manager can show "daemon control unavailable" as degrade gracefully, but for full Update-All the fleet must be brought to a consistent CLI surface — tracked as prerequisite tickets before/with the first real upgrade runs.
- **Prerelease handling:** `latest` honors GitHub's own semantics (excludes prereleases by default for the `/releases/latest` endpoint); the `prerelease` flag is stored for the UI to show a "pre-release" hint if a specific release is later pinned.
- **Cresto/income-tracker** has no committed `.goreleaser.yaml` yet — it needs a release pipeline (and possibly a `version` command) before it can join the fleet cleanly; treat as a prerequisite.