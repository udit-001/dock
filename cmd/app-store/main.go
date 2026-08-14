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
	"fmt"
	"log"
	"os"

	"github.com/udit-001/app-store/internal/gui"
)

func main() {
	inspectPath := flag.String("inspect", "", "dump the widget-tree markup to a file (use - for stdout) and exit")
	flag.Parse()

	c, err := gui.New()
	if err != nil {
		log.Fatalf("app-store: %v", err)
	}
	if *inspectPath != "" {
		markup := c.Inspect()
		if *inspectPath == "-" {
			fmt.Print(markup)
			return
		}
		if err := os.WriteFile(*inspectPath, []byte(markup), 0o644); err != nil {
			log.Fatalf("inspect: %v", err)
		}
		log.Printf("wrote widget-tree markup to %s", *inspectPath)
		return
	}
	c.Run()
}