package tests

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

func TestValidatedBindingDatabaseAndExternalTx(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			root := openTestDB(t, config.name, dsn, explicit)
			defer root.Close()
			const table = "gq_binding_rows"
			if _, err := root.SQLDB().Exec("CREATE TABLE " + table + " (tenant_id INTEGER NOT NULL, id INTEGER NOT NULL, score INTEGER NOT NULL, active BOOLEAN NOT NULL, PRIMARY KEY(tenant_id,id))"); err != nil {
				t.Fatal(err)
			}
			defer root.SQLDB().Exec("DROP TABLE " + table)
			typ := "INT"
			if config.name == orm.Postgres {
				typ = "integer"
			}
			cols := []query.WriteKeyColumn{{Name: "tenant_id", DBType: typ, Bits: 32}, {Name: "id", DBType: typ, Bits: 32}, {Name: "score", DBType: typ, Bits: 32}, {Name: "active", DBType: "boolean"}}
			schema, err := orm.NewApplicationSchema(orm.ApplicationSchemaInput{Database: "binding-fixture", Dialect: config.name, Tables: []orm.ApplicationTable{{Table: table, PlainTable: true, Columns: cols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols[:2]}}}}})
			if err != nil {
				t.Fatal(err)
			}
			policies, err := orm.NewPolicySet(orm.TablePolicy{Table: table, TenantColumn: "tenant_id"})
			if err != nil {
				t.Fatal(err)
			}
			tenant, err := orm.NewApplicationTenantContext(orm.ExecutionContextInput{TenantPresent: true, CurrentTenant: 1})
			if err != nil {
				t.Fatal(err)
			}
			db := root.WithOptions(orm.WithPolicySet(policies), orm.WithExecutionContext(tenant), orm.WithTenantPolicy("binding-fixture", schema, true))
			bc, err := query.NewBindingContext(query.BindingContextInput{Key: bytes.Repeat([]byte{0xc5}, 32), Scope: "fictional-integration", Generation: "1", Target: "binding-fixture", Dialect: config.name})
			if err != nil {
				t.Fatal(err)
			}
			// Supply the application's current DB settings independently of the Query.
			current := query.BindingCurrent{Settings: db.Settings(), BindingContext: bc}
			expiry := func() time.Time { return time.Now().Add(time.Minute) }
			q := db.Table(table)
			data := map[string]any{"id": 1, "score": int32(10), "active": true}
			h, _, err := q.ValidateInsert(nil, current, expiry(), data)
			if err != nil {
				t.Fatal(err)
			}
			r, err := q.ExecuteValidatedInsert(nil, current, h, data)
			if err != nil {
				t.Fatal(err)
			}
			if n, e := r.RowsAffected(); e != nil || n != 1 {
				t.Fatal(n, e)
			}
			batch := []map[string]any{{"id": 2, "score": int32(20), "active": false}, {"id": 3, "score": int32(30), "active": true}}
			h, _, err = q.ValidateInsertBatch(t.Context(), current, expiry(), batch)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = q.ExecuteValidatedInsertBatch(t.Context(), current, h, batch); err != nil {
				t.Fatal(err)
			}
			q = db.Table(table).Select("id", "score", "active").OrderBy("id", "asc").Limit(10)
			h, _, err = q.ValidateSelect(t.Context(), current, expiry())
			if err != nil {
				t.Fatal(err)
			}
			var rows []struct {
				ID, Score int
				Active    bool
			}
			if err = q.ExecuteValidatedSelect(t.Context(), current, h, &rows); err != nil || len(rows) != 3 || !rows[0].Active || rows[1].Active {
				t.Fatal(err, rows)
			}
			q = db.Table(table)
			h, _, err = q.ValidateCount(nil, current, expiry())
			if err != nil {
				t.Fatal(err)
			}
			if n, e := q.ExecuteValidatedCount(nil, current, h); e != nil || n != 3 {
				t.Fatal(n, e)
			}
			q = db.Table(table).Where("id", 1)
			update := map[string]any{"score": int32(11)}
			h, _, err = q.ValidateUpdate(nil, current, expiry(), update)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = q.ExecuteValidatedUpdate(nil, current, h, update); err != nil {
				t.Fatal(err)
			}
			q = db.Table(table).Where("id", 3)
			h, _, err = q.ValidateDelete(t.Context(), current, expiry())
			if err != nil {
				t.Fatal(err)
			}
			if _, err = q.ExecuteValidatedDelete(t.Context(), current, h); err != nil {
				t.Fatal(err)
			}
			tx, err := root.SQLDB().BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			tq := db.WrapTx(tx).Table(table).Where("id", 2)
			h, _, err = tq.ValidateUpdate(t.Context(), current, expiry(), update)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tq.ExecuteValidatedUpdate(t.Context(), current, h, update); err != nil {
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			q = db.Table(table).Select("score").Where("id", 2).Limit(1)
			h, _, err = q.ValidateSelect(nil, current, expiry())
			if err != nil {
				t.Fatal(err)
			}
			var after []struct{ Score int }
			if err = q.ExecuteValidatedSelect(nil, current, h, &after); err != nil || len(after) != 1 || after[0].Score != 20 {
				t.Fatal(err, after)
			}
		})
	}
}
