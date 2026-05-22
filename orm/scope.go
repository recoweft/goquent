package orm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/faciam-dev/goquent/orm/query"
)

// Scope applies reusable query mutations for advanced read and write flows.
type Scope func(*query.Query) *query.Query

// CursorColumn describes an ordered column used by keyset cursor scopes.
type CursorColumn = query.CursorColumn

// CursorAsc returns an ascending keyset cursor column.
func CursorAsc(name string) CursorColumn { return query.CursorAsc(name) }

// CursorDesc returns a descending keyset cursor column.
func CursorDesc(name string) CursorColumn { return query.CursorDesc(name) }

// CursorAscExpr returns an ascending trusted SQL expression cursor column.
func CursorAscExpr(expr string) CursorColumn { return query.CursorAscExpr(expr) }

// CursorDescExpr returns a descending trusted SQL expression cursor column.
func CursorDescExpr(expr string) CursorColumn { return query.CursorDescExpr(expr) }

// CursorAscAlias returns an ascending selected alias cursor column.
func CursorAscAlias(alias string) CursorColumn { return query.CursorAscAlias(alias) }

// CursorDescAlias returns a descending selected alias cursor column.
func CursorDescAlias(alias string) CursorColumn { return query.CursorDescAlias(alias) }

// ApplyScopes applies scopes to q in order. Nil scopes are ignored.
// If a scope returns nil, the current query is kept.
func ApplyScopes(q *query.Query, scopes ...Scope) *query.Query {
	for _, scope := range scopes {
		if scope == nil {
			continue
		}
		if next := scope(q); next != nil {
			q = next
		}
	}
	return q
}

// ComposeScopes bundles scopes into a single reusable scope.
func ComposeScopes(scopes ...Scope) Scope {
	return func(q *query.Query) *query.Query {
		return ApplyScopes(q, scopes...)
	}
}

// TenantScope adds a tenant filter scope. The default column is tenant_id.
func TenantScope(tenantID any, column ...string) Scope {
	col := "tenant_id"
	if len(column) > 0 {
		if trimmed := strings.TrimSpace(column[0]); trimmed != "" {
			col = trimmed
		}
	}
	return func(q *query.Query) *query.Query {
		return q.Where(col, tenantID)
	}
}

// CursorAfter adds a keyset pagination predicate after the given cursor.
func CursorAfter(columns []CursorColumn, values ...any) Scope {
	return func(q *query.Query) *query.Query {
		return q.WhereCursorAfter(columns, values...)
	}
}

// CursorBefore adds a keyset pagination predicate before the given cursor.
func CursorBefore(columns []CursorColumn, values ...any) Scope {
	return func(q *query.Query) *query.Query {
		return q.WhereCursorBefore(columns, values...)
	}
}

func scopedQuery(base *query.Query, scopes ...Scope) (*query.Query, error) {
	if base == nil {
		return nil, fmt.Errorf("base query is nil")
	}
	return ApplyScopes(base, scopes...), nil
}

// SelectOneBy builds a scoped query and scans the first row into T.
func SelectOneBy[T any](ctx context.Context, db *DB, base *query.Query, scopes ...Scope) (T, error) {
	var zero T
	if db == nil {
		return zero, fmt.Errorf("db is nil")
	}
	q, err := scopedQuery(base, scopes...)
	if err != nil {
		return zero, err
	}
	plan, err := q.Plan(ctx)
	if err != nil {
		return zero, err
	}
	if err := query.EnsurePlanExecutable(plan); err != nil {
		return zero, err
	}
	return SelectOne[T](ctx, db.RequireRawApproval("goquent generated scoped query"), plan.SQL, plan.Params...)
}

// SelectAllBy builds a scoped query and scans all rows into []T.
func SelectAllBy[T any](ctx context.Context, db *DB, base *query.Query, scopes ...Scope) ([]T, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	q, err := scopedQuery(base, scopes...)
	if err != nil {
		return nil, err
	}
	plan, err := q.Plan(ctx)
	if err != nil {
		return nil, err
	}
	if err := query.EnsurePlanExecutable(plan); err != nil {
		return nil, err
	}
	return SelectAll[T](ctx, db.RequireRawApproval("goquent generated scoped query"), plan.SQL, plan.Params...)
}

// UpdateBy applies scopes to base and executes an UPDATE using the resulting query.
func UpdateBy(ctx context.Context, base *query.Query, data any, scopes ...Scope) (sql.Result, error) {
	q, err := scopedQuery(base, scopes...)
	if err != nil {
		return nil, err
	}
	return q.WithContext(ctx).Update(data)
}

// UpdateByReturning applies scopes, executes an UPDATE, and scans the Postgres RETURNING row into T.
func UpdateByReturning[T any](ctx context.Context, db *DB, base *query.Query, data any, scopes ...Scope) (T, error) {
	return UpdateByReturningWithOptions[T](ctx, db, base, data, nil, scopes...)
}

// UpdateByReturningWithOptions applies scopes, executes an UPDATE with
// RETURNING, and applies write options such as NoRowsAs for guarded updates.
func UpdateByReturningWithOptions[T any](ctx context.Context, db *DB, base *query.Query, data any, opts []WriteOpt, scopes ...Scope) (T, error) {
	var zero T
	if db == nil {
		return zero, fmt.Errorf("db is nil")
	}
	o := applyWriteOpts(opts)
	q, err := scopedQuery(base, scopes...)
	if err != nil {
		return zero, err
	}
	plan, err := q.PlanUpdate(ctx, data)
	if err != nil {
		return zero, err
	}
	if err := query.EnsurePlanExecutable(plan); err != nil {
		return zero, err
	}
	if len(o.returning) == 0 {
		cols, err := returningColumnsForQuery[T]()
		if err != nil {
			return zero, err
		}
		o.returning = cols
	}
	sqlStr, err := appendReturningClause(db.drv.Dialect, plan.SQL, o.returning)
	if err != nil {
		return zero, err
	}
	return queryReturningOneWithOptions[T](ctx, db, sqlStr, o, plan.Params...)
}

// DeleteBy applies scopes to base and executes a DELETE using the resulting query.
func DeleteBy(ctx context.Context, base *query.Query, scopes ...Scope) (sql.Result, error) {
	q, err := scopedQuery(base, scopes...)
	if err != nil {
		return nil, err
	}
	return q.WithContext(ctx).Delete()
}
