package mcp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/publicoutput"
)

// Explicit expectations keep this useful after both entry points share a scanner.
func TestSharedRawJSONContract(t *testing.T) {
	if maxMessageBytes != inputjson.MaxBytes {
		t.Fatal("transport and shared raw byte limits diverged")
	}
	check := func(name string, b []byte, want bool) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if validateJSON(b) != want {
				t.Fatal("raw acceptance changed")
			}
			err := inputjson.CheckJSON(b)
			if want && err != nil || !want && err != inputjson.ErrInvalid {
				t.Fatal("shared error identity changed")
			}
		})
	}
	for _, n := range []int{1048575, 1048576, 1048577} {
		b := bytes.Repeat([]byte(" "), n)
		b[0] = '0'
		check(fmt.Sprintf("bytes/%d", n), b, n <= 1048576)
	}
	for _, n := range []int{31, 32, 33} {
		check(fmt.Sprintf("depth/%d", n), []byte(strings.Repeat("[", n)+"0"+strings.Repeat("]", n)), n <= 32)
	}
	for _, n := range []int{16383, 16384, 16385} {
		check(fmt.Sprintf("array_nodes/%d", n), []byte("[0"+strings.Repeat(",0", n-2)+"]"), n <= 16384)
		// Keys and closing delimiters are not value nodes; the root object is one.
		var b strings.Builder
		b.WriteByte('{')
		for i := 0; i < n-1; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "\"k%d\":null", i)
		}
		b.WriteByte('}')
		check(fmt.Sprintf("object_nodes/%d", n), []byte(b.String()), n <= 16384)
	}
	for _, tc := range []struct {
		name, raw string
		want      bool
	}{
		{"null", `null`, true}, {"unknown_version", `{"unknown":null,"version":999}`, true},
		{"large_number", `9007199254740993`, true}, {"large_exponent", `1e999999999999`, true},
		{"uppercase_single", `{"A":0}`, true}, {"case_duplicate", `{"a":0,"A":1}`, false},
		{"escape_duplicate", `{"a":0,"\u0061":1}`, false},
		{"unicode_lower_duplicate", `{"K":0,"K":1}`, false},
		{"not_equal_fold", `{"Σ":0,"ς":1}`, true},
		{"surrogate", `"\ud800"`, true},
		{"decoded_replacement_duplicate", `{"\ud800":0,"\ufffd":1}`, false},
		{"separate_objects", `[{"a":0},{"a":1}]`, true},
		{"trailing", `{} {}`, false}, {"empty", "", false}, {"number_syntax", `01`, false},
		{"invalid_utf8", string([]byte{'"', 255, '"'}), false},
	} {
		check(tc.name, []byte(tc.raw), tc.want)
	}
}

func TestSharedRawJSONRetainsDirectValueDifferences(t *testing.T) {
	// These are valid Operation primitives but deliberately outside MCP's direct
	// type/estimated-size contract. Delegating raw JSON must not unify these walks.
	for _, v := range []any{float32(1), []int{1}, strings.Repeat("x", 1048576/6)} {
		if _, err := inputjson.Size(v); err != nil {
			t.Fatal("Operation primitive refused")
		}
		if _, err := primitiveJSON(v); err != publicoutput.ErrOutput {
			t.Fatal("MCP direct contract changed")
		}
	}
}
