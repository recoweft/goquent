package query

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"

	qbapi "github.com/faciam-dev/goquent-query-builder/api"
	qbmysql "github.com/faciam-dev/goquent-query-builder/database/mysql"
	qbpostgres "github.com/faciam-dev/goquent-query-builder/database/postgres"
	"github.com/faciam-dev/goquent/orm/conv"
	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/scanner"
)

// Query wraps goquent QueryBuilder and the Driver.
// executor abstracts sql.DB and sql.Tx.
type executor interface {
	// Query executes a statement returning multiple rows.
	Query(query string, args ...any) (*sql.Rows, error)
	// QueryContext is the context-aware form of Query.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	// QueryRow executes a single-row query.
	QueryRow(query string, args ...any) *sql.Row
	// QueryRowContext executes QueryRow with a context.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	// Exec runs a statement that does not return rows.
	Exec(query string, args ...any) (sql.Result, error)
	// ExecContext runs Exec with a context.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Query wraps goquent QueryBuilder and the executor.
type Query struct {
	builder       *qbapi.SelectQueryBuilder
	exec          executor
	ctx           context.Context
	err           error
	dialect       driver.Dialect
	paramSeq      int
	primaryKey    string
	approval      *Approval
	suppressions  []Suppression
	policy        *TablePolicy
	accessReason  string
	withDeleted   bool
	onlyDeleted   bool
	policyApplied bool
}

// CursorColumn describes an ordered column used by keyset cursor predicates.
type CursorColumn struct {
	Name      string
	Direction string
	Raw       bool
}

// CursorAsc returns an ascending keyset cursor column.
func CursorAsc(name string) CursorColumn {
	return CursorColumn{Name: name, Direction: "asc"}
}

// CursorDesc returns a descending keyset cursor column.
func CursorDesc(name string) CursorColumn {
	return CursorColumn{Name: name, Direction: "desc"}
}

// CursorAscExpr returns an ascending trusted SQL expression for keyset cursor
// predicates.
func CursorAscExpr(expr string) CursorColumn {
	return CursorColumn{Name: expr, Direction: "asc", Raw: true}
}

// CursorDescExpr returns a descending trusted SQL expression for keyset cursor
// predicates.
func CursorDescExpr(expr string) CursorColumn {
	return CursorColumn{Name: expr, Direction: "desc", Raw: true}
}

// CursorAscAlias returns an ascending selected alias cursor column.
func CursorAscAlias(alias string) CursorColumn { return CursorAsc(alias) }

// CursorDescAlias returns a descending selected alias cursor column.
func CursorDescAlias(alias string) CursorColumn { return CursorDesc(alias) }

// New creates a Query with given db and table.
func New(exec executor, table string, dialect driver.Dialect) *Query {
	builder := newSelectBuilder(dialect)
	builder.Table(table)
	q := &Query{builder: builder, exec: exec, dialect: dialect, primaryKey: "id"}
	if policy, ok := PolicyForTable(table); ok {
		q.policy = &policy
	}
	return q
}

func builderByDialect[T any](d driver.Dialect, mysqlFn, pgFn func() T) T {
	if _, ok := d.(driver.PostgresDialect); ok {
		return pgFn()
	}
	return mysqlFn()
}

func newSelectBuilder(d driver.Dialect) *qbapi.SelectQueryBuilder {
	return builderByDialect(d,
		func() *qbapi.SelectQueryBuilder { return qbapi.NewSelectQueryBuilder(qbmysql.NewMySQLQueryBuilder()) },
		func() *qbapi.SelectQueryBuilder {
			return qbapi.NewSelectQueryBuilder(qbpostgres.NewPostgreSQLQueryBuilder())
		},
	)
}

func newInsertBuilder(d driver.Dialect) *qbapi.InsertQueryBuilder {
	return builderByDialect(d,
		func() *qbapi.InsertQueryBuilder { return qbapi.NewInsertQueryBuilder(qbmysql.NewMySQLQueryBuilder()) },
		func() *qbapi.InsertQueryBuilder {
			return qbapi.NewInsertQueryBuilder(qbpostgres.NewPostgreSQLQueryBuilder())
		},
	)
}

func newUpdateBuilder(d driver.Dialect) *qbapi.UpdateQueryBuilder {
	return builderByDialect(d,
		func() *qbapi.UpdateQueryBuilder { return qbapi.NewUpdateQueryBuilder(qbmysql.NewMySQLQueryBuilder()) },
		func() *qbapi.UpdateQueryBuilder {
			return qbapi.NewUpdateQueryBuilder(qbpostgres.NewPostgreSQLQueryBuilder())
		},
	)
}

func newDeleteBuilder(d driver.Dialect) *qbapi.DeleteQueryBuilder {
	return builderByDialect(d,
		func() *qbapi.DeleteQueryBuilder { return qbapi.NewDeleteQueryBuilder(qbmysql.NewMySQLQueryBuilder()) },
		func() *qbapi.DeleteQueryBuilder {
			return qbapi.NewDeleteQueryBuilder(qbpostgres.NewPostgreSQLQueryBuilder())
		},
	)
}

// PrimaryKey sets the primary key column for the table.
func (q *Query) PrimaryKey(col string) *Query {
	q.primaryKey = col
	return q
}

func (q *Query) getPrimaryKeyColumn() string {
	if q.primaryKey != "" {
		return q.primaryKey
	}
	return "id"
}

// WithContext sets ctx on the query for context-aware execution.
func (q *Query) WithContext(ctx context.Context) *Query {
	q.ctx = ctx
	return q
}

// RequireApproval records an explicit reason for executing a risky query.
func (q *Query) RequireApproval(reason string) *Query {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		q.err = ErrApprovalReasonRequired
		return q
	}
	q.approval = &Approval{Reason: reason, CreatedAt: time.Now().UTC()}
	return q
}

// SuppressWarning suppresses a suppressible warning for this query plan.
func (q *Query) SuppressWarning(code, reason string, opts ...SuppressionOption) *Query {
	s, err := NewSuppression(code, reason, opts...)
	if err != nil {
		q.err = err
		return q
	}
	q.suppressions = append(q.suppressions, s)
	return q
}

// AccessReason records why this query needs access to sensitive columns.
func (q *Query) AccessReason(reason string) *Query {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		q.err = ErrAccessReasonRequired
		return q
	}
	q.accessReason = reason
	return q
}

// WithDeleted disables the default soft-delete filter for a policy table.
func (q *Query) WithDeleted() *Query {
	q.withDeleted = true
	q.onlyDeleted = false
	return q
}

// OnlyDeleted restricts a soft-delete policy table to deleted rows.
func (q *Query) OnlyDeleted() *Query {
	q.onlyDeleted = true
	q.withDeleted = false
	return q
}

func (q *Query) finalizePlan(plan *QueryPlan) {
	if plan == nil {
		return
	}
	q.applyPolicyMetadata(plan)
	finalizePlanWithPolicy(plan, q.approval, q.suppressions, q.policy)
}

func (q *Query) applyPolicyPredicates() {
	if q.policyApplied || q.policy == nil || q.policy.SoftDeleteColumn == "" {
		return
	}
	switch {
	case q.onlyDeleted:
		q.builder.WhereNotNull(q.policy.SoftDeleteColumn)
		q.policyApplied = true
	case q.withDeleted:
		q.policyApplied = true
	default:
		q.builder.WhereNull(q.policy.SoftDeleteColumn)
		q.policyApplied = true
	}
}

