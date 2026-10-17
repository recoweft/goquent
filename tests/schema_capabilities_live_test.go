package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/migration"
)

// Only these dedicated namespaces are modified. Export receives a SELECT-only
// executor; desired comparison never receives a connection.
func TestSchemaCapabilitiesLive(t *testing.T) {
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
			if err = db.Ping(); err != nil {
				t.Fatal(err)
			}
			exec := func(q string) {
				t.Helper()
				if _, err := db.Exec(q); err != nil {
					t.Fatal(err)
				}
			}
			const ns = "gq_ai11_pr2_a"
			const other = "gq_ai11_pr2_b"
			drop := "DROP SCHEMA IF EXISTS "
			suffix := ""
			if tc.name == "postgres" {
				suffix = " CASCADE"
			}
			exec(drop + ns + suffix)
			exec(drop + other + suffix)
			exec("CREATE SCHEMA " + ns)
			exec("CREATE SCHEMA " + other)
			defer db.Exec(drop + other + suffix)
			defer db.Exec(drop + ns + suffix)
			exec("CREATE TABLE " + other + ".parent (x INTEGER,y INTEGER,PRIMARY KEY(x,y))")
			exec("CREATE TABLE " + ns + ".child (a INTEGER DEFAULT 1,b INTEGER DEFAULT 2,c INTEGER DEFAULT 3,v TEXT,CONSTRAINT pair_fk FOREIGN KEY(a,b) REFERENCES " + other + ".parent(x,y) ON UPDATE CASCADE ON DELETE RESTRICT,CONSTRAINT positive_ck CHECK(c > 0))")
			exec("CREATE TABLE " + other + ".child (a INTEGER,b INTEGER,c INTEGER,CONSTRAINT positive_ck CHECK(c > 0))")
			exec("CREATE INDEX expression_ix ON " + ns + ".child ((c + 1),b)")
			if tc.name == "postgres" {
				exec("CREATE INDEX partial_ix ON " + ns + ".child (b,a) WHERE a > 1")
				exec("CREATE INDEX unsupported_ix ON " + ns + ".child USING hash(a)")
				exec("ALTER TABLE " + ns + ".child ADD CONSTRAINT unvalidated_ck CHECK(b > 0) NOT VALID")
			} else {
				exec("CREATE FULLTEXT INDEX unsupported_ix ON " + ns + ".child (v)")
				// MySQL has no partial-index WHERE syntax: measure rejection explicitly.
				if _, e := db.Exec("CREATE INDEX partial_ix ON " + ns + ".child (a) WHERE a > 1"); e == nil {
					t.Fatal("unexpected partial index support")
				}
				t.Log("MySQL partial index fixture: unsupported syntax rejected")
			}
			reader := &schemaReadOnly{db: db, t: t}
			read := func(namespace string) migration.Schema {
				t.Helper()
				s, e := migration.ReadSchema(context.Background(), reader, tc.dialect, migration.WithSchemaReadSchema(namespace))
				if e != nil {
					t.Fatal(e)
				}
				return s
			}
			current := read(ns)
			different := read(other)
			if current.Context == nil || current.Context.ServerVersion == "" || current.Context.Schema != ns || len(current.Tables) != 1 {
				t.Fatal("catalog context absent")
			}
			t.Logf("actual catalog version %s; scope=%s inventory=%s", current.Context.ServerVersion, current.Context.Scope, current.Context.Coverage.Tables)
			table := current.Tables[0]
			unsupported := false
			for _, i := range table.Indexes {
				if i.Name == "unsupported_ix" && i.DefinitionUnknown && i.RawDefinition != "" {
					unsupported = true
				}
			}
			if !unsupported {
				t.Fatal("unsupported real index lost raw/unknown")
			}
			t.Log("non-btree index fixture retained as unsupported/unknown")
			if len(table.ForeignKeys) != 1 || table.ForeignKeys[0].Reference.Schema != other || len(table.ForeignKeys[0].Columns) != 2 || table.ForeignKeys[0].Columns[0] != "a" || table.ForeignKeys[0].ReferencedColumns[1] != "y" || table.ForeignKeys[0].OnUpdate != "cascade" {
				t.Fatal("ordered cross-schema reference lost")
			}
			if len(table.Checks) == 0 || table.Checks[0].Expression == "" {
				t.Fatal("check missing")
			}
			ix := -1
			for i, v := range table.Indexes {
				if v.Name == "expression_ix" {
					ix = i
				}
			}
			if ix < 0 || len(table.Indexes[ix].Keys) != 2 || table.Indexes[ix].Keys[0].Expression == "" || table.Indexes[ix].DefinitionUnknown {
				t.Fatal("expression index missing")
			}
			if tc.name == "postgres" {
				found := false
				for _, c := range table.Checks {
					if c.Name == "unvalidated_ck" && c.Validation == "not_validated" {
						found = true
					}
				}
				if !found {
					t.Fatal("validation state lost")
				}
			}
			before := reader.reads
			for _, change := range []struct {
				name   string
				mutate func(*migration.TableSchema)
				kind   migration.MigrationStepType
			}{
				{"fk_order", func(t *migration.TableSchema) { t.ForeignKeys[0].Columns = []string{"b", "a"} }, migration.AlterForeignKey},
				{"fk_reference_order", func(t *migration.TableSchema) { t.ForeignKeys[0].ReferencedColumns = []string{"y", "x"} }, migration.AlterForeignKey},
				{"fk_reference_table", func(t *migration.TableSchema) { t.ForeignKeys[0].Reference.Name = "another" }, migration.AlterForeignKey},
				{"fk_action", func(t *migration.TableSchema) { t.ForeignKeys[0].OnDelete = "cascade" }, migration.AlterForeignKey},
				{"fk_schema", func(t *migration.TableSchema) { t.ForeignKeys[0].Reference.Schema = "another" }, migration.AlterForeignKey},
				{"check", func(t *migration.TableSchema) { t.Checks[0].Expression = "a > 9" }, migration.AlterCheck},
				{"expression", func(t *migration.TableSchema) { t.Indexes[ix].Keys[0].Expression = "a + 2" }, migration.AlterIndex},
			} {
				t.Run(change.name, func(t *testing.T) {
					raw, _ := json.Marshal(current)
					var desired migration.Schema
					if e := json.Unmarshal(raw, &desired); e != nil {
						t.Fatal(e)
					}
					change.mutate(&desired.Tables[0])
					p := migration.DiffSchemas(current, desired)
					r := migration.CompareSchemaDrift(desired, current)
					found := false
					for _, s := range p.Steps {
						if s.Type == change.kind {
							found = true
						}
					}
					if !found || !r.Drifted || !r.ComparisonIncomplete || migration.EnsureExecutable(p) == nil {
						t.Fatal("known difference/unknown boundary lost")
					}
					view, _ := p.PublicView()
					pub, _ := view.ToJSON()
					for _, secret := range []string{ns, other, "positive_ck", "a > 9", current.Context.ServerVersion} {
						if bytes.Contains(pub, []byte(secret)) {
							t.Fatal("public source leak")
						}
					}
				})
			}
			if tc.name == "postgres" {
				for _, kind := range []string{"validation", "predicate"} {
					raw, _ := json.Marshal(current)
					var desired migration.Schema
					if json.Unmarshal(raw, &desired) != nil {
						t.Fatal("decode")
					}
					expected := migration.AlterCheck
					if kind == "validation" {
						desired.Tables[0].Checks[0].Validation = "not_validated"
					} else {
						expected = migration.AlterIndex
						for i := range desired.Tables[0].Indexes {
							if desired.Tables[0].Indexes[i].Name == "partial_ix" {
								desired.Tables[0].Indexes[i].Predicate.SQL = "a > 2"
							}
						}
					}
					found := false
					for _, step := range migration.DiffSchemas(current, desired).Steps {
						if step.Type == expected {
							found = true
						}
					}
					if !found {
						t.Fatal("live validation/predicate difference lost")
					}
				}
			}
			if reader.reads != before {
				t.Fatal("static comparison connected")
			}
			p := migration.DiffSchemas(current, different)
			if !p.ComparisonIncomplete {
				t.Fatal("different namespace match")
			}
			for _, step := range p.Steps {
				if step.Type == migration.DropTable || step.Type == migration.AddTable {
					t.Fatal("scope mismatch became definite collection change")
				}
			}
			a, _ := json.Marshal(current)
			b, _ := json.Marshal(read(ns))
			if !bytes.Equal(a, b) {
				t.Fatal("SELECT export or comparison changed database")
			}
			testCapabilityDDLFixture(t, db, tc.name)
			// Removing an observed table from a visibility-limited snapshot is unknown.
			var empty migration.Schema
			if json.Unmarshal(a, &empty) != nil {
				t.Fatal("decode")
			}
			empty.Tables = nil
			p = migration.DiffSchemas(current, empty)
			for _, s := range p.Steps {
				if s.Type == migration.DropTable {
					t.Fatal("partial inventory became definite drop")
				}
			}
		})
	}
}

