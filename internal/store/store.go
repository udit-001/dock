// Package store owns the managed install directory, detects installed versions,
// and performs atomic binary replacement.
package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/udit-001/app-store/internal/exec"
	"github.com/udit-001/app-store/internal/registry"
)

// Manager locates and mutates the per-app install layout under a managed root.
type Manager struct {
	Root string
	Exec exec.Executor
}

// New returns a Manager rooted at root (created if missing).
func New(root string, ex exec.Executor) (*Manager, error) {
	if ex == nil {
		ex = exec.OSExecutor{}
	}
	m := &Manager{Root: root, Exec: ex}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return m, nil
}

// Dir returns the managed directory for an app.
func (m *Manager) Dir(id string) string {
	return filepath.Join(m.Root, id)
}

// BinaryPath returns the managed binary path for an app. On Windows a ".exe"
// suffix is appended to the bare binary name.
func (m *Manager) BinaryPath(ma registry.ManifestApp) string {
	name := ma.Binary
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	return filepath.Join(m.Dir(ma.ID), name)
}

var versionRe = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)

// InstalledVersion detects the installed version of a manifest app by running
// "<binary> version" (or --version). Returns "" if not installed or unreadable.
func (m *Manager) InstalledVersion(ctx context.Context, ma registry.ManifestApp) string {
	path := m.BinaryPath(ma)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	out, _ := m.Exec.Run(ctx, path, "version")
	if out == "" {
		out, _ = m.Exec.Run(ctx, path, "--version")
	}
	return extractVersion(out)
}

// extractVersion pulls the first semver-looking triple from CLI output.
func extractVersion(s string) string {
	if m := versionRe.FindStringSubmatch(s); m != nil {
		return "v" + m[1] + "." + m[2] + "." + m[3]
	}
	return ""
}

// AtomicSwap replaces dst with the fully-written src, keeping the previous
// binary intact on failure (write-temp-then-rename). On Windows the destination
// must not be running/locked; callers stop daemons first.
func AtomicSwap(src, dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := dst + ".new"
	if err := os.Rename(src, tmp); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("place binary: %w", err)
	}
	// Executable bit on unix.
	if runtime.GOOS != "windows" {
		_ = os.Chmod(dst, 0o755)
	}
	return nil
}