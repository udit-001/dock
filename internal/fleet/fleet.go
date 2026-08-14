// Package fleet turns release metadata + detected state into the decisions the
// desktop app surfaces: which asset to download for the current platform and
// whether an update is available.
package fleet

import (
	"runtime"

	"github.com/udit-001/dock/internal/catalog"
	"github.com/udit-001/dock/internal/semver"
)

// Status is the install state of one app.
type Status int

const (
	NotInstalled Status = iota
	UpToDate
	UpgradeAvailable
	Unknown
)

// InstalledState is the typed outcome of probing for an installed binary —
// the compiler-enforced handshake between store and fleet (no magic string).
// Names are prefixed "State" so they don't collide with the Status enum's
// NotInstalled in the same package.
type InstalledState int

const (
	// StateNotInstalled means no binary was found.
	StateNotInstalled InstalledState = iota
	// StateInstalled means a binary was found with a parseable version.
	StateInstalled
	// StateVersionUnknown means a binary was found but its version string
	// couldn't be lexed — we know it's present, just not which version. Decide
	// maps it to Unknown (never "Install").
	StateVersionUnknown
)

// NeedsAction reports whether a row has work for "Update all": a fresh
// install, an available upgrade, or an unknown installed version.
func (s Status) NeedsAction() bool {
	switch s {
	case NotInstalled, UpgradeAvailable, Unknown:
		return true
	}
	return false
}

// PlatformKey returns the current os/arch key used in apps.json assets.
func PlatformKey() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// SelectAsset picks the downloadable asset for the current platform.
func SelectAsset(assets map[string]catalog.Asset) (catalog.Asset, bool) {
	a, ok := assets[PlatformKey()]
	return a, ok
}

// Decide computes the status of an app given its typed install state, the
// detected installed version ("" when not installed or unparseable), and its
// latest version from upstream.
func Decide(state InstalledState, installed, latest string) Status {
	switch state {
	case StateNotInstalled:
		return NotInstalled
	case StateVersionUnknown:
		return Unknown // present but version unknown → never show "Install"
	}
	lv, err := semver.Parse(latest)
	if err != nil {
		return UpToDate // can't tell; don't nag
	}
	iv, err := semver.Parse(installed)
	if err != nil {
		return UpToDate
	}
	if iv.Compare(lv) < 0 {
		return UpgradeAvailable
	}
	return UpToDate
}
