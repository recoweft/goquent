package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestWholeTableComparison(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*TableSchema)
		unknown bool
	}{
		{"known", func(*TableSchema) {}, false},
		{"known_expression", func(s *TableSchema) {
			s.Columns[0].DefaultState = DefaultExpression
			s.Columns[0].HasDefault = true
			s.Columns[0].DefaultExpression = "NULL"
		}, false},
		{"legacy", func(s *TableSchema) { s.Columns[0].DefaultState = "" }, true},
		{"unknown", func(s *TableSchema) { s.Columns[0].DefaultState = DefaultUnknown }, true},
		{"contradiction", func(s *TableSchema) {
			s.Columns[0].HasDefault = true
			s.Columns[0].DefaultExpression = "'private_canary'"
		}, true},
		{"unknown_index", func(s *TableSchema) { s.Indexes[0].DefinitionUnknown = true }, true},
		{"duplicate_index", func(s *TableSchema) { s.Indexes = append(s.Indexes, s.Indexes[0]) }, true},
		{"missing_column", func(s *TableSchema) { s.Indexes[0].Columns = []string{"absent"} }, true},
		{"ambiguous_index", func(s *TableSchema) { s.Indexes[0].Name = "Public.Idx" }, true},
		{"empty_index", func(s *TableSchema) { s.Indexes[0].Columns = nil }, true},
		{"duplicate_index_column", func(s *TableSchema) { s.Indexes[0].Columns = []string{"a", "a"} }, true},
		{"ambiguous_column", func(s *TableSchema) { s.Columns[0].Name = "A" }, true},
		{"duplicate_column", func(s *TableSchema) { s.Columns = append(s.Columns, s.Columns[0]) }, true},
	}
	for _, tc := range cases {
		for _, drop := range []bool{false, true} {
			for _, nonempty := range []bool{false, true} {
				t.Run(tc.name+"/"+map[bool]string{true: "drop", false: "add"}[drop]+"/"+map[bool]string{true: "nonempty", false: "empty"}[nonempty], func(t *testing.T) {
					t.Setenv("TEST_DB_DSN", "invalid://must-not-connect")
					t.Setenv("TEST_MYSQL_DSN", "invalid://must-not-connect")
					t.Setenv("TEST_POSTGRES_DSN", "invalid://must-not-connect")
					s := comparisonFixture()
					tc.mutate(&s.Tables[0])
					s.Tables[0].Indexes[0].RawDefinition = "private_canary"
					a, b := declaredFixture(Schema{}), s
					if drop {
						a, b = b, a
					}
					if nonempty {
						old, next := comparisonFixture().Tables[0], comparisonFixture().Tables[0]
						old.Name, next.Name = "existing", "existing"
						old.Identity.Name, next.Identity.Name = "existing", "existing"
						next.Columns[0].Type = "integer"
						a.Tables = append(a.Tables, old)
						b.Tables = append(b.Tables, next)
					}
					p := DiffSchemas(a, b)
					r := CompareSchemaDrift(b, a)
					if p.ComparisonIncomplete != tc.unknown || r.ComparisonIncomplete != tc.unknown || !r.Drifted {
						t.Fatal("incorrect completeness/drift")
					}
					kind := AddTable
					if drop {
						kind = DropTable
					}
					if !hasStep(p.Steps, kind, "records") || (nonempty && !hasStep(p.Steps, AlterColumnType, "a")) {
						t.Fatal("known change lost")
					}
					for _, step := range p.Steps {
						if step.Type == AddIndex || step.Type == DropIndex || step.Type == AddColumn || step.Type == DropColumn {
							t.Fatal("nested changes double counted")
						}
					}
					stepIndex := 0
					for j, step := range p.Steps {
						if step.Type == kind && step.Table == "records" {
							stepIndex = j
							break
						}
					}
					snapshot := p.Steps[stepIndex].DesiredTable
					if drop {
						snapshot = p.Steps[stepIndex].CurrentTable
					}
					if !reflect.DeepEqual(snapshot, &s.Tables[0]) {
						t.Fatal("source declaration lost")
					}
					source, e := p.ToJSON()
					if e != nil || !bytes.Contains(source, []byte("private_canary")) {
						t.Fatal("source serialization lost")
					}
					var decoded MigrationPlan
					if json.Unmarshal(source, &decoded) != nil {
						t.Fatal("source decode")
					}
					ds := decoded.Steps[stepIndex].DesiredTable
					if drop {
						ds = decoded.Steps[stepIndex].CurrentTable
					}
					if !reflect.DeepEqual(ds, snapshot) {
						t.Fatal("source roundtrip lost")
					}
					v, e := p.PublicView()
					if e != nil {
						t.Fatal(e)
					}
					if v.ComparisonIncomplete != tc.unknown {
						t.Fatal("public plan completeness lost")
					}
					rv := r.PublicView()
					if rv.Unknown != tc.unknown || rv.Known == tc.unknown {
						t.Fatal("public drift completeness lost")
					}
					public, e := v.ToJSON()
					if e != nil {
						t.Fatal(e)
					}
					var pretty bytes.Buffer
					if WritePretty(&pretty, p) != nil {
						t.Fatal("pretty")
					}
					for _, out := range [][]byte{public, pretty.Bytes()} {
						for _, secret := range []string{"private_canary", "records", "existing", "Public.Idx"} {
							if bytes.Contains(out, []byte(secret)) {
								t.Fatal("public leak")
							}
						}
					}
					if tc.unknown {
						p.Approval = &query.Approval{Reason: "approved"}
						if !errors.Is(EnsureExecutable(p), query.ErrBlockedOperation) || !p.Blocked || p.AnalysisPrecision == query.AnalysisPrecise {
							t.Fatal("unknown promoted")
						}
					} else if !nonempty && len(p.Steps) != 1 {
						t.Fatal("known table lost positive case")
					}
					s.Tables[0].Columns[0].DefaultExpression = "mutated"
					s.Tables[0].Indexes[0].RawDefinition = "mutated"
					if len(s.Tables[0].Indexes[0].Columns) > 0 {
						s.Tables[0].Indexes[0].Columns[0] = "mutated"
					}
					after, _ := p.ToJSON()
					if !bytes.Equal(source, after) { // Approval is unrelated to source detachment.
						p.Approval = nil
						after, _ = p.ToJSON()
						if !bytes.Equal(source, after) {
							t.Fatal("source aliases input")
						}
					}
				})
			}
		}
	}
}
