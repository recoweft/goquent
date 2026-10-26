//go:build linux

package review

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSecurityFileSnapshotAndExplicitInput(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	file := filepath.Join(outside, "input.sql")
	if os.WriteFile(file, []byte("SELECT 1;"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	// ReadCIInput accepts caller-selected absolute paths; it has no fixed root.
	if b, err := ReadCIInput(file, 32); err != nil || string(b) != "SELECT 1;" {
		t.Fatal("explicit path refused")
	}
	for _, kind := range []string{"symlink", "fifo", "hardlink"} {
		p := filepath.Join(root, "input.sql")
		var err error
		switch kind {
		case "symlink":
			err = os.Symlink(file, p)
		case "fifo":
			err = syscall.Mkfifo(p, 0600)
		case "hardlink":
			err = os.Link(file, p)
		}
		if err != nil {
			t.Fatal("fixture creation failed")
		}
		b, readErr := ReadCIInput(p, 32)
		tree, treeErr := readTree(context.Background(), root)
		if kind == "hardlink" {
			if readErr != nil || treeErr != nil || string(b) != "SELECT 1;" || string(tree["input.sql"]) != string(b) {
				t.Fatal("regular alias semantics changed")
			}
		} else if readErr == nil || treeErr == nil {
			t.Fatal("static special file accepted")
		}
		if os.Remove(p) != nil {
			t.Fatal("fixture cleanup failed")
		}
	}
	for _, p := range []string{"../a", "/a", `C:/a`, `C:\a`, `\\server\a`, "a/../b"} {
		if exactPath(p) {
			t.Fatal("diff path accepted")
		}
	}
}
