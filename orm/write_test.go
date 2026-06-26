package orm

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

type capturedStatement struct {
	query string
	args  []any
}

type captureExecutor struct {
	query           string
	args            []any
	statements      []capturedStatement
	lastInsertID    int64
	rowsAffected    int64
	rowsAffectedSet bool
}

func (e *captureExecutor) Query(string, ...any) (*sql.Rows, error) { return nil, nil }

func (e *captureExecutor) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}

func (e *captureExecutor) QueryRow(string, ...any) *sql.Row { return nil }

func (e *captureExecutor) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func (e *captureExecutor) Exec(query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = append([]any(nil), args...)
	e.statements = append(e.statements, capturedStatement{query: query, args: append([]any(nil), args...)})
	return captureResult{lastInsertID: e.lastInsertID, rowsAffected: e.rowsAffected, rowsAffectedSet: e.rowsAffectedSet}, nil
}

func (e *captureExecutor) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = append([]any(nil), args...)
	e.statements = append(e.statements, capturedStatement{query: query, args: append([]any(nil), args...)})
	return captureResult{lastInsertID: e.lastInsertID, rowsAffected: e.rowsAffected, rowsAffectedSet: e.rowsAffectedSet}, nil
}

type captureResult struct {
	lastInsertID    int64
	rowsAffected    int64
	rowsAffectedSet bool
}

func (r captureResult) LastInsertId() (int64, error) { return r.lastInsertID, nil }

func (r captureResult) RowsAffected() (int64, error) {
	if !r.rowsAffectedSet {
		return 1, nil
	}
	return r.rowsAffected, nil
}

func newCaptureWriteDB(d driver.Dialect) (*DB, *captureExecutor) {
	exec := &captureExecutor{}
	return &DB{
		drv:      &driver.Driver{Dialect: d},
		exec:     exec,
		scanOpts: ScanOptions{BoolPolicy: BoolCompat},
	}, exec
}

func newReturningMockDB(t *testing.T) (*DB, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewDB(sqlDB, driver.PostgresDialect{}), mock
}

type genericWriteUser struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func (genericWriteUser) TableName() string { return "users" }

type nestedDocumentTable struct {
	ID       int64  `db:"id,pk"`
	TenantID string `db:"tenant_id"`
	Title    string `db:"title"`
	Revision int64  `db:"revision"`
}

func (nestedDocumentTable) TableName() string { return "document_tables" }

type nestedDocumentRow struct {
	ID           int64  `db:"id,pk,omitempty"`
	TenantID     string `db:"tenant_id"`
	TableID      int64  `db:"table_id"`
	StableRowKey string `db:"stable_row_key"`
	Position     int    `db:"position"`
}

func (nestedDocumentRow) TableName() string { return "document_table_rows" }

type nestedDocumentCell struct {
	TenantID      string `db:"tenant_id"`
	TableID       int64  `db:"table_id"`
	RowID         int64  `db:"row_id"`
	StableCellKey string `db:"stable_cell_key"`
	ValueJSON     string `db:"value_json"`
}

func (nestedDocumentCell) TableName() string { return "document_table_cells" }

func nestedWhere(col string, value any) Scope {
	return func(q *query.Query) *query.Query {
		return q.Where(col, value)
	}
}

func hasArg(args []any, want any) bool {
	for _, arg := range args {
		if reflect.DeepEqual(arg, want) {
			return true
		}
	}
	return false
}

func TestUpsertStructAlwaysIncludesPKColumn(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		genericWriteUser{ID: 7, Name: "alice"},
		Columns("name"),
		Omit("id"),
		WherePK(),
	)
	if err != nil {
		t.Fatalf("upsert struct: %v", err)
	}

	if !strings.Contains(exec.query, "INSERT INTO `users`") {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if !strings.Contains(exec.query, "`id`") {
		t.Fatalf("expected pk column to stay in insert query, got: %s", exec.query)
	}
	if !hasArg(exec.args, int64(7)) {
		t.Fatalf("expected pk value in args, got: %#v", exec.args)
	}
}

func TestUpsertMapAlwaysIncludesPKColumn(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{"id": int64(9), "name": "bob"},
		Table("users"),
		PK("id"),
		Columns("name"),
		Omit("id"),
		WherePK(),
	)
	if err != nil {
		t.Fatalf("upsert map: %v", err)
	}

	if !strings.Contains(exec.query, "INSERT INTO `users`") {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if !strings.Contains(exec.query, "`id`") {
		t.Fatalf("expected pk column to stay in insert query, got: %s", exec.query)
	}
	if !hasArg(exec.args, int64(9)) {
		t.Fatalf("expected pk value in args, got: %#v", exec.args)
	}
}

