package migration

import (
	"context"
	sqldriver "database/sql/driver"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"testing"
)

var pgConstraintColumns = []string{"table", "name", "kind", "local", "rs", "rt", "ref", "up", "del", "match", "validated", "deferrable", "deferred", "expr", "raw", "locality", "inheritance", "noinherit"}
var pgIndexColumns = []string{"table", "name", "unique", "raw", "method", "predicate", "ord", "col", "expr", "nkeys", "options", "valid", "ready", "defaultop", "samecoll", "nullsdist", "noopts", "relkind"}
var mysqlFKColumns = []string{"table", "name", "col", "ord", "rs", "rt", "rc", "up", "del", "match"}
var mysqlIndexColumns = []string{"table", "name", "nonunique", "ord", "col", "prefix", "order", "method", "visible", "expr"}

func TestCatalogFailuresDoNotBecomeEmpty(t *testing.T) {
	for _, stage := range []string{"columns", "identity", "constraints", "indexes"} {
		for _, failure := range []string{"query", "scan", "rows"} {
			t.Run(stage+"/"+failure, func(t *testing.T) {
				db, m := newStatusMockDB(t)
				queries := []struct {
					stage, q string
					columns  []string
					row      []any
				}{
					{"columns", "information_schema.columns", []string{"table", "col", "type", "nullable", "default"}, []any{"t", "a", "integer", "YES", "1"}},
					{"identity", "current_database", []string{"catalog", "schema", "version"}, []any{"testdb", "public", "16.10"}},
					{"constraints", "FROM pg_constraint", pgConstraintColumns, []any{"t", "ck", "c", "[]", "", "", "[]", " ", " ", " ", true, false, false, "a>0", "raw", true, 0, false}},
					{"indexes", "JOIN pg_index", pgIndexColumns, []any{"t", "ix", false, "raw", "btree", "", 1, "a", "a", 1, 0, true, true, true, true, false, true, "r"}},
				}
				for _, q := range queries {
					expect := m.ExpectQuery(q.q)
					if q.stage == stage && failure == "query" {
						expect.WillReturnError(errors.New("private_catalog_error"))
						break
					}
					rows := sqlmock.NewRows(q.columns)
					if q.stage == stage && failure == "scan" {
						rows = sqlmock.NewRows([]string{"wrong"}).AddRow("x")
					} else {
						vals := make([]interface{}, len(q.row))
						copy(vals, q.row) // sqlmock uses driver.Value, supplied below through a conversion helper.
						addMockRow(rows, vals)
						if q.stage == stage && failure == "rows" {
							rows.RowError(0, errors.New("private_catalog_error"))
						}
					}
					expect.WillReturnRows(rows)
					if q.stage == stage {
						break
					}
				}
				if _, err := ReadSchema(context.Background(), db, driver.PostgresDialect{}); err == nil {
					t.Fatal("catalog failure became success")
				}
				if err := m.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func addMockRow(rows *sqlmock.Rows, values []any) {
	v := make([]sqldriver.Value, len(values))
	for i, x := range values {
		v[i] = x
	}
	rows.AddRow(v...)
}

func TestMySQLCatalogFailuresDoNotBecomeEmpty(t *testing.T) {
	for _, stage := range []string{"columns", "identity", "foreign_keys", "checks", "indexes"} {
		for _, failure := range []string{"query", "scan", "rows"} {
			t.Run(stage+"/"+failure, func(t *testing.T) {
				db, m := newStatusMockDB(t)
				queries := []struct {
					stage, q string
					columns  []string
					row      []any
				}{
					{"columns", "information_schema.columns", []string{"table", "col", "type", "nullable", "default"}, []any{"t", "a", "int", "YES", "1"}},
					{"identity", "information_schema.schemata", []string{"catalog", "schema", "version"}, []any{"def", "testdb", "8.4.6"}},
					{"foreign_keys", "key_column_usage", mysqlFKColumns, []any{"t", "fk", "a", 1, "testdb", "other", "a", "CASCADE", "RESTRICT", "NONE"}},
					{"checks", "check_constraints", []string{"table", "name", "expr", "enforced"}, []any{"t", "ck", "a>0", "YES"}},
					{"indexes", "information_schema.statistics", mysqlIndexColumns, []any{"t", "ix", 1, 1, "a", nil, "A", "BTREE", "YES", nil}},
				}
				for _, q := range queries {
					e := m.ExpectQuery(q.q)
					if q.stage == stage && failure == "query" {
						e.WillReturnError(errors.New("private_catalog_error"))
						break
					}
					r := sqlmock.NewRows(q.columns)
					if q.stage == stage && failure == "scan" {
						r = sqlmock.NewRows([]string{"wrong"}).AddRow("x")
					} else {
						addMockRow(r, q.row)
						if q.stage == stage && failure == "rows" {
							r.RowError(0, errors.New("private_catalog_error"))
						}
					}
					e.WillReturnRows(r)
					if q.stage == stage {
						break
					}
				}
				if _, err := ReadSchema(context.Background(), db, driver.MySQLDialect{}); err == nil {
					t.Fatal("catalog failure became success")
				}
				if err := m.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
