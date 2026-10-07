package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/recoweft/goquent/orm/publicoutput"
)

// saveLocalArtifact is an explicit data export, never a stdout fallback.
// O_EXCL rejects existing files (including symlinks); permissions are 0600.
func saveLocalArtifact(path string, data []byte) error {
	if path == "" || path == "-" {
		return publicoutput.ErrOutput
	}
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL", "TEAMCITY_VERSION"} {
		v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		if v != "" && v != "false" && v != "0" {
			return publicoutput.ErrOutput
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return publicoutput.ErrOutput
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return publicoutput.ErrOutput
	}
	for _, prefix := range []string{"/dev", "/proc", "/sys"} {
		if parent == prefix || strings.HasPrefix(parent, prefix+string(os.PathSeparator)) {
			return publicoutput.ErrOutput
		}
	}
	f, err := os.OpenFile(filepath.Join(parent, filepath.Base(abs)), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return publicoutput.ErrOutput
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		_ = f.Close()
		return publicoutput.ErrOutput
	}
	n, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil || n != len(data) {
		return publicoutput.ErrOutput
	}
	return nil
}
