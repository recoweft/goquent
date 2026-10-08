// Package operationtest contains shared test fixtures, not production validation.
package operationtest

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"reflect"
	"unicode/utf8"

	"github.com/recoweft/goquent/orm/manifest"
)

type Case struct {
	Name        string
	Spec        json.RawMessage
	Values      json.RawMessage
	Type        string
	TypeSource  string `json:"type_source"`
	SchemaValid bool   `json:"schema_valid"`
	Compiled    bool
	Code        string
	Partial     bool
}

func Load(path string) ([]Case, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var cases []Case
	e = json.Unmarshal(b, &cases)
	return cases, e
}
func Decode(b []byte) (any, error) {
	var v any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	e := d.Decode(&v)
	return v, e
}
func (c Case) Manifest() *manifest.Manifest {
	return &manifest.Manifest{Version: "1", Dialect: "postgres", Tables: []manifest.Table{{Name: "diagnostic_items", Columns: []manifest.Column{{Name: "v", Type: c.Type, TypeSource: c.TypeSource, NullableKnown: true}}}}}
}

// SchemaAccepts evaluates the vocabulary used by the generated OperationSpec
// schema. It fails on an unimplemented keyword; it is not a general schema engine.
func SchemaAccepts(rawSchema, rawInput []byte) bool {
	s, e := Decode(rawSchema)
	if e != nil {
		panic(e)
	}
	v, e := Decode(rawInput)
	if e != nil {
		return false
	}
	return accepts(s.(map[string]any), v)
}
func accepts(s map[string]any, v any) bool {
	for k, x := range s {
		switch k {
		case "$schema", "$id", "title", "description", "properties", "additionalProperties", "items", "then": // handled below
		case "type":
			types, ok := x.([]any)
			if !ok {
				types = []any{x}
			}
			match := false
			for _, t := range types {
				match = match || isType(t.(string), v)
			}
			if !match {
				return false
			}
		case "const":
			if !schemaEqual(x, v) {
				return false
			}
		case "enum":
			match := false
			for _, a := range x.([]any) {
				match = match || schemaEqual(a, v)
			}
			if !match {
				return false
			}
		case "required":
			if obj, ok := v.(map[string]any); ok {
				for _, k := range x.([]any) {
					if _, ok := obj[k.(string)]; !ok {
						return false
					}
				}
			}
		case "minimum", "maximum":
			if n, ok := v.(json.Number); ok {
				a, _ := new(big.Rat).SetString(string(n))
				b, _ := new(big.Rat).SetString(string(x.(json.Number)))
				if k == "minimum" && a.Cmp(b) < 0 || k == "maximum" && a.Cmp(b) > 0 {
					return false
				}
			}
		case "minLength":
			if str, ok := v.(string); ok {
				n, _ := x.(json.Number).Int64()
				if int64(utf8.RuneCountInString(str)) < n {
					return false
				}
			}
		case "minItems", "maxItems":
			if list, ok := v.([]any); ok {
				n, _ := x.(json.Number).Int64()
				if k == "minItems" && int64(len(list)) < n || k == "maxItems" && int64(len(list)) > n {
					return false
				}
			}
		case "not":
			if accepts(x.(map[string]any), v) {
				return false
			}
		case "allOf", "anyOf", "oneOf":
			n := 0
			for _, a := range x.([]any) {
				if accepts(a.(map[string]any), v) {
					n++
				}
			}
			if k == "allOf" && n != len(x.([]any)) || k == "anyOf" && n == 0 || k == "oneOf" && n != 1 {
				return false
			}
		case "if":
			branch := "else"
			if accepts(x.(map[string]any), v) {
				branch = "then"
			}
			if next, ok := s[branch]; ok && !accepts(next.(map[string]any), v) {
				return false
			}
		case "else":
		default:
			panic("unsupported schema keyword")
		}
	}
	if obj, ok := v.(map[string]any); ok {
		props, _ := s["properties"].(map[string]any)
		for k, value := range obj {
			if child, ok := props[k]; ok {
				if !accepts(child.(map[string]any), value) {
					return false
				}
			} else if s["additionalProperties"] == false {
				return false
			}
		}
	}
	if list, ok := v.([]any); ok {
		if child, ok := s["items"].(map[string]any); ok {
			for _, value := range list {
				if !accepts(child, value) {
					return false
				}
			}
		}
	}
	return true
}
func isType(t string, v any) bool {
	switch t {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(json.Number)
		return ok
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			return false
		}
		r, ok := new(big.Rat).SetString(string(n))
		return ok && r.IsInt()
	default:
		panic("unsupported schema type")
	}
}

func schemaEqual(a, b any) bool {
	x, xok := a.(json.Number)
	y, yok := b.(json.Number)
	if xok && yok {
		xr, _ := new(big.Rat).SetString(string(x))
		yr, _ := new(big.Rat).SetString(string(y))
		return xr.Cmp(yr) == 0
	}
	return reflect.DeepEqual(a, b)
}
