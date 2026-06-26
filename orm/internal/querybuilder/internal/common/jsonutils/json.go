package jsonutils

import (
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

// ParseJsonFieldAndPath splits a column in the form "field->path1->path2" into
// the base field and a slice of JSON path elements.
func ParseJsonFieldAndPath(column string) (string, []string) {
	parts := strings.Split(column, "->")
	field := parts[0]
	if len(parts) > 1 {
		return field, parts[1:]
	}
	return field, []string{}
}

// BuildJsonPathSQL builds the SQL fragment for accessing a JSON path using the
// given SQL utility for escaping identifiers.
func BuildJsonPathSQL(u interfaces.SQLUtils, field string, path []string) []byte {
	sb := make([]byte, 0, 32)
	sb = append(sb, '(')
	sb = u.EscapeReference(sb, field)
	for _, p := range path {
		sb = append(sb, "->"...)
		sb = append(sb, '\'')
		sb = append(sb, p...)
		sb = append(sb, '\'')
	}
	sb = append(sb, ')')
	return sb
}
