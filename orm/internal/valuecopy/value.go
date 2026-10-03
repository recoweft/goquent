// Package valuecopy detaches a deliberately bounded set of built-in values.
package valuecopy

import "time"

// Copy never invokes user code or normalizes a driver value. A false result
// means the returned value may still refer to external mutable state.
func Copy(value any) (any, bool) { return copyValue(value, 0) }

func copyValue(value any, depth int) (any, bool) {
	if depth > 64 {
		return value, false
	}
	switch v := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, time.Time:
		return v, true
	case []bool:
		return cloneSlice(v), true
	case []string:
		return cloneSlice(v), true
	case []int:
		return cloneSlice(v), true
	case []int8:
		return cloneSlice(v), true
	case []int16:
		return cloneSlice(v), true
	case []int32:
		return cloneSlice(v), true
	case []int64:
		return cloneSlice(v), true
	case []uint:
		return cloneSlice(v), true
	case []uint16:
		return cloneSlice(v), true
	case []uint32:
		return cloneSlice(v), true
	case []uint64:
		return cloneSlice(v), true
	case []float32:
		return cloneSlice(v), true
	case []float64:
		return cloneSlice(v), true
	case []time.Time:
		return cloneSlice(v), true
	case []byte:
		if v == nil {
			return v, true
		}
		out := make([]byte, len(v))
		copy(out, v)
		return out, true
	case []any:
		if v == nil {
			return v, true
		}
		out := make([]any, len(v))
		ok := true
		for i, x := range v {
			var child bool
			out[i], child = copyValue(x, depth+1)
			ok = ok && child
		}
		return out, ok
	case map[string]any:
		if v == nil {
			return v, true
		}
		out := make(map[string]any, len(v))
		ok := true
		for k, x := range v {
			var child bool
			out[k], child = copyValue(x, depth+1)
			ok = ok && child
		}
		return out, ok
	default:
		return value, false
	}
}

func Slice(values []any) []any {
	if values == nil {
		return nil
	}
	out := make([]any, len(values))
	for i, v := range values {
		out[i], _ = Copy(v)
	}
	return out
}

func cloneSlice[T any](v []T) []T {
	if v == nil {
		return nil
	}
	out := make([]T, len(v))
	copy(out, v)
	return out
}
