package query

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/internal/observe"
)

type executionAuthorizer func(context.Context, AuthorizationRequest) (AuthorizationDecision, error)

func (f executionAuthorizer) Authorize(c context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
	return f(c, r)
}

func authorizationExecutionFixture(t *testing.T, pg bool, fn executionAuthorizer) (*Query, BindingCurrent, *bindingSpy, sqlmock.Sqlmock) {
	t.Helper()
	q, c, spy, m := bindingFixture(t, pg)
	high := RiskHigh
	c.Settings = c.Settings.WithRiskConfig(RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &high}, WarningBulkUpdateDetected: {Severity: &high}, WarningBulkDeleteDetected: {Severity: &high}}})
	if fn != nil {
		a, e := NewAuthorization(fn, "execution-subject")
		if e != nil {
			t.Fatal(e)
		}
		c.Settings = c.Settings.WithAuthorization(a)
	}
	q = NewWithSettings(spy, "users", testDialect(pg), c.Settings).Select("id").Where("tenant_id", 1).RequireApproval("recorded intent")
	return q, c, spy, m
}

func TestAuthorizationExecutionAllowAndOrdinaryRefusal(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, compat := range []bool{false, true} {
			for _, family := range []string{"select", "update", "delete"} {
				t.Run(fmt.Sprint(pg, compat, family), func(t *testing.T) {
					calls := 0
					q, c, spy, m := authorizationExecutionFixture(t, pg, func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
						calls++
						if r.Subject() != "execution-subject" || r.Operation() != family {
							t.Fatal("descriptor")
						}
						return r.AllowUntil(r.ExpiresAt()), nil
					})
					if compat {
						c.Settings.strict = false
						q.settings = c.Settings
					}
					data := map[string]any{"score": int32(7)}
					h, p, err := q.validateBinding(nil, c, bindingExpiry(), family, data, nil, nil)
					if err != nil || h == nil || !p.Blocked || p.execution != nil || calls != 0 || spy.calls != [6]int{} {
						t.Fatal("validation", err)
					}
					p.Blocked = false
					p.SQL = "forged"
					p.Approval = &Approval{Reason: "forged"}
					if family == "select" {
						var rows []struct{ ID int }
						err = q.Get(&rows)
					} else if family == "update" {
						_, err = q.Update(data)
					} else {
						_, err = q.Delete()
					}
					if err == nil || calls != 0 || spy.calls != [6]int{} {
						t.Fatal("ordinary bypass")
					}
					if family == "select" {
						m.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
					} else {
						m.ExpectExec("UPDATE|DELETE").WillReturnResult(sqlmock.NewResult(0, 1))
					}
					p, err = q.prepareBinding(nil, c, h, family, data, nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					if family == "select" {
						var out []struct{ ID int }
						err = q.executeRows(p, func(rows observe.Reader) error { return nil })
						_ = out
					} else {
						_, err = q.executeResult(p)
					}
					if err != nil || calls != 1 {
						t.Fatal("dispatch", err, calls)
					}
					if _, err = q.prepareBinding(nil, c, h, family, data, nil, nil); err != ErrBindingConsumed {
						t.Fatal("replay", err)
					}
					if err = m.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestAuthorizationExecutionRejectionsAndConsumption(t *testing.T) {
	for _, kind := range []string{"missing", "deny", "error", "zero", "expired", "excess", "old", "value", "settings", "context", "expiry", "reentry", "database"} {
		t.Run(kind, func(t *testing.T) {
			var q *Query
			var c BindingCurrent
			var h *ValidatedPlan
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			data := map[string]any{"score": int32(3)}
			fn := executionAuthorizer(func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
				calls++
				switch kind {
				case "deny":
					return r.Deny(), nil
				case "error":
					return r.AllowUntil(r.ExpiresAt()), authorizationHostileError{}
				case "zero":
					return AuthorizationDecision{}, nil
				case "expired":
					return r.AllowUntil(time.Now().Add(-time.Second)), nil
				case "excess":
					return r.AllowUntil(r.ExpiresAt().Add(time.Second)), nil
				case "old":
					other, _ := newAuthorizationRequest(r.state.material, time.Now())
					return other.AllowUntil(other.ExpiresAt()), nil
				case "value":
					data["score"] = int32(9)
				case "settings":
					a, _ := NewAuthorization(executionAuthorizer(func(context.Context, AuthorizationRequest) (AuthorizationDecision, error) {
						panic("new provider called")
					}), "replacement")
					q.settings = q.settings.WithAuthorization(a)
				case "context":
					cancel()
				case "expiry":
					h.state.expires = time.Now().Add(-time.Second)
				case "reentry":
					if _, err := q.ExecuteValidatedUpdate(ctx, c, h, data); err != ErrBindingConsumed {
						t.Fatal("reentry", err)
					}
				}
				return r.AllowUntil(r.ExpiresAt()), nil
			})
			if kind == "missing" {
				fn = nil
			}
			var spy *bindingSpy
			var m sqlmock.Sqlmock
			q, c, spy, m = authorizationExecutionFixture(t, false, fn)
			var err error
			h, _, err = q.ValidateUpdate(ctx, c, bindingExpiry(), data)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "database" {
				m.ExpectExec("UPDATE").WillReturnError(errors.New("database failure"))
			}
			if kind == "reentry" {
				m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
			}
			_, err = q.ExecuteValidatedUpdate(ctx, c, h, data)
			if kind == "reentry" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("accepted", kind)
			}
			if kind != "database" && kind != "reentry" && spy.calls != [6]int{} {
				t.Fatal("rejected dispatched")
			}
			if _, err = q.ExecuteValidatedUpdate(ctx, c, h, data); err != ErrBindingConsumed {
				t.Fatal("not consumed", err)
			}
			want := 1
			if kind == "missing" {
				want = 0
			}
			if calls != want {
				t.Fatal("provider count", calls)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAuthorizationConcurrentSingleAttempt(t *testing.T) {
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	q, c, spy, m := authorizationExecutionFixture(t, false, func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
		calls.Add(1)
		close(entered)
		<-release
		return r.AllowUntil(r.ExpiresAt()), nil
	})
	data := map[string]any{"score": int32(3)}
	h, _, err := q.ValidateUpdate(nil, c, bindingExpiry(), data)
	if err != nil {
		t.Fatal(err)
	}
	m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	first := make(chan error, 1)
	go func() { _, e := q.ExecuteValidatedUpdate(nil, c, h, data); first <- e }()
	<-entered
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		copy := *h
		go func() {
			defer wg.Done()
			if _, e := q.ExecuteValidatedUpdate(nil, c, &copy, data); e != ErrBindingConsumed {
				t.Error("loser", e)
			}
		}()
	}
	wg.Wait()
	close(release)
	if err = <-first; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || spy.calls[0] != 1 {
		t.Fatal("attempt counts")
	}
}

func TestAuthorizationFinalDispatchAndStructuralRefusal(t *testing.T) {
	q, c, spy, _ := authorizationExecutionFixture(t, false, func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
		return r.AllowUntil(r.ExpiresAt()), nil
	})
	data := map[string]any{"score": int32(3)}
	h, _, err := q.ValidateUpdate(nil, c, bindingExpiry(), data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := q.prepareBinding(nil, c, h, "update", data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Deterministic boundary: change the retained deadline after authorization,
	// before the dispatch hook. No sleep, clock call-count or scheduler assumption.
	h.state.expires = time.Now().Add(-time.Second)
	if err = p.execution.checkAuthorizationDispatch(); err != ErrAuthorizationExpired {
		t.Fatal(err)
	}
	if _, err = q.executeResult(p); err == nil || spy.calls != [6]int{} {
		t.Fatal("expired dispatched")
	}
	for _, kind := range []string{"tenant", "blocked", "schema", "raw"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			q, c, spy, _ := authorizationExecutionFixture(t, false, func(context.Context, AuthorizationRequest) (AuthorizationDecision, error) {
				calls++
				panic("structural provider")
			})
			switch kind {
			case "tenant":
				q = NewWithSettings(spy, "users", testDialect(false), c.Settings).Where("tenant_id", 2).RequireApproval("intent")
			case "schema":
				q.settings.schema = ApplicationSchema{}
				c.Settings = q.settings
			case "blocked":
				blocked := RiskBlocked
				q.settings = q.settings.WithRiskConfig(RiskConfig{Rules: map[string]RiskRuleConfig{WarningBulkUpdateDetected: {Severity: &blocked}}})
				c.Settings = q.settings
			case "raw":
				q.WhereRaw("1=1", nil)
			}
			if h, _, err := q.ValidateUpdate(nil, c, bindingExpiry(), data); err == nil || h != nil {
				t.Fatal("structural candidate")
			}
			if calls != 0 || spy.calls != [6]int{} {
				t.Fatal("structural effects")
			}
		})
	}
}

func TestAuthorizationBindingCurrentAndOwnerRefusal(t *testing.T) {
	for _, change := range []string{"registration", "subject", "scope", "generation", "key", "target", "dialect", "tenant", "schema", "policy", "owner", "context", "values"} {
		t.Run(change, func(t *testing.T) {
			calls := 0
			fn := executionAuthorizer(func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
				calls++
				return r.AllowUntil(r.ExpiresAt()), nil
			})
			q, c, spy, _ := authorizationExecutionFixture(t, false, fn)
			data := map[string]any{"score": int32(3)}
			h, _, err := q.ValidateUpdate(nil, c, bindingExpiry(), data)
			if err != nil {
				t.Fatal(err)
			}
			var ctx context.Context
			switch change {
			case "registration", "subject":
				subject := "execution-subject"
				if change == "subject" {
					subject = "other"
				}
				a, _ := NewAuthorization(fn, subject)
				c.Settings = c.Settings.WithAuthorization(a)
			case "scope", "generation", "key", "target", "dialect":
				b := *c.BindingContext.input
				b.Key = append([]byte(nil), b.Key...)
				switch change {
				case "scope":
					b.Scope += "x"
				case "generation":
					b.Generation += "x"
				case "key":
					b.Key[0]++
				case "target":
					b.Target += "x"
				case "dialect":
					b.Dialect = "postgres"
				}
				c.BindingContext, _ = NewBindingContext(b)
			case "tenant":
				ec, _ := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
				c.Settings = c.Settings.WithExecutionContext(ec)
			case "schema":
				c.Settings.schema = ApplicationSchema{}
			case "policy":
				c.Settings.policies = PolicySet{}
			case "owner":
				copy := *q
				q = &copy
			case "context":
				ctx = context.Background()
			case "values":
				data["score"] = int32(4)
			}
			if _, err = q.ExecuteValidatedUpdate(ctx, c, h, data); err == nil {
				t.Fatal("accepted changed binding")
			}
			if calls != 0 || spy.calls != [6]int{} {
				t.Fatal("mismatch effects")
			}
			if _, err = q.ExecuteValidatedUpdate(ctx, c, h, data); err != ErrBindingConsumed {
				t.Fatal("mismatch not consumed")
			}
		})
	}
}

func TestAuthorizationDecisionMinimumDeadline(t *testing.T) {
	for _, bound := range []string{"binding", "approval", "context", "stored-context"} {
		t.Run(bound, func(t *testing.T) {
			upper := time.Now().Add(time.Minute)
			var got time.Time
			q, c, _, m := authorizationExecutionFixture(t, false, func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
				got = r.ExpiresAt()
				return r.AllowUntil(r.ExpiresAt()), nil
			})
			expiry := upper.Add(time.Minute)
			var ctx context.Context
			switch bound {
			case "binding":
				expiry = upper
			case "approval":
				q.approval.ExpiresAt = &upper
			case "context":
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(context.Background(), upper)
				defer cancel()
			case "stored-context":
				stored, cancel := context.WithDeadline(context.Background(), upper)
				defer cancel()
				q.WithContext(stored)
			}
			data := map[string]any{"score": int32(3)}
			h, _, err := q.ValidateUpdate(ctx, c, expiry, data)
			if err != nil {
				t.Fatal(err)
			}
			m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
			if _, err = q.ExecuteValidatedUpdate(ctx, c, h, data); err != nil {
				t.Fatal(err)
			}
			if !got.Equal(upper) {
				t.Fatal("minimum bound not supplied")
			}
		})
	}
}

func TestAuthorizationFinalRefusalPreservesObservationBudget(t *testing.T) {
	limit := int64(1)
	ctx, state, err := observe.Start(context.Background(), observe.Options{QueryLimit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	q, c, spy, m := authorizationExecutionFixture(t, false, func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
		return r.AllowUntil(r.ExpiresAt()), nil
	})
	data := map[string]any{"score": int32(3)}
	h, _, err := q.ValidateUpdate(ctx, c, bindingExpiry(), data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := q.prepareBinding(ctx, c, h, "update", data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	h.state.expires = time.Now().Add(-time.Second)
	if _, err = p.execution.observe("exec"); err != ErrAuthorizationExpired {
		t.Fatal(err)
	}
	if spy.calls != [6]int{} {
		t.Fatal("expired executor")
	}
	h, _, err = q.ValidateUpdate(ctx, c, bindingExpiry(), data)
	if err != nil {
		t.Fatal(err)
	}
	m.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	if _, err = q.ExecuteValidatedUpdate(ctx, c, h, data); err != nil {
		t.Fatal("refusal consumed budget", err)
	}
	summary := state.Finish()
	if summary.Dispatched != 1 || summary.InFlight != 0 || summary.PreflightRejected != 1 {
		t.Fatal("observation correspondence")
	}
}
