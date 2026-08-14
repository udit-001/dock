// Package gui is the thin Fyne surface over the tested core (fetch, decide,
// install). State lives in the core packages; this only renders rows and wires
// button actions so the UI stays minimal for non-technical users.
package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/udit-001/dock/internal/appdata"
	"github.com/udit-001/dock/internal/fleet"
	"github.com/udit-001/dock/internal/registry"
	"github.com/udit-001/dock/internal/store"
	"github.com/udit-001/dock/internal/updater"
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
	engine *updater.Engine

	listBox   *fyne.Container
	scroll    *container.Scroll
	updateBtn *widget.Button
	checkBtn  *widget.Button
	checking  bool
	statusLbl *widget.Label
	updatedLbl *widget.Label
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
	st, err := store.New(store.BinRoot(), nil)
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
		engine:    engine,
		statusLbl: widget.NewLabelWithStyle("Ready", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		errLbl:    errLbl,
		progress:  progress,
		actionLbl: actionLbl,
		rows:      []*row{},
	}, nil
}

// content builds the full window content (shared by the real app and the
// headless screenshot renderer).
func (c *Controller) content() fyne.CanvasObject {
	c.listBox = container.NewVBox()
	c.updateBtn = widget.NewButtonWithIcon("Update all", lucide("download"), c.goUpdateAll)
	c.updateBtn.Hide()
	// Slim header bar (Gear-Lever style): title on the left, actions on the
	// right. The status label is hidden when idle; progress + action/error lines
	// sit beneath and only appear when something happens.
	title := widget.NewLabelWithStyle("Dock", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	// Freshness line: "Updated <time>" from apps.json.generated_at, hidden when
	// we don't have a snapshot timestamp.
	c.updatedLbl = widget.NewLabel("")
	c.updatedLbl.Hide()
	c.checkBtn = widget.NewButtonWithIcon("Check for updates", lucide("refresh-cw"), c.goRefresh)
	// The checking state keeps the same button + refresh icon, only disabled
	// with a "Checking…" label (no separate spinner). Reserve the widest of the
	// two labels so the header never shifts when it swaps.
	busyW := widget.NewLabel("Checking…").MinSize().Width
	idleW := c.checkBtn.MinSize().Width
	spotW := busyW
	if idleW > spotW {
		spotW = idleW
	}
	right := container.NewHBox(c.updatedLbl, c.statusLbl, c.updateBtn,
		container.New(fixedWidthLayout{Width: spotW + 2}, c.checkBtn))
	headerRow := container.NewBorder(nil, nil, title, right, nil)
	header := container.NewVBox(headerRow, c.progress, c.actionLbl, c.errLbl)
	// Framed list: the whole list sits on a contrasting rounded panel (like Gear
	// Lever's LibAdwaita ListBox) that stands out from the window background.
	// The panel is a fixed surface; the rows scroll over it.
	// The list sits directly on the window background (no separate container
	// slab), so text contrast is always against the window in both themes and
	// there is no dark-panel-on-light-theme mismatch. Rows use hover highlights
	// and dividers for structure.
	scroll := container.NewVScroll(c.listBox)
	c.scroll = scroll
	panel := container.NewPadded(scroll)
	// Breathing room: pad the header so there's a gap above it and between it
	// and the list, while the Border keeps the list filling the remaining space.
	headOuter := padTop(container.NewPadded(header), 16)
	// Fit the panel to its content and center it, rather than letting it stretch
	// to fill the whole window — a short fleet shouldn't leave a big empty band.
	body := container.New(panelFitLayout{}, headOuter, panel)
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
	a.Settings().SetTheme(newNordTheme()) // keep surface colors coherent with the render
	content := c.content()
	rows, firstErr := c.populate()
	c.rows = rows
	c.showStatusLine(firstErr)
	c.render()
	win := a.NewWindow("Dock")
	win.SetContent(content)
	win.Resize(windowSize())
	content.Resize(windowSize())
	img := software.RenderCanvas(win.Canvas(), a.Settings().Theme())
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// populate fills rows deterministically for the off-screen render tooling from
// the embedded apps.json (real generated data, no network).
func (c *Controller) populate() ([]*row, string) {
	st, err := fixtureStore()
	if err != nil {
		return nil, err.Error()
	}
	c.setUpdated(st.GeneratedAt)
	return c.rowsFromStore(st), ""
}

// loadSnapshot returns the fleet Store, preferring the fresh jsDelivr copy,
// then the embedded fixture / on-disk apps.json. The message is non-empty when
// we had to fall back (offline).
func (c *Controller) loadSnapshot() (*registry.Store, string) {
	if st, err := c.fetchSnapshot(); err == nil {
		return st, ""
	}
	if st, err := fixtureStore(); err == nil {
		return st, "Offline — showing last known versions"
	}
	return nil, "No fleet metadata available"
}

// fetchSnapshot downloads apps.json from jsDelivr (the repo hosting the
// manifest). The app never calls the GitHub API — the workflow-generated
// snapshot is the only runtime source.
func (c *Controller) fetchSnapshot() (*registry.Store, error) {
	url := fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s@%s/apps.json", c.man.Repo, c.man.Branch)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jsDelivr: %s", resp.Status)
	}
	var st registry.Store
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, err
	}
	return &st, nil
}

