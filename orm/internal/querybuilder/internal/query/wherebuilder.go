package query

import (
	"time"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sliceutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type WhereBuilder[T any] struct {
	dbBuilder interfaces.QueryBuilderStrategy
	query     *structs.Query
	parent    *T
}

func NewWhereBuilder[T any](strategy interfaces.QueryBuilderStrategy) *WhereBuilder[T] {
	return &WhereBuilder[T]{
		dbBuilder: strategy,
		query: &structs.Query{
			Conditions:      &[]structs.Where{},
			ConditionGroups: []structs.WhereGroup{},
		},
	}
}

func (b *WhereBuilder[T]) SetParent(parent *T) *T {
	b.parent = parent

	return b.parent
}

// Where adds a where clause with AND operator
func (b *WhereBuilder[T]) Where(column string, condition string, value ...interface{}) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Condition: condition,
		Value:     value,
		Operator:  consts.LogicalOperator_AND,
	})
	return b.parent
}

// OrWhere adds a where clause with OR operator
func (b *WhereBuilder[T]) OrWhere(column string, condition string, value ...interface{}) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Condition: condition,
		Value:     value,
		Operator:  consts.LogicalOperator_OR,
	})
	return b.parent
}

// WhereRaw adds a raw SQL condition with AND operator
func (b *WhereBuilder[T]) WhereRaw(raw string, values map[string]any) *T {
	return b.SafeWhereRaw(raw, values)
}

// OrWhereRaw adds a raw SQL condition with OR operator
func (b *WhereBuilder[T]) OrWhereRaw(raw string, values map[string]any) *T {
	return b.SafeOrWhereRaw(raw, values)
}

// SafeWhereRaw adds a raw where clause with AND operator while enforcing parameter usage.
// It ignores calls with nil value maps to prevent accidental injection.
func (b *WhereBuilder[T]) SafeWhereRaw(raw string, values map[string]any) *T {
	if values == nil {
		values = map[string]any{}
	}
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		ValueMap: values,
		Raw:      raw,
		Operator: consts.LogicalOperator_AND,
	})
	return b.parent
}

// SafeOrWhereRaw adds a raw where clause with OR operator while enforcing parameter usage.
func (b *WhereBuilder[T]) SafeOrWhereRaw(raw string, values map[string]any) *T {
	if values == nil {
		values = map[string]any{}
	}
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		ValueMap: values,
		Raw:      raw,
		Operator: consts.LogicalOperator_OR,
	})
	return b.parent
}

// WhereSubQuery adds a where clause with AND operator
func (b *WhereBuilder[T]) WhereSubQuery(column string, condition string, q *SelectBuilder) *T {
	return b.whereOrOrWhereQuery(column, condition, q, consts.LogicalOperator_AND)
}

// OrWhereSubQuery adds a where clause with OR operator
func (b *WhereBuilder[T]) OrWhereSubQuery(column string, condition string, q *SelectBuilder) *T {
	return b.whereOrOrWhereQuery(column, condition, q, consts.LogicalOperator_OR)
}

// whereOrOrWhereQuery adds a where clause with AND or OR operator
func (b *WhereBuilder[T]) whereOrOrWhereQuery(column string, condition string, q *SelectBuilder, operator int) *T {
	q.WhereBuilder.query.ConditionGroups = append(q.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
		Conditions:   *q.WhereBuilder.query.Conditions,
		IsDummyGroup: true,
	})

	*q.WhereBuilder.query.Conditions = []structs.Where{}

	sq := &structs.Query{
		ConditionGroups: q.WhereBuilder.query.ConditionGroups,
		Table:           structs.Table{Name: q.selectQuery.Table},
		Columns:         q.selectQuery.Columns,
		Joins:           q.JoinBuilder.Joins,
		Order:           q.OrderByBuilder.Order,
	}

	args := &structs.Where{
		Column:    column,
		Condition: condition,
		Query:     sq,
		Operator:  operator,
	}

	//_, value := b.BuildSq(sq)

	*b.query.Conditions = append(*b.query.Conditions, *args)
	//
	return b.parent
}

