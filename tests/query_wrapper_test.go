package tests

import (
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestJoinSubQueryWrapper(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	sub := db.Table("profiles").Select("user_id", "bio")

	var row map[string]any
	err := db.Table("users").
		JoinSubQuery(sub, "p", "users.id", "=", "p.user_id").
		Select("users.name", "p.bio").
		Where("p.bio", "like", "%python%").
		FirstMap(&row)
	if err != nil {
		t.Fatalf("join sub query: %v", err)
	}
	if row["name"] != "bob" {
		t.Errorf("expected bob, got %v", row["name"])
	}
}

func TestJoinQueryWrapper(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	var row map[string]any
	err := db.Table("users").
		JoinQuery("profiles", func(b *query.JoinClause) {
			b.On("users.id", "=", "profiles.user_id")
		}).
		Select("users.name", "profiles.bio").
		Where("profiles.bio", "like", "%go%").
		FirstMap(&row)
	if err != nil {
		t.Fatalf("join query: %v", err)
	}
	if row["name"] != "alice" {
		t.Errorf("expected alice, got %v", row["name"])
	}
}

func TestJoinLateralWrapper(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	sub := db.Table("profiles").
		Select("bio", "user_id").
		SafeWhereRaw("profiles.user_id = users.id", map[string]any{})

	var row map[string]any
	err := db.Table("users").
		JoinLateral(sub, "p").
		Select("users.name", "p.bio").
		Where("p.bio", "like", "%go%").
		FirstMap(&row)
	if err != nil {
		t.Fatalf("join lateral: %v", err)
	}
	if row["name"] != "alice" {
		t.Errorf("expected alice, got %v", row["name"])
	}
}

func TestSafeWhereRawAndGroups(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	var rows []map[string]any
	err := db.Table("users").
		WhereGroup(func(qb *query.Query) {
			qb.SafeWhereRaw("name = :name", map[string]any{"name": "alice"})
		}).
		OrWhereGroup(func(qb *query.Query) {
			qb.SafeWhereRaw("name = :name", map[string]any{"name": "bob"})
		}).
		OrderBy("id", "asc").
		GetMaps(&rows)
	if err != nil {
		t.Fatalf("safe where raw group: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

func TestWhereNotWrappers(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	var rows []map[string]any
	err := db.Table("users").
		WhereNot(func(qb *query.Query) {
			qb.Where("name", "alice")
		}).
		OrderBy("id", "asc").
		GetMaps(&rows)
	if err != nil {
		t.Fatalf("where not: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "bob" {
		t.Errorf("unexpected rows: %v", rows)
	}
}
