package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/udit-001/app-store/internal/exec"
	"github.com/udit-001/app-store/internal/registry"
	"github.com/udit-001/app-store/internal/store"
)

// fakeSrc writes static content to dest and reports progress.
type fakeSrc struct {
	content string
}

func (f *fakeSrc) Download(_ context.Context, _, dest string, prog Progress) error {
	if prog != nil {
		prog(0, int64(len(f.content)))
	}
	if err := os.WriteFile(dest, []byte(f.content), 0o755); err != nil {
		return err
	}
	if prog != nil {
		prog(int64(len(f.content)), int64(len(f.content)))
	}
	return nil
}

// recExec records commands.
type recExec struct {
	mu   sync.Mutex
	args [][]string
}

func (r *recExec) Run(_ context.Context, name string, args ...string) (string, error) {
	r.mu.Lock()
	r.args = append(r.args, append([]string{name}, args...))
	r.mu.Unlock()
	return "", nil
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func noopStop() func(string) error { return func(string) error { return nil } }

func daemonApp(binary, content, wantHash string) (registry.ManifestApp, *registry.App) {
	ma := registry.ManifestApp{ID: "pharos", Binary: binary}
	app := &registry.App{
		Daemon: &registry.DaemonOut{HasDaemon: true, StartArgs: []string{"start"}},
		Assets: map[string]registry.Asset{
			"linux/amd64": {URL: "http://x/download", FileName: binary, SHA256: wantHash},
		},
	}
	return ma, app
}

func newEngine(t *testing.T, re *recExec, src HTTPSrc, stopper exec.Stopper) (*Engine, *store.Manager) {
	t.Helper()
	root := t.TempDir()
	m, err := store.New(root, re)
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false // don't touch the real ~/go/bin during tests
	return &Engine{Store: m, Exec: re, HTTP: src, Stopper: stopper}, m
}

func TestInstallStopsAndStartsDaemonAroundSwap(t *testing.T) {
	re := &recExec{}
	var stopped []string
	stopper := func(name string) error { stopped = append(stopped, name); return nil }
	content := "BINARY"
	ma, app := daemonApp("pharos", content, hash(content))
	eng, m := newEngine(t, re, &fakeSrc{content: content}, stopper)

	if err := eng.Install(context.Background(), ma, app, nil); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// stop via process-stopper (by binary name), then start via `binary start`.
	if len(stopped) != 1 || stopped[0] != "pharos" {
		t.Fatalf("daemon stop should kill by binary name, got %v", stopped)
	}
	re.mu.Lock()
	cmd := strings.Join(re.args[0], " ")
	re.mu.Unlock()
	if !strings.HasSuffix(cmd, "start") {
		t.Errorf("start call should run `binary start`, got %q", cmd)
	}

	data, err := os.ReadFile(m.BinaryPath(ma))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("installed binary=%q want %q", data, content)
	}
}

func TestInstallAbortsOnChecksumMismatch(t *testing.T) {
	re := &recExec{}
	content := "GOOD"
	ma, app := daemonApp("pharos", content, hash("TAMPERED")) // wrong sha
	eng, m := newEngine(t, re, &fakeSrc{content: content}, noopStop())

	err := eng.Install(context.Background(), ma, app, nil)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if _, statErr := os.Stat(m.BinaryPath(ma)); !os.IsNotExist(statErr) {
		t.Fatal("binary must not be placed when checksum mismatches")
	}
}

func TestNoDaemonAppSkipsControl(t *testing.T) {
	re := &recExec{}
	content := "BIN"
	ma := registry.ManifestApp{ID: "sea", Binary: "sea"}
	app := &registry.App{Assets: map[string]registry.Asset{
		"linux/amd64": {FileName: "sea", URL: "http://x", SHA256: hash(content)},
	}}
	eng, _ := newEngine(t, re, &fakeSrc{content: content}, noopStop())
	if err := eng.Install(context.Background(), ma, app, nil); err != nil {
		t.Fatal(err)
	}
	re.mu.Lock()
	defer re.mu.Unlock()
	if len(re.args) != 0 {
		t.Fatalf("no daemon should mean no exec calls, got %v", re.args)
	}
}

func TestExplicitStopCommandUsed(t *testing.T) {
	re := &recExec{}
	content := "B"
	ma := registry.ManifestApp{ID: "svc", Binary: "svc"}
	app := &registry.App{
		Daemon: &registry.DaemonOut{HasDaemon: true, Stop: []string{"svc", "stop"}, StartArgs: []string{"start"}},
		Assets: map[string]registry.Asset{
			"linux/amd64": {FileName: "svc", URL: "http://x", SHA256: hash(content)},
		},
	}
	called := false
	stopper := func(name string) error { called = true; return nil } // should NOT be called
	eng, _ := newEngine(t, re, &fakeSrc{content: content}, stopper)
	if err := eng.Install(context.Background(), ma, app, nil); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("explicit stop command should supersede process-stopper")
	}
	re.mu.Lock()
	defer re.mu.Unlock()
	// stop + start => two exec calls
	if len(re.args) != 2 {
		t.Fatalf("expected stop + start exec calls, got %v", re.args)
	}
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := VerifySHA256(p, hash("x")); err != nil {
		t.Errorf("valid hash rejected: %v", err)
	}
	if err := VerifySHA256(p, hash("y")); err == nil {
		t.Error("invalid hash accepted")
	}
	if err := VerifySHA256(p, ""); err != nil {
		t.Errorf("empty want should pass: %v", err)
	}
}