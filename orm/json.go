package orm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	ormdriver "github.com/recoweft/goquent/orm/driver"
)

const jsonAggPostgresEmptyArray = "'[]'::jsonb"

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

// JSONPath builds the update-map key used for JSON path updates.
//
// Query-builder updates interpret keys such as "payload->status" as a JSON
// path update. MySQL renders JSON_SET and PostgreSQL renders jsonb_set.
func JSONPath(column string, path ...string) (string, error) {
	column = strings.TrimSpace(column)
	if !safeJSONIdentifier(column) {
		return "", fmt.Errorf("goquent: JSONPath column %q is not a safe identifier", column)
	}
	if len(path) == 0 {
		return "", fmt.Errorf("goquent: JSONPath requires at least one path element")
	}
	parts := make([]string, 0, len(path)+1)
	parts = append(parts, column)
	for _, part := range path {
		part = strings.TrimSpace(part)
		if !safeJSONIdentifier(part) {
			return "", fmt.Errorf("goquent: JSONPath element %q is not a safe identifier", part)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "->"), nil
}

// JSONAggOption configures JSONAggregateArray.
type JSONAggOption func(*jsonAggOptions)

type jsonAggOptions struct {
	orderBy []string
	filter  string
}

// JSONAggOrderBy adds deterministic ordering inside JSONAggregateArray.
func JSONAggOrderBy(orderBy ...string) JSONAggOption {
	return func(o *jsonAggOptions) {
		o.orderBy = append(o.orderBy, orderBy...)
	}
}

// JSONAggFilter adds a PostgreSQL FILTER predicate to JSONAggregateArray.
//
// MySQL does not have SQL-standard aggregate FILTER support, so this option
// returns an error with MySQL dialects.
func JSONAggFilter(predicate string) JSONAggOption {
	return func(o *jsonAggOptions) {
		o.filter = predicate
	}
}

// JSONBuildObject builds a trusted JSON object SQL expression for the dialect.
//
// Field values are trusted SQL expressions. Keys are emitted as SQL string
// literals and sorted for stable output.
func JSONBuildObject(d ormdriver.Dialect, fields map[string]string) (ProjectionExpression, error) {
	if len(fields) == 0 {
		return ProjectionExpression{}, fmt.Errorf("goquent: JSONBuildObject requires fields")
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		if strings.TrimSpace(key) == "" {
			return ProjectionExpression{}, fmt.Errorf("goquent: JSONBuildObject field key is required")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		expr := strings.TrimSpace(fields[key])
		if !safeJSONSQLFragment(expr) {
			return ProjectionExpression{}, fmt.Errorf("goquent: JSONBuildObject field %q expression is not safe", key)
		}
		parts = append(parts, sqlStringLiteral(key), expr)
	}
	switch d.(type) {
	case ormdriver.PostgresDialect:
		return ProjectionSQL("jsonb_build_object(" + strings.Join(parts, ", ") + ")"), nil
	case ormdriver.MySQLDialect:
		return ProjectionSQL("JSON_OBJECT(" + strings.Join(parts, ", ") + ")"), nil
	default:
		return ProjectionExpression{}, fmt.Errorf("goquent: JSONBuildObject unsupported dialect %T", d)
	}
}

// JSONAggregateArray builds a trusted JSON array aggregate expression.
//
// Use JSONBuildObject for the element expression when aggregating object rows.
func JSONAggregateArray(d ormdriver.Dialect, expr ProjectionExpression, opts ...JSONAggOption) (ProjectionExpression, error) {
	sqlExpr := strings.TrimSpace(expr.SQL)
	if !safeJSONSQLFragment(sqlExpr) {
		return ProjectionExpression{}, fmt.Errorf("goquent: JSONAggregateArray expression is not safe")
	}
	cfg := jsonAggOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	orderBy, err := jsonAggOrderBySQL(cfg.orderBy)
	if err != nil {
		return ProjectionExpression{}, err
	}
	filter := strings.TrimSpace(cfg.filter)
	if filter != "" && !safeJSONSQLFragment(filter) {
		return ProjectionExpression{}, fmt.Errorf("goquent: JSONAggregateArray filter is not safe")
	}
	switch d.(type) {
	case ormdriver.PostgresDialect:
		agg := "jsonb_agg(" + sqlExpr + orderBy + ")"
		if filter != "" {
			agg += " FILTER (WHERE " + filter + ")"
		}
		return ProjectionSQL("COALESCE("+agg+", "+jsonAggPostgresEmptyArray+")", expr.Args...), nil
	case ormdriver.MySQLDialect:
		if filter != "" {
			return ProjectionExpression{}, fmt.Errorf("goquent: JSONAggFilter is not supported for MySQL")
		}
		return ProjectionSQL("COALESCE(JSON_ARRAYAGG("+sqlExpr+orderBy+"), JSON_ARRAY())", expr.Args...), nil
	default:
		return ProjectionExpression{}, fmt.Errorf("goquent: JSONAggregateArray unsupported dialect %T", d)
	}
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

func safeJSONIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case unicode.IsLetter(r):
		case i > 0 && unicode.IsDigit(r):
		default:
			return false
		}
	}
	return true
}

func safeJSONSQLFragment(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	upper := strings.ToUpper(s)
	return !strings.Contains(s, ";") &&
		!strings.Contains(s, "--") &&
		!strings.Contains(s, "/*") &&
		!strings.Contains(s, "*/") &&
		!strings.Contains(upper, "\x00")
}

func sqlStringLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func jsonAggOrderBySQL(orderBy []string) (string, error) {
	if len(orderBy) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(orderBy))
	for _, item := range orderBy {
		item = strings.TrimSpace(item)
		if !safeJSONSQLFragment(item) {
			return "", fmt.Errorf("goquent: JSON aggregate order expression is not safe")
		}
		parts = append(parts, item)
	}
	return " ORDER BY " + strings.Join(parts, ", "), nil
}
