package review

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/query"
)

func TestDynamicRawReasonAndFallback(t *testing.T) {
	dir := t.TempDir()
	fixtureModule(t, dir)
	path := filepath.Join(dir, "private_source.go")
	source := `package fixture
import (
 "context"
 "database/sql"
 g "github.com/recoweft/goquent/orm"
 q "github.com/recoweft/goquent/orm/query"
)
type unrelated struct{}
func (unrelated) Exec(string) {}
func example(db *sql.DB, wrapped *g.DB, executor g.Executor, text string, ctx context.Context) {
 db.Exec(text) // dynamic
 db.ExecContext(ctx, text) // dynamic
 (*sql.DB).Exec(db, text) // dynamic
 (*sql.DB).ExecContext(db, ctx, text) // dynamic
 bound := db.Exec; bound(text) // dynamic
 db.Prepare(text) // dynamic
 db.PrepareContext(ctx, text) // dynamic
 wrapped.QueryRowE(ctx, text) // dynamic
 executor.ExecContext(ctx, text) // dynamic
 q.NewRawPlan(text) // dynamic
 db.Exec("SELECT PRIVATE_DYNAMIC_CANARY") // literal
 unrelated{}.Exec(text) // unrelated
}
`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal("fixture write failed")
	}
	loaded := loadReviewTypes(nil, []string{path})
	if tf := loaded[path]; tf == nil || tf.incomplete || tf.info == nil || tf.pkg == nil {
		t.Fatal("fixture must type check")
	}
	r, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal("review failed")
	}
	byLine := map[int][]Finding{}
	for _, f := range r.Findings {
		if f.Location != nil {
			byLine[f.Location.Line] = append(byLine[f.Location.Line], f)
		}
	}
	dynamic := 0
	for i, line := range strings.Split(source, "\n") {
		fs := byLine[i+1]
		switch {
		case strings.HasSuffix(line, "// dynamic"):
			dynamic++
			if len(fs) != 1 {
				t.Fatal("dynamic finding count changed")
			}
			f := fs[0]
			if f.Code != query.WarningStaticReviewUnsupported || f.Level != query.RiskMedium || f.AnalysisPrecision != query.AnalysisUnsupported || !slices.Equal(f.Detail.Reasons, []string{"dynamic_value"}) || f.Detail.SQL != analysis.Unsupported || f.Detail.Policy != analysis.Unknown {
				t.Fatal("dynamic classification changed")
			}
			if f.Message != "dynamic SQL cannot be reconstructed" || f.Hint != "provide a supported SQL literal and review unsupported inputs" {
				t.Fatal("dynamic explanation changed")
			}
			found := false
			for _, op := range r.Operations {
				if locationKey(op.Location) != locationKey(f.Location) {
					continue
				}
				found = true
				if !slices.Equal(op.Detail.Reasons, f.Detail.Reasons) || op.Detail.SQL != analysis.Unsupported || op.Detail.Precision() != "unsupported" || op.Detail.Policy != analysis.Unknown || !slices.Contains(op.Detail.Missing, "evidence_missing") || !slices.Contains(op.Detail.Missing, "runtime_policy") {
					t.Fatal("fallback changed evidence")
				}
			}
			if !found {
				t.Fatal("fallback omitted operation")
			}
		case strings.HasSuffix(line, "// literal"):
			if !hasFinding(fs, query.WarningRawSQLUsed) {
				t.Fatal("literal warning lost")
			}
			for _, f := range fs {
				if slices.Contains(f.Detail.Reasons, "dynamic_value") || hasTypeReason(f.Detail.Reasons) {
					t.Fatal("literal misclassified")
				}
			}
		case strings.HasSuffix(line, "// unrelated"):
			if len(fs) != 0 {
				t.Fatal("recognition expanded")
			}
		}
	}
	if dynamic != 10 || len(r.Operations) != 11 || !HasFindingsAtOrAbovePrecision(r, query.AnalysisUnsupported) {
		t.Fatal("unsupported population lost")
	}
	v1, err := r.PublicView()
	if err != nil {
		t.Fatal("v1 failed")
	}
	v2, err := r.PublicViewV2()
	if err != nil {
		t.Fatal("v2 failed")
	}
	var sarif bytes.Buffer
	if WriteSARIF(&sarif, r) != nil {
		t.Fatal("SARIF failed")
	}
	sourceJSON, err := json.Marshal(r)
	if err != nil {
		t.Fatal("source encoding failed")
	}
	for _, b := range [][]byte{sourceJSON, []byte(v1.String()), []byte(v2.String()), sarif.Bytes()} {
		// Source locations remain private API data; the diagnostic never copies SQL.
		if bytes.Contains(b, []byte("PRIVATE_DYNAMIC_CANARY")) {
			t.Fatal("SQL leaked into diagnostic")
		}
	}
	for _, b := range []string{v1.String(), v2.String(), sarif.String()} {
		if strings.Contains(b, path) || strings.Contains(b, "private_source") || strings.Contains(b, "type_unavailable") {
			t.Fatal("public disclosure or stale reason")
		}
	}
	for _, b := range []string{v2.String(), sarif.String()} {
		if !strings.Contains(b, "dynamic_value") {
			t.Fatal("public reason omitted")
		}
	}
}

