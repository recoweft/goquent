package query

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	ormdriver "github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/predicate"
)

func scopeContext(dialect string, names ...string) WriteKeyContext {
	typ := "INT"
	if dialect == "postgres" {
		typ = "integer"
	}
	c := WriteKeyContext{Database: "testdb", Dialect: dialect, Table: "users", Constraints: []WriteKeyConstraint{{Name: "pk", Kind: "primary", Valid: true, AllRows: true, NotDeferrable: true}}}
	for _, n := range names {
		c.Constraints[0].Columns = append(c.Constraints[0].Columns, WriteKeyColumn{Name: n, DBType: typ, Bits: 32})
	}
	return c
}
func scopePlan(t *testing.T, q *Query) *QueryPlan {
	t.Helper()
	p, e := q.PlanUpdate(context.Background(), map[string]any{"score": 1})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestWriteScopeLogic(t *testing.T) {
	cases := []struct {
		name  string
		build func(*Query) *Query
		want  string
	}{
		{"equal", func(q *Query) *Query { return q.Where("id", 1) }, "at_most_one"},
		{"single_in", func(q *Query) *Query { return q.WhereIn("id", []int{1}) }, "at_most_one"},
		{"same_or", func(q *Query) *Query { return q.Where("id", int8(1)).OrWhere("id", uint64(1)) }, "at_most_one"},
		{"different_or", func(q *Query) *Query { return q.Where("id", 1).OrWhere("id", 2) }, "broad"},
		{"non_key_or", func(q *Query) *Query { return q.Where("id", 1).OrWhere("score", 2) }, "broad"},
		{"range", func(q *Query) *Query { return q.Where("id", ">", 1) }, "broad"},
		{"ne", func(q *Query) *Query { return q.Where("id", "!=", 1) }, "broad"},
		{"multi_in", func(q *Query) *Query { return q.WhereIn("id", []int{1, 2}) }, "broad"},
		{"null", func(q *Query) *Query { return q.WhereNull("id") }, "broad"},
		{"nil", func(q *Query) *Query { return q.Where("id", nil) }, "broad"},
		{"null_in", func(q *Query) *Query { return q.WhereIn("id", []any{nil, 1}) }, "broad"},
		{"empty_in", func(q *Query) *Query { return q.WhereIn("id", []int{}) }, "unknown"},
		{"column", func(q *Query) *Query { return q.WhereColumn("id", "=", "score") }, "broad"},
		{"not", func(q *Query) *Query { return q.WhereNot(func(q *Query) { q.Where("id", 1) }) }, "broad"},
		{"nested", func(q *Query) *Query {
			return q.WhereGroup(func(q *Query) { q.Where("id", 1).OrWhere("id", 1) }).Where("score", ">", 0)
		}, "at_most_one"},
		{"key_and_raw", func(q *Query) *Query { return q.Where("id", 1).SafeWhereRaw("score > :n", map[string]any{"n": 0}) }, "unknown"},
		{"key_or_raw", func(q *Query) *Query { return q.Where("id", 1).SafeOrWhereRaw("score > :n", map[string]any{"n": 0}) }, "unknown"},
		{"string", func(q *Query) *Query { return q.Where("id", "1") }, "unknown"},
		{"float", func(q *Query) *Query { return q.Where("id", float64(1)) }, "unknown"},
		{"overflow", func(q *Query) *Query { return q.Where("id", uint64(math.MaxUint64)) }, "unknown"},
		{"conflict", func(q *Query) *Query { return q.Where("id", 1).Where("id", 2) }, "unknown"},
	}
	for _, d := range []struct {
		name    string
		dialect ormdriver.Dialect
	}{{"mysql", ormdriver.MySQLDialect{}}, {"postgres", ormdriver.PostgresDialect{}}} {
		for _, tc := range cases {
			t.Run(d.name+"/"+tc.name, func(t *testing.T) {
				q := tc.build(New(&recordingExec{}, "users", d.dialect).WithWriteKeyContext(scopeContext(d.name, "id")))
				for _, op := range []string{"update", "delete"} {
					var p *QueryPlan
					if op == "update" {
						p = scopePlan(t, q)
					} else {
						var err error
						p, err = q.PlanDelete(context.Background())
						if err != nil {
							t.Fatal(err)
						}
					}
					r := AnalyzeWriteScope(p)
					if r.Status != tc.want {
						t.Fatalf("%s: %+v; tree=%+v", op, r, p.WhereTree)
					}
					code := WarningBulkUpdateDetected
					if op == "delete" {
						code = WarningBulkDeleteDetected
					}
					if warningCodeSet(p.Warnings)[code] != (r.Status != "at_most_one") {
						t.Fatalf("diagnostic drift: %+v", p.Warnings)
					}
					if p.WriteScope.Status != r.Status {
						t.Fatal("stale result")
					}
				}
			})
		}
	}
}
func TestWriteScopeCompositeAndAliases(t *testing.T) {
	cases := []struct {
		name, table, alias string
		build              func(*Query) *Query
		want               string
	}{
		{"complete", "users", "", func(q *Query) *Query { return q.Where("tenant", 1).WhereIn("id", []int{2}) }, "at_most_one"},
		{"partial", "users", "", func(q *Query) *Query { return q.Where("id", 2) }, "broad"},
		{"alias", "users as u", "u", func(q *Query) *Query { return q.Where("u.tenant", 1).Where("u.id", 2) }, "at_most_one"},
		{"cross_alias", "users as u", "u", func(q *Query) *Query { return q.Where("u.tenant", 1).Where("v.id", 2) }, "unknown"},
		{"self_join", "users as u", "u", func(q *Query) *Query {
			return q.Join("users as v", "u.id", "=", "v.id").Where("u.tenant", 1).Where("v.id", 2)
		}, "unknown"},
		{"or_complete", "users", "", func(q *Query) *Query {
			return q.WhereGroup(func(q *Query) { q.Where("tenant", 1).Where("id", 2) }).OrWhereGroup(func(q *Query) { q.Where("tenant", int8(1)).Where("id", int64(2)) })
		}, "at_most_one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := scopeContext("mysql", "tenant", "id")
			c.Alias = tc.alias
			p := scopePlan(t, tc.build(New(&recordingExec{}, tc.table, ormdriver.MySQLDialect{}).WithWriteKeyContext(c)))
			if r := AnalyzeWriteScope(p); r.Status != tc.want {
				t.Fatalf("%+v", r)
			}
		})
	}
}
func TestWriteScopeIntegerBounds(t *testing.T) {
	for _, bits := range []int{16, 32, 64} {
		min, max := int64(math.MinInt64), int64(math.MaxInt64)
		if bits < 64 {
			min = -(int64(1) << (bits - 1))
			max = (int64(1) << (bits - 1)) - 1
		}
		for _, v := range []any{min, max, int(1), int8(1), int16(1), int32(1), int64(1), uint(1), uint8(1), uint16(1), uint32(1), uint64(1)} {
			if _, ok := scopeInteger(v, bits); !ok {
				t.Fatalf("%d %T %v", bits, v, v)
			}
		}
		if bits < 64 {
			for _, v := range []any{min - 1, max + 1} {
				if _, ok := scopeInteger(v, bits); ok {
					t.Fatal("overflow accepted")
				}
			}
		}
	}
	type customInt int
	for _, v := range []any{uint64(math.MaxInt64) + 1, "1", []byte("1"), 1.0, true, new(int), customInt(1)} {
		if _, ok := scopeInteger(v, 64); ok {
			t.Fatalf("accepted %T", v)
		}
	}
}

