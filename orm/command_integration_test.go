package orm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/query"
)

// These fixtures use only explicitly configured dedicated test databases. They
// are sequential: the fixed ledger name is part of the deployment contract.
func TestCommandIntegration(t *testing.T) {
	for _, dialect := range []string{MySQL, Postgres} {
		t.Run(dialect, func(t *testing.T) {
			env := "TEST_MYSQL_DSN"
			if dialect == Postgres {
				env = "TEST_POSTGRES_DSN"
			}
			dsn := os.Getenv(env)
			if dsn == "" {
				t.Skip("explicit dedicated test DSN not set")
			}
			db, e := OpenWithDriver(dialect, dsn)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			raw := db.SQLDB()
			var version string
			if e = raw.QueryRow("SELECT version()").Scan(&version); e != nil {
				t.Fatal(e)
			}
			t.Log("database", dialect, version)
			var schema string
			q := "SELECT DATABASE()"
			if dialect == Postgres {
				q = "SELECT current_schema()"
			}
			if e = raw.QueryRow(q).Scan(&schema); e != nil {
				t.Fatal(e)
			}
			// Do not drop an existing ledger. Its presence requires an isolated test DB.
			ddl, e := os.ReadFile("../docs/sql/command-store-" + dialect + ".sql")
			if e != nil {
				t.Fatal(e)
			}
			if _, e = raw.Exec(string(ddl)); e != nil {
				t.Fatal("fixture ledger must not already exist:", e)
			}
			defer raw.Exec("DROP TABLE " + commandTable)
			if _, e = raw.Exec("CREATE TABLE gq10_items (tenant_id BIGINT NOT NULL, id BIGINT NOT NULL, score BIGINT, version BIGINT NOT NULL DEFAULT 0, PRIMARY KEY(tenant_id,id))"); e != nil {
				t.Fatal(e)
			}
			defer raw.Exec("DROP TABLE gq10_items")
			for _, table := range []string{"gq10_audit", "gq10_outbox"} {
				if _, e = raw.Exec("CREATE TABLE " + table + " (tenant_id BIGINT NOT NULL, id BIGINT NOT NULL, score BIGINT, PRIMARY KEY(tenant_id,id))"); e != nil {
					t.Fatal(e)
				}
				defer raw.Exec("DROP TABLE " + table)
			}
			makeDB := func(tenant int64) *DB {
				c, _ := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: tenant})
				cols := []query.WriteKeyColumn{{Name: "tenant_id", DBType: "bigint", Bits: 64}, {Name: "id", DBType: "bigint", Bits: 64}, {Name: "score", DBType: "bigint", Bits: 64}, {Name: "version", DBType: "bigint", Bits: 64}}
				if dialect == MySQL {
					for i := range cols {
						cols[i].DBType = "BIGINT"
					}
				}
				var tables []ApplicationTable
				var policyInputs []TablePolicy
				for _, table := range []string{"gq10_items", "gq10_audit", "gq10_outbox"} {
					name := schema + "." + table
					tableCols := cols
					if table != "gq10_items" {
						tableCols = cols[:3]
					}
					tables = append(tables, ApplicationTable{Table: name, PlainTable: true, Columns: tableCols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols[:2]}}})
					policyInputs = append(policyInputs, TablePolicy{Table: name, TenantColumn: "tenant_id"})
				}
				as, e := NewApplicationSchema(ApplicationSchemaInput{Database: "command-fixture", Dialect: dialect, Tables: tables})
				if e != nil {
					t.Fatal(e)
				}
				policies, e := NewPolicySet(policyInputs...)
				if e != nil {
					t.Fatal(e)
				}
				return db.WithOptions(WithSettings(NewSettings(policies, query.RiskConfig{}, c).WithTenantPolicy("command-fixture", as, true)))
			}
			trusted := makeDB(7)
			store, e := NewCommandStore(t.Context(), trusted, "gq10-fixture")
			if e != nil {
				t.Fatal(e)
			}
			t.Run("hooks", func(t *testing.T) { testCommandHooksIntegration(t, db, store, makeDB, schema, dialect) })
			t.Run("atomic-results", func(t *testing.T) { testAtomicIntegration(t, db, store, schema, dialect) })
			command := func(n int) Command {
				return Command{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Kind: "insert-score", Steps: []CommandStep{{Operation: "insert", Table: "gq10_items", Values: map[string]any{"id": int64(n), "score": int64(2)}}}}
			}
			count := func(n int) int {
				var v int
				if e := raw.QueryRow(fmt.Sprintf("SELECT count(*) FROM gq10_items WHERE tenant_id=7 AND id=%d", n)).Scan(&v); e != nil {
					t.Fatal(e)
				}
				return v
			}
			t.Run("concurrent-first-and-restart", func(t *testing.T) {
				c := command(1)
				start := make(chan struct{})
				results := make(chan CommandResult, 2)
				errs := make(chan error, 2)
				var wg sync.WaitGroup
				for i := 0; i < 2; i++ {
					wg.Add(1)
					go func() { defer wg.Done(); <-start; r, e := store.Run(t.Context(), c); results <- r; errs <- e }()
				}
				close(start)
				wg.Wait()
				close(results)
				close(errs)
				for e := range errs {
					if e != nil {
						t.Fatal(e)
					}
				}
				applied := 0
				for r := range results {
					if r.Applied {
						applied++
					}
					if len(r.Summary.RowsAffected) != 1 || r.Summary.RowsAffected[0] != 1 {
						t.Fatal("wrong summary")
					}
				}
				if applied != 1 || count(1) != 1 {
					t.Fatal("not once")
				}
				// New pool/store with no process-local replay state models restart/response loss.
				fresh, e := OpenWithDriver(dialect, dsn)
				if e != nil {
					t.Fatal(e)
				}
				defer fresh.Close()
				fresh = fresh.WithOptions(WithSettings(trusted.Settings()))
				next, e := NewCommandStore(t.Context(), fresh, "gq10-fixture")
				if e != nil {
					t.Fatal(e)
				}
				r, e := next.Run(t.Context(), c)
				if e != nil || r.Applied {
					t.Fatalf("restart replay: %v", e)
				}
				c.Kind = "different-kind"
				if _, e = next.Run(t.Context(), c); !errors.Is(e, ErrCommandIdentityConflict) || errors.Is(e, ErrConflict) {
					t.Fatal("kind conflict lost")
				}
				c = command(1)
				c.Steps[0].Values["score"] = int64(3)
				if _, e = next.Run(t.Context(), c); !errors.Is(e, ErrCommandIdentityConflict) {
					t.Fatal("input conflict lost")
				}
				other, e := NewCommandStore(t.Context(), makeDB(8), "gq10-fixture")
				if e != nil {
					t.Fatal(e)
				}
				r, e = other.Run(t.Context(), command(1))
				if e != nil || !r.Applied {
					t.Fatalf("tenant isolated: %v", e)
				}
			})
			t.Run("concurrent-different-input", func(t *testing.T) {
				a, b := command(2), command(2)
				b.Steps[0].Values["score"] = int64(9)
				start := make(chan struct{})
				errs := make(chan error, 2)
				for _, c := range []Command{a, b} {
					go func(c Command) { <-start; _, e := store.Run(t.Context(), c); errs <- e }(c)
				}
				close(start)
				good, conflict := 0, 0
				for i := 0; i < 2; i++ {
					e := <-errs
					if e == nil {
						good++
					} else if errors.Is(e, ErrCommandIdentityConflict) {
						conflict++
					} else {
						t.Fatal(e)
					}
				}
				if good != 1 || conflict != 1 || count(2) != 1 {
					t.Fatal("race identity mismatch")
				}
			})
			// Both callers enter the production reservation path, bypassing the initial
			// lookup only in this internal fixture. This deterministically exercises a
			// real unique violation and fresh-snapshot reread on each database.
			for _, v2 := range []bool{false, true} {
				for _, different := range []bool{false, true} {
					name := "forced-reservation-same-input"
					n := 11
					if different {
						name = "forced-reservation-different-input"
						n = 12
					}
					if v2 {
						name += "-v2"
						n += 20
					}
					t.Run(name, func(t *testing.T) {
						start := make(chan struct{})
						out := make(chan CommandResult, 2)
						errs := make(chan error, 2)
						for i := 0; i < 2; i++ {
							c := command(n)
							if v2 {
								one := int64(1)
								c.Steps[0].ExpectAffected = &one
							}
							if different && i == 1 {
								c.Steps[0].Values["score"] = int64(9)
							}
							b, requests, e := snapshotCommand(c, store.schema)
							if e != nil {
								t.Fatal(e)
							}
							plan := &compoundPlan{}
							for _, r := range requests {
								if e = plan.add(t.Context(), store.db, "command", 0, 1, r, nil); e != nil {
									t.Fatal(e)
								}
							}
							fp, e := commandFingerprint(b)
							if e != nil {
								t.Fatal(e)
							}
							go func(c Command, b []byte, fp [32]byte, p *compoundPlan) {
								<-start
								r, e := store.apply(t.Context(), c.ID, b, fp, p)
								out <- r
								errs <- e
							}(c, b, fp, plan)
						}
						close(start)
						applied, replayed, conflicted := 0, 0, 0
						for i := 0; i < 2; i++ {
							r := <-out
							if r.Applied {
								applied++
							} else if r.Summary.Version == 1 {
								replayed++
							}
							e := <-errs
							if errors.Is(e, ErrCommandIdentityConflict) {
								conflicted++
							} else if e != nil {
								t.Fatal(e)
							}
						}
						if applied != 1 || count(n) != 1 {
							t.Fatal("reservation race applied more than once")
						}
						if different {
							if conflicted != 1 || replayed != 0 {
								t.Fatal("race mismatch was reused")
							}
						} else if replayed != 1 || conflicted != 0 {
							t.Fatal("race result not replayed")
						}
					})
				}

			}
			t.Run("business-failure-rollback-and-retry", func(t *testing.T) {
				c := command(3)
				c.Steps = append(c.Steps, c.Steps[0])
				if _, e := store.Run(t.Context(), c); e == nil || errors.Is(e, ErrCommandIdentityConflict) {
					t.Fatal("business uniqueness treated as replay")
				}
				if count(3) != 0 {
					t.Fatal("business rollback failed")
				}
				c.Steps = c.Steps[:1]
				r, e := store.Run(t.Context(), c)
				if e != nil || !r.Applied {
					t.Fatalf("retry: %v", e)
				}
			})
			t.Run("result-save-failure", func(t *testing.T) {
				// Restrict only this operation's result; reservation still succeeds.
				target := command(4)
				idExpr := "CAST(operation_id AS CHAR)"
				if dialect == Postgres {
					idExpr = "convert_from(operation_id,'UTF8')"
				}
				check := "ALTER TABLE " + commandTable + " ADD CONSTRAINT gq10_save_failure CHECK (status=0 OR " + idExpr + " <> '" + target.ID + "')"
				if _, e := raw.Exec(check); e != nil {
					t.Fatal(e)
				}
				_, e := store.Run(t.Context(), target)
				if e == nil {
					t.Fatal("save unexpectedly succeeded")
				}
				if count(4) != 0 {
					t.Fatal("save failure retained business update")
				}
				drop := "ALTER TABLE " + commandTable + " DROP CONSTRAINT gq10_save_failure"
				if dialect == MySQL {
					drop = "ALTER TABLE " + commandTable + " DROP CHECK gq10_save_failure"
				}
				if _, e = raw.Exec(drop); e != nil {
					t.Fatal(e)
				}
				r, e := store.Run(t.Context(), target)
				if e != nil || !r.Applied {
					t.Fatalf("save retry: %v", e)
				}
			})
			t.Run("update-counts-and-zero", func(t *testing.T) {
				c := command(5)
				c.Steps = []CommandStep{{Operation: "update", Table: "gq10_items", Values: map[string]any{"score": int64(2)}, Keys: map[string]any{"id": int64(1)}}}
				r, e := store.Run(t.Context(), c)
				if e != nil {
					t.Fatal(e)
				}
				want := int64(0)
				if dialect == Postgres || strings.Contains(dsn, "clientFoundRows=true") {
					want = 1
				}
				if r.Summary.RowsAffected[0] != want {
					t.Fatal("unchanged update semantics")
				}
				c.ID = command(6).ID
				c.Steps[0].Keys["id"] = int64(900)
				r, e = store.Run(t.Context(), c)
				if e != nil || r.Summary.RowsAffected[0] != 0 {
					t.Fatalf("zero: %v", e)
				}
			})
			t.Run("cancel-while-reservation-waits", func(t *testing.T) {
				c := command(7)
				tx, e := raw.Begin()
				if e != nil {
					t.Fatal(e)
				}
				input, _, _ := snapshotCommand(c, schema)
				if _, e = tx.Exec(store.sql("INSERT INTO "+store.table+" (scope_hash,operation_id,format_version,canonical_scope,command_input,fingerprint,status,result) VALUES (?,?,1,?,?,?,0,?)"), store.scopeHash[:], []byte(c.ID), store.scope, input, make([]byte, 32), []byte{}); e != nil {
					tx.Rollback()
					t.Fatal(e)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
				defer cancel()
				_, e = store.Run(ctx, c)
				if e == nil {
					tx.Rollback()
					t.Fatal("cancel accepted")
				}
				if e = tx.Rollback(); e != nil {
					t.Fatal(e)
				}
				if count(7) != 0 {
					t.Fatal("cancel wrote")
				}
				r, e := store.Run(t.Context(), c)
				if e != nil || !r.Applied {
					t.Fatalf("cancel retry: %v", e)
				}
			})
			t.Run("strict-and-ledger-refusals", func(t *testing.T) {
				c := command(8)
				c.Steps[0].Values["tenant_id"] = int64(99)
				if _, e := store.Run(t.Context(), c); e == nil {
					t.Fatal("tenant mismatch accepted")
				}
				if count(8) != 0 {
					t.Fatal("rejection wrote")
				}
				c = command(8)
				c.Steps = append(c.Steps, CommandStep{Operation: "insert", Table: commandTable, Values: map[string]any{"status": 1}})
				if _, e := store.Run(t.Context(), c); !errors.Is(e, ErrCommandInput) {
					t.Fatal("ledger write accepted")
				}
				if count(8) != 0 {
					t.Fatal("late rejection wrote")
				}
				_, e := RunIdempotentCommand(t.Context(), trusted, IdempotentCommandSpec[int]{LookupExisting: func(context.Context, *DB) (int, error) { t.Fatal("opaque lookup called"); return 0, nil }, Apply: func(context.Context, Tx) (int, error) { t.Fatal("opaque apply called"); return 0, nil }})
				if !errors.Is(e, ErrUnsupportedCompound) {
					t.Fatal("old Strict refusal changed")
				}
				tx, e := raw.Begin()
				if e != nil {
					t.Fatal(e)
				}
				defer tx.Rollback()
				for _, d := range []*DB{trusted.WrapTx(tx), NewTxDB(tx, db.Dialect()), trusted.WrapExecutor(raw), NewDBWithExecutor(raw, db.Dialect())} {
					if _, e := NewCommandStore(t.Context(), d, "fixture"); !errors.Is(e, ErrCommandOwnership) {
						t.Fatal("external owner accepted")
					}
				}
			})

			t.Run("current-policy-and-namespace", func(t *testing.T) {
				restricted, _ := NewPolicySet(TablePolicy{Table: schema + ".gq10_items", TenantColumn: "tenant_id", ForbiddenColumns: []string{"score"}})
				denied, e := NewCommandStore(t.Context(), trusted.WithOptions(WithPolicySet(restricted)), "gq10-fixture")
				if e != nil {
					t.Fatal(e)
				}
				if _, e = denied.Run(t.Context(), command(1)); !errors.Is(e, ErrBlockedOperation) {
					t.Fatal("replay bypassed current policy")
				}
				other, e := NewCommandStore(t.Context(), trusted, "other-namespace")
				if e != nil {
					t.Fatal(e)
				}
				c := command(10)
				c.ID = command(1).ID
				r, e := other.Run(t.Context(), c)
				if e != nil || !r.Applied || count(10) != 1 {
					t.Fatalf("namespace result reused: %v", e)
				}
			})
			t.Run("corrupt-and-schema-refusal", func(t *testing.T) {
				c := command(1)
				for _, assignment := range []string{"format_version=99", "status=0", "result='{}'"} {
					var original []byte
					var v, status int
					if e = raw.QueryRow(store.sql("SELECT format_version,status,result FROM "+store.table+" WHERE scope_hash=? AND operation_id=?"), store.scopeHash[:], []byte(c.ID)).Scan(&v, &status, &original); e != nil {
						t.Fatal(e)
					}
					if _, e = raw.Exec(store.sql("UPDATE "+store.table+" SET "+assignment+" WHERE scope_hash=? AND operation_id=?"), store.scopeHash[:], []byte(c.ID)); e != nil {
						t.Fatal(e)
					}
					if _, e = store.Run(t.Context(), c); !errors.Is(e, ErrCommandCorrupt) {
						t.Fatal("corruption replayed")
					}
					if _, e = raw.Exec(store.sql("UPDATE "+store.table+" SET format_version=?,status=?,result=? WHERE scope_hash=? AND operation_id=?"), v, status, original, store.scopeHash[:], []byte(c.ID)); e != nil {
						t.Fatal(e)
					}
				}
				if _, e = raw.Exec("ALTER TABLE " + commandTable + " ADD extra_column INT"); e != nil {
					t.Fatal(e)
				}
				if _, e = store.Run(t.Context(), command(9)); !errors.Is(e, ErrCommandStore) {
					t.Fatal("schema drift accepted")
				}
				if count(9) != 0 {
					t.Fatal("schema refusal wrote")
				}
				if _, e = raw.Exec("ALTER TABLE " + commandTable + " DROP COLUMN extra_column"); e != nil {
					t.Fatal(e)
				}
				if _, e = raw.Exec("CREATE UNIQUE INDEX gq10_extra_unique ON " + commandTable + " (scope_hash,operation_id,fingerprint)"); e != nil {
					t.Fatal(e)
				}
				if _, e = NewCommandStore(t.Context(), trusted, "gq10-fixture"); !errors.Is(e, ErrCommandStore) {
					t.Fatal("secondary unique accepted")
				}
				drop := "DROP INDEX gq10_extra_unique"
				if dialect == MySQL {
					drop += " ON " + commandTable
				}
				if _, e = raw.Exec(drop); e != nil {
					t.Fatal(e)
				}
				rename := "ALTER TABLE " + commandTable + " RENAME TO gq10_missing_ledger"
				if _, e = raw.Exec(rename); e != nil {
					t.Fatal(e)
				}
				_, missing := NewCommandStore(t.Context(), trusted, "gq10-fixture")
				if _, e = raw.Exec("ALTER TABLE gq10_missing_ledger RENAME TO " + commandTable); e != nil {
					t.Fatal(e)
				}
				if missing == nil {
					t.Fatal("missing schema accepted")
				}

			})
		})
	}
}
