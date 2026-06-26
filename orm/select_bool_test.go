package orm

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

type boolRow struct {
	Nullable bool         `db:"nullable,boolstrict"`
	Unique   bool         `db:"unique"`
	Flag     sql.NullBool `db:"flag_s,boollenient"`
	Ptr      *bool        `db:"ptr"`
}

func newMockDB(t *testing.T, p BoolScanPolicy) (*DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ormDB := NewDB(db, driver.MySQLDialect{}, WithBoolScanPolicy(p))
	return ormDB.RequireRawApproval("raw bool scan test"), mock
}

func TestSelectBoolPolicies(t *testing.T) {
	ctx := context.Background()
	db, mock := newMockDB(t, BoolStrict)

	rows := sqlmock.NewRows([]string{"nullable", "unique", "flag_s", "ptr"}).AddRow(int64(1), int64(0), "2", int64(1))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	r, err := SelectOne[boolRow](ctx, db, "SELECT nullable, unique, flag_s, ptr FROM t")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if !r.Nullable || r.Unique {
		t.Fatalf("unexpected bool values: %+v", r)
	}
	if !r.Flag.Valid || !r.Flag.Bool {
		t.Fatalf("flag not true: %+v", r.Flag)
	}
	if r.Ptr == nil || !*r.Ptr {
		t.Fatalf("ptr bool wrong: %v", r.Ptr)
	}

	rows = sqlmock.NewRows([]string{"nullable", "unique", "flag_s", "ptr"}).AddRow(int64(2), int64(1), "t", nil)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	if _, err = SelectOne[boolRow](ctx, db, "SELECT nullable, unique, flag_s, ptr FROM t"); err == nil {
		t.Fatalf("expected error for strict value")
	}
}

func TestSelectBoolNilHandling(t *testing.T) {
	ctx := context.Background()
	db, mock := newMockDB(t, BoolCompat)

	rows := sqlmock.NewRows([]string{"nullable", "unique", "flag_s", "ptr"}).AddRow(int64(0), int64(1), nil, nil)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	r, err := SelectOne[boolRow](ctx, db, "SELECT nullable, unique, flag_s, ptr FROM t")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if r.Flag.Valid {
		t.Fatalf("flag should be invalid")
	}
	if r.Ptr != nil {
		t.Fatalf("ptr should be nil")
	}

	rows = sqlmock.NewRows([]string{"nullable", "unique", "flag_s", "ptr"}).AddRow(nil, int64(1), "t", nil)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)
	if _, err = SelectOne[boolRow](ctx, db, "SELECT nullable, unique, flag_s, ptr FROM t"); err == nil {
		t.Fatalf("expected error for nil bool")
	}
}
