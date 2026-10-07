package query

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/internal/valueguard"
	"github.com/recoweft/goquent/orm/predicate"
)

func recursivePayloads() map[string]any {
	m := map[string]any{}
	m["a"] = m
	m["b"] = m
	s := make([]any, 2)
	s[0] = s
	s[1] = s
	a, b := map[string]any{}, map[string]any{}
	a["a"] = b
	a["b"] = b
	b["a"] = a
	b["b"] = a
	mixed := map[string]any{}
	x := []any{mixed, mixed}
	mixed["a"] = x
	mixed["b"] = x
	return map[string]any{"map": m, "slice": s, "mutual": a, "mixed": mixed}
}
func TestRecursivePayloadsAcrossBuilderAndPlan(t *testing.T) {
	for name, v := range recursivePayloads() {
		t.Run(name, func(t *testing.T) {
			q := newPlanTestQuery(&recordingExec{}).WhereGroup(func(q *Query) { q.Where("payload", v) }).Where("id", 1)
			snapshot := q.builder.Snapshot()
			if snapshot.Error != nil {
				t.Fatal(snapshot.Error)
			}
			copied := newSelectBuilder(q.dialect)
			q.builder.CopyStateToSelect(copied)
			cp, err := q.planSelectBuilder(context.Background(), copied)
			if err != nil {
				t.Fatal(err)
			}
			p, err := q.Plan(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			sql, args, built, err := q.builder.BuildSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			for _, tree := range []*predicate.Node{snapshot.WhereTree, built.WhereTree, p.WhereTree, cp.WhereTree} {
				if tree.Correspondence != "unverified" {
					t.Fatal("ancestor not unverified")
				}
				leaf := leaves(tree)[0]
				if leaf.Values[0].Isolation != "unverified" || leaf.Values[0].Reason == "" {
					t.Fatal("missing value reason")
				}
			}
			for _, plan := range []*QueryPlan{p, cp} {
				if plan.SQL != sql || len(plan.Params) != len(args) || reflect.TypeOf(plan.Params[0]) != reflect.TypeOf(v) {
					t.Fatal("SQL/argument contract changed")
				}
				if !strings.Contains(plan.String(), `"details_omitted":true`) {
					t.Fatal("missing safe String marker")
				}
				for _, marshal := range []func() ([]byte, error){plan.ToJSON, func() ([]byte, error) { return json.Marshal(plan) }, func() ([]byte, error) { return json.MarshalIndent(plan, "", "  ") }, func() ([]byte, error) { return json.Marshal(*plan) }} {
					_, err := marshal()
					var outputErr *OutputError
					if !errors.Is(err, ErrOutputCycle) || !errors.As(err, &outputErr) {
						t.Fatalf("expected typed cycle error: %v", err)
					}
				}
				if plan.SQL != sql || reflect.TypeOf(plan.Params[0]) != reflect.TypeOf(v) {
					t.Fatal("display changed execution data")
				}
			}
			for _, build := range []func(context.Context) (*QueryPlan, error){q.PlanDelete, func(ctx context.Context) (*QueryPlan, error) { return q.PlanUpdate(ctx, map[string]any{"name": "x"}) }} {
				p, err := build(context.Background())
				if err != nil || p.WhereTree.Correspondence != "unverified" {
					t.Fatal("write copy lost opaque condition")
				}
			}
		})
	}
}
func TestDAGOutputAndIndependentSnapshots(t *testing.T) {
	child := map[string]any{"bytes": []byte{1}, "nil": nil, "typed_nil": []byte(nil)}
	dag := map[string]any{"shared": []any{child, child}}
	q := newPlanTestQuery(&recordingExec{}).Where("payload", dag)
	p, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snap1, snap2 := q.builder.Snapshot(), q.builder.Snapshot()
	if p.WhereTree.Values[0].Isolation != "detached" {
		t.Fatal("DAG mistaken for cycle")
	}
	// Compare with the previous standard encoding and formatting for safe payloads.
	type legacy QueryPlan
	want, err := json.MarshalIndent(legacy(*p), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.ToJSON()
	if err != nil || string(got) != string(want) {
		t.Fatal("ordinary JSON changed")
	}
	if strings.Contains(p.String(), fmt.Sprintf("params: %v", p.Params)) {
		t.Fatal("public String leaked params")
	}
	first := snap1.WhereTree.Values[0].Data.(map[string]any)["shared"].([]any)
	first[0].(map[string]any)["bytes"].([]byte)[0] = 9
	if first[1].(map[string]any)["bytes"].([]byte)[0] != 9 {
		t.Fatal("within-copy sharing lost")
	}
	for _, v := range []any{dag, p.Params[0], snap2.WhereTree.Values[0].Data} {
		if v.(map[string]any)["shared"].([]any)[0].(map[string]any)["bytes"].([]byte)[0] != 1 {
			t.Fatal("independent snapshots share")
		}
	}
	var big any = 1
	for i := 0; i < 30; i++ {
		big = []any{big, big}
	}
	bigPlan, err := newPlanTestQuery(&recordingExec{}).Where("payload", big).Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if bigPlan.WhereTree.Values[0].Isolation != "detached" {
		t.Fatal("compact DAG not detached")
	}
	if _, err := bigPlan.ToJSON(); !errors.Is(err, ErrOutputBudget) {
		t.Fatalf("DAG output: %v", err)
	}
	if !strings.Contains(bigPlan.String(), `"details_omitted":true`) {
		t.Fatal("DAG String not bounded")
	}
}
func TestOutputAllPayloadLocationsAndNil(t *testing.T) {
	cycle := recursivePayloads()["map"]
	for _, p := range []*QueryPlan{
		{Params: []any{cycle}}, {Metadata: map[string]any{"x": cycle}},
		{Warnings: []Warning{{Evidence: []Evidence{{Value: cycle}}}}},
		{SuppressedWarnings: []Warning{{Evidence: []Evidence{{Value: cycle}}}}},
		{WhereTree: &predicate.Node{Values: []predicate.Value{{Data: cycle}}}},
		{HavingTree: &predicate.Node{NamedValues: map[string]predicate.Value{"x": {Data: cycle}}}},
	} {
		if _, err := p.ToJSON(); !errors.Is(err, ErrOutputCycle) {
			t.Fatalf("payload bypass: %v", err)
		}
	}
	for _, v := range []any{predicate.Value{Data: cycle}, predicate.Node{Values: []predicate.Value{{Data: cycle}}}} {
		if _, err := json.Marshal(v); !errors.Is(err, ErrOutputCycle) {
			t.Fatalf("standalone condition bypass: %v", err)
		}
	}
	var p *QueryPlan
	b, err := p.ToJSON()
	if err != nil || string(b) != "null" || p.String() != "PUBLIC_VIEW_SOURCE: unsupported diagnostic source" {
		t.Fatal("nil Plan contract")
	}
	var deep any = 1
	for i := 0; i < valueguard.MaxDepth+1; i++ {
		deep = []any{deep}
	}
	for _, tc := range []struct {
		v   any
		err error
	}{{deep, ErrOutputDepth}, {make([]any, valueguard.MaxNodes), ErrOutputBudget}, {strings.Repeat("x", valueguard.MaxBytes), ErrOutputBudget}} {
		p := &QueryPlan{Params: []any{tc.v}}
		if _, err := json.MarshalIndent(p, "", "  "); !errors.Is(err, tc.err) {
			t.Fatalf("output boundary: %v", err)
		}
		if !strings.Contains(p.String(), `"details_omitted":true`) {
			t.Fatal("missing marker")
		}
	}
}

type outputProbe struct{ jsonCalls, stringCalls, formatCalls int }

func (p *outputProbe) MarshalJSON() ([]byte, error) { p.jsonCalls++; return []byte(`"custom"`), nil }
func (p *outputProbe) String() string               { p.stringCalls++; return "custom" }
func (p *outputProbe) Format(s fmt.State, _ rune)   { p.formatCalls++; fmt.Fprint(s, "custom") }
func TestPreflightDoesNotCallCustomOutputMethods(t *testing.T) {
	v := &outputProbe{}
	p := &QueryPlan{Params: []any{v}, Metadata: map[string]any{"v": v}}
	if err := valueguard.Check(*p); err != nil {
		t.Fatal(err)
	}
	if v.jsonCalls+v.stringCalls+v.formatCalls != 0 {
		t.Fatal("preflight invoked user code")
	}
	if _, err := p.ToJSON(); err != nil || v.jsonCalls != 2 {
		t.Fatal("custom JSON evaluation changed")
	}
	_ = p.String()
	if v.formatCalls != 0 || v.stringCalls != 0 {
		t.Fatal("custom format timing changed")
	}
	p.Params = append(p.Params, recursivePayloads()["map"])
	if _, err := p.ToJSON(); !errors.Is(err, ErrOutputCycle) || v.jsonCalls != 2 {
		t.Fatal("failed preflight evaluated marshaler")
	}
	_ = p.String()
	if v.formatCalls != 0 {
		t.Fatal("marker formatted dangerous payload")
	}
	// Copy budget is shared by different arguments, not reset per argument.
	large := make([]byte, valuecopy.MaxBytes/2+1)
	q := newPlanTestQuery(&recordingExec{}).Where("a", large).Where("b", append([]byte(nil), large...))
	plan, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if plan.WhereTree.Correspondence != "unverified" {
		t.Fatal("aggregate copy budget not propagated")
	}
}

type failingOutput struct {
	calls int
	err   error
}

func (v *failingOutput) MarshalJSON() ([]byte, error) { v.calls++; return nil, v.err }

type plainStringer struct{ calls int }

func (v *plainStringer) String() string { v.calls++; return "normal-stringer" }
func TestCustomOutputErrorsAndStringerTiming(t *testing.T) {
	sentinel := errors.New("custom marshal failure")
	v := &failingOutput{err: sentinel}
	p := &QueryPlan{Params: []any{v}}
	if err := valueguard.Check(*p); err != nil || v.calls != 0 {
		t.Fatal("early custom marshal")
	}
	_, err := p.ToJSON()
	var preflight *OutputError
	if !errors.Is(err, sentinel) || errors.As(err, &preflight) || v.calls != 1 {
		t.Fatal("custom error identity/timing changed")
	}
	s := &plainStringer{}
	p.Params = []any{s}
	if err := valueguard.Check(*p); err != nil || s.calls != 0 {
		t.Fatal("early Stringer")
	}
	if strings.Contains(p.String(), "normal-stringer") || s.calls != 0 {
		t.Fatal("Stringer contract changed")
	}
}
