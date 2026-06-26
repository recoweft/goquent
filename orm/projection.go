package orm

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/recoweft/goquent/orm/query"
)

// ParentChildProjection describes how to fold flat joined rows into parent
// records with ordered child collections.
type ParentChildProjection[R any, P any, C any, K comparable] struct {
	// ParentKey returns the stable parent key for one flat row.
	ParentKey func(R) K
	// Parent builds a parent value from the first row for a key.
	Parent func(R) P
	// Child builds a child value. Return ok=false for LEFT JOIN rows without a child.
	Child func(R) (child C, ok bool)
	// AppendChild appends child to parent.
	AppendChild func(parent *P, child C)
}

// ProjectParentChildren folds flat rows into parents with ordered children.
//
// Parent order follows first appearance in rows. Child order follows row order,
// so callers should put the desired child order in the SELECT query.
func ProjectParentChildren[R any, P any, C any, K comparable](rows []R, spec ParentChildProjection[R, P, C, K]) ([]P, error) {
	if spec.ParentKey == nil {
		return nil, fmt.Errorf("goquent: parent key projection is required")
	}
	if spec.Parent == nil {
		return nil, fmt.Errorf("goquent: parent projection is required")
	}
	if spec.Child == nil {
		return nil, fmt.Errorf("goquent: child projection is required")
	}
	if spec.AppendChild == nil {
		return nil, fmt.Errorf("goquent: child append projection is required")
	}

	parents := make([]P, 0)
	index := make(map[K]int, len(rows))
	for _, row := range rows {
		key := spec.ParentKey(row)
		parentIndex, ok := index[key]
		if !ok {
			parentIndex = len(parents)
			index[key] = parentIndex
			parents = append(parents, spec.Parent(row))
		}
		child, ok := spec.Child(row)
		if !ok {
			continue
		}
		spec.AppendChild(&parents[parentIndex], child)
	}
	return parents, nil
}

// AggregateHydration describes how to attach one grouped aggregate row to a
// parent record. Aggregate queries should return at most one row per parent key.
type AggregateHydration[P any, A any, K comparable] struct {
	// ParentKey returns the stable parent key.
	ParentKey func(P) K
	// AggregateKey returns the parent key represented by one aggregate row.
	AggregateKey func(A) K
	// Apply copies aggregate values onto parent.
	Apply func(parent *P, aggregate A)
}

// HydrateAggregates attaches grouped aggregate rows to parent records.
//
// Parent order is preserved. Parents without aggregate rows are left unchanged.
// Duplicate aggregate rows for the same key return an error because grouped
// aggregate queries should produce a single row per parent.
func HydrateAggregates[P any, A any, K comparable](parents []P, aggregates []A, spec AggregateHydration[P, A, K]) ([]P, error) {
	if spec.ParentKey == nil {
		return nil, fmt.Errorf("goquent: aggregate parent key projection is required")
	}
	if spec.AggregateKey == nil {
		return nil, fmt.Errorf("goquent: aggregate key projection is required")
	}
	if spec.Apply == nil {
		return nil, fmt.Errorf("goquent: aggregate apply projection is required")
	}

	byKey := make(map[K]A, len(aggregates))
	for _, aggregate := range aggregates {
		key := spec.AggregateKey(aggregate)
		if _, ok := byKey[key]; ok {
			return nil, fmt.Errorf("goquent: duplicate aggregate row for key %v", key)
		}
		byKey[key] = aggregate
	}

	out := append([]P(nil), parents...)
	for i := range out {
		aggregate, ok := byKey[spec.ParentKey(out[i])]
		if !ok {
			continue
		}
		spec.Apply(&out[i], aggregate)
	}
	return out, nil
}

// RepresentativeGroup is one grouped projection with the first row for the key
// and the number of rows that shared that key.
type RepresentativeGroup[R any, K comparable] struct {
	Key            K
	Representative R
	Count          int
}

// GroupRepresentativeRows groups rows by key, preserving first-seen group order.
//
// Representative is the first row encountered for the group. Put the desired
// representative ordering in SQL before calling this helper.
func GroupRepresentativeRows[R any, K comparable](rows []R, key func(R) K) ([]RepresentativeGroup[R, K], error) {
	if key == nil {
		return nil, fmt.Errorf("goquent: group key projection is required")
	}
	groups := make([]RepresentativeGroup[R, K], 0)
	index := make(map[K]int, len(rows))
	for _, row := range rows {
		k := key(row)
		i, ok := index[k]
		if !ok {
			index[k] = len(groups)
			groups = append(groups, RepresentativeGroup[R, K]{
				Key:            k,
				Representative: row,
				Count:          1,
			})
			continue
		}
		groups[i].Count++
	}
	return groups, nil
}

// ProjectionExpression is one SELECT expression for a typed projection column.
type ProjectionExpression struct {
	SQL  string
	Args []any
}

// ProjectionSQL builds a trusted SELECT expression for ApplyProjection.
func ProjectionSQL(sql string, args ...any) ProjectionExpression {
	return ProjectionExpression{SQL: sql, Args: append([]any(nil), args...)}
}

// ApplyProjection selects every db-tagged column in T from q.
//
// Expressions maps destination column names to SELECT expressions. Missing
// columns are selected as NULL, which keeps UNION branches aligned without
// repeating all nullable/default columns in every branch.
func ApplyProjection[T any](q *query.Query, expressions map[string]ProjectionExpression) (*query.Query, error) {
	if q == nil {
		return nil, fmt.Errorf("goquent: projection query is nil")
	}
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil {
		return nil, fmt.Errorf("goquent: projection type is required")
	}
	cols, err := structColumnNames(typ)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("goquent: projection type has no columns")
	}
	colSet := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		if !safeProjectionAlias(col) {
			return nil, fmt.Errorf("goquent: projection column %q is not a safe SQL alias", col)
		}
		colSet[col] = struct{}{}
	}
	for col := range expressions {
		if _, ok := colSet[col]; !ok {
			return nil, fmt.Errorf("goquent: projection expression column %q is not in destination type", col)
		}
	}
	for _, col := range cols {
		expr, ok := expressions[col]
		if !ok {
			expr = ProjectionSQL("NULL")
		}
		sql := strings.TrimSpace(expr.SQL)
		if sql == "" {
			return nil, fmt.Errorf("goquent: projection expression for %s is required", col)
		}
		q = q.SelectRaw(fmt.Sprintf("%s AS %s", sql, col), expr.Args...)
	}
	return q, nil
}

func safeProjectionAlias(alias string) bool {
	if alias == "" {
		return false
	}
	for i, r := range alias {
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
