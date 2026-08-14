package gui

import (
	"fyne.io/fyne/v2"
)

// screenSize returns the current display resolution (so the window opens sized
// to the screen with chrome, not fullscreen), falling back to windowSize.
func screenSize() fyne.Size {
	if w, h, ok := platformScreen(); ok && w > 0 && h > 0 {
		return fyne.NewSize(float32(w), float32(h))
	}
	return windowSize()
}