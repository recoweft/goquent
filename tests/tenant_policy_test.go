package tests

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

func TestTenantPolicyDatabaseSemantics(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			root := openTestDB(t, config.name, dsn, explicit)
			defer root.Close()
			var version string
			if err := root.SQLDB().QueryRow("SELECT version()").Scan(&version); err != nil {
				t.Fatal(err)
			}
			t.Logf("server: %s", version)
			const table = "gq_tenant_rows"
			if _, err := root.SQLDB().Exec("CREATE TABLE " + table + " (tenant_id INTEGER NOT NULL, id INTEGER NOT NULL, score INTEGER NOT NULL, deleted INTEGER NULL, secret INTEGER NULL, fixed INTEGER NULL, forbidden INTEGER NULL, PRIMARY KEY(tenant_id,id))"); err != nil {
				t.Fatal(err)
			}
			defer root.SQLDB().Exec("DROP TABLE " + table)
			if _, err := root.SQLDB().Exec("INSERT INTO " + table + " (tenant_id,id,score,deleted) VALUES (1,1,10,NULL),(1,2,20,NULL),(1,3,30,1),(2,1,40,NULL),(2,4,50,NULL)"); err != nil {
				t.Fatal(err)
			}
			typ := "INT"
			if config.name == orm.Postgres {
				typ = "integer"
			}
			var cols []query.WriteKeyColumn
			for _, name := range []string{"tenant_id", "id", "score", "deleted", "secret", "fixed", "forbidden"} {
				cols = append(cols, query.WriteKeyColumn{Name: name, DBType: typ, Bits: 32, Nullable: name == "deleted" || name == "secret" || name == "fixed" || name == "forbidden"})
			}
			schemaInput := orm.ApplicationSchemaInput{Database: "tenant-fixture", Dialect: config.name, Tables: []orm.ApplicationTable{{Table: table, PlainTable: true, Columns: cols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Name: "pk", Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols[:2]}}}}}
			schema, err := orm.NewApplicationSchema(schemaInput)
			if err != nil {
				t.Fatal(err)
			}
			policy := orm.TablePolicy{Table: table, TenantColumn: "tenant_id", SoftDeleteColumn: "deleted", PIIColumns: []string{"secret"}, ImmutableColumns: []string{"fixed"}, ForbiddenColumns: []string{"forbidden"}}
			policies, err := orm.NewPolicySet(policy)
			if err != nil {
				t.Fatal(err)
			}
			derive := func(tenant int) *orm.DB {
				c, err := orm.NewApplicationTenantContext(orm.ExecutionContextInput{TenantPresent: true, CurrentTenant: tenant})
				if err != nil {
					t.Fatal(err)
				}
				return root.WithOptions(orm.WithPolicySet(policies), orm.WithExecutionContext(c), orm.WithTenantPolicy("tenant-fixture", schema, true))
			}
			db := derive(1)
			type row struct {
				ID    int `db:"id"`
				Score int `db:"score"`
			}
			var got []row
			q := db.Table(table).Select("id", "score").Where("id", 1).OrWhere("id", 2).OrWhere("id", 3).OrWhere("id", 4).OrderBy("id", "asc")
			if err := q.Get(&got); err != nil {
				t.Fatal(err)
			}
			if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
				t.Fatalf("tenant or soft-delete escaped: %+v", got)
			}
			n, err := db.Table(table).WithDeleted().Count()
			if err != nil || n != 3 {
				t.Fatalf("WithDeleted count=%d err=%v", n, err)
			}
			n, err = db.Table(table).OnlyDeleted().Count()
			if err != nil || n != 1 {
				t.Fatalf("OnlyDeleted count=%d err=%v", n, err)
			}
			// LEFT JOIN retains unmatched base rows: candidate b.id=1 only, a.id=2 remains.
			noSoft := policy
			noSoft.SoftDeleteColumn = ""
			p2, _ := orm.NewPolicySet(noSoft)
			joinDB := db.WithOptions(orm.WithPolicySet(p2))
			got = nil
			jq := joinDB.Table(table+" as a").Select("a.id").LeftJoinQuery(table+" as b", func(j *query.JoinClause) { j.On("a.id", "=", "b.score").Where("b.tenant_id", "=", 1) }).Where("a.id", "<", 3)
			if err := jq.Get(&got); err != nil {
				t.Fatal(err)
			}
			if len(got) != 2 {
				t.Fatalf("LEFT became INNER: %+v", got)
			}
			got = nil
			if err := joinDB.Table(table+" as a").Select("a.id").Join(table+" as b", "a.id", "=", "b.id").Where("b.tenant_id", 1).Get(&got); err != nil {
				t.Fatal(err)
			}
			if len(got) != 3 {
				t.Fatalf("self join aliases: %+v", got)
			}
			input := []map[string]any{{"id": 5, "score": 50}, {"id": 6, "score": 60}}
			if _, err := db.Table(table).InsertBatch(input); err != nil {
				t.Fatal(err)
			}
			if _, ok := input[0]["tenant_id"]; ok {
				t.Fatal("input mutated")
			}
			if _, err := db.Table(table).InsertBatch([]map[string]any{{"tenant_id": 1, "id": 7, "score": 70}, {"tenant_id": 2, "id": 8, "score": 80}}); err == nil {
				t.Fatal("mixed tenant accepted")
			}
			if _, err := db.Table(table).Where("id", 5).Update(map[string]any{"score": 55}); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Table(table).Where("id", 5).Update(map[string]any{"tenant_id": 2}); err == nil {
				t.Fatal("tenant assignment accepted")
			}
			if _, err := db.Table(table).Upsert([]map[string]any{{"id": 5, "score": 56}}, []string{"tenant_id", "id"}, []string{"score"}); err != nil {
				t.Fatal(err)
			}
			var updated row
			if err := db.Table(table).Select("id", "score").Where("id", 5).First(&updated); err != nil || updated.Score != 56 {
				t.Fatalf("upsert readback %+v %v", updated, err)
			}
			if config.name == orm.Postgres {
				id, err := db.Table(table).InsertGetId(map[string]any{"id": 9, "score": 90})
				if err != nil || id != 9 {
					t.Fatalf("RETURNING result %d %v", id, err)
				}
			}
			// Additional global unique key changes MySQL's possible conflict routes.
			if _, err := root.SQLDB().Exec("CREATE UNIQUE INDEX gq_tenant_score ON " + table + " (score)"); err != nil {
				t.Fatal(err)
			}
			schemaInput.Tables[0].Constraints = append(schemaInput.Tables[0].Constraints, query.WriteKeyConstraint{Kind: "unique", AllRows: true, Valid: true, NotDeferrable: true, Columns: []query.WriteKeyColumn{cols[2]}})
			extra, _ := orm.NewApplicationSchema(schemaInput)
			extraDB := db.WithOptions(orm.WithTenantPolicy("tenant-fixture", extra, true))
			_, err = extraDB.Table(table).Upsert([]map[string]any{{"id": 5, "score": 57}}, []string{"tenant_id", "id"}, []string{"score"})
			if (err != nil) != (config.name == orm.MySQL) {
				t.Fatalf("extra unique dialect=%s err=%v", config.name, err)
			}
			check := func(d *orm.DB, want int) error {
				var r row
				err := d.Table(table).Select("id", "score").Where("id", 1).WithContext(context.Background()).First(&r)
				if err != nil {
					return err
				}
				if (want == 1 && r.Score != 10) || (want == 2 && r.Score != 40) {
					return fmt.Errorf("tenant context mixed: %d", r.Score)
				}
				return nil
			}
			var wg sync.WaitGroup
			for tenant := 1; tenant <= 2; tenant++ {
				d := derive(tenant)
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 6; i++ {
						if err := check(d.Clone(), tenant); err != nil {
							t.Error(err)
						}
					}
				}()
			}
			wg.Wait()
			if err := db.TransactionContext(context.Background(), func(tx orm.Tx) error {
				if err := check(tx.DB, 1); err != nil {
					return err
				}
				ext := &settingsExternalExecutor{Tx: tx.Tx.Tx}
				if err := check(tx.WrapExecutor(ext), 1); err != nil {
					return err
				}
				if ext.calls.Load() == 0 {
					return fmt.Errorf("custom executor not used")
				}
				before := ext.calls.Load()
				var rows []row
				err := tx.WrapExecutor(ext).Table(table).Select("secret").WithContext(context.Background()).Get(&rows)
				if err == nil || ext.calls.Load() != before {
					return fmt.Errorf("rejected projection reached executor")
				}
				return check(orm.NewTxDB(tx.Tx.Tx, db.Dialect(), orm.WithSettings(db.Settings())), 1)
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
