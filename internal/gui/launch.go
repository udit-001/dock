package gui

import (
	"os/exec"
)

// browserCandidates is the preference order for opening a dashboard in a
// standalone (app-mode) window. Only Chromium-family browsers support this
// cleanly; Helium is first because it's the installed default here and its
// whole point is opening a site in its own window.
var browserCandidates = []string{
	"helium",
	"google-chrome",
	"chromium",
	"brave-browser",
	"microsoft-edge",
	"microsoft-edge-stable",
}

// resolveStandalone returns the absolute path of the first candidate found on
// PATH, or "" if none (callers fall back to the default browser).
func resolveStandalone() string {
	for _, name := range browserCandidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// standaloneArgs builds the CLI args that open url in a standalone window.
// All candidates are Chromium-family and support --app=<url> (Helium included).
func standaloneArgs(url, bin string) []string {
	return []string{"--app=" + url}
}

// openStandalone launches url in a standalone browser window, reporting whether
// it succeeded so callers can fall back to the default browser.
func openStandalone(url string) bool {
	bin := resolveStandalone()
	if bin == "" {
		return false
	}
	cmd := exec.Command(bin, standaloneArgs(url, bin)...)
	if err := cmd.Start(); err != nil {
		return false
	}
	return true
}

// openStandaloneOrDefault opens url in a standalone window when a Chromium-
// family browser is available, otherwise in the default browser.
func openStandaloneOrDefault(url string) {
	if !openStandalone(url) {
		openURLged(url)
	}
}