func (q *Query) applyPolicyMetadata(plan *QueryPlan) {
	if plan.Metadata == nil {
		plan.Metadata = make(map[string]any)
	}
	if q.accessReason != "" {
		plan.Metadata["access_reason"] = q.accessReason
	}
	if q.policy == nil {
		return
	}
	plan.Metadata["policy_table"] = q.policy.Table
	if q.withDeleted {
		plan.Metadata["soft_delete"] = "with_deleted"
	} else if q.onlyDeleted {
		plan.Metadata["soft_delete"] = "only_deleted"
	} else if q.policy.SoftDeleteColumn != "" {
		plan.Metadata["soft_delete"] = "default"
	}
}

// queryRows executes Query or QueryContext based on whether ctx is set.
func (q *Query) queryRows(sqlStr string, args ...any) (*sql.Rows, error) {
	if q.ctx != nil {
		return q.exec.QueryContext(q.ctx, sqlStr, args...)
	}
	return q.exec.Query(sqlStr, args...)
}

func (q *Query) queryRow(sqlStr string, args ...any) *sql.Row {
	if q.ctx != nil {
		return q.exec.QueryRowContext(q.ctx, sqlStr, args...)
	}
	return q.exec.QueryRow(sqlStr, args...)
}

func (q *Query) nextParamName(prefix string) string {
	q.paramSeq++
	return fmt.Sprintf("__goquent_%s_%d", prefix, q.paramSeq)
}

// execStmt executes Exec or ExecContext depending on ctx.
func (q *Query) execStmt(sqlStr string, args ...any) (sql.Result, error) {
	if q.ctx != nil {
		return q.exec.ExecContext(q.ctx, sqlStr, args...)
	}
	return q.exec.Exec(sqlStr, args...)
}

// Select sets selected identifier columns. Use SelectRaw for SQL expressions.
func (q *Query) Select(cols ...string) *Query {
	if q.err != nil {
		return q
	}
	for _, col := range cols {
		if err := validateSelectColumn(col); err != nil {
			q.err = err
			return q
		}
	}
	q.builder.Select(cols...)
	return q
}

// Where appends a column/value comparison.
// Values are always treated as literals. Use WhereColumn for
// column-to-column comparisons.
func (q *Query) Where(col string, args ...any) *Query {
	if q.err != nil {
		return q
	}
	switch len(args) {
	case 1:
		q.builder.Where(col, "=", args[0])
	case 2:
		op, ok := args[0].(string)
		if !ok {
			q.err = fmt.Errorf("invalid operator type")
			return q
		}
		op, err := validateConditionOperator(op)
		if err != nil {
			q.err = err
			return q
		}
		q.builder.Where(col, op, args[1])
	default:
		q.err = fmt.Errorf("invalid Where usage")
	}
	return q
}

// WhereCursorAfter adds a keyset pagination predicate after the given cursor.
func (q *Query) WhereCursorAfter(columns []CursorColumn, values ...any) *Query {
	return q.whereCursor(columns, values, true)
}

// WhereCursorBefore adds a keyset pagination predicate before the given cursor.
func (q *Query) WhereCursorBefore(columns []CursorColumn, values ...any) *Query {
	return q.whereCursor(columns, values, false)
}

func (q *Query) whereCursor(columns []CursorColumn, values []any, after bool) *Query {
	if q.err != nil {
		return q
	}
	normalized, err := validateCursorColumns(columns, values)
	if err != nil {
		q.err = err
		return q
	}
	raw, vals := q.cursorPredicate(normalized, values, after)
	q.builder.WhereRaw(raw, vals)
	return q
}

func (q *Query) cursorPredicate(columns []CursorColumn, values []any, after bool) (string, map[string]any) {
	vals := make(map[string]any)
	parts := make([]string, 0, len(columns))
	for i := range columns {
		comparisons := make([]string, 0, i+1)
		for j := 0; j < i; j++ {
			name := fmt.Sprintf("__goquent_cursor_%d_%d", i, j)
			vals[name] = values[j]
			comparisons = append(comparisons, fmt.Sprintf("%s = :%s", q.cursorColumnSQL(columns[j]), name))
		}
		name := fmt.Sprintf("__goquent_cursor_%d_%d", i, i)
		vals[name] = values[i]
		comparisons = append(comparisons, fmt.Sprintf("%s %s :%s", q.cursorColumnSQL(columns[i]), cursorComparisonOperator(columns[i].Direction, after), name))
		parts = append(parts, "("+strings.Join(comparisons, " AND ")+")")
	}
	return "(" + strings.Join(parts, " OR ") + ")", vals
}

func (q *Query) cursorColumnSQL(column CursorColumn) string {
	if column.Raw {
		return column.Name
	}
	return quoteIdentifierPath(q.dialect, column.Name)
}

