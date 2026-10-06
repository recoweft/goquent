package orm

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

func TestGenericReturningDispatchAndResults(t *testing.T) {
	for _, method := range []string{"insert", "update", "upsert", "insert-many", "upsert-many", "result", "scoped"} {
		for _, contextual := range []bool{false, true} {
			t.Run(method+map[bool]string{true: "-context", false: "-plain"}[contextual], func(t *testing.T) {
				sqlDB, mock, err := sqlmock.New()
				if err != nil {
					t.Fatal(err)
				}
				defer sqlDB.Close()
				spy := &genericSpy{Executor: sqlDB}
				db := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", true)))
				data := map[string]any{"id": 3, "active": true}
				var ctx context.Context
				if contextual {
					ctx = context.WithValue(context.Background(), struct{}{}, "test")
				}
				opts := []WriteOpt{Table("users"), Returning("active")}
				if strings.HasPrefix(method, "upsert") {
					opts = append(opts, ConflictColumns("tenant_id", "id"), UpdateColumns("active"))
				}
				mock.ExpectQuery("RETURNING").WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow([]byte("1"))).RowsWillBeClosed()
				type activeRow struct {
					Active bool `db:"active"`
				}
				var out activeRow
				switch method {
				case "insert":
					out, err = InsertReturning[activeRow](ctx, db, data, opts...)
				case "update":
					out, err = UpdateReturning[activeRow](ctx, db, data, append(opts, PK("id"), WherePK())...)
				case "upsert":
					out, err = UpsertReturning[activeRow](ctx, db, data, opts...)
				case "insert-many":
					var rows []activeRow
					rows, err = InsertManyReturning[activeRow](ctx, db, []map[string]any{data}, opts...)
					if len(rows) == 1 {
						out = rows[0]
					}
				case "upsert-many":
					var rows []activeRow
					rows, err = UpsertManyReturning[activeRow](ctx, db, []map[string]any{data}, opts...)
					if len(rows) == 1 {
						out = rows[0]
					}
				case "result":
					var res sql.Result
					res, err = Insert(ctx, db, data, append(opts, ExpectAffected(1))...)
					if err == nil {
						n, e := res.RowsAffected()
						if n != 1 || e != nil {
							t.Fatal(n, e)
						}
						if _, e = res.LastInsertId(); e == nil {
							t.Fatal("RETURNING LastInsertId")
						}
						out.Active = true
					}
				case "scoped":
					out, err = UpdateByReturning[activeRow](ctx, db, db.Table("users").Where("id", 3), map[string]any{"active": true})
				}
				if err != nil || !out.Active {
					t.Fatalf("%v %v", out, err)
				}
				want := "query"
				if contextual {
					want = "query-context"
				}
				if !reflect.DeepEqual(spy.calls, []string{want}) || spy.ctx != ctx {
					t.Fatal(spy.calls, spy.ctx)
				}
				if !strings.Contains(spy.sql, `RETURNING "active"`) {
					t.Fatal(spy.sql)
				}
				if err = mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestGenericReturningErrorsAndOptionOrder(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	spy := &genericSpy{Executor: sqlDB}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", true)))
	base := db.Table("users").Where("id", 3)
	first, last := errors.New("first"), errors.New("last")
	opts := []WriteOpt{Returning("secret"), Returning(), NoRowsAs(first), NoRowsAs(last), ExpectAffected(999)}
	mock.ExpectQuery(`RETURNING "id"`).WillReturnRows(sqlmock.NewRows([]string{"id"})).RowsWillBeClosed()
	_, err = UpdateByReturningWithOptions[genericID](nil, db, base, map[string]any{"score": 3}, opts)
	var affected RowsAffectedError
	if !errors.Is(err, last) || errors.Is(err, first) || !errors.As(err, &affected) || affected.Actual != 0 || affected.Expected != 1 {
		t.Fatal(err)
	}
	// A driver error is not reclassified as NoRowsAs.
	driverErr := errors.New("driver failure")
	mock.ExpectQuery("UPDATE").WillReturnError(driverErr)
	_, err = UpdateByReturningWithOptions[genericID](nil, db, base, map[string]any{"score": 3}, opts)
	if !errors.Is(err, driverErr) || errors.Is(err, last) {
		t.Fatal(err)
	}
	// PII, including a changed final Returning option, rejects without dispatch.
	_, err = UpdateByReturningWithOptions[genericID](nil, db, base, map[string]any{"score": 3}, append(opts, Returning("secret")))
	if !errors.Is(err, ErrBlockedOperation) || errors.Is(err, last) || len(spy.calls) != 2 {
		t.Fatal(err, spy.calls)
	}
	// Scan failures are returned and rows close.
	mock.ExpectQuery("UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("not an integer")).RowsWillBeClosed()
	_, err = UpdateByReturningWithOptions[genericID](nil, db, base, map[string]any{"score": 3}, opts)
	if err == nil || errors.Is(err, last) {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	// Unsupported dialect never calls an executor.
	mysqlSpy := &genericSpy{}
	mysqlDB := NewDBWithExecutor(mysqlSpy, driver.MySQLDialect{}, WithSettings(genericSettings(t, "mysql", true)))
	_, err = InsertReturning[genericID](nil, mysqlDB, map[string]any{"id": 3}, Table("users"))
	if err == nil || !strings.Contains(err.Error(), "not supported") || len(mysqlSpy.calls) != 0 {
		t.Fatal(err)
	}
}

func TestScopedBranchesAliasesSoftDeleteAndSettingsIsolation(t *testing.T) {
	for _, name := range []string{"mysql", "postgres"} {
		t.Run(name, func(t *testing.T) {
			spy := &genericSpy{}
			db := NewDBWithExecutor(spy, genericDialect(name), WithSettings(genericSettings(t, name, false)))
			for _, base := range []*query.Query{
				db.Table("users u").Select("u.id").Where("u.tenant_id", 1).OrWhere("u.id", 3),
				db.Table("users u").Select("u.id").WhereNot(func(q *query.Query) { q.Where("u.tenant_id", 1) }),
				db.Table("users u").Select("u.id").WhereRawNoArgs("tenant_id = 1"),
			} {
				if _, err := SelectAllBy[genericID](nil, db, base); !errors.Is(err, ErrBlockedOperation) {
					t.Fatal(err)
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
			spy.Executor = sqlDB
			mock.ExpectQuery("SELECT").WithArgs(1, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3)).RowsWillBeClosed()
			rows, err := SelectAllBy[genericID](nil, db, db.Table("users u").Select("u.id").Where("u.tenant_id", 1).OrWhere("u.tenant_id", 1))
			if err != nil || len(rows) != 1 {
				t.Fatal(rows, err)
			}
			if len(spy.calls) != 1 {
				t.Fatal(spy.calls)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
	// Planning on a source DB must not persist its generated soft-delete filters.
	sourcePolicy, _ := NewPolicySet(TablePolicy{Table: "users", SoftDeleteColumn: "old_deleted"})
	source := NewDBWithExecutor(&genericSpy{}, driver.MySQLDialect{}, WithPolicySet(sourcePolicy))
	base := source.Table("users").Select("id").Where("id", 3)
	if _, err := base.Plan(nil); err != nil {
		t.Fatal(err)
	}
	destPolicy, _ := NewPolicySet(TablePolicy{Table: "users", SoftDeleteColumn: "new_deleted"})
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	spy := &genericSpy{Executor: sqlDB}
	dest := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithPolicySet(destPolicy))
	mock.ExpectQuery(`SELECT "id" FROM "users" WHERE "id" = \$1 AND "new_deleted" IS NULL`).WithArgs(3).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	if _, err = SelectOneBy[genericID](nil, dest, base); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spy.sql, "old_deleted") {
		t.Fatal(spy.sql)
	}
	if _, ok := source.settings.PolicySet().PolicyForTable("users"); !ok {
		t.Fatal("source settings lost")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGenericTransactionAndExternalExecutorInheritance(t *testing.T) {
	for _, name := range []string{"mysql", "postgres"} {
		t.Run(name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			parent := NewDB(sqlDB, genericDialect(name), WithSettings(genericSettings(t, name, true)))
			mock.ExpectBegin()
			mock.ExpectExec("INSERT").WithArgs(3, 1).WillReturnResult(sqlmock.NewResult(3, 1))
			mock.ExpectRollback()
			rollback := errors.New("rollback")
			err = parent.TransactionContext(context.Background(), func(tx Tx) error {
				spy := &genericSpy{Executor: tx.DB.exec}
				wrapped := tx.DB.WrapExecutor(spy)
				if _, e := Insert(nil, wrapped, map[string]any{"id": 3, "tenant_id": 2}, Table("users")); !errors.Is(e, ErrBlockedOperation) {
					t.Fatal(e)
				}
				if len(spy.calls) != 0 {
					t.Fatal(spy.calls)
				}
				if _, e := PlanInsert(nil, wrapped, map[string]any{"id": 3}, Table("users")); e != nil {
					t.Fatal(e)
				}
				if len(spy.calls) != 0 {
					t.Fatal(spy.calls)
				}
				if _, e := Insert(nil, wrapped, map[string]any{"id": 3}, Table("users")); e != nil {
					t.Fatal(e)
				}
				if len(spy.calls) != 1 {
					t.Fatal(spy.calls)
				}
				return rollback
			})
			if !errors.Is(err, rollback) {
				t.Fatal(err)
			}
			// Explicitly wrapping an external sql.Tx inherits only the selected parent's settings.
			mock.ExpectBegin()
			external, e := sqlDB.Begin()
			if e != nil {
				t.Fatal(e)
			}
			wrapped := parent.WrapTx(external)
			if _, e = Insert(nil, wrapped, map[string]any{"id": 3, "tenant_id": 2}, Table("users")); !errors.Is(e, ErrBlockedOperation) {
				t.Fatal(e)
			}
			mock.ExpectRollback()
			if e = external.Rollback(); e != nil {
				t.Fatal(e)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGenericConflictCoverageAndRiskParity(t *testing.T) {
	for _, name := range []string{"mysql", "postgres"} {
		t.Run(name, func(t *testing.T) {
			spy := &genericSpy{}
			settings := genericSettings(t, name, false)
			schemaInput := genericSchema(t, name).Input()
			schemaInput.Tables[0].Constraints = append(schemaInput.Tables[0].Constraints, query.WriteKeyConstraint{Kind: "unique", AllRows: true, Valid: true, NotDeferrable: true, Columns: schemaInput.Tables[0].Columns[1:2]})
			schema, err := NewApplicationSchema(schemaInput)
			if err != nil {
				t.Fatal(err)
			}
			settings = settings.WithTenantPolicy("generic-test", schema, false)
			db := NewDBWithExecutor(spy, genericDialect(name), WithSettings(settings))
			data := map[string]any{"tenant_id": 1, "id": 3, "score": 4}
			p, err := PlanUpsert(nil, db, data, Table("users"), ConflictColumns("tenant_id", "id"), UpdateColumns("score"))
			err = diagnosticError(p, err)
			if name == "mysql" {
				if !errors.Is(err, ErrBlockedOperation) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(spy.calls) != 0 {
				t.Fatal(spy.calls)
			}
			// Risk/policy decisions for supported equivalent UPDATEs match the DSL.
			db = db.WithOptions(WithSettings(genericSettings(t, name, false)))
			gp, ge := PlanUpdate(nil, db, data, Table("users"), PK("tenant_id", "id"), WherePK())
			qp, qe := db.Table("users").Where("tenant_id", 1).Where("id", 3).PlanUpdate(nil, map[string]any{"score": 4})
			if ge != nil || qe != nil {
				t.Fatal(ge, qe)
			}
			if gp.RiskLevel != qp.RiskLevel || gp.TenantPolicy.Status != qp.TenantPolicy.Status || !reflect.DeepEqual(gp.Warnings, qp.Warnings) {
				t.Fatalf("diagnostics differ: %+v %+v", gp, qp)
			}
		})
	}
}
