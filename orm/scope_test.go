package orm

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

type scopeUser struct {
	ID   int64  `db:"id,pk"`
	Name string `db:"name"`
	Age  int    `db:"age"`
}

func (scopeUser) TableName() string { return "users" }

func newScopeMockDB(t *testing.T, d driver.Dialect) (*DB, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return NewDB(sqlDB, d), mock
}

func withProfiles() Scope {
	return func(q *query.Query) *query.Query {
		return q.Join("profiles", "users.id", "=", "profiles.user_id")
	}
}

func bioLike(v string) Scope {
	return func(q *query.Query) *query.Query {
		return q.Where("profiles.bio", "like", v)
	}
}

func selectUserColumns() Scope {
	return func(q *query.Query) *query.Query {
		return q.Select("users.id", "users.name", "users.age")
	}
}

func orderUsers() Scope {
	return func(q *query.Query) *query.Query {
		return q.OrderBy("users.id", "asc")
	}
}

func groupedUserFilter(name string, minAge int) Scope {
	return func(q *query.Query) *query.Query {
		return q.WhereGroup(func(g *query.Query) {
			g.Where("users.name", name).OrWhere("users.age", ">", minAge)
		})
	}
}

func profileExists(db *DB, bio string) Scope {
	return func(q *query.Query) *query.Query {
		sub := db.Table("profiles").
			SelectRaw("1").
			SafeWhereRaw("profiles.user_id = users.id", map[string]any{}).
			Where("profiles.bio", "like", bio)
		return q.WhereExists(sub)
	}
}

func TestComposeScopesBuildsReusableScope(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})
	scope := ComposeScopes(withProfiles(), bioLike("%go%"), orderUsers())

	q := ApplyScopes(db.Table("users"), selectUserColumns(), scope)
	sqlStr, args, err := q.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(sqlStr, "JOIN `profiles`") {
		t.Fatalf("expected join in SQL, got: %s", sqlStr)
	}
	if !strings.Contains(sqlStr, "`profiles`.`bio` like ?") {
		t.Fatalf("expected where in SQL, got: %s", sqlStr)
	}
	if !strings.Contains(strings.ToUpper(sqlStr), "ORDER BY `USERS`.`ID` ASC") {
		t.Fatalf("expected order in SQL, got: %s", sqlStr)
	}
	if len(args) != 1 || args[0] != "%go%" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestComposeScopesSupportsGroupsAndExists(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})
	scope := ComposeScopes(groupedUserFilter("alice", 29), profileExists(db, "%developer%"), orderUsers())

	q := ApplyScopes(db.Table("users"), selectUserColumns(), scope)
	sqlStr, args, err := q.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(sqlStr, "EXISTS") {
		t.Fatalf("expected EXISTS in SQL, got: %s", sqlStr)
	}
	if !strings.Contains(sqlStr, " OR ") {
		t.Fatalf("expected grouped OR in SQL, got: %s", sqlStr)
	}
	if !hasArg(args, "alice") || !hasArg(args, 29) || !hasArg(args, "%developer%") {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestTenantScopeAddsDefaultTenantFilter(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})

	q := ApplyScopes(db.Table("documents"), TenantScope("tenant-1"))
	sqlStr, args, err := q.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(sqlStr, "WHERE `tenant_id` = ?") {
		t.Fatalf("expected default tenant filter, got: %s", sqlStr)
	}
	if len(args) != 1 || args[0] != "tenant-1" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestTenantScopeAllowsCustomTenantColumn(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.PostgresDialect{})

	q := ApplyScopes(db.Table("role_bindings"), TenantScope("tenant-1", "scope_tenant_id"))
	sqlStr, args, err := q.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(sqlStr, `WHERE "scope_tenant_id" = $1`) {
		t.Fatalf("expected custom tenant filter, got: %s", sqlStr)
	}
	if len(args) != 1 || args[0] != "tenant-1" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestTextSearchScopeAddsMultiColumnSearch(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.PostgresDialect{})

	plan, err := PlanSelectBy(
		context.Background(),
		db.Table("corpus_units").Select("id"),
		TenantScope("tenant-1"),
		TextSearch([]string{"title", "normalized_text", "article_no"}, "Article_10%"),
	)
	if err != nil {
		t.Fatalf("plan select by: %v", err)
	}
	if !strings.Contains(plan.SQL, `"title" ILIKE`) || !strings.Contains(plan.SQL, `"normalized_text" ILIKE`) {
		t.Fatalf("expected text search predicate, sql=%s", plan.SQL)
	}
	if len(plan.Params) != 4 || plan.Params[0] != "tenant-1" || plan.Params[1] != "%Article!_10!%%" {
		t.Fatalf("unexpected params: %#v", plan.Params)
	}
}

