// Package fleet turns release metadata + detected state into the decisions the
// desktop app surfaces: which asset to download for the current platform and
// whether an update is available.
package fleet

import (
	"runtime"

	"github.com/udit-001/dock/internal/registry"
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

// VersionUnknown is the sentinel store.InstalledVersion returns when a binary
// is present but its version string can't be parsed — we know it's installed,
// just not which version. Decide maps it to Unknown (never "Install").
const VersionUnknown = "installed"

func (s Status) String() string {
	switch s {
	case UpToDate:
		return "Up to date"
	case UpgradeAvailable:
		return "Update available"
	case Unknown:
		return "Version unknown"
	default:
		return "Not installed"
	}
}

// PlatformKey returns the current os/arch key used in apps.json assets.
func PlatformKey() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// SelectAsset picks the downloadable asset for the current platform.
func SelectAsset(assets map[string]registry.Asset) (registry.Asset, bool) {
	a, ok := assets[PlatformKey()]
	return a, ok
}

// Decide computes the status of an app given its detected installed version
// ("" means not installed) and its latest version from upstream.
func Decide(installed, latest string) Status {
	if installed == "" {
		return NotInstalled
	}
	if installed == VersionUnknown {
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