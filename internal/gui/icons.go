package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/udit-001/app-store/internal/appdata"
	"github.com/udit-001/app-store/internal/fleet"
)

// lucide returns a theme-aware resource for an embedded Lucide icon name. Fyne
// recolors the SVG to the current theme foreground, so the glyph adapts to the
// dark/light scheme (like LibAdwaita/Adwaita icons). Returns nil if absent.
func lucide(name string) fyne.Resource {
	if name == "" {
		return nil
	}
	b, ok := appdata.RawLucide(name)
	if !ok {
		return nil
	}
	return theme.NewThemedResource(fyne.NewStaticResource(name+".svg", b))
}

// statusIcon maps an install state to a recognition glyph (a surface affordance
// answering "what's going on with this app" at a glance).
func statusIcon(s fleet.Status) string {
	switch s {
	case fleet.NotInstalled:
		return "download"
	case fleet.UpToDate:
		return "circle-check"
	case fleet.UpgradeAvailable:
		return "circle-arrow-up"
	}
	return "check"
}

// luPrimaryIcon picks the icon for the primary Install/Update action button.
func luPrimaryIcon(s fleet.Status) string {
	switch s {
	case fleet.NotInstalled:
		return "download"
	case fleet.UpgradeAvailable:
		return "refresh-cw"
	}
	return "check" // up-to-date primary is disabled; harmless icon
}