// First scans the first result into dest struct.
func (q *Query) First(dest any) error {
	plan, err := q.Plan(q.ctx)
	if err != nil {
		return err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return err
	}
	rows, err := q.queryRows(plan.SQL, plan.Params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanner.Struct(dest, rows)
}

// FirstMap scans first row into map.
func (q *Query) FirstMap(dest *map[string]any) error {
	plan, err := q.Plan(q.ctx)
	if err != nil {
		return err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return err
	}
	rows, err := q.queryRows(plan.SQL, plan.Params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	m, err := scanner.Map(rows)
	if err != nil {
		return err
	}
	*dest = m
	return nil
}

// GetMaps scans all rows into slice of maps.
func (q *Query) GetMaps(dest *[]map[string]any) error {
	plan, err := q.Plan(q.ctx)
	if err != nil {
		return err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return err
	}
	rows, err := q.queryRows(plan.SQL, plan.Params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	m, err := scanner.Maps(rows)
	if err != nil {
		return err
	}
	*dest = m
	return nil
}

// Get scans all rows into the slice pointed to by dest.
func (q *Query) Get(dest any) error {
	plan, err := q.Plan(q.ctx)
	if err != nil {
		return err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return err
	}
	rows, err := q.queryRows(plan.SQL, plan.Params...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanner.Structs(dest, rows)
}

// Limit sets a limit.
func (q *Query) Limit(n int) *Query {
	q.builder.Limit(int64(n))
	return q
}

// Offset sets offset.
func (q *Query) Offset(n int) *Query {
	q.builder.Offset(int64(n))
	return q
}

// SelectRaw adds a raw select expression.
func (q *Query) SelectRaw(raw string, values ...any) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.SelectRaw(raw, values...)
	return q
}

// Count executes a COUNT query using the current conditions and returns the
// resulting row count.
func (q *Query) Count(cols ...string) (int64, error) {
	if q.err != nil {
		return 0, q.err
	}
	q.applyPolicyPredicates()

	b := newSelectBuilder(q.dialect)
	b.Table(q.builder.GetQuery().Table.Name)
	if err := copySelectBuilderState(q.builder, b); err != nil {
		return 0, err
	}
	b.Count(cols...)

	plan, err := q.planSelectBuilder(q.ctx, b)
	if err != nil {
		return 0, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return 0, err
	}

	var row *sql.Row
	if q.ctx != nil {
		row = q.exec.QueryRowContext(q.ctx, plan.SQL, plan.Params...)
	} else {
		row = q.exec.QueryRow(plan.SQL, plan.Params...)
	}
	var c int64
	if err := row.Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

// Distinct marks columns as DISTINCT.
func (q *Query) Distinct(cols ...string) *Query {
	q.builder.Distinct(cols...)
	return q
}

// Union adds a UNION with another query.
func (q *Query) Union(sub *Query) *Query {
	q.builder.Union(sub.builder)
	return q
}

// UnionAll adds a UNION ALL with another query.
func (q *Query) UnionAll(sub *Query) *Query {
	q.builder.UnionAll(sub.builder)
	return q
}

// Max adds MAX aggregate function.
func (q *Query) Max(col string) *Query { q.builder.Max(col); return q }

// Min adds MIN aggregate function.
func (q *Query) Min(col string) *Query { q.builder.Min(col); return q }

// Sum adds SUM aggregate function.
func (q *Query) Sum(col string) *Query { q.builder.Sum(col); return q }

// Avg adds AVG aggregate function.
func (q *Query) Avg(col string) *Query { q.builder.Avg(col); return q }

// Join adds INNER JOIN clause.
func (q *Query) Join(table, localColumn, cond, target string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.Join(table, localColumn, cond, target)
	return q
}

// JoinQuery adds a JOIN with additional ON/WHERE clauses defined in the callback.
func (q *Query) JoinQuery(table string, fn func(b *qbapi.JoinClauseQueryBuilder)) *Query {
	q.builder.JoinQuery(table, func(b *qbapi.JoinClauseQueryBuilder) { fn(b) })
	return q
}

// LeftJoinQuery adds a LEFT JOIN with additional clauses defined in the callback.
func (q *Query) LeftJoinQuery(table string, fn func(b *qbapi.JoinClauseQueryBuilder)) *Query {
	q.builder.LeftJoinQuery(table, func(b *qbapi.JoinClauseQueryBuilder) { fn(b) })
	return q
}

// RightJoinQuery adds a RIGHT JOIN with additional clauses defined in the callback.
func (q *Query) RightJoinQuery(table string, fn func(b *qbapi.JoinClauseQueryBuilder)) *Query {
	q.builder.RightJoinQuery(table, func(b *qbapi.JoinClauseQueryBuilder) { fn(b) })
	return q
}

// JoinSubQuery joins a subquery with alias and join condition.
func (q *Query) JoinSubQuery(sub *Query, alias, my, condition, target string) *Query {
	if q.err != nil {
		return q
	}
	condition, err := validateConditionOperator(condition)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.JoinSubQuery(sub.builder, alias, my, condition, target)
	return q
}

// LeftJoinSubQuery performs a LEFT JOIN using a subquery.
func (q *Query) LeftJoinSubQuery(sub *Query, alias, my, condition, target string) *Query {
	if q.err != nil {
		return q
	}
	condition, err := validateConditionOperator(condition)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.LeftJoinSubQuery(sub.builder, alias, my, condition, target)
	return q
}

// RightJoinSubQuery performs a RIGHT JOIN using a subquery.
func (q *Query) RightJoinSubQuery(sub *Query, alias, my, condition, target string) *Query {
	if q.err != nil {
		return q
	}
	condition, err := validateConditionOperator(condition)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.RightJoinSubQuery(sub.builder, alias, my, condition, target)
	return q
}

// JoinLateral performs a LATERAL JOIN using a subquery.
func (q *Query) JoinLateral(sub *Query, alias string) *Query {
	q.builder.JoinLateral(sub.builder, alias)
	return q
}

// LeftJoinLateral performs a LEFT LATERAL JOIN using a subquery.
func (q *Query) LeftJoinLateral(sub *Query, alias string) *Query {
	q.builder.LeftJoinLateral(sub.builder, alias)
	return q
}

// LeftJoin adds LEFT JOIN clause.
func (q *Query) LeftJoin(table, localColumn, cond, target string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.LeftJoin(table, localColumn, cond, target)
	return q
}

// RightJoin adds RIGHT JOIN clause.
func (q *Query) RightJoin(table, localColumn, cond, target string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.RightJoin(table, localColumn, cond, target)
	return q
}

// CrossJoin adds CROSS JOIN clause.
func (q *Query) CrossJoin(table string) *Query {
	q.builder.CrossJoin(table)
	return q
}

// OrderBy adds ORDER BY clause.
func (q *Query) OrderBy(col, dir string) *Query {
	if q.err != nil {
		return q
	}
	dir, err := validateOrderDirection(dir)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrderBy(col, dir)
	return q
}

// OrderByRaw adds raw ORDER BY clause.
func (q *Query) OrderByRaw(raw string) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.OrderByRaw(raw)
	return q
}

// ReOrder clears ORDER BY clauses.
func (q *Query) ReOrder() *Query {
	q.builder.ReOrder()
	return q
}

// GroupBy adds GROUP BY clause.
func (q *Query) GroupBy(cols ...string) *Query {
	q.builder.GroupBy(cols...)
	return q
}

// Having adds HAVING condition.
func (q *Query) Having(col, cond string, val any) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.Having(col, cond, val)
	return q
}

// HavingRaw adds raw HAVING condition.
func (q *Query) HavingRaw(raw string) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.HavingRaw(raw)
	return q
}

// OrHaving adds OR HAVING condition.
func (q *Query) OrHaving(col, cond string, val any) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrHaving(col, cond, val)
	return q
}

// OrHavingRaw adds raw OR HAVING condition.
func (q *Query) OrHavingRaw(raw string) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.OrHavingRaw(raw)
	return q
}

// OrWhere appends OR condition.
func (q *Query) OrWhere(col string, args ...any) *Query {
	if q.err != nil {
		return q
	}
	switch len(args) {
	case 1:
		q.builder.OrWhere(col, "=", args[0])
	case 2:
		op, ok := args[0].(string)
		if !ok {
			q.err = fmt.Errorf("invalid operator type")
			return q
		}
		op, err := validateConditionOperator(op)
		if err != nil {
			q.err = err
			return q
		}
		q.builder.OrWhere(col, op, args[1])
	default:
		q.err = fmt.Errorf("invalid OrWhere usage")
	}
	return q
}

// WhereRaw appends raw WHERE condition.
func (q *Query) WhereRaw(raw string, vals map[string]any) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.WhereRaw(raw, vals)
	return q
}

// WhereRawNoArgs appends a raw WHERE condition that has no placeholders.
func (q *Query) WhereRawNoArgs(raw string) *Query {
	return q.WhereRaw(raw, map[string]any{})
}

// WhereJSONText adds a PostgreSQL JSONB text equality predicate:
// column ->> key = value.
func (q *Query) WhereJSONText(column, key string, value any) *Query {
	if q.err != nil {
		return q
	}
	if _, ok := q.dialect.(driver.PostgresDialect); !ok {
		q.err = fmt.Errorf("goquent: JSONB predicates are only supported on PostgreSQL")
		return q
	}
	column = strings.TrimSpace(column)
	if column == "" {
		q.err = fmt.Errorf("goquent: JSONB column is required")
		return q
	}
	if key == "" {
		q.err = fmt.Errorf("goquent: JSONB key is required")
		return q
	}
	columnSQL := quoteIdentifierPath(q.dialect, column)
	keyName := q.nextParamName("json_key")
	valueName := q.nextParamName("json_value")
	return q.WhereRaw(
		fmt.Sprintf("%s ->> :%s = :%s", columnSQL, keyName, valueName),
		map[string]any{keyName: key, valueName: value},
	)
}

// WhereJSONHasKey adds a PostgreSQL JSONB key-existence predicate:
// column ? key.
func (q *Query) WhereJSONHasKey(column, key string) *Query {
	return q.whereJSONHasKey(column, key, false)
}

