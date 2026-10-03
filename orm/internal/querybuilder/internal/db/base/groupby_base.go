package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/predicate"
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
	values, _ := g.GroupBySnapshot(sb, groupBy, 0)
	return values
}

func (g GroupByBaseBuilder) GroupBySnapshot(sb *[]byte, groupBy *structs.GroupBy, offset int) ([]any, *predicate.Node) {
	if groupBy == nil || len(groupBy.Columns) == 0 {
		return nil, nil
	}
	*sb = append(*sb, " GROUP BY "...)
	for i, c := range groupBy.Columns {
		if i > 0 {
			*sb = append(*sb, ", "...)
		}
		*sb = g.u.EscapeReference(*sb, c)
	}
	if groupBy.Having == nil {
		return nil, nil
	}
	start := len(*sb)
	wb := NewWhereBaseBuilder(g.u, nil)
	values, tree, _ := RenderPredicates(sb, structs.HavingGroups(groupBy), offset, wb.RenderLeaf)
	if tree != nil { // WHERE and HAVING differ only in the clause prefix.
		tail := append([]byte(nil), (*sb)[start+7:]...)
		*sb = append((*sb)[:start], " HAVING "...)
		*sb = append(*sb, tail...)
	}
	return values, tree
}
