package conv

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/recoweft/goquent/orm/internal/stringutil"
)

// As converts v to the desired type T using reflection.
func As[T any](v any) (T, error) {
	var zero T
	if v == nil {
		return zero, fmt.Errorf("value is nil")
	}
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(zero)
	if !rv.Type().ConvertibleTo(rt) {
		return zero, fmt.Errorf("cannot convert %T to %T", v, zero)
	}
	return rv.Convert(rt).Interface().(T), nil
}

// Value returns the given key from m converted to T.
func Value[T any](m map[string]any, key string) (T, error) {
	val, ok := m[key]
	if !ok {
		var zero T
		return zero, fmt.Errorf("key %q not found", key)
	}
	return As[T](val)
}

// MapToStruct copies values from map m to the struct pointed to by dest.
// Keys are matched to struct fields using orm tags or snake_case names.
func MapToStruct(m map[string]any, dest any) error {
	if m == nil {
		return fmt.Errorf("map is nil")
	}
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("dest must be non-nil pointer to struct")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("dest must point to struct")
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		col := sf.Tag.Get("db")
		if col == "" || col == "-" {
			col = parseTag(sf.Tag.Get("orm"))
		}
		if col == "" {
			col = stringutil.ToSnake(sf.Name)
		}
		if val, ok := findValue(m, col); ok && val != nil {
			fv := reflect.ValueOf(val)
			f := v.Field(i)
			if fv.Type().ConvertibleTo(f.Type()) {
				f.Set(fv.Convert(f.Type()))
			} else {
				return fmt.Errorf("cannot convert %s to field %s", fv.Type(), sf.Name)
			}
		}
	}
	return nil
}

// MapsToStructs converts a slice of maps to a slice of structs.
func MapsToStructs(src []map[string]any, dest any) error {
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("dest must be non-nil pointer to slice")
	}
	v = v.Elem()
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("dest must point to slice")
	}
	elemType := v.Type().Elem()
	for _, m := range src {
		elemPtr := reflect.New(elemType)
		if err := MapToStruct(m, elemPtr.Interface()); err != nil {
			return err
		}
		v.Set(reflect.Append(v, elemPtr.Elem()))
	}
	return nil
}

// StructToMap converts a struct or pointer to struct into a map. Column names
// are determined by `db` or `orm` tags, falling back to snake_case field names.
// Fields tagged with `db:"-"` are omitted. Zero-value fields are included
// unless the tag contains `omitempty`.
func StructToMap(v any) (map[string]any, error) {
	if v == nil {
		return nil, fmt.Errorf("value is nil")
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("StructToMap expects struct input")
	}
	t := rv.Type()
	m := make(map[string]any, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // unexported field
			continue
		}
		dbTag := sf.Tag.Get("db")
		col, _ := splitTag(dbTag)
		if col == "-" {
			continue
		}
		tag := dbTag
		if col == "" {
			ormTag := sf.Tag.Get("orm")
			tag = ormTag
			col = parseTag(ormTag)
		}
		if col == "" {
			col = stringutil.ToSnake(sf.Name)
		}
		fv := rv.Field(i)
		if hasOmitempty(tag) && fv.IsZero() {
			continue
		}
		m[col] = fv.Interface()
	}
	return m, nil
}

func findValue(m map[string]any, name string) (any, bool) {
	if val, ok := m[name]; ok {
		return val, true
	}
	for k, val := range m {
		if normalizeKey(k) == name {
			return val, true
		}
	}
	return nil, false
}

func normalizeKey(k string) string {
	if idx := strings.LastIndex(k, "."); idx >= 0 {
		k = k[idx+1:]
	}
	return strings.Trim(k, "`")
}

func parseTag(tag string) string {
	for _, part := range strings.Split(tag, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && kv[0] == "column" {
			return kv[1]
		}
	}
	return ""
}

func splitTag(tag string) (name string, opts []string) {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return "", nil
	}
	name = parts[0]
	if len(parts) > 1 {
		opts = parts[1:]
	}
	return name, opts
}

func hasOmitempty(tag string) bool {
	_, opts := splitTag(tag)
	for _, o := range opts {
		if o == "omitempty" {
			return true
		}
	}
	return false
}
