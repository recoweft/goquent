package orm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// JSONField maps a JSON/JSONB column to a typed Go value.
//
// A NULL database value scans to Valid=false and leaves Data as the zero value.
// Use OrDefault when repository code wants a stable default for nullable JSON.
type JSONField[T any] struct {
	Data  T
	Valid bool
}

// JSONOf returns a valid JSONField for v.
func JSONOf[T any](v T) JSONField[T] {
	return JSONField[T]{Data: v, Valid: true}
}

// JSONNull returns an invalid JSONField that stores as SQL NULL.
func JSONNull[T any]() JSONField[T] {
	return JSONField[T]{}
}

// EncodeJSON marshals v for JSON/JSONB or text-JSON columns.
func EncodeJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecodeJSON decodes a JSON/JSONB or text-JSON database value. NULL returns
// fallback without error.
func DecodeJSON[T any](src any, fallback T) (T, error) {
	var field JSONField[T]
	if err := field.Scan(src); err != nil {
		return fallback, err
	}
	return field.OrDefault(fallback), nil
}

// Scan implements sql.Scanner.
func (j *JSONField[T]) Scan(src any) error {
	if j == nil {
		return fmt.Errorf("goquent: JSONField scan target is nil")
	}
	if src == nil {
		var zero T
		j.Data = zero
		j.Valid = false
		return nil
	}
	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("goquent: cannot scan JSONField from %T", src)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("goquent: JSONField cannot scan empty JSON")
	}
	var data T
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	j.Data = data
	j.Valid = true
	return nil
}

// Value implements driver.Valuer.
func (j JSONField[T]) Value() (driver.Value, error) {
	if !j.Valid {
		return nil, nil
	}
	b, err := json.Marshal(j.Data)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// OrDefault returns Data when valid, otherwise def.
func (j JSONField[T]) OrDefault(def T) T {
	if !j.Valid {
		return def
	}
	return j.Data
}

// Validate runs fn for valid JSON values.
func (j JSONField[T]) Validate(fn func(T) error) error {
	if !j.Valid || fn == nil {
		return nil
	}
	return fn(j.Data)
}
