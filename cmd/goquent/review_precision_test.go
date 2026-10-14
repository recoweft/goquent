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
