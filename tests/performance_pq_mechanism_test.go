package tests

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/recoweft/goquent/orm"
)

// pqPublicConn preserves every optional connection interface implemented by pq
// v1.10.9. This decorator is mechanism instrumentation, never native authority.
type pqPublicConn interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.Execer
	driver.ExecerContext
	driver.Queryer
	driver.QueryerContext
	driver.Pinger
	driver.SessionResetter
	driver.Validator
}
type pqRollbackGate struct {
	entered  chan struct{}
	release  chan struct{}
	returned chan error
	once     sync.Once
}

func (g *pqRollbackGate) unblock() { g.once.Do(func() { close(g.release) }) }

type pqMechanismConnector struct {
	driver.Connector
	gate *pqRollbackGate
}

func (c pqMechanismConnector) Connect(ctx context.Context) (driver.Conn, error) {
	raw, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &pqMechanismConn{pqPublicConn: raw.(pqPublicConn), gate: c.gate}, nil
}

type pqMechanismConn struct {
	pqPublicConn
	gate *pqRollbackGate
}

func (c *pqMechanismConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := c.pqPublicConn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &pqMechanismTx{Tx: tx, gate: c.gate}, nil
}

type pqMechanismTx struct {
	driver.Tx
	gate *pqRollbackGate
}

func (tx *pqMechanismTx) Rollback() error {
	close(tx.gate.entered)
	<-tx.gate.release
	err := tx.Tx.Rollback()
	tx.gate.returned <- err
	return err
}
func pqMechanismWait(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("mechanism synchronization watchdog")
	}
}

