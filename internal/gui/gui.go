// Package gui is the thin Fyne surface over the tested core (fetch, decide,
// install). State lives in the core packages; this only renders rows and wires
// button actions so the UI stays minimal for non-technical users.
package gui

import (
	"context"
	"encoding/json"
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
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
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
	updateBtn *widget.Button
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
	// Progress bar and the one-line action/error labels are hidden until a
	// status line is needed (avoids a permanent "0%" and a dead band).
	progress := widget.NewProgressBar()
	progress.Hide()
	errLbl := widget.NewLabel("")
	errLbl.Hide()
	actionLbl := widget.NewLabel("")
	actionLbl.Hide()
	return &Controller{
		man:       man,
		st:        st,
		cli:       registry.NewGHClient(man.Repo, man.Branch),
		engine:    engine,
		statusLbl: widget.NewLabelWithStyle("Ready", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		errLbl:    errLbl,
		progress:  progress,
		actionLbl: actionLbl,
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
	c.updateBtn = widget.NewButtonWithIcon("Update all", lucide("download"), c.goUpdateAll)
	c.updateBtn.Hide()
	// One compact top row: actions on the left, status right-aligned. The
	// progress bar + action/error labels sit beneath it but are hidden until
	// something happens, so they don't reserve a dead band above the list.
	top := container.NewHBox(
		widget.NewButtonWithIcon("Check for updates", lucide("refresh-cw"), c.goRefresh),
		c.updateBtn,
		layout.NewSpacer(),
		c.statusLbl,
	)
	header := container.NewVBox(top, c.progress, c.actionLbl, c.errLbl)
	scroll := container.NewVScroll(c.listBox)
	body := container.NewBorder(header, nil, nil, nil, scroll)
	// Cap the column width and center it (Bootstrap-container feel) so the list
	// doesn't sprawl across very wide windows.
	return container.New(cappedLayout{Max: contentWidth()}, body)
}

// contentWidth is the max centered column width on wide windows (Bootstrap:
// --bs-breakpoint-lg ≈ 960–992 with gutters, we shy under that).
func contentWidth() float32 { return 860 }

// Inspect renders the UI off-screen (Fyne software driver) and returns a
// Playwright-style snapshot of the widget tree as text markup. Used by the
// --inspect flag to debug layout without a display.
func (c *Controller) Inspect() string {
	content := c.content() // sets c.listBox + c.updateBtn
	rows, firstErr := c.populate()
	c.rows = rows
	c.showStatusLine(firstErr)
	c.render()             // fill the list before dumping
	win := test.NewWindow(content)
	win.Resize(windowSize())
	return test.RenderToMarkup(win.Canvas())
}

// windowSize is the target default window size, shared by the real app and the
// off-screen renderer so snapshots match production.
func windowSize() fyne.Size { return fyne.NewSize(980, 640) }

// RenderPNG renders the full window at its real size off-screen and writes it
// as a PNG. The image can be viewed to inspect visual layout.
func (c *Controller) RenderPNG(out string) error {
	a := test.NewApp()
	content := c.content()
	rows, firstErr := c.populate()
	c.rows = rows
	c.showStatusLine(firstErr)
	c.render()
	win := a.NewWindow("App Store")
	win.SetContent(content)
	win.Resize(windowSize())
	content.Resize(windowSize())
	img := software.RenderCanvas(win.Canvas(), theme.DarkTheme())
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// populate fills rows deterministically for the off-screen render tooling:
// it prefers the committed apps.json (real generated data, no network), falling
// back to the live GitHub API if the fixture is absent. The real app uses the
// live path (goRefresh → collect).
func (c *Controller) populate() ([]*row, string) {
	if rows, err := c.seedFixture(); err == nil {
		return rows, ""
	}
	return c.collect()
}

// seedFixture loads the embedded apps.json (the offline deterministic fixture)
// and maps it to rows, combining each app's metadata with installed-version
// detection. Falls back to a repo-root apps.json on disk if present.
func (c *Controller) seedFixture() ([]*row, error) {
	var data []byte
	var err error
	if data, err = appdata.Fixture(); err != nil {
		data, err = os.ReadFile("apps.json")
	}
	if err != nil {
		return nil, err
	}
	var st registry.Store
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	rows := make([]*row, 0, len(st.Apps))
	for i := range st.Apps {
		a := &st.Apps[i]
		ma := registry.ManifestApp{ID: a.ID, Binary: a.Binary, DisplayName: a.DisplayName}
		installed := c.st.InstalledVersion(context.Background(), ma)
		rows = append(rows, &row{
			ma:        ma,
			app:       a,
			installed: installed,
			status:    fleet.Decide(installed, a.LatestVersion),
		})
	}
	return rows, nil
}

// Run opens the main window and blocks.
func (c *Controller) Run() {
	c.a = app.New()
	c.win = c.a.NewWindow("App Store")
	c.win.SetContent(c.content())
	c.win.SetPadded(true)
	// Wider window gives app descriptions more room to breathe.
	c.win.Resize(windowSize())
	c.goRefresh()
	c.win.ShowAndRun()
}

// Capture is intentionally omitted — the UI is viewed by running the real app
// (make run). Off-screen rendering is not shown to the user.

// goRefresh fetches metadata + installed versions and re-renders (async).
func (c *Controller) goRefresh() {
	c.setStatus("Checking for updates…")
	go func() {
		rows, firstErr := c.collect()
		fyne.Do(func() {
			c.rows = rows
			c.render()
			c.setStatus("Ready")
			c.showStatusLine(firstErr)
		})
	}()
}

// collect resolves metadata + detected versions into rows, returning the first
// (rendering-free) error text. If GitHub is unreachable or rate-limited, it
// falls back to the embedded apps.json fixture so the app always lists the
// fleet with last-known versions rather than breaking.
func (c *Controller) collect() ([]*row, string) {
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
	if len(errs) == 0 {
		return out, ""
	}
	// GitHub unavailable (e.g. 403 rate limit): degrade to last-known data.
	if fr, ferr := c.seedFixture(); ferr == nil {
		return fr, "Offline — showing last known versions (GitHub unreachable)"
	}
	return out, fmt.Sprintf("%d app(s) could not be checked: %s", len(errs), errs[0])
}

// showStatusLine shows the one-line error/status label only when there is a
// message to convey, hiding it otherwise so it leaves no dead band.
func (c *Controller) showStatusLine(msg string) {
	if msg == "" {
		c.errLbl.Hide()
		return
	}
	c.errLbl.SetText(msg)
	c.errLbl.Show()
}

func (c *Controller) render() {
	c.listBox.Objects = nil
	if len(c.rows) == 0 {
		c.listBox.Add(c.emptyState())
	} else {
		for i, r := range c.rows {
			if i > 0 {
				c.listBox.Add(widget.NewSeparator())
			}
			c.listBox.Add(c.buildRow(r))
		}
	}
	c.listBox.Refresh()
	// Update-all is only useful when something is pending (not installed or
	// an upgrade is available).
	if c.hasPending() {
		c.updateBtn.Show()
	} else {
		c.updateBtn.Hide()
	}
	c.updateBtn.Refresh()
}

// hasPending reports whether any row has work for Update all (a fresh install
// or an available upgrade).
func (c *Controller) hasPending() bool {
	for _, r := range c.rows {
		if r.status == fleet.NotInstalled || r.status == fleet.UpgradeAvailable {
			return true
		}
	}
	return false
}

func (c *Controller) buildRow(r *row) fyne.CanvasObject {
	// Square, non-distorted icon drawn from the embedded resource: FillMode
	// contain keeps the 512px art scaled to fit a 40px rail.
	var iconObj fyne.CanvasObject
	if b, ok := appdata.Icon(r.ma.ID); ok && len(b) > 0 {
		im := canvas.NewImageFromResource(fyne.NewStaticResource("icon_"+r.ma.ID+".png", b))
		im.FillMode = canvas.ImageFillContain
		im.SetMinSize(fyne.NewSize(40, 40))
		iconObj = container.NewCenter(im) // vertical-center the icon in its rail
	} else {
		iconObj = canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}

	// Gear-Lever-style info column: heading name, one-line description,
	// then a muted "version · size" subtitle.
	title := widget.NewLabelWithStyle(r.app.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	desc := widget.NewLabel(r.app.Description)
	desc.Wrapping = fyne.TextWrapOff
	desc.Truncation = fyne.TextTruncateEllipsis
	meta := widget.NewLabelWithStyle(versionLine(r), fyne.TextAlignLeading, fyne.TextStyle{})

	// Install/Update is the primary (accent) action; Open is flat/secondary.
	btn := widget.NewButtonWithIcon(actionButtonLabel(r.status), lucide(luPrimaryIcon(r.status)), func() {
		if r.status == fleet.NotInstalled || r.status == fleet.UpgradeAvailable {
			c.goInstall(r)
		}
	})
	btn.Importance = widget.HighImportance
	if r.status == fleet.UpToDate {
		btn.Disable()
	}

	// Row content: icon on the left, info filling the middle, actions on the
	// right {"Install/Update" + "Open" when installed} — all vertically centered.
	// A state glyph sits beside the version line for at-a-glance recognition.
	statusBadge := widget.NewIcon(lucide(statusIcon(r.status)))
	metaRow := container.NewHBox(statusBadge, meta)
	info := container.NewVBox(title, desc, metaRow)
	inner := container.NewBorder(nil, nil, iconObj, nil, container.NewPadded(info))
	if r.status == fleet.NotInstalled {
		actions := container.NewCenter(container.NewVBox(btn))
		body := container.NewBorder(nil, nil, nil, actions, inner)
		return container.NewPadded(body)
	}
	openBtn := widget.NewButtonWithIcon("Open", lucide("external-link"), func() { openURLged(r.app.Homepage) })
	openBtn.Importance = widget.LowImportance
	actions := container.NewCenter(container.NewVBox(btn, openBtn))
	body := container.NewBorder(nil, nil, nil, actions, inner)
	return container.NewPadded(body)
}

// versionLine renders the muted subtitle under each app, following Gear Lever's
// "version · size" pattern.
func versionLine(r *row) string {
	var v string
	switch r.status {
	case fleet.NotInstalled:
		v = "Not installed · latest " + r.app.LatestVersion
	case fleet.UpToDate:
		v = "Installed " + r.app.LatestVersion
	default:
		v = fmt.Sprintf("%s → %s", trimV(r.installed), r.app.LatestVersion)
	}
	if sz, ok := sizeFor(r.app); ok {
		v += " · " + humanSize(sz)
	}
	return v
}

// sizeFor returns the download size (bytes) of the current-platform asset.
func sizeFor(app *registry.App) (int64, bool) {
	a, ok := app.Assets[fleet.PlatformKey()]
	if !ok {
		return 0, false
	}
	return a.Size, true
}

// humanSize formats a byte count compactly.
func humanSize(n int64) string {
	const kb = 1024
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// emptyState is the centered placeholder shown when the fleet is empty, mirroring
// Gear Lever's empty-list view.
func (c *Controller) emptyState() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("No apps yet", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	hint := widget.NewLabelWithStyle("Add an app to the fleet manifest and it will appear here.",
		fyne.TextAlignCenter, fyne.TextStyle{})
	hint.Wrapping = fyne.TextWrapWord
	return container.NewCenter(container.NewVBox(
		widget.NewIcon(lucide("package")),
		title,
		hint,
	))
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
				c.progress.Show()
				c.progress.SetValue(p)
				c.actionLbl.SetText(fmt.Sprintf("Downloading %s…", r.ma.Binary))
			})
		}
		err := c.engine.Install(context.Background(), r.ma, r.app, progress)
		fyne.Do(func() {
			c.progress.Hide()
			c.progress.SetValue(0)
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