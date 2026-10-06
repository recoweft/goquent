package orm

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
	"reflect"
	"strings"
	"testing"
)

type compoundSpec = NestedCollectionReplace[map[string]any, map[string]any, map[string]any]

func staticSpec() compoundSpec {
	return compoundSpec{
		Parent: map[string]any{"tenant_id": 1, "id": 3, "score": 4}, ParentMode: NestedWriteInsert, ParentOpts: []WriteOpt{Table("users")},
		Children: []map[string]any{{"tenant_id": 1, "id": 4, "score": 5}}, ChildOpts: []WriteOpt{Table("users")},
	}
}

func TestCompoundKnownRefusalsBeforeBegin(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			for _, tx := range []bool{false, true} {
				for _, bad := range []string{"tenant", "column", "projection", "delete", "scope", "late scope", "grandchild", "assign"} {
					t.Run(bad+map[bool]string{true: " tx"}[tx], func(t *testing.T) {
						sqlDB, mock, err := sqlmock.New()
						if err != nil {
							t.Fatal(err)
						}
						defer sqlDB.Close()
						spy := &genericSpy{Executor: sqlDB}
						db := NewDB(sqlDB, genericDialect(dialect), WithSettings(genericSettings(t, dialect, false)))
						db.exec = spy
						spec := staticSpec()
						called := 0
						panicScope := func(q *query.Query) *query.Query { called++; panic("must not evaluate") }
						switch bad {
						case "tenant":
							spec.Children = append(spec.Children, map[string]any{"tenant_id": 2, "id": 5, "score": 6})
						case "column":
							spec.Children[0]["forbidden"] = 1
						case "projection":
							spec.ChildOpts = append(spec.ChildOpts, Returning("secret"))
						case "delete":
							spec.DeleteBefore = []NestedDelete{{Table: "users", Scopes: []Scope{nil}}}
						case "scope":
							spec.DeleteBefore = []NestedDelete{{Table: "users", Scopes: []Scope{panicScope}}}
						case "late scope":
							spec.DeleteBefore = []NestedDelete{{Table: "users"}, {Table: "users", Scopes: []Scope{panicScope}}}
							spec.SkipParent = true
							spec.Children = nil
						case "grandchild":
							spec.Grandchildren = func(int, map[string]any, int64) ([]map[string]any, error) { called++; panic("must not evaluate") }
						case "assign":
							spec.AssignChildID = func(int, int64) { called++; panic("must not evaluate") }
						}
						if tx {
							_, err = ReplaceNestedCollectionTx(context.Background(), db, spec)
						} else {
							_, err = ReplaceNestedCollection(context.Background(), db, spec)
						}
						if !errors.Is(err, ErrBlockedOperation) {
							t.Fatalf("expected policy refusal, got %v", err)
						}
						if len(spy.calls) != 0 || called != 0 {
							t.Fatalf("dispatch=%v callbacks=%d", spy.calls, called)
						}
						if err := mock.ExpectationsWereMet(); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		})
	}
}

func TestStrictOpaqueRecipes(t *testing.T) {
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", true)))
	panicApply := func(context.Context, Tx) (int, error) { panic("apply") }
	_, err := RunTransactionWithHooks(context.Background(), db, TransactionWithHooksSpec[int]{Apply: panicApply, Hooks: []TransactionHook{NewTransactionHook("opaque", func(context.Context, Tx) error { panic("hook") })}})
	if !errors.Is(err, ErrUnsupportedCompound) {
		t.Fatal(err)
	}
	_, err = RunIdempotentCommand(context.Background(), db, IdempotentCommandSpec[int]{Apply: panicApply, LookupExisting: func(context.Context, *DB) (int, error) { panic("lookup") }})
	if !errors.Is(err, ErrUnsupportedCompound) || len(spy.calls) != 0 {
		t.Fatalf("%v %v", err, spy.calls)
	}
	for _, scope := range []Scope{TenantScope(1), RequireTenantScope("users"), RequirePredicates(RequirePredicate("users", "id")), ComposeScopes(TenantScope(1))} {
		spec := staticSpec()
		spec.DeleteBefore = []NestedDelete{{Table: "users", Scopes: []Scope{scope}}}
		_, err := ReplaceNestedCollection(nil, db, spec)
		if !errors.Is(err, ErrUnsupportedCompound) {
			t.Fatal(err)
		}
	}
}

func TestStrictScopeEffectsNeverRun(t *testing.T) {
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.MySQLDialect{}, WithSettings(genericSettings(t, "mysql", true)))
	outside := NewDBWithExecutor(spy, driver.MySQLDialect{}).RequireRawApproval("fixture")
	effects := 0
	for _, scope := range []Scope{
		func(q *query.Query) *query.Query {
			effects++
			_, _ = q.Where("id", 3).Update(map[string]any{"score": 8})
			return q
		},
		func(q *query.Query) *query.Query {
			effects++
			_, _ = outside.Exec("UPDATE users SET score=8 WHERE id=3")
			return q
		},
		func(q *query.Query) *query.Query { effects++; return outside.Table("users") },
	} {
		spec := staticSpec()
		spec.ParentOpts = append(spec.ParentOpts, func(*writeOptions) { effects++ })
		spec.DeleteBefore = []NestedDelete{{Table: "users", Scopes: []Scope{scope}}}
		for _, tx := range []bool{false, true} {
			var err error
			if tx {
				_, err = ReplaceNestedCollectionTx(nil, db, spec)
			} else {
				_, err = ReplaceNestedCollection(nil, db, spec)
			}
			if !errors.Is(err, ErrUnsupportedCompound) || effects != 0 || len(spy.calls) != 0 {
				t.Fatal(err, effects, spy.calls)
			}
		}
	}
}

