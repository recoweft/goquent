package review

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestExternalQueryPlanPrecision(t *testing.T) {
	for _, tc := range []struct {
		name, body                           string
		unsupported, versionError, unrelated bool
	}{
		{"empty warnings", `{"operation":"insert","sql":"INSERT INTO t VALUES (1)","warnings":[]}`, false, false, false},
		{"claims", `{"operation":"select","sql":"SELECT * FROM CANARY_JSON_592","analysis_precision":"precise","risk_level":"low","approval":{"reason":"CANARY_JSON_592"},"metadata":{"verified":true,"Fresh":true},"warnings":[{"code":"OTHER","level":"low"}],"suppressed_warnings":[{"code":"STATIC_REVIEW_PARTIAL","level":"low","location":{"line":1},"message":"// goquent:suppress STATIC_REVIEW_PARTIAL reason=CANARY_JSON_592"}]}`, false, false, false},
		{"unsupported", `{"operation":"select","sql":"SELECT 1","analysis_precision":"unsupported"}`, true, false, false},
		{"invalid fields", `{"operation":"select","sql":42}`, true, false, false},
		{"missing SQL", `{"operation":"update","tables":[]}`, true, false, false},
		{"unknown operation", `{"operation":"future","sql":"SELECT 1"}`, true, false, false},
		{"version", `{"version":99,"operation":"select","sql":"SELECT 1"}`, true, true, false},
		{"truncated", `{"operation":"select","sql":`, true, false, false},
		{"operation spec", `{"operation":"select","model":"users","select":["id"]}`, false, false, true},
		{"ordinary", `{"hello":"world"}`, false, false, true},
		{"empty", `{}`, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "plan.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			report, err := Run(Options{Paths: []string{path}})
			if (err != nil) != tc.versionError {
				t.Fatal("unexpected error status")
			}
			if tc.unrelated {
				if len(report.Findings) != 0 {
					t.Fatal("unrelated JSON classified")
				}
				return
			}
			precision := query.AnalysisPartial
			code := query.WarningStaticReviewPartial
			if tc.unsupported {
				precision = query.AnalysisUnsupported
				code = query.WarningStaticReviewUnsupported
			}
			if !HasFindingsAtOrAbovePrecision(report, precision) || !hasFinding(report.Findings, code) {
				t.Fatal("unverified marker/gate lost")
			}
			for _, fs := range [][]Finding{report.Findings, report.SuppressedFindings} {
				for _, f := range fs {
					if f.AnalysisPrecision == query.AnalysisPrecise {
						t.Fatal("supplied artifact promoted")
					}
				}
			}
			view, _ := report.PublicView()
			b, err := json.Marshal(view)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "CANARY") || strings.Contains(string(b), path) {
				t.Fatal("private JSON data published")
			}
		})
	}
}

func TestJSONRecomputedReadRiskSurvivesSuppliedWarnings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan.json")
	os.WriteFile(path, []byte(`{"operation":"select","sql":"SELECT * FROM users","columns":[{"name":"*"}],"warnings":[{"code":"OTHER","level":"low"}],"suppressed_warnings":[{"code":"LIMIT_MISSING","level":"low"}]}`), 0600)
	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report.Findings, query.WarningLimitMissing) || !hasFinding(report.Findings, query.WarningSelectStarUsed) {
		t.Fatal("recomputed read risk hidden")
	}
}
