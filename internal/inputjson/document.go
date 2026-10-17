package inputjson

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// Document validates bounded new source envelopes before typed decoding. Keys
// must use exact lowercase spelling; duplicate keys and explicit null are refused
// recursively. Optional values must be omitted, not represented by null.
func Document(b []byte) error {
	if len(b) > 8<<20 {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return ErrInvalid
		}
		tok, e := d.Token()
		if e != nil || tok == nil {
			return ErrInvalid
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				k, ok := key.(string)
				if e != nil || !ok || k != strings.ToLower(k) || seen[k] {
					return ErrInvalid
				}
				seen[k] = true
				if value(depth+1) != nil {
					return ErrInvalid
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim('}') {
				return ErrInvalid
			}
		case '[':
			for d.More() {
				if value(depth+1) != nil {
					return ErrInvalid
				}
			}
			end, e := d.Token()
			if e != nil || end != json.Delim(']') {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		return nil
	}
	if value(0) != nil {
		return ErrInvalid
	}
	if _, e := d.Token(); e != io.EOF {
		return ErrInvalid
	}
	return nil
}
