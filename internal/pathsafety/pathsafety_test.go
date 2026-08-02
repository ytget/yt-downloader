package pathsafety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckAllowsUnderRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a", "b.mp4")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(root)
	got, err := g.Check(file)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("empty")
	}
}

func TestCheckRejectsOutside(t *testing.T) {
	root := t.TempDir()
	g := New(root)
	if _, err := g.Check("/etc/passwd"); err == nil {
		t.Fatal("expected reject")
	}
}

func TestCheckRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	g := New(root)
	evil := filepath.Join(root, "..", "outside.txt")
	if _, err := g.Check(evil); err == nil {
		t.Fatal("expected reject traversal")
	}
}
