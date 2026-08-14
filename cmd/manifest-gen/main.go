// Command manifest-gen reads manifest.yaml, resolves the fleet against the
// GitHub API, and writes the generated apps.json served via jsDelivr.
//
// Usage:
//
//	manifest-gen [--manifest manifest.yaml] [--out apps.json] [--repo owner/repo]
//
// GITHUB_TOKEN is used when present (recommended for Actions to avoid rate limits).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/udit-001/dock/internal/registry"
)

func main() {
	manifestPath := flag.String("manifest", "internal/appdata/manifest.yaml", "path to manifest.yaml")
	outPath := flag.String("out", "apps.json", "output apps.json path")
	repoArg := flag.String("repo", "udit-001/dock", "owner/repo hosting icons")
	branchArg := flag.String("branch", "main", "jsDelivr branch for icon URLs")
	flag.Parse()

	m, err := registry.LoadManifest(*manifestPath)
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}
	if *repoArg == "" {
		*repoArg = m.Repo
	}
	if *repoArg == "" {
		*repoArg = "udit-001/dock"
	}

	cli := registry.NewGHClient(*repoArg, *branchArg)
	st, err := registry.Generate(m, cli)
	if err != nil {
		log.Fatalf("generate: %v", err)
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		log.Fatalf("encode: %v", err)
	}
	if err := os.WriteFile(*outPath, append(data, '\n'), 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Printf("wrote %s (%d apps)\n", *outPath, len(st.Apps))
}
