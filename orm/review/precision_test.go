package review

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

func precisionFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	fixtureModule(t, dir)
	path := filepath.Join(dir, "sample.go")
	src := `package fixture
import "github.com/recoweft/goquent/orm"
func sample(db *orm.DB) { var rows []map[string]any; _ = db.Table("PRIVATE_TABLE_CANARY").Select("id").Where("id", 1).Limit(2).Get(&rows) }
`
	if err := os.WriteFile(path, []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func receiptQuery() *query.Query {
	return query.NewWithSettings(nil, "PRIVATE_TABLE_CANARY", driver.MySQLDialect{}, query.Settings{}).Select("id").Where("id", 1).Limit(2)
}
func TestPlannerCorrespondenceSession(t *testing.T) {
	for _, mode := range []string{"match", "arguments", "target", "dialect", "structure", "mutation", "source", "duplicate", "foreign", "zero", "id_collision", "context"} {
		t.Run(mode, func(t *testing.T) {
			path := precisionFixture(t)
			r, err := Run(Options{Paths: []string{path}, EvidenceDialect: "mysql", Evidence: func(s *Session) {
				refs := s.References()
				if len(refs) != 1 {
					t.Fatalf("operations: %d", len(refs))
				}
				ref := refs[0]
				q := receiptQuery()
				switch mode {
				case "arguments":
					q = query.NewWithSettings(nil, "PRIVATE_TABLE_CANARY", driver.MySQLDialect{}, query.Settings{}).Select("id").Where("id", int64(1)).Limit(2)
				case "target":
					q = query.NewWithSettings(nil, "OTHER", driver.MySQLDialect{}, query.Settings{}).Select("id").Where("id", 1).Limit(2)
				case "dialect":
					q = query.NewWithSettings(nil, "PRIVATE_TABLE_CANARY", driver.PostgresDialect{}, query.Settings{}).Select("id").Where("id", 1).Limit(2)
				case "structure":
					q.Limit(3)
				case "context":
					q.WithDeleted()
				}
				token := s.Token()
				if mode == "foreign" {
					token = query.NewReviewToken(context.Background())
					defer token.Close()
				}
				rec, e := q.ReviewReceipt(context.Background(), token)
				if e != nil {
					t.Fatal(e)
				}
				if mode == "zero" {
					rec = query.ReviewReceipt{}
				}
				s.Record(ref, rec)
				switch mode {
				case "mutation":
					q.Limit(4)
				case "source":
					if e := os.WriteFile(path, []byte("package changed"), 0600); e != nil {
						t.Fatal(e)
					}
				case "duplicate":
					s.Record(ref, rec)
				case "id_collision":
					ref.ID = "same"
					s.Record(ref, rec)
					s.Record(ref, rec)
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Operations) != 1 {
				t.Fatalf("operations %d", len(r.Operations))
			}
			op := r.Operations[0]
			c := op.Correspondence
			if op.Detail.Policy != analysis.Unknown || op.Detail.Precision() == "precise" || !HasFindingsAtOrAbovePrecision(r, query.AnalysisPartial) {
				t.Fatalf("unknown policy/gate lost: %+v", op.Detail)
			}
			switch mode {
			case "match":
				if c.Source != analysis.Matched || c.Operation != analysis.Matched || c.Target != analysis.Matched || c.Structure != analysis.Matched || c.Arguments != analysis.Matched || c.Context != analysis.Unconfirmed {
					t.Fatalf("match: %+v", c)
				}
			case "arguments":
				if c.Arguments != analysis.Mismatched {
					t.Fatalf("typed args: %+v", c)
				}
			case "target", "dialect":
				if c.Target != analysis.Mismatched {
					t.Fatalf("target: %+v", c)
				}
			case "structure":
				if c.Structure != analysis.Mismatched {
					t.Fatalf("structure: %+v", c)
				}
			case "context":
				if c.Context != analysis.Mismatched {
					t.Fatalf("context: %+v", c)
				}
			default:
				if c.Source != analysis.Mismatched {
					t.Fatalf("invalid source: %+v", c)
				}
			}
			v, e := r.PublicViewV2()
			if e != nil {
				t.Fatal(e)
			}
			b, e := v.ToJSON()
			if e != nil {
				t.Fatal(e)
			}
			for _, canary := range []string{"PRIVATE_TABLE_CANARY", path, op.Source.Version, "SELECT", "sample.go"} {
				if strings.Contains(string(b), canary) {
					t.Fatalf("private data exposed")
				}
			}
			if _, e = DecodeReportViewV2(b); e != nil {
				t.Fatal(e)
			}
			if _, e = DecodeReportView(b); e == nil {
				t.Fatal("v1 accepted v2")
			}
		})
	}
}
func TestReviewPrecisionUnknownAndWire(t *testing.T) {
	path := precisionFixture(t)
	b, _ := os.ReadFile(path)
	b = []byte(strings.Replace(string(b), `Where("id", 1)`, `Where("id", len(rows))`, 1))
	os.WriteFile(path, b, 0600)
	r, e := Run(Options{Paths: []string{path}})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Operations) != 1 || r.Operations[0].Detail.Bindings != analysis.Unknown {
		t.Fatal("dynamic value inferred")
	}
	legacy := ReviewReport{}
	if e := json.Unmarshal([]byte(`{"findings":[]}`), &legacy); e != nil {
		t.Fatal(e)
	}
	v1, e := r.PublicView()
	if e != nil {
		t.Fatal(e)
	}
	wire, _ := v1.ToJSON()
	if _, e := DecodeReportView(wire); e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeReportViewV2(wire); e == nil {
		t.Fatal("v2 accepted v1")
	}
	v2, _ := r.PublicViewV2()
	wire, _ = v2.ToJSON()
	for _, bad := range []string{strings.Replace(string(wire), `"version":2`, `"version":3`, 1), strings.Replace(string(wire), `"version":2`, `"version":2,"extra":true`, 1), strings.Replace(string(wire), `"version":2`, `"version":2,"version":2`, 1)} {
		if _, e := DecodeReportViewV2([]byte(bad)); e == nil {
			t.Fatal("ambiguous version/field accepted")
		}
	}
	d := analysis.Detail{}.Normalize()
	if d.SQL != analysis.Unknown || d.Columns != analysis.Unknown || d.Bindings != analysis.Unknown || d.Policy != analysis.Unknown || d.Precision() == "precise" {
		t.Fatal("legacy axes promoted")
	}
}

func TestSessionRejectsOldAndAlteredReferences(t *testing.T) {
	path := precisionFixture(t)
	var old query.ReviewReceipt
	_, e := Run(Options{Paths: []string{path}, EvidenceDialect: "mysql", Evidence: func(s *Session) {
		var err error
		old, err = receiptQuery().ReviewReceipt(context.Background(), s.Token())
		if err != nil {
			t.Fatal(err)
		}
	}})
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"old", "file", "position", "version", "missing"} {
		t.Run(mode, func(t *testing.T) {
			r, e := Run(Options{Paths: []string{path}, EvidenceDialect: "mysql", Evidence: func(s *Session) {
				ref := s.References()[0]
				if mode == "missing" {
					return
				}
				receipt, err := receiptQuery().ReviewReceipt(context.Background(), s.Token())
				if err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "old":
					receipt = old
				case "file":
					ref.File = "other.go"
				case "position":
					ref.Offset++
				case "version":
					ref.Version = "supplied-known-hash"
				}
				s.Record(ref, receipt)
			}})
			if e != nil {
				t.Fatal(e)
			}
			if len(r.Operations) != 1 {
				t.Fatal("operation lost")
			}
			c := r.Operations[0].Correspondence
			if mode == "missing" {
				if c.Source != analysis.Unconfirmed {
					t.Fatal("missing evidence matched")
				}
			} else if c.Source != analysis.Mismatched {
				t.Fatalf("invalid reference: %+v", c)
			}
		})
	}
}
func TestV2BoundsAndIndependentUnknowns(t *testing.T) {
	for _, field := range []string{"sql", "columns", "bindings", "policy"} {
		d := analysis.Detail{SQL: analysis.Known, Columns: analysis.Known, Bindings: analysis.Known, Policy: analysis.Known}
		switch field {
		case "sql":
			d.SQL = analysis.Unknown
		case "columns":
			d.Columns = analysis.Unknown
		case "bindings":
			d.Bindings = analysis.Unknown
		case "policy":
			d.Policy = analysis.Unknown
		}
		if d.Precision() != "partial" {
			t.Fatal("independent unknown lost")
		}
	}
	v := ReportViewV2{Kind: ReportViewKind, Version: 2, Findings: make([]FindingView, 300), SuppressedFindings: []FindingView{}, Operations: make([]OperationView, 300)}
	b, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	out, e := DecodeReportViewV2(b)
	if e != nil {
		t.Fatal(e)
	}
	if out.FindingCount != 300 || out.OperationCount != 300 || !out.Truncated || len(out.Findings) != 256 || len(out.Operations) != 256 {
		t.Fatal("coverage truncation lost")
	}
	d := analysis.Detail{SQL: analysis.Known, Columns: analysis.Known, Bindings: analysis.Known, Policy: analysis.Known}
	source := ReviewReport{Operations: []OperationReview{{Detail: d, Correspondence: analysis.Correspondence{Source: analysis.Matched}}}}
	b, e = json.Marshal(source)
	if e != nil {
		t.Fatal(e)
	}
	var decoded ReviewReport
	if e = json.Unmarshal(b, &decoded); e != nil {
		t.Fatal(e)
	}
	if !HasFindingsAtOrAbovePrecision(decoded, query.AnalysisPartial) || decoded.Operations[0].Correspondence.Source != analysis.Unconfirmed {
		t.Fatal("source JSON recreated evidence")
	}
	for _, bad := range []string{`{"version":2}`, `{"version":1,"version":1}`, `{"version":"1"}`} {
		var d analysis.Detail
		if json.Unmarshal([]byte(bad), &d) == nil {
			t.Fatal("detail version accepted")
		}
	}
}
