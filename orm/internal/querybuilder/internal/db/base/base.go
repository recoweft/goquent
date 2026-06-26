package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type BaseQueryBuilder struct {
	UnionBaseBuilder
	SelectBaseBuilder
	FromBaseBuilder
	WhereBaseBuilder
	JoinBaseBuilder
	OrderByBaseBuilder
	GroupByBaseBuilder
	LimitBaseBuilder
	OffsetBaseBuilder
	InsertBaseBuilder
	UpdateBaseBuilder
	DeleteBaseBuilder

	util interfaces.SQLUtils
}

func NewBaseQueryBuilder() *BaseQueryBuilder {
	return newBaseQueryBuilderWithUtil(NewSQLUtils())
}

func newBaseQueryBuilderWithUtil(u interfaces.SQLUtils) *BaseQueryBuilder {
	queryBuilder := &BaseQueryBuilder{}
	queryBuilder.util = u
	queryBuilder.SelectBaseBuilder = *NewSelectBaseBuilder(u, &[]string{})
	queryBuilder.FromBaseBuilder = *NewFromBaseBuilder(u)
	queryBuilder.JoinBaseBuilder = *NewJoinBaseBuilder(u, &structs.Joins{})
	queryBuilder.WhereBaseBuilder = *NewWhereBaseBuilder(u, []structs.WhereGroup{})
	queryBuilder.OrderByBaseBuilder = *NewOrderByBaseBuilder(u, &[]structs.Order{})
	queryBuilder.GroupByBaseBuilder = *NewGroupByBaseBuilder(u)
	queryBuilder.LimitBaseBuilder = *NewLimitBaseBuilder()
	queryBuilder.OffsetBaseBuilder = *NewOffsetBaseBuilder()
	queryBuilder.InsertBaseBuilder = *NewInsertBaseBuilder(u, &structs.InsertQuery{})
	queryBuilder.UpdateBaseBuilder = *NewUpdateBaseBuilder(u, &structs.UpdateQuery{})
	queryBuilder.DeleteBaseBuilder = *NewDeleteBaseBuilder(u, &structs.DeleteQuery{})
	return queryBuilder
}

func (BaseQueryBuilder) ResetPlaceholderCounter() {
}

// Lock returns the lock statement.
func (BaseQueryBuilder) Lock(sb *[]byte, lock *structs.Lock) {
	if lock == nil || lock.LockType == "" {
		return
	}

	*sb = append(*sb, " "...)
	*sb = append(*sb, lock.LockType...)
}

// Build builds the query.
func (m BaseQueryBuilder) Build(sb *[]byte, q *structs.Query, number int, unions *[]structs.Union) ([]interface{}, error) {
	values := make([]interface{}, 0)

	// SELECT
	*sb = append(*sb, "SELECT "...)
	colValues, err := m.Select(sb, q.Columns, q.Table.Name, q.Joins)
	if err != nil {
		return nil, err
	}

	// FROM
	*sb = append(*sb, " "...)
	m.From(sb, q.Table.Name)
	values = append(values, colValues...)

	// JOIN
	joinValues := m.Join(sb, q.Joins)
	values = append(values, joinValues...)

	// WHERE
	whereValues, err := m.Where(sb, q.ConditionGroups)
	if err != nil {
		return []interface{}{}, err
	}
	values = append(values, whereValues...)

	// GROUP BY / HAVING
	groupByValues := m.GroupBy(sb, q.Group)
	values = append(values, groupByValues...)

	// ORDER BY
	m.OrderBy(sb, q.Order)

	// LIMIT
	m.Limit(sb, q.Limit)

	// OFFSET
	m.Offset(sb, q.Offset)

	// LOCK
	m.Lock(sb, q.Lock)

	// UNION
	m.Union(sb, unions, number)

	return values, nil
}
