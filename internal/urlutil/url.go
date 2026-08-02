// Package urlutil centralizes URL normalization and YouTube URL classification.
package urlutil

import (
	"fmt"
	"net/url"
	"strings"
)

// Normalize removes whitespace characters that commonly arrive from clipboard input.
func Normalize(raw string) string {
	clean := strings.NewReplacer("\n", "", "\r", "", "\t", " ").Replace(raw)
	return strings.TrimSpace(clean)
}

// Validate accepts only absolute HTTP(S) URLs with a host.
func Validate(raw string) error {
	if raw == "" {
		return fmt.Errorf("empty")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("missing host")
	}
	return nil
}

// IsPlaylist reports whether the URL contains a playlist query parameter.
func IsPlaylist(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return u.Query().Get("list") != ""
}

// VideoID extracts common YouTube/Vimeo identifiers, with a stable generic fallback.
func VideoID(raw string) string {
	u, err := url.Parse(raw)
	if err == nil {
		host := strings.ToLower(u.Hostname())
		switch {
		case strings.Contains(host, "youtube.com"):
			if id := u.Query().Get("v"); len(id) == 11 {
				return id
			}
		case host == "youtu.be":
			parts := strings.Split(strings.Trim(u.Path, "/"), "/")
			if len(parts) > 0 && len(parts[0]) == 11 {
				return parts[0]
			}
		case strings.Contains(host, "vimeo.com"):
			id := strings.Trim(strings.Split(strings.Trim(u.Path, "/"), "/")[0], " ")
			if id != "" {
				return "vimeo_" + id
			}
		}
	}

	// Preserve the service's historical fallback contract for non-standard URLs.
	hash := fmt.Sprintf("%x", len(raw))
	if len(hash) < 8 {
		hash += strings.Repeat("0", 8-len(hash))
	}
	return "video_" + hash[:8]
}