// fixtureStore loads the embedded apps.json (the offline deterministic
// snapshot), falling back to a repo-root apps.json on disk.
func fixtureStore() (*registry.Store, error) {
	data, err := appdata.Fixture()
	if err != nil {
		data, err = os.ReadFile("apps.json")
	}
	if err != nil {
		return nil, err
	}
	var st registry.Store
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// rowsFromStore maps a snapshot's apps to rows (installed-version detection +
// status decision).
func (c *Controller) rowsFromStore(st *registry.Store) []*row {
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
	return rows
}

// setUpdated shows (or hides) the "Updated <time>" freshness line.
func (c *Controller) setUpdated(t time.Time) {
	if c.updatedLbl == nil {
		return
	}
	if t.IsZero() {
		c.updatedLbl.Hide()
		return
	}
	c.updatedLbl.SetText("Updated " + t.Local().Format("Jan 2, 15:04"))
	c.updatedLbl.Show()
}

// Run opens the main window and blocks.
func (c *Controller) Run() {
	c.a = app.NewWithID("github.udit-001.app-store")
	c.a.Settings().SetTheme(newNordTheme()) // Nord palette, matching the fleet
	c.win = c.a.NewWindow("Dock")
	c.win.SetContent(c.content())
	c.win.SetPadded(true)
	// Size to the screen resolution (not fullscreen) and center it.
	c.win.Resize(screenSize())
	c.win.CenterOnScreen()
	c.goRefresh()
	c.win.ShowAndRun()
}

// Capture is intentionally omitted — the UI is viewed by running the real app
// (make run). Off-screen rendering is not shown to the user.

// goRefresh fetches metadata + installed versions and re-renders (async).
// Single-flight: while one check is running, further calls are ignored so
// concurrent fetches can't race on the final render.
func (c *Controller) goRefresh() {
	if c.checking {
		return
	}
	c.checking = true
	c.setChecking(true)
	go func() {
		rows, firstErr := c.collect()
		fyne.Do(func() {
			c.checking = false
			c.setChecking(false)
			c.rows = rows
			c.render()
			c.setStatus("")
			c.showStatusLine(firstErr)
		})
	}()
}

// setChecking flips the "Check for updates" button into its busy state: same
// button + refresh icon, disabled, labelled "Checking…". Only called from the
// main thread (button click, startup, or inside a fyne.Do block).
func (c *Controller) setChecking(on bool) {
	if c.checkBtn == nil {
		return
	}
	if on {
		c.checkBtn.SetText("Checking…")
		c.checkBtn.Disable()
	} else {
		c.checkBtn.SetText("Check for updates")
		c.checkBtn.Enable()
	}
}

// collect reads the fleet snapshot (jsDelivr, falling back to the embedded
// fixture) and resolves installed versions into rows. It never calls the GitHub
// API. The message is non-empty when the fresh snapshot was unreachable.
func (c *Controller) collect() ([]*row, string) {
	st, msg := c.loadSnapshot()
	if st == nil {
		return nil, msg
	}
	c.setUpdated(st.GeneratedAt)
	return c.rowsFromStore(st), msg
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
			c.listBox.Add(newRowCard(c.buildRow(r)))
		}
	}
	// Make the scroll hug its content so a short fleet doesn't leave a big
	// empty band: set the scroll's min size to the list's natural size. The
	// panel layout then caps it to the window and centers it when it fits,
	// and the inner scroll still takes over when the list overflows.
	if c.scroll != nil {
		ms := c.listBox.MinSize()
		// Headroom below the last row so wrapped description lines aren't clipped
		// when the panel hugs the content.
		ms.Height += theme.Padding() * 2
		c.scroll.SetMinSize(ms)
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
		if r.status == fleet.NotInstalled || r.status == fleet.UpgradeAvailable || r.status == fleet.Unknown {
			return true
		}
	}
	return false
}

