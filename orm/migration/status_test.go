package migration

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/faciam-dev/goquent/orm/driver"
)

func newStatusMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestReadStatusPostgresAppliedRowsAndPending(t *testing.T) {
	db, mock := newStatusMockDB(t)
	appliedAt := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT to_regclass($1) IS NOT NULL")).
		WithArgs("schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "version", "applied_at" FROM "schema_migrations" ORDER BY "version" ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "applied_at"}).
			AddRow("202605220001", appliedAt).
			AddRow("202605220002", appliedAt.Add(time.Minute)))

	status, err := ReadStatus(
		context.Background(),
		db,
		driver.PostgresDialect{},
		[]string{"202605220001", "202605220002", "202605220003"},
		WithStatusAppliedAtColumn("applied_at"),
	)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.Exists || status.LatestApplied != "202605220002" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(status.Applied) != 2 || status.Applied[0].AppliedAt == nil {
		t.Fatalf("expected applied timestamps: %+v", status.Applied)
	}
	if len(status.Pending) != 1 || status.Pending[0] != "202605220003" {
		t.Fatalf("unexpected pending: %+v", status.Pending)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReadStatusMissingTableReturnsPendingDesired(t *testing.T) {
	db, mock := newStatusMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT to_regclass($1) IS NOT NULL")).
		WithArgs("schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	status, err := ReadStatus(
		context.Background(),
		db,
		driver.PostgresDialect{},
		[]string{"001", "002"},
	)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if status.Exists {
		t.Fatalf("expected missing table: %+v", status)
	}
	if len(status.Pending) != 2 || status.Pending[0] != "001" || status.Pending[1] != "002" {
		t.Fatalf("unexpected pending: %+v", status.Pending)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReadStatusDirtyColumnDetection(t *testing.T) {
	db, mock := newStatusMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT to_regclass($1) IS NOT NULL")).
		WithArgs("schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "version", "dirty" FROM "schema_migrations" ORDER BY "version" ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"version", "dirty"}).
			AddRow("001", false).
			AddRow("002", true))

	status, err := ReadStatus(
		context.Background(),
		db,
		driver.PostgresDialect{},
		[]string{"001", "002"},
		WithStatusDirtyColumn("dirty"),
	)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.Dirty || !status.Applied[1].Dirty {
		t.Fatalf("expected dirty status: %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReadStatusWarnsOnExtraAppliedVersion(t *testing.T) {
	db, mock := newStatusMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT to_regclass($1) IS NOT NULL")).
		WithArgs("schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT "version" FROM "schema_migrations" ORDER BY "version" ASC`)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).
			AddRow("001").
			AddRow("999"))

	status, err := ReadStatus(
		context.Background(),
		db,
		driver.PostgresDialect{},
		[]string{"001"},
	)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.Unknown || len(status.Warnings) != 1 {
		t.Fatalf("expected unknown warning: %+v", status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReadStatusMySQLSchemaQualifiedTable(t *testing.T) {
	db, mock := newStatusMockDB(t)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?")).
		WithArgs("app", "schema_migrations").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `version` FROM `app`.`schema_migrations` ORDER BY `version` ASC")).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow("001"))

	status, err := ReadStatus(
		context.Background(),
		db,
		driver.MySQLDialect{},
		[]string{"001", "002"},
		WithStatusTable("app.schema_migrations"),
	)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !status.Exists || status.LatestApplied != "001" {
		t.Fatalf("unexpected mysql status: %+v", status)
	}
	if len(status.Pending) != 1 || status.Pending[0] != "002" {
		t.Fatalf("unexpected pending: %+v", status.Pending)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
