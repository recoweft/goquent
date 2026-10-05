package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

type UpdateBuilder struct {
	BuiltStatement *structs.UpdateQuery
	dbBuilder      interfaces.QueryBuilderStrategy
	query          *structs.UpdateQuery
	OrderByBuilder[UpdateBuilder]
	JoinBuilder[UpdateBuilder]
	WhereBuilder[UpdateBuilder]
}

func NewUpdateBuilder(strategy interfaces.QueryBuilderStrategy) *UpdateBuilder {
	ub := &UpdateBuilder{
		dbBuilder: strategy,
		query: &structs.UpdateQuery{
			Query: &structs.Query{},
		},
	}

	whereBuilder := NewWhereBuilder[UpdateBuilder](strategy)
	whereBuilder.SetParent(ub)
	ub.WhereBuilder = *whereBuilder

	joinBuilder := NewJoinBuilder[UpdateBuilder](strategy)
	joinBuilder.SetParent(ub)
	ub.JoinBuilder = *joinBuilder

	orderByBuilder := NewOrderByBuilder[UpdateBuilder](strategy)
	orderByBuilder.SetParent(ub)
	ub.OrderByBuilder = *orderByBuilder

	return ub
}

func (b *UpdateBuilder) Table(table string) *UpdateBuilder {
	b.query.Table = table
	b.JoinBuilder.Table.Name = table
	return b
}

func (b *UpdateBuilder) Update(data map[string]interface{}) *UpdateBuilder {
	b.query.Values = data

	return b
}

func (u *UpdateBuilder) Build() (string, []interface{}, error) {
	u.dbBuilder.ResetPlaceholderCounter()

	// If there are conditions, add them to the query
	if len(*u.WhereBuilder.query.Conditions) > 0 {
		u.WhereBuilder.query.ConditionGroups = append(u.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
			Conditions:   *u.WhereBuilder.query.Conditions,
			Operator:     consts.LogicalOperator_AND,
			IsDummyGroup: true,
		})
		u.WhereBuilder.query.Conditions = &[]structs.Where{}
	}

	u.query.Query.PredicateError = u.WhereBuilder.query.PredicateError
	u.query.Query.Conditions = u.WhereBuilder.query.Conditions
	u.query.Query.ConditionGroups = u.WhereBuilder.query.ConditionGroups
	u.query.Query.Joins = u.JoinBuilder.Joins
	u.query.Query.Order = u.OrderByBuilder.Order

	frozen := *u.query
	frozen.Values = make(map[string]any, len(u.query.Values))
	copier := valuecopy.New()
	for k, v := range u.query.Values {
		frozen.Values[k], _ = copier.Copy(v)
	}
	frozen.Query = structs.CloneQuery(u.query.Query)
	query, values, err := u.dbBuilder.BuildUpdate(&frozen)
	u.BuiltStatement = &frozen
	return query, values, err
}

func (b *UpdateBuilder) OrderBy(column string, direction string) *UpdateBuilder {
	b.OrderByBuilder.OrderBy(column, direction)
	return b
}

func (b *UpdateBuilder) OrderByRaw(raw string) *UpdateBuilder {
	b.OrderByBuilder.OrderByRaw(raw)
	return b
}

func (b *UpdateBuilder) ReOrder() *UpdateBuilder {
	b.OrderByBuilder.ReOrder()
	return b
}

func (b *UpdateBuilder) GetQuery() *structs.UpdateQuery {
	return b.query
}
