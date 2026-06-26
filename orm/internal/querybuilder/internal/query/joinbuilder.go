package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type JoinBuilder[T any] struct {
	dbBuilder interfaces.QueryBuilderStrategy
	Table     *structs.Table
	Joins     *structs.Joins
	parent    *T
}

func NewJoinBuilder[T any](dbBuilder interfaces.QueryBuilderStrategy) *JoinBuilder[T] {
	return &JoinBuilder[T]{
		dbBuilder: dbBuilder,
		Table:     &structs.Table{},
		Joins: &structs.Joins{
			Joins:        &[]structs.Join{},
			LateralJoins: &[]structs.Join{},
			JoinClauses:  &[]structs.JoinClause{},
		},
	}
}

func (b *JoinBuilder[T]) SetParent(parent *T) *T {
	b.parent = parent

	return b.parent
}

// Join adds a JOIN clause.
func (b *JoinBuilder[T]) Join(table string, my string, condition string, target string) *T {
	return b.joinCommon(consts.Join_INNER, table, my, condition, target)
}

// LeftJoin adds a LEFT JOIN clause.
func (b *JoinBuilder[T]) LeftJoin(table string, my string, condition string, target string) *T {
	return b.joinCommon(consts.Join_LEFT, table, my, condition, target)
}

// RightJoin adds a RIGHT JOIN clause.
func (b *JoinBuilder[T]) RightJoin(table string, my string, condition string, target string) *T {
	return b.joinCommon(consts.Join_RIGHT, table, my, condition, target)
}

// joinCommon is a helper function for JOIN, LEFT JOIN, and RIGHT JOIN.
func (b *JoinBuilder[T]) joinCommon(joinType string, table string, my string, condition string, target string) *T {
	myTable := b.Table.Name
	// If a previous JOIN exists, retrieve the table name of that JOIN.
	if b.Joins.Joins != nil && len(*b.Joins.Joins) > 0 {
		myTable = (*b.Joins.Joins)[len(*b.Joins.Joins)-1].Name
	}
	*b.Joins.Joins = append(*b.Joins.Joins, structs.Join{
		Name: myTable,
		TargetNameMap: map[string]string{
			joinType: table,
		},
		SearchColumn:       my,
		SearchCondition:    condition,
		SearchTargetColumn: target,
	})
	return b.parent
}

// CrossJoin adds a CROSS JOIN clause.
func (b *JoinBuilder[T]) CrossJoin(table string) *T {
	myTable := b.Table.Name
	// If a previous JOIN exists, retrieve the table name of that JOIN.
	if b.Joins != nil && len(*b.Joins.Joins) > 0 {
		myTable = (*b.Joins.Joins)[len(*b.Joins.Joins)-1].Name
	}
	*b.Joins.Joins = append(*b.Joins.Joins, structs.Join{
		Name: myTable,
		TargetNameMap: map[string]string{
			consts.Join_CROSS: table,
		},
	})
	return b.parent
}

func (b *JoinBuilder[T]) JoinQuery(table string, fn func(j *JoinClauseBuilder)) *T {
	jq := NewJoinClauseBuilder()
	fn(jq)

	jq.JoinClause.Name = table
	jq.JoinClause.TargetNameMap = map[string]string{
		consts.Join_INNER: table,
	}

	*b.Joins.JoinClauses = append(*b.Joins.JoinClauses, *jq.JoinClause)

	return b.parent
}

func (b *JoinBuilder[T]) LeftJoinQuery(table string, fn func(j *JoinClauseBuilder)) *T {
	jq := NewJoinClauseBuilder()
	fn(jq)

	jq.JoinClause.Name = table
	jq.JoinClause.TargetNameMap = map[string]string{
		consts.Join_LEFT: table,
	}

	*b.Joins.JoinClauses = append(*b.Joins.JoinClauses, *jq.JoinClause)

	return b.parent
}

func (b *JoinBuilder[T]) RightJoinQuery(table string, fn func(j *JoinClauseBuilder)) *T {
	jq := NewJoinClauseBuilder()
	fn(jq)

	jq.JoinClause.Name = table
	jq.JoinClause.TargetNameMap = map[string]string{
		consts.Join_RIGHT: table,
	}

	*b.Joins.JoinClauses = append(*b.Joins.JoinClauses, *jq.JoinClause)

	return b.parent
}

func (b *JoinBuilder[T]) JoinSub(q *SelectBuilder, alias, my, condition, target string) *T {
	b.joinSubCommon(consts.Join_INNER, q, alias, my, condition, target)
	return b.parent
}

func (b *JoinBuilder[T]) LeftJoinSub(q *SelectBuilder, alias, my, condition, target string) *T {
	b.joinSubCommon(consts.Join_LEFT, q, alias, my, condition, target)
	return b.parent
}

func (b *JoinBuilder[T]) RightJoinSub(q *SelectBuilder, alias, my, condition, target string) *T {
	b.joinSubCommon(consts.Join_RIGHT, q, alias, my, condition, target)
	return b.parent
}

func (b *JoinBuilder[T]) joinSubCommon(joinType string, q *SelectBuilder, alias, my, condition, target string) *T {

	q.WhereBuilder.query.ConditionGroups = append(q.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
		Conditions:   *q.WhereBuilder.query.Conditions,
		IsDummyGroup: true,
	})

	*q.WhereBuilder.query.Conditions = []structs.Where{}

	sq := &structs.Query{
		ConditionGroups: q.WhereBuilder.query.ConditionGroups,
		Table:           structs.Table{Name: q.selectQuery.Table},
		Columns:         q.selectQuery.Columns,
		Joins:           q.JoinBuilder.Joins,
		Order:           q.OrderByBuilder.Order,
	}

	myTable := b.Table.Name
	args := &structs.Join{
		Name: myTable,
		TargetNameMap: map[string]string{
			joinType: alias,
		},
		SearchColumn:       my,
		SearchCondition:    condition,
		SearchTargetColumn: target,
		Query:              sq,
	}

	*b.Joins.Joins = append(*b.Joins.Joins, *args)
	//b.joinValues = append(b.joinValues, value...)
	return b.parent
}

func (b *JoinBuilder[T]) JoinLateral(q *SelectBuilder, alias string) *T {
	return b.joinLateralCommon(consts.Join_LATERAL, q, alias)
}

func (b *JoinBuilder[T]) LeftJoinLateral(q *SelectBuilder, alias string) *T {
	return b.joinLateralCommon(consts.Join_LEFT_LATERAL, q, alias)
}

func (b *JoinBuilder[T]) joinLateralCommon(joinType string, q *SelectBuilder, alias string) *T {

	q.WhereBuilder.query.ConditionGroups = append(q.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
		Conditions:   *q.WhereBuilder.query.Conditions,
		IsDummyGroup: true,
	})

	*q.WhereBuilder.query.Conditions = []structs.Where{}

	sq := &structs.Query{
		ConditionGroups: q.WhereBuilder.query.ConditionGroups,
		Table:           structs.Table{Name: q.selectQuery.Table},
		Columns:         q.selectQuery.Columns,
		Joins:           q.JoinBuilder.Joins,
		Order:           q.OrderByBuilder.Order,
	}

	myTable := b.Table.Name
	args := &structs.Join{
		Name: myTable,
		TargetNameMap: map[string]string{
			joinType: alias,
		},
		Query: sq,
	}

	*b.Joins.LateralJoins = append(*b.Joins.LateralJoins, *args)
	return b.parent
}
