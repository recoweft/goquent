package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

// These tests are sequential because they share the repository's explicit DSNs.
func TestPerformancePlanNative(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			env := "TEST_MYSQL_DSN"
			if dialect == "postgres" {
				env = "TEST_POSTGRES_DSN"
			}
			dsn := os.Getenv(env)
			if dsn == "" {
				t.Skip("explicit database DSN unavailable")
			}
			db, e := orm.OpenWithDriver(dialect, dsn)
			if e != nil {
				t.Fatal("native connection failed")
			}
			defer db.Close()
			raw := db.SQLDB()
			table := "gq12_performance_fixture"
			if dialect == "postgres" {
				table = "public." + table
			}
			for _, statement := range []string{"DROP TABLE IF EXISTS " + table, "CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, tenant_id BIGINT NOT NULL)", "INSERT INTO " + table + " VALUES (1,7),(2,7)"} {
				if _, e := raw.Exec(statement); e != nil {
					t.Fatal("fixture setup failed")
				}
			}
			defer raw.Exec("DROP TABLE IF EXISTS " + table)
			q := db.Table(table).Select("id").Where("tenant_id", int64(7)).OrderBy("id", "asc").Limit(10)
			p, e := q.ExplainPerformance(t.Context(), query.PerformanceOptions{})
			if e != nil {
				t.Fatalf("native collection: %v", e)
			}
			if !p.Native() || len(p.Raw) == 0 || p.CapturedAt.IsZero() || p.ServerVersion == "" || len(p.Nodes) == 0 {
				t.Fatal("missing native evidence")
			}
			source, err := p.ToJSON()
			if err != nil {
				t.Fatal("native source serialization")
			}
			var decoded query.PerformancePlan
			if json.Unmarshal(source, &decoded) != nil || decoded.Native() {
				t.Fatal("native source roundtrip")
			}
			for _, unsafeQuery := range []*query.Query{
				db.Table(table).Select("id").WhereRaw("gq12_must_not_execute()", nil).Limit(1),
				db.Table(table).Select("id").Limit(1).LockForUpdate(),
			} {
				if _, e := unsafeQuery.ExplainPerformance(t.Context(), query.PerformanceOptions{}); e != query.ErrPerformanceUnsupported {
					t.Fatalf("native structural refusal: %v", e)
				}
			}
			closedRaw, err := sql.Open(dialect, dsn)
			if err != nil {
				t.Fatal("closed pool setup")
			}
			closedRaw.Close()
			if _, e := orm.NewDB(closedRaw, db.Dialect()).Table(table).Select("id").Limit(1).ExplainPerformance(t.Context(), query.PerformanceOptions{}); e != query.ErrPerformanceConnection {
				t.Fatal("connection failure boundary")
			}
			before := p.Target.SQL
			q.Limit(2).Where("id", 9)
			if p.Target.SQL != before {
				t.Fatal("old snapshot changed")
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if _, e := q.ExplainPerformance(ctx, query.PerformanceOptions{}); e != query.ErrPerformanceCanceled {
				t.Fatal("cancellation")
			}
			// A view must be refused by the catalog before target EXPLAIN.
			view := "gq12_performance_view"
			if dialect == "postgres" {
				view = "public." + view
			}
			raw.Exec("DROP VIEW IF EXISTS " + view)
			if _, e := raw.Exec("CREATE VIEW " + view + " AS SELECT id, tenant_id FROM " + table); e != nil {
				t.Fatal("view setup")
			}
			defer raw.Exec("DROP VIEW IF EXISTS " + view)
			if _, e := db.Table(view).Select("id").Limit(2).ExplainPerformance(t.Context(), query.PerformanceOptions{}); e != query.ErrPerformanceUnsupported {
				t.Fatalf("view refusal: %v", e)
			}
			tx, e := raw.BeginTx(t.Context(), &sql.TxOptions{ReadOnly: true})
			if e != nil {
				t.Fatal("external Tx")
			}
			if _, e := orm.NewTxDB(tx, db.Dialect()).Table(table).Select("id").Limit(2).ExplainPerformance(t.Context(), query.PerformanceOptions{}); e != query.ErrPerformanceUnsupported {
				t.Fatal("external Tx refusal")
			}
			tx.Rollback()
			// Unsupported schema features are rejected without executing a dangerous plan.
			alterations := []struct{ add, remove string }{}
			if dialect == "postgres" {
				alterations = append(alterations,
					struct{ add, remove string }{"ALTER TABLE " + table + " ENABLE ROW LEVEL SECURITY", "ALTER TABLE " + table + " DISABLE ROW LEVEL SECURITY"},
					struct{ add, remove string }{"CREATE INDEX gq12_partial ON " + table + " (tenant_id) WHERE tenant_id>0", "DROP INDEX public.gq12_partial"},
					struct{ add, remove string }{"ALTER TABLE " + table + " ADD COLUMN generated_id BIGINT GENERATED ALWAYS AS (id+1) STORED", "ALTER TABLE " + table + " DROP COLUMN generated_id"})
			} else {
				alterations = append(alterations, struct{ add, remove string }{"ALTER TABLE " + table + " ADD COLUMN generated_id BIGINT GENERATED ALWAYS AS (id+1) STORED", "ALTER TABLE " + table + " DROP COLUMN generated_id"})
			}
			for _, a := range alterations {
				if _, e := raw.Exec(a.add); e != nil {
					t.Fatal("unsupported fixture setup")
				}
				_, failure := db.Table(table).Select("id").Limit(2).ExplainPerformance(t.Context(), query.PerformanceOptions{})
				if _, e := raw.Exec(a.remove); e != nil {
					t.Fatal("unsupported fixture cleanup")
				}
				if failure != query.ErrPerformanceUnsupported {
					t.Fatalf("schema refusal: %v", failure)
				}
			}
			if dialect == "mysql" {
				raw.SetMaxOpenConns(1)
				if _, e := raw.Exec("CREATE TEMPORARY TABLE " + table + " (id BIGINT)"); e != nil {
					t.Fatal("temporary fixture")
				}
				_, failure := db.Table(table).Select("id").Limit(2).ExplainPerformance(t.Context(), query.PerformanceOptions{})
				if _, e := raw.Exec("DROP TEMPORARY TABLE " + table); e != nil {
					t.Fatal("temporary cleanup")
				}
				if failure != query.ErrPerformanceUnsupported {
					t.Fatalf("temporary shadow refusal: %v", failure)
				}
			}
			// Deadline covers connection acquisition, not just each individual query.
			raw.SetMaxOpenConns(1)
			conn, e := raw.Conn(t.Context())
			if e != nil {
				t.Fatal("pin connection")
			}
			started := time.Now()
			_, e = db.Table(table).Select("id").Limit(2).ExplainPerformance(t.Context(), query.PerformanceOptions{Timeout: 20 * time.Millisecond})
			ownerCtx, ownerCancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			_, ownerError := db.Table(table).Select("id").Limit(2).WithContext(ownerCtx).ExplainPerformance(t.Context(), query.PerformanceOptions{})
			ownerCancel()
			conn.Close()
			if ownerError != query.ErrPerformanceTimeout {
				t.Fatalf("owner deadline: %v", ownerError)
			}
			if e != query.ErrPerformanceTimeout || time.Since(started) > time.Second {
				t.Fatal("connection wait deadline")
			}
		})
	}
}
