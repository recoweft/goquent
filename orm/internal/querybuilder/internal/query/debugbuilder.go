package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sqlutils"
)

type DebugBuilder[T BaseBuilder, C any] struct {
	queryBuilder T
	child        *C
}

func NewDebugBuilder[T BaseBuilder, C any](queryBuilder T) *DebugBuilder[T, C] {
	return &DebugBuilder[T, C]{
		queryBuilder: queryBuilder,
	}
}

func (b *DebugBuilder[T, C]) SetChild(child *C) *C {
	b.child = child

	return b.child
}

// Dump returns the query and values.
func (b *DebugBuilder[T, C]) Dump() (string, []interface{}, error) {
	return b.queryBuilder.Build()
}

// RawSql returns the raw SQL query.
func (b *DebugBuilder[T, C]) RawSql() (string, error) {
	query, values, err := b.queryBuilder.Build()

	if err != nil {
		return "", err
	}

	return replacePlaceholders(query, values)
}

// replacePlaceholders replaces placeholders with the actual values.
func replacePlaceholders(query string, args []interface{}) (string, error) {
	return sqlutils.InlinePlaceholders(query, args)
}
