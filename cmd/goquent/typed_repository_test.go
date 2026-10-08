package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestTypedRepositoryCLIArtifactBoundary(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL", "TEAMCITY_VERSION"} {
		t.Setenv(key, "")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "source.json")
	dest := filepath.Join(dir, "repository.go")
	m := typedfixture.Manifest("postgres")
	m.SchemaFingerprint = "private-cli-canary"
	writeJSON(t, source, m)
	args := []string{"manifest", "repository", "--typed", "--manifest", source, "--projection", "Identity=id,segment", "--unsafe-local-output", dest}
	var out, stderr bytes.Buffer
	if code := run(args, &out, &stderr); code != 0 {
		t.Fatal("typed generation refused")
	}
	for _, s := range []string{"private-cli-canary", "gq08_records", "package", "fingerprint"} {
		if bytes.Contains(out.Bytes(), []byte(s)) || bytes.Contains(stderr.Bytes(), []byte(s)) {
			t.Fatal("public artifact disclosure")
		}
	}
	saved, e := os.ReadFile(dest)
	if e != nil || !bytes.Contains(saved, []byte("private-cli-canary")) {
		t.Fatal("explicit local source missing")
	}
	info, e := os.Stat(dest)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatal("artifact permissions changed")
	}
	if code := run(args, &out, &stderr); code == 0 {
		t.Fatal("overwrote existing artifact")
	}
	after, _ := os.ReadFile(dest)
	if !bytes.Equal(saved, after) {
		t.Fatal("artifact changed")
	}
	t.Setenv("CI", "true")
	args[len(args)-1] = filepath.Join(dir, "ci.go")
	if code := run(args, &out, &stderr); code == 0 {
		t.Fatal("CI artifact export accepted")
	}
	if _, e = os.Stat(args[len(args)-1]); !os.IsNotExist(e) {
		t.Fatal("CI created source artifact")
	}
	for _, alias := range []string{"repository", "repo", "skeleton"} {
		if code := run([]string{"manifest", alias, "--typed", "--manifest", source, "--projection", "Identity=id"}, &out, &stderr); code != 0 {
			t.Fatal("existing alias changed")
		}
	}
	if code := run([]string{"manifest", "repo", "--manifest", source, "--projection", "Identity=id"}, &out, &stderr); code == 0 {
		t.Fatal("projection silently ignored in legacy mode")
	}
}
