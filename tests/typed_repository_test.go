package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	mysql "github.com/go-sql-driver/mysql"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestGeneratedRepositoryDatabaseReads(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var root *orm.DB
			if dialect == "mysql" {
				root = setupDB(t)
			} else {
				root = setupPgDB(t)
			}
			defer root.Close()
			std := root.SQLDB()
			temporal := "DATETIME(6)"
			if dialect == "postgres" {
				temporal = "TIMESTAMP(6) WITHOUT TIME ZONE"
			}
			if _, err := std.Exec("CREATE TABLE gq08_records (id BIGINT NOT NULL,segment INTEGER NOT NULL,status VARCHAR(12),active BOOLEAN,flag BOOLEAN NULL,note TEXT NULL,balance DECIMAL(30,4),happened DATE,clock TIME(6),stamp " + temporal + ",immutable TEXT,computed INTEGER,hidden TEXT,PRIMARY KEY(id,segment))"); err != nil {
				t.Fatal(err)
			}
			defer std.Exec("DROP TABLE gq08_records")
			if _, err := std.Exec("INSERT INTO gq08_records VALUES (9007199254740993,0,'ready',true,false,NULL,999999999999999999.1234,'2024-02-29','12:34:56.123456','2024-02-29 12:34:56.123456','fixed',7,'hidden'),(9007199254740994,0,'closed',false,NULL,'',0,'2024-03-01','00:00:00','2024-03-01 00:00:00','fixed',8,'hidden')"); err != nil {
				t.Fatal(err)
			}
			t.Run("strict-read", func(t *testing.T) {
				schema, e := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "gq08", Dialect: dialect, Tables: []query.ApplicationTable{{Table: "gq08_records", PlainTable: true, CompleteUniqueConstraints: true, Columns: []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}, {Name: "segment", DBType: "integer", Bits: 32}}}}})
				if e != nil {
					t.Fatal(e)
				}
				strict := root.WithOptions(orm.WithSettings(query.Settings{}.WithTenantPolicy("gq08", schema, false)))
				if dialect == "mysql" {
					rows, e := typedfixture.NewMySQLRecordRepository(strict).SelectIdentity(t.Context(), typedfixture.MySQLRecordRead{})
					if e != nil || len(rows) != 2 {
						t.Fatal("strict MySQL read failed")
					}
				} else {
					rows, e := typedfixture.NewPostgresRecordRepository(strict).SelectIdentity(t.Context(), typedfixture.PostgresRecordRead{})
					if e != nil || len(rows) != 2 {
						t.Fatal("strict PostgreSQL read failed")
					}
				}
			})
			if dialect == "mysql" {
				t.Run("registered-driver", func(t *testing.T) {
					orm.RegisterDriverWithDialect("mysql-gq08", &mysql.MySQLDriver{}, driver.MySQLDialect{})
					dsn, _ := lookupTestDSN("TEST_MYSQL_DSN", defaultMySQLTestDSN)
					db, e := orm.OpenWithDriverOptions("mysql-gq08", dsn, orm.WithSettings(query.Settings{}))
					if e != nil {
						t.Fatal("registered driver open failed")
					}
					defer db.Close()
					rows, e := typedfixture.NewMySQLRecordRepository(db).SelectIdentity(t.Context(), typedfixture.MySQLRecordRead{})
					if e != nil || len(rows) != 2 {
						t.Fatal("registered driver read failed")
					}
				})
			}
			for _, external := range []bool{false, true} {
				t.Run(map[bool]string{false: "custom-executor", true: "external-tx"}[external], func(t *testing.T) {
					var exec orm.Executor = std
					if external {
						tx, err := std.BeginTx(t.Context(), nil)
						if err != nil {
							t.Fatal(err)
						}
						defer tx.Rollback()
						exec = tx
					}
					counted := &genericDatabaseExecutor{Executor: exec}
					var d driver.Dialect = driver.MySQLDialect{}
					if dialect == "postgres" {
						d = driver.PostgresDialect{}
					}
					db := orm.NewDBWithExecutor(counted, d, orm.WithSettings(query.Settings{}))
					if dialect == "mysql" {
						c := typedfixture.MySQLRecordColumns()
						r := typedfixture.NewMySQLRecordRepository(db)
						input := typedfixture.MySQLRecordRead{Filters: []typedfixture.MySQLRecordPredicate{c.ID.Eq(9007199254740993)}}
						if _, _, e := r.PlanSummary(t.Context(), input); e != nil || counted.calls.Load() != 0 {
							t.Fatal("planning executed")
						}
						rows, e := r.SelectSummary(t.Context(), input)
						if e != nil || len(rows) != 1 {
							t.Fatal("generated MySQL read failed")
						}
						v := rows[0]
						if v.ID != 9007199254740993 || !v.Active || v.Note.Valid || !v.Flag.Valid || v.Flag.Bool || v.Balance != "999999999999999999.1234" || v.Happened.Day() != 29 || v.Clock != "12:34:56.123456" || v.Stamp.Nanosecond() != 123456000 {
							t.Fatal("MySQL result precision/presence changed")
						}
						_, e = r.FindByKey(t.Context(), typedfixture.MySQLRecordKey{ID: 1, Segment: 0}, typedfixture.MySQLRecordRead{})
						if !errors.Is(e, orm.ErrNotFound) {
							t.Fatal("no-row identity changed")
						}
						v2, e := r.SelectSummary(t.Context(), typedfixture.MySQLRecordRead{Filters: []typedfixture.MySQLRecordPredicate{c.ID.Eq(9007199254740994)}})
						if e != nil || len(v2) != 1 || v2[0].Active || v2[0].Flag.Valid || !v2[0].Note.Valid || v2[0].Note.V != "" {
							t.Fatal("false/empty/NULL conflated")
						}
						n := int64(0)
						v2, e = r.SelectSummary(t.Context(), typedfixture.MySQLRecordRead{Limit: &n})
						if e != nil || len(v2) != 0 {
							t.Fatal("LIMIT 0 changed")
						}
					} else {
						c := typedfixture.PostgresRecordColumns()
						r := typedfixture.NewPostgresRecordRepository(db)
						input := typedfixture.PostgresRecordRead{Filters: []typedfixture.PostgresRecordPredicate{c.ID.Eq(9007199254740993)}}
						if _, _, e := r.PlanSummary(t.Context(), input); e != nil || counted.calls.Load() != 0 {
							t.Fatal("planning executed")
						}
						rows, e := r.SelectSummary(t.Context(), input)
						if e != nil || len(rows) != 1 {
							t.Fatal("generated PostgreSQL read failed")
						}
						v := rows[0]
						if v.ID != 9007199254740993 || !v.Active || v.Note.Valid || !v.Flag.Valid || v.Flag.Bool || v.Balance != "999999999999999999.1234" || v.Happened.Day() != 29 || v.Clock != "12:34:56.123456" || v.Stamp.Nanosecond() != 123456000 {
							t.Fatal("PostgreSQL result precision/presence changed")
						}
						_, e = r.FindByKey(t.Context(), typedfixture.PostgresRecordKey{ID: 1, Segment: 0}, typedfixture.PostgresRecordRead{})
						if !errors.Is(e, orm.ErrNotFound) {
							t.Fatal("no-row identity changed")
						}
						v2, e := r.SelectSummary(t.Context(), typedfixture.PostgresRecordRead{Filters: []typedfixture.PostgresRecordPredicate{c.ID.Eq(9007199254740994)}})
						if e != nil || len(v2) != 1 || v2[0].Active || v2[0].Flag.Valid || !v2[0].Note.Valid || v2[0].Note.V != "" {
							t.Fatal("false/empty/NULL conflated")
						}
						n := int64(0)
						v2, e = r.SelectSummary(t.Context(), typedfixture.PostgresRecordRead{Limit: &n})
						if e != nil || len(v2) != 0 {
							t.Fatal("LIMIT 0 changed")
						}
					}
					if counted.calls.Load() != 4 {
						t.Fatal("read dispatched more than once")
					}
					spec := operation.OperationSpec{Version: 1, Operation: "select", Model: "gq08_records", Select: []string{"id"}, Filters: []operation.FilterSpec{{Field: "balance", Op: "=", Value: json.Number("999999999999999999.1234")}}}
					plan, e := db.CompileOperation(t.Context(), spec, operation.Options{Manifest: typedfixture.Manifest(dialect)})
					if e != nil || !plan.Blocked || !reflect.DeepEqual(plan.Params, []any{json.Number("999999999999999999.1234")}) {
						t.Fatal("fractional private domain expanded")
					}
					if _, e = orm.SelectOperationBy[struct{ ID int64 }](t.Context(), db, spec, operation.Options{Manifest: typedfixture.Manifest(dialect)}); e == nil || counted.calls.Load() != 4 {
						t.Fatal("blocked decimal executed")
					}
					ctx, cancel := context.WithCancel(t.Context())
					cancel()
					spec.Filters = nil
					if _, e = orm.SelectOperationBy[struct{ ID int64 }](ctx, db, spec, operation.Options{Manifest: typedfixture.Manifest(dialect)}); e == nil || counted.calls.Load() != 4 {
						t.Fatal("cancelled read executed")
					}
				})
			}
		})
	}
}

// Keep the external transaction's SQL interface visible in this fixture.
var _ orm.Executor = (*sql.Tx)(nil)
