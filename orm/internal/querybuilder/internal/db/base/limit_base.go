package base

import (
	"strconv"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

type LimitBaseBuilder struct {
}

func NewLimitBaseBuilder() *LimitBaseBuilder {
	return &LimitBaseBuilder{}
}

func (LimitBaseBuilder) Limit(sb *[]byte, limit structs.Limit) {
	if limit.Limit == 0 {
		return
	}

	*sb = append(*sb, " LIMIT "...)
	*sb = strconv.AppendInt(*sb, limit.Limit, 10)
}