type scopeValuer struct{ calls *int }

func (v scopeValuer) Value() (driver.Value, error) { *v.calls++; return int64(1), nil }
func TestWriteScopeEvidence(t *testing.T) {
	makePlan := func() *QueryPlan {
		return scopePlan(t, New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(scopeContext("mysql", "id")).Where("id", 1))
	}
	for name, edit := range map[string]func(*QueryPlan){
		"sql": func(p *QueryPlan) { p.SQL += " OR 1=1" }, "params": func(p *QueryPlan) { p.Params[len(p.Params)-1] = 2 }, "type": func(p *QueryPlan) { p.Params[len(p.Params)-1] = int64(1) },
		"tree": func(p *QueryPlan) { p.WhereTree.Values[0].Data = 2 }, "target": func(p *QueryPlan) { p.Tables[0].Name = "other" }, "operation": func(p *QueryPlan) { p.Operation = OperationDelete },
		"unverified": func(p *QueryPlan) { p.Unverified = append(p.Unverified, "custom") }, "cycle": func(p *QueryPlan) { p.WhereTree.Children = []*predicate.Node{p.WhereTree} },
	} {
		t.Run(name, func(t *testing.T) {
			p := makePlan()
			edit(p)
			if r := AnalyzeWriteScope(p); r.Status != "unknown" {
				t.Fatalf("%+v", r)
			}
			DefaultRiskEngine.CheckQuery(p)
			if p.WriteScope.Status != "unknown" {
				t.Fatal("stale cached proof")
			}
		})
	}
	p := makePlan()
	b, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	var decoded QueryPlan
	if e = json.Unmarshal(b, &decoded); e != nil {
		t.Fatal(e)
	}
	if AnalyzeWriteScope(&decoded).Status != "unknown" {
		t.Fatal("json proof")
	}
	p.Metadata[MetadataTableRisk] = []TableRiskMetadata{{Table: "users", PrimaryKeyColumns: []string{"id"}}}
	p.WriteScope.Status = "unknown"
	if AnalyzeWriteScope(p).Status != "at_most_one" {
		t.Fatal("public output changed private evidence")
	}
	c := scopeContext("mysql", "id")
	q := New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(c).Where("id", 1)
	c.Constraints[0].Columns[0].Name = "score"
	if AnalyzeWriteScope(scopePlan(t, q)).Status != "at_most_one" {
		t.Fatal("context not owned")
	}
	calls := 0
	q = New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(scopeContext("mysql", "id")).Where("id", scopeValuer{&calls})
	if AnalyzeWriteScope(scopePlan(t, q)).Status != "unknown" || calls != 0 {
		t.Fatal("valuer invoked")
	}
}
func TestWriteScopeContextFailures(t *testing.T) {
	for name, edit := range map[string]func(*WriteKeyContext){
		"database": func(c *WriteKeyContext) { c.Database = "" }, "dialect": func(c *WriteKeyContext) { c.Dialect = "postgres" }, "table": func(c *WriteKeyContext) { c.Table = "other" }, "alias": func(c *WriteKeyContext) { c.Alias = "u" },
		"deferred": func(c *WriteKeyContext) { c.Constraints[0].NotDeferrable = false }, "partial": func(c *WriteKeyContext) { c.Constraints[0].Partial = true }, "expression": func(c *WriteKeyContext) { c.Constraints[0].Expression = true }, "invalid": func(c *WriteKeyContext) { c.Constraints[0].Valid = false }, "scope": func(c *WriteKeyContext) { c.Constraints[0].AllRows = false },
		"string_key": func(c *WriteKeyContext) { c.Constraints[0].Columns[0].DBType = "VARCHAR" }, "unsigned": func(c *WriteKeyContext) { c.Constraints[0].Columns[0].Unsigned = true }, "budget": func(c *WriteKeyContext) { c.Constraints = make([]WriteKeyConstraint, 65) },
	} {
		t.Run(name, func(t *testing.T) {
			c := scopeContext("mysql", "id")
			edit(&c)
			p := scopePlan(t, New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(c).Where("id", 1))
			if r := AnalyzeWriteScope(p); r.Status != "unknown" {
				t.Fatalf("%+v", r)
			}
		})
	}
	c := scopeContext("mysql", "id")
	c.Constraints[0].Kind = "unique"
	c.Constraints[0].Columns[0].Nullable = true
	if r := AnalyzeWriteScope(scopePlan(t, New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(c).Where("id", 1))); r.Status != "at_most_one" {
		t.Fatalf("%+v", r)
	}
	p := scopePlan(t, New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).PrimaryKey("id").Where("id", 1))
	before := append([]any(nil), p.Params...)
	AttachTableRiskMetadata(p, []TableRiskMetadata{{Table: "users", PrimaryKeyColumns: []string{"id"}}})
	DefaultRiskEngine.CheckQuery(p)
	if p.WriteScope.Status != "unknown" || !reflect.DeepEqual(before, p.Params) {
		t.Fatal("legacy metadata promoted or params changed")
	}
}

