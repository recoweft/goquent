package query

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/observe"
	"github.com/recoweft/goquent/orm/internal/writecheck"
	"sync/atomic"
	"time"

	"github.com/recoweft/goquent/orm/internal/planidentity"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

// plannedExecution is created only at the builder/finalizer boundary. Public
// diagnostics are never read back as SQL, arguments or permission to execute.
// This is an in-process lifecycle, not a serialized plan or external permit.
type plannedExecution struct {
	observationShape string
	results          writecheck.Options
	writeOwner       *sql.DB
	managedTx        bool
	bindingExpires   *time.Time
	identity         *planidentity.Snapshot
	identityErr      error
	owner            *Query
	ctx              context.Context
	executor         executor
	sql              string
	args             []any
	inspection       *QueryPlan
	insertRows       []map[string]any
	gate             error
	expires          *time.Time
	used             atomic.Bool
}

func (q *Query) sealExecution(p *QueryPlan) {
	e := &plannedExecution{results: q.writeResult.Clone(), writeOwner: q.writeOwner, managedTx: q.managedWriteTx, owner: q, ctx: q.ctx, executor: q.exec, sql: p.SQL,
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
	q.sealIdentity(e)
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
	if e.bindingExpires != nil && !e.bindingExpires.After(time.Now().UTC()) {
		return ErrBindingExpired
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
func (q *Query) execute(p *QueryPlan, mode executionMode, scan func(observe.Reader) error, dest ...any) (sql.Result, error) {
	switch mode {
	case executionResult:
		return q.runWrite(p, func(e *plannedExecution) (sql.Result, error) {
			t, err := e.observe("exec")
			if err != nil {
				return nil, err
			}
			var res sql.Result
			if e.ctx != nil {
				res, err = e.executor.ExecContext(e.ctx, e.sql, valuecopy.Slice(e.args)...)
			} else {
				res, err = e.executor.Exec(e.sql, valuecopy.Slice(e.args)...)
			}
			t.Dispatch()
			phase := "dispatch"
			if err == nil {
				phase = "result"
				var n int64
				var known bool
				n, known, err = e.results.ResultCount(res)
				t.Affected(n, known)
			}
			out := "success"
			if err != nil {
				out = "error"
			}
			t.End(out, observe.Classify(err, phase), observe.Metric{}, observe.Metric{}, observe.Metric{})
			if err != nil {
				return nil, err
			}
			return res, nil
		})
	case executionRow:
		if p != nil && p.execution != nil && p.execution.results.Active() {
			return nil, writecheck.ErrOptions
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
		if t == nil {
			return nil, row.Scan(dest...)
		}
		return nil, observe.WrapRow(row, nil, t).Scan(dest...)
	case executionRows:
		if p != nil && p.execution != nil && p.execution.results.Active() {
			return nil, writecheck.ErrOptions
		}
		e, err := q.bindExecution(p)
		if err != nil {
			observe.Reject(q.ctx, err)
			return nil, err
		}
		raw, t, err := e.openRows()
		if err != nil {
			return nil, err
		}
		if t == nil {
			defer raw.Close()
			return nil, scan(raw)
		}
		rows := observe.Wrap(raw, t)
		defer rows.Close()
		err = scan(rows)
		observe.Result(rows, err)
		return nil, err
	default:
		return nil, fmt.Errorf("%w: unsupported execution mode", ErrBlockedOperation)
	}
}

func (q *Query) executeOpenRows(p *QueryPlan) (*sql.Rows, error) {
	if p != nil && p.execution != nil && p.execution.results.Active() {
		return nil, writecheck.ErrOptions
	}
	e, err := q.bindExecution(p)
	if err != nil {
		observe.Reject(q.ctx, err)
		return nil, err
	}
	rows, t, err := e.openRows()
	if err == nil {
		t.End("handoff", "unknown", observe.Metric{}, observe.Metric{}, observe.Metric{})
	}
	return rows, err
}
func (q *Query) executeOpenRow(p *QueryPlan) (*sql.Row, error) {
	if p != nil && p.execution != nil && p.execution.results.Active() {
		return nil, writecheck.ErrOptions
	}
	e, err := q.bindExecution(p)
	if err != nil {
		observe.Reject(q.ctx, err)
		return nil, err
	}
	row, t, err := e.openRow()
	if err == nil {
		t.End("handoff", "unknown", observe.Metric{}, observe.Metric{}, observe.Metric{})
	}
	return row, err
}

func (q *Query) executeResult(p *QueryPlan) (sql.Result, error) {
	return q.execute(p, executionResult, nil)
}
func (q *Query) executeRows(p *QueryPlan, scan func(observe.Reader) error) error {
	_, err := q.execute(p, executionRows, scan)
	return err
}
func (q *Query) executeRow(p *QueryPlan, dest ...any) error {
	_, err := q.execute(p, executionRow, nil, dest...)
	return err
}
