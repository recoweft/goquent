package orm

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
	"testing"
)

func TestRawEveryEntryRejectsWithoutDispatch(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			spy := &genericSpy{}
			db := NewDBWithExecutor(spy, genericDialect(dialect), WithSettings(genericSettings(t, dialect, true))).RequireRawApproval("reviewed").TouchedTables("users")
			ctx := context.Background()
			sqlText := "SELECT id FROM users"
			tests := []func() error{
				func() error { _, e := db.Query(sqlText); return e }, func() error { _, e := db.QueryContext(ctx, sqlText); return e },
				func() error { _, e := db.Exec(sqlText); return e }, func() error { _, e := db.ExecContext(ctx, sqlText); return e },
				func() error { _, e := db.QueryRowE(ctx, sqlText); return e }, func() error { return db.QueryRow(sqlText).Scan(new(int)) }, func() error { return db.QueryRowContext(ctx, sqlText).Err() },
				func() error { _, e := SelectOne[genericID](ctx, db, sqlText); return e }, func() error { _, e := SelectAll[genericID](ctx, db, sqlText); return e },
			}
			for i, run := range tests {
				if e := run(); !errors.Is(e, ErrBlockedOperation) {
					t.Fatalf("entry %d: %v", i, e)
				}
			}
			p, e := db.RawPlan(ctx, sqlText)
			if e != nil || !errors.Is(EnsurePlanExecutable(p), ErrBlockedOperation) {
				t.Fatal(p, e)
			}
			p = query.NewRawPlanWithSettings(db.settings, sqlText)
			if !errors.Is(EnsurePlanExecutable(p), ErrBlockedOperation) {
				t.Fatal(p)
			}
			if len(spy.calls) != 0 {
				t.Fatal(spy.calls)
			}
		})
	}
}

func TestRowRefusalIdentityAndZeroValue(t *testing.T) {
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.MySQLDialect{})
	row := db.QueryRow("DROP TABLE users")
	err := row.Err()
	dest := 41
	if !errors.Is(err, ErrApprovalRequired) || row.Scan(&dest) != err || dest != 41 {
		t.Fatal(err, dest)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	row = db.QueryRowContext(ctx, "SELECT 1")
	if row.Err() != context.Canceled || row.Scan(&dest) != context.Canceled || len(spy.calls) != 0 {
		t.Fatal(row.Err(), spy.calls)
	}
	for _, r := range []*Row{nil, {}} {
		if r.Err() == nil || r.Scan(&dest) == nil {
			t.Fatal("uninitialized row accepted")
		}
	}
	marker := RowsAffectedError{Expected: 2, Actual: 1, Cause: ErrConflict}
	row = &Row{err: marker}
	var got RowsAffectedError
	if !errors.As(row.Scan(&dest), &got) || !errors.Is(row.Err(), ErrConflict) {
		t.Fatal(row.Err())
	}
}

func TestRowAllowedDelegation(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		for _, outcome := range []string{"bool", "empty", "driver", "scan"} {
			t.Run(outcome+map[bool]string{true: " context"}[withContext], func(t *testing.T) {
				sqlDB, mock, e := sqlmock.New()
				if e != nil {
					t.Fatal(e)
				}
				defer sqlDB.Close()
				spy := &genericSpy{Executor: sqlDB}
				db := NewDBWithExecutor(spy, driver.PostgresDialect{}).RequireRawApproval("reviewed")
				wantErr := errors.New("driver failure")
				expectation := mock.ExpectQuery("SELECT flag FROM users WHERE id = \\$1").WithArgs(int64(4))
				switch outcome {
				case "bool":
					expectation.WillReturnRows(sqlmock.NewRows([]string{"flag"}).AddRow(true))
				case "empty":
					expectation.WillReturnRows(sqlmock.NewRows([]string{"flag"}))
				case "driver":
					expectation.WillReturnError(wantErr)
				case "scan":
					expectation.WillReturnRows(sqlmock.NewRows([]string{"flag"}).AddRow("invalid bool"))
				}
				var ctx context.Context
				if withContext {
					ctx = context.WithValue(context.Background(), struct{}{}, "row")
				}
				var row *Row
				if withContext {
					row = db.QueryRowContext(ctx, "SELECT flag FROM users WHERE id = $1", int64(4))
				} else {
					row = db.QueryRow("SELECT flag FROM users WHERE id = $1", int64(4))
				}
				if len(spy.calls) != 1 || spy.ctx != ctx {
					t.Fatal(spy)
				}
				var value bool
				err := row.Scan(&value)
				switch outcome {
				case "bool":
					if err != nil || !value {
						t.Fatal(value, err)
					}
				case "empty":
					if !errors.Is(err, sql.ErrNoRows) {
						t.Fatal(err)
					}
				case "driver":
					if !errors.Is(err, wantErr) || !errors.Is(row.Err(), wantErr) {
						t.Fatal(err)
					}
				case "scan":
					if err == nil {
						t.Fatal("scan should fail")
					}
				}
				_ = row.Err()
				_ = row.Scan(&value)
				if len(spy.calls) != 1 {
					t.Fatal(spy.calls)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
