package scanner

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/recoweft/goquent/orm/internal/stringutil"
)

// Struct scans current row into dest struct using column mapping.
func Struct(dest any, rows *sql.Rows) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("dest must be non-nil pointer")
	}
	v = v.Elem()
	fields := make([]any, len(cols))
	for i := range fields {
		fields[i] = new(any)
	}
	if !rows.Next() {
		return sql.ErrNoRows
	}
	if err = rows.Scan(fields...); err != nil {
		return err
	}
	scannerType := reflect.TypeOf((*sql.Scanner)(nil)).Elem()
	for i, col := range cols {
		val := reflect.ValueOf(fields[i]).Elem().Interface()
		f := fieldByColumn(v, col, val != nil)
		if !f.IsValid() || !f.CanSet() {
			continue
		}

		// handle specialized bool types first
		switch f.Type() {
		case reflect.TypeOf(true):
			b, err := parseBoolCompat(val)
			if err != nil {
				return fmt.Errorf("scan %s: %w", col, err)
			}
			f.SetBool(b)
			continue
		case reflect.TypeOf(sql.NullBool{}):
			nb, err := parseNullBoolCompat(val)
			if err != nil {
				return fmt.Errorf("scan %s: %w", col, err)
			}
			f.Set(reflect.ValueOf(nb))
			continue
		}
		if f.Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.Bool {
			pb, err := parsePtrBoolCompat(val)
			if err != nil {
				return fmt.Errorf("scan %s: %w", col, err)
			}
			if pb == nil {
				f.Set(reflect.Zero(f.Type()))
			} else {
				f.Set(reflect.ValueOf(pb))
			}
			continue
		}

		if val == nil {
			continue
		}

		if reflect.PointerTo(f.Type()).Implements(scannerType) {
			inst := reflect.New(f.Type())
			if err := inst.Interface().(sql.Scanner).Scan(val); err != nil {
				return fmt.Errorf("scan %s: %w", col, err)
			}
			f.Set(inst.Elem())
		} else {
			fv := reflect.ValueOf(val)
			if fv.Type().ConvertibleTo(f.Type()) {
				f.Set(fv.Convert(f.Type()))
			} else {
				return fmt.Errorf("type mismatch for column %s: expected %s, got %s", col, f.Type().String(), fv.Type().String())
			}
		}
	}
	return nil
}

// Map scans the current row into a map.
func Map(rows *sql.Rows) (map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	vals := make([]any, len(cols))
	for i := range vals {
		vals[i] = new(any)
	}
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	if err = rows.Scan(vals...); err != nil {
		return nil, err
	}
	m := make(map[string]any, len(cols))
	for i, c := range cols {
		v := reflect.ValueOf(vals[i]).Elem().Interface()
		if b, ok := v.([]byte); ok {
			m[c] = string(b)
		} else {
			m[c] = v
		}
	}
	return m, nil
}

// Maps scans all remaining rows into slice of maps.
func Maps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var list []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			v := reflect.ValueOf(vals[i]).Elem().Interface()
			if b, ok := v.([]byte); ok {
				m[c] = string(b)
			} else {
				m[c] = v
			}
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

// Structs scans all remaining rows into the slice pointed to by dest.
// dest must be a pointer to a slice of structs.
func Structs(dest any, rows *sql.Rows) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	v := reflect.ValueOf(dest)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("dest must be non-nil pointer to slice")
	}
	v = v.Elem()
	if v.Kind() != reflect.Slice {
		return fmt.Errorf("dest must point to slice")
	}
	elemType := v.Type().Elem()
	scannerType := reflect.TypeOf((*sql.Scanner)(nil)).Elem()
	for rows.Next() {
		fields := make([]any, len(cols))
		for i := range fields {
			fields[i] = new(any)
		}
		if err := rows.Scan(fields...); err != nil {
			return err
		}
		elem := reflect.New(elemType).Elem()
		for i, col := range cols {
			val := reflect.ValueOf(fields[i]).Elem().Interface()
			f := fieldByColumn(elem, col, val != nil)
			if !f.IsValid() || !f.CanSet() {
				continue
			}

			switch f.Type() {
			case reflect.TypeOf(true):
				b, err := parseBoolCompat(val)
				if err != nil {
					return fmt.Errorf("scan %s: %w", col, err)
				}
				f.SetBool(b)
				continue
			case reflect.TypeOf(sql.NullBool{}):
				nb, err := parseNullBoolCompat(val)
				if err != nil {
					return fmt.Errorf("scan %s: %w", col, err)
				}
				f.Set(reflect.ValueOf(nb))
				continue
			}
			if f.Kind() == reflect.Ptr && f.Type().Elem().Kind() == reflect.Bool {
				pb, err := parsePtrBoolCompat(val)
				if err != nil {
					return fmt.Errorf("scan %s: %w", col, err)
				}
				if pb == nil {
					f.Set(reflect.Zero(f.Type()))
				} else {
					f.Set(reflect.ValueOf(pb))
				}
				continue
			}

			if val == nil {
				continue
			}

			if reflect.PointerTo(f.Type()).Implements(scannerType) {
				inst := reflect.New(f.Type())
				if err := inst.Interface().(sql.Scanner).Scan(val); err != nil {
					return fmt.Errorf("scan %s: %w", col, err)
				}
				f.Set(inst.Elem())
			} else {
				fv := reflect.ValueOf(val)
				if fv.Type().ConvertibleTo(f.Type()) {
					f.Set(fv.Convert(f.Type()))
				} else {
					return fmt.Errorf("type mismatch for column %s: expected %s, got %s", col, f.Type().String(), fv.Type().String())
				}
			}
		}
		v.Set(reflect.Append(v, elem))
	}
	return rows.Err()
}

