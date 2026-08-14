//go:build !windows

package gui

import (
	"os/exec"
	"regexp"
	"strconv"
)

// platformScreen returns the primary screen size on non-Windows systems. On
// Linux it parses `xrandr --current`; other platforms fall back to undetected.
func platformScreen() (int, int, bool) {
	if out, err := exec.Command("xrandr", "--current").Output(); err == nil {
		re := regexp.MustCompile(`current\s+(\d+)\s*x\s*(\d+)`)
		if m := re.FindSubmatch(out); m != nil {
			w, _ := strconv.Atoi(string(m[1]))
			h, _ := strconv.Atoi(string(m[2]))
			if w > 0 && h > 0 {
				return w, h, true
			}
		}
	}
	return 0, 0, false
}
