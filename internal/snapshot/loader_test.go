package snapshot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/udit-001/dock/internal/catalog"
	"github.com/udit-001/dock/internal/exec"
	"github.com/udit-001/dock/internal/fleet"
	"github.com/udit-001/dock/internal/store"
)

// testStore builds a hermetic store (ScanSystem=false so it never probes the
// real ~/go/bin). ex can override what `version` returns.
func testStore(t *testing.T, ex exec.Executor) *store.Manager {
	t.Helper()
	m, err := store.New(t.TempDir(), ex)
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false
	return m
}

func manifest() *catalog.Manifest {
	return &catalog.Manifest{Repo: "udit-001/dock", Branch: "main"}
}

func snapshotJSON(t *testing.T, at time.Time, apps ...catalog.App) []byte {
	t.Helper()
	st := catalog.Store{GeneratedAt: at, Apps: apps}
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func app(id, latest string) catalog.App {
	return catalog.App{
		ID:            id,
		DisplayName:   id,
		Binary:        id,
		LatestVersion: latest,
	}
}

// jsDelivr returns an httptest server that serves the given body, failing once
// the returned setDown func is called.
func jsDelivr(t *testing.T, body []byte) (*httptest.Server, func()) {
	t.Helper()
	var mu sync.Mutex
	down := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if down {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Write(body)
	}))
	return srv, func() {
		mu.Lock()
		down = true
		mu.Unlock()
	}
}

func TestLoadFetchesAndWritesThroughCache(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	body := snapshotJSON(t, at, app("pharos", "v0.9.3"), app("harbor", "v0.2.0"))
	srv, _ := jsDelivr(t, body)
	defer srv.Close()

	cacheDir := t.TempDir()
	m := testStore(t, exec.StaticExecutor{Out: ""}) // nothing installed
	l := &Loader{
		St:       m,
		Man:      manifest(),
		HTTP:     srv.Client(),
		CacheDir: cacheDir,
		baseURL:  srv.URL,
	}

	res, err := l.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Offline {
		t.Error("fresh fetch must not be Offline")
	}
	if !res.GeneratedAt.Equal(at) {
		t.Errorf("GeneratedAt=%v want %v", res.GeneratedAt, at)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Rows))
	}
	for _, r := range res.Rows {
		if r.Status != fleet.NotInstalled || r.Installed != "" {
			t.Errorf("row %s: got status=%v installed=%q, want NotInstalled/''", r.App.ID, r.Status, r.Installed)
		}
	}

	cached, err := os.ReadFile(filepath.Join(cacheDir, "apps.json"))
	if err != nil {
		t.Fatalf("write-through cache missing: %v", err)
	}
	var st catalog.Store
	if err := json.Unmarshal(cached, &st); err != nil {
		t.Fatalf("cache unparseable: %v", err)
	}
	if len(st.Apps) != 2 {
		t.Fatalf("cache should hold 2 apps, got %d", len(st.Apps))
	}
}

func TestLoadOfflineFallsBackToCache(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	body := snapshotJSON(t, at, app("pharos", "v0.9.3"))
	srv, setDown := jsDelivr(t, body)
	defer srv.Close()

	cacheDir := t.TempDir()
	// Prime the cache with a first (online) load, then take the server down.
	m := testStore(t, exec.StaticExecutor{Out: ""})
	l := &Loader{St: m, Man: manifest(), HTTP: srv.Client(), CacheDir: cacheDir, baseURL: srv.URL}
	if _, err := l.Load(context.Background()); err != nil {
		t.Fatalf("prime Load: %v", err)
	}

	setDown()
	res, err := l.Load(context.Background())
	if err != nil {
		t.Fatalf("offline Load: %v", err)
	}
	if !res.Offline {
		t.Error("cache fallback must be Offline")
	}
	if len(res.Rows) != 1 || res.Rows[0].App.ID != "pharos" {
		t.Fatalf("offline rows wrong: %+v", res.Rows)
	}
}

func TestLoadCorruptCacheTreatedAsAbsent(t *testing.T) {
	srv, setDown := jsDelivr(t, []byte("{}"))
	defer srv.Close()
	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "apps.json"), []byte("not json{"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := testStore(t, exec.StaticExecutor{Out: ""})
	l := &Loader{St: m, Man: manifest(), HTTP: srv.Client(), CacheDir: cacheDir, baseURL: srv.URL}

	setDown()
	_, err := l.Load(context.Background())
	if err == nil {
		t.Fatal("expected error with corrupt cache + offline")
	}
	if !strings.Contains(err.Error(), "no fleet metadata available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadNoCacheOfflineErrors(t *testing.T) {
	srv, setDown := jsDelivr(t, []byte("{}"))
	defer srv.Close()
	cacheDir := t.TempDir() // empty — no cache primed
	m := testStore(t, exec.StaticExecutor{Out: ""})
	l := &Loader{St: m, Man: manifest(), HTTP: srv.Client(), CacheDir: cacheDir, baseURL: srv.URL}

	setDown()
	_, err := l.Load(context.Background())
	if err == nil {
		t.Fatal("expected error with no cache + offline")
	}
}

func TestLoadRowsCarryStateAndStatus(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	body := snapshotJSON(t, at,
		app("pharos", "v0.9.3"),
		app("harbor", "v0.2.0"),
	)
	srv, _ := jsDelivr(t, body)
	defer srv.Close()

	// pharos is installed at v0.9.3 (up-to-date); harbor is absent.
	m := testStore(t, exec.StaticExecutor{Out: "pharos version v0.9.3"})
	ma := catalog.ManifestApp{ID: "pharos", Binary: "pharos"}
	if err := os.MkdirAll(filepath.Dir(m.BinaryPath(ma)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.BinaryPath(ma), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	l := &Loader{St: m, Man: manifest(), HTTP: srv.Client(), CacheDir: t.TempDir(), baseURL: srv.URL}
	res, err := l.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, r := range res.Rows {
		switch r.App.ID {
		case "pharos":
			if r.Status != fleet.UpToDate || r.Installed != "v0.9.3" {
				t.Errorf("pharos row: status=%v installed=%q, want UpToDate/v0.9.3", r.Status, r.Installed)
			}
		case "harbor":
			if r.Status != fleet.NotInstalled || r.Installed != "" {
				t.Errorf("harbor row: status=%v installed=%q, want NotInstalled/''", r.Status, r.Installed)
			}
		}
	}
}

func TestLoadVersionUnknownStatus(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Second)
	body := snapshotJSON(t, at, app("mystery", "v0.9.3"))
	srv, _ := jsDelivr(t, body)
	defer srv.Close()

	// Binary present but version output unparseable → VersionUnknown → Unknown.
	m := testStore(t, exec.StaticExecutor{Out: "mystery version unknown build"})
	ma := catalog.ManifestApp{ID: "mystery", Binary: "mystery"}
	if err := os.MkdirAll(filepath.Dir(m.BinaryPath(ma)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.BinaryPath(ma), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	l := &Loader{St: m, Man: manifest(), HTTP: srv.Client(), CacheDir: t.TempDir(), baseURL: srv.URL}
	res, err := l.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0].Status != fleet.Unknown {
		t.Fatalf("expected Unknown status, got %+v", res.Rows)
	}
}
