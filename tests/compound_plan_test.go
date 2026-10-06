package tests

import (
	"context"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm"
	"testing"
)

type compoundRecipeRow struct {
	ID     int  `db:"id"`
	Active bool `db:"active"`
}

func TestCompoundRecipeDatabaseSemantics(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			root := openTestDB(t, config.name, dsn, explicit)
			defer root.Close()
			ctx := context.Background()
			const table = "gq_compound_recipes"
			if _, err := root.SQLDB().Exec("CREATE TABLE gq_compound_recipes (id INTEGER PRIMARY KEY, active BOOLEAN NOT NULL)"); err != nil {
				t.Fatal(err)
			}
			defer root.SQLDB().Exec("DROP TABLE gq_compound_recipes")
			executor := &genericDatabaseExecutor{Executor: root.SQLDB()}
			db := root
			row := func(id int, active bool) map[string]any { return map[string]any{"id": id, "active": active} }
			apply := func(ctx context.Context, tx orm.Tx) (int, error) {
				_, err := orm.Insert(ctx, tx.DB, row(1, true), orm.Table(table))
				return 1, err
			}
			got, err := orm.RunTransactionWithHooks(ctx, db, orm.TransactionWithHooksSpec[int]{Apply: apply, Hooks: []orm.TransactionHook{
				orm.InsertHook("one", row(2, false), orm.Table(table)), orm.InsertManyHook("many", []map[string]any{row(3, true), row(4, false)}, orm.Table(table)),
			}})
			if err != nil || got != 1 {
				t.Fatal(got, err)
			}
			rows, err := orm.SelectAllBy[compoundRecipeRow](ctx, db, db.Table(table).Select("id", "active").OrderBy("id", "asc"))
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 4 || !rows[0].Active || rows[1].Active || !rows[2].Active || rows[3].Active {
				t.Fatal(rows)
			}
			stop := errors.New("rollback recipe")
			_, err = orm.RunTransactionWithHooks(ctx, db, orm.TransactionWithHooksSpec[int]{Apply: func(ctx context.Context, tx orm.Tx) (int, error) {
				_, e := orm.Insert(ctx, tx.DB, row(5, true), orm.Table(table))
				return 5, e
			}, Hooks: []orm.TransactionHook{
				orm.InsertHook("before-failure", row(6, false), orm.Table(table)), orm.NewTransactionHook("failure", func(context.Context, orm.Tx) error { return stop }),
			}})
			if !errors.Is(err, stop) {
				t.Fatal(err)
			}
			n, err := db.Table(table).WhereIn("id", []int{5, 6}).Count()
			if err != nil || n != 0 {
				t.Fatal(n, err)
			}
			applies := 0
			lookup := func(ctx context.Context, db *orm.DB) (compoundRecipeRow, error) {
				return orm.SelectOneBy[compoundRecipeRow](ctx, db, db.Table(table).Select("id", "active").Where("id", 7))
			}
			spec := orm.IdempotentCommandSpec[compoundRecipeRow]{LookupExisting: lookup, Apply: func(ctx context.Context, tx orm.Tx) (compoundRecipeRow, error) {
				applies++
				if _, e := orm.Insert(ctx, tx.DB, row(7, true), orm.Table(table)); e != nil {
					return compoundRecipeRow{}, e
				}
				return lookup(ctx, tx.DB)
			}}
			first, err := orm.RunIdempotentCommand(ctx, db, spec)
			if err != nil || !first.Applied || first.Value.ID != 7 || !first.Value.Active {
				t.Fatal(first, err)
			}
			second, err := orm.RunIdempotentCommand(ctx, db, spec)
			if err != nil || second.Applied || applies != 1 || second.Value != first.Value {
				t.Fatal(second, err, applies)
			}
			db = root.WrapExecutor(executor)
			before := executor.calls.Load()
			inserted, created, err := orm.InsertOnceReturning[compoundRecipeRow](ctx, db, row(9, true), orm.Table(table), orm.ConflictColumns("id"))
			if config.name == orm.MySQL {
				if err == nil || executor.calls.Load() != before {
					t.Fatal("MySQL RETURNING must refuse before dispatch", err)
				}
				return
			}
			if err != nil || !created || inserted.ID != 9 || !inserted.Active {
				t.Fatal(inserted, created, err)
			}
			existing, created, err := orm.InsertOnceReturning[compoundRecipeRow](ctx, db, row(9, false), orm.Table(table), orm.ConflictColumns("id"))
			if err != nil || created || existing != inserted {
				t.Fatal(existing, created, err)
			}
		})
	}
}

