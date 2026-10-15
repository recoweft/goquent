package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/orm/review"
)

func TestReviewCLIPlannerEvidence(t *testing.T) {
	dir := t.TempDir()
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	mod, e := os.ReadFile(filepath.Join(root, "go.mod"))
	if e != nil {
		t.Fatal(e)
	}
	mod = []byte(strings.Replace(string(mod), "module github.com/recoweft/goquent", "module fixture.example/cli", 1) + "\nrequire github.com/recoweft/goquent v0.0.0\nreplace github.com/recoweft/goquent => " + root + "\n")
	if e = os.WriteFile(filepath.Join(dir, "go.mod"), mod, 0600); e != nil {
		t.Fatal(e)
	}
	sum, e := os.ReadFile(filepath.Join(root, "go.sum"))
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0600); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(dir, "sample.go")
	if e = os.WriteFile(path, []byte(`package sample
import "github.com/recoweft/goquent/orm"
func run(db *orm.DB) { var rows []map[string]any; _ = db.Table("CLI_SECRET_TABLE").Select("id").Where("id",1).Limit(2).Get(&rows) }
`), 0600); e != nil {
		t.Fatal(e)
	}
	var out, errs bytes.Buffer
	producer := func(s *review.Session) {
		refs := s.References()
		if len(refs) != 1 {
			t.Fatalf("refs %d", len(refs))
		}
		q := query.NewWithSettings(nil, "CLI_SECRET_TABLE", driver.MySQLDialect{}, query.Settings{}).Select("id").Where("id", 1).Limit(2)
		r, e := q.ReviewReceipt(context.Background(), s.Token())
		if e != nil {
			t.Fatal(e)
		}
		s.Record(refs[0], r)
	}
	status := runReviewEvidence([]string{"--format", "json-v2", "--fail-on-precision", "partial", path}, &out, &errs, producer, "mysql", query.Settings{})
	if status != 1 {
		t.Fatalf("precision gate %d", status)
	}
	v, e := review.DecodeReportViewV2(out.Bytes())
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Operations) != 1 || v.Operations[0].Correspondence.Arguments != analysis.Matched || v.Operations[0].Detail.Policy != "unknown" {
		t.Fatalf("positive CLI: %+v", v.Operations)
	}
	if strings.Contains(out.String(), "CLI_SECRET_TABLE") || strings.Contains(out.String(), path) || errs.Len() != 0 {
		t.Fatal("CLI disclosure")
	}
	for _, input := range []string{`{"operation":"select","sql":"SELECT id FROM users","analysis_precision":"precise","verified":true,"receipt":{"hash":"same","id":"same"}}`, `{"operation":"select","sql":"SELECT id FROM users","version":99}`} {
		p := filepath.Join(dir, "external.json")
		if e = os.WriteFile(p, []byte(input), 0600); e != nil {
			t.Fatal(e)
		}
		out.Reset()
		errs.Reset()
		status = run([]string{"review", "--format", "json-v2", "--fail-on-precision", "partial", p}, &out, &errs)
		if status == 0 {
			t.Fatal("external JSON promoted")
		}
		if out.Len() > 0 {
			v, e = review.DecodeReportViewV2(out.Bytes())
			if e != nil {
				t.Fatal(e)
			}
			for _, o := range v.Operations {
				if o.Correspondence.Source == analysis.Matched {
					t.Fatal("standalone trusted JSON")
				}
			}
		}
	}
}

func TestReviewCLISourceReportBoundary(t *testing.T) {
	for _, prefix := range []string{"", `"version":0,`, `"version":1,`, `"version":99,`, `"version":1,"version":1,`} {
		for _, suffix := range []string{`}`, `,"operations":[]}`, `,"operations":[`} {
			path := filepath.Join(t.TempDir(), "report.json")
			body := `{` + prefix + `"findings":[{"code":"DELETE_WITHOUT_WHERE","analysis_precision":"precise","suppressed":true,"location":{"file":"CLI_REPORT_CANARY","line":88},"message":"// goquent:suppress STATIC_REVIEW_UNSUPPORTED reason=CLI_REPORT_CANARY"}]` + suffix
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			for _, format := range []string{"json", "json-v2"} {
				for _, threshold := range []string{"partial", "unsupported"} {
					var out, errs bytes.Buffer
					status := run([]string{"review", "--format", format, "--fail-on-precision", threshold, path}, &out, &errs)
					if status != 1 || errs.Len() != 0 {
						t.Fatalf("expected precision refusal, got %d", status)
					}
					var findings []review.FindingView
					if format == "json" {
						v, err := review.DecodeReportView(out.Bytes())
						if err != nil {
							t.Fatal(err)
						}
						findings = v.Findings
					} else {
						v, err := review.DecodeReportViewV2(out.Bytes())
						if err != nil {
							t.Fatal(err)
						}
						findings = v.Findings
					}
					if len(findings) != 1 || findings[0].Code != query.WarningStaticReviewUnsupported || findings[0].Precision != "unsupported" || findings[0].Line != 1 || findings[0].Suppressed {
						t.Fatal("report claims imported or unsupported marker lost")
					}
					if strings.Contains(out.String(), "CLI_REPORT_CANARY") || strings.Contains(out.String(), path) {
						t.Fatal("CLI report disclosure")
					}
				}
			}
			// The medium-risk unsupported diagnostic does not silently change risk gates.
			var out, errs bytes.Buffer
			if status := run([]string{"review", "--fail-on", "high", path}, &out, &errs); status != 0 || !strings.Contains(out.String(), query.WarningStaticReviewUnsupported) {
				t.Fatal("risk-only compatibility changed")
			}
		}
	}
}
