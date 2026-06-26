package query

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"

	ormdriver "github.com/recoweft/goquent/orm/driver"
)

type recordingExec struct {
	calls int
}

func (e *recordingExec) Query(string, ...any) (*sql.Rows, error) {
	e.calls++
	return nil, errors.New("unexpected query")
}

func (e *recordingExec) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	e.calls++
	return nil, errors.New("unexpected query context")
}

func (e *recordingExec) QueryRow(string, ...any) *sql.Row {
	e.calls++
	return &sql.Row{}
}

func (e *recordingExec) QueryRowContext(context.Context, string, ...any) *sql.Row {
	e.calls++
	return &sql.Row{}
}

func (e *recordingExec) Exec(string, ...any) (sql.Result, error) {
	e.calls++
	return driver.RowsAffected(0), errors.New("unexpected exec")
}

func (e *recordingExec) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	e.calls++
	return driver.RowsAffected(0), errors.New("unexpected exec context")
}

func newPlanTestQuery(exec *recordingExec) *Query {
	return New(exec, "users", ormdriver.MySQLDialect{})
}

func TestSelectPlanSnapshot(t *testing.T) {
	exec := &recordingExec{}
	plan, err := newPlanTestQuery(exec).
		Select("id", "name").
		Where("id", 10).
		OrderBy("id", "asc").
		Limit(1).
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if exec.calls != 0 {
		t.Fatalf("Plan executed database call count=%d", exec.calls)
	}
	if plan.Operation != OperationSelect {
		t.Fatalf("operation=%s", plan.Operation)
	}
	if plan.SQL != "SELECT `id`, `name` FROM `users` WHERE `id` = ? ORDER BY `id` ASC LIMIT 1" {
		t.Fatalf("sql=%q", plan.SQL)
	}
	if len(plan.Params) != 1 || plan.Params[0] != 10 {
		t.Fatalf("params=%#v", plan.Params)
	}
	if len(plan.Tables) != 1 || plan.Tables[0].Name != "users" {
		t.Fatalf("tables=%#v", plan.Tables)
	}
	if len(plan.Columns) != 2 || plan.Columns[0].Name != "id" || plan.Columns[1].Name != "name" {
		t.Fatalf("columns=%#v", plan.Columns)
	}
	if len(plan.Predicates) != 1 || plan.Predicates[0].Column != "id" || plan.Predicates[0].Operator != "=" {
		t.Fatalf("predicates=%#v", plan.Predicates)
	}
	if plan.Limit == nil || *plan.Limit != 1 {
		t.Fatalf("limit=%v", plan.Limit)
	}
}

