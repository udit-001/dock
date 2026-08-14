package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/udit-001/app-store/internal/exec"
	"github.com/udit-001/app-store/internal/registry"
)

func TestExtractVersion(t *testing.T) {
	cases := []struct{ in, want string }{
		{"pharos version v0.9.3", "v0.9.3"},
		{"v0.9.3", "v0.9.3"},
		{"dev build", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := extractVersion(c.in); got != c.want {
			t.Errorf("extractVersion(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestInstalledVersion(t *testing.T) {
	root := t.TempDir()
	m, err := New(root, exec.StaticExecutor{Out: "waypoint version v0.12.0"})
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false
	ma := registry.ManifestApp{ID: "waypoint", Binary: "waypoint"}
	// not installed yet
	if got := m.InstalledVersion(context.Background(), ma); got != "" {
		t.Fatalf("expected '' when not installed, got %q", got)
	}
	// fake an installed binary path
	path := m.BinaryPath(ma)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := m.InstalledVersion(context.Background(), ma); got != "v0.12.0" {
		t.Fatalf("InstalledVersion=%q want v0.12.0", got)
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
	ma := registry.ManifestApp{ID: "harbor", Binary: "harbor"}
	if err := os.WriteFile(filepath.Join(alt, "harbor"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := m.InstalledVersion(context.Background(), ma); got != "v0.2.0" {
		t.Fatalf("InstalledVersion(in go bin)=%q want v0.2.0", got)
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