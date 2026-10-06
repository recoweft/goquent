package tests

import (
	"context"
	"fmt"
	"github.com/recoweft/goquent/orm"
	"testing"
)

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
