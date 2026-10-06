package query

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

// plannedExecution is created only at the builder/finalizer boundary. Public
// diagnostics are never read back as SQL, arguments or permission to execute.
// This is an in-process lifecycle, not a serialized plan or external permit.
type plannedExecution struct {
	owner      *Query
	ctx        context.Context
	executor   executor
	sql        string
	args       []any
	inspection *QueryPlan
	gate       error
	expires    *time.Time
	used       atomic.Bool
}

func (q *Query) sealExecution(p *QueryPlan) {
	e := &plannedExecution{owner: q, ctx: q.ctx, executor: q.exec, sql: p.SQL,
		args: valuecopy.Slice(p.Params), gate: ensurePlanExecutable(p)}
	// Keep structural inspection alongside the exact statement, detached from the
	// diagnostic projection. Unsupported custom payloads retain compatibility
	// semantics; the strict gate already refuses values it cannot compare.
	v := *p
	v.execution = nil
	v.Params = valuecopy.Slice(p.Params)
	v.Tables = append([]TableRef(nil), p.Tables...)
	v.Columns = append([]ColumnRef(nil), p.Columns...)
	v.Joins = append([]JoinRef(nil), p.Joins...)
	for i := range v.Joins {
		v.Joins[i].OnTree = valuecopy.Node(v.Joins[i].OnTree)
	}
	v.WhereTree = valuecopy.Node(p.WhereTree)
	v.HavingTree = valuecopy.Node(p.HavingTree)
	v.Predicates = append([]PredicateRef(nil), p.Predicates...)
	if p.Limit != nil {
		n := *p.Limit
		v.Limit = &n
	}
	if p.Offset != nil {
		n := *p.Offset
		v.Offset = &n
	}
	if p.EstimatedRows != nil {
		n := *p.EstimatedRows
		v.EstimatedRows = &n
	}
	if p.UsesIndex != nil {
		b := *p.UsesIndex
		v.UsesIndex = &b
	}
	v.Unverified = append([]string(nil), p.Unverified...)
	v.Metadata = make(map[string]any, len(p.Metadata))
	for k, x := range p.Metadata {
		switch x := x.(type) {
		case []RequiredPredicate:
			v.Metadata[k] = append([]RequiredPredicate(nil), x...)
		default:
			v.Metadata[k], _ = valuecopy.Copy(x)
		}
	}
	e.inspection = &v
	if p.RequiredApproval && p.Approval != nil && p.Approval.ExpiresAt != nil {
		expiry := *p.Approval.ExpiresAt
		e.expires = &expiry
	}
	p.execution = e
}

func (q *Query) checkExecution(p *QueryPlan) error {
	if p == nil || p.execution == nil || p.execution.owner != q {
		return fmt.Errorf("%w: missing private execution plan", ErrBlockedOperation)
	}
	e := p.execution
	if e.gate != nil {
		return e.gate
	}
	if e.ctx != nil {
		if err := e.ctx.Err(); err != nil {
			return err
		}
	}
	if e.expires != nil && !e.expires.After(time.Now().UTC()) {
		return fmt.Errorf("%w: approval expired", ErrApprovalRequired)
	}
	if e.used.Load() {
		return fmt.Errorf("%w: execution plan already consumed", ErrBlockedOperation)
	}
	return nil
}

func (q *Query) bindExecution(p *QueryPlan) (*plannedExecution, error) {
	if err := q.checkExecution(p); err != nil {
		return nil, err
	}
	e := p.execution
	if !e.used.CompareAndSwap(false, true) {
		return nil, fmt.Errorf("%w: execution plan already consumed", ErrBlockedOperation)
	}
	return e, nil
}

type executionMode uint8

const (
	executionResult executionMode = iota
	executionRows
	executionRow
)

// This file owns Query and facade Raw executor dispatch. All result modes bind
// the private plan and reject before any executor call, including QueryRow.
func (q *Query) execute(p *QueryPlan, mode executionMode, scan func(*sql.Rows) error, dest ...any) (sql.Result, error) {
	switch mode {
	case executionResult:
		e, err := q.bindExecution(p)
		if err != nil {
			return nil, err
		}
		if e.ctx != nil {
			return e.executor.ExecContext(e.ctx, e.sql, valuecopy.Slice(e.args)...)
		}
		return e.executor.Exec(e.sql, valuecopy.Slice(e.args)...)
	case executionRow:
		row, err := q.executeOpenRow(p)
		if err != nil {
			return nil, err
		}
		return nil, row.Scan(dest...)
	case executionRows:
		rows, err := q.executeOpenRows(p)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return nil, scan(rows)
	default:
		return nil, fmt.Errorf("%w: unsupported execution mode", ErrBlockedOperation)
	}
}

func (q *Query) executeOpenRows(p *QueryPlan) (*sql.Rows, error) {
	e, err := q.bindExecution(p)
	if err != nil {
		return nil, err
	}
	if e.ctx != nil {
		return e.executor.QueryContext(e.ctx, e.sql, valuecopy.Slice(e.args)...)
	}
	return e.executor.Query(e.sql, valuecopy.Slice(e.args)...)
}
func (q *Query) executeOpenRow(p *QueryPlan) (*sql.Row, error) {
	e, err := q.bindExecution(p)
	if err != nil {
		return nil, err
	}
	if e.ctx != nil {
		return e.executor.QueryRowContext(e.ctx, e.sql, valuecopy.Slice(e.args)...), nil
	}
	return e.executor.QueryRow(e.sql, valuecopy.Slice(e.args)...), nil
}

func (q *Query) executeResult(p *QueryPlan) (sql.Result, error) {
	return q.execute(p, executionResult, nil)
}
func (q *Query) executeRows(p *QueryPlan, scan func(*sql.Rows) error) error {
	_, err := q.execute(p, executionRows, scan)
	return err
}
func (q *Query) executeRow(p *QueryPlan, dest ...any) error {
	_, err := q.execute(p, executionRow, nil, dest...)
	return err
}
