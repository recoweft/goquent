package inputjson

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 1048576
const MaxDepth = 32
const MaxNodes = 16384

// CheckJSON bounds raw bytes as well as structure before decoding loses keys.
func CheckJSON(b []byte) error {
	if len(b) > MaxBytes || !utf8.Valid(b) {
		return ErrInvalid
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	nodes := 0
	var scan func(int) error
	scan = func(depth int) error {
		nodes++
		if depth > MaxDepth || nodes > MaxNodes {
			return ErrInvalid
		}
		t, err := d.Token()
		if err != nil {
			return ErrInvalid
		}
		x, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch x {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				t, e := d.Token()
				if e != nil {
					return ErrInvalid
				}
				k, ok := t.(string)
				k = strings.ToLower(k)
				if !ok || seen[k] {
					return ErrInvalid
				}
				seen[k] = true
				if scan(depth+1) != nil {
					return ErrInvalid
				}
			}
			t, err = d.Token()
			if err != nil || t != json.Delim('}') {
				return ErrInvalid
			}
		case '[':
			for d.More() {
				if scan(depth+1) != nil {
					return ErrInvalid
				}
			}
			t, err = d.Token()
			if err != nil || t != json.Delim(']') {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		return nil
	}
	if scan(0) != nil {
		return ErrInvalid
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrInvalid
	}
	return nil
}

// Size counts a closed primitive representation, without calling value methods
// or coercing the original arguments. Keys count as bytes, not value nodes.
func Size(v any) (int, error) {
	bytes, nodes := 0, 0
	add := func(n int) bool {
		if n > MaxBytes-bytes {
			return false
		}
		bytes += n
		return true
	}
	str := func(s string) bool {
		if !utf8.ValidString(s) || len(s) > MaxBytes-bytes {
			return false
		}
		// The precheck bounds allocation; Marshal is applied only to a built-in string.
		b, _ := json.Marshal(s)
		return add(len(b))
	}
	var walk func(any, int) bool
	walk = func(v any, depth int) bool {
		nodes++
		if nodes > MaxNodes || depth > MaxDepth {
			return false
		}
		switch x := v.(type) {
		case nil:
			return add(4)
		case bool:
			if x {
				return add(4)
			}
			return add(5)
		case string:
			return str(x)
		case json.Number:
			s := string(x)
			if len(s) == 0 || len(s) > MaxBytes-bytes || !json.Valid([]byte(s)) || !(s[0] == '-' || s[0] >= '0' && s[0] <= '9') {
				return false
			}
			return add(len(s))
		case float32:
			return !math.IsNaN(float64(x)) && !math.IsInf(float64(x), 0) && add(len(strconv.FormatFloat(float64(x), 'g', -1, 32)))
		case float64:
			return !math.IsNaN(x) && !math.IsInf(x, 0) && add(len(strconv.FormatFloat(x, 'g', -1, 64)))
		case int, int8, int16, int32, int64:
			return add(len(strconv.FormatInt(reflect.ValueOf(v).Int(), 10)))
		case uint, uint8, uint16, uint32, uint64:
			return add(len(strconv.FormatUint(reflect.ValueOf(v).Uint(), 10)))
		case map[string]any:
			if x == nil {
				return add(4)
			}
			if len(x) > MaxNodes-nodes || !add(2) {
				return false
			}
			seen := map[string]bool{}
			i := 0
			for k, e := range x {
				lower := strings.ToLower(k)
				if seen[lower] {
					return false
				}
				seen[lower] = true
				if i > 0 && !add(1) {
					return false
				}
				i++
				if !str(k) || !add(1) || !walk(e, depth+1) {
					return false
				}
			}
			return true
		case []any, []bool, []string, []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []float32, []float64:
			r := reflect.ValueOf(v)
			if r.IsNil() {
				return add(4)
			}
			if r.Len() > MaxNodes-nodes || !add(2) {
				return false
			}
			for i := 0; i < r.Len(); i++ {
				if i > 0 && !add(1) {
					return false
				}
				if !walk(r.Index(i).Interface(), depth+1) {
					return false
				}
			}
			return true
		default:
			return false
		}
	}
	if !walk(v, 0) {
		return 0, ErrInvalid
	}
	return bytes, nil
}
