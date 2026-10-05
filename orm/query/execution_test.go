package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

type dispatchSpy struct {
	*sql.DB
	calls []string
}

func (s *dispatchSpy) Exec(q string, a ...any) (sql.Result, error) {
	s.calls = append(s.calls, "exec")
	return s.DB.Exec(q, a...)
}
func (s *dispatchSpy) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	s.calls = append(s.calls, "exec-context")
	return s.DB.ExecContext(c, q, a...)
}
func (s *dispatchSpy) Query(q string, a ...any) (*sql.Rows, error) {
	s.calls = append(s.calls, "rows")
	return s.DB.Query(q, a...)
}
func (s *dispatchSpy) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	s.calls = append(s.calls, "rows-context")
	return s.DB.QueryContext(c, q, a...)
}
func (s *dispatchSpy) QueryRow(q string, a ...any) *sql.Row {
	s.calls = append(s.calls, "row")
	return s.DB.QueryRow(q, a...)
}
func (s *dispatchSpy) QueryRowContext(c context.Context, q string, a ...any) *sql.Row {
	s.calls = append(s.calls, "row-context")
	return s.DB.QueryRowContext(c, q, a...)
}

func TestPlannedExecutionReadDispatch(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, contextual := range []bool{false, true} {
			for _, method := range []string{"First", "FirstMap", "Get", "GetMaps", "Count"} {
				t.Run(driverName(pg)+"/"+method+"/"+boolName(contextual), func(t *testing.T) {
					db, mock, err := sqlmock.New()
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					spy := &dispatchSpy{DB: db}
					dialect := testDialect(pg)
					q := New(spy, "users", dialect).Where("id", 7)
					if method != "Count" {
						q.Select("id")
					}
					if contextual {
						q.WithContext(context.Background())
					}
					p, err := q.Plan(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					if len(spy.calls) != 0 {
						t.Fatal("Plan dispatched")
					}
					sqlText := p.SQL
					mode := "rows"
					if method == "Count" {
						sqlText = "SELECT COUNT(*) FROM " + dialect.QuoteIdent("users") + " WHERE " + dialect.QuoteIdent("id") + " = ?"
						if pg {
							sqlText = sqlText[:len(sqlText)-1] + "$1"
						}
						mode = "row"
					}
					mock.ExpectQuery(regexp.QuoteMeta(sqlText)).WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
					type row struct {
						ID int `db:"id"`
					}
					switch method {
					case "First":
						var out row
						err = q.First(&out)
						if out.ID != 7 {
							t.Fatalf("out=%+v", out)
						}
					case "Get":
						var out []row
						err = q.Get(&out)
						if len(out) != 1 || out[0].ID != 7 {
							t.Fatalf("out=%+v", out)
						}
					case "FirstMap":
						var out map[string]any
						err = q.FirstMap(&out)
						if out["id"] != int64(7) {
							t.Fatalf("out=%+v", out)
						}
					case "GetMaps":
						var out []map[string]any
						err = q.GetMaps(&out)
						if len(out) != 1 || out[0]["id"] != int64(7) {
							t.Fatalf("out=%+v", out)
						}
					case "Count":
						var out int64
						out, err = q.Count()
						if out != 7 {
							t.Fatal(out)
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					if contextual {
						mode += "-context"
					}
					if !reflect.DeepEqual(spy.calls, []string{mode}) {
						t.Fatal(spy.calls)
					}
					if err := mock.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
func driverName(pg bool) string {
	if pg {
		return "postgres"
	}
	return "mysql"
}
func boolName(v bool) string {
	if v {
		return "context"
	}
	return "plain"
}
func testDialect(pg bool) driver.Dialect {
	if pg {
		return driver.PostgresDialect{}
	}
	return driver.MySQLDialect{}
}

func TestPlannedExecutionWriteDispatch(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, contextual := range []bool{false, true} {
			for _, method := range []string{"Insert", "InsertBatch", "InsertOrIgnore", "Upsert", "UpdateOrInsert", "InsertUsing", "Update", "Delete", "InsertGetId"} {
				t.Run(driverName(pg)+"/"+method+"/"+boolName(contextual), func(t *testing.T) {
					db, mock, err := sqlmock.New()
					if err != nil {
						t.Fatal(err)
					}
					defer db.Close()
					spy := &dispatchSpy{DB: db}
					q := New(spy, "users", testDialect(pg)).Where("id", 7)
					if contextual {
						q.WithContext(context.Background())
					}
					data := map[string]any{"id": 7}
					batch := []map[string]any{data}
					var plan *QueryPlan
					var run func() (sql.Result, error)
					switch method {
					case "Insert", "InsertGetId":
						plan, err = q.PlanInsert(nil, data)
						run = func() (sql.Result, error) { return q.Insert(data) }
					case "InsertBatch":
						plan, err = q.PlanInsertBatch(nil, batch)
						run = func() (sql.Result, error) { return q.InsertBatch(batch) }
					case "InsertOrIgnore":
						plan, err = q.planInsertOrIgnore(nil, batch)
						run = func() (sql.Result, error) { return q.InsertOrIgnore(batch) }
					case "Upsert":
						plan, err = q.planUpsert(nil, batch, []string{"id"}, []string{"id"})
						run = func() (sql.Result, error) { return q.Upsert(batch, []string{"id"}, []string{"id"}) }
					case "UpdateOrInsert":
						plan, err = q.planUpdateOrInsert(nil, data, data)
						run = func() (sql.Result, error) { return q.UpdateOrInsert(data, data) }
					case "InsertUsing":
						sub := New(spy, "users", testDialect(pg)).Select("id").Where("id", 7)
						plan, err = q.planInsertUsing(nil, []string{"id"}, sub)
						run = func() (sql.Result, error) { return q.InsertUsing([]string{"id"}, sub) }
					case "Update":
						plan, err = q.PlanUpdate(nil, map[string]any{"score": 9})
						run = func() (sql.Result, error) { return q.Update(map[string]any{"score": 9}) }
					case "Delete":
						plan, err = q.PlanDelete(nil)
						run = q.Delete
					}
					if err != nil {
						t.Fatal(err)
					}
					if len(spy.calls) != 0 {
						t.Fatal("planning executed")
					}
					sqlText := plan.SQL
					mode := "exec"
					if method == "InsertGetId" && pg {
						sqlText += " RETURNING \"id\""
						mode = "row"
						mock.ExpectQuery(regexp.QuoteMeta(sqlText)).WithArgs(7).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
					} else {
						expectation := mock.ExpectExec(regexp.QuoteMeta(sqlText))
						if method == "Update" {
							expectation.WithArgs(9, 7)
						} else {
							expectation.WithArgs(7)
						}
						expectation.WillReturnResult(sqlmock.NewResult(42, 1))
					}
					if method == "InsertGetId" {
						var id int64
						id, err = q.InsertGetId(data)
						if id != 42 {
							t.Fatal(id)
						}
					} else {
						var result sql.Result
						result, err = run()
						if err == nil {
							n, e := result.RowsAffected()
							if e != nil || n != 1 {
								t.Fatalf("affected=%d %v", n, e)
							}
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					if contextual {
						mode += "-context"
					}
					if !reflect.DeepEqual(spy.calls, []string{mode}) {
						t.Fatal(spy.calls)
					}
					if err := mock.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestPlannedExecutionRejectsEveryTerminalBeforeDispatch(t *testing.T) {
	for _, pg := range []bool{false, true} {
		methods := map[string]func(*Query) error{
			"First":       func(q *Query) error { var out struct{ ID int }; return q.First(&out) },
			"Get":         func(q *Query) error { var out []struct{ ID int }; return q.Get(&out) },
			"FirstMap":    func(q *Query) error { var out map[string]any; return q.FirstMap(&out) },
			"GetMaps":     func(q *Query) error { var out []map[string]any; return q.GetMaps(&out) },
			"Count":       func(q *Query) error { _, e := q.Count(); return e },
			"Insert":      func(q *Query) error { _, e := q.Insert(map[string]any{"id": 7, "tenant_id": 2}); return e },
			"InsertGetId": func(q *Query) error { _, e := q.InsertGetId(map[string]any{"id": 7, "tenant_id": 2}); return e },
			"InsertBatch": func(q *Query) error {
				_, e := q.InsertBatch([]map[string]any{{"id": 7, "tenant_id": 1}, {"id": 8, "tenant_id": 2}})
				return e
			},
			"InsertOrIgnore": func(q *Query) error { _, e := q.InsertOrIgnore([]map[string]any{{"id": 7, "tenant_id": 2}}); return e },
			"Upsert": func(q *Query) error {
				_, e := q.Upsert([]map[string]any{{"id": 7, "tenant_id": 2, "score": 1}}, []string{"tenant_id", "id"}, []string{"score"})
				return e
			},
			"UpdateOrInsert": func(q *Query) error {
				_, e := q.UpdateOrInsert(map[string]any{"id": 7, "tenant_id": 2}, map[string]any{"score": 1})
				return e
			},
			"InsertUsing": func(q *Query) error { _, e := q.InsertUsing([]string{"id"}, q); return e },
			"Update":      func(q *Query) error { _, e := q.Update(map[string]any{"score": 1}); return e },
			"Delete":      func(q *Query) error { _, e := q.Delete(); return e },
		}
		for name, run := range methods {
			t.Run(driverName(pg)+"/"+name, func(t *testing.T) {
				spy := &recordingExec{}
				q := NewWithSettings(spy, "users", testDialect(pg), tenantTestSettings(t, driverName(pg), false)).Select("id").Where("tenant_id", 2)
				if err := run(q); !errors.Is(err, ErrBlockedOperation) {
					t.Fatalf("err=%v", err)
				}
				if spy.calls != 0 {
					t.Fatalf("calls=%d", spy.calls)
				}
			})
		}
	}
}

func TestPrivateExecutionIgnoresPublicVerdictsAndValues(t *testing.T) {
	spy := &recordingExec{}
	q := New(spy, "users", driver.MySQLDialect{})
	p, err := q.PlanDelete(nil)
	if err != nil {
		t.Fatal(err)
	}
	p.Blocked = false
	p.RequiredApproval = false
	p.RiskLevel = RiskLow
	p.Warnings = nil
	p.SQL = "SELECT 1"
	p.Params = nil
	if _, err = q.executeResult(p); !errors.Is(err, ErrBlockedOperation) {
		t.Fatalf("forged verdict: %v", err)
	}
	raw, _ := json.Marshal(p)
	var decoded QueryPlan
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, err = q.executeResult(&decoded); !errors.Is(err, ErrBlockedOperation) {
		t.Fatalf("JSON: %v", err)
	}
	if spy.calls != 0 {
		t.Fatal(spy.calls)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	q = New(db, "users", driver.MySQLDialect{})
	p, err = q.PlanInsert(nil, map[string]any{"id": 7})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`id`) VALUES (?)")).WithArgs(7).WillReturnResult(sqlmock.NewResult(7, 1))
	p.SQL = "DELETE FROM users"
	p.Params[0] = 99
	p.Columns[0].Name = "secret"
	p.Tables[0].Name = "other"
	if _, err = q.executeResult(p); err != nil {
		t.Fatal(err)
	}
	if _, err = q.executeResult(p); !errors.Is(err, ErrBlockedOperation) {
		t.Fatalf("replay: %v", err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlannedExecutionDiagnosticParity(t *testing.T) {
	for _, pg := range []bool{false, true} {
		spy := &recordingExec{}
		newQuery := func() *Query {
			return NewWithSettings(spy, "users", testDialect(pg), tenantTestSettings(t, driverName(pg), false)).Select("id").Where("tenant_id", 2)
		}
		for _, kind := range []string{"select", "update", "delete", "insert"} {
			q := newQuery()
			var p *QueryPlan
			var planned, executed error
			switch kind {
			case "select":
				p, planned = q.Plan(nil)
				var out map[string]any
				executed = q.FirstMap(&out)
			case "update":
				p, planned = q.PlanUpdate(nil, map[string]any{"score": 1})
				_, executed = q.Update(map[string]any{"score": 1})
			case "delete":
				p, planned = q.PlanDelete(nil)
				_, executed = q.Delete()
			case "insert":
				p, planned = q.PlanInsert(nil, map[string]any{"tenant_id": 2})
				_, executed = q.Insert(map[string]any{"tenant_id": 2})
			}
			if planned == nil {
				planned = ensurePlanExecutable(p)
			}
			if planned == nil || executed == nil || planned.Error() != executed.Error() {
				t.Fatalf("%s %s: plan=%v execution=%v", driverName(pg), kind, planned, executed)
			}
		}
		if spy.calls != 0 {
			t.Fatal(spy.calls)
		}
	}
}

func TestReturningReinspectionAndPrivateOriginal(t *testing.T) {
	spy := &recordingExec{}
	q := NewWithSettings(spy, "users", driver.PostgresDialect{}, tenantTestSettings(t, "postgres", false))
	p, err := q.PlanInsert(nil, map[string]any{"tenant_id": 1, "id": 7})
	if err != nil {
		t.Fatal(err)
	}
	originalSQL := p.SQL
	p.SQL = "DELETE FROM users"
	p.Params[0] = 99
	p.Tables[0].Name = "other"
	if err = q.planReturning(p, []string{"secret"}); !errors.Is(err, ErrBlockedOperation) {
		t.Fatalf("PII returning: %v", err)
	}
	if err = q.planReturning(p, []string{"id"}); err != nil {
		t.Fatal(err)
	}
	if p.SQL != originalSQL+" RETURNING \"id\"" || p.Params[0] != 7 {
		t.Fatalf("public input affected statement: %s %v", p.SQL, p.Params)
	}
	if !reflect.DeepEqual(p.Metadata["returning_columns"], []string{"id"}) {
		t.Fatal(p.Metadata)
	}
	if err = ensurePlanExecutable(p); err != nil {
		t.Fatal(err)
	}
	if spy.calls != 0 {
		t.Fatal(spy.calls)
	}
}

func TestPlannedExecutionBoolAndScanErrors(t *testing.T) {
	for _, many := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		q := New(db, "users", driver.MySQLDialect{}).Select("active")
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow([]byte("1"))).RowsWillBeClosed()
		type row struct {
			Active bool `db:"active"`
		}
		if many {
			var out []row
			err = q.Get(&out)
			if len(out) != 1 || !out[0].Active {
				t.Fatal(out)
			}
		} else {
			var out row
			err = q.First(&out)
			if !out.Active {
				t.Fatal(out)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow("not-a-bool")).RowsWillBeClosed()
		if many {
			var out []row
			err = q.Get(&out)
		} else {
			var out row
			err = q.First(&out)
		}
		if err == nil {
			t.Fatal("scan error lost")
		}
		if err = mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
}
