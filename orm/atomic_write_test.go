package orm

import (
	"context"
	"database/sql/driver"
	"errors"
	"math"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

func atomicSettings(t *testing.T, dialect, table string) Settings {
	t.Helper()
	typ := "bigint"
	if dialect == MySQL {
		typ = "BIGINT"
	}
	cols := []query.WriteKeyColumn{{Name: "tenant_id", DBType: typ, Bits: 64}, {Name: "id", DBType: typ, Bits: 64}, {Name: "version", DBType: typ, Bits: 64}, {Name: "score", DBType: typ, Bits: 64}}
	schema, e := NewApplicationSchema(ApplicationSchemaInput{Database: "atomic-fixture", Dialect: dialect, Tables: []ApplicationTable{{Table: table, PlainTable: true, Columns: cols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", Columns: cols[:2], AllRows: true, Valid: true, NotDeferrable: true}}}}})
	if e != nil {
		t.Fatal(e)
	}
	tenant, e := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: int64(7)})
	if e != nil {
		t.Fatal(e)
	}
	policies, e := NewPolicySet(TablePolicy{Table: table, TenantColumn: "tenant_id"})
	if e != nil {
		t.Fatal(e)
	}
	return NewSettings(policies, query.RiskConfig{}, tenant).WithTenantPolicy("atomic-fixture", schema, true)
}

func TestAtomicWriteResultLifecycle(t *testing.T) {
	for _, kind := range []string{"success", "mismatch", "driver", "result", "commit", "rollback", "external"} {
		t.Run(kind, func(t *testing.T) {
			raw, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			db := NewDB(raw, ormdriver.PostgresDialect{})
			source := errors.New("source failure")
			if kind == "external" {
				db = NewDBWithExecutor(raw, ormdriver.PostgresDialect{})
			} else {
				m.ExpectBegin()
				ex := m.ExpectExec("INSERT")
				switch kind {
				case "driver":
					ex.WillReturnError(source)
				case "result":
					ex.WillReturnResult(sqlmock.NewErrorResult(source))
				case "mismatch", "rollback":
					ex.WillReturnResult(driver.RowsAffected(2))
				default:
					ex.WillReturnResult(driver.RowsAffected(1))
				}
				switch kind {
				case "success":
					m.ExpectCommit()
				case "commit":
					m.ExpectCommit().WillReturnError(source)
				case "rollback":
					m.ExpectRollback().WillReturnError(source)
				default:
					m.ExpectRollback()
				}
			}
			_, e = Insert(t.Context(), db, map[string]any{"id": 1}, Table("items"), AtomicWrite(), ExpectAffected(1))
			switch kind {
			case "success":
				if e != nil {
					t.Fatal(e)
				}
			case "external":
				if !errors.Is(e, ErrWriteOwnership) {
					t.Fatal(e)
				}
			case "mismatch":
				if !errors.Is(e, ErrRowsAffected) {
					t.Fatal(e)
				}
			case "commit", "rollback":
				if !errors.Is(e, ErrWriteOutcomeUnknown) || !errors.Is(e, source) {
					t.Fatal(e)
				}
			default:
				if !errors.Is(e, source) {
					t.Fatal(e)
				}
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestAtomicReturningCompleteInspection(t *testing.T) {
	for _, kind := range []string{"one", "multiple", "zero", "late", "scan", "close"} {
		t.Run(kind, func(t *testing.T) {
			raw, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			db := NewDB(raw, ormdriver.PostgresDialect{})
			source := errors.New("late source")
			rows := sqlmock.NewRows([]string{"id"})
			switch kind {
			case "one":
				rows.AddRow(1)
			case "multiple":
				rows.AddRow(1).AddRow(2)
			case "late":
				rows.AddRow(1).AddRow(2).RowError(1, source)
			case "scan":
				rows.AddRow("not integer")
			case "close":
				rows.CloseError(source)
			}
			m.ExpectBegin()
			m.ExpectQuery("INSERT").WillReturnRows(rows).RowsWillBeClosed()
			if kind == "one" {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			_, e = InsertReturning[genericID](t.Context(), db, map[string]any{"id": 1}, Table("items"), AtomicWrite(), ExpectAffected(1))
			if kind == "one" && e != nil {
				t.Fatal(e)
			}
			if kind != "one" && e == nil {
				t.Fatal("missing result failure")
			}
			if kind == "late" || kind == "close" {
				if !errors.Is(e, source) {
					t.Fatal(e)
				}
			}
			if kind == "multiple" || kind == "zero" {
				if !errors.Is(e, ErrRowsAffected) {
					t.Fatal(e)
				}
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestOptimisticPrivateUpdate(t *testing.T) {
	for _, n := range []int64{0, 1, 2} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			raw, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer raw.Close()
			db := NewDB(raw, ormdriver.PostgresDialect{}, WithSettings(atomicSettings(t, Postgres, "items")))
			m.ExpectBegin()
			m.ExpectExec(`UPDATE "items" SET "score" = \$1, "version" = \$2 WHERE`).WithArgs(int64(8), int64(4), int64(1), int64(7), int64(3)).WillReturnResult(driver.RowsAffected(n))
			if n == 1 {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			_, e = db.Table("items").Where("id", int64(1)).OptimisticVersion("version", 3).Update(map[string]any{"score": int64(8)})
			if n == 1 && e != nil {
				t.Fatal(e)
			}
			if n == 0 && !errors.Is(e, ErrConflict) {
				t.Fatal(e)
			}
			if n == 2 && !errors.Is(e, ErrRowsAffected) {
				t.Fatal(e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestAtomicRefusalsBeforeSQL(t *testing.T) {
	raw, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	db := NewDB(raw, ormdriver.PostgresDialect{}, WithSettings(atomicSettings(t, Postgres, "items")))
	for _, opts := range [][]WriteOpt{{AtomicWrite(), ExpectAffected(-1)}, {ExpectAffected(1), ExpectAffected(1)}, {OptimisticVersion("version", math.MaxInt64)}, {OptimisticVersion("version", 1), ExpectAffected(0)}, {OptimisticVersion("version", 1), NoRowsAs(ErrConflict)}, {OptimisticVersion("version", 1), OptimisticVersion("version", 1)}} {
		_, e := Update(t.Context(), db, map[string]any{"id": int64(1), "score": int64(3)}, append([]WriteOpt{Table("items"), PK("id"), WherePK()}, opts...)...)
		if e == nil {
			t.Fatal("invalid options accepted")
		}
	}
	for _, q := range []*query.Query{db.Table("items"), db.Table("items").Where("id", 1).OrWhere("id", 2), db.Table("items").Where("id", ">", 1)} {
		if _, e = q.OptimisticVersion("version", 1).Update(map[string]any{"score": int64(2)}); e == nil {
			t.Fatal("incomplete key accepted")
		}
	}
	for _, col := range []string{"id", "tenant_id", "version"} {
		if _, e = db.Table("items").Where("id", int64(1)).OptimisticVersion("version", 1).Update(map[string]any{col: int64(2)}); e == nil {
			t.Fatal("protected assignment accepted")
		}
	}
	m.ExpectBegin()
	tx, e := raw.BeginTx(context.Background(), nil)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.WrapTx(tx).Table("items").Where("id", int64(1)).AtomicWrite().Update(map[string]any{"score": int64(2)})
	if !errors.Is(e, ErrWriteOwnership) {
		t.Fatal(e)
	}
	m.ExpectRollback()
	_ = tx.Rollback()
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestAtomicMySQLStorageRefusal(t *testing.T) {
	raw, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	db := NewDB(raw, ormdriver.MySQLDialect{})
	m.ExpectBegin()
	m.ExpectQuery("SELECT ENGINE FROM information_schema.tables").WithArgs("", "items").WillReturnRows(sqlmock.NewRows([]string{"ENGINE"}).AddRow("MyISAM"))
	m.ExpectRollback()
	_, e = Insert(t.Context(), db, map[string]any{"id": 1}, Table("items"), AtomicWrite())
	if !errors.Is(e, ErrWriteOptions) {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestAtomicOpaqueValueRefused(t *testing.T) {
	raw, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	db := NewDB(raw, ormdriver.PostgresDialect{})
	_, e = Insert(t.Context(), db, map[string]any{"id": commandTrap{}}, Table("items"), AtomicWrite())
	if !errors.Is(e, ErrWriteOptions) {
		t.Fatal("opaque value accepted")
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestAtomicMySQLTriggerRefusal(t *testing.T) {
	raw, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	db := NewDB(raw, ormdriver.MySQLDialect{})
	m.ExpectBegin()
	m.ExpectQuery("SELECT ENGINE").WithArgs("", "items").WillReturnRows(sqlmock.NewRows([]string{"ENGINE"}).AddRow("InnoDB"))
	m.ExpectQuery(`SELECT COUNT\(\*\) FROM information_schema.triggers`).WithArgs("", "items").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectRollback()
	_, e = Insert(t.Context(), db, map[string]any{"id": 1}, Table("items"), AtomicWrite())
	if !errors.Is(e, ErrWriteOptions) {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestAtomicUnverifiedExpressionsRefused(t *testing.T) {
	raw, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	db := NewDB(raw, ormdriver.PostgresDialect{})
	_, e = Update(t.Context(), db, map[string]any{"id": 1, "score": 2}, Table("items"), PK("id"), WherePK(), SetExpr("score", "arbitrary_function()"), AtomicWrite())
	if e == nil {
		t.Fatal("unverified expression accepted")
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