func TestUpdateStructUsesSetArgsBeforePKArgs(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := Update(
		context.Background(),
		db,
		genericWriteUser{ID: 3, Name: "alice"},
		Columns("name"),
		WherePK(),
	)
	if err != nil {
		t.Fatalf("update struct: %v", err)
	}

	if !strings.Contains(exec.query, "SET `name`=? WHERE `id`=?") {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if len(exec.args) != 2 || exec.args[0] != "alice" || exec.args[1] != int64(3) {
		t.Fatalf("unexpected arg order: %#v", exec.args)
	}
}

func TestUpdateMapUsesSetArgsBeforePKArgs(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := Update(
		context.Background(),
		db,
		map[string]any{"id": int64(4), "name": "bob"},
		Table("users"),
		PK("id"),
		Columns("name"),
		WherePK(),
	)
	if err != nil {
		t.Fatalf("update map: %v", err)
	}

	if !strings.Contains(exec.query, "SET `name`=? WHERE `id`=?") {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if len(exec.args) != 2 || exec.args[0] != "bob" || exec.args[1] != int64(4) {
		t.Fatalf("unexpected arg order: %#v", exec.args)
	}
}

func TestUpdateExpectAffectedMapsZeroRowsToConflict(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	exec.rowsAffected = 0
	exec.rowsAffectedSet = true

	_, err := Update(
		context.Background(),
		db,
		genericWriteUser{ID: 3, Name: "alice"},
		Columns("name"),
		WherePK(),
		ExpectAffected(1),
		NoRowsAs(ErrConflict),
	)
	if !errors.Is(err, ErrConflict) || !IsConflict(err) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	var affected RowsAffectedError
	if !errors.As(err, &affected) || affected.Expected != 1 || affected.Actual != 0 {
		t.Fatalf("expected rows affected details, got %#v", err)
	}
}

func TestUpdateExpectAffectedMismatch(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	exec.rowsAffected = 2
	exec.rowsAffectedSet = true

	_, err := Update(
		context.Background(),
		db,
		genericWriteUser{ID: 3, Name: "alice"},
		Columns("name"),
		WherePK(),
		ExpectAffected(1),
	)
	if !errors.Is(err, ErrRowsAffected) {
		t.Fatalf("expected rows affected error, got %v", err)
	}
}

func TestInsertReturningPostgresAddsClause(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*RETURNING "id", "name"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice"))

	res, err := Insert(
		context.Background(),
		db,
		genericWriteUser{Name: "alice"},
		Columns("name"),
		Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("insert returning: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil || aff != 1 {
		t.Fatalf("expected rows affected 1, got %d err=%v", aff, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInsertReturningTypedInfersColumns(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*RETURNING "id", "name", "age"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(1, "alice", 30))

	row, err := InsertReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{Name: "alice"},
		Columns("name"),
	)
	if err != nil {
		t.Fatalf("insert returning typed: %v", err)
	}
	if row.ID != 1 || row.Name != "alice" || row.Age != 30 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInsertReturningMapUsesExplicitColumns(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*RETURNING "id", "name"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "alice"))

	row, err := InsertReturning[map[string]any](
		context.Background(),
		db,
		map[string]any{"name": "alice"},
		Table("users"),
		Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("insert returning map: %v", err)
	}
	if row["id"] != int64(1) || row["name"] != "alice" {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInsertManyStructBuildsSingleStatement(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]genericWriteUser{
			{Name: "alice", Age: 30},
			{Name: "bob", Age: 31},
		},
		Columns("name", "age"),
	)
	if err != nil {
		t.Fatalf("insert many struct: %v", err)
	}
	wantSQL := "INSERT INTO `users` (`name`, `age`) VALUES (?, ?), (?, ?)"
	if exec.query != wantSQL {
		t.Fatalf("unexpected query:\nwant %s\ngot  %s", wantSQL, exec.query)
	}
	wantArgs := []any{"alice", 30, "bob", 31}
	if !reflect.DeepEqual(exec.args, wantArgs) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestInsertManyMapBuildsSingleStatement(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]map[string]any{
			{"name": "alice", "age": 30},
			{"name": "bob", "age": 31},
		},
		Table("users"),
	)
	if err != nil {
		t.Fatalf("insert many map: %v", err)
	}
	wantSQL := "INSERT INTO `users` (`age`, `name`) VALUES (?, ?), (?, ?)"
	if exec.query != wantSQL {
		t.Fatalf("unexpected query:\nwant %s\ngot  %s", wantSQL, exec.query)
	}
	wantArgs := []any{30, "alice", 31, "bob"}
	if !reflect.DeepEqual(exec.args, wantArgs) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestInsertManyPostgresPlaceholderNumbering(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]genericWriteUser{
			{Name: "alice", Age: 30},
			{Name: "bob", Age: 31},
		},
		Columns("name", "age"),
	)
	if err != nil {
		t.Fatalf("insert many postgres: %v", err)
	}
	wantSQL := `INSERT INTO "users" ("name", "age") VALUES ($1, $2), ($3, $4)`
	if exec.query != wantSQL {
		t.Fatalf("unexpected query:\nwant %s\ngot  %s", wantSQL, exec.query)
	}
}

func TestInsertManyTablePathOption(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]map[string]any{
			{"name": "alice"},
			{"name": "bob"},
		},
		TablePath("app", "users"),
	)
	if err != nil {
		t.Fatalf("insert many table path: %v", err)
	}
	if !strings.Contains(exec.query, `INSERT INTO "app"."users"`) {
		t.Fatalf("expected table path, got: %s", exec.query)
	}
}

