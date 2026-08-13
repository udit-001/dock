// Package appdata embeds the canonical fleet manifest and app icons so the
// desktop app works when built locally (go install) without a network path to
// the repo files. manifest.yaml lives here so generator + app share one source.
package appdata

import (
	"embed"

	"github.com/udit-001/app-store/internal/registry"
)

//go:embed manifest.yaml icons/*
var FS embed.FS

// LoadManifest returns the embedded fleet manifest.
func LoadManifest() (*registry.Manifest, error) {
	data, err := FS.ReadFile("manifest.yaml")
	if err != nil {
		return nil, err
	}
	return registry.ParseManifest(data)
}

// Icon returns the embedded icon PNG bytes for an app id ("" if absent).
func Icon(id string) ([]byte, bool) {
	b, err := FS.ReadFile("icons/" + id + ".png")
	return b, err == nil
}