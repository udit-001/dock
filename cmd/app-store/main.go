// Command app-store is the App Store manager: a minimal desktop window that
// lists the fleet from the embedded manifest, checks GitHub releases directly,
// and installs/updates them (stopping/restarting daemons).
//
// Install locally (no Mark-of-the-Web) with:
//
//	go install github.com/udit-001/app-store@latest
//
// Headless screenshot (renders the UI off-screen to a PNG, no display needed):
//
//	app-store --screenshot out.png
package main

import (
	"flag"
	"log"

	"github.com/udit-001/app-store/internal/gui"
)

func main() {
	flag.Parse()

	c, err := gui.New()
	if err != nil {
		log.Fatalf("app-store: %v", err)
	}
	c.Run()
}