package mysql

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sqlutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type SQLUtils struct {
}

func NewSQLUtils() *SQLUtils {
	return &SQLUtils{}
}

func (s *SQLUtils) GetPlaceholder() string {
	return "?"
}

func (s *SQLUtils) EscapeRelation(sb []byte, value string) []byte {
	return sqlutils.AppendEscapedRelation(sb, value, '`')
}

func (s *SQLUtils) EscapeReference(sb []byte, value string) []byte {
	return sqlutils.AppendEscapedReference(sb, value, '`')
}

func (s *SQLUtils) EscapeAliasedValue(sb []byte, value string) []byte {
	return sqlutils.AppendEscapedAliasedValue(sb, value, '`')
}

func (s *SQLUtils) GetQueryBuilderStrategy() interfaces.QueryBuilderStrategy {
	return newMySQLQueryBuilderWithUtil(s)
}

func (s *SQLUtils) Dialect() string {
	return consts.DialectMySQL
}