// WhereJSONNotHasKey adds a negated PostgreSQL JSONB key-existence predicate:
// NOT (column ? key).
func (q *Query) WhereJSONNotHasKey(column, key string) *Query {
	return q.whereJSONHasKey(column, key, true)
}

func (q *Query) whereJSONHasKey(column, key string, negated bool) *Query {
	if q.err != nil {
		return q
	}
	if _, ok := q.dialect.(driver.PostgresDialect); !ok {
		q.err = fmt.Errorf("goquent: JSONB predicates are only supported on PostgreSQL")
		return q
	}
	column = strings.TrimSpace(column)
	if column == "" {
		q.err = fmt.Errorf("goquent: JSONB column is required")
		return q
	}
	if key == "" {
		q.err = fmt.Errorf("goquent: JSONB key is required")
		return q
	}
	columnSQL := quoteIdentifierPath(q.dialect, column)
	keyName := q.nextParamName("json_key")
	raw := fmt.Sprintf("%s ? :%s", columnSQL, keyName)
	if negated {
		raw = "NOT (" + raw + ")"
	}
	return q.WhereRaw(raw, map[string]any{keyName: key})
}

// OrWhereRaw appends raw OR WHERE condition.
func (q *Query) OrWhereRaw(raw string, vals map[string]any) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereRaw(raw, vals)
	return q
}

// OrWhereRawNoArgs appends a raw OR WHERE condition that has no placeholders.
func (q *Query) OrWhereRawNoArgs(raw string) *Query {
	return q.OrWhereRaw(raw, map[string]any{})
}

// SafeWhereRaw appends a raw WHERE condition ensuring a values map is always used.
func (q *Query) SafeWhereRaw(raw string, vals map[string]any) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.SafeWhereRaw(raw, vals)
	return q
}

// SafeOrWhereRaw appends a raw OR WHERE condition ensuring a values map is used.
func (q *Query) SafeOrWhereRaw(raw string, vals map[string]any) *Query {
	if q.err != nil {
		return q
	}
	if err := validateRawSQLFragment(raw); err != nil {
		q.err = err
		return q
	}
	q.builder.SafeOrWhereRaw(raw, vals)
	return q
}

// WhereGroup groups conditions with parentheses using AND logic.
func (q *Query) WhereGroup(fn func(g *Query)) *Query {
	if q.err != nil {
		return q
	}
	q.builder.WhereGroup(func(b *qbapi.WhereSelectQueryBuilder) {
		grp := &Query{builder: q.builder, exec: q.exec, ctx: q.ctx, dialect: q.dialect, paramSeq: q.paramSeq}
		_ = setFieldValue(reflect.ValueOf(&grp.builder.WhereQueryBuilder), "builder", reflect.ValueOf(b.GetBuilder()))
		fn(grp)
		q.paramSeq = grp.paramSeq
		if grp.err != nil {
			q.err = grp.err
		}
	})
	return q
}

// OrWhereGroup groups conditions with parentheses using OR logic.
func (q *Query) OrWhereGroup(fn func(g *Query)) *Query {
	if q.err != nil {
		return q
	}
	q.builder.OrWhereGroup(func(b *qbapi.WhereSelectQueryBuilder) {
		grp := &Query{builder: q.builder, exec: q.exec, ctx: q.ctx, dialect: q.dialect, paramSeq: q.paramSeq}
		_ = setFieldValue(reflect.ValueOf(&grp.builder.WhereQueryBuilder), "builder", reflect.ValueOf(b.GetBuilder()))
		fn(grp)
		q.paramSeq = grp.paramSeq
		if grp.err != nil {
			q.err = grp.err
		}
	})
	return q
}

// WhereNot groups conditions inside NOT (...).
func (q *Query) WhereNot(fn func(g *Query)) *Query {
	if q.err != nil {
		return q
	}
	q.builder.WhereNot(func(b *qbapi.WhereSelectQueryBuilder) {
		grp := &Query{builder: q.builder, exec: q.exec, ctx: q.ctx, dialect: q.dialect, paramSeq: q.paramSeq}
		_ = setFieldValue(reflect.ValueOf(&grp.builder.WhereQueryBuilder), "builder", reflect.ValueOf(b.GetBuilder()))
		fn(grp)
		q.paramSeq = grp.paramSeq
		if grp.err != nil {
			q.err = grp.err
		}
	})
	return q
}

// OrWhereNot groups conditions inside OR NOT (...).
func (q *Query) OrWhereNot(fn func(g *Query)) *Query {
	if q.err != nil {
		return q
	}
	q.builder.OrWhereNot(func(b *qbapi.WhereSelectQueryBuilder) {
		grp := &Query{builder: q.builder, exec: q.exec, ctx: q.ctx, dialect: q.dialect, paramSeq: q.paramSeq}
		_ = setFieldValue(reflect.ValueOf(&grp.builder.WhereQueryBuilder), "builder", reflect.ValueOf(b.GetBuilder()))
		fn(grp)
		q.paramSeq = grp.paramSeq
		if grp.err != nil {
			q.err = grp.err
		}
	})
	return q
}

// WhereIn adds WHERE IN condition.
func (q *Query) WhereIn(col string, vals any) *Query {
	q.builder.WhereIn(col, vals)
	return q
}

// WhereNotIn adds WHERE NOT IN condition.
func (q *Query) WhereNotIn(col string, vals any) *Query {
	q.builder.WhereNotIn(col, vals)
	return q
}

// OrWhereIn adds OR WHERE IN condition.
func (q *Query) OrWhereIn(col string, vals any) *Query {
	q.builder.OrWhereIn(col, vals)
	return q
}

// OrWhereNotIn adds OR WHERE NOT IN condition.
func (q *Query) OrWhereNotIn(col string, vals any) *Query {
	q.builder.OrWhereNotIn(col, vals)
	return q
}

// WhereInSubQuery adds WHERE IN (subquery) condition.
func (q *Query) WhereInSubQuery(col string, sub *Query) *Query {
	q.builder.WhereInSubQuery(col, sub.builder)
	return q
}

// WhereNotInSubQuery adds WHERE NOT IN (subquery) condition.
func (q *Query) WhereNotInSubQuery(col string, sub *Query) *Query {
	q.builder.WhereNotInSubQuery(col, sub.builder)
	return q
}

// OrWhereInSubQuery adds OR WHERE IN (subquery) condition.
func (q *Query) OrWhereInSubQuery(col string, sub *Query) *Query {
	q.builder.OrWhereInSubQuery(col, sub.builder)
	return q
}

// OrWhereNotInSubQuery adds OR WHERE NOT IN (subquery) condition.
func (q *Query) OrWhereNotInSubQuery(col string, sub *Query) *Query {
	q.builder.OrWhereNotInSubQuery(col, sub.builder)
	return q
}

// WhereAny adds grouped OR conditions across columns.
func (q *Query) WhereAny(cols []string, cond string, val any) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereAny(cols, cond, val)
	return q
}

// WhereAll adds grouped AND conditions across columns.
func (q *Query) WhereAll(cols []string, cond string, val any) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereAll(cols, cond, val)
	return q
}

