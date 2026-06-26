package tests

import (
	"context"
	"testing"

	"github.com/recoweft/goquent/orm"
)

func TestSelectOneMap(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	rawDB := db.RequireRawApproval("raw generic select test")
	ctx := context.Background()
	row, err := orm.SelectOne[map[string]any](ctx, rawDB, "SELECT id, name FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("select map: %v", err)
	}
	if row["name"] != "alice" {
		t.Errorf("expected alice, got %v", row["name"])
	}
}

func TestSelectAllStruct(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	rawDB := db.RequireRawApproval("raw generic select test")
	ctx := context.Background()
	users, err := orm.SelectAll[User](ctx, rawDB, "SELECT id, name, age FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("select structs: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "alice" || users[1].Name != "bob" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestSelectAllStructTag(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	rawDB := db.RequireRawApproval("raw generic select test")
	ctx := context.Background()
	users, err := orm.SelectAll[UserSchema](ctx, rawDB, "SELECT id, name, age, schema_name FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("select structs: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if !users[0].Schema.Valid || users[0].Schema.String != "main" {
		t.Errorf("unexpected schema: %+v", users[0].Schema)
	}
}

func TestSelectStructAlias(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	rawDB := db.RequireRawApproval("raw generic select test")
	ctx := context.Background()
	u, err := orm.SelectOne[User](ctx, rawDB, "SELECT id AS ID, name AS Name, age FROM users WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("select alias: %v", err)
	}
	if u.Name != "alice" {
		t.Errorf("expected alice, got %v", u.Name)
	}
}

func BenchmarkSelectStruct_Cold(b *testing.B) {
	db := setupDB(b)
	defer db.Close()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		orm.ResetMetaCache()
		if _, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age FROM users WHERE id = 1"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectStruct_Warm(b *testing.B) {
	db := setupDB(b)
	defer db.Close()
	ctx := context.Background()
	if _, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age FROM users WHERE id = 1"); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age FROM users WHERE id = 1"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectMap(b *testing.B) {
	db := setupDB(b)
	defer db.Close()
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		if _, err := orm.SelectOne[map[string]any](ctx, db, "SELECT id, name, age FROM users WHERE id = 1"); err != nil {
			b.Fatal(err)
		}
	}
}
