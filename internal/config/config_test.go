package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserPathsRejectsRelativeXDG(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", "relative")
	if _, err := UserPaths(); err == nil {
		t.Fatal("accepted relative XDG data directory")
	}
}

func TestAtomicWritePreservesSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "target")
	link := filepath.Join(dir, "config")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWrite(link, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink not preserved: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "new" {
		t.Fatalf("target content %q: %v", data, err)
	}
}
