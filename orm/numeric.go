package orm

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// NumericString scans SQL numeric/decimal values as exact text.
//
// Use this for persistence rows where rounding belongs in the domain layer.
type NumericString struct {
	String string
	Valid  bool
}

// NumericStringOf returns a valid NumericString.
func NumericStringOf(value string) NumericString {
	return NumericString{String: value, Valid: true}
}

// NumericStringNull returns an invalid NumericString that stores as SQL NULL.
func NumericStringNull() NumericString {
	return NumericString{}
}

// Scan implements sql.Scanner.
func (n *NumericString) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("goquent: NumericString scan target is nil")
	}
	if src == nil {
		n.String = ""
		n.Valid = false
		return nil
	}
	var value string
	switch v := src.(type) {
	case []byte:
		value = string(v)
	case string:
		value = v
	case int:
		value = strconv.FormatInt(int64(v), 10)
	case int8:
		value = strconv.FormatInt(int64(v), 10)
	case int16:
		value = strconv.FormatInt(int64(v), 10)
	case int32:
		value = strconv.FormatInt(int64(v), 10)
	case int64:
		value = strconv.FormatInt(v, 10)
	case uint:
		value = strconv.FormatUint(uint64(v), 10)
	case uint8:
		value = strconv.FormatUint(uint64(v), 10)
	case uint16:
		value = strconv.FormatUint(uint64(v), 10)
	case uint32:
		value = strconv.FormatUint(uint64(v), 10)
	case uint64:
		value = strconv.FormatUint(v, 10)
	case float32:
		value = strconv.FormatFloat(float64(v), 'g', -1, 32)
	case float64:
		value = strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return fmt.Errorf("goquent: cannot scan NumericString from %T", src)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("goquent: NumericString cannot scan empty numeric text")
	}
	n.String = value
	n.Valid = true
	return nil
}

// Value implements driver.Valuer.
func (n NumericString) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	value := strings.TrimSpace(n.String)
	if value == "" {
		return nil, fmt.Errorf("goquent: NumericString cannot store empty numeric text")
	}
	return value, nil
}

// OrDefault returns String when valid, otherwise def.
func (n NumericString) OrDefault(def string) string {
	if !n.Valid {
		return def
	}
	return n.String
}