// WhereGroup adds a where group with AND operator
func (b *WhereBuilder[T]) WhereGroup(fn func(b *WhereBuilder[T])) *T {
	b.addWhereGroup(fn, consts.LogicalOperator_AND, false)

	return b.parent
}

// OrWhereGroup adds a where group with OR operator
func (b *WhereBuilder[T]) OrWhereGroup(fn func(b *WhereBuilder[T])) *T {
	b.addWhereGroup(fn, consts.LogicalOperator_OR, false)

	return b.parent
}

// WhereNot adds a not where group with AND operator
func (b *WhereBuilder[T]) WhereNot(fn func(b *WhereBuilder[T])) *T {
	b.addWhereGroup(fn, consts.LogicalOperator_AND, true)

	return b.parent
}

// OrWhereNot adds a not where group with OR operator
func (b *WhereBuilder[T]) OrWhereNot(fn func(b *WhereBuilder[T])) *T {
	b.addWhereGroup(fn, consts.LogicalOperator_OR, true)

	return b.parent
}

// addWhereGroup adds a where group with the specified operator
func (b *WhereBuilder[T]) addWhereGroup(fn func(b *WhereBuilder[T]), operator int, isNot bool) *T {
	if len(*b.query.Conditions) > 0 {
		b.query.ConditionGroups = append(b.query.ConditionGroups, structs.WhereGroup{
			Conditions:   *b.query.Conditions,
			Operator:     operator,
			IsDummyGroup: true,
			IsNot:        false,
		})
		*b.query.Conditions = []structs.Where{}
	}

	fn(b)

	b.query.ConditionGroups = append(b.query.ConditionGroups, structs.WhereGroup{
		Conditions: *b.query.Conditions,
		Operator:   operator,
		IsNot:      isNot,
	})
	*b.query.Conditions = []structs.Where{}

	return b.parent
}

// WhereAny adds where clauses with AND operator
func (b *WhereBuilder[T]) WhereAll(columns []string, condition string, value interface{}) *T {
	return b.addWhereConditions(columns, condition, value, consts.LogicalOperator_AND)
}

