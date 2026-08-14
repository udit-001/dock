// Command dock is Dock: a minimal desktop window that
// lists the fleet from the embedded manifest, checks GitHub releases directly,
// and installs/updates them (stopping/restarting daemons).
//
// Install locally (no Mark-of-the-Web) with:
//
//	go install github.com/udit-001/dock@latest
//
// Headless screenshot (renders the UI off-screen to a PNG, no display needed):
//
//	dock --screenshot out.png
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/udit-001/dock/internal/gui"
)

func main() {
	inspectPath := flag.String("inspect", "", "dump the widget-tree markup to a file (use - for stdout) and exit")
	shotPath := flag.String("shot", "", "render the UI off-screen to a PNG at this path and exit")
	flag.Parse()

	c, err := gui.New()
	if err != nil {
		log.Fatalf("dock: %v", err)
	}
	if *shotPath != "" {
		if err := c.RenderPNG(*shotPath); err != nil {
			log.Fatalf("shot: %v", err)
		}
		log.Printf("wrote %s", *shotPath)
		return
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
