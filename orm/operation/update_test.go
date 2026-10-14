package operation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/query"
)

func updateFixture(typ, dialect string) (UpdateSpec, Options) {
	_, opts := typedFixture(typ, dialect)
	opts.Manifest.Tables[0].Columns = append(opts.Manifest.Tables[0].Columns, manifest.Column{Name: "id", Type: "bigint", TypeSource: "sql", NullableKnown: true, Primary: true})
	return UpdateSpec{Version: 1, Model: "items", Filters: []FilterSpec{{Field: "id", Op: "=", Value: int64(7)}}, Assignments: []UpdateAssignment{{Column: "v", State: UpdateValue, Value: int32(0)}}}, opts
}

func TestUpdateStatesAndDynamicParity(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		for _, tc := range []struct {
			typ   string
			value any
			state UpdateState
		}{{"integer", int32(0), UpdateValue}, {"boolean", false, UpdateValue}, {"text", "", UpdateValue}, {"text", nil, UpdateNull}, {"decimal(30,4)", json.Number("-0"), UpdateValue}, {"bigint", int64(9007199254740993), UpdateValue}} {
			t.Run(dialect+"/"+tc.typ+"/"+string(tc.state), func(t *testing.T) {
				s, o := updateFixture(tc.typ, dialect)
				o.Manifest.Tables[0].Columns[0].Nullable = true
				s.Assignments[0].Value = tc.value
				s.Assignments[0].State = tc.state
				p, v, e := CompileUpdateWithDiagnostics(t.Context(), s, o)
				if e != nil || p.Blocked || v.Outcome != "compiled" {
					t.Fatal("update refused")
				}
				q := query.NewWithSettings(nil, "items", dialectFromManifest(o.Manifest), query.SnapshotDefaultSettings())
				if querybridge.OperationEquality(q, "id", int64(7)) != nil {
					t.Fatal("reference predicate failed")
				}
				prepared, e := querybridge.Prepare(querybridge.Request{Base: q, Settings: query.SnapshotDefaultSettings(), Dialect: dialectFromManifest(o.Manifest), Context: t.Context(), Operation: "update", Rows: []map[string]any{{"v": tc.value}}, Options: writeinput.Options{Literal: true}})
				if e != nil {
					t.Fatal("dynamic planning failed")
				}
				d := prepared.Diagnostic.(*query.QueryPlan)
				if p.SQL != d.SQL || !reflect.DeepEqual(p.Params, d.Params) || !reflect.DeepEqual(p.WhereTree, d.WhereTree) || !reflect.DeepEqual(p.Warnings, d.Warnings) {
					t.Fatalf("dynamic parity: SQL=%t args=%t tree=%t warnings=%t", p.SQL == d.SQL, reflect.DeepEqual(p.Params, d.Params), reflect.DeepEqual(p.WhereTree, d.WhereTree), reflect.DeepEqual(p.Warnings, d.Warnings))
				}
				if !reflect.DeepEqual(p.Params, []any{tc.value, int64(7)}) {
					t.Fatal("native value/presence changed")
				}
				b, e := json.Marshal(s)
				if e != nil {
					t.Fatal("source serialization failed")
				}
				var decoded UpdateSpec
				if json.Unmarshal(b, &decoded) != nil {
					t.Fatal("source decode failed")
				}
				p2, _, e := CompileUpdateWithDiagnostics(t.Context(), decoded, o)
				if e != nil || p2.SQL != p.SQL {
					t.Fatal("source state lost")
				}
			})
		}
	}
}

