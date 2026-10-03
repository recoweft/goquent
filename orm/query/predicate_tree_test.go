package query

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	ormdriver "github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/predicate"
)

func leaves(n *predicate.Node) []*predicate.Node {
	if n == nil {
		return nil
	}
	if len(n.Children) == 0 {
		return []*predicate.Node{n}
	}
	var out []*predicate.Node
	for _, c := range n.Children {
		out = append(out, leaves(c)...)
	}
	return out
}
func shape(n *predicate.Node) string {
	if n == nil {
		return ""
	}
	s := n.Kind
	if len(n.Children) > 0 {
		var cs []string
		for _, c := range n.Children {
			cs = append(cs, shape(c))
		}
		s += "(" + strings.Join(cs, ",") + ")"
	}
	return s
}

// These are representation tests for C05-C11/C28, not cardinality verdicts.
func TestPredicateTreeSemanticCases(t *testing.T) {
	cases := []struct {
		name   string
		build  func(*Query) *Query
		shape  string
		params []any
	}{
		{"C05_range", func(q *Query) *Query { return q.Where("u.id", ">", 1) }, "comparison", []any{1}},
		{"C05_single_in", func(q *Query) *Query { return q.WhereIn("u.id", []int{1}) }, "in", []any{1}},
		{"C05_multiple_in", func(q *Query) *Query { return q.WhereIn("u.id", []int{1, 2}) }, "in", []any{1, 2}},
		{"C05_empty_in", func(q *Query) *Query { return q.WhereIn("u.id", []any{}) }, "in", nil},
		{"C06_precedence", func(q *Query) *Query { return q.Where("u.id", 1).OrWhere("u.name", "a").Where("u.age", 2) }, "or(comparison,and(comparison,comparison))", []any{1, "a", 2}},
		{"C07_unbound_or", func(q *Query) *Query {
			return q.WhereGroup(func(q *Query) { q.Where("u.tenant_id", 1).Where("u.id", 1) }).OrWhere("u.id", 2)
		}, "or(group(and(comparison,comparison)),comparison)", []any{1, 1, 2}},
		{"C08_negated_binding", func(q *Query) *Query { return q.WhereNot(func(q *Query) { q.Where("u.tenant_id", 1) }) }, "not(group(comparison))", []any{1}},
		{"C08_column_binding", func(q *Query) *Query { return q.WhereColumn("u.tenant_id", "=", "v.tenant_id") }, "column", nil},
		{"C09_literal_without_context", func(q *Query) *Query { return q.Where("u.tenant_id", 1) }, "comparison", []any{1}},
		{"C10_key_ranges", func(q *Query) *Query { return q.Where("u.tenant_id", ">", 1).Where("u.external_key", "<", 8) }, "and(comparison,comparison)", []any{1, 8}},

		{"C07_tenant", func(q *Query) *Query { return q.Where("u.tenant_id", 4).Where("u.id", 1) }, "and(comparison,comparison)", []any{4, 1}},
		{"C10_composite", func(q *Query) *Query { return q.Where("u.key_a", 1).Where("u.key_b", 2) }, "and(comparison,comparison)", []any{1, 2}},
		{"C11_not_nested", func(q *Query) *Query {
			return q.WhereNot(func(q *Query) {
				q.Where("u.id", 1).OrWhereGroup(func(q *Query) { q.WhereNull("u.name").Where("u.age", 2) })
			})
		}, "not(group(or(comparison,group(and(null,comparison)))))", []any{1, 2}},
		{"C11_null", func(q *Query) *Query { return q.WhereNull("u.name").OrWhereNotNull("u.age") }, "or(null,null)", nil},
		{"C11_bound_null", func(q *Query) *Query { return q.Where("u.name", "=", nil) }, "comparison", []any{nil}},
		{"C11_raw", func(q *Query) *Query {
			return q.SafeWhereRaw("u.id = :id OR u.age = :age", map[string]any{"id": 1, "age": 2})
		}, "opaque", []any{1, 2}},
		{"column_alias", func(q *Query) *Query { return q.WhereColumn("u.id", "=", "v.id") }, "column", nil},
		{"nested_after_or", func(q *Query) *Query {
			return q.Where("u.id", 1).OrWhereGroup(func(q *Query) {
				q.Where("u.age", 2).WhereNot(func(q *Query) { q.Where("u.name", "a").OrWhere("u.name", "b") })
			}).Where("u.active", true)
		}, "or(comparison,and(group(and(comparison,not(group(or(comparison,comparison))))),comparison))", []any{1, 2, "a", "b", true}},
	}
	for _, dialect := range []ormdriver.Dialect{ormdriver.MySQLDialect{}, ormdriver.PostgresDialect{}} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%T", dialect)+"/"+tc.name, func(t *testing.T) {
				e := &recordingExec{}
				q := tc.build(New(e, "users as u", dialect))
				p, err := q.Plan(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if got := shape(p.WhereTree); got != tc.shape {
					t.Fatalf("shape %s != %s; SQL %s", got, tc.shape, p.SQL)
				}
				if len(p.Params) != len(tc.params) {
					t.Fatalf("params %#v != %#v", p.Params, tc.params)
				}
				for i, want := range tc.params {
					if !reflect.DeepEqual(p.Params[i], want) {
						t.Fatalf("param %d: %#v != %#v", i, p.Params[i], want)
					}
				}
				index := 0
				for _, n := range leaves(p.WhereTree) {
					if n.Correspondence == "" || !strings.Contains(p.SQL, n.SQL) {
						t.Fatalf("missing generated SQL: %+v", n)
					}
					for i, position := range n.Parameters {
						if position != index || !reflect.DeepEqual(n.Values[i].Data, p.Params[position]) {
							t.Fatalf("parameter mapping: %+v", n)
						}
						index++
					}
				}
				if index != len(p.Params) {
					t.Fatalf("mapped %d of %d", index, len(p.Params))
				}
				if tc.name == "C05_empty_in" && p.WhereTree.OpaqueReason == "" {
					t.Fatal("empty IN must retain unknown semantics")
				}
				if tc.name == "column_alias" && (p.WhereTree.Column != "u.id" || p.WhereTree.ValueColumn != "v.id") {
					t.Fatal("lost aliases")
				}
				if e.calls != 0 {
					t.Fatal("planning executed SQL")
				}
				data, err := p.ToJSON()
				if err != nil {
					t.Fatal(err)
				}
				var decoded QueryPlan
				if err = json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				if shape(decoded.WhereTree) != tc.shape || len(decoded.Predicates) != len(p.Predicates) {
					t.Fatal("JSON compatibility")
				}
			})
		}
	}
}

