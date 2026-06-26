package tests

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/recoweft/goquent/orm"
)

type User struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

type UserSchema struct {
	ID     int64
	Name   string
	Age    int
	Schema sql.NullString `db:"schema_name"`
}

func (UserSchema) TableName() string { return "users" }

func rawQueryRow(t testing.TB, db *orm.DB, ctx context.Context, q string, args ...any) *sql.Row {
	t.Helper()
	row, err := db.RequireRawApproval("raw SQL test assertion").QueryRowE(ctx, q, args...)
	if err != nil {
		t.Fatalf("raw query row: %v", err)
	}
	return row
}

func setupDB(t testing.TB) *orm.DB {
	dsn, explicit := lookupTestDSN("TEST_MYSQL_DSN", defaultMySQLTestDSN)
	db := openTestDB(t, orm.MySQL, dsn, explicit)
	var err error
	stdDB := db.SQLDB()
	_, err = stdDB.Exec(`CREATE TABLE IF NOT EXISTS users (
          id BIGINT AUTO_INCREMENT PRIMARY KEY,
          name VARCHAR(64),
          age INT,
          schema_name VARCHAR(64),
          created_at DATETIME DEFAULT CURRENT_TIMESTAMP
  )`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = stdDB.Exec(`CREATE TABLE IF NOT EXISTS profiles (id BIGINT AUTO_INCREMENT PRIMARY KEY, user_id BIGINT, bio VARCHAR(255))`)
	if err != nil {
		t.Fatalf("create profiles table: %v", err)
	}
	_, err = stdDB.Exec("TRUNCATE TABLE users")
	if err != nil {
		t.Fatalf("truncate table: %v", err)
	}
	_, err = stdDB.Exec("TRUNCATE TABLE profiles")
	if err != nil {
		t.Fatalf("truncate profiles: %v", err)
	}
	_, err = stdDB.Exec("INSERT INTO users(name, age, schema_name, created_at) VALUES " +
		"('alice', 30, 'main', '2025-12-31 11:22:33')," +
		"('bob', 25, 'main', '2025-11-20 10:10:10')")
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}
	_, err = stdDB.Exec("INSERT INTO profiles(user_id, bio) VALUES (1, 'go developer'), (2, 'python developer')")
	if err != nil {
		t.Fatalf("insert profiles: %v", err)
	}
	return db
}

func TestFirstMap(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	var row map[string]any
	if err := db.Table("users").Where("id", 1).FirstMap(&row); err != nil {
		t.Fatalf("first map: %v", err)
	}
	if row["name"] != "alice" {
		t.Errorf("expected alice, got %v", row["name"])
	}
}

func TestGetStructs(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	var users []User
	if err := db.Model(&User{}).Where("age", ">", 20).OrderBy("id", "asc").Get(&users); err != nil {
		t.Fatalf("get structs: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "alice" || users[1].Name != "bob" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestGetStructsDBTag(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	var users []UserSchema
	if err := db.Model(&UserSchema{}).Where("age", ">", 20).OrderBy("id", "asc").Get(&users); err != nil {
		t.Fatalf("get structs: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if !users[0].Schema.Valid || users[0].Schema.String != "main" {
		t.Errorf("unexpected schema: %+v", users[0].Schema)
	}
}

func BenchmarkScannerMap(b *testing.B) {
	db := setupDB(b)
	defer db.Close()
	for i := 0; i < b.N; i++ {
		var row map[string]any
		if err := db.Table("users").Where("id", 1).FirstMap(&row); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScannerStruct(b *testing.B) {
	db := setupDB(b)
	defer db.Close()
	for i := 0; i < b.N; i++ {
		var user User
		if err := db.Model(&User{}).Where("id", 1).First(&user); err != nil {
			b.Fatal(err)
		}
	}
}

func TestInsert(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	if _, err := db.Table("users").Insert(map[string]any{"name": "charlie", "age": 40}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var row map[string]any
	if err := db.Table("users").Where("name", "charlie").FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["age"] != int64(40) {
		t.Errorf("expected age 40, got %v", row["age"])
	}
}

func TestInsertGetId(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	id, err := db.Table("users").InsertGetId(map[string]any{"name": "frank", "age": 28})
	if err != nil {
		t.Fatalf("insert get id: %v", err)
	}
	if id != 3 {
		t.Errorf("expected id 3, got %d", id)
	}
	var row map[string]any
	if err := db.Table("users").Where("id", id).FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["name"] != "frank" {
		t.Errorf("expected frank, got %v", row["name"])
	}
}

func TestInsertStructDBTag(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	u := UserSchema{Name: "greg", Age: 21, Schema: sql.NullString{String: "aux", Valid: true}}
	if _, err := db.Model(&UserSchema{}).Insert(u); err != nil {
		t.Fatalf("insert struct: %v", err)
	}
	var row map[string]any
	if err := db.Table("users").Where("name", "greg").FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["schema_name"] != "aux" {
		t.Errorf("expected schema aux, got %v", row["schema_name"])
	}
}

func TestUpdate(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	res, err := db.Table("users").Where("name", "bob").Update(map[string]any{"age": 35})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil {
		t.Fatalf("rows affected: %v", err)
	} else if aff != 1 {
		t.Errorf("expected 1 row affected, got %d", aff)
	}
	var row map[string]any
	if err := db.Table("users").Where("name", "bob").FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["age"] != int64(35) {
		t.Errorf("expected age 35, got %v", row["age"])
	}
}

func TestUpdateStruct(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	type upd struct{ Age int }
	if _, err := db.Table("users").Where("name", "bob").Update(upd{Age: 36}); err != nil {
		t.Fatalf("update struct: %v", err)
	}
	var row map[string]any
	if err := db.Table("users").Where("name", "bob").FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["age"] != int64(36) {
		t.Errorf("expected age 36, got %v", row["age"])
	}
}

func TestDelete(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	res, err := db.Table("users").Where("name", "alice").Delete()
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil {
		t.Fatalf("rows affected: %v", err)
	} else if aff != 1 {
		t.Errorf("expected 1 row affected, got %d", aff)
	}

	var row map[string]any
	err = db.Table("users").Where("name", "alice").FirstMap(&row)
	if err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}
