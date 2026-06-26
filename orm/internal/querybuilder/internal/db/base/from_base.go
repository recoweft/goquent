package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type FromBaseBuilder struct {
	u interfaces.SQLUtils
}

func NewFromBaseBuilder(util interfaces.SQLUtils) *FromBaseBuilder {
	return &FromBaseBuilder{
		u: util,
	}
}

func (f FromBaseBuilder) From(sb *[]byte, table string) {
	*sb = append(*sb, "FROM "...)
	*sb = f.u.EscapeRelation(*sb, table)
}
