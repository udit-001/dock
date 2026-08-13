package registry

import (
	"fmt"
	"strings"
	"time"
)

// Generate resolves every app in the manifest against the GitHub API and
// produces the apps.json Store.
func Generate(m *Manifest, cli *GHClient) (*Store, error) {
	if cli.Branch == "" {
		cli.Branch = m.Branch
	}
	if cli.Branch == "" {
		cli.Branch = "main"
	}
	st := &Store{GeneratedAt: time.Now().UTC()}
	for _, app := range m.Apps {
		out, err := cli.ResolveApp(app)
		if err != nil {
			// Fail the whole run so the Action notices a broken repo in the manifest,
			// rather than silently serving a stale snapshot for it.
			return nil, fmt.Errorf("%s: %w", app.ID, err)
		}
		st.Apps = append(st.Apps, *out)
	}
	return st, nil
}

// ResolveApp fetches the latest release metadata for one manifest app directly
// from the GitHub API and resolves its per-platform assets + checksums. This is
// the runtime path the desktop app uses (MVP fetches straight from GitHub;
// generated apps.json via jsDelivr is deferred).
func (c *GHClient) ResolveApp(ma ManifestApp) (*App, error) {
	repoMeta, err := c.getRepo(ma.Repo)
	if err != nil {
		return nil, fmt.Errorf("repo metadata: %w", err)
	}
	rel, err := c.getRelease(ma.Repo, "")
	if err != nil {
		return nil, fmt.Errorf("latest release: %w", err)
	}
	checksums, _ := c.checksums(ma.Repo, rel.TagName) // non-fatal if absent

	app := &App{
		ID:            ma.ID,
		DisplayName:   ma.DisplayName,
		Repo:          ma.Repo,
		Homepage:      repoMeta.HtmlURL,
		Description:   repoMeta.Description,
		Binary:        ma.Binary,
		LatestVersion: rel.TagName,
		PublishedAt:   rel.PublishedAt,
		Prerelease:    rel.Prerelease,
		Assets:        map[string]Asset{},
	}
	if ma.DisplayName == "" {
		app.DisplayName = repoMeta.FullName
	}
	if ma.Icon != "" {
		app.Icon = fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s@%s/%s",
			c.Repo, c.Branch, strings.TrimPrefix(ma.Icon, "./"))
	}
	if ma.Daemon != nil {
		app.Daemon = &DaemonOut{
			HasDaemon: true,
			Stop:      ma.Daemon.Stop,
			Start:     ma.Daemon.Start,
			Status:    ma.Daemon.Status,
		}
	}

	for _, plat := range ma.Platforms {
		asset, err := resolveAsset(rel, ma.ReleaseName, rel.TagName, plat)
		if err != nil {
			// Tolerate a missing platform rather than failing the whole app.
			continue
		}
		if sum := checksums[asset.Name]; sum != "" {
			asset.sha256 = sum
		}
		app.Assets[plat] = Asset{
			URL:      asset.BrowserDownloadURL,
			SHA256:   asset.sha256,
			Size:     asset.Size,
			FileName: asset.Name,
		}
	}

	if len(app.Assets) == 0 {
		return nil, fmt.Errorf("no platform assets resolved for release %s", rel.TagName)
	}
	return app, nil
}

// resolvedAsset is what resolve picked for one platform.
type resolvedAsset struct {
	Name               string
	BrowserDownloadURL string
	Size               int64
	sha256             string
}

// resolveAsset maps a platform (os/arch) to the GoReleaser asset for that
// platform. The base filename is {{release_name}}_{{version(no v)}}_{{os}}_{{arch}},
// and the actual asset name appends an extension (.exe on Windows, .tar.gz or
// .zip elsewhere), matching whatever the repo actually uploaded.
func resolveAsset(rel *ghRelease, releaseName, version, platform string) (*resolvedAsset, error) {
	parts := strings.SplitN(platform, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("bad platform %q", platform)
	}
	osName, arch := parts[0], parts[1]
	ver := strings.TrimPrefix(version, "v")
	base := fmt.Sprintf("%s_%s_%s_%s", releaseName, ver, osName, arch)

	preferred := []string{".tar.gz", ".zip", ".gz", ""}
	if osName == "windows" {
		preferred = []string{".exe", ".zip", ""}
	}

	var best *ghAsset
	bestIdx := len(preferred)
	for i := range rel.Assets {
		a := &rel.Assets[i]
		name := a.Name
		if !strings.HasPrefix(name, base+".") && name != base {
			continue
		}
		ext := strings.TrimPrefix(name[len(base):], ".")
		idx := -1
		for p, want := range preferred {
			if ext == strings.TrimPrefix(want, ".") {
				idx = p
				break
			}
		}
		if idx >= 0 && idx < bestIdx {
			best = a
			bestIdx = idx
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no asset matching base %q", base)
	}
	return &resolvedAsset{
		Name:               best.Name,
		BrowserDownloadURL: best.BrowserDownloadURL,
		Size:               best.Size,
	}, nil
}