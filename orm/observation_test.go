package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/observation"
	"strings"
	"testing"
)

func observationDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	t.Helper()
	raw, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		_ = raw.Close()
		if m.ExpectationsWereMet() != nil {
			t.Error("unmet SQL expectations")
		}
	})
	return NewDB(raw, driver.PostgresDialect{}).RequireRawApproval("test"), m
}
func observationEvents(t *testing.T, s *observation.Scope) []observation.Event {
	t.Helper()
	var events []observation.Event
	_, p, e := s.Drain(func(v observation.Event) { events = append(events, v) })
	if e != nil || p != 0 {
		t.Fatal("drain")
	}
	return events
}
func TestObservationRowsBoundaries(t *testing.T) {
	for _, kind := range []string{"eof", "early", "scan", "rows", "close", "incomplete", "multi", "rescan"} {
		t.Run(kind, func(t *testing.T) {
			db, m := observationDB(t)
			ctx, s, _ := observation.Start(context.Background(), observation.Options{})
			r := sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2)
			source := errors.New("private-error-canary")
			switch kind {
			case "scan":
				r = sqlmock.NewRows([]string{"id"}).AddRow("not-an-integer")
			case "rows":
				r.RowError(1, source)
			case "close":
				r.CloseError(source)
			}
			if kind == "multi" {
				m.ExpectQuery("SELECT").WillReturnRows(r, sqlmock.NewRows([]string{"id"}).AddRow(3))
			} else {
				m.ExpectQuery("SELECT").WillReturnRows(r)
			}
			rows, e := db.QueryObservedContext(ctx, "SELECT id FROM observation_secret")
			if e != nil {
				t.Fatal("query")
			}
			if s.Snapshot().InFlight != 1 {
				t.Fatal("premature completion")
			}
			if kind == "incomplete" {
				v := s.Finish()
				if v.Incomplete != 1 {
					t.Fatal(v)
				}
				_ = rows.Close()
				if s.Snapshot() != v {
					t.Fatal("late close")
				}
				return
			}
			var n int
			if kind == "early" || kind == "close" {
				rows.Next()
				_ = rows.Scan(&n)
			} else {
				for rows.Next() {
					_ = rows.Scan(&n)
					if kind == "rescan" {
						_ = rows.Scan(&n)
					}
				}
			}
			if kind == "eof" || kind == "rescan" {
				if rows.NextResultSet() {
					t.Fatal("unexpected set")
				}
			}
			if kind == "multi" {
				if !rows.NextResultSet() {
					t.Fatal("missing set")
				}
				for rows.Next() {
					_ = rows.Scan(&n)
				}
				if rows.NextResultSet() {
					t.Fatal("extra set")
				}
			}
			_ = rows.Close()
			_ = rows.Close()
			_ = rows.Err()
			events := observationEvents(t, s)
			if len(events) != 1 {
				t.Fatal("duplicate event")
			}
			ev := events[0]
			switch kind {
			case "eof":
				if !ev.Returned.Known || ev.Returned.Value != 2 || ev.Outcome != "success" {
					t.Fatal(ev)
				}
			case "rescan":
				if ev.Scans.Value != 4 || ev.Consumed.Value != 2 {
					t.Fatal(ev)
				}
			case "multi":
				if !ev.Returned.Known || ev.Returned.Value != 3 {
					t.Fatal(ev)
				}
			case "early":
				if ev.Returned.Known || ev.Consumed.Value != 1 || ev.Outcome != "partial" {
					t.Fatal(ev)
				}
			default:
				if ev.Outcome != "error" || ev.Returned.Known {
					t.Fatal(ev)
				}
			}
			if strings.Contains(fmt.Sprintf("%#v", rows), "observation_secret") || strings.Contains(ev.String(), "private-error-canary") {
				t.Fatal("leak")
			}
		})
	}
}
func TestObservationLegacyAndRow(t *testing.T) {
	db, m := observationDB(t)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{})
	m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	rows, e := db.QueryContext(ctx, "SELECT 1")
	if e != nil {
		t.Fatal(e)
	}
	_ = rows.Close()
	if s.Snapshot().Handoffs != 1 {
		t.Fatal("missing handoff")
	}
	m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	row := db.QueryRowContext(ctx, "SELECT 1")
	if row.Err() != nil || s.Snapshot().InFlight != 1 {
		t.Fatal("Err completed row")
	}
	var n int
	if row.Scan(&n) != nil {
		t.Fatal("scan")
	}
	_ = row.Scan(&n)
	if s.Snapshot().Completed != 1 {
		t.Fatal("row duplicate")
	}
	m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if db.QueryRowContext(ctx, "SELECT 1").Scan(&n) != sql.ErrNoRows {
		t.Fatal("no rows")
	}
	m.ExpectQuery("SELECT").WillReturnError(context.DeadlineExceeded)
	if db.QueryRowContext(ctx, "SELECT 1").Scan(&n) != context.DeadlineExceeded {
		t.Fatal("deadline")
	}
	events := observationEvents(t, s)
	if len(events) != 4 || events[0].Returned.Known || events[1].Consumed.Value != 1 || !events[2].Consumed.Known || events[2].Consumed.Value != 0 || events[3].Code != "deadline" {
		t.Fatal("row boundary")
	}
}
func TestObservationPolicyBudgetAndPlan(t *testing.T) {
	db, m := observationDB(t)
	limit := int64(1)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{QueryLimit: &limit})
	q := db.Table("users").WithContext(ctx).Select("id").Where("id", 1).Limit(1)
	if _, e := q.Plan(ctx); e != nil {
		t.Fatal(e)
	}
	if s.Snapshot().Attempted != 0 {
		t.Fatal("Plan dispatched")
	}
	m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, e := db.ExecContext(ctx, "UPDATE users SET id=1"); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ExecContext(ctx, "UPDATE users SET id=2"); e != observation.ErrQueryBudgetExceeded {
		t.Fatal("budget refusal")
	}
	if _, e := db.Table("users").WithContext(ctx).Delete(); e == nil || e == observation.ErrQueryBudgetExceeded {
		t.Fatal("policy bypass")
	}
	v := s.Finish()
	if v.Dispatched != 1 || v.Rejected != 1 || v.Completed != 1 {
		t.Fatal(v)
	}
}
func TestObservationStructuredRepeatedAndReuse(t *testing.T) {
	db, m := observationDB(t)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{DetectRepeated: true})
	q := db.Table("users").WithContext(ctx).Select("id").Where("id", 1).Limit(1)
	for i := 0; i < 5; i++ {
		m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(i))
		var v map[string]any
		if q.FirstMap(&v) != nil {
			t.Fatal("query")
		}
	}
	v := s.Finish()
	if v.Dispatched != 5 || v.Consumed != 5 || v.SuspectedGroups != 1 || v.MaxRepeat != 5 {
		t.Fatal(v)
	}
}
func TestObservationGenericAndTransactions(t *testing.T) {
	for _, commitError := range []bool{false, true} {
		t.Run(fmt.Sprint(commitError), func(t *testing.T) {
			db, m := observationDB(t)
			ctx, s, _ := observation.Start(context.Background(), observation.Options{})
			m.ExpectBegin()
			m.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(0, 1))
			if commitError {
				m.ExpectCommit().WillReturnError(errors.New("private commit"))
			} else {
				m.ExpectCommit()
			}
			e := db.TransactionContext(ctx, func(tx Tx) error { _, e := tx.DB.ExecContext(ctx, "INSERT INTO users(id) VALUES(1)"); return e })
			if (e != nil) != commitError {
				t.Fatal("transaction result")
			}
			v := s.Finish()
			if v.Dispatched != 1 || v.Successful != 1 || v.TxBoundaries != 2 || v.TxErrors != int64(boolInt(commitError)) {
				t.Fatal(v)
			}
		})
	}
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func TestObservationInternalBudgetRollback(t *testing.T) {
	db, m := observationDB(t)
	db.drv.Dialect = driver.MySQLDialect{}
	limit := int64(1)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{QueryLimit: &limit})
	m.ExpectBegin()
	m.ExpectQuery("SELECT ENGINE").WillReturnRows(sqlmock.NewRows([]string{"ENGINE"}).AddRow("InnoDB"))
	m.ExpectRollback()
	_, e := db.Table("users").WithContext(ctx).AtomicWrite().Insert(map[string]any{"id": 1})
	if !errors.Is(e, observation.ErrQueryBudgetExceeded) {
		t.Fatal("internal budget")
	}
	v := s.Finish()
	if v.Dispatched != 1 || v.Rejected != 1 || v.TxBoundaries != 2 {
		t.Fatal(v)
	}
}

