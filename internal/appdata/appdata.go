// Package appdata embeds the canonical fleet manifest and app icons so the
// desktop app works when built locally (go install) without a network path to
// the repo files. manifest.yaml lives here so generator + app share one source.
// The generated apps.json is NOT embedded — the app fetches it from jsDelivr
// and caches it locally (see internal/snapshot).
package appdata

import (
	"embed"

	"github.com/udit-001/dock/internal/catalog"
)

//go:embed manifest.yaml icons/*
var FS embed.FS

// LoadManifest returns the embedded fleet manifest.
func LoadManifest() (*catalog.Manifest, error) {
	data, err := FS.ReadFile("manifest.yaml")
	if err != nil {
		return nil, err
	}
	return catalog.ParseManifest(data)
}

// Icon returns the embedded icon PNG bytes for an app id ("" if absent).
func Icon(id string) ([]byte, bool) {
	b, err := FS.ReadFile("icons/" + id + ".png")
	return b, err == nil
}
