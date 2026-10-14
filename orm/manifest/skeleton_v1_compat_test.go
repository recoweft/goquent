package manifest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// The byte-preserved PR1 fixtures remain buildable against the v2 library.
// They are deliberately not represented as current regenerated v2 output.
func TestTypedV1SourceCompatibility(t *testing.T) {
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal("fixture root unavailable")
	}
	dir := t.TempDir()
	module := "module v1compat\n\ngo 1.25.0\n\nrequire github.com/recoweft/goquent v0.0.0\nreplace github.com/recoweft/goquent => " + filepath.ToSlash(root) + "\n"
	if os.WriteFile(filepath.Join(dir, "go.mod"), []byte(module), 0600) != nil {
		t.Fatal("fixture module unavailable")
	}
	for _, name := range []string{"fixture.go", "mysql_generated.go", "postgres_generated.go", "tenant_generated.go"} {
		b, e := os.ReadFile(filepath.Join(root, "tests/typedfixture/testdata/v1", name))
		if e != nil || os.WriteFile(filepath.Join(dir, name), b, 0600) != nil {
			t.Fatal("preserved fixture unavailable")
		}
	}
	cmd := exec.Command("go", "test", "-mod=mod", "-run=^$", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if _, e = cmd.CombinedOutput(); e != nil {
		t.Fatal("preserved v1 fixture compilation failed; diagnostics omitted")
	}
}