// WhereColumn adds WHERE column operator column condition.
func (q *Query) WhereColumn(col string, args ...string) *Query {
	var op, other string
	switch len(args) {
	case 1:
		op = "="
		other = args[0]
	case 2:
		op = args[0]
		other = args[1]
	default:
		q.err = fmt.Errorf("invalid WhereColumn usage")
		return q
	}
	var err error
	op, err = validateConditionOperator(op)
	if err != nil {
		q.err = err
		return q
	}
	columnsPair := []string{col, other}
	q.builder.WhereColumn(columnsPair, col, op, other)
	return q
}

// OrWhereColumn adds OR WHERE column operator column condition.
func (q *Query) OrWhereColumn(col string, args ...string) *Query {
	var op, other string
	switch len(args) {
	case 1:
		op = "="
		other = args[0]
	case 2:
		op = args[0]
		other = args[1]
	default:
		q.err = fmt.Errorf("invalid OrWhereColumn usage")
		return q
	}
	var err error
	op, err = validateConditionOperator(op)
	if err != nil {
		q.err = err
		return q
	}
	columnsPair := []string{col, other}
	q.builder.OrWhereColumn(columnsPair, col, op, other)
	return q
}

// WhereColumns adds multiple column comparison conditions joined by AND.
func (q *Query) WhereColumns(columns [][]string) *Query {
	if q.err != nil {
		return q
	}
	all, err := gatherColumns(columns)
	if err != nil {
		q.err = err
		return q
	}
	if err := validateColumnComparisons(columns); err != nil {
		q.err = err
		return q
	}
	q.builder.WhereColumns(all, columns)
	return q
}

// OrWhereColumns adds multiple column comparison conditions joined by OR.
func (q *Query) OrWhereColumns(columns [][]string) *Query {
	if q.err != nil {
		return q
	}
	all, err := gatherColumns(columns)
	if err != nil {
		q.err = err
		return q
	}
	if err := validateColumnComparisons(columns); err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereColumns(all, columns)
	return q
}

// WhereNull adds WHERE column IS NULL condition.
func (q *Query) WhereNull(col string) *Query {
	q.builder.WhereNull(col)
	return q
}

// WhereNotNull adds WHERE column IS NOT NULL condition.
func (q *Query) WhereNotNull(col string) *Query {
	q.builder.WhereNotNull(col)
	return q
}

// OrWhereNull adds OR WHERE column IS NULL condition.
func (q *Query) OrWhereNull(col string) *Query {
	q.builder.OrWhereNull(col)
	return q
}

// OrWhereNotNull adds OR WHERE column IS NOT NULL condition.
func (q *Query) OrWhereNotNull(col string) *Query {
	q.builder.OrWhereNotNull(col)
	return q
}

// WhereBetween adds WHERE BETWEEN condition.
func (q *Query) WhereBetween(col string, min, max any) *Query {
	q.builder.WhereBetween(col, min, max)
	return q
}

// WhereNotBetween adds WHERE NOT BETWEEN condition.
func (q *Query) WhereNotBetween(col string, min, max any) *Query {
	q.builder.WhereNotBetween(col, min, max)
	return q
}

// OrWhereBetween adds OR WHERE BETWEEN condition.
func (q *Query) OrWhereBetween(col string, min, max any) *Query {
	q.builder.OrWhereBetween(col, min, max)
	return q
}

// OrWhereNotBetween adds OR WHERE NOT BETWEEN condition.
func (q *Query) OrWhereNotBetween(col string, min, max any) *Query {
	q.builder.OrWhereNotBetween(col, min, max)
	return q
}

// WhereBetweenColumns adds WHERE col BETWEEN minCol AND maxCol using columns.
func (q *Query) WhereBetweenColumns(col, minCol, maxCol string) *Query {
	cols := []string{col, minCol, maxCol}
	q.builder.WhereBetweenColumns(cols, col, minCol, maxCol)
	return q
}

// OrWhereBetweenColumns adds OR WHERE col BETWEEN minCol AND maxCol using columns.
func (q *Query) OrWhereBetweenColumns(col, minCol, maxCol string) *Query {
	cols := []string{col, minCol, maxCol}
	q.builder.OrWhereBetweenColumns(cols, col, minCol, maxCol)
	return q
}

// WhereNotBetweenColumns adds WHERE col NOT BETWEEN minCol AND maxCol using columns.
func (q *Query) WhereNotBetweenColumns(col, minCol, maxCol string) *Query {
	cols := []string{col, minCol, maxCol}
	q.builder.WhereNotBetweenColumns(cols, col, minCol, maxCol)
	return q
}

// OrWhereNotBetweenColumns adds OR WHERE col NOT BETWEEN minCol AND maxCol using columns.
func (q *Query) OrWhereNotBetweenColumns(col, minCol, maxCol string) *Query {
	cols := []string{col, minCol, maxCol}
	q.builder.OrWhereNotBetweenColumns(cols, col, minCol, maxCol)
	return q
}

// WhereFullText adds full-text search condition.
func (q *Query) WhereFullText(cols []string, search string, opts map[string]any) *Query {
	q.builder.WhereFullText(cols, search, opts)
	return q
}

// OrWhereFullText adds OR full-text search condition.
func (q *Query) OrWhereFullText(cols []string, search string, opts map[string]any) *Query {
	q.builder.OrWhereFullText(cols, search, opts)
	return q
}

// WhereExists adds WHERE EXISTS (subquery) condition.
func (q *Query) WhereExists(sub *Query) *Query {
	q.builder.WhereExistsSubQuery(sub.builder)
	return q
}

// OrWhereExists adds OR WHERE EXISTS (subquery) condition.
func (q *Query) OrWhereExists(sub *Query) *Query {
	q.builder.OrWhereExistsSubQuery(sub.builder)
	return q
}

// WhereNotExists adds WHERE NOT EXISTS (subquery) condition.
func (q *Query) WhereNotExists(sub *Query) *Query {
	q.builder.WhereNotExistsQuery(sub.builder)
	return q
}

// OrWhereNotExists adds OR WHERE NOT EXISTS (subquery) condition.
func (q *Query) OrWhereNotExists(sub *Query) *Query {
	q.builder.OrWhereNotExistsQuery(sub.builder)
	return q
}

// WhereDate adds WHERE DATE(column) comparison condition.
func (q *Query) WhereDate(col, cond, date string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereDate(col, cond, date)
	return q
}

// OrWhereDate adds OR WHERE DATE(column) comparison condition.
func (q *Query) OrWhereDate(col, cond, date string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereDate(col, cond, date)
	return q
}

// WhereTime adds WHERE TIME(column) comparison condition.
func (q *Query) WhereTime(col, cond, time string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereTime(col, cond, time)
	return q
}

// OrWhereTime adds OR WHERE TIME(column) comparison condition.
func (q *Query) OrWhereTime(col, cond, time string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereTime(col, cond, time)
	return q
}

// WhereDay adds WHERE DAY(column) comparison condition.
func (q *Query) WhereDay(col, cond, day string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereDay(col, cond, day)
	return q
}

// OrWhereDay adds OR WHERE DAY(column) comparison condition.
func (q *Query) OrWhereDay(col, cond, day string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereDay(col, cond, day)
	return q
}

// WhereMonth adds WHERE MONTH(column) comparison condition.
func (q *Query) WhereMonth(col, cond, month string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereMonth(col, cond, month)
	return q
}

// OrWhereMonth adds OR WHERE MONTH(column) comparison condition.
func (q *Query) OrWhereMonth(col, cond, month string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereMonth(col, cond, month)
	return q
}

