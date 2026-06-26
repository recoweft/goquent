package tests

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/recoweft/goquent/orm"
)

func TestSelectAllByWithinTransactionRollback(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	err := db.Transaction(func(tx orm.Tx) error {
		if _, err := tx.Table("users").Insert(map[string]any{"name": "txn_scope", "age": 38}); err != nil {
			return err
		}
		if _, err := tx.Table("profiles").Insert(map[string]any{"user_id": 3, "bio": "rust developer"}); err != nil {
			return err
		}

		users, err := orm.SelectAllBy[User](
			ctx,
			tx.DB,
			tx.Model(&User{}),
			orm.ComposeScopes(profileJoinScope(), bioFilterScope("%rust%")),
			orderedUserSelectScope(),
		)
		if err != nil {
			return err
		}
		if len(users) != 1 || users[0].Name != "txn_scope" {
			t.Fatalf("unexpected users in tx: %+v", users)
		}
		return errors.New("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback error, got %v", err)
	}

	var row map[string]any
	err = db.Table("users").Where("name", "txn_scope").FirstMap(&row)
	if err != sql.ErrNoRows {
		t.Fatalf("expected rollback to remove row, got %v", err)
	}
}

func TestUpdateByWithinTransactionCommit(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	err := db.Transaction(func(tx orm.Tx) error {
		_, err := orm.UpdateBy(
			context.Background(),
			tx.Table("users"),
			map[string]any{"age": 52},
			profileJoinScope(),
			bioFilterScope("%go%"),
		)
		return err
	})
	if err != nil {
		t.Fatalf("transaction update by: %v", err)
	}

	var age int
	if err := rawQueryRow(t, db, context.Background(), "SELECT age FROM users WHERE id = ?", 1).Scan(&age); err != nil {
		t.Fatalf("select after commit: %v", err)
	}
	if age != 52 {
		t.Fatalf("expected age 52, got %d", age)
	}
}

func TestDeleteByWithinTransactionRollback(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	err := db.Transaction(func(tx orm.Tx) error {
		if _, err := orm.DeleteBy(
			context.Background(),
			tx.Table("users"),
			profileJoinScope(),
			bioFilterScope("%python%"),
		); err != nil {
			return err
		}

		var row map[string]any
		err := tx.Table("users").Where("id", 2).FirstMap(&row)
		if err != sql.ErrNoRows {
			t.Fatalf("expected no row inside tx, got %v", err)
		}
		return errors.New("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback error, got %v", err)
	}

	var row map[string]any
	if err := db.Table("users").Where("id", 2).FirstMap(&row); err != nil {
		t.Fatalf("select after rollback: %v", err)
	}
	if row["name"] != "bob" {
		t.Fatalf("expected bob after rollback, got %v", row["name"])
	}
}
