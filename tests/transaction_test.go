package tests

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/recoweft/goquent/orm"
)

func TestTransactionCommit(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	err := db.Transaction(func(tx orm.Tx) error {
		_, err := tx.Table("users").Insert(map[string]any{"name": "dave", "age": 22})
		return err
	})
	if err != nil {
		t.Fatalf("transaction commit: %v", err)
	}

	var row map[string]any
	if err := db.Table("users").Where("name", "dave").FirstMap(&row); err != nil {
		t.Fatalf("select after commit: %v", err)
	}
	if row["age"] != int64(22) {
		t.Errorf("expected age 22, got %v", row["age"])
	}
}

func TestTransactionRollback(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	err := db.Transaction(func(tx orm.Tx) error {
		if _, err := tx.Table("users").Insert(map[string]any{"name": "eve", "age": 50}); err != nil {
			return err
		}
		return fmt.Errorf("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback error, got %v", err)
	}

	var row map[string]any
	err = db.Table("users").Where("name", "eve").FirstMap(&row)
	if err != sql.ErrNoRows {
		t.Fatalf("expected no rows after rollback, got %v", err)
	}
}
