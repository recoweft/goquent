package observe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrRowsUnavailable = errors.New("OBSERVATION_ROWS_UNAVAILABLE")

// Reader is private plumbing shared by scanners and execution, not a public signature.
type Reader interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close() error
	Columns() ([]string, error)
	ColumnTypes() ([]*sql.ColumnType, error)
	NextResultSet() bool
}

// Rows observes consumption without exposing an escape to the underlying rows.
// Like database/sql.Rows, iteration and scanning require caller serialization.
type Rows struct {
	rows                           *sql.Rows
	ticket                         *Ticket
	consumed, scans                int64
	code                           string
	ended, atEnd, allSets, skipped bool
}

func Wrap(rows *sql.Rows, t *Ticket) *Rows { return &Rows{rows: rows, ticket: t, code: "none"} }
func (r *Rows) Next() bool {
	if r == nil || r.rows == nil {
		return false
	}
	ok := r.rows.Next()
	if !r.ended {
		if ok {
			if r.consumed < MaxNumber {
				r.consumed++
			} else {
				r.ticket.Saturate()
			}
			r.atEnd = false
		} else {
			r.atEnd = true
			r.note(r.rows.Err(), "rows")
		}
		r.ticket.Update(r.consumed, r.scans)
	}
	return ok
}
func (r *Rows) Scan(dest ...any) error {
	if r == nil || r.rows == nil {
		return ErrRowsUnavailable
	}
	e := r.rows.Scan(dest...)
	if !r.ended {
		if e == nil {
			if r.scans < MaxNumber {
				r.scans++
			} else {
				r.ticket.Saturate()
			}
		} else {
			r.note(e, "scan")
		}
		r.ticket.Update(r.consumed, r.scans)
	}
	return e
}
func (r *Rows) Err() error {
	if r == nil || r.rows == nil {
		return ErrRowsUnavailable
	}
	e := r.rows.Err()
	if !r.ended {
		r.note(e, "rows")
	}
	return e
}
func (r *Rows) Columns() ([]string, error) {
	if r == nil || r.rows == nil {
		return nil, ErrRowsUnavailable
	}
	return r.rows.Columns()
}
func (r *Rows) ColumnTypes() ([]*sql.ColumnType, error) {
	if r == nil || r.rows == nil {
		return nil, ErrRowsUnavailable
	}
	return r.rows.ColumnTypes()
}
func (r *Rows) NextResultSet() bool {
	if r == nil || r.rows == nil {
		return false
	}
	ok := r.rows.NextResultSet()
	if !r.ended {
		if !r.atEnd {
			r.skipped = true
		}
		if ok {
			r.atEnd = false
		} else {
			r.allSets = !r.skipped
			r.note(r.rows.Err(), "rows")
			_ = r.Close()
		}
	}
	return ok
}
func (r *Rows) note(err error, fallback string) {
	if err != nil && r.code == "none" {
		r.code = Classify(err, fallback)
	}
}

// Result notes a scanner/mapping/result-check failure before cleanup.
func Result(reader Reader, err error) {
	r, ok := reader.(*Rows)
	if !ok {
		return
	}
	if !r.ended {
		r.note(err, "result")
	}
}
func (r *Rows) Close() error {
	if r == nil || r.rows == nil {
		return ErrRowsUnavailable
	}
	e := r.rows.Close()
	if !r.ended {
		r.ended = true
		r.note(e, "close")
		r.note(r.rows.Err(), "rows")
		out := "partial"
		total := Metric{}
		if r.allSets && r.code == "none" {
			out = "success"
			total = Metric{true, r.consumed}
		}
		if r.code != "none" {
			out = "error"
		}
		r.ticket.End(out, r.code, Metric{true, r.consumed}, Metric{true, r.scans}, total)
	}
	return e
}
func (r *Rows) String() string {
	return Event{Class: "query", Method: "query", Outcome: "unknown"}.String()
}
func (r *Rows) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(r.String())) }
func (r *Rows) MarshalJSON() ([]byte, error) {
	return Event{Class: "query", Method: "query", Outcome: "unknown"}.MarshalJSON()
}

// Row retains private observation state until Scan; Err does not imply cleanup.
type Row struct {
	row    *sql.Row
	err    error
	ticket *Ticket
}

func WrapRow(row *sql.Row, err error, t *Ticket) *Row { return &Row{row: row, err: err, ticket: t} }
func (r *Row) Err() error {
	if r == nil {
		return ErrRowsUnavailable
	}
	if r.err != nil {
		return r.err
	}
	if r.row == nil {
		return ErrRowsUnavailable
	}
	return r.row.Err()
}
func (r *Row) Scan(dest ...any) error {
	if r == nil {
		return ErrRowsUnavailable
	}
	if r.row == nil && r.err == nil {
		r.ticket.End("error", "dispatch", Metric{}, Metric{}, Metric{})
		return ErrRowsUnavailable
	}
	if r.err != nil {
		return r.err
	}
	err := r.row.Scan(dest...)
	c, sc := Metric{}, Metric{}
	out := "error"
	if err == nil {
		c = Metric{true, 1}
		sc = Metric{true, 1}
		out = "success"
	} else if eq(err, sql.ErrNoRows) {
		c = Metric{true, 0}
		sc = Metric{true, 0}
	}
	r.ticket.End(out, Classify(err, "scan"), c, sc, Metric{})
	return err
}
func (r *Row) String() string             { return Event{Class: "query", Method: "row"}.String() }
func (r *Row) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(r.String())) }

// Internal SQL helpers are used only at existing library-owned dispatch sites.
type Executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func InternalExec(ctx context.Context, ex Executor, q string, args ...any) (sql.Result, error) {
	t, e := Begin(ctx, "internal", "exec", nil, "", false)
	if e != nil {
		return nil, e
	}
	r, e := ex.ExecContext(ctx, q, args...)
	t.Dispatch()
	out := "success"
	if e != nil {
		out = "error"
	}
	t.End(out, Classify(e, "dispatch"), Metric{}, Metric{}, Metric{})
	return r, e
}
func InternalQuery(ctx context.Context, ex Executor, q string, args ...any) (Reader, error) {
	t, e := Begin(ctx, "internal", "query", nil, "", false)
	if e != nil {
		return nil, e
	}
	r, e := ex.QueryContext(ctx, q, args...)
	t.Dispatch()
	if e != nil {
		t.End("error", Classify(e, "dispatch"), Metric{}, Metric{}, Metric{})
		return nil, e
	}
	if t == nil {
		return r, nil
	}
	return Wrap(r, t), nil
}

type RowReader interface {
	Scan(...any) error
	Err() error
}

func InternalRow(ctx context.Context, ex Executor, q string, args ...any) RowReader {
	t, e := Begin(ctx, "internal", "row", nil, "", false)
	if e != nil {
		return WrapRow(nil, e, nil)
	}
	r := ex.QueryRowContext(ctx, q, args...)
	t.Dispatch()
	if t == nil {
		return r
	}
	return WrapRow(r, nil, t)
}