// OrWhereAny adds where clauses with OR operator
func (b *WhereBuilder[T]) WhereAny(columns []string, condition string, value interface{}) *T {
	return b.addWhereConditions(columns, condition, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereConditions(columns []string, condition string, value interface{}, operator int) *T {
	// already have conditions, add them to the query
	if len(*b.query.Conditions) > 0 {
		b.query.ConditionGroups = append(b.query.ConditionGroups, structs.WhereGroup{
			Conditions:   *b.query.Conditions,
			Operator:     operator,
			IsDummyGroup: true,
			IsNot:        false,
		})
		*b.query.Conditions = []structs.Where{}
	}

	conditions := []structs.Where{}
	for _, c := range columns {
		conditions = append(conditions, structs.Where{
			Column:    c,
			Condition: condition,
			Value:     []interface{}{value},
			Operator:  operator,
		})
	}

	b.query.ConditionGroups = append(b.query.ConditionGroups, structs.WhereGroup{
		Conditions: conditions,
		Operator:   consts.LogicalOperator_AND,
	})

	return b.parent
}

// WhereIn adds a where in clause with AND operator
func (b *WhereBuilder[T]) WhereIn(column string, values interface{}) *T {
	return b.addWhereIn(column, consts.LogicalOperator_AND, consts.Condition_IN, values)
}

// WhereNotIn adds a not where in clause with AND operator
func (b *WhereBuilder[T]) WhereNotIn(column string, values interface{}) *T {
	return b.addWhereIn(column, consts.LogicalOperator_AND, consts.Condition_NOT_IN, values)
}

// OrWhereIn adds a where in clause with OR operator
func (b *WhereBuilder[T]) OrWhereIn(column string, values interface{}) *T {
	return b.addWhereIn(column, consts.LogicalOperator_OR, consts.Condition_IN, values)
}

// OrWhereNotIn adds a not where in clause with OR operator
func (b *WhereBuilder[T]) OrWhereNotIn(column string, values interface{}) *T {
	return b.addWhereIn(column, consts.LogicalOperator_OR, consts.Condition_NOT_IN, values)
}

// addWhereIn adds a where in clause with the specified operator
func (b *WhereBuilder[T]) addWhereIn(column string, operator int, condition string, values interface{}) *T {

	switch casted := values.(type) {
	case []interface{}:
		*b.query.Conditions = append(*b.query.Conditions, structs.Where{
			Value:     casted,
			Operator:  operator,
			Column:    column,
			Condition: condition,
		})
	case []bool, []int, []int32, []int64, []uint, []uint32, []uint64,
		[]float32, []float64, []string, []time.Time:
		nValues := sliceutils.ToInterfaceSlice(casted)
		*b.query.Conditions = append(*b.query.Conditions, structs.Where{
			Value:     nValues,
			Operator:  operator,
			Column:    column,
			Condition: condition,
		})

	case *SelectBuilder:
		return b.addWhereInSubQuery(column, operator, condition, casted)
	default:
		//log.Default().Printf("type: %T\n", reflect.TypeOf(values))
		//log.Default().Println("values type: ", reflect.TypeOf(values).String())
		//log.Default().Println("values: ", values)
		//panic("Invalid type for values")
	}

	return b.parent
}

// WhereIn adds a where in clause with AND operator
func (b *WhereBuilder[T]) WhereInSubQuery(column string, q *SelectBuilder) *T {
	return b.addWhereInSubQuery(column, consts.LogicalOperator_AND, consts.Condition_IN, q)
}

// WhereNotIn adds a not where in clause with AND operator
func (b *WhereBuilder[T]) WhereNotInSubQuery(column string, q *SelectBuilder) *T {
	return b.addWhereInSubQuery(column, consts.LogicalOperator_AND, consts.Condition_NOT_IN, q)
}

// OrWhereIn adds a where in clause with OR operator
func (b *WhereBuilder[T]) OrWhereInSubQuery(column string, q *SelectBuilder) *T {
	return b.addWhereInSubQuery(column, consts.LogicalOperator_OR, consts.Condition_IN, q)
}

// OrWhereNotIn adds a not where in clause with OR operator
func (b *WhereBuilder[T]) OrWhereNotInSubQuery(column string, q *SelectBuilder) *T {
	return b.addWhereInSubQuery(column, consts.LogicalOperator_OR, consts.Condition_NOT_IN, q)
}

// addWhereIn adds a where in clause with the specified operator
func (b *WhereBuilder[T]) addWhereInSubQuery(column string, operator int, condition string, q *SelectBuilder) *T {
	q.WhereBuilder.query.ConditionGroups = append(q.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
		Conditions:   *q.WhereBuilder.query.Conditions,
		IsDummyGroup: true,
	})
	*q.WhereBuilder.query.Conditions = []structs.Where{}

	sq := &structs.Query{
		ConditionGroups: q.WhereBuilder.query.ConditionGroups,
		Table:           structs.Table{Name: q.selectQuery.Table},
		Columns:         q.selectQuery.Columns,
		Joins:           q.JoinBuilder.Joins,
		Order:           q.OrderByBuilder.Order,
	}

	args := &structs.Where{
		Column:    column,
		Condition: condition,
		Query:     sq,
		Operator:  operator,
	}

	//_, value := b.BuildSq(sq)

	*b.query.Conditions = append(*b.query.Conditions, *args)
	//
	return b.parent
}

func (b *WhereBuilder[T]) WhereNull(column string) *T {
	return b.addWhereNull(column, consts.LogicalOperator_AND, consts.Condition_IS_NULL)
}

func (b *WhereBuilder[T]) WhereNotNull(column string) *T {
	return b.addWhereNull(column, consts.LogicalOperator_AND, consts.Condition_IS_NOT_NULL)
}

func (b *WhereBuilder[T]) OrWhereNull(column string) *T {
	return b.addWhereNull(column, consts.LogicalOperator_OR, consts.Condition_IS_NULL)
}

func (b *WhereBuilder[T]) OrWhereNotNull(column string) *T {
	return b.addWhereNull(column, consts.LogicalOperator_OR, consts.Condition_IS_NOT_NULL)
}

func (b *WhereBuilder[T]) addWhereNull(column string, operator int, condition string) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Condition: condition,
		Operator:  operator,
		Value:     nil,
	})

	return b.parent
}

