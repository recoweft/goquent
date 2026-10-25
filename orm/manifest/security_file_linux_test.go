//go:build linux

package manifest

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSecurityFileRepositoryRoot(t *testing.T) {
	base := t.TempDir()
	rootPath, outside := filepath.Join(base, "root"), filepath.Join(base, "outside")
	for _, dir := range []string{rootPath, outside} {
		if os.Mkdir(dir, 0700) != nil || os.WriteFile(filepath.Join(dir, "input"), []byte(filepath.Base(dir)), 0600) != nil {
			t.Fatal("fixture creation failed")
		}
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		t.Fatal("root unavailable")
	}
	defer root.Close()
	r := repositoryReader{root: root}
	if b, err := r.read("input", 32); err != nil || string(b) != "root" {
		t.Fatal("normal read failed")
	}
	for _, p := range []string{"../outside/input", filepath.Join(outside, "input"), "./input", "a/../input", `C:\input`, `\\server\input`} {
		if _, err := r.read(p, 32); err == nil {
			t.Fatal("nonlocal or noncanonical path accepted")
		}
	}
	if os.Symlink(outside, filepath.Join(rootPath, "link")) != nil || syscall.Mkfifo(filepath.Join(rootPath, "fifo"), 0600) != nil {
		t.Fatal("fixture creation failed")
	}
	for _, p := range []string{"link/input", "fifo", "."} {
		if _, err := r.read(p, 32); err == nil {
			t.Fatal("nonregular or symlink input accepted")
		}
	}
	// Hardlinks are regular inode aliases, not a provenance guarantee.
	if os.Link(filepath.Join(outside, "input"), filepath.Join(rootPath, "hard")) != nil {
		t.Fatal("hardlink fixture failed")
	}
	if b, err := r.read("hard", 32); err != nil || string(b) != "outside" {
		t.Fatal("hardlink semantics changed")
	}
	// Deterministic replacement AFTER root acquisition: the descriptor stays pinned.
	if os.Rename(rootPath, rootPath+"-original") != nil || os.Symlink(outside, rootPath) != nil {
		t.Fatal("root replacement failed")
	}
	if b, err := r.read("input", 32); err != nil || string(b) != "root" {
		t.Fatal("opened root lost identity")
	}
	if b, err := os.ReadFile(filepath.Join(rootPath, "input")); err != nil || string(b) != "outside" {
		t.Fatal("replacement negative control failed")
	}
}
