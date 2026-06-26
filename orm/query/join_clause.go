package query

import qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"

// JoinClause exposes join-clause operations without leaking the internal SQL
// builder package into Goquent's public API.
type JoinClause struct {
	builder *qbapi.JoinClauseQueryBuilder
}

func newJoinClause(builder *qbapi.JoinClauseQueryBuilder) *JoinClause {
	return &JoinClause{builder: builder}
}

// On adds an AND join condition.
func (c *JoinClause) On(my, condition, target string) *JoinClause {
	c.builder.On(my, condition, target)
	return c
}

// OrOn adds an OR join condition.
func (c *JoinClause) OrOn(my, condition, target string) *JoinClause {
	c.builder.OrOn(my, condition, target)
	return c
}

// Where adds an AND predicate to the join clause.
func (c *JoinClause) Where(column, condition string, value any) *JoinClause {
	c.builder.Where(column, condition, value)
	return c
}

// OrWhere adds an OR predicate to the join clause.
func (c *JoinClause) OrWhere(column, condition string, value any) *JoinClause {
	c.builder.OrWhere(column, condition, value)
	return c
}
