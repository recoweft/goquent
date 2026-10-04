package tests

import (
	"context"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

// TestPredicateTreeDatabaseSemantics compares actual result sets and writes.
// It does not certify key cardinality, tenant authorization or risk verdicts.
func TestPredicateTreeDatabaseSemantics(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			db := openTestDB(t, config.name, dsn, explicit)
			defer db.Close()
			sqlDB := db.SQLDB()
			_, err := sqlDB.Exec("CREATE TABLE IF NOT EXISTS gq_predicate_rows (tenant_id INT NOT NULL, id INT NOT NULL, parent_id INT, score INT, note VARCHAR(20), PRIMARY KEY(tenant_id,id))")
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Exec("DROP TABLE gq_predicate_rows")
			if _, err = sqlDB.Exec("DELETE FROM gq_predicate_rows"); err != nil {
				t.Fatal(err)
			}
			if _, err = sqlDB.Exec("INSERT INTO gq_predicate_rows VALUES (1,1,1,10,NULL),(1,2,1,20,'x'),(2,3,2,30,'y'),(2,4,2,40,NULL)"); err != nil {
				t.Fatal(err)
			}
			cases := []struct {
				name  string
				build func(*query.Query) *query.Query
				want  []int
			}{
				{"C05_range", func(q *query.Query) *query.Query { return q.Where("u.id", ">", 2) }, []int{3, 4}},
				{"C05_single_in", func(q *query.Query) *query.Query { return q.WhereIn("u.id", []int{2}) }, []int{2}},
				{"C05_multi_in", func(q *query.Query) *query.Query { return q.WhereIn("u.id", []int{1, 3}) }, []int{1, 3}},
				{"C06_or_precedence", func(q *query.Query) *query.Query {
					return q.Where("u.id", 1).OrWhere("u.score", ">", 20).WhereNotNull("u.note")
				}, []int{1, 3}},
				{"C07_unbound_branch", func(q *query.Query) *query.Query {
					return q.WhereGroup(func(q *query.Query) { q.Where("u.tenant_id", 1).Where("u.id", 1) }).OrWhere("u.id", 3)
				}, []int{1, 3}},
				{"C08_negated_tenant", func(q *query.Query) *query.Query {
					return q.WhereNot(func(q *query.Query) { q.Where("u.tenant_id", 1) })
				}, []int{3, 4}},
				{"C09_literal_tenant", func(q *query.Query) *query.Query { return q.Where("u.tenant_id", 1) }, []int{1, 2}},
				{"C10_composite_equal", func(q *query.Query) *query.Query { return q.Where("u.tenant_id", 1).Where("u.id", 2) }, []int{2}},
				{"C10_composite_range", func(q *query.Query) *query.Query { return q.Where("u.tenant_id", ">", 0).Where("u.id", ">", 2) }, []int{3, 4}},
				{"C11_nested_not", func(q *query.Query) *query.Query {
					return q.WhereNot(func(q *query.Query) {
						q.Where("u.id", 1).OrWhereGroup(func(q *query.Query) { q.Where("u.score", ">=", 30).WhereNotNull("u.note") })
					})
				}, []int{2, 4}},
				{"C11_null", func(q *query.Query) *query.Query { return q.WhereNull("u.note") }, []int{1, 4}},
				{"C11_bound_null", func(q *query.Query) *query.Query { return q.Where("u.note", "=", nil) }, nil},
				{"C11_not_in_null", func(q *query.Query) *query.Query { return q.WhereNotIn("u.id", []any{1, nil}) }, nil},
				{"C11_raw", func(q *query.Query) *query.Query {
					return q.SafeWhereRaw("u.id = :a OR u.id = :b", map[string]any{"a": 2, "b": 4})
				}, []int{2, 4}},
				{"self_join_aliases", func(q *query.Query) *query.Query {
					return q.Join("gq_predicate_rows as v", "u.parent_id", "=", "v.id").WhereColumn("u.tenant_id", "=", "v.tenant_id")
				}, []int{1, 2}},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					p, err := tc.build(db.Table("gq_predicate_rows as u").Select("u.id")).OrderBy("u.id", "asc").Plan(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					rows, err := sqlDB.Query(p.SQL, p.Params...)
					if err != nil {
						t.Fatalf("%s: %v", p.SQL, err)
					}
					defer rows.Close()
					var got []int
					for rows.Next() {
						var id int
						if err := rows.Scan(&id); err != nil {
							t.Fatal(err)
						}
						got = append(got, id)
					}
					if err := rows.Err(); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, tc.want) {
						t.Fatalf("got %v want %v (%s)", got, tc.want, p.SQL)
					}
				})
			}
			t.Run("empty_in_legacy_error", func(t *testing.T) {
				p, err := db.Table("gq_predicate_rows").WhereIn("id", []any{}).Plan(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				rows, err := sqlDB.Query(p.SQL, p.Params...)
				if rows != nil {
					rows.Close()
				}
				if err == nil {
					t.Fatal("legacy empty IN unexpectedly accepted")
				}
				if p.WhereTree.OpaqueReason == "" {
					t.Fatal("empty IN marked parsed")
				}
			})
			t.Run("update_delete_nested", func(t *testing.T) {
				tx, err := sqlDB.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				q := db.Table("gq_predicate_rows").WhereNot(func(q *query.Query) {
					q.Where("id", 1).OrWhereGroup(func(q *query.Query) { q.Where("score", ">=", 30).WhereNotNull("note") })
				})
				p, err := q.PlanUpdate(context.Background(), map[string]any{"score": 99})
				if err != nil {
					t.Fatal(err)
				}
				result, err := tx.Exec(p.SQL, p.Params...)
				if err != nil {
					t.Fatal(err)
				}
				n, err := result.RowsAffected()
				if err != nil || n != 2 {
					t.Fatalf("update affected %d: %v", n, err)
				}
				del, err := db.Table("gq_predicate_rows").WhereGroup(func(q *query.Query) { q.Where("id", 2).OrWhere("id", 4) }).Where("score", 99).PlanDelete(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				result, err = tx.Exec(del.SQL, del.Params...)
				if err != nil {
					t.Fatal(err)
				}
				n, err = result.RowsAffected()
				if err != nil || n != 2 {
					t.Fatalf("delete affected %d: %v", n, err)
				}
			})
		})
	}
}