// Three finite orders use real pq transactions, without acquiring a production
// performance plan. No target query, persistent object, role or schema is created.
func TestPQCancellationMechanisms(t *testing.T) {
	if os.Getenv("GOQUENT_NATIVE_DISPOSABLE_FIXTURE") != "1" || os.Getenv("TEST_MYSQL_DSN") != defaultMySQLTestDSN || os.Getenv("TEST_DB_DSN") != defaultMySQLTestDSN || os.Getenv("TEST_POSTGRES_DSN") != defaultPostgresTestDSN {
		t.Skip("disposable fixture opt-in and three explicit fixture DSNs required")
	}
	for _, order := range []string{"normal", "stored_cancel", "automatic_first"} {
		t.Run(order, func(t *testing.T) {
			owner, err := orm.OpenWithDriver("postgres", os.Getenv("TEST_POSTGRES_DSN"))
			if err != nil {
				t.Fatal("mechanism owner unavailable")
			}
			t.Cleanup(func() {
				if owner.Close() != nil {
					t.Error("mechanism owner close failed")
				}
			})
			proxy, dsn := newBoundaryProxy(t, "postgres", os.Getenv("TEST_POSTGRES_DSN"))
			connector, err := pq.NewConnector(dsn)
			if err != nil {
				t.Fatal("mechanism connector unavailable")
			}
			// Background is used for connect and the PID query. Only BeginTx gets
			// the standard cancellable lifetime context; there is no query cancel ctx.
			lifetime, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			var pid int64
			t.Cleanup(func() {
				proxy.close(t)
				if pid != 0 {
					boundaryServerGone(t, owner, "postgres", pid)
				}
				ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				var one int
				if owner.SQLDB().QueryRowContext(ctx, "SELECT 1").Scan(&one) != nil || one != 1 {
					t.Error("mechanism control failed")
				}
				t.Log("mechanism cleanup: proxy joined; session absent; control checked; owned objects none")
			})
			if order == "automatic_first" {
				gate := &pqRollbackGate{entered: make(chan struct{}), release: make(chan struct{}), returned: make(chan error, 1)}
				db := sql.OpenDB(pqMechanismConnector{Connector: connector, gate: gate})
				db.SetMaxOpenConns(1)
				var tx *sql.Tx
				var received bool
				t.Cleanup(func() {
					cancel()
					gate.unblock()
					if tx != nil {
						_ = tx.Rollback()
						if !received {
							select {
							case e := <-gate.returned:
								if e != driver.ErrBadConn {
									t.Error("mechanism cleanup driver result not exact BadConn")
								}
							case <-time.After(5 * time.Second):
								t.Error("mechanism rollback cleanup watchdog")
							}
						}
					}
					deadline := time.NewTimer(5 * time.Second)
					defer deadline.Stop()
					tick := time.NewTicker(time.Millisecond)
					defer tick.Stop()
				drain:
					for db.Stats().InUse != 0 {
						select {
						case <-tick.C:
						case <-deadline.C:
							t.Error("mechanism pool cleanup watchdog")
							break drain
						}
					}
					if db.Close() != nil {
						t.Error("mechanism pool close failed")
					}
					t.Log("mechanism cleanup: rollback joined; pool checked")
				})
				if db.QueryRowContext(context.Background(), "SELECT pg_backend_pid()").Scan(&pid) != nil {
					t.Fatal("mechanism session unavailable")
				}
				tx, err = db.BeginTx(lifetime, &sql.TxOptions{ReadOnly: true})
				if err != nil {
					t.Fatal("mechanism begin failed")
				}
				cancel()
				pqMechanismWait(t, gate.entered)       // database/sql already won Tx.done.
				pqMechanismWait(t, proxy.cancellation) // pq stored cancellation before dialing.
				if tx.Rollback() != sql.ErrTxDone {
					t.Error("mechanism explicit result not exact TxDone")
				}
				if lifetime.Err() != context.Canceled {
					t.Error("mechanism context mismatch")
				}
				gate.unblock()
				select {
				case e := <-gate.returned:
					received = true
					if e != driver.ErrBadConn {
						t.Error("mechanism automatic driver result not exact BadConn")
					}
				case <-time.After(5 * time.Second):
					t.Fatal("mechanism driver return watchdog")
				}
				t.Log("mechanism: automatic entry; cancellation forwarded; explicit TxDone; driver BadConn; ctx Canceled")
			} else {
				raw, err := connector.Connect(context.Background())
				if err != nil {
					t.Fatal("mechanism native connect failed")
				}
				conn := raw.(pqPublicConn)
				rolled := false
				var tx driver.Tx
				t.Cleanup(func() {
					if tx != nil && !rolled {
						want := error(nil)
						if order == "stored_cancel" && lifetime.Err() == context.Canceled {
							want = driver.ErrBadConn
						}
						if tx.Rollback() != want {
							t.Error("mechanism fallback rollback failed")
						}
					}
					// Cancelled pq Rollback invokes txnFinish, which closes the connection.
					// A second raw Close is not an independent close-success observation.
					if order == "normal" || tx == nil || lifetime.Err() == nil {
						if raw.Close() != nil {
							t.Error("mechanism raw close failed")
						}
					}
					t.Log("mechanism cleanup: raw transaction finished; no sql pool")
				})
				rows, err := conn.QueryContext(context.Background(), "SELECT pg_backend_pid()", nil)
				if err != nil {
					t.Fatal("mechanism session query failed")
				}
				values := make([]driver.Value, 1)
				nextErr := rows.Next(values)
				closeErr := rows.Close()
				if nextErr != nil || closeErr != nil {
					t.Fatal("mechanism session rows failed")
				}
				pid = values[0].(int64)
				tx, err = conn.BeginTx(lifetime, driver.TxOptions{ReadOnly: true})
				if err != nil {
					t.Fatal("mechanism native begin failed")
				}
				want := error(nil)
				if order == "stored_cancel" {
					cancel()
					pqMechanismWait(t, proxy.cancellation)
					want = driver.ErrBadConn
				}
				got := tx.Rollback()
				rolled = true
				if got != want {
					t.Error("mechanism native rollback classification mismatch")
				}
				if order == "stored_cancel" && lifetime.Err() != context.Canceled {
					t.Error("mechanism context mismatch")
				}
				t.Log("mechanism: native rollback fixed expectation checked; lifetime context checked")
			}
		})
	}
}
