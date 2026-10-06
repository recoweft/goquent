package tests

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

type genericDatabaseExecutor struct {
	orm.Executor
	calls atomic.Int64
}

func (e *genericDatabaseExecutor) Exec(q string, a ...any) (sql.Result, error) {
	e.calls.Add(1)
	return e.Executor.Exec(q, a...)
}
func (e *genericDatabaseExecutor) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	e.calls.Add(1)
	return e.Executor.ExecContext(c, q, a...)
}
func (e *genericDatabaseExecutor) Query(q string, a ...any) (*sql.Rows, error) {
	e.calls.Add(1)
	return e.Executor.Query(q, a...)
}
func (e *genericDatabaseExecutor) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	e.calls.Add(1)
	return e.Executor.QueryContext(c, q, a...)
}
func (e *genericDatabaseExecutor) QueryRow(q string, a ...any) *sql.Row {
	e.calls.Add(1)
	return e.Executor.QueryRow(q, a...)
}
func (e *genericDatabaseExecutor) QueryRowContext(c context.Context, q string, a ...any) *sql.Row {
	e.calls.Add(1)
	return e.Executor.QueryRowContext(c, q, a...)
}

func TestGenericPlannedDatabaseSemantics(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			root := openTestDB(t, config.name, dsn, explicit)
			defer root.Close()
			ctx := context.Background()
			const table = "gq_generic_plan_rows"
			if _, err := root.SQLDB().Exec("CREATE TABLE " + table + " (tenant_id INTEGER NOT NULL, id INTEGER NOT NULL, score INTEGER NOT NULL, active BOOLEAN NOT NULL, deleted INTEGER NULL, secret INTEGER NULL, fixed INTEGER NULL, forbidden INTEGER NULL, PRIMARY KEY (tenant_id,id))"); err != nil {
				t.Fatal(err)
			}
			defer root.SQLDB().Exec("DROP TABLE " + table)
			if _, err := root.SQLDB().Exec("INSERT INTO " + table + " (tenant_id,id,score,active,deleted) VALUES (2,1,90,true,NULL),(1,9,90,true,1)"); err != nil {
				t.Fatal(err)
			}
			typ := "INT"
			if config.name == orm.Postgres {
				typ = "integer"
			}
			var cols []query.WriteKeyColumn
			for _, n := range []string{"tenant_id", "id", "score", "deleted", "secret", "fixed", "forbidden"} {
				cols = append(cols, query.WriteKeyColumn{Name: n, DBType: typ, Bits: 32, Nullable: n == "deleted" || n == "secret" || n == "fixed" || n == "forbidden"})
			}
			cols = append(cols, query.WriteKeyColumn{Name: "active", DBType: "boolean"})
			schema, err := orm.NewApplicationSchema(orm.ApplicationSchemaInput{Database: "generic-fixture", Dialect: config.name, Tables: []orm.ApplicationTable{{Table: table, Columns: cols, PlainTable: true, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols[:2]}}}}})
			if err != nil {
				t.Fatal(err)
			}
			policies, err := orm.NewPolicySet(orm.TablePolicy{Table: table, TenantColumn: "tenant_id", SoftDeleteColumn: "deleted", PIIColumns: []string{"secret"}, ImmutableColumns: []string{"fixed"}, ForbiddenColumns: []string{"forbidden"}})
			if err != nil {
				t.Fatal(err)
			}
			tenant, err := orm.NewApplicationTenantContext(orm.ExecutionContextInput{TenantPresent: true, CurrentTenant: 1})
			if err != nil {
				t.Fatal(err)
			}
			parent := root.WithOptions(orm.WithPolicySet(policies), orm.WithExecutionContext(tenant), orm.WithTenantPolicy("generic-fixture", schema, true))
			spy := &genericDatabaseExecutor{Executor: root.SQLDB()}
			db := parent.WrapExecutor(spy)
			row := map[string]any{"id": 1, "score": 10, "active": true}
			p, err := orm.PlanInsert(ctx, db, row, orm.Table(table))
			if err != nil {
				t.Fatal(err)
			}
			if err = orm.EnsurePlanExecutable(p); err != nil {
				t.Fatal(err)
			}
			if spy.calls.Load() != 0 {
				t.Fatal("plan executed")
			}
			result, err := orm.Insert(ctx, db, row, orm.Table(table), orm.ExpectAffected(1))
			if err != nil {
				t.Fatal(err)
			}
			if n, e := result.RowsAffected(); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			if spy.calls.Load() != 1 {
				t.Fatal("insert dispatch", spy.calls.Load())
			}
			if row["tenant_id"] != nil {
				t.Fatal("input mutated")
			}
			bad := map[string]any{"id": 2, "tenant_id": 2, "score": 20, "active": false}
			if _, err = orm.InsertMany(ctx, db, []map[string]any{{"id": 2, "tenant_id": 1, "score": 20, "active": false}, bad}, orm.Table(table)); !errors.Is(err, orm.ErrBlockedOperation) {
				t.Fatal(err)
			}
			if spy.calls.Load() != 1 {
				t.Fatal("mixed tenant dispatched")
			}
			if _, err = orm.Update(ctx, db, map[string]any{"id": 1, "score": 11}, orm.Table(table), orm.PK("id"), orm.WherePK(), orm.ExpectAffected(1)); err != nil {
				t.Fatal(err)
			}
			// Soft deletion is included in the actual generic statement.
			result, err = orm.Update(ctx, db, map[string]any{"id": 9, "score": 12}, orm.Table(table), orm.PK("id"), orm.WherePK())
			if err != nil {
				t.Fatal(err)
			}
			if n, _ := result.RowsAffected(); n != 0 {
				t.Fatal(n)
			}
			upsert := []orm.WriteOpt{orm.Table(table), orm.ConflictColumns("tenant_id", "id"), orm.UpdateColumns("score", "active")}
			if _, err = orm.Upsert(ctx, db, map[string]any{"id": 1, "score": 13, "active": false}, upsert...); err != nil {
				t.Fatal(err)
			}
			type value struct {
				ID     int  `db:"id"`
				Score  int  `db:"score"`
				Active bool `db:"active"`
			}
			got, err := orm.SelectOneBy[value](ctx, db, root.Table(table).Select("id", "score", "active").Where("id", 1))
			if err != nil || got.Score != 13 || got.Active {
				t.Fatal(got, err)
			}
			many := []map[string]any{{"id": 2, "score": 20, "active": true}, {"id": 3, "score": 30, "active": false}}
			if _, err = orm.InsertMany(ctx, db, many, orm.Table(table), orm.ExpectAffected(2)); err != nil {
				t.Fatal(err)
			}
			if _, err = orm.UpsertMany(ctx, db, many, upsert...); err != nil {
				t.Fatal(err)
			}
			if config.name == orm.Postgres {
				got, err = orm.UpdateByReturning[value](ctx, db, root.Table(table).Where("id", 1), map[string]any{"score": 14})
				if err != nil || got.Score != 14 {
					t.Fatal(got, err)
				}
				added, e := orm.InsertReturning[value](ctx, db, map[string]any{"id": 4, "score": 40, "active": true}, orm.Table(table))
				if e != nil || added.ID != 4 || !added.Active {
					t.Fatal(added, e)
				}
				updated, e := orm.UpdateReturning[value](ctx, db, map[string]any{"id": 4, "score": 41}, orm.Table(table), orm.PK("id"), orm.WherePK())
				if e != nil || updated.Score != 41 {
					t.Fatal(updated, e)
				}
				merged, e := orm.UpsertReturning[value](ctx, db, map[string]any{"id": 4, "score": 42, "active": false}, upsert...)
				if e != nil || merged.Score != 42 || merged.Active {
					t.Fatal(merged, e)
				}
				returned, e := orm.InsertManyReturning[value](ctx, db, []map[string]any{{"id": 5, "score": 50, "active": true}, {"id": 6, "score": 60, "active": false}}, orm.Table(table))
				if e != nil || len(returned) != 2 {
					t.Fatal(returned, e)
				}
				returned, e = orm.UpsertManyReturning[value](ctx, db, many, upsert...)
				if e != nil || len(returned) != 2 {
					t.Fatal(returned, e)
				}
			} else {
				before := spy.calls.Load()
				if _, e := orm.InsertReturning[value](ctx, db, map[string]any{"id": 4, "score": 40, "active": true}, orm.Table(table)); e == nil || spy.calls.Load() != before {
					t.Fatal(e)
				}
			}
			before := spy.calls.Load()
			if _, err = orm.UpdateReturning[value](ctx, db, map[string]any{"id": 1, "secret": 99}, orm.Table(table), orm.PK("id"), orm.WherePK()); !errors.Is(err, orm.ErrBlockedOperation) {
				t.Fatal(err)
			}
			if spy.calls.Load() != before {
				t.Fatal("PII dispatched")
			}
			var other int
			if err = root.SQLDB().QueryRow("SELECT score FROM " + table + " WHERE tenant_id=2 AND id=1").Scan(&other); err != nil || other != 90 {
				t.Fatal(other, err)
			}
			// A caller-owned transaction retains ownership, settings and rollback behavior.
			external, err := root.SQLDB().BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			txDB := parent.WrapTx(external)
			if _, err = orm.Insert(ctx, txDB, map[string]any{"id": 7, "score": 70, "active": true}, orm.Table(table)); err != nil {
				external.Rollback()
				t.Fatal(err)
			}
			if err = external.Rollback(); err != nil {
				t.Fatal(err)
			}
			var count int
			if err = root.SQLDB().QueryRow("SELECT COUNT(*) FROM " + table + " WHERE id=7").Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
			var version string
			if err = root.SQLDB().QueryRow("SELECT version()").Scan(&version); err != nil {
				t.Fatal(err)
			}
			t.Logf("server=%s; generic executor calls=%d", version, spy.calls.Load())
		})
	}
}
