package base

import (
	"errors"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sqlutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type WhereBaseBuilder struct {
	u           interfaces.SQLUtils
	whereGroups []structs.WhereGroup
}

func NewWhereBaseBuilder(util interfaces.SQLUtils, wg []structs.WhereGroup) *WhereBaseBuilder {
	return &WhereBaseBuilder{
		u:           util,
		whereGroups: wg,
	}
}

func (wb *WhereBaseBuilder) Where(sb *[]byte, wg []structs.WhereGroup) ([]interface{}, error) {
	values, _, err := RenderPredicates(sb, wg, 0, wb.RenderLeaf)
	return values, err
}

func (wb *WhereBaseBuilder) HasCondition(wg []structs.WhereGroup) bool {
	for _, cg := range wg {
		if len(cg.Conditions) > 0 {
			return true
		}
	}
	return false
}

func (wb *WhereBaseBuilder) GetConditionGroupSeparator(cg structs.WhereGroup, index int) string {
	if cg.IsDummyGroup {
		return ""
	}
	if index == 0 {
		return ""
	}
	switch cg.Operator {
	case consts.LogicalOperator_AND:
		return " AND "
	case consts.LogicalOperator_OR:
		return " OR "
	}
	return ""
}

func (wb *WhereBaseBuilder) GetNotSeparator(cg structs.WhereGroup) string {
	if cg.IsNot {
		return "NOT "
	}
	return ""
}

func (wb *WhereBaseBuilder) GetParenthesesOpen(cg structs.WhereGroup) string {
	if cg.IsDummyGroup {
		return ""
	}
	return "("
}

func (wb *WhereBaseBuilder) GetParenthesesClose(cg structs.WhereGroup) string {
	if cg.IsDummyGroup {
		return ""
	}
	return ")"
}

func (wb *WhereBaseBuilder) GetConditionOperator(c structs.Where) string {
	switch c.Operator {
	case consts.LogicalOperator_AND:
		return " AND "
	case consts.LogicalOperator_OR:
		return " OR "
	}
	return ""
}

func (wb *WhereBaseBuilder) ProcessSubQuery(sb *[]byte, c structs.Where) ([]interface{}, error) {
	*sb = wb.u.EscapeReference(*sb, c.Column)
	*sb = append(*sb, " "...)
	*sb = append(*sb, c.Condition...)

	*sb = append(*sb, " ("...)

	b := wb.u.GetQueryBuilderStrategy()
	sqValues, err := b.Build(sb, c.Query, 0, nil)
	if err != nil {
		return nil, err
	}

	*sb = append(*sb, ")"...)
	return sqValues, nil
}

func (wb *WhereBaseBuilder) ProcessExistsQuery(sb *[]byte, c structs.Where) ([]interface{}, error) {
	*sb = append(*sb, c.Condition...)

	*sb = append(*sb, " ("...)
	b := wb.u.GetQueryBuilderStrategy()
	sqValues, err := b.Build(sb, c.Exists.Query, 0, nil)
	if err != nil {
		return nil, err
	}
	*sb = append(*sb, ")"...)

	return sqValues, nil
}

func (wb *WhereBaseBuilder) ProcessBetweenCondition(sb *[]byte, c structs.Where) []interface{} {
	values := make([]interface{}, 0, 2)
	if c.Between.IsColumn {
		*sb = wb.u.EscapeReference(*sb, c.Column)
		*sb = append(*sb, " "...)
		*sb = append(*sb, c.Condition...)
		*sb = append(*sb, " "...)
		*sb = wb.u.EscapeReference(*sb, c.Between.From.(string))
		*sb = append(*sb, " AND "...)
		*sb = wb.u.EscapeReference(*sb, c.Between.To.(string))
	} else {
		*sb = wb.u.EscapeReference(*sb, c.Column)
		*sb = append(*sb, " "...)
		*sb = append(*sb, c.Condition...)
		*sb = append(*sb, " "...)
		*sb = append(*sb, wb.u.GetPlaceholder()...)
		*sb = append(*sb, " AND "...)
		*sb = append(*sb, wb.u.GetPlaceholder()...)
		values = []interface{}{c.Between.From, c.Between.To}
	}

	return values
}

func (wb *WhereBaseBuilder) ProcessRawCondition(sb *[]byte, c structs.Where) ([]interface{}, error) {
	if c.LiteralColumn {
		*sb = writeColumn(*sb, wb.u, c.Column, true)
		*sb = append(*sb, "="+wb.u.GetPlaceholder()...)
		return c.Value, nil
	}

	if c.Raw != "" {
		if c.ValueMap != nil {
			rawSQL, values, err := sqlutils.ExpandNamedPlaceholders(c.Raw, c.ValueMap, wb.u.GetPlaceholder)
			if err != nil {
				return nil, err
			}

			*sb = append(*sb, rawSQL...)
			return values, nil
		}
		*sb = append(*sb, c.Raw...)
	} else {
		*sb = wb.u.EscapeReference(*sb, c.Column)
		*sb = append(*sb, " "...)
		*sb = append(*sb, c.Condition...)
		if c.ValueColumn != "" {
			*sb = append(*sb, " "...)
			*sb = wb.u.EscapeReference(*sb, c.ValueColumn)
		} else if c.Value != nil {
			if c.Condition == consts.Condition_IN || c.Condition == consts.Condition_NOT_IN || len(c.Value) > 1 {
				*sb = append(*sb, " ("...)
				for k := 0; k < len(c.Value); k++ {
					if k > 0 {
						*sb = append(*sb, ", "...)
					}
					*sb = append(*sb, wb.u.GetPlaceholder()...)
				}
				*sb = append(*sb, ")"...)
			} else {
				*sb = append(*sb, " "...)
				*sb = append(*sb, wb.u.GetPlaceholder()...)
			}
		}
	}

	values := c.Value

	return values, nil
}

func (wb *WhereBaseBuilder) ProcessFullText(sb *[]byte, c structs.Where) ([]interface{}, error) {
	values := []interface{}{}

	// Implement FullText

	return values, errors.New("not implemented")
}

func (wb *WhereBaseBuilder) ProcessFunction(sb *[]byte, c structs.Where) []interface{} {
	*sb = append(*sb, c.Function...)
	*sb = append(*sb, "("...)
	*sb = wb.u.EscapeReference(*sb, c.Column)
	*sb = append(*sb, ") "...)
	*sb = append(*sb, c.Condition...)
	if c.ValueColumn != "" {
		*sb = append(*sb, " "...)
		*sb = wb.u.EscapeReference(*sb, c.ValueColumn)
	} else if c.Value != nil {
		if c.Condition == consts.Condition_IN || c.Condition == consts.Condition_NOT_IN || len(c.Value) > 1 {
			*sb = append(*sb, " ("...)
			for k := 0; k < len(c.Value); k++ {
				if k > 0 {
					*sb = append(*sb, ", "...)
				}
				*sb = append(*sb, wb.u.GetPlaceholder()...)
			}
			*sb = append(*sb, ")"...)
		} else {
			*sb = append(*sb, " "...)
			*sb = append(*sb, wb.u.GetPlaceholder()...)
		}
	}

	values := c.Value

	return values
}