func TestCompoundGeneratedIDCorrespondence(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			root := openTestDB(t, config.name, dsn, explicit)
			defer root.Close()
			ctx := context.Background()
			idType := "BIGINT AUTO_INCREMENT PRIMARY KEY"
			if config.name == orm.Postgres {
				idType = "BIGSERIAL PRIMARY KEY"
			}
			for _, statement := range []string{"CREATE TABLE gq_compound_ids (id " + idType + ", input_key VARCHAR(30) NOT NULL UNIQUE)", "CREATE TABLE gq_compound_refs (input_key VARCHAR(30) NOT NULL, child_id BIGINT NOT NULL)"} {
				if _, err := root.SQLDB().Exec(statement); err != nil {
					t.Fatal(err)
				}
			}
			defer root.SQLDB().Exec("DROP TABLE gq_compound_refs")
			defer root.SQLDB().Exec("DROP TABLE gq_compound_ids")
			tx, err := root.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			// Force non-unit allocation on the actual connection used by the ORM.
			if config.name == orm.MySQL {
				if _, err := tx.Tx.Exec("SET SESSION auto_increment_increment=7"); err != nil {
					t.Fatal(err)
				}
				defer tx.Tx.Exec("SET SESSION auto_increment_increment=1")
			} else {
				if _, err := tx.Tx.Exec("ALTER SEQUENCE gq_compound_ids_id_seq INCREMENT BY 7"); err != nil {
					t.Fatal(err)
				}
			}
			executor := &genericDatabaseExecutor{Executor: tx.Tx.Tx}
			db := tx.DB.WrapExecutor(executor)
			children := []map[string]any{{"input_key": "first"}, {"input_key": "second"}}
			result, err := orm.ReplaceNestedCollection(ctx, db, orm.NestedCollectionReplace[struct{}, map[string]any, map[string]any]{
				SkipParent: true, Children: children, ChildOpts: []orm.WriteOpt{orm.Table("gq_compound_ids"), orm.ExpectAffected(2)},
				AssignChildID: func(i int, id int64) { children[i]["assigned_id"] = id },
				Grandchildren: func(i int, child map[string]any, id int64) ([]map[string]any, error) {
					if child["assigned_id"] != id {
						return nil, fmt.Errorf("assignment mismatch")
					}
					return []map[string]any{{"input_key": child["input_key"], "child_id": id}}, nil
				},
				GrandchildOpts: []orm.WriteOpt{orm.Table("gq_compound_refs")},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.ChildIDs) != 2 || result.ChildIDs[1]-result.ChildIDs[0] != 7 || executor.calls.Load() != 3 {
				t.Fatal(result, executor.calls.Load())
			}
			rows, err := tx.Tx.Query("SELECT c.input_key,c.id,r.child_id FROM gq_compound_ids c JOIN gq_compound_refs r ON c.input_key=r.input_key ORDER BY c.input_key")
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			count := 0
			for rows.Next() {
				var key string
				var actual, ref int64
				if err := rows.Scan(&key, &actual, &ref); err != nil {
					t.Fatal(err)
				}
				if actual != ref {
					t.Fatal(key, actual, ref)
				}
				count++
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatal(count)
			}
		})
	}
}
