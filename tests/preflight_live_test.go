package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/migration"
)

// Fixture DDL/DML is deliberately separate from the SELECT-only collector.
func TestPreflightLive(t *testing.T) {
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
			if e = db.Ping(); e != nil {
				t.Fatal(e)
			}
			exec := func(q string) {
				t.Helper()
				if _, e := db.Exec(q); e != nil {
					t.Fatal(e)
				}
			}
			const ns = "gq_ai11_pr3_data"
			const refs = "gq_ai11_pr3_refs"
			suffix := ""
			if tc.name == "postgres" {
				suffix = " CASCADE"
			}
			for _, s := range []string{ns, refs} {
				exec("DROP SCHEMA IF EXISTS " + s + suffix)
				exec("CREATE SCHEMA " + s)
			}
			defer db.Exec("DROP SCHEMA " + refs + suffix)
			defer db.Exec("DROP SCHEMA " + ns + suffix)
			exec("CREATE TABLE " + ns + ".child (a INTEGER,b INTEGER)")
			exec("CREATE TABLE " + refs + ".parent (x INTEGER,y INTEGER,PRIMARY KEY(x,y))")
			exec("INSERT INTO " + refs + ".parent VALUES (1,2),(3,4)")
			clock := func() time.Time { return time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC) }
			opts := migration.PreflightOptions{Clock: clock, MaxAge: migration.PreflightMaxAge}
			read := func(s, table string) migration.Schema {
				t.Helper()
				v, e := migration.ReadSchema(context.Background(), db, tc.dialect, migration.WithSchemaReadSchema(s), migration.WithSchemaReadTables(table))
				if e != nil {
					t.Fatal(e)
				}
				return v
			}
			mkplan := func(kind string) *migration.Preflight {
				t.Helper()
				current := read(ns, "child")
				reference := read(refs, "parent")
				b, _ := json.Marshal(current)
				var desired migration.Schema
				if json.Unmarshal(b, &desired) != nil {
					t.Fatal("clone")
				}
				step := migration.MigrationStep{Table: "child"}
				n := false
				switch kind {
				case "null":
					step.Type = migration.AlterNullability
					step.Column = "a"
					step.Nullable = &n
					desired.Tables[0].Columns[0].Nullable = false
				case "unique":
					step.Type = migration.AddIndex
					step.Index = "pair_unique"
					desired.Tables[0].Indexes = append(desired.Tables[0].Indexes, migration.IndexSchema{Name: "pair_unique", Columns: []string{"b", "a"}, Unique: true, Method: "btree", Predicate: migration.ExpressionDefinition{State: "none"}, Visibility: "visible", Attributes: migration.CoverageComplete})
				case "fk":
					step.Type = migration.AddForeignKey
					step.Constraint = "pair_fk"
					desired.Tables[0].ForeignKeys = append(desired.Tables[0].ForeignKeys, migration.ForeignKeySchema{Name: "pair_fk", Columns: []string{"a", "b"}, Reference: reference.Tables[0].Identity, ReferencedColumns: []string{"x", "y"}, OnUpdate: "no_action", OnDelete: "no_action", Match: "simple", Validation: "validated", Enforcement: "enforced", Deferrability: "not_deferrable", Definition: migration.CoverageComplete})
				}
				p, e := migration.PlanPreflight(&migration.MigrationPlan{CurrentSchema: &current, DesiredSchema: &desired, Steps: []migration.MigrationStep{step}}, []migration.PhaseContract{{Phase: "contract", OldReaders: "compatible", OldWriters: "compatible"}}, []migration.Schema{reference})
				if e != nil {
					t.Fatal(e)
				}
				return p
			}
			collect := func(p *migration.Preflight, want string) *migration.Preflight {
				t.Helper()
				c, e := migration.NewPreflightCollector(context.Background(), db, tc.dialect, migration.CollectorOptions{Clock: clock})
				if e != nil {
					t.Fatal(e)
				}
				defer c.Close()
				out, e := c.Collect(context.Background(), p)
				if e != nil {
					t.Fatalf("collect: %v source=%+v", e, out.Collection)
				}
				v, e := c.Evaluate(out, opts)
				if e != nil || v.Status != want {
					t.Fatalf("want %s got %v err=%v source=%+v", want, v, e, out.Collection)
				}
				external, e := migration.EvaluatePreflight(out, opts)
				if e != nil || external.Status == "satisfied" {
					t.Fatal("offline adopted receipt")
				}
				other, e := migration.NewPreflightCollector(context.Background(), db, tc.dialect, migration.CollectorOptions{Clock: clock})
				if e != nil {
					t.Fatal(e)
				}
				ov, e := other.Evaluate(out, opts)
				other.Close()
				if e != nil || ov.Status == "satisfied" {
					t.Fatal("different session accepted")
				}
				if e = c.Close(); e != nil {
					t.Fatal(e)
				}
				cv, _ := c.Evaluate(out, opts)
				if cv.Status == "satisfied" {
					t.Fatal("closed session")
				}
				return out
			}
			for _, kind := range []string{"null", "unique", "fk"} {
				t.Run(kind, func(t *testing.T) {
					exec("DELETE FROM " + ns + ".child")
					exec("INSERT INTO " + ns + ".child VALUES (1,2),(3,4)")
					p := mkplan(kind)
					collect(p, "satisfied")
					switch kind {
					case "null":
						exec("INSERT INTO " + ns + ".child VALUES (NULL,2)")
					case "unique":
						exec("INSERT INTO " + ns + ".child VALUES (1,2)")
					case "fk":
						exec("INSERT INTO " + ns + ".child VALUES (2,1)")
					}
					out := collect(p, "unsatisfied")
					if out.Collection.Results[0].Count == nil || *out.Collection.Results[0].Count != 1 {
						t.Fatal("expected one violating row/group")
					}
					if kind != "null" {
						exec("DELETE FROM " + ns + ".child")
						exec("INSERT INTO " + ns + ".child VALUES (NULL,2),(NULL,2),(1,NULL)")
						collect(p, "satisfied")
					}
				})
			}
			p := mkplan("null")
			exec("ALTER TABLE " + ns + ".child ADD COLUMN changed INTEGER")
			c, e := migration.NewPreflightCollector(context.Background(), db, tc.dialect, migration.CollectorOptions{Clock: clock})
			if e != nil {
				t.Fatal(e)
			}
			out, e := c.Collect(context.Background(), p)
			if e == nil || out == nil || out.Collection.Completed {
				t.Fatal("schema mismatch accepted")
			}
			c.Close()
			exec("ALTER TABLE " + ns + ".child DROP COLUMN changed")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			c, e = migration.NewPreflightCollector(context.Background(), db, tc.dialect, migration.CollectorOptions{Clock: clock})
			if e != nil {
				t.Fatal(e)
			}
			out, e = c.Collect(ctx, p)
			if e == nil || out == nil || out.Collection.Completed {
				t.Fatal("cancel accepted")
			}
			c.Close()
			// Hold a fixture-only lock; the collector must time out without issuing
			// any lock or write of its own.
			func() {
				lockConn, err := db.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer lockConn.Close()
				if tc.name == "postgres" {
					tx, err := lockConn.BeginTx(context.Background(), nil)
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback()
					if _, err = tx.Exec("LOCK TABLE " + ns + ".child IN ACCESS EXCLUSIVE MODE"); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err = lockConn.ExecContext(context.Background(), "LOCK TABLES "+ns+".child WRITE"); err != nil {
						t.Fatal(err)
					}
					defer lockConn.ExecContext(context.Background(), "UNLOCK TABLES")
				}
				bounded, err := migration.NewPreflightCollector(context.Background(), db, tc.dialect, migration.CollectorOptions{Clock: clock, QueryTimeout: 10 * time.Millisecond, TotalTimeout: 500 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				defer bounded.Close()
				result, err := bounded.Collect(context.Background(), p)
				if err == nil || result == nil || result.Collection.Completed {
					t.Fatal("lock timeout became success")
				}
			}()
			if tc.name == "postgres" {
				exec("ALTER TABLE " + ns + ".child ENABLE ROW LEVEL SECURITY")
				p = mkplan("null")
				collect(p, "unknown")
				exec("ALTER TABLE " + ns + ".child DISABLE ROW LEVEL SECURITY")
			}
			t.Logf("real %s version %s; null/duplicate groups/orphan tuples, nullable tuples, other session/close, schema mutation/cancel", tc.name, read(ns, "child").Context.ServerVersion)
		})
	}
}
