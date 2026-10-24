package query

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"
)

// cancellationPhase is a channel-driven context fixture, not a timer or an
// Err-call counter. The separate test deadline is only a deadlock watchdog.
type cancellationPhase struct {
	context.Context
	reason error
}

func (c cancellationPhase) Err() error {
	if c.Context.Err() != nil {
		return c.reason
	}
	return nil
}

type rollbackConnector struct {
	rollback  func() error
	rowsClose func() error
}

func (c *rollbackConnector) Connect(context.Context) (sqldriver.Conn, error) {
	return &rollbackConn{c}, nil
}
func (c *rollbackConnector) Driver() sqldriver.Driver            { return c }
func (c *rollbackConnector) Open(string) (sqldriver.Conn, error) { return &rollbackConn{c}, nil }

type rollbackConn struct{ c *rollbackConnector }

func (c *rollbackConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("fixture unsupported")
}
func (c *rollbackConn) Close() error                 { return nil }
func (c *rollbackConn) Begin() (sqldriver.Tx, error) { return c, nil }
func (c *rollbackConn) BeginTx(context.Context, sqldriver.TxOptions) (sqldriver.Tx, error) {
	return c, nil
}
func (c *rollbackConn) Commit() error   { return errors.New("fixture unexpected commit") }
func (c *rollbackConn) Rollback() error { return c.c.rollback() }
func (c *rollbackConn) QueryContext(context.Context, string, []sqldriver.NamedValue) (sqldriver.Rows, error) {
	return &cleanupRows{c.c.rowsClose}, nil
}

type cleanupRows struct{ close func() error }

func (r *cleanupRows) Columns() []string            { return make([]string, 17) }
func (r *cleanupRows) Close() error                 { return r.close() }
func (r *cleanupRows) Next([]sqldriver.Value) error { return io.EOF }

func TestPerformanceRollbackCancellation(t *testing.T) {
	independent := errors.New("fixture independent cleanup")
	for _, tc := range []struct {
		name            string
		ended, returned error
		prior, want     error
	}{
		{"canceled", context.Canceled, context.Canceled, ErrPerformanceQuery, ErrPerformanceCanceled},
		{"deadline", context.DeadlineExceeded, context.DeadlineExceeded, ErrPerformanceQuery, ErrPerformanceTimeout},
		{"live_canceled", nil, context.Canceled, nil, ErrPerformanceCleanup},
		{"live_deadline", nil, context.DeadlineExceeded, nil, ErrPerformanceCleanup},
		{"mismatched_canceled", context.Canceled, context.DeadlineExceeded, nil, ErrPerformanceCleanup},
		{"mismatched_deadline", context.DeadlineExceeded, context.Canceled, nil, ErrPerformanceCleanup},
		{"independent", context.Canceled, independent, nil, ErrPerformanceCleanup},
		{"live_independent", nil, independent, nil, ErrPerformanceCleanup},
		{"wrapped", context.Canceled, fmt.Errorf("fixture: %w", context.Canceled), nil, ErrPerformanceCleanup},
		{"joined", context.Canceled, errors.Join(context.Canceled, independent), nil, ErrPerformanceCleanup},
		{"bad_connection", context.Canceled, sqldriver.ErrBadConn, nil, ErrPerformanceCleanup},
		{"eof", context.Canceled, io.EOF, nil, ErrPerformanceCleanup},
		{"prior_cleanup", context.Canceled, context.Canceled, ErrPerformanceCleanup, ErrPerformanceCleanup},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, cancel := context.WithCancel(context.Background())
			defer cancel()
			ctx := cancellationPhase{base, tc.ended}
			entered := make(chan struct{})
			db := sql.OpenDB(&rollbackConnector{rollback: func() error {
				close(entered)
				if tc.ended != nil {
					cancel()
				}
				return tc.returned
			}})
			defer db.Close()
			tx, e := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if e != nil {
				t.Fatal("fixture begin failed")
			}
			// The explicit rollback already owns Tx.done before this driver cancels ctx.
			got := performanceRollback(ctx, tx, tc.prior)
			select {
			case <-entered:
			default:
				t.Fatal("explicit rollback not reached")
			}
			if got != tc.want || errors.Unwrap(got) != nil {
				t.Fatal("fixed rollback classification mismatch")
			}
			if db.Stats().InUse != 0 {
				t.Fatal("fixture connection retained")
			}
		})
	}
}

func TestPerformanceRollbackAwaitDone(t *testing.T) {
	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	db := sql.OpenDB(&rollbackConnector{rollback: func() error { close(done); return context.Canceled }})
	defer db.Close()
	tx, e := db.BeginTx(base, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal("fixture begin failed")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("automatic rollback watchdog")
	}
	// The driver's entry establishes that awaitDone already won Tx.done's CAS.
	if got := performanceRollback(base, tx, ErrPerformanceCanceled); got != ErrPerformanceCanceled {
		t.Fatal("automatic rollback classification mismatch")
	}
}

func TestPerformanceCancellationPreservesRowsCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sql.OpenDB(&rollbackConnector{
		rollback:  func() error { return context.Canceled },
		rowsClose: func() error { cancel(); return errors.New("fixture independent rows cleanup") },
	})
	defer db.Close()
	conn, e := db.Conn(ctx)
	if e != nil {
		t.Fatal("fixture connection failed")
	}
	defer conn.Close()
	p, e := performanceTransaction(ctx, conn, "mysql", nil, qbapi.PerformanceShape{})
	if p != nil || e != ErrPerformanceCleanup {
		t.Fatal("independent rows cleanup hidden by cancellation")
	}
}

// This is a counterexample to treating every automatic-cancellation close as
// a successful explicit Conn.Close. It preserves the independent cleanup code.
func TestPerformanceAutomaticRollbackConnectionDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closed := make(chan struct{})
	db := sql.OpenDB(&closeSignalConnector{closed: closed})
	defer db.Close()
	conn, e := db.Conn(ctx)
	if e != nil {
		t.Fatal("fixture connection failed")
	}
	tx, e := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal("fixture begin failed")
	}
	cancel()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("automatic close watchdog")
	}
	if got := performanceRollback(ctx, tx, ErrPerformanceCanceled); got != ErrPerformanceCanceled {
		t.Fatal("automatic rollback classification mismatch")
	}
	if conn.Close() != sql.ErrConnDone {
		t.Fatal("automatic close classification mismatch")
	}
	if db.Stats().InUse != 0 {
		t.Fatal("automatic close retained connection")
	}
}

type closeSignalConnector struct{ closed chan struct{} }

func (c *closeSignalConnector) Connect(context.Context) (sqldriver.Conn, error) {
	return &closeSignalConn{c.closed}, nil
}
func (c *closeSignalConnector) Driver() sqldriver.Driver { return c }
func (c *closeSignalConnector) Open(string) (sqldriver.Conn, error) {
	return &closeSignalConn{c.closed}, nil
}

type closeSignalConn struct{ closed chan struct{} }

func (c *closeSignalConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("fixture unsupported")
}
func (c *closeSignalConn) Begin() (sqldriver.Tx, error) { return c, nil }
func (c *closeSignalConn) Close() error                 { close(c.closed); return nil }
func (c *closeSignalConn) Commit() error                { return errors.New("fixture unexpected commit") }
func (c *closeSignalConn) Rollback() error              { return sqldriver.ErrBadConn }
func (c *closeSignalConn) BeginTx(context.Context, sqldriver.TxOptions) (sqldriver.Tx, error) {
	return c, nil
}

// ErrConnDone is observable while driver.Close is still blocked. Neither the
// logical handle state nor that second Close reveals an independent close error.
func TestPerformanceConnectionDoneBeforeRelease(t *testing.T) {
	for _, reason := range []error{context.Canceled, context.DeadlineExceeded} {
		name := "canceled"
		if reason == context.DeadlineExceeded {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			for _, closeFailure := range []bool{false, true} {
				label := "close_ok"
				if closeFailure {
					label = "close_failure"
				}
				t.Run(label, func(t *testing.T) {
					base, cancel := context.WithCancel(context.Background())
					defer cancel()
					ctx := cancellationPhase{base, reason}
					entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
					c := &releaseConnector{entered: entered, release: release, finished: finished, fail: closeFailure}
					db := sql.OpenDB(c)
					defer db.Close()
					defer close(release)
					conn, e := db.Conn(ctx)
					if e != nil {
						t.Fatal("fixture connection failed")
					}
					tx, e := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
					if e != nil {
						t.Fatal("fixture begin failed")
					}
					cancel()
					select {
					case <-entered:
					case <-time.After(3 * time.Second):
						t.Fatal("discard entry watchdog")
					}
					if conn.Close() != sql.ErrConnDone {
						t.Fatal("logical handle not terminated")
					}
					select {
					case <-finished:
						t.Fatal("physical release unexpectedly finished")
					default:
					}
					if performanceRollback(ctx, tx, ErrPerformanceCleanup) != ErrPerformanceCleanup {
						t.Fatal("known cleanup failure lost")
					}
					// The exact same API observations occur for successful and failed driver
					// close, and before either has completed. No production exemption follows.
				})
			}
		})
	}
}

type releaseConnector struct {
	entered, release, finished chan struct{}
	fail                       bool
}

func (c *releaseConnector) Connect(context.Context) (sqldriver.Conn, error) {
	return &releaseConn{c}, nil
}
func (c *releaseConnector) Driver() sqldriver.Driver            { return c }
func (c *releaseConnector) Open(string) (sqldriver.Conn, error) { return &releaseConn{c}, nil }

type releaseConn struct{ c *releaseConnector }

