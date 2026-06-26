package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type JoinBaseBuilder struct {
	u           interfaces.SQLUtils
	join        *structs.Joins
	columnNames *[]string
}

func NewJoinBaseBuilder(util interfaces.SQLUtils, j *structs.Joins) *JoinBaseBuilder {
	return &JoinBaseBuilder{
		u:           util,
		join:        j,
		columnNames: &[]string{},
	}
}

// Join builds the JOIN query.
func (jb *JoinBaseBuilder) Join(sb *[]byte, joins *structs.Joins) []interface{} {
	if jb.columnNames == nil {
		jb.columnNames = &[]string{}
	}

	values := jb.buildJoinStatement(sb, joins)

	return values
}

// buildJoinStatement builds the JOIN statement.
func (jb *JoinBaseBuilder) buildJoinStatement(sb *[]byte, joins *structs.Joins) []interface{} {
	if joins == nil {
		return nil
	}

	var values []interface{}
	if joins.JoinClauses != nil {
		for _, joinClause := range *joins.JoinClauses {
			jb.appendJoinClause(sb, joinClause, &values)
		}
	}

	if joins.Joins != nil {
		sortedJoins := make([]structs.Join, 0, len(*joins.Joins))
		if joins.LateralJoins != nil && len(*joins.LateralJoins) > 0 {
			sortedJoins = append(sortedJoins, (*joins.LateralJoins)...)
		}
		sortedJoins = append(sortedJoins, (*joins.Joins)...)

		for _, join := range sortedJoins {
			jb.appendSortedJoin(sb, join, &values)
		}
	}

	return values
}

func (jb *JoinBaseBuilder) appendJoinClause(sb *[]byte, joinClause structs.JoinClause, values *[]interface{}) {
	j := &structs.Join{
		TargetNameMap: joinClause.TargetNameMap,
		Name:          joinClause.Name,
	}

	joinType, targetName := jb.processJoin(j)

	*sb = append(*sb, " "...)
	*sb = append(*sb, joinType...)
	*sb = append(*sb, " JOIN "...)

	if joinClause.Query != nil {
		*sb = append(*sb, "("...)
		b := jb.u.GetQueryBuilderStrategy()
		v, _ := b.Build(sb, joinClause.Query, 0, nil)
		*values = append(*values, v...)
		*sb = append(*sb, ") as "...)
		*sb = jb.u.EscapeReference(*sb, targetName)
	} else {
		*sb = jb.u.EscapeRelation(*sb, targetName)
	}

	*sb = append(*sb, " ON "...)

	op := ""
	for i, on := range *joinClause.On {
		if i > 0 {
			op = jb.getLogicalOperator(on.Operator)
		}
		jb.appendCondition(sb, on.Column, on.Condition, on.Value, &op)
	}

	for i, condition := range *joinClause.Conditions {
		if i > 0 || len(*joinClause.On) > 0 {
			op = jb.getLogicalOperator(condition.Operator)
		}
		jb.appendCondition(sb, condition.Column, condition.Condition, condition.Value, &op)
		*values = append(*values, condition.Value...)
	}
}

func (jb *JoinBaseBuilder) appendSortedJoin(sb *[]byte, join structs.Join, values *[]interface{}) {
	joinType, targetName := jb.processJoin(&join)
	if joinType == "" || targetName == "" {
		return
	}

	if _, ok := join.TargetNameMap[consts.Join_LATERAL]; ok {
		*sb = append(*sb, " ,"...)
		*sb = append(*sb, joinType...)
	} else if _, ok := join.TargetNameMap[consts.Join_LEFT_LATERAL]; ok {
		*sb = append(*sb, " ,"...)
		*sb = append(*sb, joinType...)
	} else {
		*sb = append(*sb, " "...)
		*sb = append(*sb, joinType...)
		*sb = append(*sb, " JOIN "...)
	}

	if join.Query != nil {
		*sb = append(*sb, "("...)
		b := jb.u.GetQueryBuilderStrategy()
		v, _ := b.Build(sb, join.Query, 0, nil)
		*values = append(*values, v...)
		*sb = append(*sb, ") as "...)
		*sb = jb.u.EscapeReference(*sb, targetName)
	} else {
		*sb = jb.u.EscapeRelation(*sb, targetName)
	}

	if _, ok := join.TargetNameMap[consts.Join_CROSS]; !ok {
		if _, ok := join.TargetNameMap[consts.Join_LATERAL]; !ok {
			if _, ok := join.TargetNameMap[consts.Join_LEFT_LATERAL]; !ok {
				*sb = append(*sb, " ON "...)
				*sb = jb.u.EscapeReference(*sb, join.SearchColumn)
				*sb = append(*sb, " "...)
				*sb = append(*sb, join.SearchCondition...)
				*sb = append(*sb, " "...)
				*sb = jb.u.EscapeReference(*sb, join.SearchTargetColumn)
			}
		}
	}
}

func (jb *JoinBaseBuilder) appendCondition(sb *[]byte, column, condition string, value interface{}, op *string) {
	if *op != "" {
		*sb = append(*sb, *op...)
	}
	*sb = jb.u.EscapeReference(*sb, column)
	*sb = append(*sb, " "...)
	*sb = append(*sb, condition...)
	if value != nil {
		switch castedValue := value.(type) {
		case string:
			*sb = append(*sb, " "...)
			*sb = jb.u.EscapeReference(*sb, castedValue) // TODO: validate this cast
		default:
			*sb = append(*sb, " "+jb.u.GetPlaceholder()...)
		}
	}
}

func (jb *JoinBaseBuilder) getLogicalOperator(operator int) string {
	if operator == consts.LogicalOperator_OR {
		return " OR "
	}
	return " AND "
}

func (j *JoinBaseBuilder) processJoin(join *structs.Join) (string, string) {
	joinType := ""
	targetName := ""

	if _, ok := join.TargetNameMap[consts.Join_CROSS]; ok {
		targetName = join.TargetNameMap[consts.Join_CROSS]
		joinType = consts.Join_Type_CROSS
	}
	if _, ok := join.TargetNameMap[consts.Join_RIGHT]; ok {
		targetName = join.TargetNameMap[consts.Join_RIGHT]
		joinType = consts.Join_Type_RIGHT
	}
	if _, ok := join.TargetNameMap[consts.Join_LEFT]; ok {
		targetName = join.TargetNameMap[consts.Join_LEFT]
		joinType = consts.Join_Type_LEFT
	}
	if _, ok := join.TargetNameMap[consts.Join_INNER]; ok {
		targetName = join.TargetNameMap[consts.Join_INNER]
		joinType = consts.Join_Type_INNER
	}
	if _, ok := join.TargetNameMap[consts.Join_LATERAL]; ok {
		targetName = join.TargetNameMap[consts.Join_LATERAL]
		joinType = consts.Join_Type_LATERAL
	}
	if _, ok := join.TargetNameMap[consts.Join_LEFT_LATERAL]; ok {
		targetName = join.TargetNameMap[consts.Join_LEFT_LATERAL]
		joinType = consts.Join_Type_LEFT_LATERAL
	}

	if targetName == "" {
		return "", ""
	}

	return joinType, targetName
}
