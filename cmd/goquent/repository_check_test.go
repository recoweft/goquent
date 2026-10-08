package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryCheckPublicBoundary(t *testing.T) {
	const canary = "private-path-and-source-canary"
	for _, args := range [][]string{{"--config", "../../tests/typedfixture/repositories.json"}, {"--config", filepath.Join(t.TempDir(), canary)}, {"--" + canary}, {"--config"}, {"--config", canary, canary}} {
		var out, errout bytes.Buffer
		result := runManifest(append([]string{"check-repositories"}, args...), &out, &errout)
		if strings.Contains(out.String()+errout.String(), canary) {
			t.Fatal("checker disclosed source")
		}
		if len(args) > 1 && strings.Contains(args[1], "repositories.json") {
			if result != 0 {
				t.Fatal("registered fixture check failed")
			}
		} else if result == 0 {
			t.Fatal("invalid check accepted")
		}
	}
	path := filepath.Join(t.TempDir(), canary)
	if os.WriteFile(path, []byte(canary), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	var out, errout bytes.Buffer
	if runRepositoryCheck([]string{"--config", path}, &out, &errout) == 0 || strings.Contains(out.String()+errout.String(), canary) {
		t.Fatal("parse error boundary failed")
	}
}
