package api

import (
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	qbquery "github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/predicate"
)

// QuerySnapshot is a stable, detached metadata view of a SELECT builder.
type QuerySnapshot struct {
	AssignmentColumns []string
	WhereTree         *predicate.Node
	HavingTree        *predicate.Node
	Unverified        []string
	Error             error
	Table             string
	Columns           []ColumnSnapshot
	Limit             int64
	LimitExact        bool
	Offset            int64
	Joins             []JoinSnapshot
	Predicates        []PredicateSnapshot
}

// ColumnSnapshot describes a selected column or expression.
type ColumnSnapshot struct {
	Name       string
	Expression string
	Raw        bool
	Distinct   bool
	Count      bool
	Function   string
}

// JoinSnapshot describes a JOIN visible in the builder metadata.
type JoinSnapshot struct {
	OnTree      *predicate.Node
	Type        string
	Table       string
	Alias       string
	LeftColumn  string
	Operator    string
	RightColumn string
	Subquery    bool
}

// PredicateSnapshot describes a WHERE-like predicate visible in the builder metadata.
type PredicateSnapshot struct {
	Group       int
	Negated     bool
	Connector   string
	Column      string
	Operator    string
	ValueColumn string
	Raw         string
	Function    string
	Subquery    bool
	ValueCount  int
}

// Snapshot returns a detached metadata representation of the select query.
func (qb *SelectQueryBuilder) Snapshot() QuerySnapshot {
	src := qb.snapshotQuery()
	if err := structs.ValidateQuery(src); err != nil {
		return QuerySnapshot{Error: err}
	}
	src.WhereTree, _ = structs.InspectPredicates(src.ConditionGroups)
	src.HavingTree, _ = structs.InspectPredicates(structs.HavingGroups(src.Group))
	snapshot := snapshotFromQuery(src)
	snapshot.Unverified = append(snapshot.Unverified, "snapshot_not_rendered")
	return snapshot
}

func (qb *SelectQueryBuilder) snapshotQuery() *structs.Query {
	return structs.CloneQuery(qb.builder.GetQuery())
}

// CopyStateToSelect copies SELECT query clauses into another SELECT builder.
func (qb *SelectQueryBuilder) CopyStateToSelect(dst *SelectQueryBuilder) {
	dst.builder.ApplyQueryState(qb.snapshotQuery())
}

// CopyStateToUpdate copies SELECT query clauses that can constrain UPDATE.
func (qb *SelectQueryBuilder) CopyStateToUpdate(dst *UpdateQueryBuilder) {
	dst.builder.ApplyQueryState(qb.snapshotQuery())
}

// CopyStateToDelete copies SELECT query clauses that can constrain DELETE.
func (qb *SelectQueryBuilder) CopyStateToDelete(dst *DeleteQueryBuilder) {
	dst.builder.ApplyQueryState(qb.snapshotQuery())
}

// UseWhereBuilder points this API wrapper at a scoped WHERE builder, such as
// the builder passed to grouped predicates.
func (qb *SelectQueryBuilder) UseWhereBuilder(builder *qbquery.WhereBuilder[qbquery.SelectBuilder]) {
	qb.WhereQueryBuilder.builder = builder
}

