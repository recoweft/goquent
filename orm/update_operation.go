package orm

import (
	"context"
	"database/sql"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/operation"
)

// CompileUpdateWithDiagnostics is DB-free and forces this DB's actual settings.
func (db *DB) CompileUpdateWithDiagnostics(ctx context.Context, s operation.UpdateSpec, opts operation.Options) (*QueryPlan, operation.DiagnosticView, error) {
	if db == nil {
		return operation.CompileUpdateWithDiagnostics(ctx, s, operation.Options{})
	}
	settings := db.Settings()
	opts.Settings = &settings
	opts.Dialect = db.Dialect()
	return operation.CompileUpdateWithDiagnostics(ctx, s, opts)
}

func prepareUpdateOperation(ctx context.Context, db *DB, s operation.UpdateSpec, opts operation.Options, writeOpts ...WriteOpt) (querybridge.Planned, error) {
	if db == nil {
		return querybridge.Planned{}, operation.ErrInvalidAssignment
	}
	settings := db.Settings()
	opts.Settings = &settings
	opts.Dialect = db.Dialect()
	o := applyWriteOpts(writeOpts)
	if len(o.cols)+len(o.omit)+len(o.returning)+len(o.tablePath)+len(o.pkCols)+len(o.conflictCols)+len(o.upsertUpdateCols)+len(o.assignments) > 0 || o.table != "" || o.schema != "" || o.wherePK || o.hasUpsertUpdates || o.conflictDoNothing || o.conflictWhere != "" || o.conflictConstraint != "" || o.conflictTargetRaw != "" {
		return querybridge.Planned{}, ErrWriteOptions
	}
	return querybridge.PrepareUpdate(ctx, s, opts, db.exec, querybridge.Request{Owner: db.writeOwner(), Results: o.results})
}

// UpdateOperationBy validates and dispatches the same privately sealed UPDATE.
// It provides the existing immediate write lifecycle, not a ValidatedPlan API.
func UpdateOperationBy(ctx context.Context, db *DB, s operation.UpdateSpec, opts operation.Options, writeOpts ...WriteOpt) (sql.Result, error) {
	p, err := prepareUpdateOperation(ctx, db, s, opts, writeOpts...)
	if err != nil {
		return nil, err
	}
	return execPreparedWrite(p, len(s.Returning) > 0, applyWriteOpts(nil))
}

// UpdateOperationReturningBy uses the existing one-row scanner and bool policy.
// A first row is not an exact affected-row or automatic rollback guarantee.
func UpdateOperationReturningBy[T any](ctx context.Context, db *DB, s operation.UpdateSpec, opts operation.Options, writeOpts ...WriteOpt) (out T, err error) {
	if len(s.Returning) == 0 {
		return out, operation.ErrSelectRequired
	}
	p, err := prepareUpdateOperation(ctx, db, s, opts, writeOpts...)
	if err != nil {
		return out, err
	}
	var count int64
	err = p.ScanCount(func(rows *sql.Rows) (int64, error) {
		for {
			row, e := scanRowsOne[T](db, rows)
			if e == sql.ErrNoRows {
				break
			}
			if e != nil {
				return count, e
			}
			if count == 0 {
				out = row
			}
			count++
		}
		if count == 0 {
			return count, sql.ErrNoRows
		}
		return count, nil
	})
	if err == nil && count == 0 {
		err = sql.ErrNoRows
	}
	return out, err
}
