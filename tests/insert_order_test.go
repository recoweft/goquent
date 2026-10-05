package tests

import (
	"context"
	"database/sql"
	"testing"

	"github.com/recoweft/goquent/orm"
)

func TestInsertOrderRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(testing.TB) *orm.DB
	}{
		{"mysql", setupDB}, {"postgres", setupPgDB},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := tc.setup(t)
			defer db.Close()
			ctx := context.Background()
			type row struct {
				Name string `db:"name"`
				ID   int64  `db:"id"`
				Age  *int   `db:"age"`
			}
			age := 37
			if _, err := orm.Insert(ctx, db, row{Name: "struct-order", ID: 101, Age: &age}, orm.Table("users")); err != nil {
				t.Fatal(err)
			}
			if _, err := orm.Insert(ctx, db, map[string]any{"name": "map-order", "age": nil, "id": int64(102)}, orm.Table("users")); err != nil {
				t.Fatal(err)
			}
			if _, err := orm.InsertMany(ctx, db, []map[string]any{
				{"name": "batch-first", "id": int64(103), "age": 19},
				{"age": nil, "id": int64(104), "name": "batch-second"},
			}, orm.Table("users")); err != nil {
				t.Fatal(err)
			}
			var got []struct {
				ID   int64         `db:"id"`
				Name string        `db:"name"`
				Age  sql.NullInt64 `db:"age"`
			}
			if err := db.Table("users").Where("id", ">=", 101).OrderBy("id", "asc").Get(&got); err != nil {
				t.Fatal(err)
			}
			names := []string{"struct-order", "map-order", "batch-first", "batch-second"}
			ages := []sql.NullInt64{{Int64: 37, Valid: true}, {}, {Int64: 19, Valid: true}, {}}
			if len(got) != 4 {
				t.Fatalf("rows: %#v", got)
			}
			for i, r := range got {
				if r.ID != int64(101+i) || r.Name != names[i] || r.Age != ages[i] {
					t.Fatalf("row %d: %#v", i, r)
				}
			}
		})
	}
}
