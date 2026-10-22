package query

import (
	"database/sql"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/observe"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"weak"
)

func (e *plannedExecution) observe(method string) (*observe.Ticket, error) {
	if !observe.Enabled(e.ctx) {
		return nil, nil
	}
	var owner any
	k := e.observationShape
	if k != "" {
		ex := e.executor
		if o, ok := ex.(*ownedExecutor); ok {
			ex = o.executor
		}
		// Weak identity preserves equality without retaining a pool, transaction or its context.
		// It is never dereferenced, formatted or published.
		switch x := ex.(type) {
		case *sql.DB:
			owner = weak.Make(x)
		case *sql.Tx:
			owner = weak.Make(x)
		}
	}
	if k != "" {
		switch e.owner.dialect.(type) {
		case driver.MySQLDialect:
			k = "mysql\x00" + k
		case driver.PostgresDialect:
			k = "postgres\x00" + k
		default:
			k = ""
		}
	}
	return observe.Begin(e.ctx, "query", method, owner, k, e.inspection.Operation == OperationSelect || e.inspection.Operation == OperationRaw)
}
func (e *plannedExecution) openRows() (*sql.Rows, *observe.Ticket, error) {
	t, err := e.observe("query")
	if err != nil {
		return nil, nil, err
	}
	var rows *sql.Rows
	if e.ctx != nil {
		rows, err = e.executor.QueryContext(e.ctx, e.sql, valuecopy.Slice(e.args)...)
	} else {
		rows, err = e.executor.Query(e.sql, valuecopy.Slice(e.args)...)
	}
	t.Dispatch()
	if err != nil {
		t.End("error", observe.Classify(err, "dispatch"), observe.Metric{}, observe.Metric{}, observe.Metric{})
	}
	return rows, t, err
}
func (e *plannedExecution) openRow() (*sql.Row, *observe.Ticket, error) {
	t, err := e.observe("row")
	if err != nil {
		return nil, nil, err
	}
	var row *sql.Row
	if e.ctx != nil {
		row = e.executor.QueryRowContext(e.ctx, e.sql, valuecopy.Slice(e.args)...)
	} else {
		row = e.executor.QueryRow(e.sql, valuecopy.Slice(e.args)...)
	}
	t.Dispatch()
	return row, t, nil
}
func (q *Query) executeTrackedRows(p *QueryPlan) (*observe.Rows, error) {
	if p != nil && p.execution != nil && p.execution.results.Active() {
		return nil, ErrBlockedOperation
	}
	e, err := q.bindExecution(p)
	if err != nil {
		observe.Reject(q.ctx, err)
		return nil, err
	}
	rows, t, err := e.openRows()
	if err != nil {
		return nil, err
	}
	return observe.Wrap(rows, t), nil
}
func (q *Query) executeTrackedRow(p *QueryPlan) (*observe.Row, error) {
	if p != nil && p.execution != nil && p.execution.results.Active() {
		return nil, ErrBlockedOperation
	}
	e, err := q.bindExecution(p)
	if err != nil {
		observe.Reject(q.ctx, err)
		return nil, err
	}
	row, t, err := e.openRow()
	if err != nil {
		return nil, err
	}
	return observe.WrapRow(row, nil, t), nil
}
