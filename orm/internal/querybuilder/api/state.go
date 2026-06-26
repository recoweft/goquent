package api

import (
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	qbquery "github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

// QuerySnapshot is a stable, detached metadata view of a SELECT builder.
type QuerySnapshot struct {
	Table      string
	Columns    []ColumnSnapshot
	Limit      int64
	Offset     int64
	Joins      []JoinSnapshot
	Predicates []PredicateSnapshot
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
	return snapshotFromQuery(qb.snapshotQuery())
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
		Table:  query.Table.Name,
		Limit:  query.Limit.Limit,
		Offset: query.Offset.Offset,
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
	appendJoinSnapshots(&snapshot, query.Joins)
	appendPredicateSnapshots(&snapshot, query.ConditionGroups)
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