// WhereColumn adds a where column condition
func (b *WhereBuilder[T]) WhereColumn(allColumns []string, column string, condition string, valueColumn string) *T {
	return b.addWhereCondition(allColumns, column, condition, valueColumn, consts.LogicalOperator_AND)
}

// OrWhereColumn adds a where column condition
func (b *WhereBuilder[T]) OrWhereColumn(allColumns []string, column string, condition string, valueColumn string) *T {
	return b.addWhereCondition(allColumns, column, condition, valueColumn, consts.LogicalOperator_OR)
}

// addWhereCondition adds a where condition with the specified operator
func (b *WhereBuilder[T]) addWhereCondition(allColumns []string, column string, condition string, valueColumn string, operator int) *T {
	if !sliceutils.Contains(allColumns, column) {
		return b.parent
	}

	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:      column,
		Condition:   condition,
		ValueColumn: valueColumn,
		Operator:    operator,
	})

	return b.parent
}

// WhereColumns adds a where columns condition
func (b *WhereBuilder[T]) WhereColumns(allColumns []string, columns [][]string) *T {
	return b.addWhereColumns(allColumns, columns, consts.LogicalOperator_AND)
}

// OrWhereColumns adds a where columns condition
func (b *WhereBuilder[T]) OrWhereColumns(allColumns []string, columns [][]string) *T {
	return b.addWhereColumns(allColumns, columns, consts.LogicalOperator_OR)
}

// addWhereColumns adds a where columns condition with the specified operator
func (b *WhereBuilder[T]) addWhereColumns(allColumns []string, columns [][]string, operator int) *T {
	for _, c := range columns {
		column := ""
		cond := ""
		valueColumn := ""
		if len(c) == 2 {
			column = c[0]
			cond = consts.Condition_EQUAL
			valueColumn = c[1]
		} else if len(c) == 3 {
			column = c[0]
			cond = c[1]
			valueColumn = c[2]
		} else {
			continue
		}
		b.addWhereCondition(allColumns, column, cond, valueColumn, operator)
	}

	return b.parent
}

// WhereBetween adds a where between clause with AND operator
func (b *WhereBuilder[T]) WhereBetween(column string, from interface{}, to interface{}) *T {
	return b.addWhereBetween(column, from, to, consts.Condition_BETWEEN, consts.LogicalOperator_AND, false)
}

// WhereNotBetween adds a not where between clause with AND operator
func (b *WhereBuilder[T]) WhereNotBetween(column string, from interface{}, to interface{}) *T {
	return b.addWhereBetween(column, from, to, consts.Condition_NOT_BETWEEN, consts.LogicalOperator_AND, true)
}

// OrWhereBetween adds a where between clause with OR operator
func (b *WhereBuilder[T]) OrWhereBetween(column string, from interface{}, to interface{}) *T {
	return b.addWhereBetween(column, from, to, consts.Condition_BETWEEN, consts.LogicalOperator_OR, false)
}

// OrWhereNotBetween adds a not where between clause with OR operator
func (b *WhereBuilder[T]) OrWhereNotBetween(column string, from interface{}, to interface{}) *T {
	return b.addWhereBetween(column, from, to, consts.Condition_NOT_BETWEEN, consts.LogicalOperator_OR, true)
}

// addWhereBetween adds a where between clause with the specified operator
func (b *WhereBuilder[T]) addWhereBetween(column string, from interface{}, to interface{}, condition string, operator int, isNot bool) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Between:   &structs.WhereBetween{From: from, To: to, IsNot: isNot},
		Operator:  operator,
		Condition: condition,
	})

	return b.parent
}

