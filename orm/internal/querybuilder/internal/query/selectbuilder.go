package query

import (
	"sync"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/memutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

type SelectBuilder struct {
	BuiltQuery  *structs.Query
	dbBuilder   interfaces.QueryBuilderStrategy
	query       *structs.Query
	selectQuery *structs.SelectQuery
	*WhereBuilder[SelectBuilder]
	*JoinBuilder[SelectBuilder]
	*OrderByBuilder[SelectBuilder]
	BaseBuilder
}

func NewSelectBuilder(dbBuilder interfaces.QueryBuilderStrategy) *SelectBuilder {
	b := &SelectBuilder{
		dbBuilder: dbBuilder,
		query: &structs.Query{
			Table:           structs.Table{},
			Columns:         &[]structs.Column{},
			ConditionGroups: []structs.WhereGroup{},
			Joins:           &structs.Joins{},
			Order:           &[]structs.Order{},
			Group:           &structs.GroupBy{},
			Limit:           structs.Limit{},
			Offset:          structs.Offset{},
			Lock:            &structs.Lock{},
		},
		selectQuery: &structs.SelectQuery{
			Table:   "",
			Columns: &[]structs.Column{},
			Limit:   structs.Limit{},
			Group:   &structs.GroupBy{},
			Offset:  structs.Offset{},
			Lock:    &structs.Lock{},
			Union:   &[]structs.Union{},
		},
		//joinBuilder:    NewJoinBuilder(dbBuilder),
	}

	whereBuilder := NewWhereBuilder[SelectBuilder](dbBuilder)
	whereBuilder.SetParent(b)
	b.WhereBuilder = whereBuilder

	joinBuilder := NewJoinBuilder[SelectBuilder](dbBuilder)
	joinBuilder.SetParent(b)
	b.JoinBuilder = joinBuilder

	orderByBuilder := NewOrderByBuilder[SelectBuilder](dbBuilder)
	orderByBuilder.SetParent(b)
	b.OrderByBuilder = orderByBuilder

	return b
}

var bytebufPool = sync.Pool{
	New: func() interface{} {
		s := make([]byte, 0, consts.StringBuffer_Short_Query_Grow)
		return &s
	},
}

var interfaceSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]interface{}, 0)
		return &s
	},
}

func (b *SelectBuilder) Table(table string) *SelectBuilder {
	b.selectQuery.Table = table
	return b
}

func (b *SelectBuilder) Select(columns ...string) *SelectBuilder {
	for _, column := range columns {
		*b.selectQuery.Columns = append(*b.selectQuery.Columns, structs.Column{Name: column})
	}
	return b
}

func (b *SelectBuilder) SelectRaw(raw string, value ...interface{}) *SelectBuilder {
	*b.selectQuery.Columns = append(*b.selectQuery.Columns, structs.Column{Raw: raw, Values: value})
	return b
}

// Count adds a COUNT aggregate function to the query.
func (b *SelectBuilder) Count(columns ...string) *SelectBuilder {
	if len(columns) == 0 {
		columns = append(columns, "*")
	}

	for i, c := range *b.selectQuery.Columns {
		for _, col := range columns {
			if c.Name == col {
				(*b.selectQuery.Columns)[i].Count = true
			}
		}
	}

out:
	for _, column := range columns {
		for _, c := range *b.selectQuery.Columns {
			if c.Count {
				continue out
			}
		}

		*b.selectQuery.Columns = append(*b.selectQuery.Columns, structs.Column{
			Name:  column,
			Count: true,
		})
	}

	return b
}

func (b *SelectBuilder) aggregate(column string, aggregateFunc string) *SelectBuilder {
	*b.selectQuery.Columns = append(*b.selectQuery.Columns, structs.Column{
		Name:     column,
		Function: aggregateFunc,
	})
	return b
}

