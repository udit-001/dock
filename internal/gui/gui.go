// Package gui is the thin Fyne surface over the tested core (fetch, decide,
// install). State lives in the core packages; this only renders rows and wires
// button actions so the UI stays minimal for non-technical users.
package gui

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/udit-001/app-store/internal/appdata"
	"github.com/udit-001/app-store/internal/fleet"
	"github.com/udit-001/app-store/internal/registry"
	"github.com/udit-001/app-store/internal/store"
	"github.com/udit-001/app-store/internal/updater"
)

// row is one rendered app.
type row struct {
	ma        registry.ManifestApp
	app       *registry.App
	installed string
	status    fleet.Status
}

// Controller wires the core to the window.
type Controller struct {
	a      fyne.App
	win    fyne.Window
	man    *registry.Manifest
	st     *store.Manager
	cli    *registry.GHClient
	engine *updater.Engine

	listBox   *fyne.Container
	statusLbl *widget.Label
	errLbl    *widget.Label
	progress  *widget.ProgressBar
	actionLbl *widget.Label
	rows      []*row
}

// New builds a controller for the default manifest and managed root.
func New() (*Controller, error) {
	man, err := appdata.LoadManifest()
	if err != nil {
		return nil, fmt.Errorf("embedded manifest: %w", err)
	}
	st, err := store.New(managedRoot(), nil)
	if err != nil {
		return nil, err
	}
	engine := &updater.Engine{
		Store: st,
		Exec:  execOS{},
		HTTP:  updater.HTTPDownloader{},
	}
	return &Controller{
		man:       man,
		st:        st,
		cli:       registry.NewGHClient(man.Repo, man.Branch),
		engine:    engine,
		statusLbl: widget.NewLabelWithStyle("Ready", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		errLbl:    widget.NewLabel(""),
		progress:  widget.NewProgressBar(),
		actionLbl: widget.NewLabel(""),
		rows:      []*row{},
	}, nil
}

func managedRoot() string {
	base := "~/.appstore"
	if home, err := os.UserHomeDir(); err == nil {
		base = home + "/.appstore"
	}
	return base
}

// content builds the full window content (shared by the real app and the
// headless screenshot renderer).
func (c *Controller) content() fyne.CanvasObject {
	c.listBox = container.NewVBox()
	headBtns := container.NewHBox(
		widget.NewButton("Check for updates", c.goRefresh),
		widget.NewButton("Update all", c.goUpdateAll),
	)
	header := container.NewBorder(
		container.NewBorder(nil, nil, headBtns, layout.NewSpacer(), c.statusLbl),
		nil, nil, nil,
		container.NewVBox(c.errLbl, c.progress, c.actionLbl),
	)
	scroll := container.NewVScroll(c.listBox)
	return container.NewBorder(header, nil, nil, nil, scroll)
}

// Run opens the main window and blocks.
func (c *Controller) Run() {
	c.a = app.New()
	c.win = c.a.NewWindow("App Store")
	c.win.SetContent(c.content())
	c.win.SetPadded(true)
	c.win.Resize(fyne.NewSize(720, 600))
	c.goRefresh()
	c.win.ShowAndRun()
}

// Capture is intentionally omitted — the UI is viewed by running the real app
// (make run). Off-screen rendering is not shown to the user.

// goRefresh fetches metadata + installed versions and re-renders (async).
func (c *Controller) goRefresh() {
	c.setStatus("Checking for updates…")
	c.errLbl.SetText("")
	go func() {
		c.refreshSync()
		fyne.Do(func() {
			c.render()
			c.setStatus("Ready")
		})
	}()
}

// refreshSync resolves metadata + detected versions, computing per-app status,
// and stores rows. Errors are surfaced on the error label.
func (c *Controller) refreshSync() {
	var out []*row
	var errs []string
	for _, ma := range c.man.Apps {
		appMeta, err := c.cli.ResolveApp(ma)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ma.DisplayName, err))
			continue
		}
		installed := c.st.InstalledVersion(context.Background(), ma)
		out = append(out, &row{
			ma:        ma,
			app:       appMeta,
			installed: installed,
			status:    fleet.Decide(installed, appMeta.LatestVersion),
		})
	}
	c.rows = out
	if len(errs) > 0 {
		c.errLbl.SetText(fmt.Sprintf("%d app(s) could not be checked: %s", len(errs), errs[0]))
	}
}

