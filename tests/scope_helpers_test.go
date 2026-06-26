package tests

import (
	"context"
	"database/sql"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

func profileJoinScope() orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Join("profiles", "users.id", "=", "profiles.user_id")
	}
}

func bioFilterScope(v string) orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Where("profiles.bio", "like", v)
	}
}

func orderedUserSelectScope() orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Select("users.id", "users.name", "users.age").OrderBy("users.id", "asc")
	}
}

func TestSelectAllByWithScopes(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	users, err := orm.SelectAllBy[User](
		context.Background(),
		db,
		db.Model(&User{}),
		orm.ComposeScopes(profileJoinScope(), bioFilterScope("%developer%")),
		orderedUserSelectScope(),
	)
	if err != nil {
		t.Fatalf("select all by: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "alice" || users[1].Name != "bob" {
		t.Fatalf("unexpected users: %+v", users)
	}
}

func TestUpdateByWithScopes(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	_, err := orm.UpdateBy(
		context.Background(),
		db.Table("users"),
		map[string]any{"age": 41},
		profileJoinScope(),
		bioFilterScope("%go%"),
	)
	if err != nil {
		t.Fatalf("update by: %v", err)
	}

	var row map[string]any
	if err := db.Table("users").Where("id", 1).FirstMap(&row); err != nil {
		t.Fatalf("select after update: %v", err)
	}
	if row["age"] != int64(41) {
		t.Fatalf("expected age 41, got %v", row["age"])
	}
}

func TestDeleteByWithScopes(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	_, err := orm.DeleteBy(
		context.Background(),
		db.Table("users"),
		profileJoinScope(),
		bioFilterScope("%python%"),
	)
	if err != nil {
		t.Fatalf("delete by: %v", err)
	}

	var row map[string]any
	err = db.Table("users").Where("id", 2).FirstMap(&row)
	if err != sql.ErrNoRows {
		t.Fatalf("expected no rows, got %v", err)
	}
}
