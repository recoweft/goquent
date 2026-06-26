package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type OrderByBaseBuilder struct {
	u     interfaces.SQLUtils
	order *[]structs.Order
}

func NewOrderByBaseBuilder(util interfaces.SQLUtils, order *[]structs.Order) *OrderByBaseBuilder {
	return &OrderByBaseBuilder{
		u:     util,
		order: order,
	}
}

func (o OrderByBaseBuilder) OrderBy(sb *[]byte, order *[]structs.Order) {
	if order == nil || len(*order) == 0 {
		return
	}

	*sb = append(*sb, " ORDER BY "...)

	for i := range *order {
		if i > 0 {
			*sb = append(*sb, ", "...)
		}
		if (*order)[i].Raw != "" {
			*sb = append(*sb, (*order)[i].Raw...)
			continue
		}
		if (*order)[i].Column == "" {
			continue
		}

		desc := "DESC"
		if (*order)[i].IsAsc {
			desc = "ASC"
		}
		*sb = o.u.EscapeReference(*sb, (*order)[i].Column)
		*sb = append(*sb, " "...)
		*sb = append(*sb, desc...)
	}
}