func (c *Controller) render() {
	c.listBox.Objects = nil
	for _, r := range c.rows {
		c.listBox.Add(c.buildCard(r))
	}
	c.listBox.Refresh()
}

func (c *Controller) buildCard(r *row) *widget.Card {
	ic := canvas.NewImageFromImage(iconImage(r.ma.ID))
	ic.FillMode = canvas.ImageFillContain
	ic.SetMinSize(fyne.NewSize(48, 48))

	var actionLabel string
	switch r.status {
	case fleet.NotInstalled:
		actionLabel = "Not installed · latest " + r.app.LatestVersion
	case fleet.UpToDate:
		actionLabel = "Up to date · v" + trimV(r.installed)
	default:
		actionLabel = fmt.Sprintf("New version available · %s → %s", trimV(r.installed), r.app.LatestVersion)
	}
	title := widget.NewLabelWithStyle(r.app.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	desc := widget.NewLabel(r.app.Description)
	desc.Wrapping = fyne.TextWrapWord
	meta := widget.NewLabel(actionLabel)

	btn := widget.NewButton(actionButtonLabel(r.status), func() {
		if r.status == fleet.NotInstalled || r.status == fleet.UpgradeAvailable {
			c.goInstall(r)
		}
	})
	if r.status == fleet.UpToDate {
		btn.Disable()
	}
	openBtn := widget.NewButton("Open", func() { openURLged(r.app.Homepage) })

	// Row content: icon on the left, title/desc/status filling the middle.
	info := container.NewVBox(title, desc, meta)
	// Action buttons on the RIGHT edge so rows read left-to-right: icon, text,
	// then Install/Open.
	actions := container.NewVBox(btn, openBtn)
	body := container.NewBorder(nil, nil, nil, actions,
		container.NewHBox(ic, container.NewPadded(info)))
	return widget.NewCard("", "", body)
}

func (c *Controller) goInstall(r *row) {
	c.setStatus(fmt.Sprintf("Installing %s…", r.app.DisplayName))
	c.errLbl.SetText("")
	go func() {
		progress := func(done, total int64) {
			var p float64
			if total > 0 {
				p = float64(done) / float64(total)
			}
			fyne.Do(func() {
				c.progress.SetValue(p)
				c.actionLbl.SetText(fmt.Sprintf("Downloading %s…", r.ma.Binary))
			})
		}
		err := c.engine.Install(context.Background(), r.ma, r.app, progress)
		fyne.Do(func() {
			c.progress.SetValue(1)
			c.actionLbl.SetText("")
			if err != nil {
				c.errLbl.SetText(fmt.Sprintf("%s: %v", r.app.DisplayName, err))
			}
			c.setStatus("Ready")
			c.goRefresh()
		})
	}()
}

func (c *Controller) goUpdateAll() {
	c.setStatus("Updating all…")
	c.errLbl.SetText("")
	go func() {
		for _, r := range c.rows {
			if r.app == nil {
				continue
			}
			status := fleet.Decide(c.st.InstalledVersion(context.Background(), r.ma), r.app.LatestVersion)
			if status == fleet.NotInstalled || status == fleet.UpgradeAvailable {
				c.setStatus(fmt.Sprintf("Updating %s…", r.app.DisplayName))
				if err := c.engine.Install(context.Background(), r.ma, r.app, nil); err != nil {
					fyne.Do(func() { c.errLbl.SetText(fmt.Sprintf("update %s: %v", r.app.DisplayName, err)) })
				}
			}
		}
		fyne.Do(func() { c.actionLbl.SetText(""); c.setStatus("Ready"); c.goRefresh() })
	}()
}

func (c *Controller) setStatus(s string) {
	if c.a == nil {
		return
	}
	fyne.Do(func() { c.statusLbl.SetText(s) })
}

// --- small helpers ---

func trimV(v string) string {
	if len(v) > 1 && v[0] == 'v' {
		return v[1:]
	}
	return v
}

func actionButtonLabel(s fleet.Status) string {
	if s == fleet.NotInstalled {
		return "Install"
	}
	return "Update"
}

func iconImage(id string) image.Image {
	if b, ok := appdata.Icon(id); ok {
		if img, err := png.Decode(bytesReader(b)); err == nil {
			return img
		}
	}
	return image.NewRGBA(image.Rect(0, 0, 1, 1))
}

func openURLged(raw string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", raw)
	case "darwin":
		cmd = exec.Command("open", raw)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", raw)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}