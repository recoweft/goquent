package tests

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/operation"
)

func TestTypedOperationStoredValuesAndZeroLimit(t *testing.T) {
	for _, pg := range []bool{false, true} {
		t.Run(fmt.Sprint(pg), func(t *testing.T) {
			var db *orm.DB
			dialect := "mysql"
			if pg {
				db = setupPgDB(t)
				dialect = "postgres"
			} else {
				db = setupDB(t)
			}
			defer db.Close()
			var version string
			if e := db.SQLDB().QueryRow("SELECT VERSION()").Scan(&version); e != nil {
				t.Fatal(e)
			}
			t.Logf("fixture database version: %s", version)
			// The private SQL connection is used only for this explicit storage experiment.
			// Compile itself remains DB-free and its result is not an execution permit.
			std := db.SQLDB()
			if _, e := std.Exec("DROP TABLE IF EXISTS gq07_typed_fixture"); e != nil {
				t.Fatal(e)
			}
			if _, e := std.Exec("CREATE TABLE gq07_typed_fixture (id BIGINT NOT NULL, amount DECIMAL(20,4) NOT NULL)"); e != nil {
				t.Fatal(e)
			}
			defer std.Exec("DROP TABLE gq07_typed_fixture")
			insert := "INSERT INTO gq07_typed_fixture(id,amount) VALUES (?,?)"
			if pg {
				insert = "INSERT INTO gq07_typed_fixture(id,amount) VALUES ($1,$2)"
			}
			for _, pair := range [][2]string{{"9007199254740992", "9999999999999999.9998"}, {"9007199254740993", "9999999999999999.9999"}} {
				if _, e := std.Exec(insert, json.Number(pair[0]), json.Number(pair[1])); e != nil {
					t.Fatal(e)
				}
			}
			schema := migration.Schema{Tables: []migration.TableSchema{{Name: "gq07_typed_fixture", Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint", NullableKnown: true}, {Name: "amount", Type: "decimal(20,4)", NullableKnown: true}}}}}
			m, e := manifest.Generate(manifest.Options{Dialect: dialect, Schema: &schema})
			if e != nil {
				t.Fatal(e)
			}
			for _, filter := range []operation.FilterSpec{
				{Field: "id", Op: "=", Value: json.Number("9007199254740993")},
				{Field: "id", Op: "in", Value: []any{json.Number("9007199254740993")}},
				{Field: "amount", Op: "=", Value: json.Number("9999999999999999.9999")},
				{Field: "id", Op: "=", Value: int64(9007199254740993)},
			} {
				spec := operation.OperationSpec{Operation: "select", Model: "gq07_typed_fixture", Select: []string{"id", "amount"}, Filters: []operation.FilterSpec{filter}}
				p, e := db.CompileOperation(t.Context(), spec, operation.Options{Manifest: m})
				if e != nil {
					t.Fatal(e)
				}
				if filter.Op != "in" && !reflect.DeepEqual(p.Params[0], filter.Value) {
					t.Fatal("argument was coerced")
				}
				rows, e := std.Query(p.SQL, p.Params...)
				if e != nil {
					t.Fatal(e)
				}
				var ids, amounts []string
				for rows.Next() {
					var id, amount string
					if e := rows.Scan(&id, &amount); e != nil {
						t.Fatal(e)
					}
					ids = append(ids, id)
					amounts = append(amounts, amount)
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					t.Fatal(e)
				}
				if len(ids) != 1 || ids[0] != "9007199254740993" || amounts[0] != "9999999999999999.9999" {
					t.Fatalf("typed comparison is not exact for %s / %T (row count %d)", filter.Field, filter.Value, len(ids))
				}
			}
			spec := operation.OperationSpec{Operation: "select", Model: "gq07_typed_fixture", Select: []string{"id"}}
			for _, n := range []int64{-1, 0, 1} {
				if n >= 0 {
					spec.Limit = &n
				}
				p, e := db.CompileOperation(t.Context(), spec, operation.Options{Manifest: m})
				if e != nil {
					t.Fatal(e)
				}
				rows, e := std.Query(p.SQL, p.Params...)
				if e != nil {
					t.Fatal(e)
				}
				count := 0
				for rows.Next() {
					count++
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					t.Fatal(e)
				}
				want := int(n)
				if n < 0 {
					want = 2
				}
				if count != want {
					t.Fatal("zero limit lost", count, want)
				}
			}
			var empty []struct{ ID int64 }
			if e := db.Table("gq07_typed_fixture").Select("id").LimitExact(0).Get(&empty); e != nil || len(empty) != 0 {
				t.Fatal("Get zero rows", e)
			}
			if _, e := db.Table("gq07_typed_fixture").LimitExact(0).Count(); !errors.Is(e, sql.ErrNoRows) {
				t.Fatal("Count synthesized a row", e)
			}
			observed, e := migration.ReadSchema(t.Context(), std, db.Dialect(), migration.WithSchemaReadTables("gq07_typed_fixture"))
			if e != nil {
				t.Fatal(e)
			}
			if len(observed.Tables) != 1 {
				t.Fatal("missing live schema")
			}
			for _, c := range observed.Tables[0].Columns {
				if !c.NullableKnown || c.Nullable {
					t.Fatal("live NO lost")
				}
			}
		})
	}
}

func TestTypedOperationDriverIntegerExtremes(t *testing.T) {
	for _, pg := range []bool{false, true} {
		t.Run(fmt.Sprint(pg), func(t *testing.T) {
			var db *orm.DB
			dialect, typ, placeholder := "mysql", "bigint unsigned", "?"
			values := []any{uint64(18446744073709551614), json.Number("18446744073709551615")}
			if pg {
				db = setupPgDB(t)
				dialect, typ, placeholder = "postgres", "bigint", "$1"
				values = []any{int64(-9223372036854775808), json.Number("9223372036854775807")}
			} else {
				db = setupDB(t)
			}
			defer db.Close()
			std := db.SQLDB()
			if _, e := std.Exec("DROP TABLE IF EXISTS gq07_integer_fixture"); e != nil {
				t.Fatal(e)
			}
			if _, e := std.Exec("CREATE TABLE gq07_integer_fixture(v " + typ + " NOT NULL)"); e != nil {
				t.Fatal(e)
			}
			defer std.Exec("DROP TABLE gq07_integer_fixture")
			for _, v := range values {
				if _, e := std.Exec("INSERT INTO gq07_integer_fixture(v) VALUES ("+placeholder+")", v); e != nil {
					t.Fatal(e)
				}
			}
			m := &manifest.Manifest{Dialect: dialect, Tables: []manifest.Table{{Name: "gq07_integer_fixture", Columns: []manifest.Column{{Name: "v", Type: typ, TypeSource: "sql", NullableKnown: true}}}}}
			for _, v := range values {
				for _, op := range []string{"=", "in"} {
					arg := v
					if op == "in" {
						arg = []any{v}
					}
					spec := operation.OperationSpec{Operation: "select", Model: "gq07_integer_fixture", Select: []string{"v"}, Filters: []operation.FilterSpec{{Field: "v", Op: op, Value: arg}}}
					p, e := db.CompileOperation(t.Context(), spec, operation.Options{Manifest: m})
					if e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(p.Params[0], v) {
						t.Fatal("typed argument changed")
					}
					rows, e := std.Query(p.SQL, p.Params...)
					if e != nil {
						t.Fatal(e)
					}
					count := 0
					for rows.Next() {
						var got string
						if e := rows.Scan(&got); e != nil {
							t.Fatal(e)
						}
						if got != fmt.Sprint(v) {
							t.Fatal("stored integer changed")
						}
						count++
					}
					e = rows.Err()
					rows.Close()
					if e != nil || count != 1 {
						t.Fatal("integer comparison not exact", count, e)
					}
				}
			}
		})
	}
}
