// Package pathsafety confines open/reveal operations to known safe roots.
package pathsafety

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Guard checks that absolute paths stay under allowed roots.
type Guard struct {
	roots []string
}

// New builds a guard from root directories (empty entries ignored).
func New(roots ...string) *Guard {
	g := &Guard{}
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		g.roots = append(g.roots, filepath.Clean(abs))
	}
	return g
}

// Allow adds extra roots at runtime (e.g. after settings change).
func (g *Guard) Allow(roots ...string) {
	if g == nil {
		return
	}
	for _, r := range roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		abs, err := filepath.Abs(r)
		if err != nil {
			continue
		}
		g.roots = append(g.roots, filepath.Clean(abs))
	}
}

// Roots returns a copy of allowed roots.
func (g *Guard) Roots() []string {
	if g == nil {
		return nil
	}
	out := make([]string, len(g.roots))
	copy(out, g.roots)
	return out
}

// Check returns a cleaned absolute path if it is under an allowed root.
func (g *Guard) Check(path string) (string, error) {
	if g == nil || len(g.roots) == 0 {
		return "", fmt.Errorf("no allowed path roots configured")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return "", fmt.Errorf("remote URLs are not openable paths")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)

	// Resolve symlinks when present so ../ escapes via link are caught.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	} else if !os.IsNotExist(err) {
		// Keep cleaned abs if target does not exist yet (reveal parent may still work).
	}

	for _, root := range g.roots {
		rootResolved := root
		if r, err := filepath.EvalSymlinks(root); err == nil {
			rootResolved = r
		}
		if abs == rootResolved || isUnder(rootResolved, abs) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("path is outside allowed directories")
}

func isUnder(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
