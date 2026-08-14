# App Store

A desktop manager for the self-hosted Go fleet (Pharos, Harbor, Waypoint): it
reads an embedded manifest, queries **GitHub releases directly** (never
`api.github.com` as a download URL), and installs/updates each app — stopping
and restarting the daemon around a **sha256-verified atomic swap**.

Interface: **Go + Fyne.**

## Mental model

The manager lives at a **seam** between *remote metadata* (GitHub → latest
version, assets, checksums, description) and *local install state* (where a
binary lives, its current version, its daemon). Two ideas anchor everything:

- **shell** — the Fyne window is a *thin shell*. No logic, no I/O lives there;
  it only fetches rows from the core, renders cards, and calls the engine.
  If you're about to put behaviour in `internal/gui`, stop and put it in a core
  package behind a seam instead.
- **seam** — every side effect is injected (HTTP client, executor, daemon
  stopper, install root). Tests cross the same seam with fakes, so the engine
  is testable with no display and no network.

## Commands

Prefer the Makefile; raw `go build/test ./...` **will not link** Fyne without
the CGO flags that `make` exports (`deps` creates the local Xlib symlink).

- `make deps` — ensure `~/.local/lib/libXxf86vm.so` + fetch deps
- `make build` → `bin/app-store` · `make run` — build & open the desktop app
- `make test` — `go test ./...` · `make vet`
- `make inspect` — dump the widget-tree markup (Fyne test driver, displayless)
- `make gen` — regenerate `apps.json` from the embedded manifest
- `make winres` — regenerate Windows PE `.syso` into `cmd/app-store/`
- `make install` — `go install` (no Mark-of-the-Web) · `make version`

## Layout (deep modules)

Each core package is one **module** with a small interface:

- `internal/appdata` — **single source of truth** for the fleet: the embedded
  `manifest.yaml` + icons. Generator and app both read it; never keep a second
  manifest anywhere.
- `internal/registry` — GitHub API → release metadata; resolves per-platform
  assets from the GoReleaser name template and joins sha256 from
  `checksums.txt`. `ResolveApp` is the runtime path the shell calls.
- `internal/semver` — `Parse` + `Compare`.
- `internal/fleet` — `Decide(installed, latest)` → not-installed / up-to-date /
  upgrade-available; `SelectAsset` for the current OS/ARCH.
- `internal/store` — managed root + **system scan** (`~/go/bin`, GOBIN,
  PATH) for installed apps; `Destination` = where to swap an upgraded binary;
  installed-version detection via `<binary> version`/`--version`; `AtomicSwap`.
- `internal/updater` — the lifecycle: **stop daemon → download → verify sha256 →
  atomic swap → start daemon**; Update All; per-app progress callback.
- `internal/gui` — the `shell`.
- `cmd/manifest-gen` — batch generator (the future jsDelivr path).

## Reality checks (don't assume the fleet)

- Apps **self-daemonize** via `<binary> start` (manifest `start_args`) and write
  a `server.pid`. There are **no** `daemon stop/start` subcommands — stopping
  means terminating the process by name (`exec.ProcessStop`). Keep the daemon
  model reality-based.
- Version is exposed by `<binary> version` (also tries `--version`).
- MVP **bypasses jsDelivr**: the shell resolves via the GitHub API at runtime
  and downloads the plain `github.com/.../releases/download/...` URL. jsDelivr
  is deferred until the repo is public; don't reintroduce a CDN dependency into
  the app's runtime fetch.

## Tests

- Core tests use **fakes at the seams** (fake HTTP, recording executor, fake
  daemon stopper) — no network, no display.
- Set `Store.ScanSystem = false` in any test, or it will probe the **real**
  `~/go/bin` and break hermetic installs.
- `internal/gui` has automated widget-tree assertions (`make test`) — e.g.
  "not-installed shows Install but no Open", "Update-all hides when nothing is
  pending". Inspect failures with `make inspect` (markup dump).

A change is **done** when: `make test` is green, `make inspect` shows the
expected tree, and `make build` links. Then commit.

## Conventions & gotchas

- Ship through the Makefile to get the Fyne link flags; a raw `go build`
  fails on `-lXxf86vm` here.
- Galaxy rule: **single source of truth** for the fleet is
  `internal/appdata/manifest.yaml` — not a repo-root copy.
- `apps.json`, `assets/icons/*`, `.goreleaser.yaml`, `.github/workflows/*` are
  the jsDelivr/release surface; the app itself does **not** read `apps.json`.

## Releases

Release binaries are built by GoReleaser (`.goreleaser.yaml`) on a `git tag
v0.x.x && git push --tags`: winres PE metadata via `scripts/gen-winres.sh`,
`checksums.txt`, cross-platform archives. Note `.github/workflows/update-manifest.yml`
is **only the jsDelivr metadata generator** (regenerates `apps.json`); it does
not build binaries. First ship is `go install` — **no** winget/scoop/brew taps
(deferred).

## Repo & tracking

`github.com/udit-001/dock`. Work is tracked on Lific as project **APPS**
(spec = APPS-1, MVP plan = APPS-PLAN-1; deferred: jsDelivr, Cresto onboarding,
system-tray background checker + notifications = APPS-7).