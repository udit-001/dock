package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func writeTarGz(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		if err := tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(content)), Mode: 0o755}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.tar.gz")
	writeTarGz(t, src, map[string]string{
		"README.md":   "docs",
		"harbor":      "BIN",
		"docs/cli.md": "x",
	})
	got, err := Extract(src, filepath.Join(dir, "x"), "harbor")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(got)
	if string(data) != "BIN" {
		t.Fatalf("wrong binary content: %q", data)
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.zip")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("waypoint.exe")
	w.Write([]byte("WINBIN"))
	zw.CreateHeader(&zip.FileHeader{Name: "docs/cli.md"})
	zw.Close()
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Extract(src, filepath.Join(dir, "x"), "waypoint")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(got)
	if string(data) != "WINBIN" {
		t.Fatalf("wrong binary content: %q", data)
	}
}

func TestExtractMissing(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.tar.gz")
	writeTarGz(t, src, map[string]string{"README.md": "docs"})
	if _, err := Extract(src, filepath.Join(dir, "x"), "nope"); err == nil {
		t.Error("expected error when binary not in archive")
	}
}
