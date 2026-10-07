package operation

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type callbackInput struct{ called *bool }

func (v callbackInput) MarshalJSON() ([]byte, error) { *v.called = true; return []byte(`1`), nil }
func TestOperationSharedBudgetBoundaries(t *testing.T) {
	spec, opts := typedFixture("bigint", "postgres")
	raw, _ := json.Marshal(map[string]any{"spec": spec, "values": map[string]any{"unused": ""}})
	for _, delta := range []int{-1, 0, 1} {
		opts.Values = map[string]any{"unused": strings.Repeat("x", 1048576-len(raw)+delta)}
		_, e := Compile(t.Context(), spec, opts)
		if (e == nil) != (delta <= 0) {
			t.Fatal("shared byte budget", delta, e)
		}
	}
	for _, n := range []int{16368, 16369, 16370} {
		opts.Values = map[string]any{"unused": make([]any, n)}
		_, e := Compile(t.Context(), spec, opts)
		if (e == nil) != (n <= 16369) {
			t.Fatal("shared node budget", n, e)
		}
	}
	for _, n := range []int{29, 30, 31} {
		var v any = "x"
		for i := 0; i < n; i++ {
			v = []any{v}
		}
		opts.Values = map[string]any{"unused": v}
		_, e := Compile(t.Context(), spec, opts)
		if (e == nil) != (n <= 30) {
			t.Fatal("wrapper depth budget", n, e)
		}
	}
	called := false
	opts.Values = map[string]any{"unused": callbackInput{&called}}
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrInputLimit) || called {
		t.Fatal("callback used")
	}
	opts.Values = map[string]any{}
	opts.Values["cycle"] = opts.Values
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrInputLimit) {
		t.Fatal("cycle accepted")
	}
	raw, _ = json.Marshal(spec)
	opts.Values = nil
	for _, delta := range []int{0, 1} {
		var decoded OperationSpec
		b := []byte("{" + strings.Repeat(" ", 1048576-23-len(raw)+delta) + string(raw[1:]))
		if e := json.Unmarshal(b, &decoded); e != nil {
			t.Fatal(e)
		}
		_, e := Compile(t.Context(), decoded, opts)
		if (e == nil) != (delta == 0) {
			t.Fatal("raw spec overhead", delta, e)
		}
	}
}