func TestPredicateParameterOffsetsAndHaving(t *testing.T) {
	for _, d := range []ormdriver.Dialect{ormdriver.MySQLDialect{}, ormdriver.PostgresDialect{}} {
		q := New(&recordingExec{}, "users", d).SelectRaw("? AS marker", 9).Where("age", 20).GroupBy("id").Having("id", ">", 2).OrHaving("id", "<", 8)
		p, err := q.Plan(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Params, []any{9, 20, 2, 8}) || p.WhereTree.Parameters[0] != 1 || leaves(p.HavingTree)[0].Parameters[0] != 2 || shape(p.HavingTree) != "or(comparison,comparison)" {
			t.Fatalf("bad trace %+v", p)
		}
		u, err := q.PlanUpdate(context.Background(), map[string]any{"age": 3})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(u.Params, []any{3, 20}) || u.WhereTree.Parameters[0] != 1 || u.HavingTree != nil {
			t.Fatalf("update trace %+v", u)
		}
		del, err := q.PlanDelete(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(del.Params, []any{20}) || del.WhereTree.Parameters[0] != 0 {
			t.Fatalf("delete trace %+v", del)
		}
	}
}

func TestPredicateSnapshotsDetachSupportedValues(t *testing.T) {
	data := []byte{1, 2}
	q := newPlanTestQuery(&recordingExec{}).WhereGroup(func(q *Query) { q.Where("payload", data) }).Where("id", 1)
	p, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := q.builder.Snapshot()
	copied := newSelectBuilder(q.dialect)
	q.builder.CopyStateToSelect(copied)
	data[0] = 9
	q.Where("age", 4)
	cp, err := q.planSelectBuilder(context.Background(), copied)
	if err != nil {
		t.Fatal(err)
	}
	for _, plan := range []*QueryPlan{p, cp} {
		if !reflect.DeepEqual(plan.Params[0], []byte{1, 2}) || len(plan.Predicates) != 2 {
			t.Fatalf("shared plan: %+v", plan)
		}
	}
	if !reflect.DeepEqual(leaves(snapshot.WhereTree)[0].Values[0].Data, []byte{1, 2}) {
		t.Fatal("shared snapshot")
	}
	leaves(snapshot.WhereTree)[0].Values[0].Data.([]byte)[0] = 7
	if !reflect.DeepEqual(leaves(p.WhereTree)[0].Values[0].Data, []byte{1, 2}) {
		t.Fatal("snapshot shares plan")
	}
}

func TestPredicateDepthBoundary(t *testing.T) {
	var nest func(*Query, int)
	nest = func(q *Query, n int) {
		if n == 0 {
			q.Where("id", 1)
			return
		}
		q.WhereGroup(func(g *Query) { nest(g, n-1) })
	}
	for _, n := range []int{predicate.MaxDepth - 1, predicate.MaxDepth, predicate.MaxDepth + 1, 10000} {
		q := newPlanTestQuery(&recordingExec{})
		nest(q, n)
		_, _, buildErr := q.Build()
		_, planErr := q.Plan(context.Background())
		_, updateErr := q.PlanUpdate(context.Background(), map[string]any{"age": 1})
		_, deleteErr := q.PlanDelete(context.Background())
		for _, err := range []error{buildErr, planErr, updateErr, deleteErr} {
			if n > predicate.MaxDepth {
				if !errors.Is(err, predicate.ErrDepth) {
					t.Fatalf("depth %d: %v", n, err)
				}
			} else if err != nil {
				t.Fatalf("depth %d: %v", n, err)
			}
		}
	}
}

type deferredValue struct {
	calls int
	value string
	err   error
}

func (v *deferredValue) Value() (driver.Value, error) { v.calls++; return v.value, v.err }

type customValueExec struct {
	recordingExec
	received any
}

func (e *customValueExec) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	e.received = args[0]
	v, err := args[0].(*deferredValue).Value()
	_ = v
	return driver.RowsAffected(1), err
}
func (e *customValueExec) Exec(s string, args ...any) (sql.Result, error) {
	return e.ExecContext(context.Background(), s, args...)
}
func TestPredicateOpaqueValuePreservesExecutorContract(t *testing.T) {
	failure := errors.New("valuer error")
	v := &deferredValue{value: "before", err: failure}
	exec := &customValueExec{}
	q := New(exec, "users", ormdriver.MySQLDialect{}).Where("id", v)
	p, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = q.Build()
	if err != nil {
		t.Fatal(err)
	}
	_ = q.builder.Snapshot()
	if v.calls != 0 || p.Params[0] != v || p.WhereTree.Values[0].Isolation != "unverified" || p.WhereTree.Values[0].Data != nil {
		t.Fatalf("early evaluation or false isolation: %+v", p.WhereTree)
	}
	v.value = "after"
	_, err = exec.Exec(p.SQL, p.Params...)
	if !errors.Is(err, failure) || v.calls != 1 || exec.received != v {
		t.Fatal("changed executor type/timing/error")
	}
}

