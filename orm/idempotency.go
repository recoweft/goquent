package orm

import (
	"context"
	"errors"
	"fmt"
)

// IdempotentCommandSpec describes a read-before-write idempotent command.
//
// LookupExisting should return ErrNotFound/sql.ErrNoRows when the idempotency
// key has not been applied. Apply runs inside a transaction and should perform
// the aggregate update, patch/audit insert, event append, and final hydration.
// If Apply returns ErrConflict, RunIdempotentCommand performs a second lookup
// and returns the existing value when available.
type IdempotentCommandSpec[T any] struct {
	LookupExisting func(context.Context, *DB) (T, error)
	Apply          func(context.Context, Tx) (T, error)

	// LookupAfterConflict overrides LookupExisting for the race path where Apply
	// detects a uniqueness or optimistic-concurrency conflict.
	LookupAfterConflict func(context.Context, *DB) (T, error)
}

// IdempotentCommandResult is returned by RunIdempotentCommand.
type IdempotentCommandResult[T any] struct {
	Value   T
	Applied bool
}

// RunIdempotentCommand runs an idempotent command recipe.
//
// It first looks up an existing result by idempotency key. If found, the result
// is returned with Applied=false and no transaction is opened. If not found, it
// runs Apply inside a transaction and returns Applied=true. If Apply reports
// ErrConflict, the helper re-runs the lookup and returns the existing value when
// the conflict row is now visible.
func RunIdempotentCommand[T any](ctx context.Context, db *DB, spec IdempotentCommandSpec[T]) (IdempotentCommandResult[T], error) {
	var result IdempotentCommandResult[T]
	if err := validateIdempotentCommand(db, spec); err != nil {
		return result, err
	}

	existing, err := spec.LookupExisting(ctx, db)
	if err == nil {
		result.Value = existing
		return result, nil
	}
	if !IsNotFound(err) {
		return result, err
	}

	var applied T
	err = db.TransactionContext(ctx, func(tx Tx) error {
		var applyErr error
		applied, applyErr = spec.Apply(ctx, tx)
		return applyErr
	})
	if err == nil {
		result.Value = applied
		result.Applied = true
		return result, nil
	}
	if !errors.Is(err, ErrConflict) {
		return result, err
	}

	lookup := spec.LookupExisting
	if spec.LookupAfterConflict != nil {
		lookup = spec.LookupAfterConflict
	}
	existing, lookupErr := lookup(ctx, db)
	if lookupErr == nil {
		result.Value = existing
		return result, nil
	}
	if IsNotFound(lookupErr) {
		return result, err
	}
	return result, lookupErr
}

func validateIdempotentCommand[T any](db *DB, spec IdempotentCommandSpec[T]) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if db.drv == nil || db.exec == nil {
		return fmt.Errorf("goquent: db is not initialized")
	}
	if spec.LookupExisting == nil {
		return fmt.Errorf("goquent: idempotent command lookup is required")
	}
	if spec.Apply == nil {
		return fmt.Errorf("goquent: idempotent command apply is required")
	}
	return nil
}
