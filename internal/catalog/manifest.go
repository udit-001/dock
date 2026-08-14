// Package catalog holds the fleet data model shared by the generator
// (internal/registry) and the desktop app (appdata, fleet, store, updater,
// gui): the hand-maintained manifest and the generated apps.json snapshot.
// It is deliberately dependency-light (only yaml) so every package can import
// the schema without pulling in the GitHub client.
package catalog

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Manifest is the root of manifest.yaml (the hand-maintained source of truth).
type Manifest struct {
	Branch string        `yaml:"branch"`
	Repo   string        `yaml:"repo"`
	Apps   []ManifestApp `yaml:"apps"`
}

// ManifestApp declares one app in the fleet. Only apps that publish GitHub
// releases and satisfy the version/daemon preconditions should be registered.
type ManifestApp struct {
	ID          string   `yaml:"id"`
	Repo        string   `yaml:"repo"`
	DisplayName string   `yaml:"display_name"`
	Description string   `yaml:"description"`
	Icon        string   `yaml:"icon"`
	Binary      string   `yaml:"binary"`
	OpenURL     string   `yaml:"open_url"`
	ReleaseName string   `yaml:"release_name"`
	Daemon      *Daemon  `yaml:"daemon"`
	Platforms   []string `yaml:"platforms"`
}

// Daemon holds the commands the manager uses to control a background service.
// `start_args` launch the daemon (argv after the binary, e.g. ["start"]). The
// apps self-daemonize and return. `stop` is an optional explicit stop command;
// when absent the manager stops the service by terminating its process(es).
type Daemon struct {
	StartArgs []string `yaml:"start_args"`
	Stop      []string `yaml:"stop"`
	Status    []string `yaml:"status"`
}

// ParseManifest parses manifest.yaml bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest.yaml: %w", err)
	}
	return &m, nil
}
