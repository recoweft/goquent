package operation

import (
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/recoweft/goquent/orm/manifest"
)

var (
	ErrInputLimit      = errors.New("goquent operation: invalid or excessive input")
	ErrTypeMismatch    = errors.New("goquent operation: value does not satisfy the declared type")
	ErrTypeUnverified  = errors.New("goquent operation: required type information is unverified")
	ErrArrayBinding    = errors.New("goquent operation: array column binding is unsupported")
	ErrReservedBinding = errors.New("goquent operation: reserved binding requires application context")
	ErrInvalidManifest = errors.New("goquent operation: invalid or ambiguous manifest declaration")
)

type columnType struct {
	kind                                     string
	bits, precision, scale, length, fraction int
	unsigned, zone, array                    bool
	missing                                  string
}

var sizedType = regexp.MustCompile(`^(decimal|numeric|char|varchar|time|timestamp|datetime)\(([0-9]+)(?:,([0-9]+))?\)( unsigned| with time zone| without time zone)?$`)
var intLexeme = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var uuidLexeme = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var clockLexeme = regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?$`)
var zoneLexeme = regexp.MustCompile(`^(Z|[+-](0[0-9]|1[0-9]|2[0-3]):[0-5][0-9])$`)
var dateLexeme = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func parseColumnType(c manifest.Column, dialect string) (columnType, error) {
	t := columnType{missing: "type_source_missing"}
	if c.TypeSource != "" && c.TypeSource != "sql" && c.TypeSource != "go" {
		return t, ErrInvalidManifest
	}
	if c.TypeSource != "sql" {
		if c.TypeSource == "go" {
			t.missing = "go_type_not_db_type"
		}
		return t, nil
	}
	if dialect != "mysql" && dialect != "postgres" {
		t.missing = "dialect_unknown"
		return t, nil
	}
	s := strings.ToLower(c.Type)
	if dialect == "postgres" && strings.HasSuffix(s, "[]") {
		elem := c
		elem.Type = strings.TrimSuffix(s, "[]")
		if strings.HasSuffix(elem.Type, "[]") {
			return columnType{array: true, missing: "array_dimension_unsupported"}, nil
		}
		t, err := parseColumnType(elem, dialect)
		t.array = true
		if strings.HasSuffix(elem.Type, "[]") {
			t.missing = "array_dimension_unsupported"
		}
		return t, err
	}
	t = columnType{missing: "type_or_constraints_unknown"}
	if dialect == "mysql" && strings.HasSuffix(s, " unsigned") {
		t.unsigned = true
		s = strings.TrimSuffix(s, " unsigned")
	}
	ints := map[string]int{"smallint": 16, "int": 32, "integer": 32, "bigint": 64}
	if dialect == "mysql" {
		ints["tinyint"] = 8
		ints["mediumint"] = 24
	} else {
		ints["int2"] = 16
		ints["int4"] = 32
		ints["int8"] = 64
	}
	if bits, ok := ints[s]; ok {
		t.kind = "integer"
		t.bits = bits
		t.missing = ""
		return t, nil
	}
	if t.unsigned {
		return t, nil
	}
	switch s {
	case "bool", "boolean":
		t.kind = "bool"
		t.missing = ""
		return t, nil
	case "text":
		t.kind = "string"
		t.missing = ""
		return t, nil
	case "uuid":
		if dialect == "postgres" {
			t.kind = "uuid"
			t.missing = ""
		}
		return t, nil
	case "date":
		t.kind = "date"
		t.missing = ""
		return t, nil
	}
	base, suffix := "", ""
	a, b := -1, -1
	if m := sizedType.FindStringSubmatch(s); m != nil {
		base = m[1]
		var err error
		a, err = strconv.Atoi(m[2])
		if err != nil {
			return t, ErrInvalidManifest
		}
		if m[3] != "" {
			b, err = strconv.Atoi(m[3])
			if err != nil {
				return t, ErrInvalidManifest
			}
		}
		suffix = m[4]
	} else {
		base = s
		for _, z := range []string{" with time zone", " without time zone"} {
			if strings.HasSuffix(base, z) {
				suffix = z
				base = strings.TrimSuffix(base, z)
			}
		}
	}
	switch base {
	case "decimal", "numeric":
		if a < 0 || b < 0 || suffix != "" {
			return t, nil
		}
		maxP, maxS := 1000, 1000
		if dialect == "mysql" {
			maxP, maxS = 65, 30
		}
		if a == 0 || a > maxP || b > a || b > maxS {
			return t, ErrInvalidManifest
		}
		t.kind = "decimal"
		t.precision = a
		t.scale = b
		t.missing = ""
	case "char", "varchar":
		if a == 0 {
			return t, ErrInvalidManifest
		}
		if a < 0 || b >= 0 || suffix != "" {
			return t, nil
		}
		t.kind = "string"
		t.length = a
		t.missing = ""
	case "time", "timestamp", "datetime":
		if b >= 0 || suffix == " unsigned" || a > 6 {
			return t, ErrInvalidManifest
		}
		if base == "datetime" && dialect != "mysql" {
			return t, nil
		}
		if dialect == "mysql" && suffix != "" {
			return t, nil
		}
		if base == "time" && suffix == " with time zone" {
			return t, nil
		}
		t.kind = base
		t.zone = suffix == " with time zone"
		t.fraction = a
		if a < 0 {
			t.fraction = 0
			if dialect == "postgres" {
				t.fraction = 6
			}
		}
		t.missing = ""
		if dialect == "mysql" && base == "timestamp" {
			t.missing = "timestamp_session_semantics_unknown"
		}
	}
	return t, nil
}

func exactInteger(v any) (string, bool) {
	switch n := v.(type) {
	case json.Number:
		s := string(n)
		return s, intLexeme.MatchString(s)
	case int, int8, int16, int32, int64:
		return strconv.FormatInt(reflect.ValueOf(v).Int(), 10), true
	case uint, uint8, uint16, uint32, uint64:
		return strconv.FormatUint(reflect.ValueOf(v).Uint(), 10), true
	}
	return "", false
}
func decimalFits(v any, p, s int) bool {
	lex, ok := exactInteger(v)
	if n, is := v.(json.Number); is {
		lex = string(n)
		ok = json.Valid([]byte(lex)) && len(lex) > 0 && (lex[0] == '-' || lex[0] >= '0' && lex[0] <= '9')
	}
	if !ok {
		return false
	}
	lex = strings.TrimPrefix(lex, "-")
	mant, exp, has := strings.Cut(strings.ToLower(lex), "e")
	whole, frac, _ := strings.Cut(mant, ".")
	digits := strings.TrimLeft(whole+frac, "0")
	if digits == "" {
		return true
	}
	exponent := int64(0)
	if has {
		n, err := strconv.ParseInt(exp, 10, 64)
		if err != nil {
			return false
		}
		exponent = n
	}
	// No powers of ten or exponent-sized allocation. Bound arithmetic first.
	if exponent > int64(len(lex))+int64(p)+1 || exponent < -int64(len(lex))-int64(s)-1 {
		return false
	}
	shift := exponent - int64(len(frac))
	trimmed := strings.TrimRight(digits, "0")
	shift += int64(len(digits) - len(trimmed))
	digits = trimmed
	return shift >= -int64(s) && int64(len(digits))+shift <= int64(p-s)
}

func validateScalar(c manifest.Column, t columnType, dialect, op string, v any) error {
	if v == nil {
		return ErrTypeMismatch
	}
	switch v.(type) {
	case bool, string, json.Number, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
	default:
		return ErrTypeMismatch
	}
	if op == "like" && t.kind != "" && t.kind != "string" {
		return ErrTypeMismatch
	}
	if t.kind == "bool" && op != "=" && op != "!=" && op != "<>" && op != "in" {
		return ErrTypeMismatch
	}
	switch t.kind {
	case "integer":
		s, ok := exactInteger(v)
		if !ok {
			return ErrTypeMismatch
		}
		if t.unsigned {
			if s == "-0" {
				break
			}
			if _, err := strconv.ParseUint(s, 10, t.bits); err != nil {
				return ErrTypeMismatch
			}
		} else if _, err := strconv.ParseInt(s, 10, t.bits); err != nil {
			return ErrTypeMismatch
		}

	case "decimal":
		if !decimalFits(v, t.precision, t.scale) {
			return ErrTypeMismatch
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return ErrTypeMismatch
		}
	case "string", "uuid", "date", "time", "timestamp", "datetime":
		s, ok := v.(string)
		if !ok || !utf8.ValidString(s) {
			return ErrTypeMismatch
		}
		if t.kind == "string" && t.length > 0 && utf8.RuneCountInString(s) > t.length {
			return ErrTypeMismatch
		}
		if t.kind == "uuid" && !uuidLexeme.MatchString(s) {
			return ErrTypeMismatch
		}
		if t.kind == "date" || t.kind == "time" || t.kind == "timestamp" || t.kind == "datetime" {
			if !validTemporal(s, t, dialect) {
				return ErrTypeMismatch
			}
		}
	}
	if len(c.EnumValues) > 0 {
		s, ok := v.(string)
		if !ok {
			return ErrTypeMismatch
		}
		found := false
		for _, e := range c.EnumValues {
			found = found || s == e
		}
		if !found {
			return ErrTypeMismatch
		}
	}
	return nil
}

func validTemporal(s string, t columnType, dialect string) bool {
	if t.kind == "date" {
		if !dateLexeme.MatchString(s) {
			return false
		}
		d, e := time.Parse("2006-01-02", s)
		return e == nil && d.Year() > 0 && (dialect != "mysql" || d.Year() >= 1000)
	}
	clock := s
	layout := "15:04:05"
	if t.kind != "time" {
		if len(s) < 19 || s[10] != 'T' || !dateLexeme.MatchString(s[:10]) {
			return false
		}
		clock = s[11:]
		layout = "2006-01-02T15:04:05"
	}
	if t.zone {
		layout = time.RFC3339Nano
		end := strings.IndexAny(clock, "Z+-")
		if end < 0 || !zoneLexeme.MatchString(clock[end:]) {
			return false
		}
		clock = clock[:end]
	}
	if !clockLexeme.MatchString(clock) {
		return false
	}
	if _, frac, ok := strings.Cut(clock, "."); ok && len(frac) > t.fraction {
		return false
	}
	d, e := time.Parse(layout, s)
	if e != nil {
		return false
	}
	if t.kind != "time" && (d.Year() < 1 || dialect == "mysql" && d.Year() < 1000) {
		return false
	}
	return true
}
