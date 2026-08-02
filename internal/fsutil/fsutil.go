// Package fsutil provides download-path safety helpers: sanitization,
// writability, free-space checks, collision policy, and atomic writes.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CollisionPolicy controls behavior when the target file already exists.
type CollisionPolicy string

const (
	// CollisionSkip treats an existing non-empty file as already done.
	CollisionSkip CollisionPolicy = "skip"
	// CollisionOverwrite replaces an existing file.
	CollisionOverwrite CollisionPolicy = "overwrite"
	// CollisionRename picks a free "name (N).ext" path.
	CollisionRename CollisionPolicy = "rename"
)

// DefaultCollisionPolicy is used when unset/invalid.
const DefaultCollisionPolicy = CollisionRename

// DefaultMinFreeBytes is the minimum free space required before a media download.
// Actual media size is often unknown up-front; this is a safety floor.
const DefaultMinFreeBytes int64 = 100 * 1024 * 1024 // 100 MiB

// MaxFilenameBytes limits the final file base name (not including extension).
const MaxFilenameBytes = 180

// NormalizeCollisionPolicy returns a valid policy (default rename).
func NormalizeCollisionPolicy(p CollisionPolicy) CollisionPolicy {
	switch CollisionPolicy(strings.ToLower(strings.TrimSpace(string(p)))) {
	case CollisionSkip:
		return CollisionSkip
	case CollisionOverwrite:
		return CollisionOverwrite
	case CollisionRename:
		return CollisionRename
	default:
		return DefaultCollisionPolicy
	}
}

// EnsureWritableDir creates dir if needed and verifies a file can be created there.
func EnsureWritableDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return fmt.Errorf("download directory is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create download directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("download directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("download path is not a directory: %s", dir)
	}

	// Probe write+remove.
	f, err := os.CreateTemp(dir, ".yt-downloader-write-test-*")
	if err != nil {
		return fmt.Errorf("download directory is not writable: %w", err)
	}
	name := f.Name()
	_ = f.Close()
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("download directory cleanup failed: %w", err)
	}
	return nil
}

// CheckFreeSpace ensures path's volume has at least minBytes free.
// If free space cannot be determined, it returns nil (best-effort).
func CheckFreeSpace(path string, minBytes int64) error {
	if minBytes <= 0 {
		return nil
	}
	free, err := FreeBytes(path)
	if err != nil {
		// Best-effort: do not hard-fail unknown platforms / permission quirks.
		return nil
	}
	if free < uint64(minBytes) {
		return fmt.Errorf("insufficient free disk space: need at least %s, have %s",
			formatBytes(uint64(minBytes)), formatBytes(free))
	}
	return nil
}

func formatBytes(n uint64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case n >= gib:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// SanitizeFilename produces a cross-platform safe single path segment.
func SanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "download"
	}

	// Drop path components if a full path sneaks in.
	name = filepath.Base(name)
	name = strings.TrimSpace(name)

	var b strings.Builder
	b.Grow(len(name))
	lastWasSep := false
	for _, r := range name {
		switch {
		case r == 0 || r < 0x20 || r == 0x7f:
			// control chars
			continue
		case strings.ContainsRune(`<>:"/\|?*`, r):
			if !lastWasSep {
				b.WriteByte('_')
				lastWasSep = true
			}
		case unicode.IsSpace(r):
			if !lastWasSep {
				b.WriteByte('_')
				lastWasSep = true
			}
		case r == '.' || r == '_':
			// keep, but collapse runs of separators later via lastWasSep for '_'
			b.WriteRune(r)
			lastWasSep = r == '_'
		default:
			// Avoid other non-printable unicode.
			if !unicode.IsPrint(r) {
				continue
			}
			b.WriteRune(r)
			lastWasSep = false
		}
	}
	out := strings.Trim(b.String(), " ._")
	if out == "" {
		out = "download"
	}

	// Windows reserved device names (any extension).
	upper := strings.ToUpper(out)
	base := upper
	if i := strings.IndexByte(upper, '.'); i >= 0 {
		base = upper[:i]
	}
	if isWindowsReserved(base) {
		out = "_" + out
	}

	// Truncate on UTF-8 boundaries.
	if len(out) > MaxFilenameBytes {
		out = truncateBytes(out, MaxFilenameBytes)
		out = strings.TrimRight(out, " ._")
		if out == "" {
			out = "download"
		}
	}
	return out
}

func isWindowsReserved(baseUpper string) bool {
	switch baseUpper {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.ValidString(s[:max]) {
		max--
	}
	return s[:max]
}

// CollisionResult is the outcome of applying a collision policy to a target path.
type CollisionResult struct {
	// Path is the path that should be used for writing/downloading.
	Path string
	// Skip is true when an existing file should be treated as already complete.
	Skip bool
}

// ResolveCollision applies policy for target path. target should be the desired final path.
func ResolveCollision(target string, policy CollisionPolicy) (CollisionResult, error) {
	policy = NormalizeCollisionPolicy(policy)
	target = filepath.Clean(target)

	fi, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return CollisionResult{Path: target, Skip: false}, nil
		}
		return CollisionResult{}, err
	}
	if fi.IsDir() {
		return CollisionResult{}, fmt.Errorf("output path is a directory: %s", target)
	}

	switch policy {
	case CollisionSkip:
		if fi.Size() > 0 {
			return CollisionResult{Path: target, Skip: true}, nil
		}
		// empty file — overwrite
		return CollisionResult{Path: target, Skip: false}, nil
	case CollisionOverwrite:
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return CollisionResult{}, fmt.Errorf("cannot overwrite existing file: %w", err)
		}
		// also drop common partial sidecars
		_ = os.Remove(target + ".part")
		_ = os.Remove(target + ".ytdl")
		return CollisionResult{Path: target, Skip: false}, nil
	case CollisionRename:
		alt, err := uniquePath(target)
		if err != nil {
			return CollisionResult{}, err
		}
		return CollisionResult{Path: alt, Skip: false}, nil
	default:
		return CollisionResult{Path: target, Skip: false}, nil
	}
}

func uniquePath(target string) (string, error) {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 1; i < 10_000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find free filename for %s", target)
}

// AtomicWriteFile writes data to path via temp file + rename.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if perm == 0 {
		perm = 0o644
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		// Non-fatal on some platforms (e.g. Windows).
		_ = err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Windows cannot rename over existing file.
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	committed = true
	return nil
}
