package interfaces

type SQLUtils interface {
	GetPlaceholder() string
	EscapeRelation(sb []byte, value string) []byte
	EscapeReference(sb []byte, value string) []byte
	EscapeAliasedValue(sb []byte, value string) []byte
	GetQueryBuilderStrategy() QueryBuilderStrategy
	Dialect() string
}
