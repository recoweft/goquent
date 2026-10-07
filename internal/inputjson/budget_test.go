package inputjson

import (
	"encoding/json"
	"strings"
	"testing"
)

type opaque struct{ called *bool }

func (o opaque) MarshalJSON() ([]byte, error) { *o.called = true; return []byte(`0`), nil }
func TestBudgetsAndOpaque(t *testing.T) {
	for _, n := range []int{MaxBytes - 3, MaxBytes - 2, MaxBytes - 1} {
		_, e := Size(strings.Repeat("a", n))
		if (e == nil) != (n <= MaxBytes-2) {
			t.Fatal(n, e)
		}
	}
	for _, n := range []int{MaxNodes - 2, MaxNodes - 1, MaxNodes} {
		_, e := Size(make([]any, n))
		if (e == nil) != (n < MaxNodes) {
			t.Fatal(n, e)
		}
	}
	for _, n := range []int{31, 32, 33} {
		var v any = nil
		for i := 0; i < n; i++ {
			v = []any{v}
		}
		_, e := Size(v)
		if (e == nil) != (n <= 32) {
			t.Fatal(n, e)
		}
	}
	called := false
	if _, e := Size(opaque{&called}); e == nil || called {
		t.Fatal("opaque method used")
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	if _, e := Size(cycle); e == nil {
		t.Fatal("cycle")
	}
	for _, b := range []string{`{"n":1,"N":2}`, `{"n":1,"n":2}`, `{} {}`, strings.Repeat(" ", MaxBytes) + `0`} {
		if CheckJSON([]byte(b)) == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
	for _, v := range []any{json.Number("9007199254740993"), "猫<&\n", map[string]any{"x": []any{false, nil, 1}}} {
		n, e := Size(v)
		b, _ := json.Marshal(v)
		if e != nil || n != len(b) {
			t.Fatal("byte count", n, len(b), e)
		}
	}
}