func (c *releaseConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("fixture unsupported")
}
func (c *releaseConn) Begin() (sqldriver.Tx, error) { return c, nil }
func (c *releaseConn) BeginTx(context.Context, sqldriver.TxOptions) (sqldriver.Tx, error) {
	return c, nil
}
func (c *releaseConn) Commit() error   { return errors.New("fixture unexpected commit") }
func (c *releaseConn) Rollback() error { return sqldriver.ErrBadConn }
func (c *releaseConn) Close() error {
	close(c.c.entered)
	<-c.c.release
	close(c.c.finished)
	if c.c.fail {
		return errors.New("fixture independent physical close failure")
	}
	return nil
}

func TestPerformanceExplicitConnectionCloseBoundary(t *testing.T) {
	for _, reason := range []error{nil, context.Canceled, context.DeadlineExceeded} {
		name := "live"
		if reason == context.Canceled {
			name = "canceled"
		}
		if reason == context.DeadlineExceeded {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			for _, fail := range []bool{false, true} {
				label := "close_ok"
				if fail {
					label = "close_failure"
				}
				t.Run(label, func(t *testing.T) {
					base, cancel := context.WithCancel(context.Background())
					defer cancel()
					ctx := cancellationPhase{base, reason}
					c := &releaseConnector{entered: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{}), fail: fail}
					db := sql.OpenDB(c)
					db.SetMaxIdleConns(0)
					defer db.Close()
					conn, e := db.Conn(ctx)
					if e != nil {
						t.Fatal("fixture connection failed")
					}
					if reason != nil {
						cancel()
					}
					returned := make(chan error, 1)
					go func() { returned <- conn.Close() }()
					select {
					case <-c.entered:
					case <-time.After(3 * time.Second):
						close(c.release)
						t.Fatal("explicit close entry watchdog")
					}
					second := conn.Close()
					select {
					case <-returned:
						close(c.release)
						t.Fatal("explicit close returned before physical close")
					default:
					}
					close(c.release)
					select {
					case e = <-returned:
					case <-time.After(3 * time.Second):
						t.Fatal("explicit close completion watchdog")
					}
					if second != sql.ErrConnDone || e != nil {
						t.Fatal("explicit close API observation changed")
					}
					// database/sql does not expose this driver's independent Close error to
					// either caller, even when the context is live. Do not infer success from nil.
					if db.Stats().InUse != 0 {
						t.Fatal("explicit close retained connection")
					}
				})
			}
		})
	}
}

// Synthetic close errors exercise classification only; these are not claims
// that database/sql.Conn.Close exposes wrapped or joined driver errors.
func TestPerformanceOwnedCloseClassification(t *testing.T) {
	independent := errors.New("fixture independent close")
	for _, tc := range []struct {
		name                       string
		ended, prior, closed, want error
	}{
		{"canceled", context.Canceled, ErrPerformanceCanceled, sql.ErrConnDone, ErrPerformanceCanceled},
		{"deadline", context.DeadlineExceeded, ErrPerformanceTimeout, sql.ErrConnDone, ErrPerformanceTimeout},
		{"prior_cleanup", context.Canceled, ErrPerformanceCleanup, sql.ErrConnDone, ErrPerformanceCleanup},
		{"live", nil, ErrPerformanceCanceled, sql.ErrConnDone, ErrPerformanceCleanup},
		{"mismatch", context.Canceled, ErrPerformanceTimeout, sql.ErrConnDone, ErrPerformanceCleanup},
		{"reverse_mismatch", context.DeadlineExceeded, ErrPerformanceCanceled, sql.ErrConnDone, ErrPerformanceCleanup},
		{"prior_nil", context.Canceled, nil, sql.ErrConnDone, ErrPerformanceCleanup},
		{"prior_query", context.Canceled, ErrPerformanceQuery, sql.ErrConnDone, ErrPerformanceCleanup},
		{"prior_result", context.Canceled, ErrPerformanceResult, sql.ErrConnDone, ErrPerformanceCleanup},
		{"wrapped", context.Canceled, ErrPerformanceCanceled, fmt.Errorf("fixture: %w", sql.ErrConnDone), ErrPerformanceCleanup},
		{"joined", context.Canceled, ErrPerformanceCanceled, errors.Join(sql.ErrConnDone, independent), ErrPerformanceCleanup},
		{"independent", context.Canceled, ErrPerformanceCanceled, independent, ErrPerformanceCleanup},
		{"bad_conn", context.Canceled, ErrPerformanceCanceled, sqldriver.ErrBadConn, ErrPerformanceCleanup},
		{"eof", context.Canceled, ErrPerformanceCanceled, io.EOF, ErrPerformanceCleanup},
		{"nil_close", context.Canceled, ErrPerformanceCanceled, nil, ErrPerformanceCanceled},
		{"nil_success", nil, nil, nil, nil},
		{"nil_prior_cleanup", context.Canceled, ErrPerformanceCleanup, nil, ErrPerformanceCleanup},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if performanceCloseResult(tc.ended, tc.prior, tc.closed) != tc.want {
				t.Fatal("owned close classification mismatch")
			}
		})
	}
}
