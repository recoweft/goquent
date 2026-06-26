package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

type UnionBaseBuilder struct {
}

func NewUnionBaseBuilder() *UnionBaseBuilder {
	return &UnionBaseBuilder{}
}

func (ub *UnionBaseBuilder) Union(sb *[]byte, unions *[]structs.Union, number int) {
	if unions == nil {
		return
	}

	ub.buildUnionStatement(sb, unions, number)
}

func (ub *UnionBaseBuilder) buildUnionStatement(sb *[]byte, unions *[]structs.Union, number int) {
	if (*unions)[number].Query != nil {
		if len(*unions) > number+1 {
			if (*unions)[number].IsAll {
				*sb = append(*sb, " UNION ALL "...)
			} else {
				*sb = append(*sb, " UNION "...)
			}
		}
	}
}
