package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

type JoinClauseBuilder struct {
	JoinClause *structs.JoinClause
}

func NewJoinClauseBuilder() *JoinClauseBuilder {
	return &JoinClauseBuilder{
		JoinClause: &structs.JoinClause{
			On:         &[]structs.On{},
			Conditions: &[]structs.Where{},
		},
	}
}

func (b *JoinClauseBuilder) On(my string, condition string, target string) *JoinClauseBuilder {
	*b.JoinClause.On = append(*b.JoinClause.On, structs.On{
		Column:    my,
		Condition: condition,
		Value:     target,
		Operator:  consts.LogicalOperator_AND,
	})

	return b
}

func (b *JoinClauseBuilder) OrOn(my string, condition string, target string) *JoinClauseBuilder {
	*b.JoinClause.On = append(*b.JoinClause.On, structs.On{
		Column:    my,
		Condition: condition,
		Value:     target,
		Operator:  consts.LogicalOperator_OR,
	})

	return b
}

func (b *JoinClauseBuilder) Where(column string, condition string, value interface{}) *JoinClauseBuilder {
	*b.JoinClause.Conditions = append(*b.JoinClause.Conditions, structs.Where{
		Column:    column,
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  consts.LogicalOperator_AND,
	})

	return b
}

func (b *JoinClauseBuilder) OrWhere(column string, condition string, value interface{}) *JoinClauseBuilder {
	*b.JoinClause.Conditions = append(*b.JoinClause.Conditions, structs.Where{
		Column:    column,
		Condition: condition,
		Value:     []interface{}{value},
		Operator:  consts.LogicalOperator_OR,
	})
	return b
}
