package query

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

// These controls use database/sql with a synthetic driver, not native acquisition.
// There is no Tx context and Next is never called: EOF cannot auto-close rows.
// A live explicit Close cancels only database/sql's internal close context;
// standard query cancellation instead lets Rows.awaitDone store context.Canceled.
func TestPerformanceRowsCloseObservation(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		name := "explicit_live"
		if automatic {
			name = "automatic_canceled"
		}
		t.Run(name, func(t *testing.T) {
			independent := errors.New("fixture independent rows close")
			entered := make(chan struct{})
			release := make(chan struct{})
			returned := make(chan error, 1)
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			ctx, cancel := context.WithCancel(context.Background())
			db := sql.OpenDB(&rollbackConnector{rowsClose: func() error {
				close(entered)
				<-release
				result := independent // Sensitivity overlay changes only this driver result.
				returned <- result    // Notification is not database/sql close completion.
				return result
			}})
			var rows *sql.Rows
			var public chan error
			observed := false
			joined := false
			wait := func(ch <-chan error, label string) (error, bool) {
				select {
				case e := <-ch:
					return e, true
				case <-time.After(3 * time.Second):
					t.Error(label)
					return nil, false
				}
			}
			// Registered before acquisition and before assertions, including the gate.
			t.Cleanup(func() {
				cancel()
				unblock()
				if rows != nil {
					closed := make(chan error, 1)
					go func() { closed <- rows.Close() }()
					if e, ok := wait(closed, "fallback rows close watchdog"); ok && e != nil && e != independent {
						t.Error("unexpected fallback rows close result")
					}
					if !observed {
						_, _ = wait(returned, "driver return observer watchdog")
					}
					if public != nil && !joined {
						_, _ = wait(public, "public close observer watchdog")
					}
				}
				if db.Stats().InUse != 0 {
					t.Error("rows fixture pool retained connection")
				}
				controlCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
				defer stop()
				if db.PingContext(controlCtx) != nil {
					t.Error("separate live pool control failed")
				}
				if db.Close() != nil {
					t.Error("rows fixture pool close failed")
				}
				if db.Stats().InUse != 0 {
					t.Error("closed fixture pool retained connection")
				}
				t.Log("rows cleanup observed: release, public close, driver observer, pool, live control")
			})
			var err error
			rows, err = db.QueryContext(ctx, "fixture")
			if err != nil {
				t.Fatal("rows fixture query failed")
			}
			if automatic {
				cancel()
			} else {
				public = make(chan error, 1)
				go func() { public <- rows.Close() }()
			}
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("rows close entry watchdog")
			}
			// Forced-failure overlay inserts an assertion here while the driver is gated.
			unblock()
			actual, ok := wait(returned, "driver return watchdog")
			observed = ok
			if !ok {
				t.FailNow()
			}
			if actual != independent {
				t.Error("independent driver close result lost")
			}
			if automatic {
				public = make(chan error, 1)
				// This call joins database/sql's close mutex after the automatic close.
				go func() { public <- rows.Close() }()
			}
			got, ok := wait(public, "public close completion watchdog")
			joined = ok
			if !ok {
				t.FailNow()
			}
			if automatic {
				if got != nil || rows.Err() != context.Canceled {
					t.Error("automatic rows public observation mismatch")
				}
			} else if got != independent || rows.Err() != independent || ctx.Err() != nil {
				t.Error("explicit live rows public observation mismatch")
			}
			if db.Stats().InUse != 0 {
				t.Error("completed rows retained pool connection")
			}
		})
	}
}