// Max adds a MAX aggregate function to the query.
func (b *SelectBuilder) Max(column string) *SelectBuilder {
	return b.aggregate(column, "MAX")
}

// Min adds a MIN aggregate function to the query.
func (b *SelectBuilder) Min(column string) *SelectBuilder {
	return b.aggregate(column, "MIN")
}

// Sum adds a SUM aggregate function to the query.
func (b *SelectBuilder) Sum(column string) *SelectBuilder {
	return b.aggregate(column, "SUM")
}

// Avg adds an AVG aggregate function to the query.
func (b *SelectBuilder) Avg(column string) *SelectBuilder {
	return b.aggregate(column, "AVG")
}

func (b *SelectBuilder) Distinct(column ...string) *SelectBuilder {
	for i, c := range *b.selectQuery.Columns {
		for _, col := range column {
			if c.Name == col {
				(*b.selectQuery.Columns)[i].Distinct = true
			}
		}
	}

out:
	for _, c := range column {
		for _, c := range *b.selectQuery.Columns {
			if c.Count {
				continue out
			}
		}
		*b.selectQuery.Columns = append(*b.selectQuery.Columns, structs.Column{
			Name:     c,
			Distinct: true,
		})
	}

	return b
}

func (b *SelectBuilder) Union(sb *SelectBuilder) *SelectBuilder {
	*b.selectQuery.Union = append(*b.selectQuery.Union, structs.Union{
		Query: sb.GetQuery(),
		IsAll: false,
	})

	return b
}

func (b *SelectBuilder) UnionAll(sb *SelectBuilder) *SelectBuilder {
	*b.selectQuery.Union = append(*b.selectQuery.Union, structs.Union{
		Query: sb.GetQuery(),
		IsAll: true,
	})

	return b
}

// GroupBy adds a GROUP BY clause.
func (b *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	*b.selectQuery.Group = structs.GroupBy{
		Columns: columns,
		Having:  &[]structs.Having{},
	}
	return b
}

// Having adds a HAVING clause with an AND operator.
func (b *SelectBuilder) Having(column string, condition string, value interface{}) *SelectBuilder {
	*b.selectQuery.Group.Having = append(*b.selectQuery.Group.Having, structs.Having{
		Column:    column,
		Condition: condition,
		Value:     value,
		Operator:  consts.LogicalOperator_AND,
	})
	return b
}

// HavingRaw adds a raw HAVING clause with an AND operator.
func (b *SelectBuilder) HavingRaw(raw string) *SelectBuilder {
	*b.selectQuery.Group.Having = append(*b.selectQuery.Group.Having, structs.Having{
		Raw:      raw,
		Operator: consts.LogicalOperator_AND,
	})
	return b
}

// OrHaving adds a HAVING clause with an OR operator.
func (b *SelectBuilder) OrHaving(column string, condition string, value interface{}) *SelectBuilder {
	*b.selectQuery.Group.Having = append(*b.selectQuery.Group.Having, structs.Having{
		Column:    column,
		Condition: condition,
		Value:     value,
		Operator:  consts.LogicalOperator_OR,
	})
	return b
}

// OrHavingRaw adds a raw HAVING clause with an OR operator.
func (b *SelectBuilder) OrHavingRaw(raw string) *SelectBuilder {
	*b.selectQuery.Group.Having = append(*b.selectQuery.Group.Having, structs.Having{
		Raw:      raw,
		Operator: consts.LogicalOperator_OR,
	})
	return b
}

func (b *SelectBuilder) Limit(limit int64) *SelectBuilder {
	b.selectQuery.Limit.Limit = limit
	return b
}

func (b *SelectBuilder) Offset(offset int64) *SelectBuilder {
	b.selectQuery.Offset.Offset = offset
	return b
}

func (b *SelectBuilder) SharedLock() *SelectBuilder {
	b.selectQuery.Lock = &structs.Lock{
		LockType: consts.Lock_SHARE_MODE,
	}
	return b
}