func testCapabilityDDLFixture(t *testing.T, db *sql.DB, dialect string) {
	t.Helper()
	const table = "gq_ai11_pr2_a.tx_probe"
	if _, err := db.Exec("CREATE TABLE " + table + " (a INTEGER)"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if dialect == "postgres" {
		if _, err = tx.Exec("CREATE INDEX CONCURRENTLY forbidden_tx_ix ON " + table + " (a)"); err == nil {
			tx.Rollback()
			t.Fatal("CONCURRENTLY unexpectedly accepted in transaction")
		}
		tx.Rollback()
		tx, err = db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec("CREATE INDEX rolled_back_ix ON " + table + " (a)"); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if err = tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		var count int
		if err = db.QueryRow("SELECT count(*) FROM pg_indexes WHERE schemaname='gq_ai11_pr2_a' AND indexname='rolled_back_ix'").Scan(&count); err != nil || count != 0 {
			t.Fatal("transactional fixture did not roll back")
		}
	} else {
		if _, err = tx.Exec("INSERT INTO " + table + " VALUES(1)"); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		if _, err = tx.Exec("CREATE INDEX committed_ix ON " + table + " (a)"); err != nil {
			tx.Rollback()
			t.Fatal(err)
		}
		tx.Rollback()
		var count int
		if err = db.QueryRow("SELECT count(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatal("implicit commit fixture not observed")
		}
	}
	t.Log("dedicated DDL fixture confirmed transaction/concurrent syntax boundary; lock duration and rebuild unmeasured")
}
