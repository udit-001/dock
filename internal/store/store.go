// Package store owns the managed install directory, detects installed versions,
// and performs atomic binary replacement.
package store

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/udit-001/dock/internal/exec"
	"github.com/udit-001/dock/internal/fleet"
	"github.com/udit-001/dock/internal/registry"
)

// BinRoot returns the OS-standard user bin directory the manager installs
// binaries into (on PATH on most Unix setups): Linux/macOS use ~/.local/bin
// (honouring XDG_BIN_HOME), Windows uses %LOCALAPPDATA%\Programs.
func BinRoot() string {
	switch runtime.GOOS {
	case "windows":
		if lp := os.Getenv("LOCALAPPDATA"); lp != "" {
			return filepath.Join(lp, "Programs")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "AppData", "Local", "Programs")
		}
	default:
		if x := os.Getenv("XDG_BIN_HOME"); x != "" {
			return x
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "bin")
		}
	}
	return ""
}

// Manager locates and mutates the per-app install layout under a managed root,
// and can also detect apps installed in the Go toolchain bin dir (~/go/bin).
type Manager struct {
	Root string
	Exec exec.Executor
	// SearchDirs are extra directories to probe for installed binaries (tests
	// inject these; GOBIN and GOPATH/bin are always searched).
	SearchDirs []string
	// ScanSystem enables probing GOBIN/GOPATH-bin and the PATH for installed
	// apps. Keep true at runtime; tests set false for hermetic installs.
	ScanSystem bool
}

// New returns a Manager rooted at root (created if missing).
func New(root string, ex exec.Executor) (*Manager, error) {
	if ex == nil {
		ex = exec.OSExecutor{}
	}
	m := &Manager{Root: root, Exec: ex, ScanSystem: true}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return m, nil
}

// BinaryPath returns the managed binary path for an app (flat, in the OS bin
// root — e.g. ~/.local/bin/pharos, not a per-app subfolder).
func (m *Manager) BinaryPath(ma registry.ManifestApp) string {
	return filepath.Join(m.Root, binaryName(ma))
}

// binaryName returns the platform-correct executable filename for a manifest app.
func binaryName(ma registry.ManifestApp) string {
	name := ma.Binary
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	return name
}

// Search locates an installed binary for the app, probing the managed bin root,
// then the Go toolchain bin directories (GOBIN, GOPATH/bin), then the PATH.
func (m *Manager) Search(ma registry.ManifestApp) (string, bool) {
	name := binaryName(ma)
	dirs := []string{m.Root}
	if m.ScanSystem {
		dirs = append(dirs, binDirs()...)
	}
	dirs = append(dirs, m.SearchDirs...)
	for _, dir := range dirs {
		p := filepath.Join(dir, name)
		if isFile(p) {
			return p, true
		}
	}
	if m.ScanSystem {
		if p, err := osexec.LookPath(name); err == nil {
			return p, true
		}
	}
	return "", false
}

// Destination returns where an upgraded binary should be written: the existing
// install location if the app is already installed (deep-located or Go bin), or
// the managed fresh-install path otherwise.
func (m *Manager) Destination(ma registry.ManifestApp) string {
	if p, ok := m.Search(ma); ok {
		return p
	}
	return m.BinaryPath(ma)
}

// binDirs returns the Go toolchain binary directories to search: GOBIN, then
// GOPATH/bin (falling back to ~/go/bin).
func binDirs() []string {
	var dirs []string
	if g := os.Getenv("GOBIN"); g != "" {
		dirs = append(dirs, g)
	}
	if gp := os.Getenv("GOPATH"); gp != "" {
		dirs = append(dirs, filepath.Join(filepath.SplitList(gp)[0], "bin"))
	}
	if len(dirs) == 0 {
		if home, err := os.UserHomeDir(); err == nil {
			dirs = append(dirs, filepath.Join(home, "go", "bin"))
		}
	}
	return dirs
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

var versionRe = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)

// InstalledVersion detects the installed version of a manifest app by running
// "<binary> version" (or --version) at its installed location (managed bin root,
// Go toolchain bin, or PATH). Returns "" if not installed, or fleet.VersionUnknown
// when the binary is present but its version string can't be parsed (so Decide
// never falls back to "Not installed → Install").
func (m *Manager) InstalledVersion(ctx context.Context, ma registry.ManifestApp) string {
	path, ok := m.Search(ma)
	if !ok {
		return ""
	}
	out, _ := m.Exec.Run(ctx, path, "version")
	if out == "" {
		out, _ = m.Exec.Run(ctx, path, "--version")
	}
	if v := extractVersion(out); v != "" {
		return v
	}
	return fleet.VersionUnknown
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