// WhereBetweenColumns adds a where between columns clause with AND operator
func (b *WhereBuilder[T]) WhereBetweenColumns(allColumns []string, column string, min string, max string) *T {
	return b.addWhereBetweenColumns(allColumns, column, min, max, consts.Condition_BETWEEN, consts.LogicalOperator_AND, false)
}

// WhereNotBetweenColumns adds a not where between columns clause with AND operator
func (b *WhereBuilder[T]) WhereNotBetweenColumns(allColumns []string, column string, min string, max string) *T {
	return b.addWhereBetweenColumns(allColumns, column, min, max, consts.Condition_NOT_BETWEEN, consts.LogicalOperator_AND, true)
}

// OrWhereBetweenColumns adds a where between columns clause with OR operator
func (b *WhereBuilder[T]) OrWhereBetweenColumns(allColumns []string, column string, min string, max string) *T {
	return b.addWhereBetweenColumns(allColumns, column, min, max, consts.Condition_BETWEEN, consts.LogicalOperator_OR, false)
}

// OrWhereNotBetweenColumns adds a not where between columns clause with OR operator
func (b *WhereBuilder[T]) OrWhereNotBetweenColumns(allColumns []string, column string, min string, max string) *T {
	return b.addWhereBetweenColumns(allColumns, column, min, max, consts.Condition_NOT_BETWEEN, consts.LogicalOperator_OR, true)
}

func (b *WhereBuilder[T]) addWhereBetweenColumns(allColumns []string, column string, min string, max string, condition string, operator int, isNot bool) *T {
	if !sliceutils.Contains(allColumns, column) {
		return b.parent
	}

	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Between:   &structs.WhereBetween{From: min, To: max, IsColumn: true, IsNot: isNot},
		Operator:  operator,
		Condition: condition,
	})

	return b.parent
}

// WhereDate adds a where date clause with AND operator
func (b *WhereBuilder[T]) WhereExists(fn func(b *SelectBuilder)) *T {
	return b.addWhereExists(fn, consts.Condition_EXISTS, consts.LogicalOperator_AND, false)
}

// WhereNotExists adds a not where date clause with AND operator
func (b *WhereBuilder[T]) WhereNotExists(fn func(b *SelectBuilder)) *T {
	return b.addWhereExists(fn, consts.Condition_NOT_EXISTS, consts.LogicalOperator_AND, true)
}

// OrWhereDate adds a where date clause with OR operator
func (b *WhereBuilder[T]) OrWhereExists(fn func(b *SelectBuilder)) *T {
	return b.addWhereExists(fn, consts.Condition_EXISTS, consts.LogicalOperator_OR, false)
}

// OrWhereNotExists adds a not where date clause with OR operator
func (b *WhereBuilder[T]) OrWhereNotExists(fn func(b *SelectBuilder)) *T {
	return b.addWhereExists(fn, consts.Condition_NOT_EXISTS, consts.LogicalOperator_OR, true)
}

func (b *WhereBuilder[T]) addWhereExists(fn func(aq *SelectBuilder), condition string, operator int, isNot bool) *T {
	nb := NewSelectBuilder(b.dbBuilder)
	//nb.SetJoinBuilder(NewJoinBuilder[Builder](b.dbBuilder))
	//log.Default().Printf("nb: %+v\n", *&nb.selectQuery.Table)

	fn(nb)

	nb.WhereBuilder.query.ConditionGroups = append(nb.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
		Conditions:   *nb.WhereBuilder.query.Conditions,
		IsDummyGroup: true,
	})

	*nb.WhereBuilder.query.Conditions = []structs.Where{}

	sq := &structs.Query{
		ConditionGroups: nb.WhereBuilder.query.ConditionGroups,
		Table:           structs.Table{Name: nb.selectQuery.Table},
		Columns:         nb.selectQuery.Columns,
		Joins:           nb.JoinBuilder.Joins,
		Order:           nb.OrderByBuilder.Order,
	}

	args := &structs.Where{
		Column:    "",
		Condition: condition,
		Query:     nil,
		Operator:  operator,
		Exists:    &structs.Exists{IsNot: isNot, Query: sq},
	}

	//_, value := b.BuildSq(sq)

	*b.query.Conditions = append(*b.query.Conditions, *args)
	//
	return b.parent
}

