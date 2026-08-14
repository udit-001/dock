package catalog

import "time"

// Store is the root of apps.json.
type Store struct {
	GeneratedAt time.Time `json:"generated_at"`
	Apps        []App     `json:"apps"`
}

// App is one entry in apps.json.
type App struct {
	ID            string           `json:"id"`
	DisplayName   string           `json:"display_name"`
	Repo          string           `json:"repo"`
	Homepage      string           `json:"homepage"`
	Description   string           `json:"description"`
	Icon          string           `json:"icon"`
	Daemon        *DaemonOut       `json:"daemon"`
	Binary        string           `json:"binary"`
	OpenURL       string           `json:"open_url"`
	LatestVersion string           `json:"latest_version"`
	PublishedAt   string           `json:"published_at"`
	Prerelease    bool             `json:"prerelease"`
	Assets        map[string]Asset `json:"assets"`
}

// DaemonOut tells the desktop app whether an app has a controllable daemon.
type DaemonOut struct {
	HasDaemon bool     `json:"has_daemon"`
	StartArgs []string `json:"start_args"`
	Stop      []string `json:"stop"`
	Status    []string `json:"status"`
}

// Asset is one downloadable artifact for an os/arch. URL is always the plain
// github.com download URL (never api.github.com). SHA256 comes from the
// release checksums.txt when present.
type Asset struct {
	URL      string `json:"url"`
	SHA256   string `json:"sha256,omitempty"`
	Size     int64  `json:"size"`
	FileName string `json:"filename"`
}
