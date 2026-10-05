package orm

import (
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
)

// Field order intentionally differs from lexical column order.
type insertOrderRow struct {
	Z       int64  `db:"z,pk"`
	A       string `db:"a"`
	N       *int   `db:"n"`
	O       string `db:"optional,omitempty"`
	R       string `db:"generated,readonly"`
	Ignored string `db:"-"`
}

func TestInsertColumnArgumentOrder(t *testing.T) {
	for _, dialect := range []driver.Dialect{driver.MySQLDialect{}, driver.PostgresDialect{}} {
		db := NewDB(nil, dialect)
		for iteration := 0; iteration < 100; iteration++ {
			row := map[string]any{}
			keys := []string{"z", "a", "n"}
			vals := []any{int64(42), "first", nil}
			for j := range keys {
				k := (j + iteration) % len(keys)
				row[keys[k]] = vals[k]
			}
			opts := applyWriteOpts([]WriteOpt{Table("ordered_rows")})
			sql, args, err := buildInsertStatement(db, row, opts)
			want := "INSERT INTO `ordered_rows` (`a`, `n`, `z`) VALUES (?, ?, ?)"
			if _, ok := dialect.(driver.PostgresDialect); ok {
				want = `INSERT INTO "ordered_rows" ("a", "n", "z") VALUES ($1, $2, $3)`
			}
			if err != nil || sql != want || !reflect.DeepEqual(args, []any{"first", nil, int64(42)}) {
				t.Fatalf("map: %s %#v %v", sql, args, err)
			}
			// Different values in the second row detect accidental cross-row binding.
			batchSQL, batchArgs, err := buildInsertManyStatement(db, []map[string]any{row, {"n": "present", "z": int64(7), "a": "second"}}, opts)
			suffix := ", (?, ?, ?)"
			if _, ok := dialect.(driver.PostgresDialect); ok {
				suffix = ", ($4, $5, $6)"
			}
			if err != nil || batchSQL != want+suffix || !reflect.DeepEqual(batchArgs, []any{"first", nil, int64(42), "second", "present", int64(7)}) {
				t.Fatalf("batch: %s %#v %v", batchSQL, batchArgs, err)
			}
			if !reflect.DeepEqual(row, map[string]any{"z": int64(42), "a": "first", "n": nil}) {
				t.Fatal("input mutated")
			}

			value := insertOrderRow{Z: 42, A: "first", R: "database generated"}
			sql, args, err = buildInsertStatement(db, value, opts)
			want = "INSERT INTO `ordered_rows` (`z`, `a`, `n`) VALUES (?, ?, ?)"
			if _, ok := dialect.(driver.PostgresDialect); ok {
				want = `INSERT INTO "ordered_rows" ("z", "a", "n") VALUES ($1, $2, $3)`
			}
			expected := []any{int64(42), "first", (*int)(nil)}
			if err != nil || sql != want || !reflect.DeepEqual(args, expected) {
				t.Fatalf("struct: %s %#v %v", sql, args, err)
			}
			batchSQL, batchArgs, err = buildInsertManyStatement(db, []insertOrderRow{value, {Z: 7, A: "second"}}, opts)
			if err != nil || batchSQL != want+suffix || !reflect.DeepEqual(batchArgs, append(expected, int64(7), "second", (*int)(nil))) {
				t.Fatalf("struct batch: %s %#v %v", batchSQL, batchArgs, err)
			}
		}
	}
}

func TestInsertSelectionAndReturningOrder(t *testing.T) {
	db := NewDB(nil, driver.PostgresDialect{})
	opts := applyWriteOpts([]WriteOpt{Table("ordered_rows"), Columns("z", "missing", "a", "n"), Omit("n"), Returning("z", "a")})
	sql, args, err := buildInsertStatement(db, map[string]any{"z": int64(42), "a": "first", "n": nil, "ignored": true}, opts)
	want := `INSERT INTO "ordered_rows" ("a", "z") VALUES ($1, $2) RETURNING "z", "a"`
	if err != nil || sql != want || !reflect.DeepEqual(args, []any{"first", int64(42)}) {
		t.Fatalf("map selection: %s %#v %v", sql, args, err)
	}
	sql, args, err = buildInsertStatement(db, insertOrderRow{Z: 42, A: "first"}, opts)
	want = `INSERT INTO "ordered_rows" ("z", "a") VALUES ($1, $2) RETURNING "z", "a"`
	if err != nil || sql != want || !reflect.DeepEqual(args, []any{int64(42), "first"}) {
		t.Fatalf("struct selection: %s %#v %v", sql, args, err)
	}
}

func TestInsertNamedMapKey(t *testing.T) {
	type column string
	row := map[column]any{"z": int64(42), "a": "first"}
	db := NewDB(nil, driver.MySQLDialect{})
	sql, args, err := buildInsertStatement(db, row, applyWriteOpts([]WriteOpt{Table("ordered_rows")}))
	if err != nil || sql != "INSERT INTO `ordered_rows` (`a`, `z`) VALUES (?, ?)" || !reflect.DeepEqual(args, []any{"first", int64(42)}) {
		t.Fatalf("named keys: %s %#v %v", sql, args, err)
	}
}
