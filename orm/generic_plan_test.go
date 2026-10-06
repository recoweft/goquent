package orm

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/query"
)

type genericSpy struct {
	Executor
	calls []string
	sql   string
	args  []any
	ctx   context.Context
}

func (s *genericSpy) record(method, q string, a []any, c context.Context) {
	s.calls = append(s.calls, method)
	s.sql = q
	s.args = append([]any(nil), a...)
	s.ctx = c
}
func (s *genericSpy) Exec(q string, a ...any) (sql.Result, error) {
	s.record("exec", q, a, nil)
	if s.Executor == nil {
		return captureResult{}, nil
	}
	return s.Executor.Exec(q, a...)
}
func (s *genericSpy) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	s.record("exec-context", q, a, c)
	if s.Executor == nil {
		return captureResult{}, nil
	}
	return s.Executor.ExecContext(c, q, a...)
}
func (s *genericSpy) Query(q string, a ...any) (*sql.Rows, error) {
	s.record("query", q, a, nil)
	return s.Executor.Query(q, a...)
}
func (s *genericSpy) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	s.record("query-context", q, a, c)
	return s.Executor.QueryContext(c, q, a...)
}
func (s *genericSpy) QueryRow(q string, a ...any) *sql.Row {
	s.record("row", q, a, nil)
	return s.Executor.QueryRow(q, a...)
}
func (s *genericSpy) QueryRowContext(c context.Context, q string, a ...any) *sql.Row {
	s.record("row-context", q, a, c)
	return s.Executor.QueryRowContext(c, q, a...)
}

func genericSettings(t *testing.T, name string, auto bool) Settings {
	t.Helper()
	schema := genericSchema(t, name)
	policies, err := NewPolicySet(TablePolicy{Table: "users", TenantColumn: "tenant_id", PIIColumns: []string{"secret"}, ImmutableColumns: []string{"fixed"}, ForbiddenColumns: []string{"forbidden"}})
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 1})
	if err != nil {
		t.Fatal(err)
	}
	return NewSettings(policies, RiskConfig{}, tenant).WithTenantPolicy("generic-test", schema, auto)
}
func genericDialect(name string) driver.Dialect {
	if name == "postgres" {
		return driver.PostgresDialect{}
	}
	return driver.MySQLDialect{}
}
func diagnosticError(p *QueryPlan, e error) error {
	if e != nil {
		return e
	}
	return EnsurePlanExecutable(p)
}

type genericSecret struct {
	Secret int `db:"secret"`
}
type genericID struct {
	ID int `db:"id"`
}

