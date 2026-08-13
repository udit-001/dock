package registry

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// Output types mirror the generated apps.json schema served via jsDelivr.

// Store is the root of apps.json.
type Store struct {
	GeneratedAt time.Time `json:"generated_at"`
	Apps        []App     `json:"apps"`
}

// App is one entry in apps.json.
type App struct {
	ID            string        `json:"id"`
	DisplayName   string        `json:"display_name"`
	Repo          string        `json:"repo"`
	Homepage      string        `json:"homepage"`
	Description   string        `json:"description"`
	Icon          string        `json:"icon"`
	Daemon        *DaemonOut    `json:"daemon"`
	Binary        string        `json:"binary"`
	LatestVersion string        `json:"latest_version"`
	PublishedAt   string        `json:"published_at"`
	Prerelease    bool          `json:"prerelease"`
	Assets        map[string]Asset `json:"assets"`
}

// DaemonOut tells the desktop app whether an app has a controllable daemon.
type DaemonOut struct {
	HasDaemon bool     `json:"has_daemon"`
	Stop      []string `json:"stop"`
	Start     []string `json:"start"`
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

// GHClient is a thin GitHub API client. token "" means unauthenticated.
type GHClient struct {
	HTTP   *http.Client
	Token  string
	Branch string // jsDelivr branch for icon URLs
	Repo   string // this repo (owner/repo) hosting icons
}

// NewGHClient builds a client, preferring a GITHUB_TOKEN env var when token is empty.
func NewGHClient(ownerRepo, branch string) *GHClient {
	token := os.Getenv("GITHUB_TOKEN")
	return &GHClient{
		HTTP:   &http.Client{Timeout: 30 * time.Second},
		Token:  token,
		Branch: branch,
		Repo:   ownerRepo,
	}
}

// apiGET calls the GitHub REST API, decoding JSON into v, returning the body on
// non-2xx as an error too.
func (c *GHClient) apiGET(url string, v any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return jsonDecode(resp, v)
}