package orm

import (
	"context"
	"github.com/recoweft/goquent/orm/observation"
	"os"
	"testing"
)

// Native fixtures run sequentially and keep credentials and raw errors out of logs.
func TestObservationIntegration(t *testing.T) {
	for _, dialect := range []string{MySQL, Postgres} {
		t.Run(dialect, func(t *testing.T) {
			key := "TEST_MYSQL_DSN"
			if dialect == Postgres {
				key = "TEST_POSTGRES_DSN"
			}
			dsn := os.Getenv(key)
			if dsn == "" {
				t.Skip("explicit database source absent")
			}
			db, e := OpenWithDriver(dialect, dsn)
			if e != nil {
				t.Fatal("native connection unavailable")
			}
			defer db.Close()
			raw := db.SQLDB()
			var version string
			if raw.QueryRow("SELECT VERSION()").Scan(&version) != nil {
				t.Fatal("version unavailable")
			}
			t.Log("native version", version)
			table := "gq12_observation_fixture"
			_, _ = raw.Exec("DROP TABLE IF EXISTS " + table)
			if _, e = raw.Exec("CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, value BIGINT NOT NULL)"); e != nil {
				t.Fatal("fixture creation")
			}
			defer raw.Exec("DROP TABLE " + table)
			ctx, s, e := observation.Start(context.Background(), observation.Options{DetectRepeated: true})
			if e != nil {
				t.Fatal(e)
			}
			if _, e = db.Table(table).WithContext(ctx).Insert(map[string]any{"id": int64(1), "value": int64(2)}); e != nil {
				t.Fatal("insert")
			}
			for i := 0; i < 5; i++ {
				var result map[string]any
				if e = db.Table(table).WithContext(ctx).Select("id").Where("id", int64(1)).Limit(1).FirstMap(&result); e != nil {
					t.Fatal("structured read")
				}
			}
			approved := db.RequireRawApproval("native fixture")
			rows, e := approved.QueryObservedContext(ctx, "SELECT id FROM "+table)
			if e != nil {
				t.Fatal("observed read")
			}
			for rows.Next() {
				var id int64
				if rows.Scan(&id) != nil {
					t.Fatal("scan")
				}
			}
			if rows.NextResultSet() {
				t.Fatal("unexpected set")
			}
			if rows.Close() != nil {
				t.Fatal("close")
			}
			tx, e := approved.BeginTx(ctx, nil)
			if e != nil {
				t.Fatal("begin")
			}
			var id int64
			if tx.QueryRowContext(ctx, "SELECT id FROM "+table).Scan(&id) != nil {
				t.Fatal("tx scan")
			}
			if tx.Rollback() != nil {
				t.Fatal("rollback")
			}
			if dialect == Postgres {
				type observationNativeRow struct {
					ID    int64 `db:"id"`
					Value int64 `db:"value"`
				}
				_, e = InsertReturning[observationNativeRow, observationNativeRow](ctx, db, observationNativeRow{2, 3}, Table(table))
				if e != nil {
					t.Fatal("returning")
				}
			}
			v := s.Finish()
			expected := int64(8)
			if dialect == Postgres {
				expected++
			}
			if v.Dispatched != expected || v.Incomplete != 0 || v.SuspectedGroups != 1 || v.TxBoundaries != 2 {
				t.Fatal(v)
			}
			limit := int64(0)
			blocked, bs, _ := observation.Start(context.Background(), observation.Options{QueryLimit: &limit})
			if _, e = approved.ExecContext(blocked, "DELETE FROM "+table); e != observation.ErrQueryBudgetExceeded {
				t.Fatal("budget")
			}
			var count int
			if raw.QueryRow("SELECT COUNT(*) FROM "+table).Scan(&count) != nil || count == 0 {
				t.Fatal("rejected write changed rows")
			}
			if bs.Finish().Dispatched != 0 {
				t.Fatal("blocked dispatch")
			}
			// Caller-owned external transactions stay open after a budget refusal.
			external, e := raw.BeginTx(context.Background(), nil)
			if e != nil {
				t.Fatal("external begin")
			}
			defer external.Rollback()
			externalDB := NewTxDB(external, db.Dialect()).RequireRawApproval("native fixture")
			if externalDB.QueryRowContext(blocked, "SELECT id FROM "+table).Scan(&id) != observation.ErrQueryBudgetExceeded {
				t.Fatal("external budget")
			}
			if external.QueryRow("SELECT id FROM "+table).Scan(&id) != nil {
				t.Fatal("external transaction was changed")
			}
			if external.Rollback() != nil {
				t.Fatal("external rollback")
			}
		})
	}
}
