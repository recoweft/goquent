package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

type JoinQueryBuilder[T QueryBuilderStrategy[T, C], C any] struct {
	builder *query.JoinBuilder[C]
	parent  *T
}

func NewJoinQueryBuilder[T QueryBuilderStrategy[T, C], C any](strategy interfaces.QueryBuilderStrategy) *JoinQueryBuilder[T, C] {
	return &JoinQueryBuilder[T, C]{
		builder: query.NewJoinBuilder[C](strategy),
	}
}

func (b *JoinQueryBuilder[T, C]) SetParent(parent *T) *T {
	b.parent = parent

	return b.parent
}

func (qb *JoinQueryBuilder[T, C]) Join(table, my, condition, target string) T {
	(*qb.parent).GetJoinBuilder().Join(table, my, condition, target)

	return (*qb.parent).GetQueryBuilder()
}

func (qb *JoinQueryBuilder[T, C]) LeftJoin(table, my, condition, target string) T {
	(*qb.parent).GetJoinBuilder().LeftJoin(table, my, condition, target)
	return (*qb.parent).GetQueryBuilder()
}

func (qb *JoinQueryBuilder[T, C]) RightJoin(table, my, condition, target string) T {
	(*qb.parent).GetJoinBuilder().RightJoin(table, my, condition, target)
	return (*qb.parent).GetQueryBuilder()
}

func (qb *JoinQueryBuilder[T, C]) CrossJoin(table string) T {
	(*qb.parent).GetJoinBuilder().CrossJoin(table)
	return (*qb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) JoinQuery(table string, fn func(b *JoinClauseQueryBuilder)) T {
	(*jb.parent).GetJoinBuilder().JoinQuery(table, func(b *query.JoinClauseBuilder) {
		fn(&JoinClauseQueryBuilder{builder: b})
	})
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) LeftJoinQuery(table string, fn func(b *JoinClauseQueryBuilder)) T {
	(*jb.parent).GetJoinBuilder().LeftJoinQuery(table, func(b *query.JoinClauseBuilder) {
		fn(&JoinClauseQueryBuilder{builder: b})
	})
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) RightJoinQuery(table string, fn func(b *JoinClauseQueryBuilder)) T {
	(*jb.parent).GetJoinBuilder().RightJoinQuery(table, func(b *query.JoinClauseBuilder) {
		fn(&JoinClauseQueryBuilder{builder: b})
	})
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) JoinSubQuery(qb *SelectQueryBuilder, alias, my, condition, target string) T {
	(*jb.parent).GetJoinBuilder().JoinSub(qb.builder, alias, my, condition, target)
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) LeftJoinSubQuery(qb *SelectQueryBuilder, alias, my, condition, target string) T {
	(*jb.parent).GetJoinBuilder().LeftJoinSub(qb.builder, alias, my, condition, target)
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) RightJoinSubQuery(qb *SelectQueryBuilder, alias, my, condition, target string) T {
	(*jb.parent).GetJoinBuilder().RightJoinSub(qb.builder, alias, my, condition, target)
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) JoinLateral(qb *SelectQueryBuilder, alias string) T {
	(*jb.parent).GetJoinBuilder().JoinLateral(qb.builder, alias)
	return (*jb.parent).GetQueryBuilder()
}

func (jb *JoinQueryBuilder[T, C]) LeftJoinLateral(qb *SelectQueryBuilder, alias string) T {
	(*jb.parent).GetJoinBuilder().LeftJoinLateral(qb.builder, alias)
	return (*jb.parent).GetQueryBuilder()
}
