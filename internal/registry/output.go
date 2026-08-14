package registry

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

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
