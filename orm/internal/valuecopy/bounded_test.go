package valuecopy

import (
	"github.com/recoweft/goquent/orm/predicate"
	"reflect"
	"strconv"
	"testing"
)

func cycleValues() []any {
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
	mix := map[string]any{}
	x := []any{mix, mix}
	mix["a"] = x
	mix["b"] = x
	return []any{m, s, a, mix}
}
func TestBranchingCyclesHaveBoundedCopies(t *testing.T) {
	for i, v := range cycleValues() {
		c := New()
		out, ok := c.Copy(v)
		if ok || reflect.TypeOf(out) != reflect.TypeOf(v) {
			t.Fatalf("case %d: lost opaque type", i)
		}
		if len(c.memo) > 2 || c.slots > 6 {
			t.Fatalf("case %d: expanded cycle: %d/%d", i, len(c.memo), c.slots)
		}
		// Allocation count is deterministic evidence in addition to the charged budget.
		n := testing.AllocsPerRun(10, func() { Copy(v) })
		if n > 40 {
			t.Fatalf("case %d: allocations=%g", i, n)
		}
		t.Logf("case %d: memo=%d slots=%d allocations=%g", i, len(c.memo), c.slots, n)
	}
}
func TestSharedDAGMemoAndIndependentCopies(t *testing.T) {
	leaf := []byte{1}
	var v any = leaf
	for i := 0; i < 30; i++ {
		v = []any{v, v}
	}
	c := New()
	out, ok := c.Copy(v)
	if !ok || len(c.memo) != 31 || c.slots != 91 {
		t.Fatalf("DAG: isolated=%v memo=%d slots=%d", ok, len(c.memo), c.slots)
	}
	root := out.([]any)
	if reflect.ValueOf(root[0]).Pointer() != reflect.ValueOf(root[1]).Pointer() {
		t.Fatal("shared child expanded")
	}
	n := testing.AllocsPerRun(10, func() { Copy(v) })
	if n > 250 {
		t.Fatalf("DAG allocations=%g", n)
	}
	t.Logf("30-level branching DAG: memo=%d slots=%d allocations=%g", len(c.memo), c.slots, n)
	other, _ := Copy(v)
	descend := func(x any) []byte {
		for i := 0; i < 30; i++ {
			x = x.([]any)[0]
		}
		return x.([]byte)
	}
	descend(out)[0] = 9
	if leaf[0] != 1 || descend(other)[0] != 1 {
		t.Fatal("copies share mutable state")
	}
}
func TestCopyBudgetsAndDepth(t *testing.T) {
	for _, n := range []int{MaxDepth, MaxDepth + 1} {
		var v any = 1
		for i := 0; i < n; i++ {
			v = []any{v}
		}
		_, ok := Copy(v)
		if ok != (n == MaxDepth) {
			t.Fatalf("depth %d isolated=%v", n, ok)
		}
	}
	for _, n := range []int{MaxSlots - 1, MaxSlots, MaxSlots + 1000} {
		v := make([]any, n)
		c := New()
		out, ok := c.Copy(v)
		if ok != (n == MaxSlots-1) {
			t.Fatalf("slots %d isolated=%v", n, ok)
		}
		if !ok && (c.slots != 0 || reflect.ValueOf(out).Pointer() != reflect.ValueOf(v).Pointer()) {
			t.Fatal("over-budget allocation or lost reference")
		}
	}
	for _, n := range []int{MaxBytes, MaxBytes + 1} {
		v := make([]byte, n)
		c := New()
		_, ok := c.Copy(v)
		if ok != (n == MaxBytes) {
			t.Fatalf("bytes %d isolated=%v", n, ok)
		}
		if !ok && c.bytes != 0 {
			t.Fatal("reserved excessive bytes")
		}
	}
	c := New()
	for i := 0; i < 3; i++ {
		_, ok := c.Copy(make([]byte, MaxBytes/2))
		if ok != (i < 2) {
			t.Fatalf("aggregate argument budget %d", i)
		}
	}
	// Node must not silently preserve a detached label after its aggregate budget fails.
	data := make([]byte, MaxBytes/2+1)
	node := &predicate.Node{Correspondence: "generated", Children: []*predicate.Node{
		{Correspondence: "generated", Values: []predicate.Value{{Isolation: "detached", Data: data}}},
		{Correspondence: "generated", Values: []predicate.Value{{Isolation: "detached", Data: append([]byte(nil), data...)}}},
	}}
	out := Node(node)
	if out.Correspondence != "unverified" || out.Children[1].Values[0].Isolation != "unverified" || out.Children[1].Values[0].Reason != UnverifiedReason {
		t.Fatal("budget failure not propagated")
	}
}

func TestNodeGraphMemo(t *testing.T) {
	n := &predicate.Node{Kind: "comparison", Correspondence: "generated", Values: []predicate.Value{{Isolation: "detached", Data: []byte{1}}}}
	for i := 0; i < 30; i++ {
		n = &predicate.Node{Kind: "and", Correspondence: "generated", Children: []*predicate.Node{n, n}}
	}
	c := New()
	out := c.Node(n)
	if len(c.nodes) != 31 || out.Children[0] != out.Children[1] || out == n {
		t.Fatal("condition graph expanded or shared with source")
	}
	cyc := &predicate.Node{Kind: "and", Correspondence: "generated"}
	cyc.Children = []*predicate.Node{cyc, cyc}
	copied := Node(cyc)
	if copied.Correspondence != "unverified" || copied.Children[0] != copied {
		t.Fatal("condition cycle not bounded")
	}
}

func TestWideMapAndDeeperMemoOccurrence(t *testing.T) {
	for _, n := range []int{MaxSlots - 1, MaxSlots} {
		m := make(map[string]any, n)
		for i := 0; i < n; i++ {
			m[strconv.Itoa(i)] = i
		}
		c := New()
		out, ok := c.Copy(m)
		if ok != (n == MaxSlots-1) {
			t.Fatalf("map slots %d isolated=%v", n, ok)
		}
		if !ok && (c.slots != 0 || reflect.ValueOf(out).Pointer() != reflect.ValueOf(m).Pointer()) {
			t.Fatal("wide map allocated or changed")
		}
	}
	var child any = 1
	for i := 0; i < 10; i++ {
		child = []any{child}
	}
	var deeper any = child
	for i := 0; i < 55; i++ {
		deeper = []any{deeper}
	}
	_, ok := Copy([]any{child, deeper})
	if ok {
		t.Fatal("memo bypassed depth at deeper occurrence")
	}
}
