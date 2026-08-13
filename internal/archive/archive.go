// Package archive extracts release archives (tar.gz / zip) produced by
// GoReleaser and locates the binary inside them.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extract unpacks src into destDir and returns the path of the entry whose base
// name matches want (either "binary" or "binary.exe"). Returns an error if the
// binary is not found.
func Extract(src, destDir, want string) (string, error) {
	lower := strings.ToLower(src)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(src, destDir, want)
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(src, destDir, want)
	default:
		return "", fmt.Errorf("unsupported archive %q", src)
	}
}

func matches(name, want string) bool {
	base := filepath.Base(name)
	wantExe := want + ".exe"
	return base == want || strings.EqualFold(base, want) ||
		base == wantExe || strings.EqualFold(base, wantExe)
}

func extractTarGz(src, destDir, want string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var found string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if matches(hdr.Name, want) {
			out := filepath.Join(destDir, want)
			if err := writeFile(out, tr, os.FileMode(hdr.Mode&0o777)); err != nil {
				return "", err
			}
			found = out
		}
	}
	if found == "" {
		return "", fmt.Errorf("binary %q not found in %s", want, src)
	}
	return found, nil
}

func extractZip(src, destDir, want string) (string, error) {
	z, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer z.Close()
	var found string
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if matches(f.Name, want) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			out := filepath.Join(destDir, want)
			if err := writeFile(out, rc, 0o755); err != nil {
				rc.Close()
				return "", err
			}
			rc.Close()
			found = out
		}
	}
	if found == "" {
		return "", fmt.Errorf("binary %q not found in %s", want, src)
	}
	return found, nil
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}