// Package registry is the generator adapter: it turns the hand-maintained
// fleet manifest (internal/catalog) into the generated apps.json served to the
// desktop app via jsDelivr. It owns the GitHub client and release resolution;
// the shared data model lives in internal/catalog.
package registry

import (
	"os"

	"github.com/udit-001/dock/internal/catalog"
)

// LoadManifest reads and parses manifest.yaml from path.
func LoadManifest(path string) (*catalog.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return catalog.ParseManifest(data)
}