func TestGenericPlansAndRejections(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			spy := &genericSpy{}
			db := NewDBWithExecutor(spy, genericDialect(dialect), WithSettings(genericSettings(t, dialect, false)))
			table := Table("users")
			bad := map[string]any{"tenant_id": 2, "id": 3, "score": 4}
			good := map[string]any{"tenant_id": 1, "id": 3, "score": 4}
			cases := []struct {
				name string
				run  func() error
			}{
				{"insert", func() error { _, e := Insert(nil, db, bad, table); return e }},
				{"insert plan", func() error { return diagnosticError(PlanInsert(nil, db, bad, table)) }},
				{"batch all rows", func() error { _, e := InsertMany(nil, db, []map[string]any{good, bad}, table); return e }},
				{"batch plan", func() error { return diagnosticError(PlanInsertMany(nil, db, []map[string]any{good, bad}, table)) }},
				{"insert returning", func() error { _, e := InsertReturning[genericID](nil, db, bad, table); return e }},
				{"batch returning", func() error {
					_, e := InsertManyReturning[genericID](nil, db, []map[string]any{good, bad}, table)
					return e
				}},
				{"upsert", func() error {
					_, e := Upsert(nil, db, bad, table, ConflictColumns("tenant_id", "id"), UpdateColumns("score"))
					return e
				}},
				{"upsert plan", func() error {
					return diagnosticError(PlanUpsert(nil, db, bad, table, ConflictColumns("tenant_id", "id"), UpdateColumns("score")))
				}},
				{"upsert many", func() error {
					_, e := UpsertMany(nil, db, []map[string]any{good, bad}, table, ConflictColumns("tenant_id", "id"), UpdateColumns("score"))
					return e
				}},
				{"upsert many plan", func() error {
					return diagnosticError(PlanUpsertMany(nil, db, []map[string]any{good, bad}, table, ConflictColumns("tenant_id", "id"), UpdateColumns("score")))
				}},
				{"upsert returning", func() error {
					_, e := UpsertReturning[genericID](nil, db, bad, table, ConflictColumns("tenant_id", "id"), UpdateColumns("score"))
					return e
				}},
				{"upsert many returning", func() error {
					_, e := UpsertManyReturning[genericID](nil, db, []map[string]any{good, bad}, table, ConflictColumns("tenant_id", "id"), UpdateColumns("score"))
					return e
				}},
				{"missing tenant", func() error { _, e := Insert(nil, db, map[string]any{"id": 3}, table); return e }},
				{"inferred PII", func() error { _, e := InsertReturning[genericSecret](nil, db, good, table); return e }},
				{"returning option", func() error { _, e := Insert(nil, db, good, table, Returning("forbidden")); return e }},
				{"returning plan", func() error { return diagnosticError(PlanInsert(nil, db, good, table, Returning("secret"))) }},
				{"unsupported conflict", func() error {
					_, e := Upsert(nil, db, good, table, ConflictTargetRaw("(tenant_id, id)"), UpdateColumns("score"))
					return e
				}},
				{"uncovered unique", func() error {
					_, e := Upsert(nil, db, good, table, ConflictColumns("id"), UpdateColumns("score"))
					return e
				}},
				{"update other tenant", func() error { _, e := Update(nil, db, bad, table, PK("tenant_id", "id"), WherePK()); return e }},
				{"update returning other tenant", func() error {
					_, e := UpdateReturning[genericID](nil, db, bad, table, PK("tenant_id", "id"), WherePK())
					return e
				}},
				{"update plan other tenant", func() error {
					return diagnosticError(PlanUpdate(nil, db, bad, table, PK("tenant_id", "id"), WherePK()))
				}},
			}
			for _, col := range []string{"tenant_id", "fixed", "forbidden", "secret"} {
				col := col
				cases = append(cases, struct {
					name string
					run  func() error
				}{"assignment " + col, func() error {
					_, e := Update(nil, db, map[string]any{"id": 3, col: 1}, table, PK("id"), WherePK())
					return e
				}})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					e := tc.run()
					if !errors.Is(e, ErrBlockedOperation) {
						t.Fatalf("expected blocked: %v", e)
					}
					if len(spy.calls) != 0 {
						t.Fatal(spy.calls)
					}
				})
			}
			for _, settings := range []Settings{db.settings.WithExecutionContext(ExecutionContext{}), db.settings.WithTenantPolicy("wrong", ApplicationSchema{}, false)} {
				_, err := Insert(nil, db.WithOptions(WithSettings(settings)), good, table)
				if !errors.Is(err, ErrBlockedOperation) {
					t.Fatal(err)
				}
			}
			// Corresponding Query and generic operations retain the same rejection identity/reason.
			_, qe := db.Table("users").Insert(bad)
			_, ge := Insert(nil, db, bad, table)
			if qe.Error() != ge.Error() {
				t.Fatalf("%v != %v", qe, ge)
			}
			if len(spy.calls) != 0 {
				t.Fatal(spy.calls)
			}
		})
	}
}

