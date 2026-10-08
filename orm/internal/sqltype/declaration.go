// Package sqltype recognizes the shared conservative SQL declaration subset.
package sqltype

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

var ErrInvalidDeclaration = errors.New("goquent operation: invalid or ambiguous manifest declaration")

type Declaration struct{ Type, TypeSource string }

type Type struct {
	Kind                                     string
	Bits, Precision, Scale, Length, Fraction int
	Unsigned, Zone, Array                    bool
	Missing                                  string
}

var sizedType = regexp.MustCompile(`^(decimal|numeric|char|varchar|time|timestamp|datetime)\(([0-9]+)(?:,([0-9]+))?\)( unsigned| with time zone| without time zone)?$`)

func Parse(c Declaration, dialect string) (Type, error) {
	t := Type{Missing: "type_source_missing"}
	if c.TypeSource != "" && c.TypeSource != "sql" && c.TypeSource != "go" {
		return t, ErrInvalidDeclaration
	}
	if c.TypeSource != "sql" {
		if c.TypeSource == "go" {
			t.Missing = "go_type_not_db_type"
		}
		return t, nil
	}
	if dialect != "mysql" && dialect != "postgres" {
		t.Missing = "dialect_unknown"
		return t, nil
	}
	s := strings.ToLower(c.Type)
	if dialect == "postgres" && strings.HasSuffix(s, "[]") {
		elem := c
		elem.Type = strings.TrimSuffix(s, "[]")
		if strings.HasSuffix(elem.Type, "[]") {
			return Type{Array: true, Missing: "array_dimension_unsupported"}, nil
		}
		t, err := Parse(elem, dialect)
		t.Array = true
		if strings.HasSuffix(elem.Type, "[]") {
			t.Missing = "array_dimension_unsupported"
		}
		return t, err
	}
	t = Type{Missing: "type_or_constraints_unknown"}
	if dialect == "mysql" && strings.HasSuffix(s, " unsigned") {
		t.Unsigned = true
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
	if Bits, ok := ints[s]; ok {
		t.Kind = "integer"
		t.Bits = Bits
		t.Missing = ""
		return t, nil
	}
	if t.Unsigned {
		return t, nil
	}
	switch s {
	case "bool", "boolean":
		t.Kind = "bool"
		t.Missing = ""
		return t, nil
	case "text":
		t.Kind = "string"
		t.Missing = ""
		return t, nil
	case "uuid":
		if dialect == "postgres" {
			t.Kind = "uuid"
			t.Missing = ""
		}
		return t, nil
	case "date":
		t.Kind = "date"
		t.Missing = ""
		return t, nil
	}
	base, suffix := "", ""
	a, b := -1, -1
	if m := sizedType.FindStringSubmatch(s); m != nil {
		base = m[1]
		var err error
		a, err = strconv.Atoi(m[2])
		if err != nil {
			return t, ErrInvalidDeclaration
		}
		if m[3] != "" {
			b, err = strconv.Atoi(m[3])
			if err != nil {
				return t, ErrInvalidDeclaration
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
			return t, ErrInvalidDeclaration
		}
		t.Kind = "decimal"
		t.Precision = a
		t.Scale = b
		t.Missing = ""
	case "char", "varchar":
		if a == 0 {
			return t, ErrInvalidDeclaration
		}
		if a < 0 || b >= 0 || suffix != "" {
			return t, nil
		}
		t.Kind = "string"
		t.Length = a
		t.Missing = ""
	case "time", "timestamp", "datetime":
		if b >= 0 || suffix == " unsigned" || a > 6 {
			return t, ErrInvalidDeclaration
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
		t.Kind = base
		t.Zone = suffix == " with time zone"
		t.Fraction = a
		if a < 0 {
			t.Fraction = 0
			if dialect == "postgres" {
				t.Fraction = 6
			}
		}
		t.Missing = ""
		if dialect == "mysql" && base == "timestamp" {
			t.Missing = "timestamp_session_semantics_unknown"
		}
	}
	return t, nil
}