func TestPredicateRefIsGeneratedCompatibilityView(t *testing.T) {
	p, err := newPlanTestQuery(&recordingExec{}).Where("age", 20).Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	p.Predicates = []PredicateRef{{Column: "invented"}}
	if PlanHasPredicateColumn(p, "", "invented") || !PlanHasPredicateColumn(p, "", "age") {
		t.Fatal("display metadata became a second source")
	}
	old := &QueryPlan{Predicates: []PredicateRef{{Column: "age"}}}
	if !PlanHasPredicateColumn(old, "", "age") {
		t.Fatal("legacy input compatibility changed")
	}
	data, _ := p.ToJSON()
	var decoded QueryPlan
	if err = json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.generatedConditions || decoded.conditionSource != nil {
		t.Fatal("JSON created trusted generation state")
	}
}

func TestPredicateUnionAndCopyOffsets(t *testing.T) {
	q := newPlanTestQuery(&recordingExec{}).Where("id", 2)
	sub := newPlanTestQuery(&recordingExec{}).Where("id", 1)
	q.Union(sub)
	p, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	copy := newSelectBuilder(q.dialect)
	q.builder.CopyStateToSelect(copy)
	sub.Where("age", 99)
	q.Where("age", 88)
	cp, err := q.planSelectBuilder(context.Background(), copy)
	if err != nil {
		t.Fatal(err)
	}
	if p.SQL != cp.SQL || !reflect.DeepEqual(p.Params, cp.Params) || cp.WhereTree.Parameters[0] != 1 || len(cp.Unverified) == 0 {
		t.Fatal("union copy or parameter offset lost")
	}
}