func (b *SelectBuilder) LockForUpdate() *SelectBuilder {
	b.selectQuery.Lock = &structs.Lock{
		LockType: consts.Lock_FOR_UPDATE,
	}
	return b
}

// Build generates the SQL query string and parameter values based on the query builder's current state.
// It returns the generated query string and a slice of parameter values.
func (b *SelectBuilder) Build() (string, []interface{}, error) {
	b.dbBuilder.ResetPlaceholderCounter()
	b.buildQuery()
	if err := structs.ValidateQuery(b.query); err != nil {
		return "", nil, err
	}
	queries := make([]structs.Union, 0, len(*b.selectQuery.Union)+1)
	for _, u := range *b.selectQuery.Union {
		if err := structs.ValidateQuery(u.Query); err != nil {
			return "", nil, err
		}
		queries = append(queries, structs.Union{Query: structs.CloneQuery(u.Query), IsAll: u.IsAll})
	}
	frozen := structs.CloneQuery(b.query)
	queries = append(queries, structs.Union{Query: frozen})
	ptr := bytebufPool.Get().(*[]byte)
	sb := (*ptr)[:0]
	defer func() { memutils.ZeroBytes(sb); *ptr = sb[:0]; bytebufPool.Put(ptr) }()
	vp := interfaceSlicePool.Get().(*[]interface{})
	values := (*vp)[:0]
	defer func() { memutils.ZeroInterfaces(values); *vp = values[:0]; interfaceSlicePool.Put(vp) }()
	b.BuiltQuery = nil
	for i, u := range queries {
		v, err := b.dbBuilder.Build(&sb, u.Query, i, &queries)
		if err != nil {
			return "", nil, err
		}
		valuecopy.Shift(u.Query.WhereTree, len(values))
		valuecopy.Shift(u.Query.HavingTree, len(values))
		values = append(values, v...)
	}
	b.BuiltQuery = frozen
	return string(sb), valuecopy.Slice(values), nil
}

func (b *SelectBuilder) buildQuery() {
	// preprocess WHERE
	if len(*b.WhereBuilder.query.Conditions) > 0 {
		b.WhereBuilder.query.ConditionGroups = append(b.WhereBuilder.query.ConditionGroups,
			structs.WhereGroup{
				Conditions:   *b.WhereBuilder.query.Conditions,
				Operator:     consts.LogicalOperator_AND,
				IsDummyGroup: true,
			})
		b.WhereBuilder.query.Conditions = &[]structs.Where{}
	}

	// preprocess ORDER BY
	o := b.OrderByBuilder.Order

	b.query.Unions = *b.selectQuery.Union
	b.query.PredicateError = b.WhereBuilder.query.PredicateError
	b.query.Table = structs.Table{
		Name: b.selectQuery.Table,
	}
	b.query.Columns = b.selectQuery.Columns
	b.query.ConditionGroups = b.WhereBuilder.query.ConditionGroups
	b.query.Joins = b.JoinBuilder.Joins
	b.query.Order = o
	b.query.Group = b.selectQuery.Group
	b.query.Limit = b.selectQuery.Limit
	b.query.Offset = b.selectQuery.Offset
	b.query.Lock = b.selectQuery.Lock

}

func (b *SelectBuilder) GetQuery() *structs.Query {
	b.buildQuery()
	return b.query
}

func (b *SelectBuilder) GetStrategy() interfaces.QueryBuilderStrategy {
	return b.dbBuilder
}

func (b *SelectBuilder) GetWhereBuilder() *WhereBuilder[SelectBuilder] {
	return b.WhereBuilder
}

func (b *SelectBuilder) GetJoinBuilder() *JoinBuilder[SelectBuilder] {
	return b.JoinBuilder
}

func (b *SelectBuilder) GetOrderByBuilder() *OrderByBuilder[SelectBuilder] {
	return b.OrderByBuilder
}
