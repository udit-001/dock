//go:build windows

package gui

import "syscall"

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	getSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	smCXScreen = 0
	smCYScreen = 1
)

// platformScreen returns the primary screen size on Windows.
func platformScreen() (int, int, bool) {
	w, _, _ := getSystemMetrics.Call(smCXScreen)
	h, _, _ := getSystemMetrics.Call(smCYScreen)
	return int(w), int(h), true
}