func TestObservationCommandDispatchCounts(t *testing.T) {
	for _, limit := range []int64{1, 2, 3} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			store, m, p, b, fp := commandMock(t)
			ctx, scope, _ := observation.Start(t.Context(), observation.Options{QueryLimit: &limit})
			m.ExpectBegin()
			m.ExpectExec("INSERT INTO.*goquent_commands_v1").WillReturnResult(sqlmock.NewResult(0, 1))
			if limit >= 2 {
				m.ExpectExec("INSERT INTO.*items").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if limit == 3 {
				m.ExpectExec("UPDATE.*goquent_commands_v1").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			r, e := store.apply(ctx, commandTestID, b, fp, p)
			if limit == 3 {
				if e != nil || !r.Applied {
					t.Fatal("command")
				}
			} else if !errors.Is(e, observation.ErrQueryBudgetExceeded) || r.Applied {
				t.Fatal("budget command")
			}
			v := scope.Finish()
			if v.Dispatched != limit || v.TxBoundaries != 2 {
				t.Fatal(v)
			}
			if m.ExpectationsWereMet() != nil {
				t.Fatal("dispatch mismatch")
			}
		})
	}
}
func TestObservationRowNeverScanAndZeroRows(t *testing.T) {
	db, m := observationDB(t)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{})
	m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	row := db.QueryRowContext(ctx, "SELECT 1")
	v := s.Finish()
	if v.Incomplete != 1 || v.Completed != 0 {
		t.Fatal(v)
	}
	_ = row.Scan(new(int))
	if s.Snapshot() != v {
		t.Fatal("late Scan")
	}
	var zero observation.Rows
	if zero.Next() || zero.Err() == nil || zero.Scan(new(int)) == nil || zero.Close() == nil {
		t.Fatal("zero rows")
	}
}