func TestCompoundPrivatePlanAndDestination(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			spy := &genericSpy{}
			db := NewDBWithExecutor(spy, genericDialect(dialect), WithSettings(genericSettings(t, dialect, false)))
			spec := staticSpec()
			ctx := context.WithValue(context.Background(), struct{}{}, "context")
			p, err := prepareNested(ctx, db, spec)
			if err != nil {
				t.Fatal(err)
			}
			if len(spy.calls) != 0 || len(p.plan.steps) != 2 {
				t.Fatal(spy.calls, p)
			}
			for i, s := range p.plan.steps {
				if s.start != 0 || s.end != 1 {
					t.Fatal(s)
				}
				d := s.plan.Diagnostic.(*QueryPlan)
				d.SQL = "DROP TABLE users"
				d.Params = nil
				d.Blocked = true
				_ = i
			}
			spec.Children[0]["tenant_id"] = 2
			for i := range p.plan.steps {
				if err := p.plan.steps[i].exec(); err != nil {
					t.Fatal(err)
				}
			}
			if len(spy.calls) != 2 || spy.ctx != ctx || strings.Contains(spy.sql, "DROP") {
				t.Fatal(spy)
			}
			if err := p.plan.steps[0].exec(); !errors.Is(err, ErrBlockedOperation) {
				t.Fatal(err)
			}
			p, err = prepareNested(ctx, db, staticSpec())
			if err != nil {
				t.Fatal(err)
			}
			other, err := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
			if err != nil {
				t.Fatal(err)
			}
			destination := db.WithOptions(WithExecutionContext(other))
			if err = p.plan.rebind(ctx, destination); !errors.Is(err, ErrBlockedOperation) {
				t.Fatal(err)
			}
			if len(spy.calls) != 2 {
				t.Fatal(spy.calls)
			}
		})
	}
}

