package orm

import (
	"context"
	"fmt"
	"strings"
)

// TransactionHook runs after the main transaction body and before commit.
//
// Use hooks for audit rows, outbox messages, or other write-side records that
// must commit atomically with the aggregate write.
type TransactionHook struct {
	Name string
	Run  func(context.Context, Tx) error
}

// NewTransactionHook creates a named transaction hook.
func NewTransactionHook(name string, run func(context.Context, Tx) error) TransactionHook {
	return TransactionHook{Name: strings.TrimSpace(name), Run: run}
}

// InsertHook inserts one row inside the surrounding transaction.
func InsertHook[T any](name string, value T, opts ...WriteOpt) TransactionHook {
	return NewTransactionHook(name, func(ctx context.Context, tx Tx) error {
		_, err := Insert(ctx, tx.DB, value, opts...)
		return err
	})
}

// InsertManyHook inserts rows inside the surrounding transaction.
func InsertManyHook[T any](name string, values []T, opts ...WriteOpt) TransactionHook {
	return NewTransactionHook(name, func(ctx context.Context, tx Tx) error {
		if len(values) == 0 {
			return nil
		}
		_, err := InsertMany(ctx, tx.DB, values, opts...)
		return err
	})
}

// TransactionWithHooksSpec describes one transaction body plus post-write hooks.
type TransactionWithHooksSpec[T any] struct {
	Apply func(context.Context, Tx) (T, error)
	Hooks []TransactionHook
}

// RunTransactionWithHooks runs Apply, then hooks, inside one transaction.
//
// Hooks run only if Apply succeeds. Any hook error rolls the transaction back.
func RunTransactionWithHooks[T any](ctx context.Context, db *DB, spec TransactionWithHooksSpec[T]) (T, error) {
	var zero T
	if db == nil {
		return zero, fmt.Errorf("db is nil")
	}
	if db.drv == nil || db.exec == nil {
		return zero, fmt.Errorf("goquent: db is not initialized")
	}
	if spec.Apply == nil {
		return zero, fmt.Errorf("goquent: transaction apply is required")
	}
	for _, hook := range spec.Hooks {
		if hook.Run == nil {
			return zero, fmt.Errorf("goquent: transaction hook %q is missing run function", hook.Name)
		}
	}
	var out T
	err := db.TransactionContext(ctx, func(tx Tx) error {
		value, err := spec.Apply(ctx, tx)
		if err != nil {
			return err
		}
		for _, hook := range spec.Hooks {
			if err := hook.Run(ctx, tx); err != nil {
				if hook.Name == "" {
					return err
				}
				return fmt.Errorf("goquent: transaction hook %s failed: %w", hook.Name, err)
			}
		}
		out = value
		return nil
	})
	if err != nil {
		return zero, err
	}
	return out, nil
}
