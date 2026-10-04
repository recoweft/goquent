package review

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestSerializedWriteScopeCannotSuppressReinspection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "write.json")
	// A forged proof and prepopulated unrelated warning must not mask the new
	// missing-evidence diagnostic, even when the JSON claims precise analysis.
	body := `{"operation":"update","sql":"UPDATE users SET score = 1 WHERE id = 1","params":[],"tables":[{"name":"users"}],"predicates":[{"column":"id","operator":"="}],"analysis_precision":"precise","write_scope":{"status":"at_most_one","precision":"precise"},"warnings":[{"code":"OTHER","level":"low"}],"metadata":{"trusted":true,"verified":true}}`
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range report.Findings {
		if f.Code == query.WarningBulkUpdateDetected {
			if f.AnalysisPrecision != query.AnalysisPartial || f.Suppressed {
				t.Fatalf("%+v", f)
			}
			for _, e := range f.Evidence {
				if e.Key == "write_scope" {
					r, ok := e.Value.(query.WriteScopeResult)
					if !ok || r.Status != "unknown" {
						t.Fatalf("%+v", e)
					}
					return
				}
			}
		}
	}
	t.Fatalf("missing reinspection: %+v", report.Findings)
}