// WhereYear adds WHERE YEAR(column) comparison condition.
func (q *Query) WhereYear(col, cond, year string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.WhereYear(col, cond, year)
	return q
}

// OrWhereYear adds OR WHERE YEAR(column) comparison condition.
func (q *Query) OrWhereYear(col, cond, year string) *Query {
	if q.err != nil {
		return q
	}
	cond, err := validateConditionOperator(cond)
	if err != nil {
		q.err = err
		return q
	}
	q.builder.OrWhereYear(col, cond, year)
	return q
}

// Take is an alias of Limit.
func (q *Query) Take(n int) *Query { return q.Limit(n) }

// Skip is an alias of Offset.
func (q *Query) Skip(n int) *Query { return q.Offset(n) }

// SharedLock adds LOCK IN SHARE MODE clause.
func (q *Query) SharedLock() *Query {
	q.builder.SharedLock()
	return q
}

// LockForUpdate adds FOR UPDATE clause.
func (q *Query) LockForUpdate() *Query {
	q.builder.LockForUpdate()
	return q
}

// Build returns the SQL and args.
func (q *Query) Build() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	return q.builder.Build()
}

// Dump returns SQL and args for debugging.
func (q *Query) Dump() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	return q.builder.Dump()
}

// RawSQL returns interpolated SQL for debugging.
func (q *Query) RawSQL() (string, error) {
	if q.err != nil {
		return "", q.err
	}
	return q.builder.RawSql()
}

func dataToMap(data any) (map[string]any, error) {
	if m, ok := data.(map[string]any); ok {
		return m, nil
	}
	return conv.StructToMap(data)
}

// Insert executes an INSERT with the given data.
func (q *Query) Insert(data any) (sql.Result, error) {
	plan, err := q.PlanInsert(q.ctx, data)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

// PlanInsert builds an INSERT plan for data without executing it.
func (q *Query) PlanInsert(ctx context.Context, data any) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	m, err := dataToMap(data)
	if err != nil {
		return nil, err
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.builder.GetQuery().Table.Name).Insert(m)
	sqlStr, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: q.builder.GetQuery().Table.Name})
	plan.Columns = columnRefsFromNames(sortedMapKeys(m))
	q.finalizePlan(plan)
	return plan, nil
}

// InsertGetId executes an INSERT and returns the auto-increment ID.
// For PostgreSQL, it appends a RETURNING clause for the configured
// primary key column because the driver does not support LastInsertId.
func (q *Query) InsertGetId(data any) (int64, error) {
	m, err := dataToMap(data)
	if err != nil {
		return 0, err
	}
	if _, ok := q.dialect.(driver.PostgresDialect); ok {
		plan, err := q.PlanInsert(q.ctx, m)
		if err != nil {
			return 0, err
		}
		plan.SQL += " RETURNING " + q.dialect.QuoteIdent(q.getPrimaryKeyColumn())
		var id int64
		if err := q.queryRow(plan.SQL, plan.Params...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}

	res, err := q.Insert(m)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// InsertBatch executes a bulk INSERT with the given slice of data maps.
func (q *Query) InsertBatch(data []map[string]any) (sql.Result, error) {
	plan, err := q.PlanInsertBatch(q.ctx, data)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

// PlanInsertBatch builds a batch INSERT plan without executing it.
func (q *Query) PlanInsertBatch(ctx context.Context, data []map[string]any) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.builder.GetQuery().Table.Name).InsertBatch(data)
	sqlStr, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: q.builder.GetQuery().Table.Name})
	plan.Columns = columnRefsFromNames(sortedBatchMapKeys(data))
	plan.Metadata = map[string]any{"batch_size": len(data)}
	q.finalizePlan(plan)
	return plan, nil
}

// InsertOrIgnore executes an INSERT IGNORE.
func (q *Query) InsertOrIgnore(data []map[string]any) (sql.Result, error) {
	plan, err := q.planInsertOrIgnore(q.ctx, data)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

func (q *Query) planInsertOrIgnore(ctx context.Context, data []map[string]any) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.builder.GetQuery().Table.Name).InsertOrIgnore(data)
	sqlStr, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: q.builder.GetQuery().Table.Name})
	plan.Columns = columnRefsFromNames(sortedBatchMapKeys(data))
	plan.Metadata = map[string]any{"insert_mode": "ignore", "batch_size": len(data)}
	q.finalizePlan(plan)
	return plan, nil
}

// Upsert executes an UPSERT using ON DUPLICATE KEY UPDATE.
func (q *Query) Upsert(data []map[string]any, unique []string, updateCols []string) (sql.Result, error) {
	plan, err := q.planUpsert(q.ctx, data, unique, updateCols)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

func (q *Query) planUpsert(ctx context.Context, data []map[string]any, unique []string, updateCols []string) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.builder.GetQuery().Table.Name).Upsert(data, unique, updateCols)
	sqlStr, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: q.builder.GetQuery().Table.Name})
	plan.Columns = columnRefsFromNames(sortedBatchMapKeys(data))
	plan.Metadata = map[string]any{"insert_mode": "upsert", "unique_columns": unique, "update_columns": updateCols}
	q.finalizePlan(plan)
	return plan, nil
}

// UpdateOrInsert performs UPDATE or INSERT based on condition.
func (q *Query) UpdateOrInsert(cond map[string]any, values map[string]any) (sql.Result, error) {
	plan, err := q.planUpdateOrInsert(q.ctx, cond, values)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

func (q *Query) planUpdateOrInsert(ctx context.Context, cond map[string]any, values map[string]any) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.builder.GetQuery().Table.Name).UpdateOrInsert(cond, values)
	sqlStr, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: q.builder.GetQuery().Table.Name})
	merged := make(map[string]any, len(cond)+len(values))
	for k, v := range cond {
		merged[k] = v
	}
	for k, v := range values {
		merged[k] = v
	}
	plan.Columns = columnRefsFromNames(sortedMapKeys(merged))
	plan.Metadata = map[string]any{"insert_mode": "update_or_insert", "condition_columns": sortedMapKeys(cond), "update_columns": sortedMapKeys(values)}
	q.finalizePlan(plan)
	return plan, nil
}

// InsertUsing executes an INSERT INTO ... SELECT statement using columns from a subquery.
func (q *Query) InsertUsing(columns []string, sub *Query) (sql.Result, error) {
	plan, err := q.planInsertUsing(q.ctx, columns, sub)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

func (q *Query) planInsertUsing(ctx context.Context, columns []string, sub *Query) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.builder.GetQuery().Table.Name).InsertUsing(columns, sub.builder)
	sqlStr, args, err := ib.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: q.builder.GetQuery().Table.Name})
	plan.Columns = columnRefsFromNames(columns)
	plan.Metadata = map[string]any{"insert_mode": "insert_using"}
	q.finalizePlan(plan)
	return plan, nil
}

// Update executes an UPDATE with the given data.
func (q *Query) Update(data any) (sql.Result, error) {
	plan, err := q.PlanUpdate(q.ctx, data)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

// PlanUpdate builds an UPDATE plan for data without executing it.
func (q *Query) PlanUpdate(ctx context.Context, data any) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	q.applyPolicyPredicates()
	m, err := dataToMap(data)
	if err != nil {
		return nil, err
	}
	ub := newUpdateBuilder(q.dialect)
	ub.Table(q.builder.GetQuery().Table.Name).Update(m)
	copyBuilderState(q.builder, ub)
	sqlStr, args, err := ub.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationUpdate, sqlStr, args)
	appendTableRef(plan, q.builder.GetQuery().Table.Name, "")
	plan.Columns = columnRefsFromNames(sortedMapKeys(m))
	appendSelectBuilderWriteMetadata(plan, q.builder)
	q.finalizePlan(plan)
	return plan, nil
}

