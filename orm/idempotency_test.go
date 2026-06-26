package orm

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

type idempotentCommandRow struct {
	ID     int64
	Status string
}

func newIdempotencyMockDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewDB(sqlDB, driver.PostgresDialect{}), mock
}

func TestRunIdempotentCommandReturnsExistingWithoutTransaction(t *testing.T) {
	db, mock := newIdempotencyMockDB(t)
	lookupCalls := 0
	applyCalls := 0

	result, err := RunIdempotentCommand(
		context.Background(),
		db,
		IdempotentCommandSpec[idempotentCommandRow]{
			LookupExisting: func(context.Context, *DB) (idempotentCommandRow, error) {
				lookupCalls++
				return idempotentCommandRow{ID: 7, Status: "existing"}, nil
			},
			Apply: func(context.Context, Tx) (idempotentCommandRow, error) {
				applyCalls++
				return idempotentCommandRow{}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("run idempotent command: %v", err)
	}
	if result.Applied || result.Value.Status != "existing" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if lookupCalls != 1 || applyCalls != 0 {
		t.Fatalf("lookup=%d apply=%d", lookupCalls, applyCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRunIdempotentCommandAppliesInTransaction(t *testing.T) {
	db, mock := newIdempotencyMockDB(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	result, err := RunIdempotentCommand(
		context.Background(),
		db,
		IdempotentCommandSpec[idempotentCommandRow]{
			LookupExisting: func(context.Context, *DB) (idempotentCommandRow, error) {
				return idempotentCommandRow{}, sql.ErrNoRows
			},
			Apply: func(_ context.Context, tx Tx) (idempotentCommandRow, error) {
				if tx.DB == nil || tx.Tx.Tx == nil {
					t.Fatalf("expected transaction-bound DB")
				}
				return idempotentCommandRow{ID: 8, Status: "applied"}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("run idempotent command: %v", err)
	}
	if !result.Applied || result.Value.Status != "applied" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRunIdempotentCommandLooksUpAfterConflict(t *testing.T) {
	db, mock := newIdempotencyMockDB(t)
	mock.ExpectBegin()
	mock.ExpectRollback()
	lookupCalls := 0
	conflictLookupCalls := 0

	result, err := RunIdempotentCommand(
		context.Background(),
		db,
		IdempotentCommandSpec[idempotentCommandRow]{
			LookupExisting: func(context.Context, *DB) (idempotentCommandRow, error) {
				lookupCalls++
				return idempotentCommandRow{}, sql.ErrNoRows
			},
			Apply: func(context.Context, Tx) (idempotentCommandRow, error) {
				return idempotentCommandRow{}, ErrConflict
			},
			LookupAfterConflict: func(context.Context, *DB) (idempotentCommandRow, error) {
				conflictLookupCalls++
				return idempotentCommandRow{ID: 9, Status: "replayed"}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("run idempotent command: %v", err)
	}
	if result.Applied || result.Value.Status != "replayed" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if lookupCalls != 1 || conflictLookupCalls != 1 {
		t.Fatalf("lookup=%d conflictLookup=%d", lookupCalls, conflictLookupCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRunIdempotentCommandKeepsConflictWhenReplayMissing(t *testing.T) {
	db, mock := newIdempotencyMockDB(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := RunIdempotentCommand(
		context.Background(),
		db,
		IdempotentCommandSpec[idempotentCommandRow]{
			LookupExisting: func(context.Context, *DB) (idempotentCommandRow, error) {
				return idempotentCommandRow{}, sql.ErrNoRows
			},
			Apply: func(context.Context, Tx) (idempotentCommandRow, error) {
				return idempotentCommandRow{}, ErrConflict
			},
		},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
