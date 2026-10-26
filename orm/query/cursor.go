package query

import (
	"fmt"
	"strings"
)

// CursorColumn describes an ordered column used by keyset cursor predicates.
type CursorColumn struct {
	Name      string
	Direction string
	Raw       bool
}

// CursorAsc returns an ascending keyset cursor column.
func CursorAsc(name string) CursorColumn {
	return CursorColumn{Name: name, Direction: "asc"}
}

// CursorDesc returns a descending keyset cursor column.
func CursorDesc(name string) CursorColumn {
	return CursorColumn{Name: name, Direction: "desc"}
}

// CursorAscExpr returns an ascending trusted SQL expression for keyset cursor
// predicates.
func CursorAscExpr(expr string) CursorColumn {
	return CursorColumn{Name: expr, Direction: "asc", Raw: true}
}

// CursorDescExpr returns a descending trusted SQL expression for keyset cursor
// predicates.
func CursorDescExpr(expr string) CursorColumn {
	return CursorColumn{Name: expr, Direction: "desc", Raw: true}
}

// CursorAscAlias returns an ascending selected alias cursor column.
func CursorAscAlias(alias string) CursorColumn { return CursorAsc(alias) }

// CursorDescAlias returns a descending selected alias cursor column.
func CursorDescAlias(alias string) CursorColumn { return CursorDesc(alias) }

// WhereCursorAfter adds a keyset pagination predicate after the given cursor.
func (q *Query) WhereCursorAfter(columns []CursorColumn, values ...any) *Query {
	return q.whereCursor(columns, values, true)
}

// WhereCursorBefore adds a keyset pagination predicate before the given cursor.
func (q *Query) WhereCursorBefore(columns []CursorColumn, values ...any) *Query {
	return q.whereCursor(columns, values, false)
}

func (q *Query) whereCursor(columns []CursorColumn, values []any, after bool) *Query {
	if q.err != nil {
		return q
	}
	normalized, err := validateCursorColumns(columns, values)
	if err != nil {
		q.err = err
		return q
	}
	raw, vals := q.cursorPredicate(normalized, values, after)
	q.builder.WhereRaw(raw, vals)
	return q
}

func (q *Query) cursorPredicate(columns []CursorColumn, values []any, after bool) (string, map[string]any) {
	vals := make(map[string]any)
	parts := make([]string, 0, len(columns))
	for i := range columns {
		comparisons := make([]string, 0, i+1)
		for j := 0; j < i; j++ {
			name := fmt.Sprintf("__goquent_cursor_%d_%d", i, j)
			vals[name] = values[j]
			comparisons = append(comparisons, fmt.Sprintf("%s = :%s", q.cursorColumnSQL(columns[j]), name))
		}
		name := fmt.Sprintf("__goquent_cursor_%d_%d", i, i)
		vals[name] = values[i]
		comparisons = append(comparisons, fmt.Sprintf("%s %s :%s", q.cursorColumnSQL(columns[i]), cursorComparisonOperator(columns[i].Direction, after), name))
		parts = append(parts, "("+strings.Join(comparisons, " AND ")+")")
	}
	return "(" + strings.Join(parts, " OR ") + ")", vals
}

func (q *Query) cursorColumnSQL(column CursorColumn) string {
	if column.Raw {
		return column.Name
	}
	return quoteIdentifierPath(q.dialect, column.Name)
}

func validateCursorColumns(columns []CursorColumn, values []any) ([]CursorColumn, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("goquent: cursor columns are required")
	}
	if len(columns) != len(values) {
		return nil, fmt.Errorf("goquent: cursor column/value count mismatch")
	}
	normalized := make([]CursorColumn, len(columns))
	for i, col := range columns {
		name := strings.TrimSpace(col.Name)
		if name == "" {
			return nil, fmt.Errorf("goquent: cursor column name is required")
		}
		if col.Raw {
			if err := validateRawSQLFragment(name); err != nil {
				return nil, fmt.Errorf("goquent: invalid cursor expression %q: %w", col.Name, err)
			}
		} else {
			if strings.Contains(name, "*") {
				return nil, fmt.Errorf("goquent: cursor column %q cannot be a wildcard", col.Name)
			}
			if err := validateSelectColumn(name); err != nil {
				return nil, fmt.Errorf("goquent: invalid cursor column %q: %w", col.Name, err)
			}
		}
		dir, err := validateOrderDirection(col.Direction)
		if err != nil {
			return nil, err
		}
		if values[i] == nil {
			return nil, fmt.Errorf("goquent: cursor value for %s is nil", name)
		}
		normalized[i] = CursorColumn{Name: name, Direction: dir, Raw: col.Raw}
	}
	return normalized, nil
}

func cursorComparisonOperator(direction string, after bool) string {
	desc := strings.EqualFold(direction, "desc")
	if after {
		if desc {
			return "<"
		}
		return ">"
	}
	if desc {
		return ">"
	}
	return "<"
}
