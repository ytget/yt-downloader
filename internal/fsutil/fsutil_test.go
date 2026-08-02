package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"":                    "download",
		"  hello world  ":     "hello_world",
		`a/b\c:d*e?f"g<h>i|j`: "b_c_d_e_f_g_h_i_j",
		"CON":                 "_CON",
		"con.txt":             "_con.txt",
		"nul":                 "_nul",
		"normal-name":         "normal-name",
		"dots...":             "dots",
		"../escape":           "escape",
	}
	for in, want := range cases {
		got := SanitizeFilename(in)
		if got != want {
			t.Errorf("SanitizeFilename(%q)=%q want %q", in, got, want)
		}
	}

	long := strings.Repeat("a", 500)
	got := SanitizeFilename(long)
	if len(got) > MaxFilenameBytes {
		t.Fatalf("expected truncation <= %d, got %d", MaxFilenameBytes, len(got))
	}
}

func TestEnsureWritableDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dl")
	if err := EnsureWritableDir(dir); err != nil {
		t.Fatalf("EnsureWritableDir: %v", err)
	}
	// file path should fail
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWritableDir(f); err == nil {
		t.Fatal("expected error for file path")
	}
}

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	if err := AtomicWriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "hello" {
		t.Fatalf("got %q", raw)
	}
	// overwrite atomically
	if err := AtomicWriteFile(path, []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if string(raw) != "world" {
		t.Fatalf("got %q", raw)
	}
	// no leftover temps
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp %s", e.Name())
		}
	}
}

func TestResolveCollision(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	skip, err := ResolveCollision(target, CollisionSkip)
	if err != nil || !skip.Skip || skip.Path != target {
		t.Fatalf("skip: %+v err=%v", skip, err)
	}

	ren, err := ResolveCollision(target, CollisionRename)
	if err != nil || ren.Skip || ren.Path == target {
		t.Fatalf("rename: %+v err=%v", ren, err)
	}
	if _, err := os.Stat(ren.Path); !os.IsNotExist(err) {
		t.Fatalf("rename path should not exist yet: %v", err)
	}
	if !strings.Contains(filepath.Base(ren.Path), "(1)") {
		t.Fatalf("expected (1) in name, got %s", ren.Path)
	}

	over, err := ResolveCollision(target, CollisionOverwrite)
	if err != nil || over.Skip || over.Path != target {
		t.Fatalf("overwrite: %+v err=%v", over, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatal("overwrite should remove existing file")
	}
}

func TestCheckFreeSpaceSmoke(t *testing.T) {
	dir := t.TempDir()
	// absurdly small requirement always ok
	if err := CheckFreeSpace(dir, 1); err != nil {
		t.Fatalf("CheckFreeSpace: %v", err)
	}
	// absurdly large may fail depending on disk; just ensure no panic
	_ = CheckFreeSpace(dir, 1<<62)
}
