package query

type BaseBuilder interface {
	Build() (string, []interface{}, error)
}
