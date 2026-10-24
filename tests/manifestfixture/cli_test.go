package manifestfixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The shell publication canary uses a fake go executable. Keep actual CLI
// execution in the Go suite, independently of that wrapper's mock regressions.
func TestManifestEvidenceRealCLI(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal("repository root unavailable")
	}
	out := t.TempDir()
	if os.Chmod(out, 0700) != nil {
		t.Fatal("private directory failed")
	}
	cli := filepath.Join(out, "goquent")
	build := exec.Command("go", "build", "-o", cli, "./cmd/goquent")
	build.Dir = root
	if _, err := build.CombinedOutput(); err != nil {
		t.Fatal("CLI build failed; details omitted")
	}
	const script = `import json, os, sys
from pathlib import Path
sys.path.insert(0, 'scripts/release_validation')
import run
os.umask(0o077)
root, out = Path.cwd(), Path(sys.argv[1])
collector = run.Collector(root, out)
row = collector.manifest_evidence()
assert row['status'] == 'pass'
assert row['counts'] == dict(passed=24, failed=0)
assert row['cases']['mysql/database-disabled']['status'] == 'skip'
assert row['cases']['postgres/missing-code']['exit'] == 1
for path in out.rglob('*'):
    assert path.stat().st_mode & 0o077 == 0
`
	// The new private binary is executable by its owner only.
	if os.Chmod(cli, 0700) != nil {
		t.Fatal("private binary mode failed")
	}
	check := exec.Command("python3", "-c", script, out)
	check.Dir = root
	if _, err := check.CombinedOutput(); err != nil {
		t.Fatal("finite real CLI matrix failed; details omitted")
	}
}
