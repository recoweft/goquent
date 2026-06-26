package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sliceutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sqlutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type SelectBaseBuilder struct {
	columnNames *[]string
	u           interfaces.SQLUtils
}

func NewSelectBaseBuilder(u interfaces.SQLUtils, columnNames *[]string) *SelectBaseBuilder {
	return &SelectBaseBuilder{
		columnNames: columnNames,
		u:           u,
	}
}

func (b *SelectBaseBuilder) Select(sb *[]byte, columns *[]structs.Column, tableName string, joins *structs.Joins) ([]interface{}, error) {
	if columns == nil {
		*sb = append(*sb, "*"...)
		return []interface{}{}, nil
	}

	outputed := false
	// if there are no columns to select, select all columns
	if len(*columns) == 0 && joins != nil {
		sortedJoins := make([]structs.Join, 0)
		if joins.LateralJoins != nil {
			sortedJoins = append(sortedJoins, (*joins.LateralJoins)...)
		}
		if joins.Joins != nil {
			sortedJoins = append(sortedJoins, (*joins.Joins)...)
		}
		for i, join := range sortedJoins {
			b.processJoin(sb, &join, tableName, i)
			outputed = true
		}

		if joins.JoinClauses != nil {
			for _, joinClause := range *joins.JoinClauses {
				join := structs.Join{
					TargetNameMap: joinClause.TargetNameMap,
					Name:          joinClause.Name,
				}
				b.processJoin(sb, &join, tableName, 0)
				outputed = true
			}
		}

	}

	if len(*columns) == 0 && !outputed {
		*sb = append(*sb, "*"...)
		return []interface{}{}, nil
	}

	// if there are columns has values
	var colValues []interface{}
	hasValues := false
	for i := 0; i < len(*columns); i++ {
		if len((*columns)[i].Values) > 0 {
			hasValues = true
			break
		}
	}
	if hasValues {
		colValues = make([]interface{}, 0, len(*columns))
	}

	// if there are columns to select
	firstDistinct := false
	for i := 0; i < len(*columns); i++ {
		if (*columns)[i].Distinct && !(*columns)[i].Count && !firstDistinct {
			*sb = append(*sb, "DISTINCT "...)
			firstDistinct = true
		}

		if (*columns)[i].Count {
			*sb = append(*sb, "COUNT("...)
			if (*columns)[i].Distinct {
				*sb = append(*sb, "DISTINCT "...)
			}
			if (*columns)[i].Name != "" {
				*sb = b.u.EscapeAliasedValue(*sb, (*columns)[i].Name)
			} else {
				*sb = append(*sb, "*"...)
			}
			*sb = append(*sb, ")"...)
			if i < len(*columns)-1 {
				*sb = append(*sb, ", "...)
			}

			continue
		}

		if (*columns)[i].Function != "" {
			if i > 0 {
				*sb = append(*sb, ", "...)
			}
			*sb = append(*sb, (*columns)[i].Function...)
			*sb = append(*sb, "("...)
			if (*columns)[i].Distinct {
				*sb = append(*sb, "DISTINCT "...)
			}
			if (*columns)[i].Name != "" {
				*sb = b.u.EscapeAliasedValue(*sb, (*columns)[i].Name)
			} else {
				*sb = append(*sb, "*"...)
			}
			*sb = append(*sb, ")"...)
		} else if (*columns)[i].Raw != "" {
			if len((*columns)[i].Values) > 0 {
				colValues = append(colValues, (*columns)[i].Values...)
			}
			if i > 0 {
				*sb = append(*sb, ", "...)
			}
			rawSQL := (*columns)[i].Raw
			if len((*columns)[i].Values) > 0 {
				expanded, err := sqlutils.ExpandPositionalPlaceholders(rawSQL, len((*columns)[i].Values), b.u.GetPlaceholder)
				if err != nil {
					return nil, err
				}
				rawSQL = expanded
			}
			*sb = append(*sb, rawSQL...) // or colNames = column.Raw
		} else if (*columns)[i].Name != "" {
			if i > 0 {
				*sb = append(*sb, ", "...)
			}

			*sb = b.u.EscapeAliasedValue(*sb, (*columns)[i].Name)
		}
	}

	return colValues, nil
}

func (j *SelectBaseBuilder) processJoin(sb *[]byte, join *structs.Join, tableName string, idx int) {
	targetName := ""
	//joinedTablesForSelect := ""

	if _, ok := join.TargetNameMap[consts.Join_CROSS]; ok {
		targetName = join.TargetNameMap[consts.Join_CROSS]
	}
	if _, ok := join.TargetNameMap[consts.Join_RIGHT]; ok {
		targetName = join.TargetNameMap[consts.Join_RIGHT]
	}
	if _, ok := join.TargetNameMap[consts.Join_LEFT]; ok {
		targetName = join.TargetNameMap[consts.Join_LEFT]
	}
	if _, ok := join.TargetNameMap[consts.Join_INNER]; ok {
		targetName = join.TargetNameMap[consts.Join_INNER]
	}
	if _, ok := join.TargetNameMap[consts.Join_LATERAL]; ok {
		targetName = join.TargetNameMap[consts.Join_LATERAL]
	}
	if _, ok := join.TargetNameMap[consts.Join_LEFT_LATERAL]; ok {
		targetName = join.TargetNameMap[consts.Join_LEFT_LATERAL]
	}

	if targetName == "" {
		return
	}

	name := tableName
	if join.Name != "" {
		name = join.Name
	}

	wsb := make([]byte, 0, consts.StringBuffer_Short_Query_Grow)
	targetReference := sqlutils.RelationSelectReference(targetName)
	wsb = j.u.EscapeReference(wsb, targetReference)
	wsb = append(wsb, ".*"...)
	targetNameForSelect := string(wsb)
	wsb = wsb[:0]

	outputed := false
	if !sliceutils.Contains(*j.columnNames, targetNameForSelect) {
		if idx > 0 {
			*sb = append(*sb, ", "...)
		}
		*sb = append(*sb, targetNameForSelect...)
		*j.columnNames = append(*j.columnNames, targetNameForSelect)
		outputed = true
	}

	nameReference := sqlutils.RelationSelectReference(name)
	wsb = j.u.EscapeReference(wsb, nameReference)
	wsb = append(wsb, ".*"...)
	nameForSelect := string(wsb)

	if !sliceutils.Contains(*j.columnNames, nameForSelect) {
		if idx > 0 || outputed {
			*sb = append(*sb, ", "...)
		}
		*sb = append(*sb, nameForSelect...)
		*j.columnNames = append(*j.columnNames, nameForSelect)
	}

}
