package valuecopy

import (
	"reflect"
	"testing"
)

func TestSliceOwnsContainerAfterBudgetExhaustion(t *testing.T) {
	for _, n := range []int{MaxSlots - 1, MaxSlots, MaxSlots + 1} {
		source := make([]any, n)
		for i := range source {
			source[i] = i
		}
		c := New()
		// Exhaust the budget before cloning a library-owned argument list.
		if _, ok := c.Copy(make([]any, MaxSlots-1)); !ok {
			t.Fatal("budget setup")
		}
		out := c.Slice(source)
		second := c.Slice(source)
		for i := range source {
			source[i] = nil
		}
		for i := range out {
			if out[i] != i || second[i] != i {
				t.Fatalf("cleanup destroyed argument %d", i)
			}
			out[i] = "mutated"
			if second[i] != i {
				t.Fatal("outer memo shared library containers")
			}
		}
	}
	if Slice(nil) != nil || Slice([]any{}) == nil {
		t.Fatal("nil/empty distinction lost")
	}
}

func TestSlicePayloadBudgetAndMemo(t *testing.T) {
	a, b := make([]byte, MaxBytes/2+1), make([]byte, MaxBytes/2+1)
	opaque := make([]any, MaxSlots)
	c := New()
	out := c.Slice([]any{a, a, b, opaque, nil, []byte(nil)})
	if &out[0].([]byte)[0] == &a[0] || &out[0].([]byte)[0] != &out[1].([]byte)[0] {
		t.Fatal("payload memo/isolation lost")
	}
	if &out[2].([]byte)[0] != &b[0] || &out[3].([]any)[0] != &opaque[0] {
		t.Fatal("refused payload reference lost")
	}
	if c.bytes != len(a) || c.slots != 1 {
		t.Fatal("payload budget reset or outer container charged")
	}
	if out[4] != nil || !reflect.DeepEqual(out[5], []byte(nil)) {
		t.Fatal("NULL types changed")
	}
	independent := Slice([]any{a})
	out[0].([]byte)[0] = 9
	if independent[0].([]byte)[0] != 0 {
		t.Fatal("independent payload shared")
	}
}
