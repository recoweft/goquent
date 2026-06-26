package query

import "github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"

func (b *WhereBuilder[T]) ApplyQueryState(q *structs.Query) {
	clone := structs.CloneQuery(q)
	if clone == nil {
		clone = &structs.Query{}
	}
	if clone.Conditions == nil {
		clone.Conditions = &[]structs.Where{}
	}
	if clone.ConditionGroups == nil {
		clone.ConditionGroups = []structs.WhereGroup{}
	}
	b.query = clone
}

func (b *JoinBuilder[T]) ApplyJoins(joins *structs.Joins) {
	clone := structs.CloneJoins(joins)
	if clone == nil {
		clone = &structs.Joins{}
	}
	if clone.Joins == nil {
		clone.Joins = &[]structs.Join{}
	}
	if clone.JoinClauses == nil {
		clone.JoinClauses = &[]structs.JoinClause{}
	}
	if clone.LateralJoins == nil {
		clone.LateralJoins = &[]structs.Join{}
	}
	b.Joins = clone
}

func (b *OrderByBuilder[T]) ApplyOrder(order *[]structs.Order) {
	clone := structs.CloneOrdersPtr(order)
	if clone == nil {
		clone = &[]structs.Order{}
	}
	b.Order = clone
}

func (b *SelectBuilder) ApplyQueryState(q *structs.Query) {
	clone := structs.CloneQuery(q)
	if clone == nil {
		clone = &structs.Query{}
	}
	b.query = clone
	b.selectQuery.Table = clone.Table.Name
	b.selectQuery.Columns = clone.Columns
	b.selectQuery.Limit = clone.Limit
	b.selectQuery.Offset = clone.Offset
	b.selectQuery.Group = clone.Group
	b.selectQuery.Lock = clone.Lock
	if b.selectQuery.Columns == nil {
		b.selectQuery.Columns = &[]structs.Column{}
	}
	if b.selectQuery.Group == nil {
		b.selectQuery.Group = &structs.GroupBy{}
	}
	if b.selectQuery.Lock == nil {
		b.selectQuery.Lock = &structs.Lock{}
	}
	b.WhereBuilder.ApplyQueryState(clone)
	b.JoinBuilder.ApplyJoins(clone.Joins)
	b.JoinBuilder.Table.Name = clone.Table.Name
	b.OrderByBuilder.ApplyOrder(clone.Order)
}

func (b *UpdateBuilder) ApplyQueryState(q *structs.Query) {
	clone := structs.CloneQuery(q)
	if clone == nil {
		clone = &structs.Query{}
	}
	b.WhereBuilder.ApplyQueryState(clone)
	b.JoinBuilder.ApplyJoins(clone.Joins)
	b.JoinBuilder.Table.Name = b.query.Table
	b.OrderByBuilder.ApplyOrder(clone.Order)
}

func (b *DeleteBuilder) ApplyQueryState(q *structs.Query) {
	clone := structs.CloneQuery(q)
	if clone == nil {
		clone = &structs.Query{}
	}
	b.WhereBuilder.ApplyQueryState(clone)
	b.JoinBuilder.ApplyJoins(clone.Joins)
	b.JoinBuilder.Table.Name = b.query.Table
	b.OrderByBuilder.ApplyOrder(clone.Order)
}