func TestUpdateRefusalsAndUnknown(t *testing.T) {
	cases := []struct {
		name   string
		change func(*UpdateSpec, *Options)
		want   error
	}{
		{"empty", func(s *UpdateSpec, o *Options) { s.Assignments = nil }, ErrEmptyPatch},
		{"unchanged", func(s *UpdateSpec, o *Options) {
			s.Assignments[0] = UpdateAssignment{Column: "v", State: UpdateUnchanged}
		}, ErrEmptyPatch},
		{"nil_value", func(s *UpdateSpec, o *Options) { s.Assignments[0].Value = nil; s.Assignments[0].ValuePresent = true }, ErrInvalidAssignment},
		{"missing", func(s *UpdateSpec, o *Options) { s.Assignments[0].Value = nil }, ErrInvalidAssignment},
		{"nonnullable", func(s *UpdateSpec, o *Options) { s.Assignments[0] = UpdateAssignment{Column: "v", State: UpdateNull} }, ErrTypeMismatch},
		{"unknown_null", func(s *UpdateSpec, o *Options) {
			s.Assignments[0] = UpdateAssignment{Column: "v", State: UpdateNull}
			o.Manifest.Tables[0].Columns[0].NullableKnown = false
		}, ErrTypeMismatch},
		{"null_value", func(s *UpdateSpec, o *Options) { s.Assignments[0].State = UpdateNull }, ErrInvalidAssignment},
		{"duplicate", func(s *UpdateSpec, o *Options) { s.Assignments = append(s.Assignments, s.Assignments[0]) }, ErrInvalidAssignment},
		{"qualified", func(s *UpdateSpec, o *Options) { s.Assignments[0].Column = "items.v" }, ErrInvalidAssignment},
		{"unknown_column", func(s *UpdateSpec, o *Options) { s.Assignments[0].Column = "missing" }, ErrUnknownField},
		{"readonly", func(s *UpdateSpec, o *Options) { o.Manifest.Tables[0].Columns[0].Readonly = true }, ErrForbiddenField},
		{"generated_unchanged", func(s *UpdateSpec, o *Options) {
			o.Manifest.Tables[0].Columns[0].Generated = true
			s.Assignments[0] = UpdateAssignment{Column: "v", State: UpdateUnchanged}
		}, ErrForbiddenField},
		{"primary", func(s *UpdateSpec, o *Options) { s.Assignments[0].Column = "id" }, ErrForbiddenField},
		{"protected", func(s *UpdateSpec, o *Options) {
			o.Manifest.Tables[0].Policies = []manifest.Policy{{Type: "protected", Column: "v"}}
		}, ErrForbiddenField},
		{"reserved_values", func(s *UpdateSpec, o *Options) { o.Values = map[string]any{" CURRENT_TENANT ": 7} }, ErrReservedBinding},
		{"opaque", func(s *UpdateSpec, o *Options) { s.Assignments[0].Value = updateOpaque{} }, ErrInputLimit},
		{"array", func(s *UpdateSpec, o *Options) { o.Manifest.Tables[0].Columns[0].Type = "integer[]" }, ErrArrayBinding},
		{"array_null", func(s *UpdateSpec, o *Options) {
			o.Manifest.Tables[0].Columns[0].Type = "integer[]"
			o.Manifest.Tables[0].Columns[0].Nullable = true
			s.Assignments[0] = UpdateAssignment{Column: "v", State: UpdateNull}
		}, ErrArrayBinding},
		{"bytes", func(s *UpdateSpec, o *Options) { s.AccessReason = strings.Repeat("x", 1<<20) }, ErrInputLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, o := updateFixture("integer", "postgres")
			tc.change(&s, &o)
			_, v, e := CompileUpdateWithDiagnostics(t.Context(), s, o)
			if !errors.Is(e, tc.want) || v.Outcome != "rejected" {
				t.Fatal("expected fixed refusal")
			}
		})
	}
	for _, unknown := range []string{"nullability", "type", "decimal"} {
		s, o := updateFixture("integer", "postgres")
		switch unknown {
		case "nullability":
			o.Manifest.Tables[0].Columns[0].NullableKnown = false
		case "type":
			o.Manifest.Tables[0].Columns[0].TypeSource = "go"
		case "decimal":
			o.Manifest.Tables[0].Columns[0].Type = "decimal(5,2)"
			s.Assignments[0].Value = json.Number("1.20")
		}
		p, v, e := CompileUpdateWithDiagnostics(t.Context(), s, o)
		if e != nil || !p.Blocked || v.Coverage != "partial" {
			t.Fatal("unknown became executable")
		}
		o = strictFixture(t, o, false, false)
		if _, _, e = CompileUpdateWithDiagnostics(t.Context(), s, o); !errors.Is(e, ErrTypeUnverified) {
			t.Fatal("Strict unknown accepted")
		}
	}
}

type updateOpaque struct{}

func (updateOpaque) MarshalJSON() ([]byte, error) { panic("opaque method called") }
func (updateOpaque) String() string               { panic("opaque method called") }

func TestUpdateClosedWire(t *testing.T) {
	for _, b := range []string{`{"version":1,"Version":1}`, `{"version":1,"assignments":[{"column":"v","state":"null","extra":0}]}`, `{"version":2}`, `{"version":1,"model":"items","raw":"hidden"}`, `{"version":1,"assignments":[{"column":"v","state":"value","value":0,"Value":1}]}`} {
		var s UpdateSpec
		if json.Unmarshal([]byte(b), &s) == nil {
			t.Fatal("ambiguous source accepted")
		}
	}
	for _, state := range []UpdateState{UpdateValue, UpdateNull, UpdateUnchanged} {
		a := UpdateAssignment{Column: "v", State: state}
		if state == UpdateValue {
			a.Value = false
		}
		b, e := json.Marshal(a)
		if e != nil {
			t.Fatal("encode failed")
		}
		var next UpdateAssignment
		if json.Unmarshal(b, &next) != nil || next.State != state || next.hasValue() != (state == UpdateValue) {
			t.Fatal("wire state lost")
		}
	}
}

func TestUpdatePrivateSealAndReturning(t *testing.T) {
	for _, returning := range []bool{false, true} {
		std, mock, e := sqlmock.New()
		if e != nil {
			t.Fatal("mock unavailable")
		}
		defer std.Close()
		s, o := updateFixture("integer", "postgres")
		if returning {
			s.Returning = []string{"v"}
		}
		p, e := querybridge.PrepareUpdate(t.Context(), s, o, std)
		if e != nil {
			t.Fatal("prepare failed")
		}
		d := p.Diagnostic.(*query.QueryPlan)
		text := d.SQL
		d.SQL = "tampered"
		d.Params = []any{"tampered"}
		d.Blocked = false
		if returning {
			mock.ExpectQuery(regexp.QuoteMeta(text)).WithArgs(int32(0), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(0))
			e = p.Scan(func(rows *sql.Rows) error {
				if !rows.Next() {
					t.Fatal("missing row")
				}
				var v int32
				return rows.Scan(&v)
			})
		} else {
			mock.ExpectExec(regexp.QuoteMeta(text)).WithArgs(int32(0), int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
			_, e = p.Exec()
		}
		if e != nil {
			t.Fatal("sealed update failed")
		}
		if _, e = p.Exec(); !errors.Is(e, query.ErrBlockedOperation) {
			t.Fatal("private write replayed")
		}
		if mock.ExpectationsWereMet() != nil {
			t.Fatal("sealed dispatch changed")
		}
	}
}
