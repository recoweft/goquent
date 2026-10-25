package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/publicoutput"
)

func TestSecurityInputIncompleteFrame(t *testing.T) {
	for _, src := range []string{"Content-Length: 1\r\n", "Content-Length: 1\r\nContent-Type: application/json\r\n"} {
		var out bytes.Buffer
		err := NewServer(Options{}).Serve(context.Background(), strings.NewReader(src), &out)
		if err != publicoutput.ErrOutput || out.Len() != 0 {
			t.Fatal("incomplete frame must fail without dispatch")
		}
	}
}

// Counts bytes delivered to the transport, including bounded bufio read-ahead.
type inputCountReader struct {
	r io.Reader
	n int
}

func (r *inputCountReader) Read(p []byte) (int, error) { n, e := r.r.Read(p); r.n += n; return n, e }

func TestSecurityInputFramingBoundaries(t *testing.T) {
	for _, n := range []int{maxMessageBytes - 1, maxMessageBytes, maxMessageBytes + 1} {
		// Newline is part of the line/header budget, not the returned payload.
		src := strings.Repeat(" ", n-2) + "0\n"
		r := &inputCountReader{r: strings.NewReader(src)}
		b, e := readMessage(bufio.NewReader(r))
		if (e == nil) != (n <= maxMessageBytes) {
			t.Fatal("line boundary")
		}
		if e == nil && len(b) != n-1 {
			t.Fatal("line payload changed")
		}
		if r.n > maxMessageBytes+4096 {
			t.Fatal("line read-ahead exceeded buffer")
		}
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", n)
		r = &inputCountReader{r: io.MultiReader(strings.NewReader(header), io.LimitReader(zeroInput{}, int64(n)))}
		b, e = readMessage(bufio.NewReader(r))
		if (e == nil) != (n <= maxMessageBytes) {
			t.Fatal("framed body boundary")
		}
		if e == nil && len(b) != n {
			t.Fatal("framed body short read")
		}
		if n > maxMessageBytes && r.n > 4096 {
			t.Fatal("oversized advertised length read body")
		}
		// Header accumulation is independently bounded from the declared body.
		prefix := "Content-Length: 1\nX:"
		header = prefix + strings.Repeat("x", n-len(prefix)-2) + "\n\n"
		_, e = readMessage(bufio.NewReader(strings.NewReader(header + "0")))
		if (e == nil) != (n <= maxMessageBytes) {
			t.Fatal("header boundary")
		}
	}
	valid := "Content-Length: 2\r\n\r\n{}"
	for _, wrap := range []func(io.Reader) io.Reader{iotest.OneByteReader, iotest.HalfReader, iotest.DataErrReader} {
		b, e := readMessage(bufio.NewReader(wrap(strings.NewReader(valid))))
		if e != nil || string(b) != "{}" {
			t.Fatal("split frame changed")
		}
	}
	for _, src := range []string{"Content-Length: 2\r\n\r\n{", "Content-Length: 2\r\ncontent-length: 2\r\n\r\n{}", "Content-Length: -1\n\n", "Content-Length: 99999999999999999999999\n\n"} {
		if _, e := readMessage(bufio.NewReader(strings.NewReader(src))); e != publicoutput.ErrOutput {
			t.Fatal("invalid framing classification")
		}
	}
	for _, prefix := range []string{"", "Content-Length: 2\r\n", "Content-Length: 2\r\n\r\n{"} {
		var out bytes.Buffer
		e := NewServer(Options{}).Serve(context.Background(), io.MultiReader(strings.NewReader(prefix), iotest.ErrReader(inputErrorBomb{})), &out)
		if e != publicoutput.ErrOutput || out.Len() != 0 {
			t.Fatal("source error retained or dispatched")
		}
	}
}

type zeroInput struct{}

func (zeroInput) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type inputErrorBomb struct{}

func (inputErrorBomb) Error() string { panic("reader error must not be formatted") }

