package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type DeleteBuilder struct {
	dbBuilder interfaces.QueryBuilderStrategy
	query     *structs.DeleteQuery
	WhereBuilder[DeleteBuilder]
	JoinBuilder[DeleteBuilder]
	OrderByBuilder[DeleteBuilder]
}

func NewDeleteBuilder(strategy interfaces.QueryBuilderStrategy) *DeleteBuilder {
	db := &DeleteBuilder{
		dbBuilder: strategy,
		query: &structs.DeleteQuery{
			Query: &structs.Query{},
		},
	}

	whereBuilder := NewWhereBuilder[DeleteBuilder](strategy)
	whereBuilder.SetParent(db)
	db.WhereBuilder = *whereBuilder

	joinBuilder := NewJoinBuilder[DeleteBuilder](strategy)
	joinBuilder.SetParent(db)
	db.JoinBuilder = *joinBuilder

	orderByBuilder := NewOrderByBuilder[DeleteBuilder](strategy)
	orderByBuilder.SetParent(db)
	db.OrderByBuilder = *orderByBuilder

	return db
}

func (b *DeleteBuilder) Table(table string) *DeleteBuilder {
	b.query.Table = table
	b.JoinBuilder.Table.Name = table
	return b
}

// Delete
func (b *DeleteBuilder) Delete() *DeleteBuilder {
	return b
}

func (d *DeleteBuilder) Build() (string, []interface{}, error) {
	d.dbBuilder.ResetPlaceholderCounter()

	// If there are conditions, add them to the query
	if len(*d.WhereBuilder.query.Conditions) > 0 {
		d.WhereBuilder.query.ConditionGroups = append(d.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
			Conditions:   *d.WhereBuilder.query.Conditions,
			Operator:     consts.LogicalOperator_AND,
			IsDummyGroup: true,
		})
		d.WhereBuilder.query.Conditions = &[]structs.Where{}
	}

	d.query.Query.Conditions = d.WhereBuilder.query.Conditions
	d.query.Query.ConditionGroups = d.WhereBuilder.query.ConditionGroups
	d.query.Query.Joins = d.JoinBuilder.Joins
	d.query.Query.Order = d.OrderByBuilder.Order

	query, values, err := d.dbBuilder.BuildDelete(d.query)
	return query, values, err
}

/*
func (b *DeleteBuilder) OrderBy(column string, direction string) *DeleteBuilder {
	b.orderByBuilder.OrderBy(column, direction)
	return b
}

func (b *DeleteBuilder) OrderByRaw(raw string) *DeleteBuilder {
	b.orderByBuilder.OrderByRaw(raw)
	return b
}

func (b *DeleteBuilder) ReOrder() *DeleteBuilder {
	b.orderByBuilder.ReOrder()
	return b
}
*/

func (b *DeleteBuilder) GetQuery() *structs.DeleteQuery {
	return b.query
}
