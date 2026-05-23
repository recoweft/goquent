package orm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
)

// SelectOne runs the query and scans the first row into T.
func SelectOne[T any](ctx context.Context, db *DB, q string, args ...any) (T, error) {
	var zero T
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return zero, err
	}
	defer rows.Close()
	return scanRowsOne[T](db, rows)
}

func scanRowsOne[T any](db *DB, rows *sql.Rows) (T, error) {
	var zero T
	var t T
	typ := reflect.TypeOf(t)
	switch {
	case isMapStringInterface(typ):
		cols, err := rows.Columns()
		if err != nil {
			return zero, err
		}
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return zero, err
			}
			return zero, sql.ErrNoRows
		}
		if err := rows.Scan(vals...); err != nil {
			return zero, err
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
		return any(m).(T), nil
	case typ.Kind() == reflect.Struct:
		meta, err := getTypeMeta(typ)
		if err != nil {
			return zero, err
		}
		cols, err := rows.Columns()
		if err != nil {
			return zero, err
		}
		fms := make([]*fieldMeta, len(cols))
		for i, c := range cols {
			if fm, ok := meta.FieldsByName[c]; ok {
				fms[i] = fm
			} else if fm, ok := meta.FieldsByNorm[normalize(c)]; ok {
				fms[i] = fm
			}
		}
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return zero, err
			}
			return zero, sql.ErrNoRows
		}
		if err := rows.Scan(vals...); err != nil {
			return zero, err
		}
		v := reflect.New(typ).Elem()
		scannerType := reflect.TypeOf((*sql.Scanner)(nil)).Elem()
		for i, fm := range fms {
			if fm == nil || fm.IndexPath == nil {
				continue
			}
			val := reflect.ValueOf(vals[i]).Elem().Interface()
			f := v.FieldByIndex(fm.IndexPath)
			if !f.CanSet() {
				continue
			}
			if fm.Decoder != nil {
				pol := db.scanOpts.BoolPolicy
				if fm.BoolPolicy != nil {
					pol = *fm.BoolPolicy
				}
				if err := fm.Decoder(f, val, pol); err != nil {
					if e, ok := err.(ErrBoolParse); ok {
						e.Column = fm.Col
						return zero, e
					}
					return zero, fmt.Errorf("scan %s: %w", fm.Col, err)
				}
				continue
			}
			if val == nil {
				continue
			}
			if reflect.PointerTo(f.Type()).Implements(scannerType) {
				inst := reflect.New(f.Type())
				if err := inst.Interface().(sql.Scanner).Scan(val); err != nil {
					return zero, fmt.Errorf("scan %s: %w", fm.Col, err)
				}
				f.Set(inst.Elem())
			} else {
				fv := reflect.ValueOf(val)
				if fv.Type().AssignableTo(f.Type()) {
					f.Set(fv)
				} else if fv.Type().ConvertibleTo(f.Type()) {
					f.Set(fv.Convert(f.Type()))
				} else {
					return zero, fmt.Errorf("column %q type conversion failed: %s -> %s", fm.Col, fv.Type(), f.Type())
				}
			}
		}
		return v.Interface().(T), nil
	default:
		return zero, fmt.Errorf("unsupported type %s", typ)
	}
}

// SelectAll runs the query and scans all rows into []T.
func SelectAll[T any](ctx context.Context, db *DB, q string, args ...any) ([]T, error) {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRowsAll[T](db, rows)
}

func scanRowsAll[T any](db *DB, rows *sql.Rows) ([]T, error) {
	var t T
	typ := reflect.TypeOf(t)
	switch {
	case isMapStringInterface(typ):
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		var res []T
		for rows.Next() {
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
			res = append(res, any(m).(T))
		}
		return res, rows.Err()
	case typ.Kind() == reflect.Struct:
		meta, err := getTypeMeta(typ)
		if err != nil {
			return nil, err
		}
		cols, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		fms := make([]*fieldMeta, len(cols))
		for i, c := range cols {
			if fm, ok := meta.FieldsByName[c]; ok {
				fms[i] = fm
			} else if fm, ok := meta.FieldsByNorm[normalize(c)]; ok {
				fms[i] = fm
			}
		}
		vals := make([]any, len(cols))
		for i := range vals {
			vals[i] = new(any)
		}
		var res []T
		scannerType := reflect.TypeOf((*sql.Scanner)(nil)).Elem()
		for rows.Next() {
			if err := rows.Scan(vals...); err != nil {
				return nil, err
			}
			v := reflect.New(typ).Elem()
			for i, fm := range fms {
				if fm == nil || fm.IndexPath == nil {
					continue
				}
				val := reflect.ValueOf(vals[i]).Elem().Interface()
				f := v.FieldByIndex(fm.IndexPath)
				if !f.CanSet() {
					continue
				}
				if fm.Decoder != nil {
					pol := db.scanOpts.BoolPolicy
					if fm.BoolPolicy != nil {
						pol = *fm.BoolPolicy
					}
					if err := fm.Decoder(f, val, pol); err != nil {
						if e, ok := err.(ErrBoolParse); ok {
							e.Column = fm.Col
							return nil, e
						}
						return nil, fmt.Errorf("scan %s: %w", fm.Col, err)
					}
					continue
				}
				if val == nil {
					continue
				}
				if reflect.PointerTo(f.Type()).Implements(scannerType) {
					inst := reflect.New(f.Type())
					if err := inst.Interface().(sql.Scanner).Scan(val); err != nil {
						return nil, fmt.Errorf("scan %s: %w", fm.Col, err)
					}
					f.Set(inst.Elem())
				} else {
					fv := reflect.ValueOf(val)
					if fv.Type().AssignableTo(f.Type()) {
						f.Set(fv)
					} else if fv.Type().ConvertibleTo(f.Type()) {
						f.Set(fv.Convert(f.Type()))
					} else {
						return nil, fmt.Errorf("column %q type conversion failed: %s -> %s", fm.Col, fv.Type(), f.Type())
					}
				}
			}
			res = append(res, v.Interface().(T))
		}
		return res, rows.Err()
	default:
		return nil, fmt.Errorf("unsupported type %s", typ)
	}
}

// isMapStringInterface checks if t is map[string]interface{} where the interface has zero methods.
func isMapStringInterface(t reflect.Type) bool {
	return t.Kind() == reflect.Map && t.Key().Kind() == reflect.String && t.Elem().Kind() == reflect.Interface && t.Elem().NumMethod() == 0
}