func snapshotFromQuery(query *structs.Query) QuerySnapshot {
	if query == nil {
		return QuerySnapshot{}
	}
	snapshot := QuerySnapshot{
		WhereTree:  valuecopy.Node(query.WhereTree),
		HavingTree: valuecopy.Node(query.HavingTree),
		Table:      query.Table.Name,
		Limit:      query.Limit.Limit,
		LimitExact: query.Limit.Exact,
		Offset:     query.Offset.Offset,
	}
	if query.Columns != nil {
		for _, column := range *query.Columns {
			snapshot.Columns = append(snapshot.Columns, ColumnSnapshot{
				Name:       column.Name,
				Expression: column.Raw,
				Raw:        column.Raw != "",
				Distinct:   column.Distinct,
				Count:      column.Count,
				Function:   column.Function,
			})
		}
	}
	if query.Order != nil {
		for _, o := range *query.Order {
			if o.Raw != "" {
				snapshot.Unverified = append(snapshot.Unverified, "order_expression_not_inspected")
			}
		}
	}
	appendJoinSnapshots(&snapshot, query.Joins)
	if len(snapshot.Joins) > 0 {
		snapshot.Unverified = append(snapshot.Unverified, "join_conditions_not_inspected")
	}
	appendPredicateSnapshots(&snapshot, query.ConditionGroups)
	if len(query.Unions) > 0 {
		snapshot.Unverified = append(snapshot.Unverified, "union_branches_not_inspected")
	}
	if query.Columns != nil {
		for _, c := range *query.Columns {
			if c.Raw != "" {
				snapshot.Unverified = append(snapshot.Unverified, "selected_expression_not_inspected")
				break
			}
		}
	}
	return snapshot
}

func appendJoinSnapshots(snapshot *QuerySnapshot, joins *structs.Joins) {
	if joins == nil {
		return
	}
	if joins.JoinClauses != nil {
		for _, join := range *joins.JoinClauses {
			appendJoinSnapshot(snapshot, joinSnapshotFromClause(join, false))
		}
	}
	if joins.LateralJoins != nil {
		for _, join := range *joins.LateralJoins {
			appendJoinSnapshot(snapshot, joinSnapshotFromJoin(join, true))
		}
	}
	if joins.Joins != nil {
		for _, join := range *joins.Joins {
			appendJoinSnapshot(snapshot, joinSnapshotFromJoin(join, false))
		}
	}
}

func appendJoinSnapshot(snapshot *QuerySnapshot, join JoinSnapshot) {
	if join.Table == "" && join.Alias == "" {
		return
	}
	snapshot.Joins = append(snapshot.Joins, join)
}

func joinSnapshotFromJoin(join structs.Join, lateral bool) JoinSnapshot {
	joinType, target := joinTarget(join.TargetNameMap)
	out := JoinSnapshot{
		Type:        joinType,
		Table:       target,
		LeftColumn:  join.SearchColumn,
		Operator:    join.SearchCondition,
		RightColumn: join.SearchTargetColumn,
		Subquery:    lateral || join.Query != nil,
	}
	if out.Subquery {
		out.Alias = target
		out.Table = ""
	}
	return out
}

func joinSnapshotFromClause(join structs.JoinClause, lateral bool) JoinSnapshot {
	joinType, target := joinTarget(join.TargetNameMap)
	out := JoinSnapshot{
		Type:     joinType,
		Table:    target,
		Subquery: lateral || join.Query != nil,
	}
	if out.Subquery {
		out.Alias = target
		out.Table = ""
	}
	var children []*predicate.Node
	unsupported := false
	if join.On != nil {
		for _, on := range *join.On {
			right, ok := on.Value.(string)
			if !ok || on.Operator != 0 {
				unsupported = true
			}
			children = append(children, &predicate.Node{Kind: "column", Column: on.Column, Operator: on.Condition, ValueColumn: right, Correspondence: "generated"})
		}
	}
	if join.Conditions != nil {
		for _, c := range *join.Conditions {
			if c.Operator != 0 || len(c.Value) != 1 {
				unsupported = true
				continue
			}
			v, ok := valuecopy.Copy(c.Value[0])
			isolation := "detached"
			if !ok {
				isolation = "unsupported"
				unsupported = true
			}
			children = append(children, &predicate.Node{Kind: "comparison", Column: c.Column, Operator: c.Condition, Values: []predicate.Value{{Data: v, Isolation: isolation}}, Correspondence: "generated"})
		}
	}
	out.OnTree = &predicate.Node{Kind: "and", Children: children, Correspondence: "generated"}
	if unsupported {
		out.OnTree.OpaqueReason = "join_disjunction_or_value_unsupported"
	}
	return out
}

