package gui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// rowCard wraps one app row in a rounded, hover-sensitive surface. Resting rows
// are flat; the active row lifts with a short fade to the theme's hover color.
// This is state-only, consistent across all rows, and quick (150ms) — per the
// surface layer's consistency/prominence discipline and the product register's
// "motion conveys state, not decoration". Hover is an affordance hint only:
// the rows' buttons remain the actual interaction points.
type rowCard struct {
	widget.BaseWidget
	content fyne.CanvasObject
	rect    *canvas.Rectangle
	hover   bool
}

func newRowCard(content fyne.CanvasObject) *rowCard {
	r := &rowCard{content: content}
	r.ExtendBaseWidget(r)
	return r
}

func (r *rowCard) CreateRenderer() fyne.WidgetRenderer {
	r.rect = canvas.NewRectangle(color.Transparent)
	r.rect.CornerRadius = 6
	return widget.NewSimpleRenderer(container.NewStack(r.rect, r.content))
}

func (r *rowCard) hoverFill() color.Color {
	return themed(theme.ColorNameHover)
}

// setFill animates the background toward to over 150ms, so the highlight fades
// (conveys state) rather than snapping.
func (r *rowCard) animate(to color.Color) {
	if r.rect == nil {
		return
	}
	from := r.rect.FillColor
	anim := canvas.NewColorRGBAAnimation(from, to, 150*time.Millisecond, func(c color.Color) {
		r.rect.FillColor = c
		r.rect.Refresh()
	})
	anim.Start()
}

var _ desktop.Hoverable = (*rowCard)(nil)

func (r *rowCard) MouseIn(*desktop.MouseEvent) {
	if r.hover {
		return
	}
	r.hover = true
	r.animate(r.hoverFill())
}

func (r *rowCard) MouseMoved(*desktop.MouseEvent) {}

func (r *rowCard) MouseOut() {
	if !r.hover {
		return
	}
	r.hover = false
	r.animate(color.Transparent)
}
