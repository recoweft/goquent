package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	"os"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/migration"
)

func TestPreflightVisibilityLive(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		dialect   driver.Dialect
	}{{"mysql", "TEST_MYSQL_DSN", driver.MySQLDialect{}}, {"postgres", "TEST_POSTGRES_DSN", driver.PostgresDialect{}}} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.key)
			if dsn == "" {
				t.Skip("explicit integration DSN required")
			}
			db, e := sql.Open(tc.name, dsn)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			exec := func(q string) {
				t.Helper()
				if _, e := db.Exec(q); e != nil {
					t.Fatal(e)
				}
			}
			const ns = "gq_ai11_pr3_visibility"
			const user = "gq_ai11_pr3_reader"
			const pass = "fixture_only_pr3"
			suffix := ""
			if tc.name == "postgres" {
				suffix = " CASCADE"
			}
			exec("DROP SCHEMA IF EXISTS " + ns + suffix)
			exec("CREATE SCHEMA " + ns)
			defer db.Exec("DROP SCHEMA " + ns + suffix)
			exec("CREATE TABLE " + ns + ".child(a INTEGER,b INTEGER)")
			exec("CREATE TABLE " + ns + ".parent(x INTEGER,y INTEGER,PRIMARY KEY(x,y))")
			exec("INSERT INTO " + ns + ".child VALUES (NULL,-1),(7,-1),(7,-1)")
			exec("INSERT INTO " + ns + ".parent VALUES (7,-1)")
			var limitedDSN string
			if tc.name == "postgres" {
				exec("DROP ROLE IF EXISTS " + user)
				exec("CREATE ROLE " + user + " LOGIN PASSWORD '" + pass + "'")
				defer func() { db.Exec("DROP OWNED BY " + user); db.Exec("DROP ROLE " + user) }()
				exec("GRANT USAGE ON SCHEMA " + ns + " TO " + user)
				exec("GRANT SELECT ON ALL TABLES IN SCHEMA " + ns + " TO " + user)
				exec("ALTER TABLE " + ns + ".child ENABLE ROW LEVEL SECURITY")
				exec("CREATE POLICY hidden_child ON " + ns + ".child USING (b > 0)")
				exec("ALTER TABLE " + ns + ".parent ENABLE ROW LEVEL SECURITY")
				exec("CREATE POLICY hidden_parent ON " + ns + ".parent USING (y > 0)")
				u, e := url.Parse(dsn)
				if e != nil {
					t.Fatal(e)
				}
				u.User = url.UserPassword(user, pass)
				limitedDSN = u.String()
			} else {
				exec("DROP USER IF EXISTS '" + user + "'@'%'")
				exec("CREATE USER '" + user + "'@'%' IDENTIFIED BY '" + pass + "'")
				defer db.Exec("DROP USER '" + user + "'@'%'")
				exec("GRANT SELECT ON " + ns + ".child TO '" + user + "'@'%'")
				cfg, e := mysql.ParseDSN(dsn)
				if e != nil {
					t.Fatal(e)
				}
				cfg.User = user
				cfg.Passwd = pass
				cfg.DBName = ns
				limitedDSN = cfg.FormatDSN()
			}
			limited, e := sql.Open(tc.name, limitedDSN)
			if e != nil {
				t.Fatal(e)
			}
			defer limited.Close()
			if e = limited.Ping(); e != nil {
				t.Fatal(e)
			}
			clock := func() time.Time { return time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC) }
			current, e := migration.ReadSchema(context.Background(), db, tc.dialect, migration.WithSchemaReadSchema(ns), migration.WithSchemaReadTables("child", "parent"))
			if e != nil {
				t.Fatal(e)
			}
			b, _ := json.Marshal(current)
			var desired migration.Schema
			if json.Unmarshal(b, &desired) != nil {
				t.Fatal("clone")
			}
			desired.Tables[0].Columns[0].Nullable = false
			n := false
			step := migration.MigrationStep{Type: migration.AlterNullability, Table: "child", Column: "a", Nullable: &n}
			plan, e := migration.PlanPreflight(&migration.MigrationPlan{CurrentSchema: &current, DesiredSchema: &desired, Steps: []migration.MigrationStep{step}}, []migration.PhaseContract{{Phase: "contract", OldReaders: "compatible", OldWriters: "compatible"}}, nil)
			if e != nil {
				t.Fatal(e)
			}
			c, e := migration.NewPreflightCollector(context.Background(), limited, tc.dialect, migration.CollectorOptions{Clock: clock})
			if e != nil {
				t.Fatal(e)
			}
			defer c.Close()
			out, e := c.Collect(context.Background(), plan)
			if out == nil {
				t.Fatal("missing partial source")
			}
			v, _ := c.Evaluate(out, migration.PreflightOptions{Clock: clock, MaxAge: migration.PreflightMaxAge})
			if v.Status == "satisfied" {
				t.Fatal("visibility promoted")
			}
			if tc.name == "mysql" {
				if e == nil {
					t.Fatal("partial inventory mismatch should fail")
				}
				return
			}
			if e != nil || v.Status != "unknown" {
				t.Fatalf("RLS %v %v", v, e)
			}
			var visible, actual int
			if e = limited.QueryRow("SELECT COUNT(*) FROM " + ns + ".child WHERE a IS NULL").Scan(&visible); e != nil {
				t.Fatal(e)
			}
			if e = db.QueryRow("SELECT COUNT(*) FROM " + ns + ".child WHERE a IS NULL").Scan(&actual); e != nil {
				t.Fatal(e)
			}
			if visible != 0 || actual != 1 {
				t.Fatal("RLS did not hide NULL")
			}
			if e = limited.QueryRow("SELECT COUNT(*) FROM (SELECT a,b FROM " + ns + ".child GROUP BY a,b HAVING COUNT(*)>1) g").Scan(&visible); e != nil {
				t.Fatal(e)
			}
			if visible != 0 {
				t.Fatal("RLS did not hide duplicates")
			}
			// Reference-side RLS is checked even when the local table is fully visible.
			exec("ALTER TABLE " + ns + ".child DISABLE ROW LEVEL SECURITY")
			desired.Tables[0].ForeignKeys = []migration.ForeignKeySchema{{Name: "pair_fk", Columns: []string{"a", "b"}, Reference: current.Tables[1].Identity, ReferencedColumns: []string{"x", "y"}, OnUpdate: "no_action", OnDelete: "no_action", Match: "simple", Validation: "validated", Enforcement: "enforced", Deferrability: "not_deferrable", Definition: migration.CoverageComplete}}
			step = migration.MigrationStep{Type: migration.AddForeignKey, Table: "child", Constraint: "pair_fk"}
			plan, e = migration.PlanPreflight(&migration.MigrationPlan{CurrentSchema: &current, DesiredSchema: &desired, Steps: []migration.MigrationStep{step}}, []migration.PhaseContract{{Phase: "contract", OldReaders: "compatible", OldWriters: "compatible"}}, nil)
			if e != nil {
				t.Fatal(e)
			}
			out, e = c.Collect(context.Background(), plan)
			if e != nil {
				t.Fatal(e)
			}
			v, e = c.Evaluate(out, migration.PreflightOptions{Clock: clock, MaxAge: migration.PreflightMaxAge})
			if e != nil || v.Status != "unknown" {
				t.Fatalf("reference visibility %v %v", v, e)
			}
			exec("REVOKE SELECT ON " + ns + ".child FROM " + user)
			out, e = c.Collect(context.Background(), plan)
			if e == nil || out.Collection.Completed {
				t.Fatal("permission loss became complete")
			}
		})
	}
}
