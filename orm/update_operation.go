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

func prepareUpdateOperation(ctx context.Context, db *DB, s operation.UpdateSpec, opts operation.Options) (querybridge.Planned, error) {
	if db == nil {
		return querybridge.Planned{}, operation.ErrInvalidAssignment
	}
	settings := db.Settings()
	opts.Settings = &settings
	opts.Dialect = db.Dialect()
	return querybridge.PrepareUpdate(ctx, s, opts, db.exec)
}

// UpdateOperationBy validates and dispatches the same privately sealed UPDATE.
// It provides the existing immediate write lifecycle, not a ValidatedPlan API.
func UpdateOperationBy(ctx context.Context, db *DB, s operation.UpdateSpec, opts operation.Options) (sql.Result, error) {
	p, err := prepareUpdateOperation(ctx, db, s, opts)
	if err != nil {
		return nil, err
	}
	return execPreparedWrite(p, len(s.Returning) > 0, applyWriteOpts(nil))
}

// UpdateOperationReturningBy uses the existing one-row scanner and bool policy.
// A first row is not an exact affected-row or automatic rollback guarantee.
func UpdateOperationReturningBy[T any](ctx context.Context, db *DB, s operation.UpdateSpec, opts operation.Options) (out T, err error) {
	if len(s.Returning) == 0 {
		return out, operation.ErrSelectRequired
	}
	p, err := prepareUpdateOperation(ctx, db, s, opts)
	if err != nil {
		return out, err
	}
	err = p.Scan(func(rows *sql.Rows) error { out, err = scanRowsOne[T](db, rows); return err })
	return out, err
}
