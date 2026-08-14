package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/udit-001/dock/internal/catalog"
	"github.com/udit-001/dock/internal/exec"
	"github.com/udit-001/dock/internal/fleet"
)

func TestInstalledVersion(t *testing.T) {
	root := t.TempDir()
	m, err := New(root, exec.StaticExecutor{Out: "waypoint version v0.12.0"})
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false
	ma := catalog.ManifestApp{ID: "waypoint", Binary: "waypoint"}
	// not installed yet
	if state, got := m.InstalledVersion(context.Background(), ma); state != fleet.StateNotInstalled || got != "" {
		t.Fatalf("expected (NotInstalled, '') when not installed, got (%v, %q)", state, got)
	}
	// fake an installed binary path
	path := m.BinaryPath(ma)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if state, got := m.InstalledVersion(context.Background(), ma); state != fleet.StateInstalled || got != "v0.12.0" {
		t.Fatalf("InstalledVersion=%v,%q want (Installed, v0.12.0)", state, got)
	}
}

func TestInstalledVersionUnparseableIsInstalled(t *testing.T) {
	// A present binary whose version output we can't parse must read as installed
	// (never "Not installed → Install").
	m, err := New(t.TempDir(), exec.StaticExecutor{Out: "some unrecognizable output"})
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false
	ma := catalog.ManifestApp{ID: "app", Binary: "app"}
	if err := os.MkdirAll(filepath.Dir(m.BinaryPath(ma)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(m.BinaryPath(ma), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	state, got := m.InstalledVersion(context.Background(), ma)
	if state != fleet.StateVersionUnknown || got != "" {
		t.Fatalf("InstalledVersion=%v,%q want (VersionUnknown, '')", state, got)
	}
	if fleet.Decide(state, got, "v0.9.3") != fleet.Unknown {
		t.Fatal("present-but-unparseable must be Unknown version, not Not installed")
	}
}

func TestInstalledVersionInGoBin(t *testing.T) {
	// Detect an app installed in the Go toolchain bin dir (e.g. ~/go/bin).
	alt := t.TempDir()
	m, err := New(t.TempDir(), exec.StaticExecutor{Out: "harbor version v0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false
	m.SearchDirs = []string{alt}
	ma := catalog.ManifestApp{ID: "harbor", Binary: "harbor"}
	if err := os.WriteFile(filepath.Join(alt, "harbor"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if state, got := m.InstalledVersion(context.Background(), ma); state != fleet.StateInstalled || got != "v0.2.0" {
		t.Fatalf("InstalledVersion(in go bin)=%v,%q want (Installed, v0.2.0)", state, got)
	}
	// Destination should point at the detected Go-bin location, not the managed root.
	dst := m.Destination(ma)
	if filepath.Dir(dst) != alt {
		t.Fatalf("Destination should be in detected Go bin dir %q, got %q", alt, dst)
	}
}

func TestAtomicSwapPreservesOnFailure(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "new.bin")
	dst := filepath.Join(root, "app", "app.bin")
	if err := os.WriteFile(src, []byte("NEW"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AtomicSwap(src, dst); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "NEW" {
		t.Fatalf("dst=%q want NEW", data)
	}
	_ = runtime.GOOS
}
