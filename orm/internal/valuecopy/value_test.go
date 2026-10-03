package valuecopy

import (
	"reflect"
	"testing"
)

func TestCopyIsolationAndOpaqueDescendants(t *testing.T) {
	bytes := []byte{1}
	nested := map[string]any{"nested": []any{bytes, nil, []byte(nil)}}
	v, ok := Copy(nested)
	if !ok {
		t.Fatal("supported container")
	}
	bytes[0] = 9
	if !reflect.DeepEqual(v.(map[string]any)["nested"].([]any)[0], []byte{1}) {
		t.Fatal("shared bytes")
	}
	pointer := new(int)
	nested["opaque"] = pointer
	v, ok = Copy(nested)
	if ok || v.(map[string]any)["opaque"] != pointer {
		t.Fatal("opaque descendant lost")
	}
	var typedNil *int
	v, ok = Copy(typedNil)
	if ok || reflect.TypeOf(v) != reflect.TypeOf(typedNil) {
		t.Fatal("typed nil changed")
	}
	type custom int
	v, ok = Copy(custom(3))
	if ok || v != custom(3) {
		t.Fatal("named type normalized")
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	_, ok = Copy(cycle)
	if ok {
		t.Fatal("cycle falsely isolated")
	}
}
