//go:build !windows

package fsutil

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// FreeBytes returns available bytes on the volume containing path.
func FreeBytes(path string) (uint64, error) {
	path = filepath.Clean(path)
	// Statfs needs an existing path; walk up if needed.
	var lastErr error
	for p := path; ; p = filepath.Dir(p) {
		var st unix.Statfs_t
		if err := unix.Statfs(p, &st); err != nil {
			lastErr = err
			parent := filepath.Dir(p)
			if parent == p {
				break
			}
			continue
		}
		// Prefer free blocks available to non-root (Bavail).
		return uint64(st.Bavail) * uint64(st.Bsize), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("statfs failed for %s", path)
	}
	return 0, lastErr
}
