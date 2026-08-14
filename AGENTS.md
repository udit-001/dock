# Dock

A desktop manager for the self-hosted Go fleet (Pharos, Harbor, Waypoint): it
reads the published fleet **snapshot** (`apps.json` from jsDelivr), never the
GitHub API, and installs/updates each app — stopping and restarting the daemon
around a **sha256-verified atomic swap**.

Interface: **Go + Fyne.**

## Mental model

The manager sits at a **seam** between *remote metadata* (the published
snapshot: latest versions, assets, checksums, descriptions) and *local install
state* (where a binary lives, its current version, its daemon). Two ideas
anchor everything:

- **shell** — the Fyne window is a *thin shell*. No logic, no I/O lives there;
  it only fetches rows from the core, renders cards, and calls the engine. If
  you're about to put behaviour in `internal/gui`, stop and put it in a core
  package behind a seam instead.
- **seam** — every side effect is injected (HTTP client, executor, daemon
  stopper, install root). Tests cross the same seam with fakes, so the engine
  is testable with no display and no network.

## Data flow (who reads what)

- `internal/appdata/manifest.yaml` — the **hand-maintained fleet list** (name,
  description, repo, binary, platforms, daemon). Input to the generator only.
- `.github/workflows/update-manifest.yml` — reads the manifest, resolves each
  repo's **strict `releases/latest`**, writes both `apps.json` copies (repo
  root = jsDelivr + embedded fallback). Triggers: daily cron, manual, or a
  manifest edit.
- The **app** reads only `apps.json` (jsDelivr → embedded fixture fallback) and
  **never calls the GitHub API**. Downloads still use plain
  `github.com/.../releases/download` URLs.

## Commands

Prefer the Makefile; raw `go build/test ./...` **will not link** Fyne without
the CGO flags that `make` exports (`deps` creates the local Xlib symlink).

- `make deps` — ensure `~/.local/lib/libXxf86vm.so` + fetch deps
- `make build` → `bin/app-store` · `make run` — build & open the desktop app
- `make test` — `go test ./...` · `make vet`
- `make inspect` — dump the widget-tree markup (Fyne test driver, displayless)
- `make gen` — regenerate `apps.json` from the manifest
- `make winres` — regenerate Windows PE `.syso` into `cmd/app-store/`
- `make install` — `go install` (no Mark-of-the-Web) · `make version`

## Layout (deep modules)

Each core package is one **module** with a small interface:

- `internal/appdata` — the embedded manifest + icons; single source for the
  generator. Never keep a second manifest.
- `internal/registry` — the **generator's** GitHub client: release metadata,
  per-platform assets from the GoReleaser name template, sha256 from
  `checksums.txt`. `ResolveApp` is the generator path — not the app's.
- `internal/semver` — `Parse` + `Compare`.
- `internal/fleet` — `Decide(installed, latest)` → not-installed / up-to-date /
  upgrade-available / **version-unknown** (present but unparseable); `SelectAsset`.
- `internal/store` — managed root + **system scan** (`~/go/bin`, GOBIN, PATH)
  for installed apps; `Destination` = where to swap an upgraded binary;
  installed-version detection via `<binary> version`/`--version`; `AtomicSwap`.
- `internal/updater` — the lifecycle: **stop daemon → download → verify sha256 →
  atomic swap → start daemon**; Update All; per-app progress callback.
- `internal/gui` — the `shell`.
- `cmd/manifest-gen` — the generator the workflow runs.

## Reality checks (don't assume the fleet)

- Apps **self-daemonize** via `<binary> start` (manifest `start_args`) and write
  a `server.pid`. There are **no** `daemon stop/start` subcommands — stopping
  means terminating the process by name (`exec.ProcessStop`).
- Version is exposed by `<binary> version` (also tries `--version`).

## Tests

- Core tests use **fakes at the seams** (fake HTTP, recording executor, fake
  daemon stopper) — no network, no display.
- Set `Store.ScanSystem = false` in any test, or it will probe the **real**
  `~/go/bin` and break hermetic installs.
- `internal/gui` has automated widget-tree assertions (`make test`) — e.g.
  "not-installed shows Install but no Launch", "Update-all hides when nothing is
  pending". Inspect failures with `make inspect` (markup dump).

A change is **done** when: `make test` is green, `make inspect` shows the
expected tree, and `make build` links. Then commit.

## Conventions & gotchas

- Ship through the Makefile to get the Fyne link flags; a raw `go build`
  fails on `-lXxf86vm` here.
- Galaxy rule: the fleet list lives only in `internal/appdata/manifest.yaml`;
  `apps.json` is generated, never hand-edited.
- Builds carry the `migrated_fynedo` tag (Fyne threading model) via the Makefile.

## Releases

`.github/workflows/release.yml` builds native per-OS binaries (CGO enabled) on
`git tag v0.x.x`: winres PE metadata via `scripts/gen-winres.sh`, tar.gz
archives + `checksums.txt`. Fyne needs CGO, so no cross-compiling — each OS
builds on its own runner. First ship is `go install` — **no** winget/scoop/brew
taps (deferred).

## Repo & tracking

`github.com/udit-001/dock`. Work is tracked on Lific as project **APPS**
(spec = APPS-1, plan = APPS-PLAN-1; deferred: Cresto onboarding, system-tray
background checker + notifications = APPS-7).