// Delete executes a DELETE query using current conditions.
func (q *Query) Delete() (sql.Result, error) {
	plan, err := q.PlanDelete(q.ctx)
	if err != nil {
		return nil, err
	}
	if err := ensurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return q.execStmt(plan.SQL, plan.Params...)
}

// PlanDelete builds a DELETE plan without executing it.
func (q *Query) PlanDelete(ctx context.Context) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	q.applyPolicyPredicates()
	delBuilder := newDeleteBuilder(q.dialect)
	delBuilder.Table(q.builder.GetQuery().Table.Name).Delete()
	copyBuilderStateDelete(q.builder, delBuilder)
	sqlStr, args, err := delBuilder.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationDelete, sqlStr, args)
	appendTableRef(plan, q.builder.GetQuery().Table.Name, "")
	appendSelectBuilderWriteMetadata(plan, q.builder)
	q.finalizePlan(plan)
	return plan, nil
}

// copyBuilderState duplicates where, join and order clauses from src to dst.
func copyBuilderState(src *qbapi.SelectQueryBuilder, dst *qbapi.UpdateQueryBuilder) {
	// copy where
	srcWb := src.GetWhereBuilder()
	dstWb := dst.GetWhereBuilder()
	_ = setFieldValue(reflect.ValueOf(dstWb), "query", reflect.ValueOf(srcWb.GetQuery()))

	// copy join
	srcJb := src.GetJoinBuilder()
	dstJb := dst.GetJoinBuilder()
	// deep copy joins to avoid sharing slices between builders. The query
	// builder does not expose a cloning API, so reflection is used here.
	newJoins := deepCopyJoins(srcJb)
	_ = setFieldValue(reflect.ValueOf(dstJb), "Joins", newJoins)

	// copy order
	srcOb := src.GetOrderByBuilder()
	dstOb := dst.GetOrderByBuilder()
	_ = setFieldValue(reflect.ValueOf(dstOb), "Order", reflect.ValueOf(srcOb).Elem().FieldByName("Order"))
}

// copyBuilderStateDelete duplicates where, join and order clauses from src to a DeleteQueryBuilder.
func copyBuilderStateDelete(src *qbapi.SelectQueryBuilder, dst *qbapi.DeleteQueryBuilder) {
	// copy where
	srcWb := src.GetWhereBuilder()
	dstWb := dst.GetWhereBuilder()
	_ = setFieldValue(reflect.ValueOf(dstWb), "query", reflect.ValueOf(srcWb.GetQuery()))

	// copy join
	srcJb := src.GetJoinBuilder()
	dstJb := dst.GetJoinBuilder()
	newJoins := deepCopyJoins(srcJb)
	_ = setFieldValue(reflect.ValueOf(dstJb), "Joins", newJoins)

	// copy order
	srcOb := src.GetOrderByBuilder()
	dstOb := dst.GetOrderByBuilder()
	_ = setFieldValue(reflect.ValueOf(dstOb), "Order", reflect.ValueOf(srcOb).Elem().FieldByName("Order"))
}

// copySelectBuilderState duplicates where, join, group and lock clauses from src
// to dst. This relies on reflection because goquent-query-builder does not
// expose a safe cloning API. TODO: replace this with qbapi.Clone when available.
func copySelectBuilderState(src *qbapi.SelectQueryBuilder, dst *qbapi.SelectQueryBuilder) error {
	srcWb := src.GetWhereBuilder()
	dstWb := dst.GetWhereBuilder()
	clonedWhere := reflect.New(reflect.ValueOf(srcWb.GetQuery()).Elem().Type())
	clonedWhere.Elem().Set(reflect.ValueOf(srcWb.GetQuery()).Elem())
	if err := setFieldValue(reflect.ValueOf(dstWb), "query", clonedWhere); err != nil {
		return err
	}

	srcJb := src.GetJoinBuilder()
	dstJb := dst.GetJoinBuilder()
	newJoins := deepCopyJoins(srcJb)
	if err := setFieldValue(reflect.ValueOf(dstJb), "Joins", newJoins); err != nil {
		return err
	}

	srcOb := src.GetOrderByBuilder()
	dstOb := dst.GetOrderByBuilder()
	if err := setFieldValue(reflect.ValueOf(dstOb), "Order", reflect.ValueOf(srcOb).Elem().FieldByName("Order")); err != nil {
		return err
	}

	srcSB := reflect.ValueOf(src).Elem().FieldByName("builder").Elem()
	dstSB := reflect.ValueOf(dst).Elem().FieldByName("builder").Elem()
	srcSel := srcSB.FieldByName("selectQuery").Elem()
	dstSel := dstSB.FieldByName("selectQuery").Elem()

	dstSelAddr := reflect.NewAt(dstSel.Type(), unsafe.Pointer(dstSel.UnsafeAddr())).Elem()

	if err := setFieldValue(dstSelAddr.Addr(), "Group", srcSel.FieldByName("Group")); err != nil {
		return err
	}
	if err := setFieldValue(dstSelAddr.Addr(), "Lock", srcSel.FieldByName("Lock")); err != nil {
		return err
	}

	return nil
}

// deepCopyJoins clones the Joins value from a JoinBuilder using reflection.
// Each field of Joins is a pointer to a slice, so we copy the underlying
// slices to ensure the destination builder can modify them independently.
func deepCopyJoins(jb any) reflect.Value {
	joinsVal := reflect.ValueOf(jb).Elem().FieldByName("Joins")
	newJoins := reflect.New(joinsVal.Elem().Type())
	newJoins.Elem().Set(joinsVal.Elem())
	for _, name := range []string{"Joins", "JoinClauses", "LateralJoins"} {
		slice := joinsVal.Elem().FieldByName(name)
		if slice.IsValid() && !slice.IsNil() {
			sliceType := slice.Type().Elem()
			newSlice := reflect.MakeSlice(sliceType, slice.Elem().Len(), slice.Elem().Len())
			reflect.Copy(newSlice, slice.Elem())
			newSlicePtr := reflect.New(sliceType)
			newSlicePtr.Elem().Set(newSlice)
			newJoins.Elem().FieldByName(name).Set(newSlicePtr)
		}
	}
	return newJoins
}

// setFieldValue assigns value to an exported field using reflection.
// If the target field is unexported or cannot be set, it returns an error.
func setFieldValue(targetValue reflect.Value, field string, value reflect.Value) error {
	v := targetValue.Elem().FieldByName(field)
	if !v.IsValid() {
		return fmt.Errorf("field %q does not exist in target", field)
	}
	if v.Type() != value.Type() {
		return fmt.Errorf("type mismatch for field %q", field)
	}

	// Use unsafe to bypass Go's restrictions on setting unexported fields.
	// We create a writable handle for the destination field.
	dest := reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()

	// Special handling for zero-size types
	if value.Type().Size() == 0 {
		return nil
	}

	// Handle the case where value might not be addressable
	if !value.CanAddr() {
		// For non-addressable values, we need to make them addressable first
		// Create a new value of the same type and copy the content
		tempValue := reflect.New(value.Type()).Elem()

		// Use a different approach based on whether we can get the interface
		if value.CanInterface() {
			// For interface-able values, we can safely recreate them
			tempValue.Set(reflect.ValueOf(value.Interface()))
			value = tempValue
		} else {
			// For non-interface-able values, try to copy using reflection
			// This works for most basic types and some complex types
			if copyValueByReflection(tempValue, value) {
				value = tempValue
			} else {
				return fmt.Errorf("cannot copy non-addressable value for field %q", field)
			}
		}
	}

	// Now both src and dst should be addressable
	srcPtr := unsafe.Pointer(value.UnsafeAddr())
	dstPtr := unsafe.Pointer(dest.UnsafeAddr())
	size := int(value.Type().Size())

	// Copy memory directly
	destSlice := unsafe.Slice((*byte)(dstPtr), size)
	srcSlice := unsafe.Slice((*byte)(srcPtr), size)
	copy(destSlice, srcSlice)

	return nil
}

