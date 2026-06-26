package orm

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

type transactionHookUser struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func (transactionHookUser) TableName() string { return "users" }

type transactionHookAudit struct {
	ID     int64  `db:"id"`
	UserID int64  `db:"user_id"`
	Action string `db:"action"`
}

func (transactionHookAudit) TableName() string { return "audit_events" }

func TestRunTransactionWithHooksCommitsAuditHook(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db := NewDB(sqlDB, driver.MySQLDialect{})

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`id`, `name`) VALUES (?, ?)")).
		WithArgs(int64(1), "alice").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `audit_events`")).
		WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectCommit()

	row, err := RunTransactionWithHooks(
		context.Background(),
		db,
		TransactionWithHooksSpec[transactionHookUser]{
			Apply: func(ctx context.Context, tx Tx) (transactionHookUser, error) {
				user := transactionHookUser{ID: 1, Name: "alice"}
				_, err := Insert(ctx, tx.DB, user)
				return user, err
			},
			Hooks: []TransactionHook{
				InsertHook("audit", transactionHookAudit{ID: 10, UserID: 1, Action: "created"}),
			},
		},
	)
	if err != nil {
		t.Fatalf("run transaction with hooks: %v", err)
	}
	if row.ID != 1 || row.Name != "alice" {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRunTransactionWithHooksRollsBackOnHookError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db := NewDB(sqlDB, driver.MySQLDialect{})
	hookErr := errors.New("outbox unavailable")

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `users` (`id`, `name`) VALUES (?, ?)")).
		WithArgs(int64(1), "alice").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectRollback()

	_, err = RunTransactionWithHooks(
		context.Background(),
		db,
		TransactionWithHooksSpec[transactionHookUser]{
			Apply: func(ctx context.Context, tx Tx) (transactionHookUser, error) {
				user := transactionHookUser{ID: 1, Name: "alice"}
				_, err := Insert(ctx, tx.DB, user)
				return user, err
			},
			Hooks: []TransactionHook{
				NewTransactionHook("outbox", func(context.Context, Tx) error {
					return hookErr
				}),
			},
		},
	)
	if !errors.Is(err, hookErr) || !strings.Contains(err.Error(), "outbox") {
		t.Fatalf("expected wrapped outbox error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
