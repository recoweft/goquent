package query

import (
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type OrderByBuilder[T any] struct {
	Order  *[]structs.Order
	parent *T
}

func NewOrderByBuilder[T any](strategy interfaces.QueryBuilderStrategy) *OrderByBuilder[T] {
	return &OrderByBuilder[T]{
		Order: &[]structs.Order{},
	}
}

func (b *OrderByBuilder[T]) SetParent(parent *T) *T {
	b.parent = parent

	return b.parent
}

// OrderBy adds an ORDER BY clause.
func (b *OrderByBuilder[T]) OrderBy(column string, ascDesc string) *T {
	ascDesc = strings.ToUpper(ascDesc)

	if ascDesc == consts.Order_ASC {
		*b.Order = append(*b.Order, structs.Order{
			Column: column,
			IsAsc:  consts.Order_FLAG_ASC,
		})
	} else if ascDesc == consts.Order_DESC {
		*b.Order = append(*b.Order, structs.Order{
			Column: column,
			IsAsc:  consts.Order_FLAG_DESC,
		})
	}
	return b.parent
}

// ReOrder removes all ORDER BY clauses.
func (b *OrderByBuilder[T]) ReOrder() *T {
	*b.Order = []structs.Order{}
	return b.parent
}

// OrderByRaw adds a raw ORDER BY clause.
func (b *OrderByBuilder[T]) OrderByRaw(raw string) *T {
	*b.Order = append(*b.Order, structs.Order{
		Raw: raw,
	})
	return b.parent
}
