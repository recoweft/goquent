package tests

import (
	"bytes"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock" // TODO: consider removing external mock library

	"github.com/recoweft/goquent/orm/scanner"
)

func TestMapConvertsBytes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	m, err := scanner.Map(r)
	if err != nil {
		t.Fatalf("scan map: %v", err)
	}
	if m["name"] != "alice" {
		t.Errorf("expected alice, got %v", m["name"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestMapsConvertsBytes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "alice").
		AddRow(2, "bob")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	ms, err := scanner.Maps(r)
	if err != nil {
		t.Fatalf("scan maps: %v", err)
	}
	if len(ms) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(ms))
	}
	if ms[0]["name"] != "alice" || ms[1]["name"] != "bob" {
		t.Errorf("unexpected rows: %v", ms)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructsHandlesID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT id, name FROM users")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	var users []struct {
		ID   int64
		Name string
	}
	if err := scanner.Structs(&users, r); err != nil {
		t.Fatalf("scan structs: %v", err)
	}
	if len(users) != 1 || users[0].ID != 1 {
		t.Errorf("unexpected users: %+v", users)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructsDBTag(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"user_id"}).AddRow(2)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT user_id FROM users")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	var users []struct {
		ID int `db:"user_id"`
	}
	if err := scanner.Structs(&users, r); err != nil {
		t.Fatalf("scan structs: %v", err)
	}
	if len(users) != 1 || users[0].ID != 2 {
		t.Errorf("unexpected users: %+v", users)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructsDBTagOptions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(7)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT id FROM users")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	var users []struct {
		ID int64 `db:"id,pk"`
	}
	if err := scanner.Structs(&users, r); err != nil {
		t.Fatalf("scan structs: %v", err)
	}
	if len(users) != 1 || users[0].ID != 7 {
		t.Errorf("unexpected users: %+v", users)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructsNestedDBPrefix(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"user_id", "user_name", "profile_id", "profile_bio"}).
		AddRow(1, "alice", 10, "go developer").
		AddRow(2, "bob", nil, nil)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT joined rows")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	type userRow struct {
		ID   int64  `db:"id,pk"`
		Name string `db:"name"`
	}
	type profileRow struct {
		ID  int64          `db:"id"`
		Bio sql.NullString `db:"bio"`
	}
	var rowsOut []struct {
		User    userRow     `db:"user,prefix"`
		Profile *profileRow `db:"profile,prefix"`
	}
	if err := scanner.Structs(&rowsOut, r); err != nil {
		t.Fatalf("scan structs: %v", err)
	}
	if len(rowsOut) != 2 {
		t.Fatalf("expected two rows, got %+v", rowsOut)
	}
	if rowsOut[0].User.ID != 1 || rowsOut[0].User.Name != "alice" {
		t.Fatalf("unexpected first user: %+v", rowsOut[0].User)
	}
	if rowsOut[0].Profile == nil || rowsOut[0].Profile.ID != 10 || rowsOut[0].Profile.Bio.String != "go developer" {
		t.Fatalf("unexpected first profile: %+v", rowsOut[0].Profile)
	}
	if rowsOut[1].User.ID != 2 || rowsOut[1].User.Name != "bob" {
		t.Fatalf("unexpected second user: %+v", rowsOut[1].User)
	}
	if rowsOut[1].Profile != nil {
		t.Fatalf("expected nil profile for left join miss, got %+v", rowsOut[1].Profile)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructsDBRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	enc := []byte{0xde, 0xad}
	rows := sqlmock.NewRows([]string{"id", "driver", "dsn", "schema_name", "dsn_enc"}).
		AddRow(3, "mysql", "root:pass@tcp(localhost:3306)/db", "main", enc)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT id, driver, dsn, schema_name, dsn_enc FROM db_records")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	type dbRecord struct {
		ID     int64
		Driver string
		DSN    string
		Schema sql.NullString `db:"schema_name"`
		DSNEnc []byte         `db:"dsn_enc"`
	}
	var recs []dbRecord
	if err := scanner.Structs(&recs, r); err != nil {
		t.Fatalf("scan structs: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(recs))
	}
	rec := recs[0]
	if rec.ID != 3 || rec.Driver != "mysql" || rec.DSN != "root:pass@tcp(localhost:3306)/db" {
		t.Errorf("unexpected record values: %+v", rec)
	}
	if !rec.Schema.Valid || rec.Schema.String != "main" {
		t.Errorf("unexpected schema: %+v", rec.Schema)
	}
	if !bytes.Equal(rec.DSNEnc, enc) {
		t.Errorf("unexpected dsn_enc: %v", rec.DSNEnc)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructBoolFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"nullable", "flag", "ptr"}).AddRow(int64(2), int64(0), int64(-3))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT nullable, flag, ptr FROM t")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	var out struct {
		Nullable bool
		Flag     sql.NullBool
		Ptr      *bool
	}
	if err := scanner.Struct(&out, r); err != nil {
		t.Fatalf("scan struct: %v", err)
	}
	if !out.Nullable {
		t.Errorf("nullable not true: %v", out.Nullable)
	}
	if !out.Flag.Valid || out.Flag.Bool {
		t.Errorf("flag not false: %+v", out.Flag)
	}
	if out.Ptr == nil || !*out.Ptr {
		t.Errorf("ptr not true: %v", out.Ptr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructBoolFieldsBytes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"nullable", "flag", "ptr"}).AddRow([]byte(" TrUe "), []byte("0"), []byte("t"))
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT nullable, flag, ptr FROM t")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	var out struct {
		Nullable bool
		Flag     sql.NullBool
		Ptr      *bool
	}
	if err := scanner.Struct(&out, r); err != nil {
		t.Fatalf("scan struct: %v", err)
	}
	if !out.Nullable {
		t.Errorf("nullable not true: %v", out.Nullable)
	}
	if !out.Flag.Valid || out.Flag.Bool {
		t.Errorf("flag not false: %+v", out.Flag)
	}
	if out.Ptr == nil || !*out.Ptr {
		t.Errorf("ptr not true: %v", out.Ptr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStructBoolFieldsNil(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"nullable", "flag", "ptr"}).AddRow(nil, nil, nil)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	r, err := db.Query("SELECT nullable, flag, ptr FROM t")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer r.Close()

	var out struct {
		Nullable bool
		Flag     sql.NullBool
		Ptr      *bool
	}
	if err := scanner.Struct(&out, r); err != nil {
		t.Fatalf("scan struct: %v", err)
	}
	if out.Nullable {
		t.Errorf("nullable expected false: %v", out.Nullable)
	}
	if out.Flag.Valid {
		t.Errorf("flag expected invalid: %+v", out.Flag)
	}
	if out.Ptr != nil {
		t.Errorf("ptr expected nil: %v", out.Ptr)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