func TestObservationConnectionCancelAndResultMetrics(t *testing.T) {
	for _, source := range []error{context.Canceled, context.DeadlineExceeded, errors.New("private-connection-canary")} {
		db, m := observationDB(t)
		ctx, s, _ := observation.Start(context.Background(), observation.Options{})
		m.ExpectExec("UPDATE").WillReturnError(source)
		if _, e := db.ExecContext(ctx, "UPDATE users SET id=1"); e != source {
			t.Fatal("error identity")
		}
		v := s.Finish()
		if v.Dispatched != 1 || v.Failed != 1 || v.ReturnedKnown != 0 {
			t.Fatal(v)
		}
		for _, e := range observationEvents(t, s) {
			if strings.Contains(e.String(), "private-connection-canary") {
				t.Fatal("cause leak")
			}
		}
	}
	db, m := observationDB(t)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{})
	m.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, e := db.Table("users").WithContext(ctx).ExpectAffected(1).Insert(map[string]any{"id": 1}); e != nil {
		t.Fatal("write")
	}
	ev := observationEvents(t, s)
	if len(ev) != 1 || !ev[0].Affected.Known || ev[0].Affected.Value != 1 || ev[0].Returned.Known {
		t.Fatal("count distinction")
	}
}
func TestObservationGenericReturningAndRawOwnedReads(t *testing.T) {
	db, m := observationDB(t)
	ctx, s, _ := observation.Start(context.Background(), observation.Options{})
	type row struct {
		ID int64 `db:"id"`
	}
	m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	out, e := SelectAll[row](ctx, db, "SELECT id FROM users")
	if e != nil || len(out) != 2 {
		t.Fatal("generic read")
	}
	m.ExpectQuery("INSERT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	if _, e = InsertReturning[row, row](ctx, db, row{3}, Table("users")); e != nil {
		t.Fatal("returning")
	}
	v := s.Finish()
	if v.Dispatched != 2 || v.Consumed != 3 || v.Scans != 3 || v.Incomplete != 0 {
		t.Fatal(v)
	}
}
func TestObservationDifferentOwnersAndUnknownRaw(t *testing.T) {
	ctx, s, _ := observation.Start(context.Background(), observation.Options{DetectRepeated: true})
	for owner := 0; owner < 2; owner++ {
		db, m := observationDB(t)
		for i := 0; i < 3; i++ {
			m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			var v map[string]any
			if db.Table("users").WithContext(ctx).Select("id").Where("id", i).Limit(1).FirstMap(&v) != nil {
				t.Fatal("query")
			}
		}
		m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		if db.QueryRowContext(ctx, "SELECT id FROM users").Scan(new(int)) != nil {
			t.Fatal("raw")
		}
	}
	v := s.Finish()
	if v.SuspectedGroups != 0 || v.MaxRepeat != 3 || v.ShapeUnknown != 2 {
		t.Fatal(v)
	}
}
func TestObservationCustomResultNoExtraCalls(t *testing.T) {
	spy := &observationResultSpy{}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{}).RequireRawApproval("test")
	ctx, s, _ := observation.Start(context.Background(), observation.Options{})
	if _, e := db.ExecContext(ctx, "UPDATE users SET id=1"); e != nil {
		t.Fatal(e)
	}
	if spy.resultCalls != 0 {
		t.Fatal("extra custom Result call")
	}
	if observationEvents(t, s)[0].Affected.Known {
		t.Fatal("invented count")
	}
}

type observationResultSpy struct {
	*sql.DB
	resultCalls int
}

func (s *observationResultSpy) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return s, nil
}
func (s *observationResultSpy) RowsAffected() (int64, error) { s.resultCalls++; return 1, nil }
func (s *observationResultSpy) LastInsertId() (int64, error) { s.resultCalls++; return 1, nil }
