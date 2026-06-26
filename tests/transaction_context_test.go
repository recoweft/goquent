package tests

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/recoweft/goquent/orm"
)

func TestTransactionContextCommit(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	err := db.TransactionContext(ctx, func(tx orm.Tx) error {
		_, err := tx.Table("users").Insert(map[string]any{"name": "ctx_dave", "age": 23})
		return err
	})
	if err != nil {
		t.Fatalf("transaction context commit: %v", err)
	}

	var row map[string]any
	if err := db.Table("users").Where("name", "ctx_dave").FirstMap(&row); err != nil {
		t.Fatalf("select after commit: %v", err)
	}
	if row["age"] != int64(23) {
		t.Errorf("expected age 23, got %v", row["age"])
	}
}

func TestTransactionContextRollback(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	err := db.TransactionContext(ctx, func(tx orm.Tx) error {
		if _, err := tx.Table("users").Insert(map[string]any{"name": "ctx_eve", "age": 51}); err != nil {
			return err
		}
		return fmt.Errorf("rollback")
	})
	if err == nil || err.Error() != "rollback" {
		t.Fatalf("expected rollback error, got %v", err)
	}

	var row map[string]any
	err = db.Table("users").Where("name", "ctx_eve").FirstMap(&row)
	if err != sql.ErrNoRows {
		t.Fatalf("expected no rows after rollback, got %v", err)
	}
}

func TestBeginTxCommit(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.Table("users").Insert(map[string]any{"name": "ctx_mallory", "age": 56}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var row map[string]any
	if err := db.Table("users").Where("name", "ctx_mallory").FirstMap(&row); err != nil {
		t.Fatalf("select after commit: %v", err)
	}
	if row["age"] != int64(56) {
		t.Errorf("expected age 56, got %v", row["age"])
	}
}

func TestBeginTxRollback(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if _, err := tx.Table("users").Insert(map[string]any{"name": "ctx_trent", "age": 45}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var row map[string]any
	err = db.Table("users").Where("name", "ctx_trent").FirstMap(&row)
	if err != sql.ErrNoRows {
		t.Fatalf("expected no rows after rollback, got %v", err)
	}
}

func TestBeginTxCanceled(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := db.BeginTx(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled begin tx, got %v", err)
	}
}
func TestBeginTxWithOptions(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	ctx := context.Background()
	opts := &sql.TxOptions{Isolation: sql.LevelSerializable}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		t.Fatalf("begin tx with opts: %v", err)
	}
	if _, err := tx.Table("users").Insert(map[string]any{"name": "ctx_opt", "age": 32}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var row map[string]any
	if err := db.Table("users").Where("name", "ctx_opt").FirstMap(&row); err != nil {
		t.Fatalf("select after commit: %v", err)
	}
	if row["age"] != int64(32) {
		t.Errorf("expected age 32, got %v", row["age"])
	}
}