func fieldByColumn(v reflect.Value, col string, allocateNested bool) reflect.Value {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		if fieldUsesPrefix(sf) {
			continue
		}
		name := fieldColumnName(sf)
		if name == col {
			return v.Field(i)
		}
	}

	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		prefix, ok := fieldPrefix(sf)
		if !ok {
			continue
		}
		nestedCol, ok := strings.CutPrefix(col, prefix+"_")
		if !ok || nestedCol == "" {
			continue
		}
		nested := nestedStructValue(v.Field(i), allocateNested)
		if !nested.IsValid() {
			continue
		}
		if f := fieldByColumn(nested, nestedCol, allocateNested); f.IsValid() {
			return f
		}
	}
	return reflect.Value{}
}

func fieldColumnName(sf reflect.StructField) string {
	name := sf.Tag.Get("db")
	if name == "-" {
		return "-"
	}
	if name != "" {
		name = parseDBTagName(name)
	} else if tag := sf.Tag.Get("orm"); tag != "" {
		name = parseORMTag(tag)
	}
	if name == "" {
		name = stringutil.ToSnake(sf.Name)
	}
	return name
}

func parseDBTagName(tag string) string {
	parts := strings.Split(tag, ",")
	return strings.TrimSpace(parts[0])
}

func parseORMTag(tag string) string {
	for _, part := range strings.Split(tag, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && kv[0] == "column" {
			return kv[1]
		}
	}
	return ""
}

func fieldUsesPrefix(sf reflect.StructField) bool {
	_, ok := fieldPrefix(sf)
	return ok
}

func fieldPrefix(sf reflect.StructField) (string, bool) {
	tag := sf.Tag.Get("db")
	if tag == "" || tag == "-" || !tagHasOption(tag, "prefix") {
		return "", false
	}
	name := parseDBTagName(tag)
	if name == "" {
		name = stringutil.ToSnake(sf.Name)
	}
	return name, true
}

func tagHasOption(tag, option string) bool {
	for _, part := range strings.Split(tag, ",")[1:] {
		if strings.TrimSpace(part) == option {
			return true
		}
	}
	return false
}

func nestedStructValue(v reflect.Value, allocate bool) reflect.Value {
	switch v.Kind() {
	case reflect.Struct:
		return v
	case reflect.Ptr:
		if v.Type().Elem().Kind() != reflect.Struct {
			return reflect.Value{}
		}
		if v.IsNil() {
			if !allocate || !v.CanSet() {
				return reflect.Value{}
			}
			v.Set(reflect.New(v.Type().Elem()))
		}
		return v.Elem()
	default:
		return reflect.Value{}
	}
}

// bool parsing helpers with default compatibility policy

func parseBoolCompat(src any) (bool, error) {
	switch v := src.(type) {
	case bool:
		return v, nil
	case int64:
		if v == 0 {
			return false, nil
		}
		return true, nil
	case string:
		x := strings.TrimSpace(strings.ToLower(v))
		switch x {
		case "true", "t", "1":
			return true, nil
		case "false", "f", "0":
			return false, nil
		}
	case []byte:
		x := bytes.TrimSpace(bytes.ToLower(v))
		switch {
		case bytes.Equal(x, []byte("true")), bytes.Equal(x, []byte("t")), bytes.Equal(x, []byte("1")):
			return true, nil
		case bytes.Equal(x, []byte("false")), bytes.Equal(x, []byte("f")), bytes.Equal(x, []byte("0")):
			return false, nil
		}
	case nil:
		// nil into bool returns default value (false) with no error for compatibility
		return false, nil
	}
	return false, fmt.Errorf("cannot parse bool from %T(%v)", src, src)
}

func parseNullBoolCompat(src any) (sql.NullBool, error) {
	if src == nil {
		return sql.NullBool{Bool: false, Valid: false}, nil
	}
	b, err := parseBoolCompat(src)
	if err != nil {
		return sql.NullBool{}, err
	}
	return sql.NullBool{Bool: b, Valid: true}, nil
}

func parsePtrBoolCompat(src any) (*bool, error) {
	if src == nil {
		return nil, nil
	}
	b, err := parseBoolCompat(src)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
