package valueguard

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestBudgets(t *testing.T) {
	for _, n := range []int{MaxDepth, MaxDepth + 1} {
		var v any = 1
		for i := 0; i < n; i++ {
			v = []any{v}
		}
		err := Check(v)
		if (err == nil) != (n == MaxDepth) {
			t.Fatalf("depth %d: %v", n, err)
		}
	}
	for _, n := range []int{MaxNodes - 1, MaxNodes} {
		err := Check(make([]any, n))
		if (err == nil) != (n == MaxNodes-1) {
			t.Fatalf("nodes %d: %v", n, err)
		}
	}
	for _, n := range []int{(MaxBytes - 32) / 6, (MaxBytes-32)/6 + 1} {
		err := Check(strings.Repeat("x", n))
		if (err == nil) != (n == (MaxBytes-32)/6) {
			t.Fatalf("bytes %d: %v", n, err)
		}
	}
	var dag any = 1
	for i := 0; i < 40; i++ {
		dag = []any{dag, dag}
	}
	if !errors.Is(Check(dag), ErrBudget) {
		t.Fatal("DAG not bounded")
	}
	n := testing.AllocsPerRun(10, func() { Check(dag) })
	if n > 250 {
		t.Fatalf("preflight allocates per expansion: %g", n)
	}
	t.Logf("40-level DAG preflight allocations=%g", n)
}

func TestExpandedAggregateAndMapBoundaries(t *testing.T) {
	for _, n := range []int{(MaxNodes - 1) / 2, (MaxNodes-1)/2 + 1} {
		m := make(map[string]any, n)
		for i := 0; i < n; i++ {
			m[strconv.Itoa(i)] = nil
		}
		err := Check(m)
		if (err == nil) != (n == (MaxNodes-1)/2) {
			t.Fatalf("map nodes %d: %v", n, err)
		}
	}
	shared := make([]any, MaxNodes/2)
	if err := Check(shared); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(Check([]any{shared, shared}), ErrBudget) {
		t.Fatal("repeated argument expansion undercharged")
	}
}
