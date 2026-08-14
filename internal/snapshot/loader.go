// Package snapshot loads the generated fleet snapshot (apps.json) and builds
// the per-app rows the GUI renders. It owns all network + cache + row-building
// logic behind one deep seam (Loader.Load) so the Fyne shell stays a thin
// renderer: fetch from jsDelivr → atomic write-through to a local cache →
// offline fallback to the cached copy.
package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/udit-001/dock/internal/catalog"
	"github.com/udit-001/dock/internal/fleet"
	"github.com/udit-001/dock/internal/store"
)

// Row is one app rendered by the shell: its manifest identity, resolved
// metadata, detected installed version, and decided status.
type Row struct {
	App       catalog.App
	Manifest  catalog.ManifestApp
	Installed string
	Status    fleet.Status
}

// Result is everything the shell needs to render the fleet in one shot:
// the rows plus snapshot freshness and whether the data came from cache.
type Result struct {
	Rows        []Row
	GeneratedAt time.Time
	Offline     bool
}

// Loader fetches, caches, and row-builds the fleet snapshot. Every seam is
// injectable (HTTP client, store, cache dir); a zero-value CacheDir defaults to
// os.UserConfigDir()/dock. baseURL is internal so tests can point at an httptest
// server; the runtime default is the jsDelivr URL derived from Man.Repo/Branch.
type Loader struct {
	St       *store.Manager
	Man      *catalog.Manifest
	HTTP     *http.Client
	CacheDir string

	baseURL string
}

// Load returns the fleet rows, fetching the fresh snapshot from jsDelivr and
// writing it through to the cache. When the fetch fails it falls back to the
// cached copy (Result.Offline = true); with no usable cache it returns an error
// ("no fleet metadata available") so the shell can render an empty state.
func (l *Loader) Load(ctx context.Context) (Result, error) {
	st, offline, err := l.loadStore(ctx)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Rows:        l.rowsFromStore(ctx, st),
		GeneratedAt: st.GeneratedAt,
		Offline:     offline,
	}, nil
}

// loadStore returns the snapshot, preferring the fresh jsDelivr copy (written
// through to the cache), then the cached copy on failure.
func (l *Loader) loadStore(ctx context.Context) (*catalog.Store, bool, error) {
	st, err := l.fetch(ctx)
	if err == nil {
		_ = l.writeCache(st) // write-through; cache failures are non-fatal
		return st, false, nil
	}
	if cached, cerr := l.readCache(); cerr == nil {
		return cached, true, nil
	}
	return nil, false, errors.New("no fleet metadata available")
}

// fetch downloads and decodes apps.json from jsDelivr (the repo hosting the
// manifest). The app never calls the GitHub API — the workflow-generated
// snapshot is the only runtime source.
func (l *Loader) fetch(ctx context.Context) (*catalog.Store, error) {
	cli := l.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.snapshotURL(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jsDelivr: %s", resp.Status)
	}
	var st catalog.Store
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

// snapshotURL is the jsDelivr URL for the fleet snapshot, or the injected test
// base URL when set.
func (l *Loader) snapshotURL() string {
	if l.baseURL != "" {
		return l.baseURL
	}
	branch := l.Man.Branch
	if branch == "" {
		branch = "main"
	}
	return fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s@%s/apps.json", l.Man.Repo, branch)
}

// readCache loads the cached snapshot, treating a missing or corrupt file as
// absent (the caller then errors — a stale-but-parseable cache is better than
// nothing, a garbage one is not trusted).
func (l *Loader) readCache() (*catalog.Store, error) {
	data, err := os.ReadFile(filepath.Join(l.cacheDir(), "apps.json"))
	if err != nil {
		return nil, err
	}
	var st catalog.Store
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// writeCache persists the snapshot atomically (temp + rename) so a crash or
// concurrent read never observes a half-written file.
func (l *Loader) writeCache(st *catalog.Store) error {
	dir := l.cacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "apps.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "apps.json"))
}

func (l *Loader) cacheDir() string {
	if l.CacheDir != "" {
		return l.CacheDir
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "dock")
	}
	return filepath.Join(os.TempDir(), "dock")
}

// rowsFromStore maps a snapshot's apps to rows (installed-version detection +
// status decision).
func (l *Loader) rowsFromStore(ctx context.Context, st *catalog.Store) []Row {
	rows := make([]Row, 0, len(st.Apps))
	for i := range st.Apps {
		a := &st.Apps[i]
		ma := catalog.ManifestApp{ID: a.ID, Binary: a.Binary, DisplayName: a.DisplayName}
		state, installed := l.St.InstalledVersion(ctx, ma)
		rows = append(rows, Row{
			App:       *a,
			Manifest:  ma,
			Installed: installed,
			Status:    fleet.Decide(state, installed, a.LatestVersion),
		})
	}
	return rows
}
