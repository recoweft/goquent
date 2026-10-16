package orm

import (
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"math"
	"os"
	"sync"
	"testing"
)

func testAtomicIntegration(t *testing.T, db *DB, store *CommandStore, schema, dialect string) {
	raw := db.SQLDB()
	ctx := t.Context()
	table := schema + ".gq10_items"
	trusted := db.WithOptions(WithSettings(atomicSettings(t, dialect, table)))
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := raw.Exec(store.sql(q), args...); e != nil {
			t.Fatal(e)
		}
	}
	exec("INSERT INTO " + table + " (tenant_id,id,score,version) VALUES (7,900,1,0),(7,901,1,0)")
	read := func(id int64) (score, version int64) {
		t.Helper()
		if e := raw.QueryRow(store.sql("SELECT score,version FROM "+table+" WHERE tenant_id=7 AND id=?"), id).Scan(&score, &version); e != nil {
			t.Fatal(e)
		}
		return
	}
	t.Run("concurrent-cas", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, e := trusted.Table(table).Where("id", int64(900)).OptimisticVersion("version", 0).Update(map[string]any{"score": int64(2)})
				results <- e
			}()
		}
		close(start)
		wg.Wait()
		close(results)
		success, conflict := 0, 0
		for e := range results {
			if e == nil {
				success++
			} else if errors.Is(e, ErrConflict) {
				conflict++
			} else {
				t.Fatal(e)
			}
		}
		if success != 1 || conflict != 1 {
			t.Fatal(success, conflict)
		}
		score, v := read(900)
		if score != 2 || v != 1 {
			t.Fatal(score, v)
		}
	})
	t.Run("owned-mismatch-rollback", func(t *testing.T) {
		_, e := trusted.Table(table).Where("id", ">=", int64(900)).AtomicWrite().ExpectAffected(1).Update(map[string]any{"score": int64(9)})
		var count RowsAffectedError
		if !errors.As(e, &count) || count.Actual != 2 {
			t.Fatal(e)
		}
		score, _ := read(900)
		other, _ := read(901)
		if score != 2 || other != 1 {
			t.Fatal("mismatch persisted")
		}
		_, e = trusted.Table(table).Where("id", int64(9999)).AtomicWrite().ExpectAffected(1).Update(map[string]any{"score": int64(9)})
		if !errors.As(e, &count) || count.Actual != 0 {
			t.Fatal(e)
		}
	})
	t.Run("unchanged-and-upsert-counts", func(t *testing.T) {
		unchanged := int64(1)
		if dialect == MySQL {
			unchanged = 0
		}
		res, e := trusted.Table(table).Where("id", int64(901)).AtomicWrite().ExpectAffected(unchanged).Update(map[string]any{"score": int64(1)})
		if e != nil {
			t.Fatal(e)
		}
		n, _ := res.RowsAffected()
		t.Log("unchanged UPDATE", dialect, n)
		for i, want := range []int64{1, 1, 1, 0} {
			if dialect == MySQL {
				want = []int64{1, 2, 0, 0}[i]
			}
			score := int64(3)
			if i > 0 {
				score = 4
			}
			opts := []WriteOpt{Table(table), ConflictColumns("tenant_id", "id"), UpdateColumns("score"), AtomicWrite(), ExpectAffected(want)}
			if i == 3 {
				opts = append(opts, ConflictDoNothing())
			}
			res, e := Upsert(ctx, trusted, map[string]any{"tenant_id": int64(7), "id": int64(902), "score": score}, opts...)
			if e != nil {
				t.Fatal(i, e)
			}
			n, _ := res.RowsAffected()
			t.Log("UPSERT case", i, dialect, n)
		}
	})
	t.Run("delete-and-version-bound", func(t *testing.T) {
		_, e := trusted.Table(table).Where("id", ">=", int64(900)).AtomicWrite().ExpectAffected(1).Delete()
		if !errors.Is(e, ErrRowsAffected) {
			t.Fatal(e)
		}
		read(901)
		exec("UPDATE "+table+" SET version=? WHERE id=901", int64(math.MaxInt64))
		_, e = trusted.Table(table).Where("id", int64(901)).OptimisticVersion("version", math.MaxInt64).Update(map[string]any{"score": int64(99)})
		if !errors.Is(e, ErrWriteOptions) {
			t.Fatal(e)
		}
		score, v := read(901)
		if score != 1 || v != math.MaxInt64 {
			t.Fatal("overflow wrote")
		}
		exec("UPDATE " + table + " SET version=0 WHERE id=901")
	})
	if dialect == MySQL {
		t.Run("client-found-rows", func(t *testing.T) {
			cfg, e := mysql.ParseDSN(os.Getenv("TEST_MYSQL_DSN"))
			if e != nil {
				t.Fatal(e)
			}
			cfg.ClientFoundRows = true
			found, e := OpenWithDriver(MySQL, cfg.FormatDSN())
			if e != nil {
				t.Fatal(e)
			}
			defer found.Close()
			found = found.WithOptions(WithSettings(atomicSettings(t, MySQL, table)))
			res, e := found.Table(table).Where("id", int64(901)).AtomicWrite().ExpectAffected(1).Update(map[string]any{"score": int64(1)})
			if e != nil {
				t.Fatal(e)
			}
			n, _ := res.RowsAffected()
			t.Log("clientFoundRows unchanged UPDATE", n)
			res, e = Upsert(ctx, found, map[string]any{"tenant_id": int64(7), "id": int64(902), "score": int64(4)}, Table(table), ConflictColumns("tenant_id", "id"), UpdateColumns("score"), AtomicWrite(), ExpectAffected(1))
			if e != nil {
				t.Fatal(e)
			}
			n, _ = res.RowsAffected()
			t.Log("clientFoundRows unchanged UPSERT", n)
		})
	}
	t.Run("returning", func(t *testing.T) {
		if dialect == MySQL {
			_, e := UpdateByReturningWithOptions[genericID](ctx, trusted, trusted.Table(table).Where("id", int64(900)), map[string]any{"score": int64(11)}, []WriteOpt{Returning("id"), AtomicWrite(), ExpectAffected(1)})
			if e == nil {
				t.Fatal("MySQL RETURNING accepted")
			}
			return
		}
		_, manyErr := InsertManyReturning[genericID](ctx, trusted, []map[string]any{{"id": int64(904), "score": int64(1)}, {"id": int64(905), "score": int64(1)}}, Table(table), Returning("id"), AtomicWrite(), ExpectAffected(1))
		if !errors.Is(manyErr, ErrRowsAffected) {
			t.Fatal(manyErr)
		}
		var remaining int
		if e := raw.QueryRow("SELECT count(*) FROM " + table + " WHERE id IN (904,905)").Scan(&remaining); e != nil || remaining != 0 {
			t.Fatal("Many RETURNING rollback", e)
		}
		_, e := UpdateByReturningWithOptions[genericID](ctx, trusted, trusted.Table(table).Where("id", ">=", int64(900)), map[string]any{"score": int64(11)}, []WriteOpt{Returning("id"), AtomicWrite(), ExpectAffected(1)})
		if !errors.Is(e, ErrRowsAffected) {
			t.Fatal(e)
		}
		score, _ := read(900)
		if score != 2 {
			t.Fatal("RETURNING mismatch persisted")
		}
		_, e = UpdateByReturningWithOptions[struct{ Score bool }](ctx, trusted, trusted.Table(table).Where("id", int64(900)), map[string]any{"score": int64(11)}, []WriteOpt{Returning("score"), AtomicWrite()})
		if e == nil {
			t.Fatal("scan conversion accepted")
		}
		score, _ = read(900)
		if score != 2 {
			t.Fatal("scan failure persisted")
		}
	})
	t.Run("command-v2-rollback-and-replay", func(t *testing.T) {
		one := int64(1)
		c := Command{ID: "00000000-0000-4000-8000-000000009000", Kind: "optimistic", Steps: []CommandStep{{Operation: "update", Table: "gq10_items", Keys: map[string]any{"id": int64(900)}, Values: map[string]any{"score": int64(6)}, Version: &CommandVersion{Column: "version", Expected: 1}}, {Operation: "update", Table: "gq10_items", Keys: map[string]any{"id": int64(9999)}, Values: map[string]any{"score": int64(8)}, ExpectAffected: &one}}}
		_, e := store.Run(ctx, c)
		if !errors.Is(e, ErrRowsAffected) {
			t.Fatal(e)
		}
		score, v := read(900)
		if score != 2 || v != 1 {
			t.Fatal("earlier step persisted")
		}
		c.Steps = c.Steps[:1]
		r, e := store.Run(ctx, c)
		if e != nil || !r.Applied {
			t.Fatal(e)
		}
		r, e = store.Run(ctx, c)
		if e != nil || r.Applied {
			t.Fatal("saved CAS replay", e)
		}
		c.Steps[0].Version.Expected = 2
		_, e = store.Run(ctx, c)
		if !errors.Is(e, ErrCommandIdentityConflict) {
			t.Fatal(e)
		}
		c.Steps[0].Version = nil
		_, e = store.Run(ctx, c)
		if !errors.Is(e, ErrCommandIdentityConflict) {
			t.Fatal("v1 downgrade", e)
		}
		input, requests, e := snapshotCommand(c, schema)
		if e != nil {
			t.Fatal(e)
		}
		fp, e := commandFingerprint(input)
		if e != nil {
			t.Fatal(e)
		}
		old := &pr1ReaderFixture{store}
		_, e = old.lookup(ctx, c.ID, input, fp, 1)
		if !errors.Is(e, ErrCommandCorrupt) {
			t.Fatal("PR1 reader v2", e)
		}
		var plan compoundPlan
		for _, request := range requests {
			if e = plan.add(ctx, store.db, "command", 0, 1, request, nil); e != nil {
				t.Fatal(e)
			}
		}
		_, e = old.apply(ctx, c.ID, input, fp, &plan)
		if !errors.Is(e, ErrCommandCorrupt) {
			t.Fatal("PR1 reservation race reader v2", e)
		}
		score, v = read(900)
		if score != 6 || v != 2 {
			t.Fatal("old reader reapplied")
		}
	})
	t.Run("external-owner-refusal", func(t *testing.T) {
		tx, e := raw.BeginTx(ctx, nil)
		if e != nil {
			t.Fatal(e)
		}
		defer tx.Rollback()
		_, e = trusted.WrapTx(tx).Table(table).Where("id", int64(900)).AtomicWrite().Update(map[string]any{"score": int64(15)})
		if !errors.Is(e, ErrWriteOwnership) {
			t.Fatal(e)
		}
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
		score, _ := read(900)
		if score != 6 {
			t.Fatal("external SQL executed")
		}
	})
	exec("DELETE FROM " + table + " WHERE id>=900")
	// Only this fixture's ledger entry is removed; the enclosing fixture owns it.
	exec("DELETE FROM "+store.table+" WHERE operation_id=?", []byte(fmt.Sprintf("00000000-0000-4000-8000-%012d", 9000)))
}