func TestInsertManyEmptySliceReturnsError(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertMany[genericWriteUser](context.Background(), db, nil)
	if err == nil || !strings.Contains(err.Error(), "goquent: no rows to insert") {
		t.Fatalf("expected no rows error, got: %v", err)
	}
}

func TestInsertManyMapRejectsInconsistentKeys(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]map[string]any{
			{"name": "alice"},
			{"name": "bob", "age": 31},
		},
		Table("users"),
	)
	if err == nil || !strings.Contains(err.Error(), "inconsistent columns") {
		t.Fatalf("expected inconsistent map columns error, got: %v", err)
	}
}

func TestInsertManyReturningTypedStructScan(t *testing.T) {
	db, mock := newReturningMockDB(t)
	sqlText := `INSERT INTO "users" ("name", "age") VALUES ($1, $2), ($3, $4) RETURNING "id", "name", "age"`
	mock.ExpectQuery(regexp.QuoteMeta(sqlText)).
		WithArgs("alice", 30, "bob", 31).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).
			AddRow(1, "alice", 30).
			AddRow(2, "bob", 31))

	rows, err := InsertManyReturning[genericWriteUser](
		context.Background(),
		db,
		[]genericWriteUser{
			{Name: "alice", Age: 30},
			{Name: "bob", Age: 31},
		},
		Columns("name", "age"),
	)
	if err != nil {
		t.Fatalf("insert many returning: %v", err)
	}
	if len(rows) != 2 || rows[0].ID != 1 || rows[1].Name != "bob" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInsertManyReturningMapRequiresReturning(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertManyReturning[map[string]any](
		context.Background(),
		db,
		[]map[string]any{{"name": "alice"}},
		Table("users"),
	)
	if err == nil || !strings.Contains(err.Error(), "Returning columns are required for map return values") {
		t.Fatalf("expected returning columns error, got: %v", err)
	}
}

func TestInsertManyRejectsAssignmentOptions(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]map[string]any{{"name": "alice"}},
		Table("users"),
		SetRaw("updated_at", "now()"),
	)
	if err == nil || !strings.Contains(err.Error(), "assignment options are not supported for InsertMany") {
		t.Fatalf("expected assignment insert many error, got: %v", err)
	}
}

func TestInsertManyRejectsConflictOptions(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := InsertMany(
		context.Background(),
		db,
		[]map[string]any{{"id": 1, "name": "alice"}},
		Table("users"),
		ConflictColumns("id"),
	)
	if err == nil || !strings.Contains(err.Error(), "conflict/upsert options are not supported for InsertMany") {
		t.Fatalf("expected conflict insert many error, got: %v", err)
	}
}

