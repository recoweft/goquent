package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func comparisonFixture() Schema {
	s := Schema{Tables: []TableSchema{{Name: "records", Columns: []ColumnSchema{{Name: "a", Type: "text", DefaultState: DefaultNone}, {Name: "b", Type: "text", DefaultState: DefaultNone}}, Indexes: []IndexSchema{{Name: "records_idx", Columns: []string{"a", "b"}, Attributes: CoverageComplete, Method: "btree", Visibility: "visible", Predicate: ExpressionDefinition{State: "none"}}}}}}
	return declaredFixture(s)
}
func TestDefaultStateComparison(t *testing.T) {
	none := ColumnSchema{DefaultState: DefaultNone}
	expr := func(s string) ColumnSchema {
		return ColumnSchema{HasDefault: true, DefaultExpression: s, DefaultState: DefaultExpression}
	}
	for _, tc := range []struct {
		name            string
		a, b            ColumnSchema
		change, unknown bool
	}{
		{"none", none, none, false, false}, {"add", none, expr("NULL"), true, false}, {"remove", expr("NULL"), none, true, false},
		{"literal", expr("'a'"), expr("'A'"), true, false}, {"spaces", expr("' a '"), expr("'a'"), true, false},
		{"casts", expr("'a'"), expr("'a'::text"), true, false}, {"empty literal", none, expr("''"), true, false},
		{"same expression", expr("now()"), expr("now()"), false, false},
		{"legacy expression", ColumnSchema{HasDefault: true, DefaultExpression: "now()"}, expr("now()"), false, false},
		{"legacy absent", ColumnSchema{}, none, false, true}, {"both unknown", ColumnSchema{}, ColumnSchema{}, false, true},
		{"missing expression", ColumnSchema{HasDefault: true}, none, false, true},
		{"contradiction", ColumnSchema{DefaultExpression: "NULL"}, none, false, true},
		{"none contradiction", ColumnSchema{DefaultState: DefaultNone, HasDefault: true, DefaultExpression: "1"}, none, false, true},
		{"expression contradiction", ColumnSchema{DefaultState: DefaultExpression, DefaultExpression: "1"}, none, false, true},
		{"unknown enum", ColumnSchema{DefaultState: "future"}, none, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := comparisonFixture(), comparisonFixture()
			tc.a.Name = "a"
			tc.b.Name = "a"
			tc.a.Type = "text"
			tc.b.Type = "text"
			a.Tables[0].Columns[0] = tc.a
			b.Tables[0].Columns[0] = tc.b
			p := DiffSchemas(a, b)
			r := CompareSchemaDrift(b, a)
			if r.Drifted != tc.change || r.ComparisonIncomplete != tc.unknown || p.ComparisonIncomplete != tc.unknown {
				t.Fatalf("unexpected change/incomplete: %v %v", r.Drifted, r.ComparisonIncomplete)
			}
			if tc.unknown && p.Steps[0].CurrentDefault != nil {
				d := p.Steps[0].CurrentDefault
				if d.DeclaredState != tc.a.DefaultState || d.HasDefault != tc.a.HasDefault || d.Expression != tc.a.DefaultExpression {
					t.Fatal("ambiguous source declaration lost")
				}
			}
			if tc.change {
				if len(p.Steps) != 1 || p.Steps[0].Type != AlterColumnDefault || p.Steps[0].CurrentDefault == nil || p.Steps[0].DesiredDefault == nil {
					t.Fatal("lost default definition")
				}
			}
			if tc.change || tc.unknown {
				p.Approval = &query.Approval{Reason: "approved"}
				if !errors.Is(EnsureExecutable(p), query.ErrBlockedOperation) {
					t.Fatal("structural plan executable")
				}
			}
		})
	}
}
func TestDefaultJSONRoundTripAndAmbiguity(t *testing.T) {
	for _, c := range []ColumnSchema{{}, {DefaultState: DefaultNone}, {DefaultState: DefaultUnknown}, {DefaultState: DefaultExpression, HasDefault: true, DefaultExpression: "'secret'", DefaultSource: "postgres.information_schema.columns"}} {
		b, e := json.Marshal(c)
		if e != nil {
			t.Fatal(e)
		}
		var out ColumnSchema
		if json.Unmarshal(b, &out) != nil || out != c {
			t.Fatal("source state lost")
		}
	}
	for _, s := range []string{`{"has_default":null}`, `{"default_expression":null}`, `{"default_state":null}`, `{"default_state":"none","default_state":"expression"}`, `{"HAS_DEFAULT":null}`} {
		var c ColumnSchema
		if json.Unmarshal([]byte(s), &c) == nil {
			t.Fatal("ambiguous input accepted")
		}
	}
	for _, s := range []string{`{}`, `{"has_default":false}`, `{"has_default":false,"default_expression":"1"}`} {
		var c ColumnSchema
		if json.Unmarshal([]byte(s), &c) != nil || columnDefault(c).State != DefaultUnknown {
			t.Fatal("legacy promoted")
		}
	}
}
func TestIndexDefinitionsAndDetachedSource(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		columns                             []string
		unique, concurrent, change, unknown bool
	}{
		{name: "same", columns: []string{"a", "b"}}, {name: "order", columns: []string{"b", "a"}, change: true},
		{name: "composition", columns: []string{"a"}, change: true}, {name: "unique", columns: []string{"a", "b"}, unique: true, change: true},
		{name: "method", columns: []string{"a", "b"}, concurrent: true}, {name: "nil", unknown: true}, {name: "empty", columns: []string{}, unknown: true},
		{name: "missing", columns: []string{"missing"}, unknown: true}, {name: "duplicate", columns: []string{"a", "a"}, unknown: true},
		{name: "expression", columns: []string{"lower(a)"}, unknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := comparisonFixture(), comparisonFixture()
			b.Tables[0].Indexes[0].Columns = tc.columns
			b.Tables[0].Indexes[0].Unique = tc.unique
			b.Tables[0].Indexes[0].Concurrent = tc.concurrent
			p := DiffSchemas(a, b)
			r := CompareSchemaDrift(b, a)
			if r.Drifted != tc.change || r.ComparisonIncomplete != tc.unknown {
				t.Fatal("incorrect index comparison")
			}
			if tc.change {
				s := p.Steps[0]
				if s.Type != AlterIndex || s.CurrentIndex == nil || s.DesiredIndex == nil || s.DesiredIndex.Unique != tc.unique {
					t.Fatal("lost source")
				}
				b.Tables[0].Indexes[0].Columns[0] = "mutated"
				if s.DesiredIndex.Columns[0] == "mutated" {
					t.Fatal("aliased source")
				}
			}
		})
	}
	a, b := comparisonFixture(), comparisonFixture()
	b.Tables[0].Indexes = nil
	p := DiffSchemas(a, b)
	if p.Steps[0].CurrentIndex == nil {
		t.Fatal("drop lost definition")
	}
	p = DiffSchemas(b, a)
	if p.Steps[0].DesiredIndex == nil {
		t.Fatal("add lost definition")
	}
}
func TestAmbiguousSchemaAndKnownChanges(t *testing.T) {
	for _, mutate := range []func(*Schema){
		func(s *Schema) { s.Tables = append(s.Tables, s.Tables[0]) },
		func(s *Schema) { s.Tables[0].Name = "public.records" },
		func(s *Schema) { s.Tables[0].Columns = append(s.Tables[0].Columns, s.Tables[0].Columns[0]) },
		func(s *Schema) { s.Tables[0].Indexes = append(s.Tables[0].Indexes, s.Tables[0].Indexes[0]) },
		func(s *Schema) { s.Tables[0].Indexes[0].Name = "Records_idx" },
		func(s *Schema) { s.Tables[0].Indexes[0].DefinitionUnknown = true },
	} {
		a, b := comparisonFixture(), comparisonFixture()
		mutate(&a)
		mutate(&b)
		b.Tables = append(b.Tables, TableSchema{Name: "added", Identity: ObjectIdentity{"testdb", "public", "added", CoverageComplete}, Coverage: completeFixtureCoverage()})
		p := DiffSchemas(a, b)
		if !p.ComparisonIncomplete || !hasStep(p.Steps, AddTable, "added") {
			t.Fatal("unknown erased or known change lost")
		}
	}
	if !DiffSchemas(Schema{}, Schema{Tables: []TableSchema{}}).ComparisonIncomplete {
		t.Fatal("legacy empty must remain unknown")
	}
}
func TestComparisonPublicCanary(t *testing.T) {
	a, b := comparisonFixture(), comparisonFixture()
	a.Tables[0].Columns[0].DefaultState = DefaultExpression
	a.Tables[0].Columns[0].HasDefault = true
	a.Tables[0].Columns[0].DefaultExpression = "'private-canary@example.test'"
	b.Tables[0].Indexes[0].Unique = true
	p := DiffSchemas(a, b)
	v, e := p.PublicView()
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = DecodeMigrationPlanView(encoded); e != nil {
		t.Fatal(e)
	}
	var pretty bytes.Buffer
	if WritePretty(&pretty, p) != nil {
		t.Fatal("format")
	}
	for _, out := range []string{string(encoded), pretty.String(), fmt.Sprint(p), fmt.Sprint(v)} {
		for _, secret := range []string{"private-canary", "records", "records_idx", "example.test"} {
			if strings.Contains(out, secret) {
				t.Fatal("public data leak")
			}
		}
		if !strings.Contains(out, "alter_column_default") || !strings.Contains(out, "alter_index") {
			t.Fatal("classification lost")
		}
	}
	raw, e := p.ToJSON()
	if e != nil || !bytes.Contains(raw, []byte("private-canary")) {
		t.Fatal("source definition lost")
	}
}
func TestPostgresIndexBoundedReader(t *testing.T) {
	for _, s := range []string{"CREATE INDEX i ON public.t USING btree (a) WHERE a IS NOT NULL", "CREATE INDEX i ON public.t USING btree (lower(a))", "CREATE INDEX i ON public.t USING btree (a DESC)", "CREATE INDEX i ON public.t USING btree (a) INCLUDE (b)", `CREATE INDEX i ON public.t USING btree ("A")`} {
		if parsePostgresIndexColumns(s) != nil {
			t.Fatal("unsupported definition promoted")
		}
	}
	if len(parsePostgresIndexColumns("CREATE UNIQUE INDEX i ON public.t USING btree (a, b)")) != 2 {
		t.Fatal("ordinary index rejected")
	}
}

