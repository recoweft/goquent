package orm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// Called by the explicitly configured, sequential CommandIntegration fixture.
// DDL below applies only to tables created by that fixture; it never cleans up
// a pre-existing application ledger. All four durable effects are queried.
func testCommandHooksIntegration(t *testing.T, db *DB, store *CommandStore, makeDB func(int64) *DB, schema, dialect string) {
	raw := db.SQLDB()
	command := func(n int) Command {
		one := int64(1)
		return Command{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", n), Kind: "audit-outbox", Steps: []CommandStep{{Operation: "insert", Table: "gq10_items", Values: map[string]any{"id": int64(n), "score": int64(2)}, ExpectAffected: &one}}, Hooks: []CommandHook{
			{Name: "audit", Table: "gq10_audit", Values: map[string]any{"id": int64(n), "score": int64(3)}},
			{Name: "outbox", Table: "gq10_outbox", Values: map[string]any{"id": int64(n), "score": int64(4)}},
		}}
	}
	assertCounts := func(t *testing.T, n int, tenant int64, want int) {
		t.Helper()
		for _, table := range []string{"gq10_items", "gq10_audit", "gq10_outbox"} {
			var got int
			if e := raw.QueryRow(store.sql("SELECT count(*) FROM "+table+" WHERE id=? AND tenant_id=?"), n, tenant).Scan(&got); e != nil {
				t.Fatal(e)
			}
			if got != want {
				t.Fatalf("%s id=%d tenant=%d count=%d want=%d", table, n, tenant, got, want)
			}
		}
		// Scope-specific ledger checks must not confuse the other tenant's record.
		s := store
		if tenant != 7 {
			var e error
			s, e = NewCommandStore(t.Context(), makeDB(tenant), "gq10-fixture")
			if e != nil {
				t.Fatal(e)
			}
		}
		var got int
		if e := raw.QueryRow(s.sql("SELECT count(*) FROM "+s.table+" WHERE operation_id=? AND scope_hash=?"), []byte(command(n).ID), s.scopeHash[:]).Scan(&got); e != nil {
			t.Fatal(e)
		}
		if got != want {
			t.Fatalf("ledger count=%d want=%d", got, want)
		}
	}
	run := func(t *testing.T, c Command, applied bool) {
		t.Helper()
		r, e := store.Run(t.Context(), c)
		if e != nil || r.Applied != applied || !reflect.DeepEqual(r.Summary.RowsAffected, []int64{1, 1, 1}) {
			t.Fatalf("run applied=%v: %+v %v", applied, r, e)
		}
	}
	t.Run("success-replay-input-and-tenant", func(t *testing.T) {
		c := command(501)
		run(t, c, true)
		run(t, c, false)
		assertCounts(t, 501, 7, 1)
		// Reconstruct a store: neither hook registration nor replay is process-local.
		fresh, e := NewCommandStore(t.Context(), makeDB(7), "gq10-fixture")
		if e != nil {
			t.Fatal(e)
		}
		r, e := fresh.Run(t.Context(), c)
		if e != nil || r.Applied {
			t.Fatal(r, e)
		}
		for _, mutate := range []func(*Command){
			func(c *Command) { c.Hooks[0].Values["score"] = int64(9) },
			func(c *Command) { c.Hooks[0].Name = "changed-config" },
			func(c *Command) { c.Hooks[0], c.Hooks[1] = c.Hooks[1], c.Hooks[0] },
			func(c *Command) { c.Hooks = nil },
		} {
			c := command(501)
			mutate(&c)
			if _, e = store.Run(t.Context(), c); !errors.Is(e, ErrCommandIdentityConflict) || errors.Is(e, ErrConflict) {
				t.Fatal("hook identity", e)
			}
			assertCounts(t, 501, 7, 1)
		}
		other, e := NewCommandStore(t.Context(), makeDB(8), "gq10-fixture")
		if e != nil {
			t.Fatal(e)
		}
		// Same UUID, distinct trusted scope, distinct data and result.
		c = command(501)
		c.Hooks[0].Values["score"] = int64(88)
		r, e = other.Run(t.Context(), c)
		if e != nil || !r.Applied {
			t.Fatal("independent tenant", e)
		}
		assertCounts(t, 501, 8, 1)
		assertCounts(t, 501, 7, 1)
		for _, tenant := range []int64{7, 8} {
			var score int64
			if e = raw.QueryRow(store.sql("SELECT score FROM gq10_audit WHERE id=501 AND tenant_id=?"), tenant).Scan(&score); e != nil {
				t.Fatal(e)
			}
			want := int64(3)
			if tenant == 8 {
				want = 88
			}
			if score != want {
				t.Fatal("scope leaked", score)
			}
		}
	})
	for i, which := range []string{"business", "audit", "outbox"} {
		t.Run("wrong-tenant-"+which, func(t *testing.T) {
			n := 510 + i
			c := command(n)
			if which == "business" {
				c.Steps[0].Values["tenant_id"] = int64(8)
			} else {
				j := 0
				if which == "outbox" {
					j = 1
				}
				c.Hooks[j].Values["tenant_id"] = int64(8)
			}
			if _, e := store.Run(t.Context(), c); !errors.Is(e, ErrBlockedOperation) {
				t.Fatal("wrong tenant accepted", e)
			}
			assertCounts(t, n, 7, 0)
			assertCounts(t, n, 8, 0)
		})
	}
	for i, which := range []string{"audit", "outbox", "both", "save"} {
		t.Run("rollback-and-same-input-recovery-"+which, func(t *testing.T) {
			n := 520 + i
			c := command(n)
			tables := []string{"gq10_" + which}
			if which == "both" {
				tables = []string{"gq10_audit", "gq10_outbox"}
			}
			if which == "save" {
				tables = []string{commandTable}
			}
			for _, table := range tables {
				predicate := fmt.Sprintf("id <> %d", n)
				if table == commandTable {
					idExpr := "CAST(operation_id AS CHAR)"
					if dialect == Postgres {
						idExpr = "convert_from(operation_id,'UTF8')"
					}
					predicate = "status=0 OR " + idExpr + " <> '" + c.ID + "'"
				}
				if _, e := raw.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + table + "_hook_failure CHECK (" + predicate + ")"); e != nil {
					t.Fatal(e)
				}
			}
			_, e := store.Run(t.Context(), c)
			if e == nil || errors.Is(e, ErrCommandIdentityConflict) {
				t.Fatal("failure treated as replay", e)
			}
			assertCounts(t, n, 7, 0)
			for _, table := range tables {
				drop := " DROP CONSTRAINT "
				if dialect == MySQL {
					drop = " DROP CHECK "
				}
				if _, e = raw.Exec("ALTER TABLE " + table + drop + table + "_hook_failure"); e != nil {
					t.Fatal(e)
				}
			}
			run(t, c, true)
			run(t, c, false)
			assertCounts(t, n, 7, 1)
		})
	}
	for i, mode := range []string{"zero", "multiple", "cas", "later-business"} {
		t.Run("prior-step-rollback-"+mode, func(t *testing.T) {
			n := 530 + i
			c := command(n)
			one := int64(1)
			next := CommandStep{Operation: "update", Table: "gq10_items", Values: map[string]any{"score": int64(9)}, Keys: map[string]any{"id": int64(999999)}, ExpectAffected: &one}
			if mode == "multiple" {
				next.Keys = map[string]any{"score": int64(2)}
			}
			if mode == "cas" {
				next.Keys = map[string]any{"id": int64(n)}
				next.Version = &CommandVersion{Column: "version", Expected: 9}
			}
			if mode == "later-business" {
				next = c.Steps[0]
			}
			c.Steps = append(c.Steps, next)
			_, e := store.Run(t.Context(), c)
			if e == nil || errors.Is(e, ErrCommandIdentityConflict) {
				t.Fatal("business rejection", e)
			}
			if mode == "cas" && !errors.Is(e, ErrConflict) {
				t.Fatal("CAS identity", e)
			}
			assertCounts(t, n, 7, 0)
			var score int64
			if e = raw.QueryRow("SELECT score FROM gq10_items WHERE tenant_id=7 AND id=501").Scan(&score); e != nil || score != 2 {
				t.Fatal("prior data changed", score, e)
			}
		})
	}
	// Force the production reservation path after preflight, without the initial
	// lookup. This internal seam guarantees a real unique conflict on both DBs.
	for i, mode := range []string{"public-first", "reservation-same", "reservation-different"} {
		t.Run(mode, func(t *testing.T) {
			n := 540 + i
			start := make(chan struct{})
			type outcome struct {
				r CommandResult
				e error
			}
			out := make(chan outcome, 2)
			for j := 0; j < 2; j++ {
				c := command(n)
				if mode == "reservation-different" && j == 1 {
					c.Hooks[0].Values["score"] = int64(8)
				}
				if mode == "public-first" {
					go func(c Command) { <-start; r, e := store.Run(t.Context(), c); out <- outcome{r, e} }(c)
				} else {
					b, req, e := snapshotCommand(c, schema)
					if e != nil {
						t.Fatal(e)
					}
					fp, e := commandFingerprint(b)
					if e != nil {
						t.Fatal(e)
					}
					p := &compoundPlan{}
					for _, r := range req {
						if e = p.add(t.Context(), store.db, "command", 0, 1, r, nil); e != nil {
							t.Fatal(e)
						}
						if e = store.validatePlainTable(t.Context(), r.Options.TableParts[1]); e != nil {
							t.Fatal(e)
						}
					}
					go func(c Command, b []byte, fp [32]byte, p *compoundPlan) {
						<-start
						r, e := store.apply(t.Context(), c.ID, b, fp, p)
						out <- outcome{r, e}
					}(c, b, fp, p)
				}
			}
			close(start)
			applied, replay, conflict := 0, 0, 0
			for j := 0; j < 2; j++ {
				o := <-out
				if errors.Is(o.e, ErrCommandIdentityConflict) {
					conflict++
				} else if o.e != nil {
					t.Fatal(o.e)
				} else if o.r.Applied {
					applied++
				} else {
					replay++
				}
			}
			if applied != 1 {
				t.Fatal("double execution", applied)
			}
			if mode == "reservation-different" {
				if conflict != 1 || replay != 0 {
					t.Fatal("race mismatch reused")
				}
			} else if replay != 1 || conflict != 0 {
				t.Fatal("replay missing")
			}
			assertCounts(t, n, 7, 1)
		})
	}
	t.Run("cancel-reservation-and-same-input-recovery", func(t *testing.T) {
		c := command(549)
		tx, e := raw.Begin()
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		b, _, e := snapshotCommand(c, schema)
		if e != nil {
			t.Fatal(e)
		}
		fp, _ := commandFingerprint(b)
		if _, e = tx.Exec(store.sql("INSERT INTO "+store.table+" (scope_hash,operation_id,format_version,canonical_scope,command_input,fingerprint,status,result) VALUES (?,?,1,?,?,?,0,?)"), store.scopeHash[:], []byte(c.ID), store.scope, b, fp[:], []byte{}); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
		defer cancel()
		if _, e = store.Run(ctx, c); e == nil {
			t.Fatal("cancel accepted")
		}
		if e = tx.Rollback(); e != nil {
			t.Fatal(e)
		}
		assertCounts(t, 549, 7, 0)
		run(t, c, true)
		assertCounts(t, 549, 7, 1)
	})
	t.Run("missing-hook-catalog-and-invalid-target", func(t *testing.T) {
		c := command(551)
		if _, e := raw.Exec("ALTER TABLE gq10_outbox RENAME TO gq10_hidden_outbox"); e != nil {
			t.Fatal(e)
		}
		_, runErr := store.Run(t.Context(), c)
		if _, e := raw.Exec("ALTER TABLE gq10_hidden_outbox RENAME TO gq10_outbox"); e != nil {
			t.Fatal(e)
		}
		if runErr == nil {
			t.Fatal("missing hook table accepted")
		}
		assertCounts(t, 551, 7, 0)
		for _, table := range []string{commandTable, "unknown_hook", schema + ".gq10_outbox"} {
			c := command(551)
			c.Hooks[1].Table = table
			if _, e := store.Run(t.Context(), c); e == nil {
				t.Fatal("invalid target accepted")
			}
			assertCounts(t, 551, 7, 0)
		}
	})

	t.Run("changed-hook-policy-and-external-owner", func(t *testing.T) {
		restricted, e := NewPolicySet(TablePolicy{Table: schema + ".gq10_items", TenantColumn: "tenant_id"}, TablePolicy{Table: schema + ".gq10_audit", TenantColumn: "tenant_id", ForbiddenColumns: []string{"score"}}, TablePolicy{Table: schema + ".gq10_outbox", TenantColumn: "tenant_id"})
		if e != nil {
			t.Fatal(e)
		}
		denied, e := NewCommandStore(t.Context(), makeDB(7).WithOptions(WithPolicySet(restricted)), "gq10-fixture")
		if e != nil {
			t.Fatal(e)
		}
		if _, e = denied.Run(t.Context(), command(501)); !errors.Is(e, ErrBlockedOperation) {
			t.Fatal("replay bypassed hook policy", e)
		}
		assertCounts(t, 501, 7, 1)
		tx, e := raw.Begin()
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		for _, d := range []*DB{makeDB(7).WrapTx(tx), NewTxDB(tx, db.Dialect()), makeDB(7).WrapExecutor(raw), NewDBWithExecutor(raw, db.Dialect())} {
			if _, e = NewCommandStore(t.Context(), d, "gq10-fixture"); !errors.Is(e, ErrCommandOwnership) {
				t.Fatal("external owner", e)
			}
		}
		assertCounts(t, 550, 7, 0)
	})
}
