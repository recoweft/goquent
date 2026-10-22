package orm

import (
	"context"
	"github.com/recoweft/goquent/orm/internal/observe"
	"sync/atomic"
)

type transactionObservation struct {
	ctx   context.Context
	ended atomic.Bool
}

// Commit delegates unchanged; its boundary does not rewrite earlier SQL events.
func (tx Tx) Commit() error {
	var ctx context.Context
	if tx.DB != nil && tx.txObservation != nil {
		ctx = tx.txObservation.ctx
	}
	start := observe.Clock(ctx)
	err := tx.Tx.Commit()
	if tx.DB != nil && tx.txObservation != nil && tx.txObservation.ended.CompareAndSwap(false, true) {
		observe.Tx(ctx, "commit", err, observe.Elapsed(start))
	}
	return err
}

// Rollback never runs automatically merely because an observation scope finishes.
func (tx Tx) Rollback() error {
	var ctx context.Context
	if tx.DB != nil && tx.txObservation != nil {
		ctx = tx.txObservation.ctx
	}
	start := observe.Clock(ctx)
	err := tx.Tx.Rollback()
	if tx.DB != nil && tx.txObservation != nil && tx.txObservation.ended.CompareAndSwap(false, true) {
		observe.Tx(ctx, "rollback", err, observe.Elapsed(start))
	}
	return err
}