func (c *Controller) buildRow(r *row) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(r.app.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	// Version shown inline next to the title, in brackets and subdued (the row's
	// lowest visual tier) so "Name (v1.2.3)" reads at a glance. No status glyph:
	// the action buttons (Install/Update/Launch) already convey install state.
	ver := canvas.NewText(titleVersion(r), mutedTextColor())
	titleLine := container.NewHBox(title, ver)

	// Single-line description, truncated at whatever fits the row width (with an
	// ellipsis) so rows stay uniform regardless of how long the summary is.
	desc := widget.NewLabel(r.app.Description)
	desc.Truncation = fyne.TextTruncateClip

	// Action column, stacked vertically: Install/Update only when relevant, Launch
	// only when installed. Centered so the button aligns to the row middle.
	var col []fyne.CanvasObject
	if r.status != fleet.UpToDate {
		btn := widget.NewButtonWithIcon(actionButtonLabel(r.status), lucide(luPrimaryIcon(r.status)), func() {
			if r.status == fleet.NotInstalled || r.status == fleet.UpgradeAvailable || r.status == fleet.Unknown {
				c.goInstall(r)
			}
		})
		btn.Importance = widget.HighImportance
		col = append(col, btn)
	}
	if r.status != fleet.NotInstalled {
		launchBtn := widget.NewButton("Launch", func() { c.openApp(r) })
		launchBtn.Importance = widget.MediumImportance
		col = append(col, launchBtn)
	}
	actionCol := container.NewCenter(container.NewVBox(col...))

	// Uniform row: a fixed-height cell whose three columns (icon | text | action)
	// are all vertically centered by the Border layout. Descriptions that exist
	// are one line, so every row has the same rhythm and the action button never
	// floats in dead space.
	info := container.NewVBox(titleLine, desc)
	infoCol := container.New(vCenterLayout{}, info)
	body := container.NewBorder(nil, nil, iconTile(r), actionCol, infoCol)
	return container.NewPadded(hPad(container.New(minHeightLayout{Min: rowHeight()}, body), cardPad()))
}

// iconTile renders the app icon on a consistent rounded tile so icons with
// different built-in backgrounds read as a uniform rail across rows.
func iconTile(r *row) fyne.CanvasObject {
	// Transparent well: reserves a fixed 48px square so rows stay aligned, but
	// draws no background fill over the card surface.
	surf := canvas.NewRectangle(color.Transparent)
	surf.SetMinSize(fyne.NewSize(tileSize(), tileSize()))
	var img fyne.CanvasObject
	if b, ok := appdata.Icon(r.ma.ID); ok && len(b) > 0 {
		im := canvas.NewImageFromResource(fyne.NewStaticResource("icon_"+r.ma.ID+".png", b))
		im.FillMode = canvas.ImageFillContain
		im.SetMinSize(fyne.NewSize(tileSize()-12, tileSize()-12))
		img = container.NewCenter(im)
	} else {
		img = canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	}
	return container.NewStack(surf, container.NewCenter(img))
}

// themed returns the given color name from the ACTIVE theme at the current
// variant, so a surface color is always coherent with the window background.
func themed(name fyne.ThemeColorName) color.Color {
	a := fyne.CurrentApp()
	if a == nil || a.Settings() == nil {
		return nordRGB(nord1)
	}
	return a.Settings().Theme().Color(name, themeVariant())
}