// WhereExistsQuery adds a where exists query with AND operator
func (b *WhereBuilder[T]) WhereExistsQuery(q *SelectBuilder) *T {
	return b.addWhereExistsQuery(q, consts.Condition_EXISTS, consts.LogicalOperator_AND, false)
}

// WhereNotExistsQuery adds a not where exists query with AND operator
func (b *WhereBuilder[T]) WhereNotExistsQuery(q *SelectBuilder) *T {
	return b.addWhereExistsQuery(q, consts.Condition_NOT_EXISTS, consts.LogicalOperator_AND, true)
}

// OrWhereExistsQuery adds a where exists query with OR operator
func (b *WhereBuilder[T]) OrWhereExistsQuery(q *SelectBuilder) *T {
	return b.addWhereExistsQuery(q, consts.Condition_EXISTS, consts.LogicalOperator_OR, false)
}

// OrWhereNotExistsQuery adds a not where exists query with OR operator
func (b *WhereBuilder[T]) OrWhereNotExistsQuery(q *SelectBuilder) *T {
	return b.addWhereExistsQuery(q, consts.Condition_NOT_EXISTS, consts.LogicalOperator_OR, true)
}

func (b *WhereBuilder[T]) addWhereExistsQuery(q *SelectBuilder, condition string, operator int, isNot bool) *T {
	q.WhereBuilder.query.ConditionGroups = append(q.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
		Conditions:   *q.WhereBuilder.query.Conditions,
		IsDummyGroup: true,
	})

	*q.WhereBuilder.query.Conditions = []structs.Where{}

	sq := &structs.Query{
		ConditionGroups: q.WhereBuilder.query.ConditionGroups,
		Table:           structs.Table{Name: q.selectQuery.Table},
		Columns:         q.selectQuery.Columns,
		Joins:           q.JoinBuilder.Joins,
		Order:           q.OrderByBuilder.Order,
	}

	args := &structs.Where{
		Column:    "",
		Condition: condition,
		Query:     nil,
		Operator:  operator,
		Exists:    &structs.Exists{IsNot: isNot, Query: sq},
	}

	//

	*b.query.Conditions = append(*b.query.Conditions, *args)
	//b.whereValues = append(b.whereValues, value...)
	return b.parent
}

// WhereFullText adds a where full text clause with AND operator
func (b *WhereBuilder[T]) WhereFullText(columns []string, search string, options map[string]interface{}) *T {
	return b.addWhereFullText(columns, search, options, consts.LogicalOperator_AND, false)
}

// OrWhereFullText adds a where full text clause with OR operator
func (b *WhereBuilder[T]) OrWhereFullText(columns []string, search string, options map[string]interface{}) *T {
	return b.addWhereFullText(columns, search, options, consts.LogicalOperator_OR, false)
}

func (b *WhereBuilder[T]) addWhereFullText(columns []string, search string, options map[string]interface{}, operator int, isNot bool) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		FullText: &structs.FullText{Columns: columns, Search: search, Options: options, IsNot: isNot},
		Operator: operator,
	})

	return b.parent
}

// WhereJsonContains adds a where json contains clause with AND operator
func (b *WhereBuilder[T]) WhereJsonContains(column string, value interface{}) *T {
	return b.addWhereJsonContains(column, value, consts.LogicalOperator_AND)
}

// OrWhereJsonContains adds a where json contains clause with OR operator
func (b *WhereBuilder[T]) OrWhereJsonContains(column string, value interface{}) *T {
	return b.addWhereJsonContains(column, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereJsonContains(column string, value interface{}, operator int) *T {
	var values []interface{}
	switch v := value.(type) {
	case []interface{}:
		values = v
	case []string, []int, []int32, []int64, []uint, []uint32, []uint64, []float32, []float64, []bool:
		values = sliceutils.ToInterfaceSlice(v)
	default:
		values = []interface{}{value}
	}

	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:       column,
		JsonContains: &structs.JsonContains{Values: values},
		Operator:     operator,
	})

	return b.parent
}

