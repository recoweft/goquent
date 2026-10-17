package tests

import (
	"context"
	"database/sql"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/migration"
	"net/url"
	"os"
	"testing"
)

func TestSchemaVisibilityLive(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		dialect   driver.Dialect
	}{{"mysql", "TEST_MYSQL_DSN", driver.MySQLDialect{}}, {"postgres", "TEST_POSTGRES_DSN", driver.PostgresDialect{}}} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.key)
			if dsn == "" {
				t.Skip("explicit integration DSN required")
			}
			db, err := sql.Open(tc.name, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			exec := func(q string) {
				t.Helper()
				if _, e := db.Exec(q); e != nil {
					t.Fatal(e)
				}
			}
			const ns = "gq_ai11_pr2_visibility"
			const user = "gq_ai11_pr2_reader"
			const pass = "fixture_only_pr2"
			suffix := ""
			if tc.name == "postgres" {
				suffix = " CASCADE"
			}
			exec("DROP SCHEMA IF EXISTS " + ns + suffix)
			exec("CREATE SCHEMA " + ns)
			defer db.Exec("DROP SCHEMA " + ns + suffix)
			exec("CREATE TABLE " + ns + ".visible (a INTEGER,b INTEGER)")
			exec("CREATE TABLE " + ns + ".hidden (a INTEGER)")
			var restrictedDSN string
			if tc.name == "mysql" {
				exec("DROP USER IF EXISTS '" + user + "'@'%'")
				exec("CREATE USER '" + user + "'@'%' IDENTIFIED BY '" + pass + "'")
				defer db.Exec("DROP USER '" + user + "'@'%'")
				exec("GRANT SELECT(a) ON " + ns + ".visible TO '" + user + "'@'%'")
				cfg, e := mysql.ParseDSN(dsn)
				if e != nil {
					t.Fatal(e)
				}
				cfg.User = user
				cfg.Passwd = pass
				cfg.DBName = ns
				restrictedDSN = cfg.FormatDSN()
			} else {
				exec("DROP ROLE IF EXISTS " + user)
				exec("CREATE ROLE " + user + " LOGIN PASSWORD '" + pass + "'")
				defer func() { db.Exec("DROP OWNED BY " + user); db.Exec("DROP ROLE " + user) }()
				exec("GRANT USAGE ON SCHEMA " + ns + " TO " + user)
				exec("GRANT SELECT(a) ON " + ns + ".visible TO " + user)
				u, e := url.Parse(dsn)
				if e != nil {
					t.Fatal(e)
				}
				u.User = url.UserPassword(user, pass)
				restrictedDSN = u.String()
			}
			limited, err := sql.Open(tc.name, restrictedDSN)
			if err != nil {
				t.Fatal(err)
			}
			defer limited.Close()
			if err = limited.Ping(); err != nil {
				t.Fatal(err)
			}
			reader := &schemaReadOnly{db: limited, t: t}
			observed, err := migration.ReadSchema(context.Background(), reader, tc.dialect, migration.WithSchemaReadSchema(ns), migration.WithSchemaReadTables("visible", "hidden"))
			if err != nil {
				t.Fatal(err)
			}
			if observed.Context.Coverage.Tables == migration.CoverageComplete || !migration.DiffSchemas(observed, observed).ComparisonIncomplete {
				t.Fatal("limited visibility became complete")
			}
			if len(observed.Tables) != 1 || observed.Tables[0].Name != "visible" || len(observed.Tables[0].Columns) != 1 {
				t.Fatal("fixture did not produce partial visibility")
			}
			missing, err := migration.ReadSchema(context.Background(), reader, tc.dialect, migration.WithSchemaReadSchema(ns), migration.WithSchemaReadTables("hidden"))
			if err != nil {
				t.Fatal(err)
			}
			if len(missing.Tables) != 0 || !migration.DiffSchemas(missing, missing).ComparisonIncomplete {
				t.Fatal("invisible table treated as known empty")
			}
			t.Log("real restricted role: one column visible; requested hidden table not promoted to absence")
		})
	}
}
