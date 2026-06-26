package postgres

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/base"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type PostgreSQLQueryBuilder struct {
	base.BaseQueryBuilder
	base.DeleteBaseBuilder
	base.InsertBaseBuilder
	base.UpdateBaseBuilder

	WherePostgreSQLBuilder

	util interfaces.SQLUtils
}

func NewPostgreSQLQueryBuilder() *PostgreSQLQueryBuilder {
	return newPostgreSQLQueryBuilderWithUtil(NewSQLUtils())
}

func newPostgreSQLQueryBuilderWithUtil(u interfaces.SQLUtils) *PostgreSQLQueryBuilder {
	queryBuilder := &PostgreSQLQueryBuilder{}
	queryBuilder.util = u
	queryBuilder.SelectBaseBuilder = *base.NewSelectBaseBuilder(u, &[]string{})
	queryBuilder.JoinBaseBuilder = *base.NewJoinBaseBuilder(u, &structs.Joins{})
	queryBuilder.FromBaseBuilder = *base.NewFromBaseBuilder(u)
	queryBuilder.GroupByBaseBuilder = *base.NewGroupByBaseBuilder(u)
	queryBuilder.OrderByBaseBuilder = *base.NewOrderByBaseBuilder(u, &[]structs.Order{})
	queryBuilder.DeleteBaseBuilder = *base.NewDeleteBaseBuilder(u, &structs.DeleteQuery{})
	queryBuilder.InsertBaseBuilder = *base.NewInsertBaseBuilder(u, &structs.InsertQuery{})
	queryBuilder.UpdateBaseBuilder = *base.NewUpdateBaseBuilder(u, &structs.UpdateQuery{})
	queryBuilder.WherePostgreSQLBuilder = *NewWherePostgreSQLBuilder(u, []structs.WhereGroup{})
	return queryBuilder
}

func (m PostgreSQLQueryBuilder) ResetPlaceholderCounter() {
	if resetter, ok := m.util.(*SQLUtils); ok {
		resetter.ResetPlaceholderCounter()
	}
}

func (m PostgreSQLQueryBuilder) InsertIgnore(q *structs.InsertQuery) (string, []interface{}, error) {
	return m.InsertBaseBuilder.InsertIgnore(q)
}

func (m PostgreSQLQueryBuilder) Upsert(q *structs.InsertQuery) (string, []interface{}, error) {
	return m.InsertBaseBuilder.Upsert(q)
}

// Build builds the query.
func (m PostgreSQLQueryBuilder) Build(sb *[]byte, q *structs.Query, number int, unions *[]structs.Union) ([]interface{}, error) {
	// SELECT
	*sb = append(*sb, "SELECT "...)
	colValues, err := m.Select(sb, q.Columns, q.Table.Name, q.Joins)
	if err != nil {
		return nil, err
	}

	*sb = append(*sb, " "...)
	m.From(sb, q.Table.Name)
	values := colValues

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

	//query := sb.String()
	//sb.Reset()

	return values, nil
}

func (m PostgreSQLQueryBuilder) Where(sb *[]byte, conditionGroups []structs.WhereGroup) ([]interface{}, error) {
	return m.WherePostgreSQLBuilder.Where(sb, conditionGroups)
}
