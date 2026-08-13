package registry

import (
	"encoding/json"
	"io"
	"net/http"
)

func jsonDecode(resp *http.Response, v any) error {
	return json.NewDecoder(resp.Body).Decode(v)
}

// GitHub API response shapes (only the fields we need).

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt string    `json:"published_at"`
	Assets      []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type ghRepo struct {
	FullName    string `json:"full_name"`
	HtmlURL     string `json:"html_url"`
	Description string `json:"description"`
}

func (a *GHClient) getRelease(ownerRepo, version string) (*ghRelease, error) {
	url := apiBase + "/repos/" + ownerRepo + "/releases/latest"
	if version != "" {
		url = apiBase + "/repos/" + ownerRepo + "/releases/tags/" + version
	}
	var rel ghRelease
	if err := a.apiGET(url, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func (a *GHClient) getRepo(ownerRepo string) (*ghRepo, error) {
	var repo ghRepo
	if err := a.apiGET(apiBase+"/repos/"+ownerRepo, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// checksums downloads checksums.txt from a release and returns sha256 by filename.
func (a *GHClient) checksums(ownerRepo, version string) (map[string]string, error) {
	rawURL := "https://github.com/" + ownerRepo + "/releases/download/" + version + "/checksums.txt"
	resp, err := a.HTTP.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, io.ErrUnexpectedEOF // treat missing checksums as non-fatal downstream
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range splitLines(string(data)) {
		fs := splitFields(line)
		if len(fs) == 2 && isHex(fs[0], 64) {
			out[fs[1]] = fs[0]
		}
	}
	return out, nil
}

const apiBase = "https://api.github.com"