func TestGenericPlansAllowedAndPrivateLifecycle(t *testing.T) {
	for _, name := range []string{"mysql", "postgres"} {
		for _, contextual := range []bool{false, true} {
			t.Run(fmt.Sprint(name, contextual), func(t *testing.T) {
				spy := &genericSpy{}
				db := NewDBWithExecutor(spy, genericDialect(name), WithSettings(genericSettings(t, name, true)))
				var ctx context.Context
				if contextual {
					ctx = context.WithValue(context.Background(), struct{}{}, "marker")
				}
				data := map[string]any{"id": 3, "score": 4}
				opts := []WriteOpt{Table("users")}
				p, err := PlanInsert(ctx, db, data, opts...)
				if err != nil {
					t.Fatal(err)
				}
				if err = EnsurePlanExecutable(p); err != nil {
					t.Fatal(err)
				}
				if len(spy.calls) != 0 || data["tenant_id"] != nil {
					t.Fatal("planning executed or mutated input")
				}
				_, err = Insert(ctx, db, data, opts...)
				if err != nil {
					t.Fatal(err)
				}
				if spy.sql != p.SQL || !reflect.DeepEqual(spy.args, p.Params) || len(spy.calls) != 1 || spy.ctx != ctx {
					t.Fatalf("dispatch %s %v %+v", spy.sql, spy.args, spy.calls)
				}
				method := "exec"
				if contextual {
					method = "exec-context"
				}
				if spy.calls[0] != method {
					t.Fatal(spy.calls)
				}
				r, err := buildInsertInput(db, data, applyWriteOpts(opts))
				if err != nil {
					t.Fatal(err)
				}
				prepared, err := prepareWrite(ctx, db, r)
				if err != nil {
					t.Fatal(err)
				}
				diagnostic := prepared.Diagnostic.(*QueryPlan)
				diagnostic.SQL = "DELETE FROM users"
				diagnostic.Params[0] = 99
				diagnostic.RiskLevel = RiskLow
				data["id"] = 100
				if _, err = prepared.Exec(); err != nil {
					t.Fatal(err)
				}
				if spy.sql != p.SQL || !reflect.DeepEqual(spy.args, p.Params) {
					t.Fatal("public plan/input changed execution")
				}
				if _, err = prepared.Exec(); !errors.Is(err, ErrBlockedOperation) {
					t.Fatal("replayed", err)
				}
				var public QueryPlan
				blob, _ := json.Marshal(p)
				if err = json.Unmarshal(blob, &public); err != nil {
					t.Fatal(err)
				}
				_, err = querybridge.Prepare(querybridge.Request{Base: &public, Settings: db.settings, Executor: spy, Dialect: db.Dialect(), Operation: "select"})
				if !errors.Is(err, ErrBlockedOperation) && err == nil {
					t.Fatal("public artifact accepted")
				}
				if len(spy.calls) != 2 {
					t.Fatal(spy.calls)
				}
			})
		}
	}
}

