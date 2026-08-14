// Package appdata embeds the canonical fleet manifest and app icons so the
// desktop app works when built locally (go install) without a network path to
// the repo files. manifest.yaml lives here so generator + app share one source.
package appdata

import (
	"embed"

	"github.com/udit-001/app-store/internal/registry"
)

//go:embed manifest.yaml icons/* icons/lucide/* apps.json
var FS embed.FS

// LoadManifest returns the embedded fleet manifest.
func LoadManifest() (*registry.Manifest, error) {
	data, err := FS.ReadFile("manifest.yaml")
	if err != nil {
		return nil, err
	}
	return registry.ParseManifest(data)
}

// Fixture returns the embedded apps.json (generated fleet metadata) used as a
// deterministic, offline source for the -shot/-inspect design tooling.
func Fixture() ([]byte, error) {
	b, err := FS.ReadFile("apps.json")
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Icon returns the embedded icon PNG bytes for an app id ("" if absent).
func Icon(id string) ([]byte, bool) {
	b, err := FS.ReadFile("icons/" + id + ".png")
	return b, err == nil
}

// RawLucide returns the raw SVG bytes of an embedded Lucide icon (nil if absent).
func RawLucide(name string) ([]byte, bool) {
	b, err := FS.ReadFile("icons/lucide/" + name + ".svg")
	return b, err == nil
}