func TestDynamicRawIncompleteAndInvalid(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"missing_import", `import "missing.invalid/package"; func example(){ packageMissing.Exec("PRIVATE_CANARY") }`},
		{"invalid_arguments", `import "database/sql"; func example(db *sql.DB){ db.Exec(); db.Exec(123) }`},
		{"dynamic_incomplete", `import "database/sql"; func example(db *sql.DB, text string){ db.Exec(text); missing_PRIVATE_CANARY() }`},
		{"parse_failure", `func example( {`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fixtureModule(t, dir)
			path := filepath.Join(dir, "source.go")
			if os.WriteFile(path, []byte("package fixture\n"+tc.body), 0600) != nil {
				t.Fatal("fixture write failed")
			}
			r, err := Run(Options{Paths: []string{path}})
			if (err != nil) != (tc.name == "parse_failure") {
				t.Fatal("parse error boundary changed")
			}
			findings, operations := 0, 0
			for _, f := range r.Findings {
				if hasTypeReason(f.Detail.Reasons) {
					findings++
				}
				if slices.Contains(f.Detail.Reasons, "dynamic_value") {
					t.Fatal("incomplete type relabeled")
				}
			}
			for _, op := range r.Operations {
				if hasTypeReason(op.Detail.Reasons) {
					operations++
				}
				if slices.Contains(op.Detail.Reasons, "dynamic_value") {
					t.Fatal("incomplete fallback relabeled")
				}
			}
			if findings == 0 || operations == 0 || !HasFindingsAtOrAbovePrecision(r, query.AnalysisUnsupported) {
				t.Fatal("type failure lost")
			}
		})
	}
}

func TestDiagnosticUnqualifiedRawMatcher(t *testing.T) {
	dir := t.TempDir()
	if os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module github.com/recoweft/goquent/orm/query\ngo 1.25.0\n"), 0600) != nil {
		t.Fatal("module write failed")
	}
	path := filepath.Join(dir, "source.go")
	source := `package query
func NewRawPlan(string, ...any) {}
func example(text string) { NewRawPlan(text, 1) }
`
	if os.WriteFile(path, []byte(source), 0600) != nil {
		t.Fatal("source write failed")
	}
	tf := loadReviewTypes(nil, []string{path})[path]
	if tf == nil || tf.incomplete {
		t.Fatal("unqualified fixture unavailable")
	}
	fs, err := reviewTypedGo(newReviewContext(Options{}), path, tf)
	if err != nil || len(fs) != 1 || !slices.Equal(fs[0].Detail.Reasons, []string{"dynamic_value"}) {
		t.Fatal("unqualified recognition changed")
	}
	if !diagnosticDynamicRaw(tf, fs[0].Location) || !diagnosticUnqualifiedRaw(tf, fs[0].Location) {
		t.Fatal("unqualified diagnostic cause not matched")
	}
}
