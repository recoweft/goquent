package tests

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestGeneratedPatchDatabaseStates(t *testing.T) {
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
			var d driver.Dialect = driver.MySQLDialect{}
			if dialect == "postgres" {
				temporal = "TIMESTAMP(6) WITHOUT TIME ZONE"
				d = driver.PostgresDialect{}
			}
			if _, e := std.Exec("CREATE TABLE gq08_records (id BIGINT NOT NULL,segment INTEGER NOT NULL,status VARCHAR(12),active BOOLEAN,flag BOOLEAN NULL,note TEXT NULL,balance DECIMAL(30,4),happened DATE,clock TIME(6),stamp " + temporal + ",immutable TEXT,computed INTEGER,hidden TEXT,PRIMARY KEY(id,segment))"); e != nil {
				t.Fatal("table setup failed")
			}
			defer std.Exec("DROP TABLE gq08_records")
			for _, mode := range []string{"custom-executor", "external-tx", "strict"} {
				t.Run(mode, func(t *testing.T) {
					if _, e := std.Exec("DELETE FROM gq08_records"); e != nil {
						t.Fatal("fixture reset failed")
					}
					if _, e := std.Exec("INSERT INTO gq08_records VALUES (9007199254740993,0,'ready',true,true,'keep',5,'2024-02-29','12:34:56.123456','2024-02-29 12:34:56.123456','fixed',7,'hidden')"); e != nil {
						t.Fatal("fixture insert failed")
					}
					var executor orm.Executor = std
					var tx *sql.Tx
					if mode == "external-tx" {
						var e error
						tx, e = std.BeginTx(t.Context(), nil)
						if e != nil {
							t.Fatal("begin failed")
						}
						defer tx.Rollback()
						executor = tx
					}
					counted := &genericDatabaseExecutor{Executor: executor}
					settings := query.Settings{}
					if mode == "strict" {
						cols := []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}, {Name: "segment", DBType: "integer", Bits: 32}}
						if dialect == "mysql" {
							cols[0].DBType = "BIGINT"
							cols[1].DBType = "INT"
						}
						schemaCols := append([]query.WriteKeyColumn(nil), cols...)
						for _, c := range typedfixture.Manifest(dialect).Tables[0].Columns {
							if c.Name != "id" && c.Name != "segment" {
								schemaCols = append(schemaCols, query.WriteKeyColumn{Name: c.Name, DBType: c.Type, Nullable: c.Nullable})
							}
						}
						schema, e := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: dialect, Tables: []query.ApplicationTable{{Table: "gq08_records", PlainTable: true, Columns: schemaCols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols}}}}})
						if e != nil {
							t.Fatal("schema refused")
						}
						settings = settings.WithTenantPolicy("fixture", schema, false)
					}
					db := orm.NewDBWithExecutor(counted, d, orm.WithSettings(settings))
					var plan func() error
					var apply func(int) error
					if dialect == "mysql" {
						r := typedfixture.NewMySQLRecordRepository(db)
						c := typedfixture.MySQLRecordColumns()
						key := typedfixture.MySQLRecordKey{ID: 9007199254740993, Segment: 0}
						input := typedfixture.MySQLRecordUpdate{}
						first := typedfixture.MySQLRecordPatch{Active: c.Active.Set(false), Balance: c.Balance.Set(json.Number("0")), Flag: c.Flag.SetNull(), Note: c.Note.Set("")}
						if mode == "strict" {
							first.Balance = typedfixture.MySQLRecordBalancePatch{}
						}
						plan = func() error { _, _, e := r.PlanUpdateByKey(t.Context(), key, first, input); return e }
						apply = func(step int) error {
							switch step {
							case 0:
								_, e := r.UpdateByKey(t.Context(), key, first, input)
								return e
							case 1:
								_, e := r.UpdateByKey(t.Context(), key, typedfixture.MySQLRecordPatch{Note: c.Note.SetNull(), Flag: c.Flag.Set(false), Status: c.Status.Set(typedfixture.MySQLRecordStatusValueChoice2), Happened: c.Happened.Set("2024-03-01"), Clock: c.Clock.Set("00:00:00"), Stamp: c.Stamp.Set("2024-03-01T00:00:00")}, input)
								return e
							case 2:
								_, e := r.UpdateByKey(t.Context(), key, typedfixture.MySQLRecordPatch{}, input)
								return e
							case 3:
								_, e := r.UpdateByKey(t.Context(), key, typedfixture.MySQLRecordPatch{Balance: c.Balance.Set(json.Number("1.20"))}, input)
								return e
							default:
								_, e := r.UpdateIdentityByKey(t.Context(), key, first, input)
								return e
							}
						}
					} else {
						r := typedfixture.NewPostgresRecordRepository(db)
						c := typedfixture.PostgresRecordColumns()
						key := typedfixture.PostgresRecordKey{ID: 9007199254740993, Segment: 0}
						input := typedfixture.PostgresRecordUpdate{}
						first := typedfixture.PostgresRecordPatch{Active: c.Active.Set(false), Balance: c.Balance.Set(json.Number("0")), Flag: c.Flag.SetNull(), Note: c.Note.Set("")}
						if mode == "strict" {
							first.Balance = typedfixture.PostgresRecordBalancePatch{}
						}
						plan = func() error { _, _, e := r.PlanUpdateByKey(t.Context(), key, first, input); return e }
						apply = func(step int) error {
							switch step {
							case 0:
								_, e := r.UpdateByKey(t.Context(), key, first, input)
								return e
							case 1:
								_, e := r.UpdateByKey(t.Context(), key, typedfixture.PostgresRecordPatch{Note: c.Note.SetNull(), Flag: c.Flag.Set(false), Status: c.Status.Set(typedfixture.PostgresRecordStatusValueChoice2), Happened: c.Happened.Set("2024-03-01"), Clock: c.Clock.Set("00:00:00"), Stamp: c.Stamp.Set("2024-03-01T00:00:00")}, input)
								return e
							case 2:
								_, e := r.UpdateByKey(t.Context(), key, typedfixture.PostgresRecordPatch{}, input)
								return e
							case 3:
								_, e := r.UpdateByKey(t.Context(), key, typedfixture.PostgresRecordPatch{Balance: c.Balance.Set(json.Number("1.20"))}, input)
								return e
							default:
								row, e := r.UpdateIdentityByKey(t.Context(), key, first, input)
								if e == nil && (row.ID != 9007199254740993 || row.Segment != 0) {
									t.Fatal("returning key changed")
								}
								return e
							}
						}
					}
					if plan() != nil || counted.calls.Load() != 0 {
						t.Fatal("plan refused or executed")
					}
					if e := apply(0); e != nil {
						t.Fatal("value/null update failed")
					}
					verify := func(second bool) {
						var active bool
						var note sql.NullString
						var flag sql.NullBool
						var status, balance, fixed string
						var computed int
						var happened, stamp time.Time
						var clock typedfixture.PostgresRecordTimeText
						e := executor.QueryRow("SELECT active,note,flag,status,balance,immutable,computed,happened,clock,stamp FROM gq08_records WHERE id=9007199254740993 AND segment=0").Scan(&active, &note, &flag, &status, &balance, &fixed, &computed, &happened, &clock, &stamp)
						wantBalance := "0.0000"
						if mode == "strict" {
							wantBalance = "5.0000"
						}
						if e != nil || active || balance != wantBalance || fixed != "fixed" || computed != 7 {
							t.Fatal("stored update values changed")
						}
						if second {
							expectedClock := typedfixture.PostgresRecordTimeText("00:00:00")
							if dialect == "mysql" {
								expectedClock = "00:00:00.000000"
							}
							if happened.Day() != 1 || happened.Month() != time.March || clock != expectedClock || stamp.Day() != 1 || stamp.Nanosecond() != 0 {
								t.Fatal("stored temporal update changed")
							}
							if note.Valid || !flag.Valid || flag.Bool || status != "closed" {
								t.Fatal("explicit NULL/false result lost")
							}
						} else {
							if !note.Valid || note.String != "" || flag.Valid || status != "ready" {
								t.Fatal("empty/NULL/unchanged result lost")
							}
						}
					}
					verify(false)
					if e := apply(1); e != nil {
						t.Fatal("temporal/enum/null update failed")
					}
					verify(true)
					calls := counted.calls.Load()
					if e := apply(2); !errors.Is(e, operation.ErrEmptyPatch) {
						t.Fatal("empty patch not refused")
					}
					if apply(3) == nil || counted.calls.Load() != calls {
						t.Fatal("refused update executed")
					}
					e := apply(4)
					if dialect == "mysql" {
						if e == nil || counted.calls.Load() != calls {
							t.Fatal("MySQL returning executed")
						}
					} else if e != nil || counted.calls.Load() != calls+1 {
						t.Fatal("PG returning failed")
					}
					if tx != nil {
						if tx.Rollback() != nil {
							t.Fatal("rollback failed")
						}
						var note string
						if std.QueryRow("SELECT note FROM gq08_records WHERE id=9007199254740993 AND segment=0").Scan(&note) != nil || note != "keep" {
							t.Fatal("external transaction ownership changed")
						}
					}
				})
			}
		})
	}
}
