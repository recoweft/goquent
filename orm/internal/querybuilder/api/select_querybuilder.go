package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

type QueryBuilderStrategy[T, C any] interface {
	GetQueryBuilder() T
	GetWhereBuilder() *query.WhereBuilder[C]
	GetJoinBuilder() *query.JoinBuilder[C]
	GetOrderByBuilder() *query.OrderByBuilder[C]
}

type SelectQueryBuilder struct {
	WhereQueryBuilder[*SelectQueryBuilder, query.SelectBuilder]
	JoinQueryBuilder[*SelectQueryBuilder, query.SelectBuilder]
	OrderByQueryBuilder[*SelectQueryBuilder, query.SelectBuilder]
	builder *query.SelectBuilder
	Queries *[]structs.Query
	QueryBuilderStrategy[SelectQueryBuilder, query.SelectBuilder]
}

func NewSelectQueryBuilder(strategy interfaces.QueryBuilderStrategy) *SelectQueryBuilder {
	sb := &SelectQueryBuilder{
		//WhereQueryBuilder: *NewWhereQueryBuilder[SelectBuilder, query.Builder](strategy),
		builder: query.NewSelectBuilder(strategy),
	}
	sb.Queries = &[]structs.Query{}

	whereBuilder := NewWhereQueryBuilder[*SelectQueryBuilder, query.SelectBuilder](strategy)
	whereBuilder.SetParent(&sb)
	sb.WhereQueryBuilder = *whereBuilder

	joinBuilder := NewJoinQueryBuilder[*SelectQueryBuilder, query.SelectBuilder](strategy)
	joinBuilder.SetParent(&sb)
	sb.JoinQueryBuilder = *joinBuilder

	orderByBuilder := NewOrderByQueryBuilder[*SelectQueryBuilder, query.SelectBuilder](strategy)
	orderByBuilder.SetParent(&sb)
	sb.OrderByQueryBuilder = *orderByBuilder

	return sb
}

func (qb *SelectQueryBuilder) Table(table string) *SelectQueryBuilder {
	qb.builder.Table(table)
	return qb
}

func (qb *SelectQueryBuilder) Select(columns ...string) *SelectQueryBuilder {
	qb.builder.Select(columns...)
	return qb
}

func (qb *SelectQueryBuilder) SelectRaw(raw string, value ...interface{}) *SelectQueryBuilder {
	qb.builder.SelectRaw(raw, value...)
	return qb
}

func (qb *SelectQueryBuilder) Count(columns ...string) *SelectQueryBuilder {
	qb.builder.Count(columns...)
	return qb
}

func (qb *SelectQueryBuilder) Max(column string) *SelectQueryBuilder {
	qb.builder.Max(column)
	return qb
}

func (qb *SelectQueryBuilder) Min(column string) *SelectQueryBuilder {
	qb.builder.Min(column)
	return qb
}

func (qb *SelectQueryBuilder) Sum(column string) *SelectQueryBuilder {
	qb.builder.Sum(column)
	return qb
}

func (qb *SelectQueryBuilder) Avg(column string) *SelectQueryBuilder {
	qb.builder.Avg(column)
	return qb
}

func (qb *SelectQueryBuilder) Distinct(column ...string) *SelectQueryBuilder {
	qb.builder.Distinct(column...)
	return qb
}

func (qb *SelectQueryBuilder) Union(sb *SelectQueryBuilder) *SelectQueryBuilder {
	*qb.Queries = append(*qb.Queries, *sb.GetQuery())
	qb.builder.Union(sb.builder)
	return qb
}

func (qb *SelectQueryBuilder) UnionAll(sb *SelectQueryBuilder) *SelectQueryBuilder {
	*qb.Queries = append(*qb.Queries, *sb.GetQuery())
	qb.builder.UnionAll(sb.builder)
	return qb
}

func (qb *SelectQueryBuilder) GroupBy(columns ...string) *SelectQueryBuilder {
	qb.builder.GroupBy(columns...)
	return qb
}

func (qb *SelectQueryBuilder) Having(column, condition string, value interface{}) *SelectQueryBuilder {
	qb.builder.Having(column, condition, value)
	return qb
}

func (qb *SelectQueryBuilder) HavingRaw(raw string) *SelectQueryBuilder {
	qb.builder.HavingRaw(raw)
	return qb
}

func (qb *SelectQueryBuilder) OrHaving(column, condition string, value interface{}) *SelectQueryBuilder {
	qb.builder.OrHaving(column, condition, value)
	return qb
}

func (qb *SelectQueryBuilder) OrHavingRaw(raw string) *SelectQueryBuilder {
	qb.builder.OrHavingRaw(raw)
	return qb
}

func (qb *SelectQueryBuilder) Limit(limit int64) *SelectQueryBuilder {
	qb.builder.Limit(limit)
	return qb
}

func (qb *SelectQueryBuilder) Take(limit int64) *SelectQueryBuilder {
	qb.builder.Limit(limit)
	return qb
}

func (qb *SelectQueryBuilder) Offset(offset int64) *SelectQueryBuilder {
	qb.builder.Offset(offset)
	return qb
}

func (qb *SelectQueryBuilder) Skip(offset int64) *SelectQueryBuilder {
	qb.builder.Offset(offset)
	return qb
}

func (qb *SelectQueryBuilder) SharedLock() *SelectQueryBuilder {
	qb.builder.SharedLock()
	return qb
}

func (qb *SelectQueryBuilder) LockForUpdate() *SelectQueryBuilder {
	qb.builder.LockForUpdate()
	return qb
}

func (qb *SelectQueryBuilder) Build() (string, []interface{}, error) {
	return qb.builder.Build()
}

func (qb *SelectQueryBuilder) GetQuery() *structs.Query {
	return qb.builder.GetQuery()
}

func (qb *SelectQueryBuilder) Dump() (string, []interface{}, error) {
	b := query.NewDebugBuilder[*query.SelectBuilder, SelectQueryBuilder](qb.builder)

	return b.Dump()
}

func (qb *SelectQueryBuilder) RawSql() (string, error) {
	b := query.NewDebugBuilder[*query.SelectBuilder, SelectQueryBuilder](qb.builder)

	return b.RawSql()
}

func (qb *SelectQueryBuilder) GetQueryBuilder() *SelectQueryBuilder {
	return qb
}

func (qb *SelectQueryBuilder) GetWhereBuilder() *query.WhereBuilder[query.SelectBuilder] {
	return qb.builder.WhereBuilder
}

func (qb *SelectQueryBuilder) GetJoinBuilder() *query.JoinBuilder[query.SelectBuilder] {
	return qb.builder.JoinBuilder
}

func (qb *SelectQueryBuilder) GetOrderByBuilder() *query.OrderByBuilder[query.SelectBuilder] {
	return qb.builder.GetOrderByBuilder()
}