func TestUpsertManyPostgresBuildsSingleStatement(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := UpsertMany(
		context.Background(),
		db,
		[]map[string]any{
			{
				"tenant_id":  "tenant-1",
				"field_key":  "weekly_hours",
				"value_json": "{}",
				"updated_at": "now",
			},
			{
				"tenant_id":  "tenant-1",
				"field_key":  "overtime_hours",
				"value_json": "{}",
				"updated_at": "later",
			},
		},
		Table("profile_values"),
		ConflictColumns("tenant_id", "field_key"),
		UpdateColumns("value_json", "updated_at"),
	)
	if err != nil {
		t.Fatalf("upsert many postgres: %v", err)
	}
	wantSQL := `INSERT INTO "profile_values" ("field_key", "tenant_id", "updated_at", "value_json") VALUES ($1, $2, $3, $4), ($5, $6, $7, $8) ON CONFLICT ("tenant_id", "field_key") DO UPDATE SET "value_json"=EXCLUDED."value_json", "updated_at"=EXCLUDED."updated_at"`
	if exec.query != wantSQL {
		t.Fatalf("unexpected query:\nwant %s\ngot  %s", wantSQL, exec.query)
	}
	wantArgs := []any{"weekly_hours", "tenant-1", "now", "{}", "overtime_hours", "tenant-1", "later", "{}"}
	if !reflect.DeepEqual(exec.args, wantArgs) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestUpsertManyStructKeepsPKWhenFiltered(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := UpsertMany(
		context.Background(),
		db,
		[]genericWriteUser{
			{ID: 1, Name: "alice", Age: 30},
			{ID: 2, Name: "bob", Age: 31},
		},
		Columns("name"),
		Omit("id"),
		WherePK(),
	)
	if err != nil {
		t.Fatalf("upsert many struct: %v", err)
	}
	wantSQL := "INSERT INTO `users` (`id`, `name`) VALUES (?, ?), (?, ?) ON DUPLICATE KEY UPDATE `name`=VALUES(`name`)"
	if exec.query != wantSQL {
		t.Fatalf("unexpected query:\nwant %s\ngot  %s", wantSQL, exec.query)
	}
	wantArgs := []any{int64(1), "alice", int64(2), "bob"}
	if !reflect.DeepEqual(exec.args, wantArgs) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestUpsertManyConflictDoNothing(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := UpsertMany(
		context.Background(),
		db,
		[]map[string]any{
			{"tenant_id": "tenant-1", "idempotency_key": "idem-1", "payload_json": "{}"},
			{"tenant_id": "tenant-1", "idempotency_key": "idem-2", "payload_json": "{}"},
		},
		Table("submission_attempts"),
		ConflictColumns("tenant_id", "idempotency_key"),
		ConflictDoNothing(),
	)
	if err != nil {
		t.Fatalf("upsert many do nothing: %v", err)
	}
	if !strings.Contains(exec.query, `ON CONFLICT ("tenant_id", "idempotency_key") DO NOTHING`) {
		t.Fatalf("expected DO NOTHING conflict action, got: %s", exec.query)
	}
	if strings.Contains(exec.query, "DO UPDATE") {
		t.Fatalf("expected no conflict update, got: %s", exec.query)
	}
}

func TestUpsertManyConflictDoNothingRejectsUpdateColumns(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := UpsertMany(
		context.Background(),
		db,
		[]map[string]any{{"id": int64(9), "name": "alice"}},
		Table("users"),
		ConflictColumns("id"),
		ConflictDoNothing(),
		UpdateColumns("name"),
	)
	if err == nil || !strings.Contains(err.Error(), "ConflictDoNothing cannot be combined") {
		t.Fatalf("expected do-nothing update error, got: %v", err)
	}
}

func TestUpsertManyReturningTypedStructScan(t *testing.T) {
	db, mock := newReturningMockDB(t)
	sqlText := `INSERT INTO "users" ("id", "name", "age") VALUES ($1, $2, $3), ($4, $5, $6) ON CONFLICT ("id") DO UPDATE SET "name"=EXCLUDED."name", "age"=EXCLUDED."age" RETURNING "id", "name", "age"`
	mock.ExpectQuery(regexp.QuoteMeta(sqlText)).
		WithArgs(int64(1), "alice", 30, int64(2), "bob", 31).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).
			AddRow(1, "alice", 30).
			AddRow(2, "bob", 31))

	rows, err := UpsertManyReturning[genericWriteUser](
		context.Background(),
		db,
		[]genericWriteUser{
			{ID: 1, Name: "alice", Age: 30},
			{ID: 2, Name: "bob", Age: 31},
		},
		WherePK(),
	)
	if err != nil {
		t.Fatalf("upsert many returning: %v", err)
	}
	if len(rows) != 2 || rows[0].ID != 1 || rows[1].Name != "bob" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpsertManyRejectsInconsistentColumns(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := UpsertMany(
		context.Background(),
		db,
		[]map[string]any{
			{"tenant_id": "tenant-1", "field_key": "weekly_hours", "value_json": "{}"},
			{"tenant_id": "tenant-1", "field_key": "overtime_hours", "value_json": "{}", "extra": true},
		},
		Table("profile_values"),
		ConflictColumns("tenant_id", "field_key"),
	)
	if err == nil || !strings.Contains(err.Error(), "inconsistent columns") {
		t.Fatalf("expected inconsistent columns error, got: %v", err)
	}
}

func TestReplaceNestedCollectionMySQLBuildsOrderedPlan(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	exec.lastInsertID = 101

	tenantID := "tenant-1"
	tableID := int64(7)
	rows := []nestedDocumentRow{
		{TenantID: tenantID, TableID: tableID, StableRowKey: "r1", Position: 0},
		{TenantID: tenantID, TableID: tableID, StableRowKey: "r2", Position: 1},
	}

	result, err := ReplaceNestedCollection[nestedDocumentTable, nestedDocumentRow, nestedDocumentCell](
		context.Background(),
		db,
		NestedCollectionReplace[nestedDocumentTable, nestedDocumentRow, nestedDocumentCell]{
			Parent: nestedDocumentTable{ID: tableID, TenantID: tenantID, Title: "Hours", Revision: 2},
			ParentOpts: []WriteOpt{
				WherePK(),
				UpdateColumns("tenant_id", "title", "revision"),
			},
			DeleteBefore: []NestedDelete{
				{Table: "document_table_cells", Scopes: []Scope{TenantScope(tenantID), nestedWhere("table_id", tableID)}},
				{Table: "document_table_rows", Scopes: []Scope{TenantScope(tenantID), nestedWhere("table_id", tableID)}},
			},
			Children:      rows,
			ChildIDColumn: "id",
			AssignChildID: func(index int, id int64) {
				rows[index].ID = id
			},
			Grandchildren: func(index int, row nestedDocumentRow, rowID int64) ([]nestedDocumentCell, error) {
				return []nestedDocumentCell{{
					TenantID:      row.TenantID,
					TableID:       row.TableID,
					RowID:         rowID,
					StableCellKey: row.StableRowKey + ":c1",
					ValueJSON:     `{"text":"ok"}`,
				}}, nil
			},
			GrandchildMode: NestedWriteUpsert,
			GrandchildOpts: []WriteOpt{
				ConflictColumns("tenant_id", "table_id", "stable_cell_key"),
				UpdateColumns("row_id", "value_json"),
			},
		},
	)
	if err != nil {
		t.Fatalf("replace nested collection: %v", err)
	}
	if !reflect.DeepEqual(result.ChildIDs, []int64{101, 102}) {
		t.Fatalf("unexpected child ids: %#v", result.ChildIDs)
	}
	if rows[0].ID != 101 || rows[1].ID != 102 {
		t.Fatalf("assign child id did not update rows: %+v", rows)
	}
	if result.GrandchildCount != 2 {
		t.Fatalf("expected two grandchildren, got %d", result.GrandchildCount)
	}
	if len(exec.statements) != 5 {
		t.Fatalf("expected 5 statements, got %d: %#v", len(exec.statements), exec.statements)
	}
	wantFragments := []string{
		"INSERT INTO `document_tables`",
		"DELETE FROM `document_table_cells`",
		"DELETE FROM `document_table_rows`",
		"INSERT INTO `document_table_rows`",
		"INSERT INTO `document_table_cells`",
	}
	for i, want := range wantFragments {
		if !strings.Contains(exec.statements[i].query, want) {
			t.Fatalf("statement %d should contain %q, got: %s", i, want, exec.statements[i].query)
		}
	}
	if !strings.Contains(exec.statements[0].query, "ON DUPLICATE KEY UPDATE") {
		t.Fatalf("parent should be upserted, got: %s", exec.statements[0].query)
	}
	if strings.Contains(exec.statements[3].query, "`id`") {
		t.Fatalf("child insert should omit generated id, got: %s", exec.statements[3].query)
	}
	if !strings.Contains(exec.statements[4].query, "ON DUPLICATE KEY UPDATE") {
		t.Fatalf("grandchildren should be upserted, got: %s", exec.statements[4].query)
	}
	if !hasArg(exec.statements[4].args, int64(101)) || !hasArg(exec.statements[4].args, int64(102)) {
		t.Fatalf("grandchild args should include generated row ids, got: %#v", exec.statements[4].args)
	}
}

func TestReplaceNestedCollectionPostgresCollectsChildIDsWithReturning(t *testing.T) {
	db, mock := newReturningMockDB(t)
	sqlText := `INSERT INTO "document_table_rows" ("tenant_id", "table_id", "stable_row_key", "position") VALUES ($1, $2, $3, $4), ($5, $6, $7, $8) RETURNING "id"`
	mock.ExpectQuery(regexp.QuoteMeta(sqlText)).
		WithArgs("tenant-1", int64(7), "r1", 0, "tenant-1", int64(7), "r2", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(201).AddRow(202))

	rows := []nestedDocumentRow{
		{TenantID: "tenant-1", TableID: 7, StableRowKey: "r1", Position: 0},
		{TenantID: "tenant-1", TableID: 7, StableRowKey: "r2", Position: 1},
	}
	var assigned []int64
	result, err := ReplaceNestedCollection[struct{}, nestedDocumentRow, struct{}](
		context.Background(),
		db,
		NestedCollectionReplace[struct{}, nestedDocumentRow, struct{}]{
			SkipParent:    true,
			Children:      rows,
			ChildIDColumn: "id",
			AssignChildID: func(_ int, id int64) {
				assigned = append(assigned, id)
			},
		},
	)
	if err != nil {
		t.Fatalf("replace nested collection postgres: %v", err)
	}
	if !reflect.DeepEqual(result.ChildIDs, []int64{201, 202}) {
		t.Fatalf("unexpected child ids: %#v", result.ChildIDs)
	}
	if !reflect.DeepEqual(assigned, []int64{201, 202}) {
		t.Fatalf("unexpected assigned ids: %#v", assigned)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestReplaceNestedCollectionRejectsChildIDCollectionForUpsert(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := ReplaceNestedCollection[struct{}, nestedDocumentRow, struct{}](
		context.Background(),
		db,
		NestedCollectionReplace[struct{}, nestedDocumentRow, struct{}]{
			SkipParent:    true,
			Children:      []nestedDocumentRow{{TenantID: "tenant-1", TableID: 7, StableRowKey: "r1"}},
			ChildMode:     NestedWriteUpsert,
			ChildIDColumn: "id",
			AssignChildID: func(int, int64) {},
			ChildOpts:     []WriteOpt{ConflictColumns("tenant_id", "table_id", "stable_row_key")},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "nested child IDs can only be collected") {
		t.Fatalf("expected child id collection error, got: %v", err)
	}
}

func TestUpdateReturningPostgresAddsClause(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`UPDATE "users" SET .* RETURNING "id", "name"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(3, "alice"))

	res, err := Update(
		context.Background(),
		db,
		genericWriteUser{ID: 3, Name: "alice"},
		Columns("name"),
		WherePK(),
		Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("update returning: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil || aff != 1 {
		t.Fatalf("expected rows affected 1, got %d err=%v", aff, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpdateReturningTypedInfersColumns(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`UPDATE "users" SET .* RETURNING "id", "name", "age"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(3, "alice", 31))

	row, err := UpdateReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{ID: 3, Name: "alice"},
		Columns("name"),
		WherePK(),
	)
	if err != nil {
		t.Fatalf("update returning typed: %v", err)
	}
	if row.ID != 3 || row.Name != "alice" || row.Age != 31 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpdateReturningNoRowsAsConflict(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`UPDATE "users" SET .* RETURNING "id", "name", "age"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}))

	_, err := UpdateReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{ID: 3, Name: "alice"},
		Columns("name"),
		WherePK(),
		NoRowsAs(ErrConflict),
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpsertReturningPostgresAddsClause(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*ON CONFLICT \("id"\) DO UPDATE SET .* RETURNING "id", "name"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(5, "alice"))

	res, err := Upsert(
		context.Background(),
		db,
		genericWriteUser{ID: 5, Name: "alice"},
		WherePK(),
		Returning("id", "name"),
	)
	if err != nil {
		t.Fatalf("upsert returning: %v", err)
	}
	if aff, err := res.RowsAffected(); err != nil || aff != 1 {
		t.Fatalf("expected rows affected 1, got %d err=%v", aff, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpsertReturningTypedInfersColumns(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*ON CONFLICT \("id"\) DO UPDATE SET .* RETURNING "id", "name", "age"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(5, "alice", 32))

	row, err := UpsertReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{ID: 5, Name: "alice"},
		WherePK(),
	)
	if err != nil {
		t.Fatalf("upsert returning typed: %v", err)
	}
	if row.ID != 5 || row.Name != "alice" || row.Age != 32 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpsertPostgresConflictWhere(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{
			"id":              "audit-1",
			"tenant_id":       "tenant-1",
			"idempotency_key": "idem-1",
			"payload_json":    "{}",
		},
		Table("ai_audit_logs"),
		ConflictColumns("tenant_id", "idempotency_key"),
		ConflictWhere("idempotency_key <> ''"),
	)
	if err != nil {
		t.Fatalf("upsert conflict where: %v", err)
	}
	if !strings.Contains(exec.query, `ON CONFLICT ("tenant_id", "idempotency_key") WHERE idempotency_key <> '' DO UPDATE SET`) {
		t.Fatalf("expected partial-index conflict target, got: %s", exec.query)
	}
	if strings.Contains(exec.query, `"tenant_id"=EXCLUDED."tenant_id"`) || strings.Contains(exec.query, `"idempotency_key"=EXCLUDED."idempotency_key"`) {
		t.Fatalf("conflict columns should not be updated: %s", exec.query)
	}
}

func TestUpsertPostgresUpdateColumnsSeparatesInsertAndUpdate(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{
			"id":               "field-1",
			"tenant_id":        "tenant-1",
			"form_instance_id": "form-1",
			"field_key":        "weekly_hours",
			"value_text":       "40",
			"needs_update":     false,
		},
		Table("form_fields"),
		ConflictColumns("tenant_id", "form_instance_id", "field_key"),
		UpdateColumns("value_text", "needs_update"),
	)
	if err != nil {
		t.Fatalf("upsert update columns: %v", err)
	}
	if !strings.Contains(exec.query, `"id"`) {
		t.Fatalf("expected insert-only id column to be present, got: %s", exec.query)
	}
	if !strings.Contains(exec.query, `"value_text"=EXCLUDED."value_text"`) || !strings.Contains(exec.query, `"needs_update"=EXCLUDED."needs_update"`) {
		t.Fatalf("expected explicit update columns, got: %s", exec.query)
	}
	for _, col := range []string{`"id"=EXCLUDED."id"`, `"tenant_id"=EXCLUDED."tenant_id"`, `"form_instance_id"=EXCLUDED."form_instance_id"`, `"field_key"=EXCLUDED."field_key"`} {
		if strings.Contains(exec.query, col) {
			t.Fatalf("column %s should not be updated: %s", col, exec.query)
		}
	}
}

func TestUpsertConflictDoNothing(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{
			"tenant_id":       "tenant-1",
			"idempotency_key": "idem-1",
			"payload_json":    "{}",
		},
		Table("submission_attempts"),
		ConflictColumns("tenant_id", "idempotency_key"),
		ConflictDoNothing(),
	)
	if err != nil {
		t.Fatalf("upsert do nothing: %v", err)
	}
	if !strings.Contains(exec.query, `ON CONFLICT ("tenant_id", "idempotency_key") DO NOTHING`) {
		t.Fatalf("expected DO NOTHING conflict action, got: %s", exec.query)
	}
	if strings.Contains(exec.query, "DO UPDATE") {
		t.Fatalf("expected no conflict update, got: %s", exec.query)
	}
}

func TestUpsertPostgresConflictTargetRaw(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{
			"tenant_id":      "tenant-1",
			"target_node_id": nil,
			"payload_json":   "{}",
		},
		Table("citation_links"),
		ConflictTargetRaw(`("tenant_id", COALESCE("target_node_id", '')) WHERE "active"`),
		ConflictDoNothing(),
	)
	if err != nil {
		t.Fatalf("upsert raw conflict target: %v", err)
	}
	if !strings.Contains(exec.query, `ON CONFLICT ("tenant_id", COALESCE("target_node_id", '')) WHERE "active" DO NOTHING`) {
		t.Fatalf("expected raw expression conflict target, got: %s", exec.query)
	}
}

func TestInsertOnceReturningInsertedRow(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*ON CONFLICT \("id"\) DO NOTHING RETURNING "id", "name", "age"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(5, "alice", 32))

	row, inserted, err := InsertOnceReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{ID: 5, Name: "alice", Age: 32},
		WherePK(),
	)
	if err != nil {
		t.Fatalf("insert once returning: %v", err)
	}
	if !inserted {
		t.Fatal("expected inserted=true")
	}
	if row.ID != 5 || row.Name != "alice" || row.Age != 32 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestInsertOnceReturningExistingRow(t *testing.T) {
	db, mock := newReturningMockDB(t)
	mock.ExpectQuery(`INSERT INTO "users".*ON CONFLICT \("id"\) DO NOTHING RETURNING "id", "name", "age"$`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}))
	mock.ExpectQuery(`SELECT "id", "name", "age" FROM "users" WHERE "id" = \$1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(5, "existing", 40))

	row, inserted, err := InsertOnceReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{ID: 5, Name: "alice", Age: 32},
		WherePK(),
	)
	if err != nil {
		t.Fatalf("insert once returning existing: %v", err)
	}
	if inserted {
		t.Fatal("expected inserted=false")
	}
	if row.ID != 5 || row.Name != "existing" || row.Age != 40 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpsertUpdateColumnsRequireInsertedColumn(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{
			"tenant_id": "tenant-1",
			"field_key": "weekly_hours",
		},
		Table("form_fields"),
		ConflictColumns("tenant_id", "field_key"),
		UpdateColumns("value_text"),
	)
	if err == nil || !strings.Contains(err.Error(), "UpdateColumns requires inserted column value_text") {
		t.Fatalf("expected missing update column error, got: %v", err)
	}
}

func TestUpsertPostgresConflictConstraint(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{"name": "alice", "age": 30},
		Table("users"),
		ConflictConstraint("users_name_key"),
	)
	if err != nil {
		t.Fatalf("upsert conflict constraint: %v", err)
	}
	if !strings.Contains(exec.query, `ON CONFLICT ON CONSTRAINT "users_name_key" DO UPDATE SET`) {
		t.Fatalf("expected named constraint conflict target, got: %s", exec.query)
	}
}

