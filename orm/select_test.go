package orm

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSelectOneReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	db, mock := newReturningMockDB(t)

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}))

	_, err := SelectOne[genericWriteUser](ctx, db.RequireRawApproval("not found test"), "SELECT id, name, age FROM users WHERE id = $1", 42)
	if !errors.Is(err, ErrNotFound) || !IsNotFound(err) {
		t.Fatalf("expected not found error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
