// Package valuecopy detaches a deliberately bounded set of built-in values.
package valuecopy

import (
	"reflect"
	"time"

	"github.com/recoweft/goquent/orm/predicate"
)

const MaxDepth = 64
const MaxSlots = 65536
const MaxBytes = 8 << 20
const UnverifiedReason = "unsupported_recursive_deep_or_oversized_value"

type identity struct {
	typ    reflect.Type
	ptr    uintptr
	length int
}
type entry struct {
	// Retain the source: uintptr identity alone would allow GC/address reuse
	// between different arguments in the same operation.
	source any
	value  any
	ok     bool
	active bool
	height int
}

// Copier owns memoization and allocation budgets for one copy operation.
// It must not be reused for independent snapshots or after input mutation.
type Copier struct {
	nodes        map[*predicate.Node]*nodeEntry
	memo         map[identity]*entry
	slots, bytes int
}

func New() *Copier { return &Copier{memo: make(map[identity]*entry)} }

// Copy never invokes user code or normalizes a driver value. A false result
// means the returned value may still refer to external mutable state.
func Copy(value any) (any, bool)             { return New().Copy(value) }
func (c *Copier) Copy(value any) (any, bool) { v, ok, _ := c.copy(value, 0); return v, ok }
func key(v any) identity {
	r := reflect.ValueOf(v)
	n := 0
	if r.Kind() == reflect.Slice {
		n = r.Len()
	}
	return identity{r.Type(), r.Pointer(), n}
}
func (c *Copier) reserve(slots, bytes int) bool {
	if slots > MaxSlots-c.slots || bytes > MaxBytes-c.bytes {
		return false
	}
	c.slots += slots
	c.bytes += bytes
	return true
}
func (c *Copier) copy(value any, depth int) (any, bool, int) {
	if depth > MaxDepth {
		return value, false, 0
	}
	switch v := value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, time.Time:
		return v, true, 0
	case []bool:
		return copyScalarSlice(c, v)
	case []string:
		return copyScalarSlice(c, v)
	case []int:
		return copyScalarSlice(c, v)
	case []int8:
		return copyScalarSlice(c, v)
	case []int16:
		return copyScalarSlice(c, v)
	case []int32:
		return copyScalarSlice(c, v)
	case []int64:
		return copyScalarSlice(c, v)
	case []uint:
		return copyScalarSlice(c, v)
	case []uint16:
		return copyScalarSlice(c, v)
	case []uint32:
		return copyScalarSlice(c, v)
	case []uint64:
		return copyScalarSlice(c, v)
	case []float32:
		return copyScalarSlice(c, v)
	case []float64:
		return copyScalarSlice(c, v)
	case []time.Time:
		return copyScalarSlice(c, v)

	case []byte:
		return copyScalarSlice(c, v)
	case []any, map[string]any:
		r := reflect.ValueOf(v)
		if r.IsNil() {
			return v, true, 0
		}
		k := key(v)
		if e := c.memo[k]; e != nil {
			if e.active || depth+e.height > MaxDepth {
				return v, false, 0
			}
			return e.value, e.ok, e.height
		}
		// Reserve before make. Map slot allowance is not exact runtime heap usage.
		n := r.Len()
		size := 16
		if r.Kind() == reflect.Map {
			size = 64
		}
		if n > MaxSlots || !c.reserve(n+1, n*size) {
			return v, false, 0
		}
		e := &entry{source: v, active: true, ok: true}
		c.memo[k] = e
		height := 0
		child := func(x any) any {
			y, ok, h := c.copy(x, depth+1)
			e.ok = e.ok && ok
			if h+1 > height {
				height = h + 1
			}
			return y
		}
		switch x := v.(type) {
		case []any:
			out := make([]any, len(x))
			e.value = out
			for i, a := range x {
				out[i] = child(a)
			}
		case map[string]any:
			out := make(map[string]any, len(x))
			e.value = out
			for k, a := range x {
				out[k] = child(a)
			}
		}
		e.active = false
		e.height = height
		return e.value, e.ok, height
	default:
		return value, false, 0
	}
}
func copyScalarSlice[T any](c *Copier, v []T) (any, bool, int) {
	if v == nil {
		return v, true, 0
	}
	k := key(v)
	if e := c.memo[k]; e != nil {
		return e.value, e.ok, 0
	}
	size := int(reflect.TypeOf(v).Elem().Size())
	if len(v) > MaxBytes/size || !c.reserve(1, len(v)*size) {
		return v, false, 0
	}
	out := make([]T, len(v))
	copy(out, v)
	c.memo[k] = &entry{source: v, value: out, ok: true}
	return out, true, 0
}
func Slice(values []any) []any             { return New().Slice(values) }
func (c *Copier) Slice(values []any) []any { v, _ := c.Copy(values); return v.([]any) }
