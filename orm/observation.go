package orm

import (
	"context"
	"github.com/recoweft/goquent/orm/internal/observe"
	"github.com/recoweft/goquent/orm/observation"
)

// QueryObservedContext preserves raw policy checks and tracks consumption until Close.
// Legacy QueryContext returns concrete *sql.Rows with an incomplete handoff event.
func (db *DB) QueryObservedContext(ctx context.Context, q string, args ...any) (*observation.Rows, error) {
	p, err := db.prepareRaw(ctx, q, args...)
	if err != nil {
		observe.Reject(ctx, err)
		return nil, err
	}
	return p.ObservedRows()
}