// copyValueByReflection attempts to copy value using pure reflection
// Returns true if successful, false otherwise
func copyValueByReflection(dst, src reflect.Value) bool {
	if dst.Type() != src.Type() {
		return false
	}

	switch src.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.String:
		// For basic types, we can try to extract and set the value
		if src.CanInterface() {
			dst.Set(reflect.ValueOf(src.Interface()))
			return true
		}
		return false
	case reflect.Slice, reflect.Array:
		// For slices and arrays, copy element by element
		if src.Len() != dst.Len() {
			return false
		}
		for i := 0; i < src.Len(); i++ {
			if !copyValueByReflection(dst.Index(i), src.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Struct:
		// For structs, copy field by field
		for i := 0; i < src.NumField(); i++ {
			if !copyValueByReflection(dst.Field(i), src.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Ptr:
		if src.IsNil() {
			dst.Set(reflect.Zero(dst.Type()))
			return true
		}
		if dst.IsNil() {
			dst.Set(reflect.New(src.Type().Elem()))
		}
		return copyValueByReflection(dst.Elem(), src.Elem())
	default:
		return false
	}
}

func validateConditionOperator(op string) (string, error) {
	op = strings.TrimSpace(op)
	switch strings.ToUpper(op) {
	case "=", "!=", "<>", ">", ">=", "<", "<=", "LIKE", "NOT LIKE":
		return op, nil
	default:
		return "", fmt.Errorf("goquent: unsupported SQL operator %q", op)
	}
}

func validateOrderDirection(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "asc", nil
	}
	switch strings.ToUpper(dir) {
	case "ASC", "DESC":
		return dir, nil
	default:
		return "", fmt.Errorf("goquent: unsupported ORDER BY direction %q", dir)
	}
}

func validateRawSQLFragment(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("goquent: raw SQL fragment is required")
	}
	if strings.ContainsAny(trimmed, ";\x00") ||
		strings.Contains(trimmed, "--") ||
		strings.Contains(trimmed, "/*") ||
		strings.Contains(trimmed, "*/") {
		return fmt.Errorf("goquent: raw SQL fragment contains a statement separator or comment")
	}
	upper := strings.ToUpper(trimmed)
	for _, token := range []string{"ALTER", "CREATE", "DELETE", "DROP", "GRANT", "INSERT", "REVOKE", "TRUNCATE", "UPDATE"} {
		if containsSQLWord(upper, token) {
			return fmt.Errorf("goquent: raw SQL fragment contains disallowed SQL token %q", token)
		}
	}
	return nil
}

func validateSelectColumn(col string) error {
	trimmed := strings.TrimSpace(col)
	if trimmed == "" {
		return fmt.Errorf("goquent: Select column name is required")
	}
	if trimmed != col || selectFieldLooksLikeSQLExpression(trimmed) {
		return fmt.Errorf("goquent: Select received a SQL expression-like field %q; use SelectRaw(...) for SQL expressions", col)
	}
	return nil
}

func validateCursorColumns(columns []CursorColumn, values []any) ([]CursorColumn, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("goquent: cursor columns are required")
	}
	if len(columns) != len(values) {
		return nil, fmt.Errorf("goquent: cursor column/value count mismatch")
	}
	normalized := make([]CursorColumn, len(columns))
	for i, col := range columns {
		name := strings.TrimSpace(col.Name)
		if name == "" {
			return nil, fmt.Errorf("goquent: cursor column name is required")
		}
		if col.Raw {
			if err := validateRawSQLFragment(name); err != nil {
				return nil, fmt.Errorf("goquent: invalid cursor expression %q: %w", col.Name, err)
			}
		} else {
			if strings.Contains(name, "*") {
				return nil, fmt.Errorf("goquent: cursor column %q cannot be a wildcard", col.Name)
			}
			if err := validateSelectColumn(name); err != nil {
				return nil, fmt.Errorf("goquent: invalid cursor column %q: %w", col.Name, err)
			}
		}
		dir, err := validateOrderDirection(col.Direction)
		if err != nil {
			return nil, err
		}
		if values[i] == nil {
			return nil, fmt.Errorf("goquent: cursor value for %s is nil", name)
		}
		normalized[i] = CursorColumn{Name: name, Direction: dir, Raw: col.Raw}
	}
	return normalized, nil
}

func cursorComparisonOperator(direction string, after bool) string {
	desc := strings.EqualFold(direction, "desc")
	if after {
		if desc {
			return "<"
		}
		return ">"
	}
	if desc {
		return ">"
	}
	return "<"
}

func quoteIdentifierPath(d driver.Dialect, ident string) string {
	parts := strings.Split(ident, ".")
	for i, part := range parts {
		parts[i] = d.QuoteIdent(part)
	}
	return strings.Join(parts, ".")
}

func selectFieldLooksLikeSQLExpression(field string) bool {
	if strings.ContainsAny(field, " \t\r\n\v\f()[],;'\"`\x00") ||
		strings.Contains(field, "::") ||
		strings.Contains(field, "--") ||
		strings.Contains(field, "/*") ||
		strings.Contains(field, "*/") {
		return true
	}
	if selectFieldHasInvalidWildcard(field) {
		return true
	}
	for _, op := range []string{"->", "->>", "+", "-", "/", "%", "=", "<", ">", "||", "&"} {
		if strings.Contains(field, op) {
			return true
		}
	}
	upper := strings.ToUpper(field)
	for _, token := range []string{"AS", "CASE"} {
		if containsSQLWord(upper, token) {
			return true
		}
	}
	return false
}

func selectFieldHasInvalidWildcard(field string) bool {
	if !strings.Contains(field, "*") {
		return false
	}
	if field == "*" {
		return false
	}
	return !(strings.HasSuffix(field, ".*") && strings.Count(field, "*") == 1)
}

func validateColumnComparisons(columns [][]string) error {
	for _, c := range columns {
		if len(c) != 3 {
			continue
		}
		if _, err := validateConditionOperator(c[1]); err != nil {
			return err
		}
	}
	return nil
}

// gatherColumns extracts unique column names from column comparison slices.
// Each slice must have length 2 (column, otherColumn) or 3 (column, operator, otherColumn).
// Returns an error if any slice has an unexpected length.
func gatherColumns(cols [][]string) ([]string, error) {
	set := make(map[string]struct{})
	for i, c := range cols {
		switch len(c) {
		case 2:
			set[c[0]] = struct{}{}
			set[c[1]] = struct{}{}
		case 3:
			set[c[0]] = struct{}{}
			set[c[2]] = struct{}{}
		default:
			return nil, fmt.Errorf("invalid column slice at index %d", i)
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out, nil
}
