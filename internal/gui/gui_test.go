package gui

import (
	"os"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/udit-001/dock/internal/catalog"
	"github.com/udit-001/dock/internal/exec"
	"github.com/udit-001/dock/internal/fleet"
	"github.com/udit-001/dock/internal/store"
)

// newTestController returns a Controller over a temp store and an empty
// manifest (no network is touched).
func newTestController(t *testing.T) *Controller {
	t.Helper()
	m, err := store.New(t.TempDir(), exec.StaticExecutor{Out: ""})
	if err != nil {
		t.Fatal(err)
	}
	m.ScanSystem = false
	c := &Controller{
		st:        m,
		man:       &catalog.Manifest{},
		statusLbl: widget.NewLabel(""),
		errLbl:    widget.NewLabel(""),
		actionLbl: widget.NewLabel(""),
	}
	_ = c.content() // initialises c.listBox + c.updateBtn
	return c
}

func mkApp(installed, latest, name string) *row {
	return &row{
		ma:        catalog.ManifestApp{ID: name, Binary: name, DisplayName: name},
		app:       &catalog.App{DisplayName: name, Description: "A test app", Homepage: "https://github.com/x/" + name, LatestVersion: latest},
		installed: installed,
		status:    fleet.Decide(installed, latest),
	}
}

func markupOf(o fyne.CanvasObject) string {
	return test.RenderToMarkup(test.NewWindow(o).Canvas())
}

func TestCardNotInstalledShowsInstallNoOpen(t *testing.T) {
	c := newTestController(t)
	card := c.buildRow(mkApp("", "v0.9.3", "pharos"))
	m := markupOf(card)
	if !strings.Contains(m, "Install") {
		t.Errorf("Not-installed card should show Install; markup:\n%s", m)
	}
	if strings.Contains(m, "Launch") {
		t.Errorf("Not-installed card should NOT show Launch; markup:\n%s", m)
	}
}

func TestCardInstalledShowsOpen(t *testing.T) {
	c := newTestController(t)
	card := c.buildRow(mkApp("v0.9.3", "v0.9.3", "pharos"))
	m := markupOf(card)
	if !strings.Contains(m, "Launch") {
		t.Errorf("Installed (up-to-date) card should show Launch; markup:\n%s", m)
	}
}

func TestUpdateAllHiddenWhenNothingPending(t *testing.T) {
	c := newTestController(t)
	c.rows = []*row{
		mkApp("v0.9.3", "v0.9.3", "pharos"),
		mkApp("v0.2.0", "v0.2.0", "harbor"),
	}
	if c.hasPending() {
		t.Error("fully up-to-date rows should have no pending work")
	}
	c.render()
	if !c.updateBtn.Hidden {
		t.Error("Update all should be hidden when nothing is pending")
	}
}

func TestUpdateAllShownWhenPending(t *testing.T) {
	c := newTestController(t)
	c.rows = []*row{
		mkApp("v0.9.3", "v0.9.3", "pharos"),
		mkApp("", "v0.2.0", "harbor"), // not installed
	}
	if !c.hasPending() {
		t.Error("a not-installed app should count as pending")
	}
	c.render()
	if c.updateBtn.Hidden {
		t.Error("Update all should be visible when an app is pending")
	}
}

func TestEmptyStateShownWhenNoRows(t *testing.T) {
	c := newTestController(t)
	c.rows = nil
	c.render()
	m := markupOf(c.listBox)
	if !strings.Contains(m, "No apps yet") {
		t.Errorf("empty-state placeholder expected; markup:\n%s", m)
	}
}

func TestTitleShowsVersionInline(t *testing.T) {
	app := &catalog.App{DisplayName: "pharos", LatestVersion: "v0.9.3"}
	r := &row{installed: "v0.9.3", status: fleet.UpToDate, app: app}
	if got := titleVersion(r); got != "(v0.9.3)" {
		t.Errorf(`titleVersion should give "(v0.9.3)", got %q`, got)
	}
	noApp := &row{status: fleet.UpToDate, app: nil}
	if got := titleVersion(noApp); got != "" {
		t.Errorf("titleVersion with nil app should be empty, got %q", got)
	}
}

func TestStandaloneArgs(t *testing.T) {
	url := "http://127.0.0.1:9090"
	want := []string{"--app=" + url}
	for _, bin := range []string{"/usr/bin/helium", "/usr/bin/google-chrome", "/usr/bin/chromium"} {
		if got := standaloneArgs(url, bin); len(got) != 1 || got[0] != want[0] {
			t.Errorf("standaloneArgs(%q) = %q, want %q", bin, got, want)
		}
	}
}

func TestResolveStandalonePrefersHelium(t *testing.T) {
	dir := t.TempDir()
	mkBin := func(name string) {
		p := dir + "/" + name
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mkBin("helium")
	mkBin("google-chrome")
	t.Setenv("PATH", dir)
	if got := resolveStandalone(); got != dir+"/helium" {
		t.Errorf("expected helium first, got %q", got)
	}
}
