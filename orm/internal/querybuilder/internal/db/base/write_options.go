package base

import (
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"strings"
)

func writeDialect(u interfaces.SQLUtils) driver.Dialect {
	if u.Dialect() == consts.DialectPostgreSQL {
		return driver.PostgresDialect{}
	}
	return driver.MySQLDialect{}
}
func writeColumn(sb []byte, u interfaces.SQLUtils, col string, literal bool) []byte {
	if literal {
		return append(sb, writeDialect(u).QuoteIdent(col)...)
	}
	return u.EscapeReference(sb, col)
}
func writeTable(sb []byte, u interfaces.SQLUtils, table string, o writeinput.Options) []byte {
	if !o.Literal {
		return u.EscapeRelation(sb, table)
	}
	parts := o.TableParts
	if len(parts) == 0 {
		parts = strings.Split(table, ".")
	}
	for i, p := range parts {
		if i > 0 {
			sb = append(sb, '.')
		}
		sb = append(sb, writeDialect(u).QuoteIdent(p)...)
	}
	return sb
}
