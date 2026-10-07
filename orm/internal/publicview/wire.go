// Package publicview contains only fixed display policy and bounded wire helpers.
// It never accepts execution data or invokes application value methods.
package publicview

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const MaxBytes = 1 << 20

var ErrInvalid = errors.New("PUBLIC_VIEW_INVALID: invalid public view")
var ErrSource = errors.New("PUBLIC_VIEW_SOURCE: unsupported diagnostic source")
var ErrWrite = errors.New("PUBLIC_VIEW_WRITE: output failed")

// Decode rejects ambiguous keys, nulls, unknown fields and trailing documents.
// dst must be a library-owned plain wire struct, never application data.
func Decode(b []byte, kind string, dst any) error {
	if len(b) > MaxBytes {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if !scan(d, 0) {
		return ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInvalid
	}
	var envelope struct {
		Kind    string          `json:"kind"`
		Version json.RawMessage `json:"version"`
	}
	if json.Unmarshal(b, &envelope) != nil || envelope.Kind != kind || string(envelope.Version) != "1" {
		return ErrInvalid
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil {
		return ErrInvalid
	}
	return nil
}
func scan(d *json.Decoder, depth int) bool {
	if depth > 32 {
		return false
	}
	t, err := d.Token()
	if err != nil || t == nil {
		return false
	}
	delim, ok := t.(json.Delim)
	if !ok {
		return true
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return false
			}
			k, ok := key.(string)
			if !ok || seen[k] || strings.ToLower(k) != k {
				return false
			}
			seen[k] = true
			if !scan(d, depth+1) {
				return false
			}
		}
		end, err := d.Token()
		return err == nil && end == json.Delim('}')
	case '[':
		for d.More() {
			if !scan(d, depth+1) {
				return false
			}
		}
		end, err := d.Token()
		return err == nil && end == json.Delim(']')
	}
	return false
}

func Write(w io.Writer, b []byte) error {
	if w == nil {
		return ErrWrite
	}
	n, err := w.Write(b)
	if err != nil || n != len(b) {
		return ErrWrite
	}
	return nil
}
func Nonnegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
func Operation(s string) string {
	switch s {
	case "select", "insert", "update", "delete", "raw":
		return s
	}
	return "unknown"
}
func Risk(s string) string {
	switch s {
	case "low", "medium", "high", "destructive", "blocked":
		return s
	}
	return "unknown"
}
func Precision(s string) string {
	switch s {
	case "precise", "partial", "unsupported":
		return s
	}
	return "unknown"
}
