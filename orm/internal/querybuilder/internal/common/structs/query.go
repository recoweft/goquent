package structs

type Column struct {
	Name     string
	Raw      string
	Values   []interface{}
	Distinct bool
	Count    bool
	Function string
}

type Table struct {
	Name string
}

type Where struct {
	Column       string
	Condition    string
	Value        []interface{}
	ValueColumn  string
	ValueMap     map[string]any
	Operator     int
	Query        *Query
	Between      *WhereBetween
	Exists       *Exists
	FullText     *FullText
	JsonContains *JsonContains
	JsonLength   *JsonLength
	Raw          string
	Function     string
}

type WhereBetween struct {
	IsColumn bool
	IsNot    bool
	From     interface{}
	To       interface{}
}

type Exists struct {
	IsNot bool
	Query *Query
}

type FullText struct {
	IsNot   bool
	Columns []string
	Search  string
	Options map[string]interface{}
}

type JsonContains struct {
	Values []interface{}
}

type JsonLength struct {
	Operator string
	Value    interface{}
}

type FullTextOptions struct {
	Mode string
	With string
}

type WhereGroup struct {
	Conditions   []Where
	Operator     int
	IsDummyGroup bool
	IsNot        bool
}

type Query struct {
	Columns         *[]Column
	Table           Table
	Joins           *Joins
	ConditionGroups []WhereGroup
	Conditions      *[]Where
	Limit           Limit
	Offset          Offset
	Order           *[]Order
	Group           *GroupBy
	Lock            *Lock
}

type Union struct {
	Query *Query
	IsAll bool
}

type SelectQuery struct {
	Table   string
	Columns *[]Column
	Limit   Limit
	Offset  Offset
	Union   *[]Union
	Group   *GroupBy
	Lock    *Lock
}

type InsertQuery struct {
	Table       string
	Values      map[string]interface{}
	ValuesBatch []map[string]interface{}
	Columns     []string
	Query       *Query
	Ignore      bool
	Upsert      *Upsert
}

type Upsert struct {
	UniqueColumns []string
	UpdateColumns []string
}

type UpdateQuery struct {
	Table  string
	Values map[string]interface{}
	Query  *Query
}

type DeleteQuery struct {
	Table string
	Query *Query
}

type On struct {
	Column    string
	Condition string
	Value     interface{}
	Operator  int
}

type JoinClause struct {
	On              *[]On
	ConditionGroups *[]WhereGroup
	Conditions      *[]Where
	Name            string
	TargetNameMap   map[string]string
	Query           *Query
}

type Join struct {
	Name               string
	TargetNameMap      map[string]string
	SearchColumn       string
	SearchCondition    string
	SearchTargetColumn string
	Query              *Query
}

type Joins struct {
	Name          string
	TargetNameMap map[string]string
	Joins         *[]Join
	JoinClauses   *[]JoinClause
	LateralJoins  *[]Join
	Operator      int
	IsDummyGroup  bool
}

type Limit struct {
	Limit int64
}

type Offset struct {
	Offset int64
}

type Order struct {
	Column string
	IsAsc  bool
	Raw    string
}

type Orders struct {
	Orders *[]Order
}

type GroupBy struct {
	Columns []string
	Having  *[]Having
}

type Having struct {
	Column    string
	Condition string
	Value     interface{}
	Operator  int
	Raw       string
}

type Lock struct {
	LockType string
}
