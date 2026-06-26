package consts

const (
	Condition_EQUAL                 = "="
	Condition_NOT_EQUAL             = "!="
	Condition_GREATER_THAN          = ">"
	Condition_GREATER_THAN_OR_EQUAL = ">="
	Condition_LESS_THAN             = "<"
	Condition_LESS_THAN_OR_EQUAL    = "<="
	Condition_LIKE                  = "LIKE"
	Condition_NOT_LIKE              = "NOT LIKE"
	Condition_IN                    = "IN"
	Condition_NOT_IN                = "NOT IN"
	Condition_IS_NULL               = "IS NULL"
	Condition_IS_NOT_NULL           = "IS NOT NULL"
	Condition_BETWEEN               = "BETWEEN"
	Condition_NOT_BETWEEN           = "NOT BETWEEN"
	Condition_ANY                   = "ANY"
	Condition_ALL                   = "ALL"
	Condition_EXISTS                = "EXISTS"
	Condition_NOT_EXISTS            = "NOT EXISTS"
)

const (
	Join_INNER        = "inner"
	Join_LEFT         = "left"
	Join_RIGHT        = "right"
	Join_CROSS        = "cross"
	Join_LATERAL      = "lateral"
	Join_LEFT_LATERAL = "left_lateral"
)

const (
	Join_Type_INNER        = "INNER"
	Join_Type_LEFT         = "LEFT"
	Join_Type_RIGHT        = "RIGHT"
	Join_Type_CROSS        = "CROSS"
	Join_Type_LATERAL      = "LATERAL"
	Join_Type_LEFT_LATERAL = "LEFT LATERAL"
)

const (
	LogicalOperator_AND = iota
	LogicalOperator_OR
)

const (
	Order_ASC       = "ASC"
	Order_DESC      = "DESC"
	Order_FLAG_ASC  = true
	Order_FLAG_DESC = false
)

const (
	Lock_FOR_UPDATE = "FOR UPDATE"
	Lock_SHARE_MODE = "LOCK IN SHARE MODE"
)

const (
	StringBuffer_Short_Query_Grow  = 96
	StringBuffer_Middle_Query_Grow = 512
	StringBuffer_Long_Query_Grow   = 1024

	StringBuffer_Column_Grow  = 16
	StringBuffer_Where_Grow   = 32
	StringBuffer_Join_Grow    = 32
	StringBuffer_GroupBy_Grow = 128

	StringBuffer_Update_Grow = 128
	StringBuffer_Delete_Grow = 128
)

const (
	DialectBase       = "base"
	DialectMySQL      = "mysql"
	DialectPostgreSQL = "postgres"
)
