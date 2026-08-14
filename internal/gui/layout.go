package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// cardPad is the extra horizontal breathing room inside a list row, on top of
// the theme's default padding, so card content doesn't hug the row edges.
func cardPad() float32 { return 13 }

// rowHeight is the fixed height of every app row, so cards stay uniform even
// when some apps lack a description line.
func rowHeight() float32 { return 78 }

// tileSize is the uniform square size of each row's app-icon tile.
func tileSize() float32 { return 48 }

// panelGap is the small fixed gap between the header (incl. any offline notice)
// and the app-list panel.
func panelGap() float32 { return 12 }

// hPad wraps obj with a fixed horizontal gutter on each side (transparent
// spacers in a Border), giving rows more horizontal breathing room without
// adding vertical space.
func hPad(obj fyne.CanvasObject, px float32) fyne.CanvasObject {
	spacer := func() fyne.CanvasObject {
		r := canvas.NewRectangle(color.Transparent)
		r.SetMinSize(fyne.NewSize(px, 0))
		return r
	}
	return container.NewBorder(nil, nil, spacer(), spacer(), obj)
}

// minHeightLayout sizes its single child to fill the given area but never lets
// its minimum height drop below Min, so a row keeps a uniform height even when
// its content is short.
type minHeightLayout struct{ Min float32 }

func (m minHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (m minHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	s := objects[0].MinSize()
	if s.Height < m.Min {
		s.Height = m.Min
	}
	return s
}

// vCenterLayout left-aligns its single child and centers it vertically in the
// available height, so a row's text block hugs the icon column instead of
// floating in the middle of the wide center column.
type vCenterLayout struct{}

func (vCenterLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		ms := o.MinSize()
		y := (size.Height - ms.Height) / 2
		if y < 0 {
			y = 0
		}
		o.Resize(fyne.NewSize(size.Width, ms.Height))
		o.Move(fyne.NewPos(0, y))
	}
}

func (vCenterLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return objects[0].MinSize()
}

// fixedWidthLayout reserves a fixed width and centers its single child inside
// it, so toggling between a wide button and a narrow spinner doesn't shift the
// surrounding layout (used for the in-button "checking" spinner).
type fixedWidthLayout struct{ Width float32 }

func (l fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		ms := o.MinSize()
		x := (size.Width - ms.Width) / 2
		y := (size.Height - ms.Height) / 2
		o.Resize(ms)
		o.Move(fyne.NewPos(x, y))
	}
}

func (l fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objects {
		if ms := o.MinSize().Height; ms > h {
			h = ms
		}
	}
	return fyne.NewSize(l.Width, h)
}

// cappedLayout centers a single child and caps its width (like a Bootstrap
// container): full width up to `Max`, then centered with equal side margins on
// wider surfaces. Height fills the child's natural size.
type cappedLayout struct {
	Max float32
}

func (c cappedLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		w := size.Width
		if w > c.Max {
			w = c.Max
		}
		off := (size.Width - w) / 2
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos(off, 0))
	}
}

func (c cappedLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		ms := o.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		if ms.Height > h {
			h = ms.Height
		}
	}
	if w > c.Max {
		w = c.Max
	}
	return fyne.NewSize(w, h)
}

// panelFitLayout stacks a header (natural height) above a content panel. The
// panel hugs the header with a small fixed gap and sizes to its own content
// (or fills the window when the fleet overflows). It is NOT vertically
// centered: centering would open a large dead gap between the header/offline
// notice and the list when the fleet is short.
type panelFitLayout struct{}

func (p panelFitLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) != 2 {
		return
	}
	header, panel := objects[0], objects[1]
	headerH := header.MinSize().Height
	header.Resize(fyne.NewSize(size.Width, headerH))
	header.Move(fyne.NewPos(0, 0))

	top := headerH + panelGap()
	avail := size.Height - top
	ph := panel.MinSize().Height
	if ph > avail {
		ph = avail
	}
	panel.Resize(fyne.NewSize(size.Width, ph))
	panel.Move(fyne.NewPos(0, top))
}

func (p panelFitLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) != 2 {
		return fyne.NewSize(0, 0)
	}
	h := objects[0].MinSize().Height + panelGap() + objects[1].MinSize().Height
	w := objects[1].MinSize().Width
	return fyne.NewSize(w, h)
}

// topPadLayout surrounds a single child with a fixed top gap (breathing room
// above a header without depending on theme padding).
type topPadLayout struct {
	Top float32
}

func (t topPadLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(fyne.NewSize(size.Width, size.Height-t.Top))
		o.Move(fyne.NewPos(0, t.Top))
	}
}

func (t topPadLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		ms := o.MinSize()
		if ms.Width > w {
			w = ms.Width
		}
		if ms.Height > h {
			h = ms.Height
		}
	}
	return fyne.NewSize(w, h+t.Top)
}

// padTop wraps obj so it gets px of extra space above it.
func padTop(obj fyne.CanvasObject, px float32) fyne.CanvasObject {
	return container.New(topPadLayout{Top: px}, obj)
}
