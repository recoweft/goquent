// Package valueguard checks library-owned output and built-in payloads without
// calling user methods. Custom types are opaque boundaries, not safety claims.
package valueguard

import (
	"errors"
	"fmt"
	"reflect"
)

// Output includes library struct/pointer/slice wrappers around condition groups.
// 256 accommodates the builder's 64-group limit plus those wrappers.
const MaxDepth = 256
const MaxNodes = 65536
const MaxBytes = 8 << 20

var ErrCycle = errors.New("cyclic built-in output")
var ErrDepth = errors.New("output depth exceeds 256")
var ErrBudget = errors.New("output expansion exceeds budget")

type Error struct{ Cause error }

func (e *Error) Error() string { return fmt.Sprintf("goquent output: %v", e.Cause) }
func (e *Error) Unwrap() error { return e.Cause }

type id struct {
	typ reflect.Type
	ptr uintptr
	n   int
}
type cost struct{ nodes, bytes, height int }
type record struct {
	active bool
	cost   cost
}
type checker struct{ memo map[id]*record }

// Check measures expanded cost using memoized subtree costs. A repeated edge
// costs its entire output again, but its subtree is inspected only once.
func Check(v any) error {
	c := checker{memo: make(map[id]*record)}
	_, err := c.walk(reflect.ValueOf(v), 0)
	if err != nil {
		return &Error{Cause: err}
	}
	return nil
}
func owned(t reflect.Type) bool {
	return t.PkgPath() == "github.com/recoweft/goquent/orm/query" || t.PkgPath() == "github.com/recoweft/goquent/orm/predicate"
}
func (c *checker) walk(v reflect.Value, depth int) (cost, error) {
	if depth > MaxDepth {
		return cost{}, ErrDepth
	}
	out := cost{nodes: 1, bytes: 32}
	if !v.IsValid() {
		return out, nil
	}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return out, nil
		}
		return c.walk(v.Elem(), depth)
	}
	t := v.Type()
	if t.PkgPath() != "" && !owned(t) {
		return out, nil
	}
	// Only library-owned pointers are traversed. Arbitrary user objects remain opaque.
	if v.Kind() == reflect.Pointer && !owned(t.Elem()) {
		return out, nil
	}
	var memo *record
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if v.IsNil() {
			return out, nil
		}
		if v.Kind() == reflect.Map && t.Key() != reflect.TypeOf("") {
			return out, nil
		}
		n := 0
		if v.Kind() != reflect.Pointer {
			n = v.Len()
		}
		k := id{t, v.Pointer(), n}
		if e := c.memo[k]; e != nil {
			if e.active {
				return cost{}, ErrCycle
			}
			if depth+e.cost.height > MaxDepth {
				return cost{}, ErrDepth
			}
			return e.cost, nil
		}
		memo = &record{active: true}
		c.memo[k] = memo
	}
	add := func(child cost) error {
		if child.nodes > MaxNodes-out.nodes || child.bytes > MaxBytes-out.bytes {
			return ErrBudget
		}
		out.nodes += child.nodes
		out.bytes += child.bytes
		if child.height+1 > out.height {
			out.height = child.height + 1
		}
		return nil
	}
	visit := func(x reflect.Value) error {
		a, e := c.walk(x, depth+1)
		if e != nil {
			return e
		}
		return add(a)
	}
	var err error
	switch v.Kind() {
	case reflect.String:
		// JSON worst-case escaping, plus quotes/pretty-print overhead per node.
		if v.Len() > (MaxBytes-out.bytes)/6 {
			return cost{}, ErrBudget
		}
		out.bytes += v.Len() * 6
	case reflect.Pointer:
		err = visit(v.Elem())
	case reflect.Struct:
		if owned(t) {
			for i := 0; i < v.NumField(); i++ {
				if t.Field(i).IsExported() {
					if err = visit(v.Field(i)); err != nil {
						break
					}
				}
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Len() > MaxNodes-out.nodes {
			return cost{}, ErrBudget
		}
		for i := 0; i < v.Len(); i++ {
			if err = visit(v.Index(i)); err != nil {
				break
			}
		}
	case reflect.Map:
		if v.Len() > (MaxNodes-out.nodes)/2 {
			return cost{}, ErrBudget
		}
		it := v.MapRange()
		for it.Next() {
			if err = visit(it.Key()); err != nil {
				break
			}
			if err = visit(it.Value()); err != nil {
				break
			}
		}
	}
	if err != nil {
		return cost{}, err
	}
	if memo != nil {
		memo.active = false
		memo.cost = out
	}
	return out, nil
}
