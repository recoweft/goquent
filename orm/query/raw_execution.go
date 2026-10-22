package query

import (
	"database/sql"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/observe"
	"github.com/recoweft/goquent/orm/internal/querybridge"
)

func prepareRaw(r querybridge.RawRequest) (querybridge.Planned, error) {
	var out querybridge.Planned
	s, ok := r.Settings.(Settings)
	if !ok {
		return out, fmt.Errorf("%w: invalid internal settings", ErrBlockedOperation)
	}
	if err := s.Err(); err != nil {
		return out, err
	}
	if r.Context != nil {
		if err := r.Context.Err(); err != nil {
			return out, err
		}
	}
	q := NewWithSettings(r.Executor, "", r.Dialect, s).WithContext(r.Context)
	p := NewRawPlanWithSettings(s, r.SQL, r.Args...)
	if a, ok := r.Approval.(*Approval); ok && a != nil {
		c := *a
		p.Approval = &c
	}
	for _, t := range r.Tables {
		p.Tables = append(p.Tables, TableRef{Name: t})
	}
	q.sealExecution(p)
	out.ObservedRows = func() (*observe.Rows, error) { return q.executeTrackedRows(p) }
	out.ObservedRow = func() (*observe.Row, error) { return q.executeTrackedRow(p) }
	out.Scan = func(scan func(observe.Reader) error) error { return q.executeRows(p, scan) }
	out.Diagnostic = p
	out.Check = func() error { return q.checkExecution(p) }
	out.Exec = func() (sql.Result, error) { return q.executeResult(p) }
	out.Rows = func() (*sql.Rows, error) { return q.executeOpenRows(p) }
	out.Row = func() (*sql.Row, error) { return q.executeOpenRow(p) }

	return out, nil
}
