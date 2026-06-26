package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

type WhereQueryBuilder[T QueryBuilderStrategy[T, C], C any] struct {
	builder *query.WhereBuilder[C]
	parent  *T
}

func NewWhereQueryBuilder[T QueryBuilderStrategy[T, C], C any](strategy interfaces.QueryBuilderStrategy) *WhereQueryBuilder[T, C] {
	return &WhereQueryBuilder[T, C]{
		builder: query.NewWhereBuilder[C](strategy),
	}
}

// WhereSelectQueryBuilder is a type that represents a where select builder
type WhereSelectQueryBuilder = WhereQueryBuilder[*SelectQueryBuilder, query.SelectBuilder]

// WhereInsertBuilder is a type that represents a where insert builder
type WhereUpdateQueryBuilder = WhereQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder]

// WhereDeleteQueryBuilder is a type that represents a where delete builder
type WhereDeleteQueryBuilder = WhereQueryBuilder[*DeleteQueryBuilder, query.DeleteBuilder]

func (b *WhereQueryBuilder[T, C]) SetParent(parent *T) *T {
	b.parent = parent

	return b.parent
}

// Where is a function that allows you to add a where condition
func (wb *WhereQueryBuilder[T, C]) Where(column string, condition string, value interface{}) T {
	switch v := value.(type) {
	case *SelectQueryBuilder:
		(*wb.parent).GetWhereBuilder().WhereSubQuery(column, condition, v.builder)
	case []interface{}:
		(*wb.parent).GetWhereBuilder().Where(column, condition, v...)
	default:
		(*wb.parent).GetWhereBuilder().Where(column, condition, value)
	}
	return (*wb.parent).GetQueryBuilder()
}

// OrWhere is a function that allows you to add a or where condition
func (wb *WhereQueryBuilder[T, C]) OrWhere(column string, condition string, value interface{}) T {
	switch v := value.(type) {
	case *SelectQueryBuilder:
		(*wb.parent).GetWhereBuilder().OrWhereSubQuery(column, condition, v.builder)
	case []interface{}:
		(*wb.parent).GetWhereBuilder().OrWhere(column, condition, v...)
	default:
		(*wb.parent).GetWhereBuilder().OrWhere(column, condition, value)
	}
	return (*wb.parent).GetQueryBuilder()
}

