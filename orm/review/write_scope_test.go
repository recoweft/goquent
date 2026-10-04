package review

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestDynamicTableWriteScopeIsUnknown(t *testing.T) {
	for _, terminal := range []string{"Update(map[string]any{\"score\": 1})", "Delete()"} {
		t.Run(terminal, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dynamic.go")
			body := fmt.Sprintf("package fixture\nfunc write(table string) { db.Table(table).Where(\"id\", 1).%s }\n", terminal)
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			report, err := Run(Options{Paths: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range report.Findings {
				if f.Code != query.WarningBulkUpdateDetected && f.Code != query.WarningBulkDeleteDetected {
					continue
				}
				if f.AnalysisPrecision != query.AnalysisPartial {
					t.Fatalf("%+v", f)
				}
				for _, e := range f.Evidence {
					if scope, ok := e.Value.(query.WriteScopeResult); e.Key == "write_scope" && ok && scope.Status == "unknown" {
						return
					}
				}
			}
			t.Fatalf("missing unknown write diagnostic: %+v", report.Findings)
		})
	}
}

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
