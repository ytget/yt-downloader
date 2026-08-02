// Package updatecheck optionally compares the running version to the latest
// GitHub release. Network access is explicit and user/settings gated.
package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultAPI = "https://api.github.com/repos/ytget/yt-downloader/releases/latest"

// Result is a non-intrusive update notice.
type Result struct {
	Checked         bool   `json:"checked"`
	UpdateAvailable bool   `json:"update_available"`
	Current         string `json:"current"`
	Latest          string `json:"latest,omitempty"`
	HTMLURL         string `json:"html_url,omitempty"`
	Error           string `json:"error,omitempty"`
}

type releasePayload struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// CheckLatest queries GitHub Releases for a newer non-draft tag.
// current should be semver-ish without requiring a leading v.
func CheckLatest(ctx context.Context, current string) Result {
	return CheckLatestURL(ctx, current, defaultAPI)
}

// CheckLatestURL is the testable entrypoint.
func CheckLatestURL(ctx context.Context, current, apiURL string) Result {
	res := Result{Current: strings.TrimSpace(current)}
	if res.Current == "" || res.Current == "dev" {
		res.Error = "dev builds skip update check"
		return res
	}
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "yt-downloader-updatecheck")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	res.Checked = true
	if resp.StatusCode != http.StatusOK {
		res.Error = fmt.Sprintf("github api status %d", resp.StatusCode)
		return res
	}
	var payload releasePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		res.Error = err.Error()
		return res
	}
	if payload.Draft || payload.Prerelease {
		return res
	}
	latest := normalizeTag(payload.TagName)
	res.Latest = latest
	res.HTMLURL = payload.HTMLURL
	res.UpdateAvailable = compareSemver(latest, normalizeTag(current)) > 0
	return res
}

func normalizeTag(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	// strip build metadata
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	return s
}

// compareSemver returns 1 if a>b, -1 if a<b, 0 if equal/unparseable equalish.
func compareSemver(a, b string) int {
	ap := parseSemver(a)
	bp := parseSemver(b)
	for i := 0; i < 3; i++ {
		if ap[i] > bp[i] {
			return 1
		}
		if ap[i] < bp[i] {
			return -1
		}
	}
	return 0
}

func parseSemver(s string) [3]int {
	var out [3]int
	// drop prerelease suffix
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		for _, ch := range parts[i] {
			if ch < '0' || ch > '9' {
				break
			}
			n = n*10 + int(ch-'0')
		}
		out[i] = n
	}
	return out
}