func TestSchemaPlanningNeverConnects(t *testing.T) {
	t.Setenv("TEST_DB_DSN", "invalid://must-not-connect")
	t.Setenv("TEST_MYSQL_DSN", "invalid://must-not-connect")
	t.Setenv("TEST_POSTGRES_DSN", "invalid://must-not-connect")
	s := comparisonFixture()
	if p := DiffSchemas(s, s); p.ComparisonIncomplete || len(p.Steps) != 0 {
		t.Fatal("known comparison")
	}
	if _, e := New("CREATE TABLE offline_fixture (id INTEGER)").Plan(nil); e != nil {
		t.Fatal(e)
	}
	if _, e := New("CREATE TABLE offline_fixture (id INTEGER)").DryRun(nil); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []MigrationStepType{AlterIndex, AlterColumnDefault} {
		p := PlanSteps([]MigrationStep{{Type: kind}})
		p.Approval = &query.Approval{Reason: "not evidence"}
		if EnsureExecutable(p) == nil {
			t.Fatal("empty structural statements accepted")
		}
	}
}

func TestUnknownColumnPreservesOtherChanges(t *testing.T) {
	a, b := comparisonFixture(), comparisonFixture()
	a.Tables[0].Columns = append(a.Tables[0].Columns, a.Tables[0].Columns[0])
	b.Tables[0].Columns = append(b.Tables[0].Columns, b.Tables[0].Columns[0])
	b.Tables[0].Columns[1].DefaultState = DefaultExpression
	b.Tables[0].Columns[1].HasDefault = true
	b.Tables[0].Columns[1].DefaultExpression = "'known change'"
	p := DiffSchemas(a, b)
	if !p.ComparisonIncomplete || !hasStep(p.Steps, AlterColumnDefault, "b") {
		t.Fatal("known column change erased")
	}
	marker := PlanSteps([]MigrationStep{comparisonUnknown("private", "", "")})
	if !marker.ComparisonIncomplete || len(marker.Warnings) != 1 || marker.Warnings[0].Code != WarningSchemaComparisonUnknown {
		t.Fatal("structured unknown state lost")
	}
}
