package inputjson

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

// Produces bytes on demand: no oversized fixture is held in memory or on disk.
type countedSource struct{ remaining, read int }

func (r *countedSource) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	for i := range p[:n] {
		p[i] = ' '
	}
	r.remaining -= n
	r.read += n
	return n, nil
}
func TestSecurityInputSourceReadBudget(t *testing.T) {
	for _, n := range []int{MaxSourceBytes - 1, MaxSourceBytes, MaxSourceBytes + 1, MaxSourceBytes * 2} {
		r := &countedSource{remaining: n}
		b, err := readSource(r)
		if r.read != min(n, MaxSourceBytes+1) {
			t.Fatal("read budget changed")
		}
		if n > MaxSourceBytes {
			if err != ErrInvalid || b != nil {
				t.Fatal("oversized source retained or accepted")
			}
		} else if err != nil || len(b) != n {
			t.Fatal("bounded source rejected")
		}
	}
}
func TestSecurityInputSourceReadErrors(t *testing.T) {
	source := []byte(`{"version":1}`)
	for _, wrap := range []func(io.Reader) io.Reader{iotest.OneByteReader, iotest.HalfReader, iotest.DataErrReader} {
		b, e := readSource(wrap(bytes.NewReader(source)))
		if e != nil || !bytes.Equal(b, source) {
			t.Fatal("short read or final data+EOF lost")
		}
	}
	injected := errors.New("fixture read error")
	for _, e := range []error{injected, io.ErrUnexpectedEOF} {
		b, err := readSource(io.MultiReader(bytes.NewReader(source), iotest.ErrReader(e)))
		if err != e || b != nil {
			t.Fatal("read failure converted to success or retained bytes")
		}
	}
	// A plain EOF is not a length promise: incomplete JSON is rejected by decode.
	b, e := readSource(bytes.NewReader([]byte(`{"`)))
	if e != nil || CheckJSON(b) != ErrInvalid {
		t.Fatal("truncated document semantics changed")
	}
}

func TestSecurityInputJSONStructuralBoundaries(t *testing.T) {
	for _, n := range []int{MaxBytes - 1, MaxBytes, MaxBytes + 1} {
		b := bytes.Repeat([]byte(" "), n)
		b[0] = '0'
		if (CheckJSON(b) == nil) != (n <= MaxBytes) {
			t.Fatal("raw byte boundary")
		}
	}
	for _, n := range []int{MaxDepth - 1, MaxDepth, MaxDepth + 1} {
		b := append(bytes.Repeat([]byte("["), n), '0')
		b = append(b, bytes.Repeat([]byte("]"), n)...)
		if (CheckJSON(b) == nil) != (n <= MaxDepth) {
			t.Fatal("depth boundary")
		}
	}
	for _, n := range []int{MaxNodes - 1, MaxNodes, MaxNodes + 1} {
		b := []byte("[0")
		b = append(b, bytes.Repeat([]byte(",0"), n-2)...)
		b = append(b, ']')
		if (CheckJSON(b) == nil) != (n <= MaxNodes) {
			t.Fatal("value node boundary")
		}
	}
	for _, b := range [][]byte{[]byte(`{"a":0,"A":1}`), []byte(`{"a":0,"\u0061":1}`), []byte(`{} {}`), []byte{'"', 255, '"'}, []byte(`01`)} {
		if CheckJSON(b) != ErrInvalid {
			t.Fatal("ambiguous or malformed JSON accepted")
		}
	}
	// The raw checker intentionally leaves null, unknown keys and number meaning
	// to the consuming typed decoder; do not silently turn it into a new schema.
	for _, b := range [][]byte{[]byte(`null`), []byte(`{"unknown":null}`), []byte(`1e999999999999`)} {
		if CheckJSON(b) != nil {
			t.Fatal("raw checker changed typed-decoder responsibility")
		}
	}
}