// WhereJsonLength adds a where json length clause with AND operator
func (b *WhereBuilder[T]) WhereJsonLength(column string, args ...interface{}) *T {
	return b.addWhereJsonLength(column, args, consts.LogicalOperator_AND)
}

// OrWhereJsonLength adds a where json length clause with OR operator
func (b *WhereBuilder[T]) OrWhereJsonLength(column string, args ...interface{}) *T {
	return b.addWhereJsonLength(column, args, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereJsonLength(column string, args []interface{}, operator int) *T {
	if len(args) == 0 {
		return b.parent
	}
	op := "="
	val := args[0]
	if len(args) == 2 {
		if s, ok := args[0].(string); ok {
			op = s
			val = args[1]
		}
	}

	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:     column,
		JsonLength: &structs.JsonLength{Operator: op, Value: val},
		Operator:   operator,
	})

	return b.parent
}

// WhereDate adds a where date clause with AND operator
func (b *WhereBuilder[T]) WhereDate(column string, condition string, value string) *T {
	return b.addWhereDate(column, condition, value, consts.LogicalOperator_AND)
}

// OrWhereDate adds a where date clause with OR operator
func (b *WhereBuilder[T]) OrWhereDate(column string, condition string, value string) *T {
	return b.addWhereDate(column, condition, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereDate(column string, condition string, value string, operator int) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Function:  "DATE",
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  operator,
	})

	return b.parent
}

// WhereMonth adds a where month clause with AND operator
func (b *WhereBuilder[T]) WhereMonth(column string, condition string, value string) *T {
	return b.addWhereMonth(column, condition, value, consts.LogicalOperator_AND)
}

// OrWhereMonth adds a where month clause with OR operator
func (b *WhereBuilder[T]) OrWhereMonth(column string, condition string, value string) *T {
	return b.addWhereMonth(column, condition, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereMonth(column string, condition string, value string, operator int) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Function:  "MONTH",
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  operator,
	})

	return b.parent
}

// WhereDay adds a where day clause with AND operator
func (b *WhereBuilder[T]) WhereDay(column string, condition string, value string) *T {
	return b.addWhereDay(column, condition, value, consts.LogicalOperator_AND)
}

// OrWhereDay adds a where day clause with OR operator
func (b *WhereBuilder[T]) OrWhereDay(column string, condition string, value string) *T {
	return b.addWhereDay(column, condition, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereDay(column string, condition string, value string, operator int) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Function:  "DAY",
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  operator,
	})

	return b.parent
}

// WhereYear adds a where year clause with AND operator
func (b *WhereBuilder[T]) WhereYear(column string, condition string, value string) *T {
	return b.addWhereYear(column, condition, value, consts.LogicalOperator_AND)
}

// OrWhereYear adds a where year clause with OR operator
func (b *WhereBuilder[T]) OrWhereYear(column string, condition string, value string) *T {
	return b.addWhereYear(column, condition, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereYear(column string, condition string, value string, operator int) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Function:  "YEAR",
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  operator,
	})

	return b.parent
}

// WhereTime adds a where time clause with AND operator
func (b *WhereBuilder[T]) WhereTime(column string, condition string, value string) *T {
	return b.addWhereTime(column, condition, value, consts.LogicalOperator_AND)
}

// OrWhereTime adds a where time clause with OR operator
func (b *WhereBuilder[T]) OrWhereTime(column string, condition string, value string) *T {
	return b.addWhereTime(column, condition, value, consts.LogicalOperator_OR)
}

func (b *WhereBuilder[T]) addWhereTime(column string, condition string, value string, operator int) *T {
	*b.query.Conditions = append(*b.query.Conditions, structs.Where{
		Column:    column,
		Function:  "TIME",
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  operator,
	})

	return b.parent
}

func (b *WhereBuilder[T]) GetQuery() *structs.Query {
	return b.query
}
