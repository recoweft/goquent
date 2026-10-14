package query

import (
	"context"
	"database/sql"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

func init() {
	querybridge.PlanOperationUpdate = planOperationUpdate
	querybridge.OperationEquality = func(base any, column string, value any) error {
		q, ok := base.(*Query)
		if !ok || q == nil || !scopeIdent(column) {
			return ErrBlockedOperation
		}
		q.builder.WhereLiteral(column, value)
		return nil
	}
}

func planOperationUpdate(ctx context.Context, base any, values map[string]any, returning []string) (querybridge.Planned, error) {
	q, ok := base.(*Query)
	if !ok || q == nil || q.operationValidation == nil {
		return querybridge.Planned{}, ErrBlockedOperation
	}
	p, err := q.planUpdateValues(ctx, values, writeinput.Options{Literal: true})
	if err != nil {
		return querybridge.Planned{}, err
	}
	if len(returning) > 0 {
		if err := q.planReturningProjection(p, returning, true); err != nil {
			return querybridge.Planned{}, err
		}
	}
	if p.execution == nil || p.execution.owner != q || p.Operation != OperationUpdate {
		return querybridge.Planned{}, ErrBlockedOperation
	}
	return querybridge.Planned{Diagnostic: p, Check: func() error { return q.checkExecution(p) }, Exec: func() (sql.Result, error) { return q.executeResult(p) }, Scan: func(scan func(*sql.Rows) error) error { return q.executeRows(p, scan) }}, nil
}