// themeVariant returns the active theme variant (dark/light), defaulting to
// dark so cards don't collapse into the window when no app is running yet.
func themeVariant() fyne.ThemeVariant {
	a := fyne.CurrentApp()
	if a == nil || a.Settings() == nil {
		return theme.VariantDark
	}
	return a.Settings().ThemeVariant()
}

// mutedTextColor is the subdued tone for the version tag — the lowest visual
// tier in a row. It must be dimmer than the description yet still readable
// (>=4.5:1): dark uses a mid light-gray on the nord1 surface (~4.6:1), light
// uses nord3 on white (~7.4:1). Different because all of Nord's polar-night
// shades are too close to nord1 to be legible.
func mutedTextColor() color.Color {
	if themeVariant() == theme.VariantLight {
		return nordRGB(nord3)
	}
	return nordRGB(0xA6B1C0)
}

// titleVersion renders the version shown inline next to the app title, in
// brackets and muted: "Name (v1.2.3)". Uses the latest release version; for a
// not-installed app we still show it (that's what "Install" would pull).
func titleVersion(r *row) string {
	if r.status == fleet.Unknown {
		return "(version unknown)"
	}
	if r.app == nil || r.app.LatestVersion == "" {
		return ""
	}
	return "(" + r.app.LatestVersion + ")"
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

// openApp opens an installed web app's dashboard in the browser. These apps are
// not desktop apps: they serve a local web UI, and their `binary start` only
// backgrounds the server (it does NOT auto-launch the browser). So Open starts
// the dashboard if needed, waits until it accepts connections, then opens the
// browser to the local open_url.
func (c *Controller) openApp(r *row) {
	if r.app == nil {
		return
	}
	target := r.app.OpenURL
	if target == "" {
		target = r.app.Homepage
		openURLged(target)
		return
	}
	go func() {
		if r.app.Daemon != nil && r.app.Daemon.HasDaemon && len(r.app.Daemon.StartArgs) > 0 {
			bin := c.st.Destination(r.ma)
			c.engine.Exec.Run(context.Background(), bin, r.app.Daemon.StartArgs...)
		}
		// The daemon binds asynchronously; wait for it, then hand off to a
		// standalone browser window (falls back to the default browser tab).
		waitForURL(target, 6*time.Second)
		openStandaloneOrDefault(target)
	}()
}

// waitForURL polls a local dashboard URL until it responds (up to timeout ms),
// so the browser opens against a live server instead of a connection-refused page.
func waitForURL(url string, timeout time.Duration) {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(400 * time.Millisecond)
	}
}

// setStatusOnGoroutine is setStatus wrapped for use from a background
// goroutine (thread-safe via fyne.Do).
func (c *Controller) setStatusOnGoroutine(s string) {
	fyne.Do(func() { c.setStatus(s) })
}

// goInstall wires an install/update and its progress to the UI (async).
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
			c.setStatus("")
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
			if status == fleet.NotInstalled || status == fleet.UpgradeAvailable || status == fleet.Unknown {
				c.setStatusOnGoroutine(fmt.Sprintf("Updating %s…", r.app.DisplayName))
				if err := c.engine.Install(context.Background(), r.ma, r.app, nil); err != nil {
					fyne.Do(func() { c.errLbl.SetText(fmt.Sprintf("update %s: %v", r.app.DisplayName, err)) })
				}
			}
		}
		fyne.Do(func() { c.actionLbl.SetText(""); c.setStatus(""); c.goRefresh() })
	}()
}

// setStatus shows a transient status in the header, hiding it when the message
// is empty (idle) so no permanent "Ready" text lingers. It mutates directly:
// callers on the main thread call it as-is, and goroutines must wrap the call
// in fyne.Do (this is the Fyne 2.8 threading model — do NOT call fyne.Do from
// the main thread or before the event loop starts).
func (c *Controller) setStatus(s string) {
	if c.a == nil {
		return
	}
	if s == "" {
		c.statusLbl.Hide()
		return
	}
	c.statusLbl.SetText(s)
	c.statusLbl.Show()
}

// --- small helpers ---

// actionButtonLabel reports the text for the primary Install/Update action.
func actionButtonLabel(s fleet.Status) string {
	switch s {
	case fleet.NotInstalled:
		return "Install"
	case fleet.UpgradeAvailable, fleet.Unknown:
		return "Update"
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