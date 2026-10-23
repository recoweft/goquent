package orm

import (
	"context"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

type ordinaryAuthorizationProvider struct{ calls int }

func (p *ordinaryAuthorizationProvider) Authorize(context.Context, query.AuthorizationRequest) (query.AuthorizationDecision, error) {
	p.calls++
	panic("ordinary provider invocation")
}

func TestAuthorizationOrdinaryFacadeRefusesWithoutFallback(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, method := range []string{"insert", "batch", "update", "upsert", "returning", "batch-returning", "scoped-update", "scoped-returning", "scoped-select", "raw-exec", "raw-query", "raw-row", "clone", "wrapper"} {
			t.Run(method+map[bool]string{true: "-pg", false: "-mysql"}[pg], func(t *testing.T) {
				p := &ordinaryAuthorizationProvider{}
				a, err := NewAuthorization(p, "ordinary-subject")
				if err != nil {
					t.Fatal(err)
				}
				spy := &genericSpy{}
				var dialect driver.Dialect = driver.MySQLDialect{}
				if pg {
					dialect = driver.PostgresDialect{}
				}
				// Quoted reserved table identifiers are conservatively high-risk even when
				// configured severity and reason would otherwise permit compatibility writes.
				db := NewDBWithExecutor(spy, dialect, WithAuthorization(a)).RequireRawApproval("not authority")
				q := db.Table("alter").Where("id", 1).RequireApproval("not authority")
				data := map[string]any{"id": 1, "active": true}
				opts := []WriteOpt{Table("alter")}
				switch method {
				case "insert":
					_, err = Insert(nil, db, data, opts...)
				case "batch":
					_, err = InsertMany(nil, db, []map[string]any{data}, opts...)
				case "update":
					_, err = Update(nil, db, data, append(opts, PK("id"), WherePK())...)
				case "upsert":
					_, err = Upsert(nil, db, data, append(opts, ConflictColumns("id"), UpdateColumns("active"))...)
				case "returning":
					_, err = InsertReturning[struct{ ID int }](nil, db, data, append(opts, Returning("id"))...)
				case "batch-returning":
					_, err = InsertManyReturning[struct{ ID int }](nil, db, []map[string]any{data}, append(opts, Returning("id"))...)
				case "scoped-update":
					_, err = UpdateBy(nil, q, data)
				case "scoped-returning":
					_, err = UpdateByReturning[struct{ ID int }](nil, db, q, data)
				case "scoped-select":
					_, err = SelectAllBy[struct{ ID int }](nil, db, q)
				case "raw-exec":
					_, err = db.Exec("UPDATE users SET active = ? WHERE id = ?", true, 1)
				case "raw-query":
					_, err = db.Query("SELECT id FROM users")
				case "raw-row":
					err = db.QueryRow("SELECT id FROM users").Scan(new(int))
				case "clone":
					_, err = db.Clone().Exec("SELECT id FROM users")
				case "wrapper":
					_, err = db.WrapExecutor(spy).Exec("SELECT id FROM users")
				}
				if err == nil || len(spy.calls) != 0 || p.calls != 0 {
					t.Fatal("ordinary authorization bypass", method)
				}
			})
		}
	}
}

func TestAuthorizationNestedAndHooksRetainGate(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		p := &ordinaryAuthorizationProvider{}
		a, _ := NewAuthorization(p, "compound-subject")
		spy := &genericSpy{}
		db := NewDBWithExecutor(spy, genericDialect(dialect), WithAuthorization(a))
		spec := staticSpec()
		spec.ParentOpts = []WriteOpt{Table("alter")}
		spec.ChildOpts = []WriteOpt{Table("alter")}
		if _, err := ReplaceNestedCollection(nil, db, spec); err == nil {
			t.Fatal("nested high risk accepted")
		}
		for _, hook := range []TransactionHook{InsertHook("high", map[string]any{"id": 1}, Table("alter")), InsertManyHook("high", []map[string]any{{"id": 1}}, Table("alter"))} {
			if err := hook.Run(context.Background(), Tx{DB: db}); err == nil {
				t.Fatal("hook high risk accepted")
			}
		}
		if p.calls != 0 || len(spy.calls) != 0 {
			t.Fatal("compound bypass")
		}
	}
}

func TestAuthorizationRiskDowngradeCannotPermitRaw(t *testing.T) {
	p := &ordinaryAuthorizationProvider{}
	a, _ := NewAuthorization(p, "risk-subject")
	disabled := false
	spy := &genericSpy{}
	db := NewDBWithExecutor(spy, driver.PostgresDialect{}, WithAuthorization(a), WithRiskConfig(RiskConfig{Rules: map[string]query.RiskRuleConfig{query.WarningRawSQLUsed: {Enabled: &disabled}}})).RequireRawApproval("not authority")
	if _, err := db.RawPlan(nil, "SELECT id FROM users"); err != nil {
		t.Fatal(err)
	}
	if p.calls != 0 || len(spy.calls) != 0 {
		t.Fatal("planning effects")
	}
	if _, err := db.Exec("SELECT id FROM users"); err == nil || p.calls != 0 || len(spy.calls) != 0 {
		t.Fatal("downgrade bypass")
	}
}
