package tests

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

// These are real database tests, not schema reads performed by planning. Each
// supplied context asserts the exact DDL created here and this test's executor.
func TestWriteScopeDatabaseSemantics(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			db := openTestDB(t, config.name, dsn, explicit)
			defer db.Close()
			sqlDB := db.SQLDB()
			var version string
			if err := sqlDB.QueryRow("SELECT version()").Scan(&version); err != nil {
				t.Fatal(err)
			}
			t.Logf("server: %s", version)
			for _, bits := range []int{16, 32, 64} {
				t.Run(fmt.Sprint(bits), func(t *testing.T) {
					typ := map[int]string{16: "SMALLINT", 32: "INT", 64: "BIGINT"}[bits]
					if config.name == orm.Postgres {
						typ = map[int]string{16: "smallint", 32: "integer", 64: "bigint"}[bits]
					}
					table := fmt.Sprintf("gq_write_scope_%d", bits)
					ddl := fmt.Sprintf("CREATE TABLE %s (tenant %s NOT NULL, id %s NOT NULL, n1 %s, n2 %s, score INT, PRIMARY KEY(tenant,id), UNIQUE(n1,n2))", table, typ, typ, typ, typ)
					if _, err := sqlDB.Exec(ddl); err != nil {
						t.Fatal(err)
					}
					defer sqlDB.Exec("DROP TABLE " + table)
					if _, err := sqlDB.Exec("INSERT INTO " + table + " VALUES (1,1,1,1,0),(1,2,2,2,0),(2,1,NULL,NULL,0),(2,2,NULL,NULL,0)"); err != nil {
						t.Fatal(err)
					}
					ctx := query.WriteKeyContext{Database: "testdb", Dialect: config.name, Table: table, Constraints: []query.WriteKeyConstraint{{Name: "primary", Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: []query.WriteKeyColumn{{Name: "tenant", DBType: typ, Bits: bits}, {Name: "id", DBType: typ, Bits: bits}}}}}
					t.Run("self_join_cross_alias_key_is_unknown", func(t *testing.T) {
						c := ctx
						c.Alias = "u"
						q := db.Table(table+" as u").WithWriteKeyContext(c).
							Join(table+" as v", "u.tenant", "=", "v.tenant").Where("u.tenant", 1).Where("v.id", 1)
						p, err := q.PlanUpdate(context.Background(), map[string]any{"score": 7})
						if err != nil {
							t.Fatal(err)
						}
						if r := query.AnalyzeWriteScope(p); r.Status != "unknown" {
							t.Fatalf("cross-alias write: %+v", r)
						}
						// Inspect the actual joined target set in both dialects.
						// The two mentioned key columns belong to different aliases:
						// both target ids 1 and 2 satisfy them. PostgreSQL joined
						// UPDATE rendering itself remains outside the proof surface.
						selected, err := q.Select("u.id").OrderBy("u.id", "asc").Plan(context.Background())
						if err != nil {
							t.Fatal(err)
						}
						rows, err := sqlDB.Query(selected.SQL, selected.Params...)
						if err != nil {
							t.Fatal(err)
						}
						defer rows.Close()
						count := 0
						for rows.Next() {
							var id int
							if err := rows.Scan(&id); err != nil {
								t.Fatal(err)
							}
							count++
							if id != count {
								t.Fatalf("id=%d want=%d", id, count)
							}
						}
						if err := rows.Err(); err != nil {
							t.Fatal(err)
						}
						if count != 2 {
							t.Fatalf("target rows=%d", count)
						}
					})
					cases := []struct {
						name   string
						build  func(*query.Query) *query.Query
						status string
						rows   int64
						unique bool
					}{
						{"composite_equal", func(q *query.Query) *query.Query { return q.Where("tenant", int8(1)).Where("id", uint16(2)) }, "at_most_one", 1, false},
						{"composite_in", func(q *query.Query) *query.Query { return q.WhereIn("tenant", []uint32{1}).WhereIn("id", []int64{2}) }, "at_most_one", 1, false},
						{"missing_row", func(q *query.Query) *query.Query { return q.Where("tenant", 3).Where("id", 3) }, "at_most_one", 0, false},
						{"partial", func(q *query.Query) *query.Query { return q.Where("id", 1) }, "broad", 2, false},
						{"range", func(q *query.Query) *query.Query { return q.Where("tenant", 1).Where("id", ">", 0) }, "broad", 2, false},
						{"ne", func(q *query.Query) *query.Query { return q.Where("tenant", 1).Where("id", "!=", 3) }, "broad", 2, false},
						{"multi_in", func(q *query.Query) *query.Query { return q.Where("tenant", 1).WhereIn("id", []int{1, 2}) }, "broad", 2, false},
						{"empty_in", func(q *query.Query) *query.Query { return q.Where("tenant", 1).WhereIn("id", []int{}) }, "unknown", -1, false},
						{"same_or", func(q *query.Query) *query.Query {
							return q.WhereGroup(func(q *query.Query) { q.Where("tenant", 1).Where("id", 1) }).OrWhereGroup(func(q *query.Query) { q.Where("tenant", uint8(1)).Where("id", int64(1)) })
						}, "at_most_one", 1, false},
						{"different_or", func(q *query.Query) *query.Query {
							return q.WhereGroup(func(q *query.Query) { q.Where("tenant", 1).Where("id", 1) }).OrWhereGroup(func(q *query.Query) { q.Where("tenant", 1).Where("id", 2) })
						}, "broad", 2, false},
						{"non_key_or", func(q *query.Query) *query.Query {
							return q.WhereGroup(func(q *query.Query) { q.Where("tenant", 1).Where("id", 1) }).OrWhere("score", 0)
						}, "broad", 4, false},
						{"not", func(q *query.Query) *query.Query { return q.WhereNot(func(q *query.Query) { q.Where("id", 1) }) }, "broad", 2, false},
						{"column", func(q *query.Query) *query.Query { return q.WhereColumn("tenant", "=", "id") }, "broad", 2, false},
						{"raw_branch", func(q *query.Query) *query.Query {
							return q.WhereGroup(func(q *query.Query) { q.Where("tenant", 1).Where("id", 1) }).SafeOrWhereRaw("score = :s", map[string]any{"s": 0})
						}, "unknown", 4, false},
						{"nullable_unique", func(q *query.Query) *query.Query { return q.Where("n1", 1).Where("n2", 1) }, "at_most_one", 1, true},
						{"null_unique", func(q *query.Query) *query.Query { return q.WhereNull("n1").WhereNull("n2") }, "broad", 2, true},
						{"bound_null", func(q *query.Query) *query.Query { return q.Where("n1", nil).Where("n2", nil) }, "broad", 0, true},
						{"null_in", func(q *query.Query) *query.Query { return q.WhereIn("n1", []any{nil, 1}).Where("n2", 1) }, "broad", 1, true},
					}
					for _, tc := range cases {
						t.Run(tc.name, func(t *testing.T) {
							c := ctx
							if tc.unique {
								c.Constraints = []query.WriteKeyConstraint{{Name: "nullable_unique", Kind: "unique", AllRows: true, Valid: true, NotDeferrable: true, Columns: []query.WriteKeyColumn{{Name: "n1", DBType: typ, Bits: bits, Nullable: true}, {Name: "n2", DBType: typ, Bits: bits, Nullable: true}}}}
							}
							for _, op := range []string{"update", "delete"} {
								tx, err := sqlDB.Begin()
								if err != nil {
									t.Fatal(err)
								}
								func() {
									defer tx.Rollback()
									q := tc.build(db.Table(table).WithWriteKeyContext(c))
									var p *query.QueryPlan
									if op == "update" {
										p, err = q.PlanUpdate(context.Background(), map[string]any{"score": 7})
									} else {
										p, err = q.PlanDelete(context.Background())
									}
									if err != nil {
										t.Fatal(err)
									}
									if r := query.AnalyzeWriteScope(p); r.Status != tc.status {
										t.Fatalf("%s: %+v", op, r)
									}
									result, err := tx.Exec(p.SQL, p.Params...)
									if tc.name == "empty_in" {
										// Existing rendering emits IN (), rejected by both DBs.
										// Unknown must not turn this into a zero-row proof.
										if err == nil {
											t.Fatal("empty IN unexpectedly executed")
										}
										return
									}
									if err != nil {
										t.Fatalf("%s: %v", p.SQL, err)
									}
									n, err := result.RowsAffected()
									if err != nil || n != tc.rows {
										t.Fatalf("%s rows=%d want=%d err=%v", op, n, tc.rows, err)
									}
								}()
							}
						})
					}
					// Single-column keys and actual extrema exercise parameter conversion in
					// each driver; uint64 is accepted only inside the signed int64 range.
					single := table + "_single"
					if _, err := sqlDB.Exec("CREATE TABLE " + single + " (id " + typ + " PRIMARY KEY, score INT)"); err != nil {
						t.Fatal(err)
					}
					defer sqlDB.Exec("DROP TABLE " + single)
					min, max := int64(math.MinInt64), int64(math.MaxInt64)
					if bits < 64 {
						min = -(int64(1) << (bits - 1))
						max = (int64(1) << (bits - 1)) - 1
					}
					if _, err := sqlDB.Exec(fmt.Sprintf("INSERT INTO %s VALUES (%d,0),(%d,0)", single, min, max)); err != nil {
						t.Fatal(err)
					}
					c := ctx
					c.Table = single
					c.Constraints = []query.WriteKeyConstraint{{Name: "pk", Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: []query.WriteKeyColumn{{Name: "id", DBType: typ, Bits: bits}}}}
					for _, v := range []any{min, max, uint64(max)} {
						for _, in := range []bool{false, true} {
							q := db.Table(single).WithWriteKeyContext(c)
							if in {
								q.WhereIn("id", []any{v})
							} else {
								q.Where("id", v)
							}
							p, err := q.PlanDelete(context.Background())
							if err != nil {
								t.Fatal(err)
							}
							if r := query.AnalyzeWriteScope(p); r.Status != "at_most_one" {
								t.Fatalf("boundary %T %v: %+v", v, v, r)
							}
							tx, err := sqlDB.Begin()
							if err != nil {
								t.Fatal(err)
							}
							res, err := tx.Exec(p.SQL, p.Params...)
							if err != nil {
								tx.Rollback()
								t.Fatal(err)
							}
							n, err := res.RowsAffected()
							tx.Rollback()
							if err != nil || n != 1 {
								t.Fatalf("boundary rows=%d: %v", n, err)
							}
						}
					}
				})
			}
			t.Run("string_unique_numeric_comparison_is_unknown", func(t *testing.T) {
				if _, err := sqlDB.Exec("CREATE TABLE gq_write_text (id VARCHAR(20) UNIQUE)"); err != nil {
					t.Fatal(err)
				}
				defer sqlDB.Exec("DROP TABLE gq_write_text")
				if _, err := sqlDB.Exec("INSERT INTO gq_write_text VALUES ('1'),('01')"); err != nil {
					t.Fatal(err)
				}
				c := query.WriteKeyContext{Database: "testdb", Dialect: config.name, Table: "gq_write_text", Constraints: []query.WriteKeyConstraint{{Kind: "unique", Valid: true, AllRows: true, NotDeferrable: true, Columns: []query.WriteKeyColumn{{Name: "id", DBType: "VARCHAR", Nullable: true}}}}}
				p, err := db.Table(c.Table).WithWriteKeyContext(c).Where("id", 1).PlanDelete(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if r := query.AnalyzeWriteScope(p); r.Status != "unknown" {
					t.Fatalf("%+v", r)
				}
				tx, err := sqlDB.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
				res, err := tx.Exec(p.SQL, p.Params...)
				// MySQL coerces both distinct strings to numeric 1; PostgreSQL
				// infers the parameter as text and matches only the literal 1.
				if err != nil {
					t.Fatal(err)
				}
				n, err := res.RowsAffected()
				want := int64(2)
				if config.name == orm.Postgres {
					want = 1
				}
				if err != nil || n != want {
					t.Fatalf("rows=%d want=%d err=%v", n, want, err)
				}
			})
			if config.name == orm.Postgres {
				t.Run("deferred_unique_is_unknown", func(t *testing.T) {
					if _, err := sqlDB.Exec("CREATE TABLE gq_write_deferred (id integer UNIQUE DEFERRABLE INITIALLY DEFERRED)"); err != nil {
						t.Fatal(err)
					}
					defer sqlDB.Exec("DROP TABLE gq_write_deferred")
					tx, err := sqlDB.Begin()
					if err != nil {
						t.Fatal(err)
					}
					defer tx.Rollback()
					if _, err = tx.Exec("INSERT INTO gq_write_deferred VALUES(1),(1)"); err != nil {
						t.Fatal(err)
					}
					c := query.WriteKeyContext{Database: "testdb", Dialect: "postgres", Table: "gq_write_deferred", Constraints: []query.WriteKeyConstraint{{Kind: "unique", Valid: true, AllRows: true, NotDeferrable: false, Columns: []query.WriteKeyColumn{{Name: "id", DBType: "integer", Bits: 32, Nullable: true}}}}}
					p, err := db.Table(c.Table).WithWriteKeyContext(c).Where("id", 1).PlanDelete(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					if r := query.AnalyzeWriteScope(p); r.Status != "unknown" {
						t.Fatalf("%+v", r)
					}
					res, err := tx.Exec(p.SQL, p.Params...)
					if err != nil {
						t.Fatal(err)
					}
					n, err := res.RowsAffected()
					if err != nil || n != 2 {
						t.Fatalf("rows=%d err=%v", n, err)
					}
				})
			}
		})
	}
}
