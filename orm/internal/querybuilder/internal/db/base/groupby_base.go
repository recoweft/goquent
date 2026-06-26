package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type GroupByBaseBuilder struct {
	u interfaces.SQLUtils
}

func NewGroupByBaseBuilder(util interfaces.SQLUtils) *GroupByBaseBuilder {
	return &GroupByBaseBuilder{
		u: util,
	}
}

func (g GroupByBaseBuilder) GroupBy(sb *[]byte, groupBy *structs.GroupBy) []interface{} {
	if groupBy == nil || len(groupBy.Columns) == 0 {
		return []interface{}{}
	}

	groupByColumns := groupBy.Columns
	if len(groupByColumns) > 0 {
		*sb = append(*sb, " GROUP BY "...)
		for i := range groupByColumns {
			if i > 0 {
				*sb = append(*sb, ", "...)
			}
			*sb = g.u.EscapeReference(*sb, groupByColumns[i])
		}
	}

	values := make([]interface{}, 0, len(*groupBy.Having))

	if len(*groupBy.Having) > 0 {
		*sb = append(*sb, " HAVING "...)

		//havingValues := make([]interface{}, 0, len(*groupBy.Having))
		for n := range *groupBy.Having {
			op := "AND"
			if (*groupBy.Having)[n].Operator == consts.LogicalOperator_AND {
				op = "AND"
			} else if (*groupBy.Having)[n].Operator == consts.LogicalOperator_OR {
				op = "OR"
			}

			if (*groupBy.Having)[n].Raw != "" {
				if n > 0 {
					*sb = append(*sb, " "...)
					*sb = append(*sb, op...)
					*sb = append(*sb, " "...)
				}
				*sb = append(*sb, (*groupBy.Having)[n].Raw...)
				continue
			}
			if (*groupBy.Having)[n].Column == "" {
				continue
			}
			if (*groupBy.Having)[n].Condition == "" {
				continue
			}
			if (*groupBy.Having)[n].Value == "" {
				continue
			}
			//havingValues = append(havingValues, having.Value)
			values = append(values, (*groupBy.Having)[n].Value)

			if n > 0 {
				*sb = append(*sb, " "...)
				*sb = append(*sb, op...)
				*sb = append(*sb, " "...)
			}
			*sb = g.u.EscapeReference(*sb, (*groupBy.Having)[n].Column)
			*sb = append(*sb, " "...)
			*sb = append(*sb, (*groupBy.Having)[n].Condition...)
			*sb = append(*sb, " "...)
			*sb = append(*sb, g.u.GetPlaceholder()...)
		}

		//if len(havingValues) > 0 {
		//	values = append(values, havingValues...)
		//}
	}

	return values
}
