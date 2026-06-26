package migration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
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

func TestReadSchemaPostgres(t *testing.T) {
	db, mock := newStatusMockDB(t)
	mock.ExpectQuery("information_schema\\.columns").
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "data_type", "is_nullable", "column_default"}).
			AddRow("users", "id", "uuid", "NO", nil).
			AddRow("users", "email", "text", "NO", nil).
			AddRow("users", "created_at", "timestamp with time zone", "NO", "now()"))
	mock.ExpectQuery("pg_index").
		WithArgs("public").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "index_name", "indisunique", "indexdef"}).
			AddRow("users", "users_email_idx", true, "CREATE UNIQUE INDEX users_email_idx ON public.users USING btree (email)"))

	schema, err := ReadSchema(context.Background(), db, driver.PostgresDialect{}, WithSchemaReadSchema("public"))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "users" {
		t.Fatalf("unexpected schema tables: %+v", schema.Tables)
	}
	table := schema.Tables[0]
	if len(table.Columns) != 3 || table.Columns[2].DefaultExpression != "now()" {
		t.Fatalf("unexpected columns: %+v", table.Columns)
	}
	if len(table.Indexes) != 1 || !table.Indexes[0].Unique || table.Indexes[0].Columns[0] != "email" {
		t.Fatalf("unexpected indexes: %+v", table.Indexes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReadSchemaMySQLFiltersTables(t *testing.T) {
	db, mock := newStatusMockDB(t)
	mock.ExpectQuery("information_schema\\.columns").
		WithArgs("app", "app").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "is_nullable", "column_default"}).
			AddRow("users", "id", "bigint", "NO", nil).
			AddRow("users", "email", "varchar(255)", "NO", nil).
			AddRow("ignored", "id", "bigint", "NO", nil))
	mock.ExpectQuery("information_schema\\.statistics").
		WithArgs("app", "app").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "index_name", "non_unique", "seq_in_index", "column_name"}).
			AddRow("users", "users_email_idx", 0, 1, "email").
			AddRow("ignored", "ignored_idx", 1, 1, "id"))

	schema, err := ReadSchema(
		context.Background(),
		db,
		driver.MySQLDialect{},
		WithSchemaReadSchema("app"),
		WithSchemaReadTables("users"),
	)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if len(schema.Tables) != 1 || schema.Tables[0].Name != "users" {
		t.Fatalf("unexpected schema tables: %+v", schema.Tables)
	}
	if len(schema.Tables[0].Indexes) != 1 || schema.Tables[0].Indexes[0].Columns[0] != "email" {
		t.Fatalf("unexpected indexes: %+v", schema.Tables[0].Indexes)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestWriteSchemaPrettyAndJSON(t *testing.T) {
	schema := Schema{Tables: []TableSchema{{
		Name:    "users",
		Columns: []ColumnSchema{{Name: "id", Type: "uuid"}},
	}}}
	var pretty bytes.Buffer
	if err := WriteSchemaPretty(&pretty, schema); err != nil {
		t.Fatalf("pretty schema: %v", err)
	}
	if !strings.Contains(pretty.String(), "Schema Export") || !strings.Contains(pretty.String(), "users") {
		t.Fatalf("unexpected pretty output: %s", pretty.String())
	}
	var jsonBuf bytes.Buffer
	if err := WriteSchemaJSON(&jsonBuf, schema); err != nil {
		t.Fatalf("json schema: %v", err)
	}
	var decoded Schema
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode schema json: %v", err)
	}
	if len(decoded.Tables) != 1 || decoded.Tables[0].Name != "users" {
		t.Fatalf("unexpected json schema: %+v", decoded)
	}
}

func TestWriteStatusPrettyAndJSON(t *testing.T) {
	status := Status{
		Table:         "schema_migrations",
		Exists:        true,
		LatestApplied: "001",
		Pending:       []string{"002"},
		Warnings:      []string{"extra migration"},
		Unknown:       true,
	}
	var pretty bytes.Buffer
	if err := WriteStatusPretty(&pretty, status); err != nil {
		t.Fatalf("pretty status: %v", err)
	}
	if !strings.Contains(pretty.String(), "Migration Status") || !strings.Contains(pretty.String(), "pending:") {
		t.Fatalf("unexpected pretty output: %s", pretty.String())
	}
	var jsonBuf bytes.Buffer
	if err := WriteStatusJSON(&jsonBuf, status); err != nil {
		t.Fatalf("json status: %v", err)
	}
	var decoded Status
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode status json: %v", err)
	}
	if decoded.Table != status.Table || len(decoded.Pending) != 1 {
		t.Fatalf("unexpected json status: %+v", decoded)
	}
}
