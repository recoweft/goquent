//go:build linux

package main

import (
	"context"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/publicoutput"
)

func TestSecurityFileExportAliases(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL", "TEAMCITY_VERSION"} {
		t.Setenv(key, "")
	}
	dir, outside := t.TempDir(), t.TempDir()
	original := filepath.Join(outside, "original")
	alias := filepath.Join(dir, "alias")
	if os.WriteFile(original, []byte("original"), 0600) != nil || os.Link(original, alias) != nil {
		t.Fatal("fixture creation failed")
	}
	if saveLocalArtifact(alias, []byte("replacement")) == nil {
		t.Fatal("hardlink overwritten")
	}
	if b, err := os.ReadFile(original); err != nil || string(b) != "original" {
		t.Fatal("original changed")
	}
	parent := filepath.Join(dir, "parent")
	if os.Symlink(outside, parent) != nil {
		t.Fatal("parent fixture failed")
	}
	// Explicit caller-selected parents are resolved; there is no checkout root.
	if saveLocalArtifact(filepath.Join(parent, "new"), []byte("fixture")) != nil {
		t.Fatal("explicit resolved parent refused")
	}
	info, err := os.Stat(filepath.Join(outside, "new"))
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		t.Fatal("export mode changed")
	}
}

func TestSecurityFilePartialExport(t *testing.T) {
	dir := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal("helper unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "-test.run=^TestSecurityFileExportHelper$")
	cmd.Env = []string{"GQ_EXPORT_FIXTURE=" + filepath.Join(dir, "partial")}
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if cmd.Run() != nil {
		t.Fatal("partial export probe failed")
	}
	b, err := os.ReadFile(filepath.Join(dir, "partial"))
	info, statErr := os.Stat(filepath.Join(dir, "partial"))
	if err != nil || statErr != nil || string(b) != "fixt" || info.Mode().Perm() != 0600 {
		t.Fatal("partial file contract changed")
	}
}

func TestSecurityFileExportHelper(t *testing.T) {
	p := os.Getenv("GQ_EXPORT_FIXTURE")
	if p == "" {
		return
	}
	// Per-process limit on a disposable fixture: no full disk or host setting change.
	signal.Ignore(syscall.SIGXFSZ)
	if syscall.Setrlimit(syscall.RLIMIT_FSIZE, &syscall.Rlimit{Cur: 4, Max: 4}) != nil {
		t.Fatal("fixture limit unavailable")
	}
	if saveLocalArtifact(p, []byte("fixture-data")) != publicoutput.ErrOutput {
		t.Fatal("partial write accepted")
	}
}