func TestWriteScopeBudgetsAndDiagnostics(t *testing.T) {
	q := New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(scopeContext("mysql", "id")).Where("id", 1)
	p := scopePlan(t, q)
	// Public DAG expansion and depth are bounded before correspondence comparison.
	for i := 0; i < 20; i++ {
		p.WhereTree = &predicate.Node{Kind: "and", Correspondence: "generated", Children: []*predicate.Node{p.WhereTree, p.WhereTree}}
	}
	if r := AnalyzeWriteScope(p); r.Status != "unknown" || r.Reason != "correspondence_budget_or_cycle" {
		t.Fatalf("%+v", r)
	}
	p = scopePlan(t, q)
	for i := 0; i < 300; i++ {
		p.WhereTree = &predicate.Node{Kind: "group", Correspondence: "generated", Children: []*predicate.Node{p.WhereTree}}
	}
	if AnalyzeWriteScope(p).Status != "unknown" {
		t.Fatal("deep tree accepted")
	}
	// Disabling/suppressing a diagnostic never supplies evidence.
	p = scopePlan(t, New(&recordingExec{}, "users", ormdriver.MySQLDialect{}).Where("id", 1).SuppressWarning(WarningBulkUpdateDetected, "reviewed externally"))
	if p.RiskLevel != RiskLow || p.WriteScope.Status != "unknown" {
		t.Fatalf("risk=%s scope=%+v", p.RiskLevel, p.WriteScope)
	}
	disabled := false
	r := NewRiskEngine(RiskConfig{Rules: map[string]RiskRuleConfig{WarningBulkUpdateDetected: {Enabled: &disabled}}}).CheckQuery(p)
	if r.Level != RiskLow || AnalyzeWriteScope(p).Status != "unknown" {
		t.Fatal("diagnostic configuration supplied proof")
	}
}

type scopeCaptureExec struct {
	recordingExec
	args []any
	sql  string
}

func (e *scopeCaptureExec) ExecContext(_ context.Context, sql string, args ...any) (sql.Result, error) {
	e.calls++
	e.sql = sql
	e.args = append([]any(nil), args...)
	return driver.RowsAffected(1), nil
}
func (e *scopeCaptureExec) Exec(sql string, args ...any) (sql.Result, error) {
	return e.ExecContext(context.Background(), sql, args...)
}
func TestWriteScopePreservesExecutorParameters(t *testing.T) {
	e := &scopeCaptureExec{}
	q := New(e, "users", ormdriver.MySQLDialect{}).WithWriteKeyContext(scopeContext("mysql", "id")).Where("id", uint16(2)).OrWhere("id", int8(2))
	p := scopePlan(t, q)
	if e.calls != 0 {
		t.Fatal("planning executed SQL")
	}
	if _, err := q.Update(map[string]any{"score": 1}); err != nil {
		t.Fatal(err)
	}
	if e.calls != 1 || e.sql != p.SQL || !reflect.DeepEqual(e.args, []any{1, uint16(2), int8(2)}) {
		t.Fatalf("calls=%d args=%#v", e.calls, e.args)
	}
}