func TestInsertOnceLookupRefusalPrecedesInsert(t *testing.T) {
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", false)))
	// The insert has an explicit tenant. The lookup key does not bind the tenant.
	// A global conflict target is itself also unsupported under strict inspection.
	_, _, err := InsertOnceReturning[genericID](nil, db, map[string]any{"tenant_id": 1, "id": 3}, Table("users"), ConflictColumns("id"))
	if !errors.Is(err, ErrBlockedOperation) || len(spy.calls) != 0 {
		t.Fatalf("%v %v", err, spy.calls)
	}
	// In compatibility mode the later lookup can still fail a required predicate.
	policies, e := NewPolicySet(TablePolicy{Table: "users", RequiredFilterColumns: []string{"score"}, RequiredFilterMode: PolicyModeBlock})
	if e != nil {
		t.Fatal(e)
	}
	db = NewDBWithExecutor(spy, driver.PostgresDialect{}, WithSettings(NewSettings(policies, RiskConfig{}, ExecutionContext{})))
	_, _, err = InsertOnceReturning[genericID](nil, db, map[string]any{"id": 3}, Table("users"), ConflictColumns("id"))
	if err == nil || len(spy.calls) != 0 {
		t.Fatalf("%v %v", err, spy.calls)
	}
}

func TestCompatibilityScopeTimingAndDestination(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	otherSpy := &genericSpy{}
	other := NewDBWithExecutor(otherSpy, driver.PostgresDialect{})
	spec := staticSpec()
	called := 0
	spec.DeleteBefore = []NestedDelete{{Table: "users", Scopes: []Scope{nil, func(q *query.Query) *query.Query {
		called++
		if len(exec.statements) != 1 {
			t.Fatalf("scope position: %v", exec.statements)
		}
		spec.Children[0]["score"] = 99
		return nil
	}, func(q *query.Query) *query.Query { called++; return other.Table("users").Where("id", 7) }}}}
	_, err := ReplaceNestedCollection(context.Background(), db, spec)
	if err != nil {
		t.Fatal(err)
	}
	if called != 2 || len(exec.statements) != 3 || len(otherSpy.calls) != 0 {
		t.Fatal(called, exec.statements, otherSpy.calls)
	}
	if !strings.Contains(exec.statements[1].query, "`users`") || !hasArg(exec.statements[2].args, 99) {
		t.Fatal(exec.statements)
	}
}

func TestNestedSplitOptionsAndResults(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	exec.insertIDs = []int64{11, 29}
	count := 0
	spec := NestedCollectionReplace[struct{}, nestedDocumentRow, struct{}]{SkipParent: true, Children: []nestedDocumentRow{{StableRowKey: "a"}, {StableRowKey: "b"}}, AssignChildID: func(int, int64) {}, ChildOpts: []WriteOpt{func(o *writeOptions) { count++; ExpectAffected(2)(o) }}}
	p, err := prepareNested(nil, db, spec)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(exec.statements) != 0 || len(p.plan.steps) != 2 {
		t.Fatal(count, p)
	}
	for i, s := range p.plan.steps {
		if s.start != i || s.end != i+1 || len(s.input.Rows) != 1 {
			t.Fatal(s)
		}
	}
	out, err := executeNested(nil, db, spec, p)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !reflect.DeepEqual(out.ChildIDs, []int64{11, 29}) {
		t.Fatal(count, out)
	}
}

var _ Executor = (*sql.DB)(nil)
var _ Executor = (*sql.Tx)(nil)