func TestPredicateOpaqueDescendantAndMetadataCopy(t *testing.T) {
	v := &deferredValue{}
	q := newPlanTestQuery(&recordingExec{}).Where("payload", map[string]any{"nested": []any{v}})
	p, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if p.WhereTree.Values[0].Isolation != "unverified" || v.calls != 0 {
		t.Fatal("opaque descendant considered detached")
	}
	md := []TableRiskMetadata{{Table: "users", PrimaryKeyColumns: []string{"id"}, UniqueIndexes: [][]string{{"tenant", "key"}}, RequiredFilterColumns: []string{"tenant"}}}
	AttachTableRiskMetadata(p, md)
	md[0].PrimaryKeyColumns[0] = "changed"
	md[0].UniqueIndexes[0][0] = "changed"
	md[0].RequiredFilterColumns[0] = "changed"
	got := p.Metadata[MetadataTableRisk].([]TableRiskMetadata)[0]
	if got.PrimaryKeyColumns[0] != "id" || got.UniqueIndexes[0][0] != "tenant" || got.RequiredFilterColumns[0] != "tenant" {
		t.Fatal("shared key metadata")
	}
}

func TestPredicateSubqueryOpaqueAndRawRepeatedBinding(t *testing.T) {
	for _, d := range []ormdriver.Dialect{ormdriver.MySQLDialect{}, ormdriver.PostgresDialect{}} {
		sub := New(&recordingExec{}, "children", d).Select("parent_id").Where("name", "child")
		q := New(&recordingExec{}, "users", d).WhereInSubQuery("id", sub).SafeWhereRaw("age > :a OR age < :a", map[string]any{"a": 20})
		p, err := q.Plan(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		ns := leaves(p.WhereTree)
		if len(ns) != 2 || ns[0].Kind != "opaque" || ns[0].OpaqueReason != "subquery" || ns[1].Kind != "opaque" || !reflect.DeepEqual(p.Params, []any{"child", 20, 20}) || !reflect.DeepEqual(ns[1].Parameters, []int{1, 2}) {
			t.Fatalf("opaque mapping: %+v", p)
		}
	}
}

type observedJSON struct{ calls *int }

func (v observedJSON) MarshalJSON() ([]byte, error) { *v.calls++; return []byte(`{"k":1}`), nil }
func TestPredicateSnapshotDoesNotEvaluateJSON(t *testing.T) {
	calls := 0
	q := newPlanTestQuery(&recordingExec{})
	q.builder.GetWhereBuilder().WhereJsonContains("payload", observedJSON{&calls})
	snapshot := q.builder.Snapshot()
	if snapshot.Error != nil || calls != 0 || snapshot.WhereTree.Correspondence != "unverified" {
		t.Fatal("snapshot evaluated user JSON code")
	}
	_, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("Plan evaluated JSON %d times", calls)
	}
}

func TestPredicateSnapshotBeforeBuildPreservesRawBindings(t *testing.T) {
	b := []byte{1}
	q := newPlanTestQuery(&recordingExec{}).SafeWhereRaw("payload = :p", map[string]any{"p": b})
	snapshot := q.builder.Snapshot()
	b[0] = 2
	if snapshot.Error != nil || snapshot.WhereTree.Correspondence != "unverified" || len(snapshot.WhereTree.Parameters) != 0 || !reflect.DeepEqual(snapshot.WhereTree.NamedValues["p"].Data, []byte{1}) {
		t.Fatal("unrendered snapshot lost binding or claimed correspondence")
	}
}