func FuzzSecurityInputRPC(f *testing.F) {
	for _, s := range []string{`{"jsonrpc":"2.0","id":1,"method":"ping"}`, `{"a":1,"A":2}`, `{"id":0e9999999999999}`, `null`, `{} {}`, `"\ud800"`, string([]byte{'"', 255, '"'}), `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"compile_operation_spec","arguments":{"spec":"{\"version\":1}"}}}`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		// Independent implementations currently promise the same raw JSON grammar
		// and structural budgets. Typed validation remains separate.
		if validateJSON(b) != (inputjson.CheckJSON(b) == nil) {
			t.Fatal("raw validator disagreement")
		}
		out, ok := NewServer(Options{}).HandleJSONRPC(context.Background(), b)
		if ok {
			if !json.Valid(out) || len(out) > maxMessageBytes {
				t.Fatal("invalid public response")
			}
			var resp struct {
				Error *rpcError `json:"error"`
			}
			if json.Unmarshal(out, &resp) != nil {
				t.Fatal("response decode")
			}
			if resp.Error != nil {
				switch resp.Error.Message {
				case "PUBLIC_INPUT: invalid JSON", "PUBLIC_INPUT: invalid request", "PUBLIC_OUTPUT: operation failed; details omitted":
				default:
					t.Fatal("nonfixed public error")
				}
			}
		} else if len(out) != 0 {
			t.Fatal("notification emitted bytes")
		}
	})
}

func FuzzSecurityInputFrame(f *testing.F) {
	for _, s := range []string{"{}\n", "Content-Length: 2\r\n\r\n{}", "Content-Length: 1\n", "Content-Length: 3\n\n{}", "\n\n", "Content-Length: 0\n\n"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			t.Skip()
		}
		p1, e1 := readMessage(bufio.NewReader(bytes.NewReader(b)))
		p2, e2 := readMessage(bufio.NewReader(iotest.HalfReader(bytes.NewReader(b))))
		if (e1 == nil) != (e2 == nil) || !bytes.Equal(p1, p2) {
			t.Fatal("fragmentation changed frame")
		}
		if len(p1) > maxMessageBytes {
			t.Fatal("frame exceeded budget")
		}
	})
}

func TestSecurityInputRawRPCBoundaries(t *testing.T) {
	s := NewServer(Options{})
	base := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	for _, n := range []int{maxMessageBytes - 1, maxMessageBytes, maxMessageBytes + 1} {
		b := bytes.Repeat([]byte(" "), n)
		copy(b, base)
		out, ok := s.HandleJSONRPC(context.Background(), b)
		if !ok || (bytes.Contains(out, []byte(`"error"`))) != (n > maxMessageBytes) {
			t.Fatal("direct RPC raw byte boundary")
		}
	}
	for _, n := range []int{1023, 1024, 1025} {
		if validID([]byte(`"`+strings.Repeat("x", n)+`"`)) != (n <= 1024) {
			t.Fatal("decoded ID boundary")
		}
	}
	// A huge exponent must not expand into an exponent-sized allocation.
	if validID([]byte("1e" + strings.Repeat("9", 4096))) {
		t.Fatal("huge exponent ID accepted")
	}
	for _, src := range []string{`{"a":1,"a":2}`, `{"a":1,"A":2}`, strings.Repeat("[", 33) + "0" + strings.Repeat("]", 33), `{} {}`} {
		var out map[string]any
		if decodeAny(src, &out) == nil {
			t.Fatal("string-contained JSON bypassed validation")
		}
	}
	var values map[string]any
	if decodeAny(`{"n":9007199254740993,"escape":"\u0061"}`, &values) != nil || values["n"] != json.Number("9007199254740993") || values["escape"] != "a" {
		t.Fatal("string-contained typed values changed")
	}
	// Encoding expansion is independently checked before marshaling direct values.
	if _, e := primitiveJSON(strings.Repeat("\x00", maxMessageBytes/6)); e != publicoutput.ErrOutput {
		t.Fatal("escaping budget bypass")
	}
}
