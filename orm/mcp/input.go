package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/recoweft/goquent/orm/publicoutput"
)

// primitiveJSON validates before encoding; unknown types never invoke methods.
func primitiveJSON(v any) ([]byte, error) {
	budget := maxMessageBytes
	elements := 0
	var check func(any, int) bool
	check = func(v any, depth int) bool {
		elements++
		budget -= 8
		if depth > 32 || elements > 16384 || budget < 0 {
			return false
		}
		switch x := v.(type) {
		case nil, bool:
			return true
		case string:
			budget -= len(x) * 6
			return budget >= 0 && utf8.ValidString(x)
		case json.Number:
			budget -= len(x)
			return budget >= 0 && json.Valid([]byte(x)) && len(x) > 0 && x[0] != '"' && x != "null" && x != "true" && x != "false" && x[0] != '[' && x[0] != '{'
		case float64:
			return !math.IsNaN(x) && !math.IsInf(x, 0)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			budget -= 24
			return budget >= 0
		case []any:
			if len(x) > 16384-elements {
				return false
			}
			for _, e := range x {
				if !check(e, depth+1) {
					return false
				}
			}
			return true
		case map[string]any:
			if len(x) > 16384-elements {
				return false
			}
			for k, e := range x {
				budget -= len(k)*6 + 4
				if !utf8.ValidString(k) || !check(e, depth+1) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	if !check(v, 0) {
		return nil, publicoutput.ErrOutput
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) > maxMessageBytes {
		return nil, publicoutput.ErrOutput
	}
	return b, nil
}

// validateJSON bounds direct and transport input and rejects duplicate/ambiguous
// object keys before decoding loses information. Null values remain legal data.
func validateJSON(b []byte) bool {
	if len(b) > maxMessageBytes || !utf8.Valid(b) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	elements := 0
	var scan func(int) bool
	scan = func(depth int) bool {
		elements++
		if depth > 32 || elements > 16384 {
			return false
		}
		t, err := d.Token()
		if err != nil {
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
				k = strings.ToLower(k)
				if !ok || seen[k] {
					return false
				}
				seen[k] = true
				if !scan(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for d.More() {
				if !scan(depth + 1) {
					return false
				}
			}
			end, err := d.Token()
			return err == nil && end == json.Delim(']')
		}
		return false
	}
	if !scan(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}

// validID accepts UTF-8 strings or mathematical safe integers without float64.
func validID(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	if b[0] == '"' {
		var s string
		return json.Unmarshal(b, &s) == nil && utf8.ValidString(s) && len(s) <= 1024
	}
	if !json.Valid(b) || b[0] == 'n' || b[0] == 't' || b[0] == 'f' || b[0] == '[' || b[0] == '{' {
		return false
	}
	s := string(b)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	mant, expText, hasExp := strings.Cut(strings.ToLower(s), "e")
	integer, fraction, _ := strings.Cut(mant, ".")
	digits := strings.TrimLeft(integer+fraction, "0")
	if digits == "" {
		return true
	}
	exp := int64(0)
	if hasExp {
		n, err := strconv.ParseInt(expText, 10, 64)
		if err != nil {
			return false
		}
		exp = n
	}
	if exp > int64(len(b))+32 || exp < -int64(len(b))-32 {
		return false
	}
	shift := exp - int64(len(fraction))
	if shift < 0 {
		n := int64(len(digits)) + shift
		if n <= 0 {
			return false
		}
		for _, c := range digits[n:] {
			if c != '0' {
				return false
			}
		}
		digits = digits[:n]
	} else {
		if int64(len(digits))+shift > 16 {
			return false
		}
		digits += strings.Repeat("0", int(shift))
	}
	if len(digits) > 16 {
		return false
	}
	if neg {
		digits = "-" + digits
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	return err == nil && n >= -9007199254740991 && n <= 9007199254740991
}
