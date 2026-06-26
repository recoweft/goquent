package tests

import (
	"context"
	"errors"
	"testing"

	"github.com/recoweft/goquent/orm"
)

func TestExecAndQueryRow(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	rawDB := db.RequireRawApproval("raw SQL test fixture setup")

	_, err := rawDB.Exec("INSERT INTO users(name, age) VALUES(?, ?)", "greg", 55)
	if err != nil {
		t.Fatalf("exec raw: %v", err)
	}

	var name string
	row, err := rawDB.QueryRowE(context.Background(), "SELECT name FROM users WHERE name=?", "greg")
	if err != nil {
		t.Fatalf("queryrow policy: %v", err)
	}
	if err := row.Scan(&name); err != nil {
		t.Fatalf("queryrow: %v", err)
	}
	if name != "greg" {
		t.Errorf("expected greg, got %s", name)
	}

	rows, err := rawDB.Query("SELECT name FROM users ORDER BY id ASC")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		count++
	}
	if count == 0 {
		t.Error("expected rows from Query")
	}
}

func TestExecAndQueryRowContext(t *testing.T) {
	db := setupDB(t)
	defer db.Close()
	rawDB := db.RequireRawApproval("raw SQL test fixture update")

	ctx := context.Background()
	_, err := rawDB.ExecContext(ctx, "UPDATE users SET age=? WHERE name=?", 31, "alice")
	if err != nil {
		t.Fatalf("exec context: %v", err)
	}

	var age int
	row, err := rawDB.QueryRowE(ctx, "SELECT age FROM users WHERE name=?", "alice")
	if err != nil {
		t.Fatalf("queryrow context policy: %v", err)
	}
	if err := row.Scan(&age); err != nil {
		t.Fatalf("queryrow context: %v", err)
	}
	if age != 31 {
		t.Errorf("expected 31, got %d", age)
	}

	rows, err := rawDB.QueryContext(ctx, "SELECT id FROM users")
	if err != nil {
		t.Fatalf("query context: %v", err)
	}
	rows.Close()
}

func TestRawSQLRequiresApproval(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	if _, err := db.Exec("INSERT INTO users(name, age) VALUES(?, ?)", "blocked", 1); !errors.Is(err, orm.ErrApprovalRequired) {
		t.Fatalf("expected raw exec approval error, got %v", err)
	}
	if _, err := db.QueryRowE(context.Background(), "SELECT name FROM users WHERE id=?", 1); !errors.Is(err, orm.ErrApprovalRequired) {
		t.Fatalf("expected raw queryrow approval error, got %v", err)
	}
}

func TestCanceledContextErrors(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := db.ExecContext(ctx, "INSERT INTO users(name, age) VALUES('x',1)"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled exec, got %v", err)
	}

	if _, err := db.QueryContext(ctx, "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled query, got %v", err)
	}

	if _, err := db.QueryRowE(ctx, "SELECT 1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled queryrow, got %v", err)
	}
}
