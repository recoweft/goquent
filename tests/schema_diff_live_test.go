package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/migration"
)

// The fixture owns its DDL. The library receives only this read surface.
type schemaReadOnly struct {
	db    *sql.DB
	t     *testing.T
	reads int
}

func (r *schemaReadOnly) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	r.t.Helper()
	if !strings.HasPrefix(strings.TrimSpace(q), "SELECT ") {
		r.t.Fatal("non-read export")
	}
	r.reads++
	return r.db.QueryContext(ctx, q, args...)
}
func (r *schemaReadOnly) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	r.t.Fatal("unexpected query-row")
	return nil
}

func TestSchemaDefinitionLiveExport(t *testing.T) {
	for _, tc := range []struct {
		name, key, drv string
		dialect        driver.Dialect
	}{
		{"mysql", "TEST_MYSQL_DSN", "mysql", driver.MySQLDialect{}}, {"postgres", "TEST_POSTGRES_DSN", "postgres", driver.PostgresDialect{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.key)
			if dsn == "" {
				t.Skip("explicit integration DSN required")
			}
			db, e := sql.Open(tc.drv, dsn)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			if e = db.Ping(); e != nil {
				t.Fatal(e)
			}
			var version string
			if e = db.QueryRow("SELECT version()").Scan(&version); e != nil {
				t.Fatal(e)
			}
			t.Logf("actual database version: %s", version)
			const table = "gq_ai_11_diff_fixture"
			// Fixed dedicated fixture table; no application schema is altered.
			if _, e = db.Exec("DROP TABLE IF EXISTS " + table); e != nil {
				t.Fatal(e)
			}
			if _, e = db.Exec("CREATE TABLE " + table + " (a INTEGER DEFAULT 1, b INTEGER DEFAULT 2, literal_value VARCHAR(64) DEFAULT 'secret-fixture', empty_value VARCHAR(64) DEFAULT '', null_value INTEGER DEFAULT NULL, absent_value INTEGER)"); e != nil {
				t.Fatal(e)
			}
			defer db.Exec("DROP TABLE " + table)
			if _, e = db.Exec("CREATE INDEX gq_ai_11_pair_idx ON " + table + " (a,b)"); e != nil {
				t.Fatal(e)
			}
			reader := &schemaReadOnly{db: db, t: t}
			current, e := migration.ReadSchema(context.Background(), reader, tc.dialect, migration.WithSchemaReadTables(table))
			if e != nil {
				t.Fatal(e)
			}
			if len(current.Tables) != 1 || len(current.Tables[0].Indexes) != 1 {
				t.Fatal("fixture export incomplete")
			}
			// Catalog unknowns must survive whole-table addition and removal too.
			for _, drop := range []bool{false, true} {
				a, b := migration.Schema{}, current
				if drop {
					a, b = b, a
				}
				p := migration.DiffSchemas(a, b)
				if !p.ComparisonIncomplete || !migration.CompareSchemaDrift(b, a).ComparisonIncomplete || migration.EnsureExecutable(p) == nil {
					t.Fatal("whole-table export unknown lost")
				}
			}
			cols := current.Tables[0].Columns
			if cols[0].DefaultState != migration.DefaultExpression || cols[0].DefaultExpression != "1" {
				t.Fatal("numeric default unavailable")
			}
			if cols[4].DefaultState != migration.DefaultUnknown || cols[5].DefaultState != migration.DefaultUnknown {
				t.Fatal("catalog NULL promoted")
			}
			if tc.name == "mysql" {
				if cols[2].DefaultState != migration.DefaultUnknown || cols[3].DefaultState != migration.DefaultUnknown {
					t.Fatal("mysql literal reconstructed")
				}
			} else {
				if cols[2].DefaultState != migration.DefaultExpression || cols[3].DefaultState != migration.DefaultExpression {
					t.Fatal("postgres expression lost")
				}
			}
			for _, change := range []struct {
				name   string
				mutate func(*migration.Schema)
				kind   migration.MigrationStepType
			}{
				{"order", func(s *migration.Schema) {
					s.Tables[0].Indexes[0].Columns = []string{"b", "a"}
					s.Tables[0].Indexes[0].Keys[0], s.Tables[0].Indexes[0].Keys[1] = s.Tables[0].Indexes[0].Keys[1], s.Tables[0].Indexes[0].Keys[0]
				}, migration.AlterIndex},
				{"composition", func(s *migration.Schema) {
					s.Tables[0].Indexes[0].Columns = []string{"a"}
					s.Tables[0].Indexes[0].Keys = s.Tables[0].Indexes[0].Keys[:1]
				}, migration.AlterIndex},
				{"unique", func(s *migration.Schema) { s.Tables[0].Indexes[0].Unique = true }, migration.AlterIndex},
				{"default", func(s *migration.Schema) { s.Tables[0].Columns[0].DefaultExpression = "3" }, migration.AlterColumnDefault},
				{"remove default", func(s *migration.Schema) {
					s.Tables[0].Columns[0].DefaultExpression = ""
					s.Tables[0].Columns[0].HasDefault = false
					s.Tables[0].Columns[0].DefaultState = migration.DefaultNone
				}, migration.AlterColumnDefault},
			} {
				t.Run(change.name, func(t *testing.T) {
					raw, _ := json.Marshal(current)
					var desired migration.Schema
					if json.Unmarshal(raw, &desired) != nil {
						t.Fatal("decode")
					}
					change.mutate(&desired)
					before := reader.reads
					p := migration.DiffSchemas(current, desired)
					r := migration.CompareSchemaDrift(desired, current)
					if reader.reads != before {
						t.Fatal("comparison read database")
					}
					found := false
					for _, s := range p.Steps {
						if s.Type == change.kind {
							found = true
						}
					}
					if !found || !r.Drifted || !r.ComparisonIncomplete {
						t.Fatal("known changes or catalog unknown lost")
					}
				})
			}
			after, e := migration.ReadSchema(context.Background(), reader, tc.dialect, migration.WithSchemaReadTables(table))
			if e != nil {
				t.Fatal(e)
			}
			a, _ := json.Marshal(current)
			b, _ := json.Marshal(after)
			if string(a) != string(b) {
				t.Fatal("export/comparison changed schema")
			}
			unsupportedDDL := "CREATE INDEX gq_ai_11_unsupported_idx ON " + table + " (a) WHERE a > 0"
			if tc.name == "mysql" {
				unsupportedDDL = "CREATE INDEX gq_ai_11_unsupported_idx ON " + table + " (literal_value(4))"
			}
			if _, e = db.Exec(unsupportedDDL); e != nil {
				t.Fatal(e)
			}
			unsupported, e := migration.ReadSchema(context.Background(), reader, tc.dialect, migration.WithSchemaReadTables(table))
			if e != nil {
				t.Fatal(e)
			}
			foundUnknown := false
			for _, idx := range unsupported.Tables[0].Indexes {
				if idx.Name == "gq_ai_11_unsupported_idx" {
					foundUnknown = !idx.DefinitionUnknown && (idx.Predicate.State == "expression" || (len(idx.Keys) == 1 && idx.Keys[0].Prefix == 4))
				}
			}
			if !foundUnknown {
				t.Fatal("extended index representation lost")
			}
			report := migration.CompareSchemaDrift(unsupported, unsupported)
			if report.Drifted || !report.ComparisonIncomplete {
				t.Fatal("unknown index interpreted as verified match")
			}

		})
	}
}
