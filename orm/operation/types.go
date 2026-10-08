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

	"github.com/recoweft/goquent/orm/internal/sqltype"
	"github.com/recoweft/goquent/orm/manifest"
)

var (
	ErrInputLimit      = errors.New("goquent operation: invalid or excessive input")
	ErrTypeMismatch    = errors.New("goquent operation: value does not satisfy the declared type")
	ErrTypeUnverified  = errors.New("goquent operation: required type information is unverified")
	ErrArrayBinding    = errors.New("goquent operation: array column binding is unsupported")
	ErrReservedBinding = errors.New("goquent operation: reserved binding requires application context")
	ErrInvalidManifest = sqltype.ErrInvalidDeclaration
)

type columnType struct {
	kind                                     string
	bits, precision, scale, length, fraction int
	unsigned, zone, array                    bool
	missing                                  string
}

var intLexeme = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
var uuidLexeme = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var clockLexeme = regexp.MustCompile(`^[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?$`)
var zoneLexeme = regexp.MustCompile(`^(Z|[+-](0[0-9]|1[0-9]|2[0-3]):[0-5][0-9])$`)
var dateLexeme = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

func parseColumnType(c manifest.Column, dialect string) (columnType, error) {
	t, err := sqltype.Parse(sqltype.Declaration{Type: c.Type, TypeSource: c.TypeSource}, dialect)
	return columnType{kind: t.Kind, bits: t.Bits, precision: t.Precision, scale: t.Scale, length: t.Length, fraction: t.Fraction, unsigned: t.Unsigned, zone: t.Zone, array: t.Array, missing: t.Missing}, err
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
