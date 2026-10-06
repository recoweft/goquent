package orm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

func newWriteInput(op, table string, cols []string, args []any, count int, pk []string, o *writeOptions) (querybridge.Request, error) {
	r := querybridge.Request{Operation: op, Returning: append([]string(nil), o.returning...)}
	parts := append([]string(nil), o.tablePath...)
	if len(parts) == 0 {
		table = strings.TrimSpace(table)
		if table == "" {
			return r, fmt.Errorf("goquent: table name is required")
		}
		if schema := strings.TrimSpace(o.schema); schema != "" {
			if strings.Contains(table, ".") {
				return r, fmt.Errorf("goquent: Schema cannot be combined with schema-qualified table %q", table)
			}
			parts = []string{schema, table}
		} else {
			parts = strings.Split(table, ".")
		}
	}
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
		if parts[i] == "" {
			return r, fmt.Errorf("goquent: identifier path contains an empty part")
		}
	}
	r.Table = strings.Join(parts, ".")
	r.Options = writeinput.Options{Columns: append([]string{}, cols...), TableParts: parts, Assignments: append([]writeinput.Assignment(nil), o.assignments...), Literal: true}
	if err := writeinput.ValidateAssignments(r.Options.Assignments); err != nil {
		return r, err
	}
	if len(args) != count*len(cols) {
		return r, fmt.Errorf("invalid internal write shape")
	}
	r.Rows = make([]map[string]any, count)
	for i := range r.Rows {
		r.Rows[i] = make(map[string]any, len(cols))
		for j, c := range cols {
			r.Rows[i][c] = args[i*len(cols)+j]
		}
	}
	if op == "upsert" {
		target := conflictTargetColumns(o, pk)
		updates, err := upsertUpdateColumns(cols, target, o)
		if err != nil {
			return r, err
		}
		r.Options.Conflict = &writeinput.Conflict{Columns: append([]string(nil), target...), Updates: updates, Where: o.conflictWhere, Constraint: o.conflictConstraint, RawTarget: o.conflictTargetRaw, ExplicitColumns: len(o.conflictCols) > 0, DoNothing: len(updates) == 0 && len(o.assignments) == 0}
	}
	return r, nil
}

func prepareWrite(ctx context.Context, db *DB, r querybridge.Request) (querybridge.Planned, error) {
	if db == nil {
		return querybridge.Planned{}, fmt.Errorf("db is nil")
	}
	r.Settings = db.settings
	r.Executor = db.exec
	r.Dialect = db.drv.Dialect
	r.Context = ctx
	return querybridge.Prepare(r)
}

func execWriteInput(ctx context.Context, db *DB, r querybridge.Request, o *writeOptions) (sql.Result, error) {
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return nil, err
	}
	return execPreparedWrite(p, len(r.Returning) > 0, o)
}

func execPreparedWrite(p querybridge.Planned, returning bool, o *writeOptions) (sql.Result, error) {
	var err error
	var res sql.Result
	if !returning {
		res, err = p.Exec()
	} else {
		var count int64
		err = p.Scan(func(rows *sql.Rows) error {
			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			dest := make([]any, len(cols))
			values := make([]any, len(cols))
			for i := range dest {
				dest[i] = &values[i]
			}
			for rows.Next() {
				if len(dest) > 0 {
					if err := rows.Scan(dest...); err != nil {
						return err
					}
				}
				count++
			}
			return rows.Err()
		})
		res = returningResult{rowsAffected: count}
	}
	if err != nil {
		return nil, err
	}
	if err = checkRowsAffected(res, o); err != nil {
		return nil, err
	}
	return res, nil
}
func queryWriteOne[T any](ctx context.Context, db *DB, r querybridge.Request) (out T, err error) {
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return out, err
	}
	err = p.Scan(func(rows *sql.Rows) error { out, err = scanRowsOne[T](db, rows); return err })
	return out, err
}
func queryWriteAll[T any](ctx context.Context, db *DB, r querybridge.Request) (out []T, err error) {
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return nil, err
	}
	err = p.Scan(func(rows *sql.Rows) error { out, err = scanRowsAll[T](db, rows); return err })
	return out, err
}
func queryWriteOneWithOptions[T any](ctx context.Context, db *DB, r querybridge.Request, o *writeOptions) (T, error) {
	row, err := queryWriteOne[T](ctx, db, r)
	if err == nil || !IsNotFound(err) || o.zeroRowsErr == nil {
		return row, err
	}
	var zero T
	return zero, RowsAffectedError{Expected: 1, Actual: 0, Cause: o.zeroRowsErr}
}
func writeDiagnostic(ctx context.Context, db *DB, r querybridge.Request, err error) (*QueryPlan, error) {
	if err != nil {
		return nil, err
	}
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return nil, err
	}
	return p.Diagnostic.(*QueryPlan), nil
}

// PlanInsert inspects the same structural input as Insert without executing SQL.
// The result is diagnostic only; it cannot be submitted for execution.
func PlanInsert[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (*QueryPlan, error) {
	r, err := buildInsertInput(db, v, applyWriteOpts(opts))
	return writeDiagnostic(ctx, db, r, err)
}

// PlanUpdate inspects the same structural input as Update without executing SQL.
func PlanUpdate[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (*QueryPlan, error) {
	r, err := buildUpdateInput(db, v, applyWriteOpts(opts))
	return writeDiagnostic(ctx, db, r, err)
}

// PlanUpsert inspects the same structural input as Upsert without executing SQL.
func PlanUpsert[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (*QueryPlan, error) {
	r, err := buildUpsertInput(db, v, applyWriteOpts(opts))
	return writeDiagnostic(ctx, db, r, err)
}

// PlanInsertMany inspects one INSERT statement, not a compound batch operation.
func PlanInsertMany[T any](ctx context.Context, db *DB, v []T, opts ...WriteOpt) (*QueryPlan, error) {
	r, err := buildInsertManyInput(db, v, applyWriteOpts(opts))
	return writeDiagnostic(ctx, db, r, err)
}

// PlanUpsertMany inspects one UPSERT statement, not a compound batch operation.
func PlanUpsertMany[T any](ctx context.Context, db *DB, v []T, opts ...WriteOpt) (*QueryPlan, error) {
	r, err := buildUpsertManyInput(db, v, applyWriteOpts(opts))
	return writeDiagnostic(ctx, db, r, err)
}
