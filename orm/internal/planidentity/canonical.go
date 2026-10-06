// Package planidentity provides internal correspondence primitives, not permits.
package planidentity

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const Version = 1
const MaxDepth = 64
const MaxSlots = 65536
const MaxBytes = 8 << 20

var ErrUnavailable = errors.New("goquent: private identity unavailable")

// Canonical encodes a closed typed domain without calling user methods.
// Each record is tag + decimal byte length + ':' + payload. Container payloads
// concatenate child records; maps alternate sorted string keys and values.
func Canonical(v any) ([]byte, error) {
	slots := 0
	var encode func(any, int) ([]byte, error)
	encode = func(v any, depth int) ([]byte, error) {
		slots++
		if depth > MaxDepth || slots > MaxSlots {
			return nil, ErrUnavailable
		}
		tag, payload := "", ""
		switch x := v.(type) {
		case nil:
			tag = "null"
		case bool:
			tag = "bool"
			payload = strconv.FormatBool(x)
		case string:
			if !utf8.ValidString(x) {
				return nil, ErrUnavailable
			}
			tag, payload = "str", x
		case int:
			tag, payload = "int"+strconv.Itoa(strconv.IntSize), strconv.FormatInt(int64(x), 10)
		case int8:
			tag, payload = "i8", strconv.FormatInt(int64(x), 10)
		case int16:
			tag, payload = "i16", strconv.FormatInt(int64(x), 10)
		case int32:
			tag, payload = "i32", strconv.FormatInt(int64(x), 10)
		case int64:
			tag, payload = "i64", strconv.FormatInt(x, 10)
		case uint:
			tag, payload = "uint"+strconv.Itoa(strconv.IntSize), strconv.FormatUint(uint64(x), 10)
		case uint8:
			tag, payload = "u8", strconv.FormatUint(uint64(x), 10)
		case uint16:
			tag, payload = "u16", strconv.FormatUint(uint64(x), 10)
		case uint32:
			tag, payload = "u32", strconv.FormatUint(uint64(x), 10)
		case uint64:
			tag, payload = "u64", strconv.FormatUint(x, 10)
		case float32:
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, ErrUnavailable
			}
			tag, payload = "f32", strconv.FormatUint(uint64(math.Float32bits(x)), 16)
		case float64:
			if math.IsNaN(x) || math.IsInf(x, 0) {
				return nil, ErrUnavailable
			}
			tag, payload = "f64", strconv.FormatUint(math.Float64bits(x), 16)
		case json.Number:
			// JSON numbers are lexemes, not evidence of a native SQL integer width.
			s := string(x)
			if strings.TrimSpace(s) != s {
				return nil, ErrUnavailable
			}
			if !json.Valid([]byte(s)) || strings.ContainsAny(s, ".eE") || s == "" || (s[0] != '-' && (s[0] < '0' || s[0] > '9')) {
				return nil, ErrUnavailable
			}
			tag, payload = "jsonint", s
		case []byte:
			tag = "bytes"
			if x == nil {
				tag = "nilbytes"
			}
			if len(x) > MaxBytes/2 {
				return nil, ErrUnavailable
			}
			payload = base64.StdEncoding.EncodeToString(x)
		case []any:
			tag = "list"
			if x == nil {
				tag = "nillist"
			}
			if len(x) > MaxSlots {
				return nil, ErrUnavailable
			}
			var b bytes.Buffer
			for _, a := range x {
				p, e := encode(a, depth+1)
				if e != nil {
					return nil, e
				}
				if b.Len()+len(p) > MaxBytes {
					return nil, ErrUnavailable
				}
				b.Write(p)
			}
			payload = b.String()
		case map[string]any:
			tag = "map"
			if x == nil {
				tag = "nilmap"
			}
			if len(x) > MaxSlots/2 {
				return nil, ErrUnavailable
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var b bytes.Buffer
			for _, k := range keys {
				for _, a := range []any{k, x[k]} {
					p, e := encode(a, depth+1)
					if e != nil {
						return nil, e
					}
					if b.Len()+len(p) > MaxBytes {
						return nil, ErrUnavailable
					}
					b.Write(p)
				}
			}
			payload = b.String()
		default:
			return nil, ErrUnavailable
		}
		if len(payload) > MaxBytes {
			return nil, ErrUnavailable
		}
		out := []byte(tag + strconv.Itoa(len(payload)) + ":" + payload)
		if len(out) > MaxBytes {
			return nil, ErrUnavailable
		}
		return out, nil
	}
	return encode(v, 0)
}