func TestScopedDestinationAndIgnoredOptions(t *testing.T) {
	for index, group := range [][]WriteOpt{
		{Table("other; DROP TABLE users"), TablePath("wrong", "table"), SchemaName("wrong")},
		{Columns("id"), Omit("score", "secret"), PK("secret"), WherePK()},
		{SetRaw("secret", "forbidden"), SetExpr("tenant_id", "?", 99), SetColumn("score", "secret"), Increment("score", 999)},
		{ConflictColumns("secret"), ConflictWhere("bad"), ConflictConstraint("bad"), ConflictTargetRaw("bad"), UpdateColumns("secret"), ConflictDoNothing(), ExpectAffected(999)},
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			dest := &genericSpy{Executor: sqlDB}
			source := &genericSpy{}
			db := NewDBWithExecutor(dest, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", false)))
			base := NewDBWithExecutor(source, driver.MySQLDialect{}).Table("users").Where("tenant_id", 1).Where("id", 3)
			mock.ExpectQuery(`UPDATE "users" SET "score" = \$1 WHERE \("tenant_id" = \$2 AND "id" = \$3\) RETURNING "id"`).WithArgs(4, 1, 3).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3)).RowsWillBeClosed()
			row, err := UpdateByReturningWithOptions[genericID](context.Background(), db, base, map[string]any{"score": 4}, group)
			if err != nil || row.ID != 3 {
				t.Fatalf("row=%v error=%v", row, err)
			}
			if len(dest.calls) != 1 || len(source.calls) != 0 {
				t.Fatalf("dest %v source %v", dest.calls, source.calls)
			}
			_, err = UpdateByReturningWithOptions[genericID](nil, db, base, map[string]any{"secret": 4}, group)
			if !errors.Is(err, ErrBlockedOperation) || len(dest.calls) != 1 {
				t.Fatalf("ignored options hid data: %v %v", err, dest.calls)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	// A source query's plan cannot bless another destination's tenant/settings.
	source := &genericSpy{}
	dest := &genericSpy{}
	src := NewDBWithExecutor(source, driver.MySQLDialect{}, WithSettings(genericSettings(t, "mysql", false)))
	tenant, _ := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
	db := NewDBWithExecutor(dest, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", false).WithExecutionContext(tenant)))
	q := src.Table("users").Select("id").Where("tenant_id", 1).Where("id", 3)
	if _, err := q.Plan(nil); err != nil {
		t.Fatal(err)
	}
	_, err := SelectOneBy[genericID](nil, db, q)
	if !errors.Is(err, ErrBlockedOperation) {
		t.Fatal(err)
	}
	_, err = SelectAllBy[genericID](nil, db, q)
	if !errors.Is(err, ErrBlockedOperation) {
		t.Fatal(err)
	}
	_, err = UpdateByReturning[genericID](nil, db, q, map[string]any{"score": 4})
	if !errors.Is(err, ErrBlockedOperation) {
		t.Fatal(err)
	}
	if len(dest.calls)+len(source.calls) != 0 {
		t.Fatal("destination mismatch dispatched")
	}
}

type opaqueGenericValue struct{ calls *int }

func (v opaqueGenericValue) Value() (sqldriver.Value, error) { *v.calls++; return int64(9), nil }
func (v opaqueGenericValue) String() string                  { panic("String invoked") }
func (v opaqueGenericValue) MarshalJSON() ([]byte, error)    { panic("MarshalJSON invoked") }
func TestGenericOpaqueValuesAndRawBoundary(t *testing.T) {
	calls := 0
	v := opaqueGenericValue{&calls}
	spy := &genericSpy{}
	compatible := NewDBWithExecutor(spy, driver.MySQLDialect{})
	if _, err := PlanInsert(nil, compatible, map[string]any{"score": v}, Table("users")); err != nil {
		t.Fatal(err)
	}
	strict := compatible.WithOptions(WithSettings(genericSettings(t, "mysql", true)))
	if _, err := PlanInsert(nil, strict, map[string]any{"id": 3, "score": v}, Table("users")); !errors.Is(err, ErrBlockedOperation) {
		t.Fatal(err)
	}
	if calls != 0 || len(spy.calls) != 0 {
		t.Fatal("planning called user code/executor")
	}
	for _, read := range []func() error{
		func() error { _, e := SelectOne[genericID](nil, strict, "SELECT id FROM users"); return e },
		func() error { _, e := SelectAll[genericID](nil, strict, "SELECT id FROM users"); return e },
		func() error { _, e := SelectStruct[genericID](nil, strict, "SELECT id FROM users"); return e },
		func() error { _, e := SelectStructs[genericID](nil, strict, "SELECT id FROM users"); return e },
		func() error {
			_, e := strict.RequireRawApproval("reason").TouchedTables("users").SelectMaps(nil, "SELECT id FROM users")
			return e
		},
		func() error { _, e := strict.SelectMap(nil, "SELECT id FROM users"); return e },
	} {
		if e := read(); !errors.Is(e, ErrBlockedOperation) {
			t.Fatal(e)
		}
	}
	if len(spy.calls) != 0 {
		t.Fatal(spy.calls)
	}
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	mock.ExpectExec("INSERT").WithArgs(int64(9)).WillReturnResult(sqlmock.NewResult(7, 1))
	_, err = Insert(nil, NewDB(sqlDB, driver.MySQLDialect{}), map[string]any{"score": v}, Table("users"))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGenericHooksInheritStrict(t *testing.T) {
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.MySQLDialect{}, WithSettings(genericSettings(t, "mysql", false)))
	tx := Tx{DB: db}
	for _, h := range []TransactionHook{InsertHook("bad", map[string]any{"tenant_id": 2}, Table("users")), InsertManyHook("bad-many", []map[string]any{{"tenant_id": 1}, {"tenant_id": 2}}, Table("users"))} {
		if err := h.Run(context.Background(), tx); !errors.Is(err, ErrBlockedOperation) {
			t.Fatal(err)
		}
	}
	if len(spy.calls) != 0 {
		t.Fatal(spy.calls)
	}
}

func TestGenericLiteralPKAndPlanMetadata(t *testing.T) {
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{})
	p, e := PlanUpdate(nil, db, map[string]any{"a.b": nil, "score": 3}, Table("users"), PK("a.b"), WherePK())
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(p.SQL, `"a.b"=$2`) || len(p.Params) != 2 || p.Params[1] != nil {
		t.Fatalf("%s %v", p.SQL, p.Params)
	}
	if len(spy.calls) != 0 {
		t.Fatal(spy.calls)
	}
}

func genericSchema(t *testing.T, name string) ApplicationSchema {
	t.Helper()
	typ := "INT"
	if name == "postgres" {
		typ = "integer"
	}
	var cols []query.WriteKeyColumn
	for _, n := range []string{"tenant_id", "id", "score", "deleted", "secret", "fixed", "forbidden", "active"} {
		cols = append(cols, query.WriteKeyColumn{Name: n, DBType: typ, Bits: 32})
	}
	schema, err := NewApplicationSchema(ApplicationSchemaInput{Database: "generic-test", Dialect: name, Tables: []ApplicationTable{{Table: "users", PlainTable: true, Columns: cols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols[:2]}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return schema
}