func TestSelectRejectsExpressionLikeFields(t *testing.T) {
	cases := []string{
		"CASE WHEN result IS NULL THEN NULL ELSE result::text END AS result_json",
		"COUNT(id)",
		"result::text",
		"name AS display_name",
		"result->>'status'",
		"first name",
	}

	for _, field := range cases {
		t.Run(field, func(t *testing.T) {
			_, err := newPlanTestQuery(&recordingExec{}).
				Select("finished_at", field).
				Plan(context.Background())
			if err == nil {
				t.Fatal("expected Select expression-like field error")
			}
			if !strings.Contains(err.Error(), "SelectRaw(...)") || !strings.Contains(err.Error(), field) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestSelectRawAcceptsExpressionSelection(t *testing.T) {
	exec := &recordingExec{}
	plan, err := newPlanTestQuery(exec).
		Select("finished_at").
		SelectRaw("CASE WHEN result IS NULL THEN NULL ELSE result::text END AS result_json").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if exec.calls != 0 {
		t.Fatalf("Plan executed database call count=%d", exec.calls)
	}
	if !strings.Contains(plan.SQL, "CASE WHEN result IS NULL THEN NULL ELSE result::text END AS result_json") {
		t.Fatalf("sql=%q", plan.SQL)
	}
}

func TestSelectAllowsIdentifierAndWildcardFields(t *testing.T) {
	_, err := newPlanTestQuery(&recordingExec{}).
		Select("id", "users.name", "*", "profiles.*").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
}

func TestWhereCursorAfterBuildsLexicographicPredicate(t *testing.T) {
	plan, err := New(&recordingExec{}, "jobs", ormdriver.PostgresDialect{}).
		Select("id").
		WhereCursorAfter([]CursorColumn{CursorDesc("due_at"), CursorDesc("id")}, "2026-05-19T00:00:00Z", "job-1").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !strings.Contains(plan.SQL, `"due_at" <`) || !strings.Contains(plan.SQL, `"due_at" =`) || !strings.Contains(plan.SQL, `"id" <`) {
		t.Fatalf("expected descending cursor predicate, sql=%q", plan.SQL)
	}
	if len(plan.Params) != 3 ||
		plan.Params[0] != "2026-05-19T00:00:00Z" ||
		plan.Params[1] != "2026-05-19T00:00:00Z" ||
		plan.Params[2] != "job-1" {
		t.Fatalf("params=%#v", plan.Params)
	}
}

func TestWhereCursorRejectsInvalidInput(t *testing.T) {
	_, err := newPlanTestQuery(&recordingExec{}).
		Select("id").
		WhereCursorAfter([]CursorColumn{CursorAsc("due_at")}, "cursor", 1).
		Plan(context.Background())
	if err == nil || !strings.Contains(err.Error(), "cursor column/value count mismatch") {
		t.Fatalf("expected count mismatch error, got %v", err)
	}
}

func TestWhereCursorSupportsTrustedExpression(t *testing.T) {
	plan, err := New(&recordingExec{}, "events", ormdriver.PostgresDialect{}).
		SelectRaw("(entity_type || ':' || entity_id) AS entity_sort_key").
		WhereCursorAfter([]CursorColumn{CursorDescExpr("(entity_type || ':' || entity_id)")}, "source:1").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !strings.Contains(plan.SQL, `(entity_type || ':' || entity_id) <`) {
		t.Fatalf("expected raw cursor expression, sql=%q", plan.SQL)
	}
	if len(plan.Params) != 1 || plan.Params[0] != "source:1" {
		t.Fatalf("params=%#v", plan.Params)
	}
}

func TestWhereTextSearchPostgresUsesILikeAcrossColumns(t *testing.T) {
	plan, err := New(&recordingExec{}, "corpus_units", ormdriver.PostgresDialect{}).
		Select("id").
		Where("tenant_id", "tenant-1").
		WhereTextSearch([]string{"title", "normalized_text", "article_no"}, "Article_10%").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, want := range []string{`"title" ILIKE`, `"normalized_text" ILIKE`, `"article_no" ILIKE`, `ESCAPE '!'`} {
		if !strings.Contains(plan.SQL, want) {
			t.Fatalf("expected %q in sql=%q", want, plan.SQL)
		}
	}
	if len(plan.Params) != 4 || plan.Params[0] != "tenant-1" {
		t.Fatalf("params=%#v", plan.Params)
	}
	for _, param := range plan.Params[1:] {
		if param != "%Article!_10!%%" {
			t.Fatalf("expected escaped search pattern, params=%#v", plan.Params)
		}
	}
}

func TestWhereTextSearchMySQLUsesLike(t *testing.T) {
	plan, err := newPlanTestQuery(&recordingExec{}).
		Select("id").
		WhereTextSearch([]string{"name", "email"}, "alice!").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !strings.Contains(plan.SQL, "`name` LIKE") || !strings.Contains(plan.SQL, "`email` LIKE") {
		t.Fatalf("expected LIKE text search, sql=%q", plan.SQL)
	}
	if len(plan.Params) != 2 || plan.Params[0] != "%alice!!%" || plan.Params[1] != "%alice!!%" {
		t.Fatalf("params=%#v", plan.Params)
	}
}

func TestWhereTextSearchRejectsInvalidInput(t *testing.T) {
	_, err := newPlanTestQuery(&recordingExec{}).
		Select("id").
		WhereTextSearch([]string{"name"}, "   ").
		Plan(context.Background())
	if err == nil || !strings.Contains(err.Error(), "text search term is required") {
		t.Fatalf("expected empty term error, got %v", err)
	}

	_, err = newPlanTestQuery(&recordingExec{}).
		Select("id").
		WhereTextSearch([]string{"LOWER(name)"}, "alice").
		Plan(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid text search column") {
		t.Fatalf("expected invalid column error, got %v", err)
	}
}

func TestPostgresJSONBPredicates(t *testing.T) {
	plan, err := New(&recordingExec{}, "events", ormdriver.PostgresDialect{}).
		Select("id").
		WhereJSONText("payload", "reason", "initial_sync").
		WhereJSONHasKey("payload", "cache_invalidated_at").
		WhereJSONNotHasKey("payload", "ignored_at").
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, want := range []string{`"payload" ->>`, `"payload" ?`, `NOT ("payload" ?`} {
		if !strings.Contains(plan.SQL, want) {
			t.Fatalf("expected %q in sql=%q", want, plan.SQL)
		}
	}
	if len(plan.Params) != 4 ||
		plan.Params[0] != "reason" ||
		plan.Params[1] != "initial_sync" ||
		plan.Params[2] != "cache_invalidated_at" ||
		plan.Params[3] != "ignored_at" {
		t.Fatalf("params=%#v", plan.Params)
	}
}

func TestJSONBPredicatesRequirePostgres(t *testing.T) {
	_, err := newPlanTestQuery(&recordingExec{}).
		Select("id").
		WhereJSONHasKey("payload", "reason").
		Plan(context.Background())
	if err == nil || !strings.Contains(err.Error(), "only supported on PostgreSQL") {
		t.Fatalf("expected postgres-only error, got %v", err)
	}
}

func TestWritePlanSnapshots(t *testing.T) {
	ctx := context.Background()

	t.Run("insert", func(t *testing.T) {
		exec := &recordingExec{}
		plan, err := newPlanTestQuery(exec).PlanInsert(ctx, map[string]any{"name": "alice", "age": 30})
		if err != nil {
			t.Fatalf("PlanInsert: %v", err)
		}
		if exec.calls != 0 {
			t.Fatalf("PlanInsert executed database call count=%d", exec.calls)
		}
		if plan.Operation != OperationInsert {
			t.Fatalf("operation=%s", plan.Operation)
		}
		if plan.SQL != "INSERT INTO `users` (`age`, `name`) VALUES (?, ?)" {
			t.Fatalf("sql=%q", plan.SQL)
		}
		if len(plan.Params) != 2 || plan.Params[0] != 30 || plan.Params[1] != "alice" {
			t.Fatalf("params=%#v", plan.Params)
		}
		if len(plan.Columns) != 2 || plan.Columns[0].Name != "age" || plan.Columns[1].Name != "name" {
			t.Fatalf("columns=%#v", plan.Columns)
		}
	})

	t.Run("update", func(t *testing.T) {
		exec := &recordingExec{}
		plan, err := newPlanTestQuery(exec).
			Where("id", 10).
			PlanUpdate(ctx, map[string]any{"name": "alice"})
		if err != nil {
			t.Fatalf("PlanUpdate: %v", err)
		}
		if exec.calls != 0 {
			t.Fatalf("PlanUpdate executed database call count=%d", exec.calls)
		}
		if plan.Operation != OperationUpdate {
			t.Fatalf("operation=%s", plan.Operation)
		}
		if plan.SQL != "UPDATE `users` SET `name` = ? WHERE `id` = ?" {
			t.Fatalf("sql=%q", plan.SQL)
		}
		if len(plan.Params) != 2 || plan.Params[0] != "alice" || plan.Params[1] != 10 {
			t.Fatalf("params=%#v", plan.Params)
		}
		if len(plan.Predicates) != 1 || plan.Predicates[0].Column != "id" {
			t.Fatalf("predicates=%#v", plan.Predicates)
		}
	})

	t.Run("delete", func(t *testing.T) {
		exec := &recordingExec{}
		plan, err := newPlanTestQuery(exec).
			Where("id", 10).
			PlanDelete(ctx)
		if err != nil {
			t.Fatalf("PlanDelete: %v", err)
		}
		if exec.calls != 0 {
			t.Fatalf("PlanDelete executed database call count=%d", exec.calls)
		}
		if plan.Operation != OperationDelete {
			t.Fatalf("operation=%s", plan.Operation)
		}
		if plan.SQL != "DELETE FROM `users` WHERE `id` = ?" {
			t.Fatalf("sql=%q", plan.SQL)
		}
		if len(plan.Params) != 1 || plan.Params[0] != 10 {
			t.Fatalf("params=%#v", plan.Params)
		}
	})
}

func TestRawPlanSnapshot(t *testing.T) {
	plan := NewRawPlan("DELETE FROM users WHERE id = ?", 10)
	if plan.Operation != OperationRaw {
		t.Fatalf("operation=%s", plan.Operation)
	}
	if plan.RiskLevel != RiskHigh {
		t.Fatalf("risk=%s", plan.RiskLevel)
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0].Code != WarningRawSQLUsed {
		t.Fatalf("warnings=%#v", plan.Warnings)
	}
	if len(plan.Params) != 1 || plan.Params[0] != 10 {
		t.Fatalf("params=%#v", plan.Params)
	}
	if _, err := plan.ToJSON(); err != nil {
		t.Fatalf("ToJSON: %v", err)
	}
	if !strings.Contains(plan.String(), "RAW_SQL_USED") {
		t.Fatalf("String()=%q", plan.String())
	}
}

func TestPlanParamsOrderingAndInjectionRegression(t *testing.T) {
	injected := "alice' OR 1=1 --"
	plan, err := newPlanTestQuery(&recordingExec{}).
		Where("age", ">", 20).
		Where("name", injected).
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.Params) != 2 || plan.Params[0] != 20 || plan.Params[1] != injected {
		t.Fatalf("params=%#v", plan.Params)
	}
	if strings.Contains(plan.SQL, injected) {
		t.Fatalf("SQL contains injected value: %q", plan.SQL)
	}
	if got := strings.Count(plan.SQL, "?"); got != 2 {
		t.Fatalf("placeholder count=%d sql=%q", got, plan.SQL)
	}
}

func TestWhereRawNoArgsPlan(t *testing.T) {
	plan, err := newPlanTestQuery(&recordingExec{}).
		WhereRawNoArgs("deleted_at IS NULL").
		Limit(1).
		Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !strings.Contains(plan.SQL, "deleted_at IS NULL") {
		t.Fatalf("expected raw predicate in SQL, got %q", plan.SQL)
	}
	if len(plan.Params) != 0 {
		t.Fatalf("expected no params, got %#v", plan.Params)
	}
	if len(plan.Predicates) != 1 || plan.Predicates[0].Raw != "deleted_at IS NULL" {
		t.Fatalf("predicates=%#v", plan.Predicates)
	}
}