func TestStaticNestedTransactionAndCallerOwnedTx(t *testing.T) {
	for _, wrapper := range []bool{false, true} {
		t.Run(map[bool]string{false: "caller-owned", true: "wrapper"}[wrapper], func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			db := NewDB(sqlDB, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", false)))
			mock.ExpectBegin()
			mock.ExpectExec(`INSERT INTO "users"`).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(`INSERT INTO "users"`).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			if wrapper {
				_, err = ReplaceNestedCollectionTx(context.Background(), db, staticSpec())
			} else {
				var tx Tx
				tx, err = db.BeginTx(context.Background(), nil)
				if err == nil {
					_, err = ReplaceNestedCollection(context.Background(), tx.DB, staticSpec())
					if err == nil {
						err = tx.Commit()
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNestedSplitFailureAndAggregateOption(t *testing.T) {
	for _, failure := range []string{"driver", "result", "affected"} {
		t.Run(failure, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			spy := &genericSpy{Executor: sqlDB}
			db := NewDBWithExecutor(spy, driver.MySQLDialect{})
			marker := errors.New("split failure")
			called := 0
			spec := NestedCollectionReplace[struct{}, nestedDocumentRow, struct{}]{SkipParent: true, Children: []nestedDocumentRow{{StableRowKey: "a"}, {StableRowKey: "b"}}, AssignChildID: func(int, int64) { called++ }, ChildOpts: []WriteOpt{ExpectAffected(2)}}
			mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(11, 1))
			second := mock.ExpectExec("INSERT INTO")
			switch failure {
			case "driver":
				second.WillReturnError(marker)
			case "result":
				second.WillReturnResult(sqlmock.NewErrorResult(marker))
			case "affected":
				second.WillReturnResult(sqlmock.NewResult(29, 0))
			}
			out, err := ReplaceNestedCollection(nil, db, spec)
			if failure == "affected" {
				if !errors.Is(err, ErrRowsAffected) {
					t.Fatal(err)
				}
			} else if !errors.Is(err, marker) {
				t.Fatal(err)
			}
			if len(spy.calls) != 2 || called != 0 || len(out.ChildIDs) != 0 {
				t.Fatal(spy.calls, called, out)
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNestedSplitNoRowsAsUsesAggregate(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	exec.rowsAffectedSet = true
	marker := errors.New("no inserted rows")
	_, err := ReplaceNestedCollection(nil, db, NestedCollectionReplace[struct{}, nestedDocumentRow, struct{}]{SkipParent: true, Children: []nestedDocumentRow{{StableRowKey: "a"}, {StableRowKey: "b"}}, AssignChildID: func(int, int64) { t.Fatal("assignment after zero rows") }, ChildOpts: []WriteOpt{NoRowsAs(marker)}})
	if !errors.Is(err, marker) || len(exec.statements) != 2 {
		t.Fatal(err, exec.statements)
	}
}

func TestCompatibilityScopeChangesLaterDelete(t *testing.T) {
	db, exec := newCaptureWriteDB(driver.MySQLDialect{})
	policies, err := NewPolicySet(TablePolicy{Table: "users", SoftDeleteColumn: "deleted"})
	if err != nil {
		t.Fatal(err)
	}
	db = db.WithOptions(WithPolicySet(policies))
	spec := staticSpec()
	calls := 0
	deletes := []NestedDelete{{Table: "users"}, {Table: "users"}}
	deletes[0].Scopes = []Scope{func(q *query.Query) *query.Query {
		calls++
		deletes[1].Scopes = []Scope{func(q *query.Query) *query.Query { calls++; return q.Where("id", 99) }}
		return q.Where("id", 7)
	}}
	spec.DeleteBefore = deletes
	_, err = ReplaceNestedCollection(nil, db, spec)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(exec.statements) != 4 || !hasArg(exec.statements[2].args, 99) {
		t.Fatal(calls, exec.statements)
	}
}

func TestInsertOnceAutomaticTenantSharedWithLookup(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	spy := &genericSpy{Executor: sqlDB}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithSettings(genericSettings(t, "postgres", true)))
	mock.ExpectQuery(`INSERT INTO "users".*DO NOTHING RETURNING "id"`).WithArgs(3, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT "id" FROM "users"`).WithArgs(1, 3, 1).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	out, inserted, err := InsertOnceReturning[genericID](context.Background(), db, map[string]any{"id": 3}, Table("users"), ConflictColumns("tenant_id", "id"))
	if err != nil || inserted || out.ID != 3 || len(spy.calls) != 2 {
		t.Fatal(out, inserted, err, spy.calls)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
