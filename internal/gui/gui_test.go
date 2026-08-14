package gui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/udit-001/app-store/internal/exec"
	"github.com/udit-001/app-store/internal/fleet"
	"github.com/udit-001/app-store/internal/registry"
	"github.com/udit-001/app-store/internal/store"
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
		man:       &registry.Manifest{},
		statusLbl: widget.NewLabel(""),
		errLbl:    widget.NewLabel(""),
		actionLbl: widget.NewLabel(""),
	}
	_ = c.content() // initialises c.listBox + c.updateBtn
	return c
}

func mkApp(installed, latest, name string) *row {
	return &row{
		ma:        registry.ManifestApp{ID: name, Binary: name, DisplayName: name},
		app:       &registry.App{DisplayName: name, Description: "A test app", Homepage: "https://github.com/x/" + name, LatestVersion: latest},
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
	if strings.Contains(m, "Open") {
		t.Errorf("Not-installed card should NOT show Open; markup:\n%s", m)
	}
}

func TestCardInstalledShowsOpen(t *testing.T) {
	c := newTestController(t)
	card := c.buildRow(mkApp("v0.9.3", "v0.9.3", "pharos"))
	m := markupOf(card)
	if !strings.Contains(m, "Open") {
		t.Errorf("Installed (up-to-date) card should show Open; markup:\n%s", m)
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

func TestVersionLineIncludesSize(t *testing.T) {
	app := &registry.App{
		DisplayName: "pharos", LatestVersion: "v0.9.3",
		Assets: map[string]registry.Asset{fleet.PlatformKey(): {Size: 15 * 1024 * 1024}},
	}
	r := &row{installed: "v0.9.3", status: fleet.UpToDate, app: app}
	line := versionLine(r)
	if !strings.Contains(line, "Installed v0.9.3") || !strings.Contains(line, "15.0 MB") {
		t.Errorf("version+size subtitle expected, got %q", line)
	}
}