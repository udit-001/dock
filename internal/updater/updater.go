// Package updater drives the install/upgrade lifecycle: download a release
// asset, verify its sha256, stage it, atomically swap it in, and (for daemon
// apps) stop then restart the service around the swap.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/udit-001/dock/internal/archive"
	"github.com/udit-001/dock/internal/catalog"
	"github.com/udit-001/dock/internal/exec"
	"github.com/udit-001/dock/internal/fleet"
	"github.com/udit-001/dock/internal/store"
)

// Progress reports download progress (done/total; total may be -1 if unknown).
type Progress func(done, total int64)

// HTTPSrc downloads with progress. Exported so callers can supply a fake.
type HTTPSrc interface {
	Download(ctx context.Context, url, dest string, progress Progress) error
}

// HTTPDownloader implements HTTPSrc using the default HTTP client.
type HTTPDownloader struct{}

func (d HTTPDownloader) Download(ctx context.Context, url, dest string, progress Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	total := resp.ContentLength
	if progress != nil {
		progress(0, total)
		defer progress(total, total)
	}
	buf := make([]byte, 64*1024)
	var done int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// VerifySHA256 hashes a file and compares it to the expected hex digest. An
// empty want skips the check (returns nil).
func VerifySHA256(path, want string) error {
	want = trimHex(want)
	if want == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("sha256 mismatch: got %s want %s", got[:12], want[:12])
	}
	return nil
}

func trimHex(s string) string {
	s = filepath.Base(s) // tolerate "hex  name" lines passed whole
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			continue
		}
		return s[:i]
	}
	return s
}

// Engine performs install/upgrade against the local store.
type Engine struct {
	Store *store.Manager
	Exec  exec.Executor
	HTTP  HTTPSrc
	// Stopper terminates a daemon by binary name when the app has no explicit
	// stop command. Defaults to exec.ProcessStop. Tests inject a fake.
	Stopper exec.Stopper
}

func (e *Engine) stopper() exec.Stopper {
	if e.Stopper != nil {
		return e.Stopper
	}
	return exec.ProcessStop
}

// Install ensures the latest release of app is installed, stopping/restarting a
// daemon when the app declares one. progress optionally receives staged bytes.
func (e *Engine) Install(ctx context.Context, ma catalog.ManifestApp, app *catalog.App, progress Progress) error {
	asset, ok := fleet.SelectAsset(app.Assets)
	if !ok {
		return fmt.Errorf("no asset for this platform (%s/%s)", runtime.GOOS, runtime.GOARCH)
	}
	if err := e.controlDaemon(ctx, ma, app, "stop"); err != nil {
		return fmt.Errorf("stop daemon: %w", err)
	}
	staged, cleanup, err := e.downloadAndStage(ctx, ma, app, asset, progress)
	if err != nil {
		return fmt.Errorf("stage release: %w", err)
	}
	defer cleanup()

	dst := e.Store.Destination(ma)
	if err := store.AtomicSwap(staged, dst); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	return e.controlDaemon(ctx, ma, app, "start")
}

// controlDaemon starts/stops the app's daemon around a swap. Start runs the
// managed binary with the manifest start_args (the fleet apps self-daemonize
// and return). Stop runs an explicit command if declared, else kills the
// daemon by binary name.
func (e *Engine) controlDaemon(ctx context.Context, ma catalog.ManifestApp, app *catalog.App, verb string) error {
	if app.Daemon == nil || !app.Daemon.HasDaemon {
		return nil
	}
	bin := e.Store.Destination(ma)
	switch verb {
	case "stop":
		if len(app.Daemon.Stop) > 0 {
			_, err := e.Exec.Run(ctx, bin, app.Daemon.Stop[1:]...)
			return err
		}
		return e.stopper()(ma.Binary)
	case "start":
		_, err := e.Exec.Run(ctx, bin, app.Daemon.StartArgs...)
		return err
	}
	return nil
}

// downloadAndStage fetches the asset (converting archives to a single binary),
// verifies its sha256, and returns the staged binary path + a cleanup func.
func (e *Engine) downloadAndStage(ctx context.Context, ma catalog.ManifestApp, app *catalog.App, asset catalog.Asset, progress Progress) (string, func(), error) {
	dir, err := os.MkdirTemp("", "dock-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	raw := filepath.Join(dir, "download")
	if err := e.HTTP.Download(ctx, asset.URL, raw, progress); err != nil {
		return "", cleanup, err
	}
	if err := VerifySHA256(raw, asset.SHA256); err != nil {
		return "", cleanup, err
	}

	ext := filepath.Ext(asset.FileName)
	if ext == ".exe" || asset.FileName == ma.Binary {
		// raw binary; stage as-is
		staged := filepath.Join(dir, "binary")
		if err := os.Rename(raw, staged); err != nil {
			return "", cleanup, err
		}
		return staged, cleanup, nil
	}
	if ext == ".zip" || strings.HasSuffix(asset.FileName, ".tar.gz") {
		staged, err := archive.Extract(raw, filepath.Join(dir, "x"), ma.Binary)
		if err != nil {
			return "", cleanup, err
		}
		return staged, cleanup, nil
	}
	return "", cleanup, fmt.Errorf("unhandled asset %q", asset.FileName)
}
