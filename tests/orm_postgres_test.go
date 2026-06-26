package tests

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

type pgUpsertOmitEmptyUser struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name,omitempty"`
}

func (pgUpsertOmitEmptyUser) TableName() string { return "users" }

func setupPgDB(t testing.TB) *orm.DB {
	dsn, explicit := lookupTestDSN("TEST_POSTGRES_DSN", defaultPostgresTestDSN)
	db := openTestDB(t, orm.Postgres, dsn, explicit)
	var err error
	stdDB := db.SQLDB()
	_, err = stdDB.Exec(`CREATE TABLE IF NOT EXISTS users (
        id SERIAL PRIMARY KEY,
        name TEXT,
        age INT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = stdDB.Exec(`CREATE TABLE IF NOT EXISTS profiles (
        id SERIAL PRIMARY KEY,
        user_id INT,
        bio TEXT
    )`)
	if err != nil {
		t.Fatalf("create profiles table: %v", err)
	}
	_, err = stdDB.Exec("TRUNCATE TABLE profiles, users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	_, err = stdDB.Exec("INSERT INTO users(name, age) VALUES ('alice', 30), ('bob', 25)")
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}
	_, err = stdDB.Exec("INSERT INTO profiles(user_id, bio) VALUES (1, 'go developer'), (2, 'python developer')")
	if err != nil {
		t.Fatalf("seed profiles: %v", err)
	}
	return db
}

func profileBioExistsScope(db *orm.DB, bio string) orm.Scope {
	return func(q *query.Query) *query.Query {
		sub := db.Table("profiles").
			SelectRaw("1").
			SafeWhereRaw("profiles.user_id = users.id", map[string]any{}).
			Where("profiles.bio", "like", bio)
		return q.WhereExists(sub)
	}
}

func TestPostgresInsertSelect(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()
	if _, err := db.Table("users").Insert(map[string]any{"name": "pg", "age": 10}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var row map[string]any
	if err := db.Table("users").Where("name", "pg").FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["age"] != int64(10) {
		t.Errorf("expected age 10, got %v", row["age"])
	}
}

func TestPostgresInsertGetId(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()
	id, err := db.Table("users").InsertGetId(map[string]any{"name": "pg2", "age": 11})
	if err != nil {
		t.Fatalf("insert get id: %v", err)
	}
	if id != 3 {
		t.Errorf("expected id 3, got %d", id)
	}
}

func TestPostgresInsertGetIdCustomPrimaryKey(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()
	stdDB := db.SQLDB()
	if _, err := stdDB.Exec(`CREATE TABLE IF NOT EXISTS items (
               item_id SERIAL PRIMARY KEY,
               name TEXT
       )`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := stdDB.Exec("TRUNCATE TABLE items RESTART IDENTITY"); err != nil {
		t.Fatalf("truncate items: %v", err)
	}
	defer stdDB.Exec("DROP TABLE items")
	id, err := db.Table("items").PrimaryKey("item_id").InsertGetId(map[string]any{"name": "foo"})
	if err != nil {
		t.Fatalf("insert get id: %v", err)
	}
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
}

func TestPostgresInsertReturning(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()

	ctx := context.Background()
	res, err := orm.Insert(
		ctx,
		db,
		map[string]any{"name": "carol", "age": 41},
		orm.Table("users"),
		orm.Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("insert returning: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil || aff != 1 {
		t.Fatalf("expected rows affected 1, got %d err=%v", aff, err)
	}

	var row map[string]any
	if err := db.Table("users").Where("name", "carol").FirstMap(&row); err != nil {
		t.Fatalf("select inserted: %v", err)
	}
	if row["age"] != int64(41) {
		t.Fatalf("expected age 41, got %v", row["age"])
	}
}

func TestPostgresUpdateReturning(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()

	ctx := context.Background()
	res, err := orm.Update(
		ctx,
		db,
		User{ID: 1, Name: "alice_pg"},
		orm.Columns("name"),
		orm.WherePK(),
		orm.Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("update returning: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil || aff != 1 {
		t.Fatalf("expected rows affected 1, got %d err=%v", aff, err)
	}

	var name string
	if err := rawQueryRow(t, db, ctx, "SELECT name FROM users WHERE id = $1", 1).Scan(&name); err != nil {
		t.Fatalf("select updated: %v", err)
	}
	if name != "alice_pg" {
		t.Fatalf("expected alice_pg, got %s", name)
	}
}

func TestPostgresUpsertReturning(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()

	ctx := context.Background()
	res, err := orm.Upsert(
		ctx,
		db,
		User{ID: 2, Name: "bob_pg"},
		orm.Columns("name"),
		orm.WherePK(),
		orm.Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("upsert returning: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil || aff != 1 {
		t.Fatalf("expected rows affected 1, got %d err=%v", aff, err)
	}

	var name string
	if err := rawQueryRow(t, db, ctx, "SELECT name FROM users WHERE id = $1", 2).Scan(&name); err != nil {
		t.Fatalf("select upserted: %v", err)
	}
	if name != "bob_pg" {
		t.Fatalf("expected bob_pg, got %s", name)
	}
}

func TestPostgresUpsertKeepsPKWhenFiltered(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()

	ctx := context.Background()
	t.Run("struct", func(t *testing.T) {
		if _, err := orm.Upsert(
			ctx,
			db,
			User{ID: 2, Name: "bob_struct_pg"},
			orm.Columns("name"),
			orm.Omit("id"),
			orm.WherePK(),
		); err != nil {
			t.Fatalf("upsert struct filtered: %v", err)
		}

		var name string
		if err := rawQueryRow(t, db, ctx, "SELECT name FROM users WHERE id = $1", 2).Scan(&name); err != nil {
			t.Fatalf("select struct: %v", err)
		}
		if name != "bob_struct_pg" {
			t.Fatalf("expected bob_struct_pg, got %s", name)
		}
	})

	t.Run("map", func(t *testing.T) {
		if _, err := orm.Upsert(
			ctx,
			db,
			map[string]any{"id": int64(1), "name": "alice_map_pg"},
			orm.Table("users"),
			orm.PK("id"),
			orm.Columns("name"),
			orm.Omit("id"),
			orm.WherePK(),
		); err != nil {
			t.Fatalf("upsert map filtered: %v", err)
		}

		var name string
		if err := rawQueryRow(t, db, ctx, "SELECT name FROM users WHERE id = $1", 1).Scan(&name); err != nil {
			t.Fatalf("select map: %v", err)
		}
		if name != "alice_map_pg" {
			t.Fatalf("expected alice_map_pg, got %s", name)
		}
	})

	t.Run("omitempty", func(t *testing.T) {
		if _, err := orm.Upsert(ctx, db, pgUpsertOmitEmptyUser{ID: 12}, orm.WherePK()); err != nil {
			t.Fatalf("upsert omitempty: %v", err)
		}

		var name sql.NullString
		if err := rawQueryRow(t, db, ctx, "SELECT name FROM users WHERE id = $1", 12).Scan(&name); err != nil {
			t.Fatalf("select omitempty: %v", err)
		}
		if name.Valid {
			t.Fatalf("expected NULL name, got %q", name.String)
		}
	})
}

func TestPostgresSelectAllByWithScopes(t *testing.T) {
	db := setupPgDB(t)
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

func TestPostgresUpdateByWithWhereExistsScope(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()

	_, err := orm.UpdateBy(
		context.Background(),
		db.Table("users"),
		map[string]any{"age": 44},
		profileBioExistsScope(db, "%go%"),
	)
	if err != nil {
		t.Fatalf("update by: %v", err)
	}

	var age int
	if err := rawQueryRow(t, db, context.Background(), "SELECT age FROM users WHERE id = $1", 1).Scan(&age); err != nil {
		t.Fatalf("select updated age: %v", err)
	}
	if age != 44 {
		t.Fatalf("expected age 44, got %d", age)
	}
}

func TestPostgresDeleteByWithWhereExistsScope(t *testing.T) {
	db := setupPgDB(t)
	defer db.Close()

	_, err := orm.DeleteBy(
		context.Background(),
		db.Table("users"),
		profileBioExistsScope(db, "%python%"),
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
