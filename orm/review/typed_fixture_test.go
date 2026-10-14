package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTypedLegacy retains the original snippet and creates a buildable sibling
// with the same physical call/comment lines. Legacy any methods were not Go APIs.
func writeTypedLegacy(t *testing.T, path, source string) {
	t.Helper()
	original := source
	if err := os.WriteFile(path+".legacy.txt", []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	imports := []string{}
	if strings.Contains(source, "db any") {
		source = strings.ReplaceAll(source, "db any", "db *orm.DB")
		imports = append(imports, `"github.com/recoweft/goquent/orm"`)
	}
	if strings.Contains(source, "q any") {
		source = strings.ReplaceAll(source, "q any", "q *query.Query")
		imports = append(imports, `"github.com/recoweft/goquent/orm/query"`)
	}
	if len(imports) > 0 {
		source = strings.Replace(source, "\n", "; import ("+strings.Join(imports, ";")+")\n", 1)
	}
	source = strings.ReplaceAll(source, "s := `", "_ = `")
	source = strings.ReplaceAll(source, "_, _, err = ParseInlineSuppression(", "_, _, _ = query.ParseInlineSuppression(")
	if strings.Contains(source, "query.ParseInlineSuppression") && !strings.Contains(source, `"github.com/recoweft/goquent/orm/query"`) {
		source = strings.Replace(source, "; import (", "; import (\"github.com/recoweft/goquent/orm/query\";", 1)
	}
	fixtureModule(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	legacy, _ := Run(Options{Paths: []string{path}})
	if !hasFinding(legacy.Findings, "STATIC_REVIEW_UNSUPPORTED") {
		t.Fatal("legacy type-invalid input must retain unsupported evidence")
	}
	for _, f := range legacy.Findings {
		if f.AnalysisPrecision == "precise" {
			t.Fatal("legacy type-invalid input promoted")
		}
	}
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}
func fixtureModule(t *testing.T, dir string) {
	t.Helper()
	root := moduleRoot(".")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	mod, err := os.ReadFile(filepath.Join(abs, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Replace(string(mod), "module github.com/recoweft/goquent", "module fixture.example/review", 1) + "\nrequire github.com/recoweft/goquent v0.0.0\nreplace github.com/recoweft/goquent => " + filepath.ToSlash(abs) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	sum, err := os.ReadFile(filepath.Join(abs, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0600); err != nil {
		t.Fatal(err)
	}
}
