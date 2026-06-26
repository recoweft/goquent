package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

type UpdateQueryBuilder struct {
	WhereQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder]
	JoinQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder]
	OrderByQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder]
	builder *query.UpdateBuilder
	QueryBuilderStrategy[UpdateQueryBuilder, query.UpdateBuilder]
}

func NewUpdateQueryBuilder(strategy interfaces.QueryBuilderStrategy) *UpdateQueryBuilder {
	ub := &UpdateQueryBuilder{
		builder: query.NewUpdateBuilder(strategy),
	}

	whereQueryBuilder := NewWhereQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder](strategy)
	whereQueryBuilder.SetParent(&ub)
	ub.WhereQueryBuilder = *whereQueryBuilder

	joinQueryBuilder := NewJoinQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder](strategy)
	joinQueryBuilder.SetParent(&ub)
	ub.JoinQueryBuilder = *joinQueryBuilder

	orderByQueryBuilder := NewOrderByQueryBuilder[*UpdateQueryBuilder, query.UpdateBuilder](strategy)
	orderByQueryBuilder.SetParent(&ub)
	ub.OrderByQueryBuilder = *orderByQueryBuilder

	return ub
}

// Update
func (ub *UpdateQueryBuilder) Update(data map[string]interface{}) *UpdateQueryBuilder {
	ub.builder.Update(data)

	return ub
}

// Table
func (ub *UpdateQueryBuilder) Table(table string) *UpdateQueryBuilder {
	ub.builder.Table(table)
	return ub
}

// Build
func (ub *UpdateQueryBuilder) Build() (string, []interface{}, error) {
	return ub.builder.Build()
}

func (ub *UpdateQueryBuilder) Dump() (string, []interface{}, error) {
	b := query.NewDebugBuilder[*query.UpdateBuilder, UpdateQueryBuilder](ub.builder)

	return b.Dump()
}

func (ub *UpdateQueryBuilder) RawSql() (string, error) {
	b := query.NewDebugBuilder[*query.UpdateBuilder, UpdateQueryBuilder](ub.builder)

	return b.RawSql()
}

func (qb *UpdateQueryBuilder) GetQueryBuilder() *UpdateQueryBuilder {
	return qb
}

func (qb *UpdateQueryBuilder) GetWhereBuilder() *query.WhereBuilder[query.UpdateBuilder] {
	return &qb.builder.WhereBuilder
}

func (qb *UpdateQueryBuilder) GetJoinBuilder() *query.JoinBuilder[query.UpdateBuilder] {
	return &qb.builder.JoinBuilder
}

func (qb *UpdateQueryBuilder) GetOrderByBuilder() *query.OrderByBuilder[query.UpdateBuilder] {
	return &qb.builder.OrderByBuilder
}
