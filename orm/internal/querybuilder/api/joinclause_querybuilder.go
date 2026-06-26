package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

type JoinClauseQueryBuilder struct {
	builder *query.JoinClauseBuilder
}

func NewJoinClauseQueryBuilder() *JoinClauseQueryBuilder {
	return &JoinClauseQueryBuilder{
		builder: query.NewJoinClauseBuilder(),
	}
}

func (qb *JoinClauseQueryBuilder) On(my, condition, target string) *JoinClauseQueryBuilder {
	qb.builder.On(my, condition, target)
	return qb
}

func (qb *JoinClauseQueryBuilder) OrOn(my, condition, target string) *JoinClauseQueryBuilder {
	qb.builder.OrOn(my, condition, target)
	return qb
}

func (qb *JoinClauseQueryBuilder) Where(column, condition string, value interface{}) *JoinClauseQueryBuilder {
	qb.builder.Where(column, condition, value)
	return qb
}

func (qb *JoinClauseQueryBuilder) OrWhere(column, condition string, value interface{}) *JoinClauseQueryBuilder {
	qb.builder.OrWhere(column, condition, value)
	return qb
}
