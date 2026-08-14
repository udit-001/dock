package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/udit-001/dock/internal/fleet"
)

// lucide maps a logical icon name to a Fyne built-in themed icon. Fyne's theme
// icons always render and are recolored to the current theme automatically
// (they're the robust equivalent of the Lucide glyphs we want). Returns a safe
// fallback so call sites never receive nil.
func lucide(name string) fyne.Resource {
	switch name {
	case "refresh-cw":
		return theme.ViewRefreshIcon() // Check for updates / Update
	case "download":
		return theme.DownloadIcon() // Install / Update all
	case "external-link":
		return theme.VisibilityIcon() // Open
	case "circle-check":
		return theme.ConfirmIcon() // up to date
	case "circle-arrow-up":
		return theme.ViewRefreshIcon() // update available
	case "package":
		return theme.StorageIcon() // empty state
	default:
		return theme.ConfirmIcon()
	}
}

// luPrimaryIcon picks the icon for the primary Install/Update action button.
func luPrimaryIcon(s fleet.Status) string {
	switch s {
	case fleet.NotInstalled:
		return "download"
	case fleet.UpgradeAvailable:
		return "refresh-cw"
	}
	return "check"
}