func TestGenericWriteQuotesSchemaQualifiedTable(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Insert(
		context.Background(),
		db,
		map[string]any{"name": "alice"},
		Table("app.users"),
	)
	if err != nil {
		t.Fatalf("insert schema-qualified table: %v", err)
	}
	if !strings.Contains(exec.query, `INSERT INTO "app"."users"`) {
		t.Fatalf("expected schema-qualified table path, got: %s", exec.query)
	}
}

func TestGenericWriteTablePathOption(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := Insert(
		context.Background(),
		db,
		map[string]any{"name": "alice"},
		TablePath("app", "users"),
	)
	if err != nil {
		t.Fatalf("insert table path: %v", err)
	}
	if !strings.Contains(exec.query, "INSERT INTO `app`.`users`") {
		t.Fatalf("expected table path, got: %s", exec.query)
	}
}

func TestGenericWriteSchemaNameOption(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Insert(
		context.Background(),
		db,
		genericWriteUser{Name: "alice"},
		SchemaName("app"),
		Columns("name"),
	)
	if err != nil {
		t.Fatalf("insert schema name: %v", err)
	}
	if !strings.Contains(exec.query, `INSERT INTO "app"."users"`) {
		t.Fatalf("expected schema name table path, got: %s", exec.query)
	}
}