// WhereSubQuery is a function that allows you to add a where query condition
func (wb *WhereQueryBuilder[T, C]) WhereSubQuery(column string, condition string, qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().WhereSubQuery(column, condition, qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereSubQuery is a function that allows you to add a or where query condition
func (wb *WhereQueryBuilder[T, C]) OrWhereSubQuery(column string, condition string, qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().OrWhereSubQuery(column, condition, qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// WhereRaw adds a raw SQL condition with AND operator
func (wb *WhereQueryBuilder[T, C]) WhereRaw(raw string, values map[string]any) T {
	(*wb.parent).GetWhereBuilder().SafeWhereRaw(raw, values)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereRaw adds a raw SQL condition with OR operator
func (wb *WhereQueryBuilder[T, C]) OrWhereRaw(raw string, values map[string]any) T {
	(*wb.parent).GetWhereBuilder().SafeOrWhereRaw(raw, values)
	return (*wb.parent).GetQueryBuilder()
}

// SafeWhereRaw adds a raw where clause with AND operator while enforcing parameter usage.
func (wb *WhereQueryBuilder[T, C]) SafeWhereRaw(raw string, values map[string]any) T {
	(*wb.parent).GetWhereBuilder().SafeWhereRaw(raw, values)
	return (*wb.parent).GetQueryBuilder()
}

// SafeOrWhereRaw adds a raw where clause with OR operator while enforcing parameter usage.
func (wb *WhereQueryBuilder[T, C]) SafeOrWhereRaw(raw string, values map[string]any) T {
	(*wb.parent).GetWhereBuilder().SafeOrWhereRaw(raw, values)
	return (*wb.parent).GetQueryBuilder()
}

// WhereGroup is a function that allows you to group where conditions
func (wb *WhereQueryBuilder[T, C]) WhereGroup(fn func(wqb *WhereQueryBuilder[T, C])) T {
	(*wb.parent).GetWhereBuilder().WhereGroup(func(b *query.WhereBuilder[C]) {
		fn(&WhereQueryBuilder[T, C]{builder: b, parent: wb.parent})
	})
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereGroup is a function that allows you to group or where conditions
func (wb *WhereQueryBuilder[T, C]) OrWhereGroup(fn func(wb *WhereQueryBuilder[T, C])) T {
	(*wb.parent).GetWhereBuilder().OrWhereGroup(func(b *query.WhereBuilder[C]) {
		fn(&WhereQueryBuilder[T, C]{builder: b, parent: wb.parent})
	})
	return (*wb.parent).GetQueryBuilder()
}

// WhereNot is a function that allows you to add a where not condition
func (wb *WhereQueryBuilder[T, C]) WhereNot(fn func(wb *WhereQueryBuilder[T, C])) T {
	(*wb.parent).GetWhereBuilder().WhereNot(func(b *query.WhereBuilder[C]) {
		fn(&WhereQueryBuilder[T, C]{builder: b, parent: wb.parent})
	})
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNot is a function that allows you to add a or where not condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNot(fn func(wb *WhereQueryBuilder[T, C])) T {
	(*wb.parent).GetWhereBuilder().OrWhereNot(func(b *query.WhereBuilder[C]) {
		fn(&WhereQueryBuilder[T, C]{builder: b, parent: wb.parent})
	})
	return (*wb.parent).GetQueryBuilder()
}

// WhereAny is a function that allows you to add a where any condition
func (wb *WhereQueryBuilder[T, C]) WhereAny(columns []string, condition string, value interface{}) T {
	(*wb.parent).GetWhereBuilder().WhereAny(columns, condition, value)
	return (*wb.parent).GetQueryBuilder()
}

// WhereAll is a function that allows you to add a where all condition
func (wb *WhereQueryBuilder[T, C]) WhereAll(columns []string, condition string, value interface{}) T {
	(*wb.parent).GetWhereBuilder().WhereAll(columns, condition, value)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereAny is a function that allows you to add a or where any condition
func (wb *WhereQueryBuilder[T, C]) WhereIn(column string, values interface{}) T {
	switch casted := values.(type) {
	case *SelectQueryBuilder:
		(*wb.parent).GetWhereBuilder().WhereInSubQuery(column, casted.builder)
	default:
		(*wb.parent).GetWhereBuilder().WhereIn(column, values)
	}
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereAll is a function that allows you to add a or where all condition
func (wb *WhereQueryBuilder[T, C]) WhereNotIn(column string, values interface{}) T {
	switch casted := values.(type) {
	case *SelectQueryBuilder:
		(*wb.parent).GetWhereBuilder().WhereNotInSubQuery(column, casted.builder)
	default:
		(*wb.parent).GetWhereBuilder().WhereNotIn(column, values)
	}
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereIn is a function that allows you to add a or where in condition
func (wb *WhereQueryBuilder[T, C]) OrWhereIn(column string, values interface{}) T {
	switch casted := values.(type) {
	case *SelectQueryBuilder:
		(*wb.parent).GetWhereBuilder().OrWhereInSubQuery(column, casted.builder)
	default:
		(*wb.parent).GetWhereBuilder().OrWhereIn(column, values)
	}
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotIn is a function that allows you to add a or where not in condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotIn(column string, values interface{}) T {
	switch casted := values.(type) {
	case *SelectQueryBuilder:
		(*wb.parent).GetWhereBuilder().OrWhereNotInSubQuery(column, casted.builder)
	default:
		(*wb.parent).GetWhereBuilder().OrWhereNotIn(column, values)
	}
	return (*wb.parent).GetQueryBuilder()
}

// WhereInSubQuery is a function that allows you to add a where in sub query condition
func (wb *WhereQueryBuilder[T, C]) WhereInSubQuery(column string, qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().WhereInSubQuery(column, qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// WhereNotInSubQuery is a function that allows you to add a where not in sub query condition
func (wb *WhereQueryBuilder[T, C]) WhereNotInSubQuery(column string, qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().WhereNotInSubQuery(column, qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereInSubQuery is a function that allows you to add a or where in sub query condition
func (wb *WhereQueryBuilder[T, C]) OrWhereInSubQuery(column string, qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().OrWhereInSubQuery(column, qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotInSubQuery is a function that allows you to add a or where not in sub query condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotInSubQuery(column string, qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().OrWhereNotInSubQuery(column, qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// WhereNull is a function that allows you to add a where null condition
func (wb *WhereQueryBuilder[T, C]) WhereNull(column string) T {
	(*wb.parent).GetWhereBuilder().WhereNull(column)
	return (*wb.parent).GetQueryBuilder()
}

// WhereNotNull is a function that allows you to add a where not null condition
func (wb *WhereQueryBuilder[T, C]) WhereNotNull(column string) T {
	(*wb.parent).GetWhereBuilder().WhereNotNull(column)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNull is a function that allows you to add a or where null condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNull(column string) T {
	(*wb.parent).GetWhereBuilder().OrWhereNull(column)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotNull is a function that allows you to add a or where not null condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotNull(column string) T {
	(*wb.parent).GetWhereBuilder().OrWhereNotNull(column)
	return (*wb.parent).GetQueryBuilder()
}

// WhereColumn is a function that allows you to add a where column condition
func (wb *WhereQueryBuilder[T, C]) WhereColumn(allColumns []string, column string, cond ...string) T {
	operator := consts.Condition_EQUAL
	valueColumn := column
	if len(cond) > 0 {
		valueColumn = cond[0]
	}
	if len(cond) > 1 {
		operator = cond[0]
		valueColumn = cond[1]
	}

	(*wb.parent).GetWhereBuilder().WhereColumn(allColumns, column, operator, valueColumn)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereColumn is a function that allows you to add a or where column condition
func (wb *WhereQueryBuilder[T, C]) OrWhereColumn(allColumns []string, column string, cond ...string) T {
	operator := consts.Condition_EQUAL
	valueColumn := column
	if len(cond) > 0 {
		valueColumn = cond[0]
	}
	if len(cond) > 1 {
		operator = cond[0]
		valueColumn = cond[1]
	}

	(*wb.parent).GetWhereBuilder().OrWhereColumn(allColumns, column, operator, valueColumn)
	return (*wb.parent).GetQueryBuilder()
}

// WhereColumns is a function that allows you to add a where columns condition
func (wb *WhereQueryBuilder[T, C]) WhereColumns(allColumns []string, columns [][]string) T {
	(*wb.parent).GetWhereBuilder().WhereColumns(allColumns, columns)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereColumns is a function that allows you to add a or where columns condition
func (wb *WhereQueryBuilder[T, C]) OrWhereColumns(allColumns []string, columns [][]string) T {
	(*wb.parent).GetWhereBuilder().OrWhereColumns(allColumns, columns)
	return (*wb.parent).GetQueryBuilder()
}

// WhereBetween is a function that allows you to add a where between condition
func (wb *WhereQueryBuilder[T, C]) WhereBetween(column string, min interface{}, max interface{}) T {
	(*wb.parent).GetWhereBuilder().WhereBetween(column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereBetween is a function that allows you to add a or where between condition
func (wb *WhereQueryBuilder[T, C]) OrWhereBetween(column string, min interface{}, max interface{}) T {
	(*wb.parent).GetWhereBuilder().OrWhereBetween(column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// WhereNotBetween is a function that allows you to add a where not between condition
func (wb *WhereQueryBuilder[T, C]) WhereNotBetween(column string, min interface{}, max interface{}) T {
	(*wb.parent).GetWhereBuilder().WhereNotBetween(column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotBetween is a function that allows you to add a or where not between condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotBetween(column string, min interface{}, max interface{}) T {
	(*wb.parent).GetWhereBuilder().OrWhereNotBetween(column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// WhereBetweenColumns is a function that allows you to add a where between columns condition
func (wb *WhereQueryBuilder[T, C]) WhereBetweenColumns(allColumns []string, column string, min string, max string) T {
	(*wb.parent).GetWhereBuilder().WhereBetweenColumns(allColumns, column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereBetweenColumns is a function that allows you to add a or where between columns condition
func (wb *WhereQueryBuilder[T, C]) OrWhereBetweenColumns(allColumns []string, column string, min string, max string) T {
	(*wb.parent).GetWhereBuilder().OrWhereBetweenColumns(allColumns, column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// WhereNotBetweenColumns is a function that allows you to add a where not between columns condition
func (wb *WhereQueryBuilder[T, C]) WhereNotBetweenColumns(allColumns []string, column string, min string, max string) T {
	(*wb.parent).GetWhereBuilder().WhereNotBetweenColumns(allColumns, column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotBetweenColumns is a function that allows you to add a or where not between columns condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotBetweenColumns(allColumns []string, column string, min string, max string) T {
	(*wb.parent).GetWhereBuilder().OrWhereNotBetweenColumns(allColumns, column, min, max)
	return (*wb.parent).GetQueryBuilder()
}

func (wb *WhereQueryBuilder[T, C]) WhereExists(fn func(q *SelectQueryBuilder)) T {
	(*wb.parent).GetWhereBuilder().WhereExists(func(b *query.SelectBuilder) {
		sqb := NewSelectQueryBuilder(b.GetStrategy())
		sqb.builder = b
		fn(sqb)
	})
	return (*wb.parent).GetQueryBuilder()
}

// WhereDateQuery is a function that allows you to add a where date condition
func (wb *WhereQueryBuilder[T, C]) WhereExistsSubQuery(qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().WhereExistsQuery(qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereExists is a function that allows you to add a or where exists condition
func (wb *WhereQueryBuilder[T, C]) OrWhereExists(fn func(q *SelectQueryBuilder)) T {
	(*wb.parent).GetWhereBuilder().OrWhereExists(func(b *query.SelectBuilder) {
		sqb := NewSelectQueryBuilder(b.GetStrategy())
		sqb.builder = b
		fn(sqb)
	})
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereExistsSubQuery is a function that allows you to add a or where exists condition
func (wb *WhereQueryBuilder[T, C]) OrWhereExistsSubQuery(qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().OrWhereExistsQuery(qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// WhereNotExists is a function that allows you to add a where not exists condition
func (wb *WhereQueryBuilder[T, C]) WhereNotExists(fn func(q *SelectQueryBuilder)) T {
	(*wb.parent).GetWhereBuilder().WhereNotExists(func(b *query.SelectBuilder) {
		sqb := NewSelectQueryBuilder(b.GetStrategy())
		sqb.builder = b
		fn(sqb)
	})
	return (*wb.parent).GetQueryBuilder()
}

// WhereNotExistsQuery is a function that allows you to add a where not exists condition
func (wb *WhereQueryBuilder[T, C]) WhereNotExistsQuery(qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().WhereNotExistsQuery(qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotExists is a function that allows you to add a or where not exists condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotExists(fn func(q *SelectQueryBuilder)) T {
	(*wb.parent).GetWhereBuilder().OrWhereNotExists(func(b *query.SelectBuilder) {
		sqb := NewSelectQueryBuilder(b.GetStrategy())
		sqb.builder = b
		fn(sqb)
	})
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereNotExistsQuery is a function that allows you to add a or where not exists condition
func (wb *WhereQueryBuilder[T, C]) OrWhereNotExistsQuery(qb *SelectQueryBuilder) T {
	(*wb.parent).GetWhereBuilder().OrWhereNotExistsQuery(qb.builder)
	return (*wb.parent).GetQueryBuilder()
}

// WhereFullText is a function that allows you to add a where full text condition
func (wb *WhereQueryBuilder[T, C]) WhereFullText(columns []string, value string, options map[string]interface{}) T {
	(*wb.parent).GetWhereBuilder().WhereFullText(columns, value, options)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereFullText is a function that allows you to add a or where full text condition
func (wb *WhereQueryBuilder[T, C]) OrWhereFullText(columns []string, value string, options map[string]interface{}) T {
	(*wb.parent).GetWhereBuilder().OrWhereFullText(columns, value, options)
	return (*wb.parent).GetQueryBuilder()
}

// WhereDate is a function that allows you to add a where date condition
func (wb *WhereQueryBuilder[T, C]) WhereDate(column string, cond string, date string) T {
	(*wb.parent).GetWhereBuilder().WhereDate(column, cond, date)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereDate is a function that allows you to add a or where date condition
func (wb *WhereQueryBuilder[T, C]) OrWhereDate(column string, cond string, date string) T {
	(*wb.parent).GetWhereBuilder().OrWhereDate(column, cond, date)
	return (*wb.parent).GetQueryBuilder()
}

// WhereTime is a function that allows you to add a where time condition
func (wb *WhereQueryBuilder[T, C]) WhereTime(column string, cond string, time string) T {
	(*wb.parent).GetWhereBuilder().WhereTime(column, cond, time)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereTime is a function that allows you to add a or where time condition
func (wb *WhereQueryBuilder[T, C]) OrWhereTime(column string, cond string, time string) T {
	(*wb.parent).GetWhereBuilder().OrWhereTime(column, cond, time)
	return (*wb.parent).GetQueryBuilder()
}

// WhereDay is a function that allows you to add a where day condition
func (wb *WhereQueryBuilder[T, C]) WhereDay(column string, cond string, day string) T {
	(*wb.parent).GetWhereBuilder().WhereDay(column, cond, day)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereDay is a function that allows you to add a or where day condition
func (wb *WhereQueryBuilder[T, C]) OrWhereDay(column string, cond string, day string) T {
	(*wb.parent).GetWhereBuilder().OrWhereDay(column, cond, day)
	return (*wb.parent).GetQueryBuilder()
}

// WhereMonth is a function that allows you to add a where month condition
func (wb *WhereQueryBuilder[T, C]) WhereMonth(column string, cond string, month string) T {
	(*wb.parent).GetWhereBuilder().WhereMonth(column, cond, month)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereMonth is a function that allows you to add a or where month condition
func (wb *WhereQueryBuilder[T, C]) OrWhereMonth(column string, cond string, month string) T {
	(*wb.parent).GetWhereBuilder().OrWhereMonth(column, cond, month)
	return (*wb.parent).GetQueryBuilder()
}

// WhereYear is a function that allows you to add a where year condition
func (wb *WhereQueryBuilder[T, C]) WhereYear(column string, cond string, year string) T {
	(*wb.parent).GetWhereBuilder().WhereYear(column, cond, year)
	return (*wb.parent).GetQueryBuilder()
}

// OrWhereYear is a function that allows you to add a or where year condition
func (wb *WhereQueryBuilder[T, C]) OrWhereYear(column string, cond string, year string) T {
	(*wb.parent).GetWhereBuilder().OrWhereYear(column, cond, year)
	return (*wb.parent).GetQueryBuilder()
}

// GetBuilder is a function that allows you to get the where builder
func (wb *WhereQueryBuilder[T, C]) GetBuilder() *query.WhereBuilder[C] {
	return wb.builder
}
