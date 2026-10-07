// Package inputjson reads library-owned objects without last-wins ambiguity.
package inputjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

var ErrInvalid = errors.New("goquent: invalid or ambiguous input")

// Object preserves field presence and exact spelling. A nil allowlist leaves
// legacy unknown fields alone while still refusing ambiguous keys.
func Object(b []byte, allowed map[string]bool) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, ErrInvalid
	}
	out := map[string]json.RawMessage{}
	seen := map[string]bool{}
	for d.More() {
		t, err := d.Token()
		if err != nil {
			return nil, ErrInvalid
		}
		k, ok := t.(string)
		folded := strings.ToLower(k)
		if !ok || seen[folded] || (allowed != nil && !allowed[k]) {
			return nil, ErrInvalid
		}
		seen[folded] = true
		var v json.RawMessage
		if d.Decode(&v) != nil {
			return nil, ErrInvalid
		}
		out[k] = v
	}
	if _, err := d.Token(); err != nil {
		return nil, ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrInvalid
	}
	return out, nil
}

// Nullable reads only the existing nullable field; no wire verdict is accepted.
func Nullable(fields map[string]json.RawMessage) (bool, bool, error) {
	for k := range fields {
		if strings.EqualFold(k, "NullableKnown") || strings.EqualFold(k, "nullable_known") || (strings.EqualFold(k, "nullable") && k != "nullable") {
			return false, false, ErrInvalid
		}
	}
	b, present := fields["nullable"]
	if !present {
		return false, false, nil
	}
	switch string(b) {
	case "true":
		return true, true, nil
	case "false":
		return false, true, nil
	}
	return false, false, ErrInvalid
}