func joinTarget(targetMap map[string]string) (string, string) {
	if len(targetMap) == 0 {
		return "", ""
	}
	keys := make([]string, 0, len(targetMap))
	for key := range targetMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		return strings.ToUpper(strings.ReplaceAll(key, "_", " ")), targetMap[key]
	}
	return "", ""
}

func appendPredicateSnapshots(snapshot *QuerySnapshot, groups []structs.WhereGroup) {
	for i, group := range groups {
		for _, condition := range group.Conditions {
			if condition.Nested != nil {
				nested := QuerySnapshot{}
				appendPredicateSnapshots(&nested, condition.Nested)
				for _, p := range nested.Predicates {
					p.Group = i
					p.Negated = p.Negated || group.IsNot
					snapshot.Predicates = append(snapshot.Predicates, p)
				}
				continue
			}
			snapshot.Predicates = append(snapshot.Predicates, PredicateSnapshot{
				Group:       i,
				Negated:     group.IsNot,
				Connector:   logicalOperator(condition.Operator),
				Column:      condition.Column,
				Operator:    condition.Condition,
				ValueColumn: condition.ValueColumn,
				Raw:         condition.Raw,
				Function:    condition.Function,
				Subquery:    condition.Query != nil || condition.Exists != nil,
				ValueCount:  valueCount(condition),
			})
		}
	}
}

func valueCount(condition structs.Where) int {
	switch {
	case len(condition.Value) > 0:
		return len(condition.Value)
	case len(condition.ValueMap) > 0:
		return len(condition.ValueMap)
	case condition.Between != nil:
		return 2
	case condition.FullText != nil:
		return 1
	case condition.JsonContains != nil:
		return len(condition.JsonContains.Values)
	case condition.JsonLength != nil:
		return 1
	default:
		return 0
	}
}

func logicalOperator(op int) string {
	if op == 1 {
		return "OR"
	}
	return "AND"
}

// BuildSnapshot captures SQL, parameters and inspection from one frozen build.
func (qb *SelectQueryBuilder) BuildSnapshot() (string, []any, QuerySnapshot, error) {
	sql, args, err := qb.Build()
	if err != nil {
		return "", nil, QuerySnapshot{Error: err}, err
	}
	snapshot := snapshotFromQuery(qb.builder.BuiltQuery)
	return sql, args, snapshot, nil
}
func (qb *UpdateQueryBuilder) BuildSnapshot() (string, []any, QuerySnapshot, error) {
	sql, args, err := qb.Build()
	if err != nil {
		return "", nil, QuerySnapshot{Error: err}, err
	}
	built := qb.builder.BuiltStatement
	snapshot := snapshotFromQuery(built.Query)
	snapshot.Table = built.Table
	for col := range built.Values {
		snapshot.AssignmentColumns = append(snapshot.AssignmentColumns, col)
	}
	for _, a := range built.Options.Assignments {
		snapshot.AssignmentColumns = append(snapshot.AssignmentColumns, a.Column)
		snapshot.Unverified = append(snapshot.Unverified, "assignment_expression")
	}
	sort.Strings(snapshot.AssignmentColumns)
	return sql, args, snapshot, nil
}
func (qb *DeleteQueryBuilder) BuildSnapshot() (string, []any, QuerySnapshot, error) {
	sql, args, err := qb.Build()
	if err != nil {
		return "", nil, QuerySnapshot{Error: err}, err
	}
	built := qb.builder.BuiltStatement
	snapshot := snapshotFromQuery(built.Query)
	snapshot.Table = built.Table
	return sql, args, snapshot, nil
}

// GroupWhere wraps the complete predicate list before appending mandatory ANDs.
// Call on a detached builder so planning never alters caller conditions.
func (qb *SelectQueryBuilder) GroupWhere() {
	q := qb.snapshotQuery()
	if len(q.ConditionGroups) > 0 {
		q.ConditionGroups = []structs.WhereGroup{{Conditions: []structs.Where{{Nested: q.ConditionGroups}}}}
	}
	qb.builder.ApplyQueryState(q)
}