func TestRequireTenantScopeBlocksMissingScopedQuery(t *testing.T) {
	ctx := context.Background()
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})

	_, err := SelectAllBy[scopeUser](
		ctx,
		db,
		db.Table("users"),
		RequireTenantScope("users"),
	)
	if !errors.Is(err, query.ErrBlockedOperation) {
		t.Fatalf("expected blocked operation for missing tenant scope, got %v", err)
	}
}

func TestRequireTenantScopePassesWithTenantScope(t *testing.T) {
	ctx := context.Background()
	db, mock := newScopeMockDB(t, driver.MySQLDialect{})

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` WHERE `tenant_id` = ?")).
		WithArgs("tenant-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(1, "alice", 30))

	users, err := SelectAllBy[scopeUser](
		ctx,
		db,
		db.Table("users"),
		RequireTenantScope("users"),
		TenantScope("tenant-1"),
	)
	if err != nil {
		t.Fatalf("select all by: %v", err)
	}
	if len(users) != 1 || users[0].Name != "alice" {
		t.Fatalf("unexpected users: %+v", users)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestPlanSelectBySnapshotsScopedQueryWithoutExecuting(t *testing.T) {
	ctx := context.Background()
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})

	plan, err := PlanSelectBy(
		ctx,
		db.Table("users"),
		selectUserColumns(),
		RequireTenantScope("users"),
	)
	if err != nil {
		t.Fatalf("plan select by: %v", err)
	}
	if plan.Operation != OperationSelect {
		t.Fatalf("operation=%s", plan.Operation)
	}
	if !plan.Blocked || !warningCode(plan.Warnings, WarningRequiredPredicateMissing) {
		t.Fatalf("expected blocked missing required predicate, warnings=%#v", plan.Warnings)
	}
	if !strings.Contains(plan.SQL, "SELECT `users`.`id`, `users`.`name`, `users`.`age` FROM `users`") {
		t.Fatalf("unexpected sql: %s", plan.SQL)
	}
}

func TestPlanUpdateBySnapshotsScopedUpdate(t *testing.T) {
	ctx := context.Background()
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})

	plan, err := PlanUpdateBy(
		ctx,
		db.Table("users"),
		map[string]any{"age": 41},
		RequireTenantScope("users"),
		TenantScope("tenant-1"),
		func(q *query.Query) *query.Query {
			return q.Where("id", 7)
		},
	)
	if err != nil {
		t.Fatalf("plan update by: %v", err)
	}
	if plan.Operation != OperationUpdate {
		t.Fatalf("operation=%s", plan.Operation)
	}
	if plan.Blocked || warningCode(plan.Warnings, WarningRequiredPredicateMissing) {
		t.Fatalf("unexpected required predicate warning=%#v", plan.Warnings)
	}
	if !strings.Contains(plan.SQL, "UPDATE `users` SET `age` = ? WHERE `tenant_id` = ? AND `id` = ?") {
		t.Fatalf("unexpected sql: %s", plan.SQL)
	}
	if len(plan.Params) != 3 || plan.Params[0] != 41 || plan.Params[1] != "tenant-1" || plan.Params[2] != 7 {
		t.Fatalf("unexpected params: %#v", plan.Params)
	}
}

func TestPlanDeleteBySnapshotsScopedDelete(t *testing.T) {
	ctx := context.Background()
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})

	plan, err := PlanDeleteBy(
		ctx,
		db.Table("users"),
		RequireTenantScope("users"),
		TenantScope("tenant-1"),
	)
	if err != nil {
		t.Fatalf("plan delete by: %v", err)
	}
	if plan.Operation != OperationDelete {
		t.Fatalf("operation=%s", plan.Operation)
	}
	if plan.Blocked || warningCode(plan.Warnings, WarningRequiredPredicateMissing) {
		t.Fatalf("unexpected required predicate warning=%#v", plan.Warnings)
	}
	if !strings.Contains(plan.SQL, "DELETE FROM `users` WHERE `tenant_id` = ?") {
		t.Fatalf("unexpected sql: %s", plan.SQL)
	}
}

func TestSelectAllByBuildsFromScopedQuery(t *testing.T) {
	ctx := context.Background()
	db, mock := newScopeMockDB(t, driver.MySQLDialect{})

	rows := sqlmock.NewRows([]string{"id", "name", "age"}).
		AddRow(1, "alice", 30).
		AddRow(2, "bob", 25)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT `users`.`id`, `users`.`name`, `users`.`age` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`bio` like ? ORDER BY `users`.`id` ASC")).
		WithArgs("%go%").
		WillReturnRows(rows)

	users, err := SelectAllBy[scopeUser](
		ctx,
		db,
		db.Model(&scopeUser{}),
		selectUserColumns(),
		ComposeScopes(withProfiles(), bioLike("%go%"), orderUsers()),
	)
	if err != nil {
		t.Fatalf("select all by: %v", err)
	}
	if len(users) != 2 || users[0].Name != "alice" || users[1].Name != "bob" {
		t.Fatalf("unexpected users: %+v", users)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func warningCode(warnings []Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}

func TestUpdateByAppliesScopes(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := UpdateBy(
		context.Background(),
		db.Table("users"),
		map[string]any{"age": 55},
		withProfiles(),
		bioLike("%go%"),
	)
	if err != nil {
		t.Fatalf("update by: %v", err)
	}

	if !strings.Contains(exec.query, "JOIN `profiles`") {
		t.Fatalf("expected join query, got: %s", exec.query)
	}
	if !strings.Contains(exec.query, "WHERE `profiles`.`bio` like ?") {
		t.Fatalf("expected where query, got: %s", exec.query)
	}
	if len(exec.args) != 2 || exec.args[0] != 55 || exec.args[1] != "%go%" {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestUpdateByReturningAppliesScopes(t *testing.T) {
	ctx := context.Background()
	db, mock := newReturningMockDB(t)

	mock.ExpectQuery(`UPDATE "users" SET .* WHERE .* RETURNING "id", "name", "age"$`).
		WithArgs("alice", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).AddRow(1, "alice", 34))

	row, err := UpdateByReturning[genericWriteUser](
		ctx,
		db,
		db.Table("users"),
		map[string]any{"name": "alice"},
		func(q *query.Query) *query.Query {
			return q.Where("id", 1)
		},
	)
	if err != nil {
		t.Fatalf("update by returning: %v", err)
	}
	if row.ID != 1 || row.Name != "alice" || row.Age != 34 {
		t.Fatalf("unexpected row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestUpdateByReturningWithOptionsMapsNoRows(t *testing.T) {
	ctx := context.Background()
	db, mock := newReturningMockDB(t)

	mock.ExpectQuery(`UPDATE "users" SET .* WHERE .* RETURNING "id", "name", "age"$`).
		WithArgs("alice", 1, "hash-old").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}))

	_, err := UpdateByReturningWithOptions[genericWriteUser](
		ctx,
		db,
		db.Table("users"),
		map[string]any{"name": "alice"},
		[]WriteOpt{NoRowsAs(ErrConflict)},
		func(q *query.Query) *query.Query {
			return q.Where("id", 1).Where("content_hash", "hash-old")
		},
	)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestDeleteByAppliesScopes(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})

	_, err := DeleteBy(
		context.Background(),
		db.Table("users"),
		withProfiles(),
		bioLike("%python%"),
	)
	if err != nil {
		t.Fatalf("delete by: %v", err)
	}

	if !strings.Contains(exec.query, "DELETE `users` FROM `users`") {
		t.Fatalf("expected delete query, got: %s", exec.query)
	}
	if !strings.Contains(exec.query, "JOIN `profiles`") {
		t.Fatalf("expected join query, got: %s", exec.query)
	}
	if len(exec.args) != 1 || exec.args[0] != "%python%" {
		t.Fatalf("unexpected args: %#v", exec.args)
	}
}

func TestScopedHelpersRejectNilBase(t *testing.T) {
	ctx := context.Background()
	db, _ := newScopeMockDB(t, driver.MySQLDialect{})

	if _, err := SelectOneBy[scopeUser](ctx, db, nil); err == nil {
		t.Fatalf("expected nil base error for select one")
	}
	if _, err := SelectAllBy[scopeUser](ctx, db, nil); err == nil {
		t.Fatalf("expected nil base error for select all")
	}
	if _, err := UpdateBy(ctx, nil, map[string]any{"age": 1}); err == nil {
		t.Fatalf("expected nil base error for update")
	}
	if _, err := DeleteBy(ctx, nil); err == nil {
		t.Fatalf("expected nil base error for delete")
	}
}
