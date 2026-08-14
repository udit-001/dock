package gui

import "fyne.io/fyne/v2"

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