func TestNewDBWithExecutorUsesExternalExecutor(t *testing.T) {
	exec := &captureExecutor{}
	db := NewDBWithExecutor(exec, driver.PostgresDialect{})
	defer db.Close()

	if db.SQLDB() != nil {
		t.Fatalf("external executor DB should not expose sql.DB")
	}
	if _, err := Insert(context.Background(), db, map[string]any{"name": "alice"}, Table("users")); err != nil {
		t.Fatalf("insert with external executor: %v", err)
	}
	if !strings.Contains(exec.query, `INSERT INTO "users"`) {
		t.Fatalf("expected executor query capture, got: %s", exec.query)
	}
}

func TestUpdateExpressionAssignments(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Update(
		context.Background(),
		db,
		map[string]any{"id": int64(7), "email_verified_at": "ignored"},
		Table("app.users"),
		PK("id"),
		WherePK(),
		SetExpr("email_verified_at", "COALESCE(email_verified_at, ?)", "2026-05-22T00:00:00Z"),
		Increment("credential_version", 1),
	)
	if err != nil {
		t.Fatalf("update expression assignments: %v", err)
	}
	if !strings.Contains(exec.query, `UPDATE "app"."users" SET "email_verified_at"=COALESCE(email_verified_at, $1), "credential_version"="credential_version" + $2 WHERE "id"=$3`) {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if len(exec.args) != 3 || exec.args[0] != "2026-05-22T00:00:00Z" || exec.args[1] != 1 || exec.args[2] != int64(7) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestUpdateSetColumnAssignment(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Update(
		context.Background(),
		db,
		map[string]any{"id": int64(7)},
		Table("users"),
		PK("id"),
		WherePK(),
		SetColumn("updated_at", "password_changed_at"),
	)
	if err != nil {
		t.Fatalf("update set column assignment: %v", err)
	}
	if !strings.Contains(exec.query, `SET "updated_at"="password_changed_at" WHERE "id"=$1`) {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if len(exec.args) != 1 || exec.args[0] != int64(7) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestUpsertExpressionAssignments(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{"id": int64(9), "name": "alice"},
		Table("users"),
		ConflictColumns("id"),
		UpdateColumns("name"),
		Increment("credential_version", 1),
	)
	if err != nil {
		t.Fatalf("upsert expression assignments: %v", err)
	}
	if !strings.Contains(exec.query, `ON CONFLICT ("id") DO UPDATE SET "name"=EXCLUDED."name", "credential_version"="credential_version" + $3`) {
		t.Fatalf("unexpected query: %s", exec.query)
	}
	if !hasArg(exec.args, int64(9)) || !hasArg(exec.args, "alice") || !hasArg(exec.args, 1) {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestInsertRejectsExpressionAssignments(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Insert(
		context.Background(),
		db,
		map[string]any{"name": "alice"},
		Table("users"),
		SetRaw("updated_at", "now()"),
	)
	if err == nil || !strings.Contains(err.Error(), "assignment options are not supported for Insert") {
		t.Fatalf("expected assignment insert error, got: %v", err)
	}
}

func TestConflictDoNothingRejectsExpressionAssignments(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, err := Upsert(
		context.Background(),
		db,
		map[string]any{"id": int64(9), "name": "alice"},
		Table("users"),
		ConflictColumns("id"),
		ConflictDoNothing(),
		Increment("credential_version", 1),
	)
	if err == nil || !strings.Contains(err.Error(), "ConflictDoNothing cannot be combined") {
		t.Fatalf("expected do-nothing assignment error, got: %v", err)
	}
}

func TestInsertOnceReturningRejectsExpressionAssignments(t *testing.T) {
	db, _ := newCaptureWriteDB(driver.PostgresDialect{})

	_, _, err := InsertOnceReturning[genericWriteUser](
		context.Background(),
		db,
		genericWriteUser{ID: 5, Name: "alice"},
		WherePK(),
		Increment("credential_version", 1),
	)
	if err == nil || !strings.Contains(err.Error(), "ConflictDoNothing cannot be combined") {
		t.Fatalf("expected insert-once assignment error, got: %v", err)
	}
}
