// Package updater checks GitHub Releases for new Karina versions using the
// public GitHub API (no token required for public repositories).
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultRepo is where Karina publishes its releases.
const DefaultRepo = "Kevin25DC/karina-token"

// Release is the essential metadata of the latest GitHub release.
type Release struct {
	TagName  string `json:"tag_name"`
	HTMLURL  string `json:"html_url"`
	Body     string `json:"body"`
	AssetURL string `json:"asset_url,omitempty"`
}

// Info is the update status shown to the user.
type Info struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	HasUpdate   bool   `json:"has_update"`
	ReleaseURL  string `json:"release_url"`
	DownloadURL string `json:"download_url"`
	Notes       string `json:"notes"`
	Error       string `json:"error,omitempty"`
}

// Checker queries the GitHub API.
type Checker struct {
	HTTP   *http.Client
	APIURL string
}

// NewChecker returns a checker against the default Karina repository.
func NewChecker() *Checker {
	return &Checker{
		HTTP:   &http.Client{Timeout: 15 * time.Second},
		APIURL: "https://api.github.com/repos/" + DefaultRepo + "/releases/latest",
	}
}

// Latest fetches the newest release from GitHub.
func (c *Checker) Latest(ctx context.Context) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.APIURL, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Karina-updater/0.2")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("github api http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Release{}, err
	}

	var raw struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Release{}, err
	}
	r := Release{TagName: raw.TagName, HTMLURL: raw.HTMLURL, Body: raw.Body}
	for _, a := range raw.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".zip") {
			r.AssetURL = a.BrowserDownloadURL
			break
		}
	}
	if r.TagName == "" {
		return Release{}, fmt.Errorf("respuesta inesperada de github")
	}
	return r, nil
}

// Check compares the current version with the latest release.
func Check(current string, rel Release) Info {
	info := Info{
		Current:     current,
		Latest:      normalize(rel.TagName),
		ReleaseURL:  rel.HTMLURL,
		DownloadURL: rel.AssetURL,
		Notes:       rel.Body,
	}
	info.HasUpdate = Newer(normalize(rel.TagName), normalize(current))
	return info
}

// Newer reports whether a > b following semantic-ish numeric comparison.
func Newer(a, b string) bool {
	as := splitNum(a)
	bs := splitNum(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(as) {
			x = as[i]
		}
		if i < len(bs) {
			y = bs[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

func splitNum(v string) []int {
	var out []int
	for _, part := range strings.Split(v, ".") {
		out = append(out, leadingInt(part))
	}
	return out
}

// leadingInt parses the leading numeric part of a token, e.g. "2-rc1" -> 2.
func leadingInt(s string) int {
	n := 0
	any := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			break
		}
		any = true
		n = n*10 + int(c-'0')
	}
	if !any {
		return 0
	}
	return n
}
