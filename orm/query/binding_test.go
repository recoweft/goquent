package query

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type bindingSpy struct {
	executor
	calls [6]int
	sql   string
	args  []any
}

func (s *bindingSpy) record(i int, q string, a []any) {
	s.calls[i]++
	s.sql = q
	s.args = append([]any(nil), a...)
}
func (s *bindingSpy) Exec(q string, a ...any) (sql.Result, error) {
	s.record(0, q, a)
	return s.executor.Exec(q, a...)
}
func (s *bindingSpy) ExecContext(c context.Context, q string, a ...any) (sql.Result, error) {
	s.record(1, q, a)
	return s.executor.ExecContext(c, q, a...)
}
func (s *bindingSpy) Query(q string, a ...any) (*sql.Rows, error) {
	s.record(2, q, a)
	return s.executor.Query(q, a...)
}
func (s *bindingSpy) QueryContext(c context.Context, q string, a ...any) (*sql.Rows, error) {
	s.record(3, q, a)
	return s.executor.QueryContext(c, q, a...)
}
func (s *bindingSpy) QueryRow(q string, a ...any) *sql.Row {
	s.record(4, q, a)
	return s.executor.QueryRow(q, a...)
}
func (s *bindingSpy) QueryRowContext(c context.Context, q string, a ...any) *sql.Row {
	s.record(5, q, a)
	return s.executor.QueryRowContext(c, q, a...)
}
func bindingFixture(t *testing.T, pg bool) (*Query, BindingCurrent, *bindingSpy, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	spy := &bindingSpy{executor: db}
	settings := tenantTestSettings(t, driverName(pg), false)
	bc, err := NewBindingContext(BindingContextInput{Key: bytes.Repeat([]byte{0xa5}, 32), Scope: "fictional-binding-test", Generation: "1", Target: "app-db", Dialect: driverName(pg)})
	if err != nil {
		t.Fatal(err)
	}
	q := NewWithSettings(spy, "users", testDialect(pg), settings).Select("id").Where("tenant_id", 1).Where("id", int64(2)).Limit(5)
	return q, BindingCurrent{Settings: settings, BindingContext: bc}, spy, m
}
func bindingExpiry() time.Time { return time.Now().Add(time.Minute) }
func bindingRow() map[string]any {
	return map[string]any{"tenant_id": 1, "id": int64(2), "score": int32(3)}
}
func TestBindingSixDispatchAndPrivateMaterial(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, contextual := range []bool{false, true} {
			for _, family := range []string{"select", "count", "insert", "batch", "update", "delete"} {
				t.Run(fmt.Sprintf("%v/%v/%s", pg, contextual, family), func(t *testing.T) {
					q, c, spy, m := bindingFixture(t, pg)
					var ctx context.Context
					if contextual {
						ctx = t.Context()
					}
					data := bindingRow()
					if family == "update" {
						data = map[string]any{"score": int32(3)}
					}
					batch := []map[string]any{bindingRow(), {"tenant_id": 1, "id": int64(4), "score": int32(5)}}
					var columns []string
					if family == "count" {
						columns = []string{"id"}
					}
					h, p, err := q.validateBinding(ctx, c, bindingExpiry(), family, data, batch, columns)
					if err != nil {
						t.Fatal(err)
					}
					if spy.calls != [6]int{} || p.execution != nil || p.tenantEvidence != nil {
						t.Fatal("plan dispatched or retained evidence")
					}
					view, viewErr := p.PublicView()
					if viewErr != nil {
						t.Fatal(viewErr)
					}
					view.Operation = viewCanaries[0]
					exercisePlanView(t, view)
					if spy.calls != [6]int{} || h.state.used.Load() {
						t.Fatal("view consumed handle or dispatched")
					}
					sqlText := p.SQL
					args := append([]any(nil), p.Params...)
					switch family {
					case "select", "count":
						m.ExpectQuery(regexp.QuoteMeta(sqlText)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
					default:
						m.ExpectExec(regexp.QuoteMeta(sqlText)).WillReturnResult(sqlmock.NewResult(2, 1))
					}
					p.SQL = "forged"
					p.Params = []any{"forged"}
					p.Blocked = false
					p.RiskLevel = RiskLow
					p.Approval = &Approval{Reason: "forged"}
					p.Tables = nil
					switch family {
					case "select":
						var out []struct{ ID int }
						err = q.ExecuteValidatedSelect(ctx, c, h, &out)
						if err == nil && (len(out) != 1 || out[0].ID != 2) {
							t.Fatal(out)
						}
					case "count":
						var n int64
						n, err = q.ExecuteValidatedCount(ctx, c, h, "id")
						if err == nil && n != 2 {
							t.Fatal(n)
						}
					case "insert":
						_, err = q.ExecuteValidatedInsert(ctx, c, h, data)
					case "batch":
						_, err = q.ExecuteValidatedInsertBatch(ctx, c, h, batch)
					case "update":
						_, err = q.ExecuteValidatedUpdate(ctx, c, h, data)
					case "delete":
						_, err = q.ExecuteValidatedDelete(ctx, c, h)
					}
					if err != nil {
						t.Fatal(err)
					}
					want := [6]int{}
					i := 0
					if family == "select" {
						i = 2
					}
					if family == "count" {
						i = 4
					}
					if contextual {
						i++
					}
					want[i] = 1
					if spy.calls != want || spy.sql != sqlText || !reflect.DeepEqual(spy.args, args) {
						t.Fatalf("dispatch mismatch: %v", spy.calls)
					}
					if _, err = q.prepareBinding(ctx, c, h, family, data, batch, []string{"id"}); !errors.Is(err, ErrBindingConsumed) {
						t.Fatal(err)
					}
					if err = m.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
func TestBindingRejectsCurrentChangesAndBurnsCopies(t *testing.T) {
	changes := map[string]func(*Query, *BindingCurrent){
		"key": func(q *Query, c *BindingCurrent) {
			v := *c.BindingContext.input
			v.Key = bytes.Repeat([]byte{0xb6}, 32)
			c.BindingContext, _ = NewBindingContext(v)
		},
		"scope": func(q *Query, c *BindingCurrent) {
			v := *c.BindingContext.input
			v.Scope = "other"
			c.BindingContext, _ = NewBindingContext(v)
		},
		"generation": func(q *Query, c *BindingCurrent) {
			v := *c.BindingContext.input
			v.Generation = "2"
			c.BindingContext, _ = NewBindingContext(v)
		},
		"target": func(q *Query, c *BindingCurrent) {
			v := *c.BindingContext.input
			v.Target = "other"
			c.BindingContext, _ = NewBindingContext(v)
		},
		"dialect": func(q *Query, c *BindingCurrent) {
			v := *c.BindingContext.input
			v.Dialect = "postgres"
			c.BindingContext, _ = NewBindingContext(v)
		},
		"tenant": func(q *Query, c *BindingCurrent) {
			c.Settings.execution, _ = NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
		},
		"policy": func(q *Query, c *BindingCurrent) {
			ps := c.Settings.policies.Policies()
			ps[0].RequiredFilterColumns = []string{"score"}
			c.Settings.policies, _ = NewPolicySet(ps...)
		},
		"schema": func(q *Query, c *BindingCurrent) {
			v := c.Settings.schema.Input()
			v.Tables[0].Columns[0].Bits = 64
			c.Settings.schema, _ = NewApplicationSchema(v)
		},
		"config": func(q *Query, c *BindingCurrent) {
			c.Settings = c.Settings.WithRiskConfig(RiskConfig{Environment: "changed"})
		},
		"missing":    func(q *Query, c *BindingCurrent) { c.BindingContext = BindingContext{} },
		"query":      func(q *Query, c *BindingCurrent) { q.Where("score", 3) },
		"order":      func(q *Query, c *BindingCurrent) { q.OrderBy("id", "desc") },
		"projection": func(q *Query, c *BindingCurrent) { q.Select("score") },
		"opaque":     func(q *Query, c *BindingCurrent) { q.WhereRaw("score = :n", map[string]any{"n": 3}) },
		"context":    func(q *Query, c *BindingCurrent) { q.WithContext(context.Background()) },
		"executor":   func(q *Query, c *BindingCurrent) { q.exec = &recordingExec{} },
		"unknown":    func(q *Query, c *BindingCurrent) { p := *q.policy; p.Version = 99; q.policy = &p },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			q, c, spy, _ := bindingFixture(t, false)
			h, _, err := q.ValidateSelect(nil, c, bindingExpiry())
			if err != nil {
				t.Fatal(err)
			}
			alias := *h
			old := c
			oldq := *q
			change(q, &c)
			var dest []struct{ ID int }
			if err = q.ExecuteValidatedSelect(nil, c, h, &dest); err == nil {
				t.Fatal("change accepted")
			}
			*q = oldq
			c = old
			if err = q.ExecuteValidatedSelect(nil, c, &alias, &dest); !errors.Is(err, ErrBindingConsumed) {
				t.Fatal(err)
			}
			if spy.calls != [6]int{} {
				t.Fatal("refusal dispatched", spy.calls)
			}
		})
	}
}
func TestBindingWriteInputsFamiliesAndUnusedClauses(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, batchMode := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%v", pg, batchMode), func(t *testing.T) {
				q, c, spy, m := bindingFixture(t, pg)
				data := bindingRow()
				batch := []map[string]any{data}
				family := "insert"
				if batchMode {
					family = "batch"
				}
				h, p, err := q.validateBinding(nil, c, bindingExpiry(), family, data, batch, nil)
				if err != nil {
					t.Fatal(err)
				}
				q.WhereRaw("unused = :n", map[string]any{"n": bindingOpaque{}}).SelectRaw("unused").OrderByRaw("unused")
				m.ExpectExec(regexp.QuoteMeta(p.SQL)).WillReturnResult(sqlmock.NewResult(1, 1))
				plan, err := q.prepareBinding(nil, c, h, family, data, batch, nil)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = q.executeResult(plan); err != nil {
					t.Fatal(err)
				}
				if spy.calls[0] != 1 || spy.sql != p.SQL || !reflect.DeepEqual(spy.args, p.Params) {
					t.Fatal("unused clause affected insert")
				}
				if _, _, err = q.ValidateSelect(nil, c, bindingExpiry()); err == nil {
					t.Fatal("used raw accepted")
				}
			})
		}
	}
	for name, change := range map[string]func(map[string]any){"value": func(v map[string]any) { v["score"] = int32(4) }, "type": func(v map[string]any) { v["score"] = int64(3) }} {
		t.Run(name, func(t *testing.T) {
			q, c, spy, _ := bindingFixture(t, false)
			data := bindingRow()
			h, _, err := q.ValidateInsert(nil, c, bindingExpiry(), data)
			if err != nil {
				t.Fatal(err)
			}
			change(data)
			if _, err = q.ExecuteValidatedInsert(nil, c, h, data); !errors.Is(err, ErrBindingMismatch) {
				t.Fatal(err)
			}
			if spy.calls != [6]int{} {
				t.Fatal(spy.calls)
			}
		})
	}
	for _, family := range []string{"select", "insert"} {
		q, c, spy, _ := bindingFixture(t, false)
		var h *ValidatedPlan
		var err error
		if family == "select" {
			h, _, err = q.ValidateSelect(nil, c, bindingExpiry())
		} else {
			h, _, err = q.ValidateInsert(nil, c, bindingExpiry(), bindingRow())
		}
		if err != nil {
			t.Fatal(err)
		}
		if family == "select" {
			_, err = q.ExecuteValidatedCount(nil, c, h, "id")
		} else {
			_, err = q.ExecuteValidatedInsertBatch(nil, c, h, []map[string]any{bindingRow()})
		}
		if !errors.Is(err, ErrBindingMismatch) || spy.calls != [6]int{} {
			t.Fatal(err, spy.calls)
		}
	}
	q, c, spy, _ := bindingFixture(t, false)
	h, _, err := q.ValidateCount(nil, c, bindingExpiry(), "id")
	if err != nil {
		t.Fatal(err)
	}
	_, err = q.ExecuteValidatedCount(nil, c, h, "score")
	if !errors.Is(err, ErrBindingMismatch) || spy.calls != [6]int{} {
		t.Fatal(err)
	}
}

type bindingOpaque struct{}

func (bindingOpaque) String() string               { panic("must not call String") }
func (bindingOpaque) MarshalJSON() ([]byte, error) { panic("must not call MarshalJSON") }
func TestBindingLifecycleAndSerialization(t *testing.T) {
	q, c, spy, m := bindingFixture(t, false)
	for _, key := range [][]byte{nil, make([]byte, 31)} {
		if _, err := NewBindingContext(BindingContextInput{Key: key, Scope: "s", Generation: "g", Target: "t", Dialect: "mysql"}); !errors.Is(err, ErrBindingContext) {
			t.Fatal(err)
		}
	}
	if _, _, err := q.ValidateSelect(nil, c, time.Time{}); !errors.Is(err, ErrBindingExpired) {
		t.Fatal(err)
	}
	h, p, err := q.ValidateSelect(nil, c, bindingExpiry())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []any{h, *h, c.BindingContext, &c.BindingContext} {
		if _, err = json.Marshal(v); err == nil {
			t.Fatal("serialized opaque value")
		}
		if bytes.Contains([]byte(fmt.Sprintf("%+v %#v", v, v)), []byte("fictional-binding-test")) {
			t.Fatal("context leak")
		}
	}
	for _, input := range []string{`{}`, `{"version":0}`, `{"version":1,"risk_level":"low","blocked":false,"approval":{"reason":"fake"}}`, `{"version":99}`} {
		var restored ValidatedPlan
		if json.Unmarshal([]byte(input), &restored) == nil || restored.state != nil {
			t.Fatal("JSON restored handle")
		}
		var diagnostic QueryPlan
		_ = json.Unmarshal([]byte(input), &diagnostic)
		if diagnostic.execution != nil {
			t.Fatal("JSON restored execution")
		}
	}
	h.state.expires = time.Now().Add(-time.Second)
	var out []struct{ ID int }
	if err = q.ExecuteValidatedSelect(nil, c, h, &out); !errors.Is(err, ErrBindingExpired) {
		t.Fatal(err)
	}
	if spy.calls != [6]int{} {
		t.Fatal(spy.calls)
	}
	h, p, err = q.ValidateSelect(nil, c, bindingExpiry())
	if err != nil {
		t.Fatal(err)
	}
	m.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnError(errors.New("fixture database failure"))
	if err = q.ExecuteValidatedSelect(nil, c, h, &out); err == nil {
		t.Fatal("lost DB error")
	}
	if err = q.ExecuteValidatedSelect(nil, c, h, &out); !errors.Is(err, ErrBindingConsumed) {
		t.Fatal(err)
	}
	q, c, spy, m = bindingFixture(t, false)
	h, p, err = q.ValidateInsert(nil, c, bindingExpiry(), bindingRow())
	if err != nil {
		t.Fatal(err)
	}
	m.ExpectExec(regexp.QuoteMeta(p.SQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := q.ExecuteValidatedInsert(nil, c, h, bindingRow()); errs <- e }()
	}
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		} else if !errors.Is(e, ErrBindingConsumed) {
			t.Fatal(e)
		}
	}
	if success != 1 || spy.calls[0] != 1 {
		t.Fatal(success, spy.calls)
	}
}

type bindingNonComparableExecutor struct {
	*bindingSpy
	padding []byte
}
type bindingNonComparableContext struct {
	context.Context
	padding []byte
}
type bindingCheckContext struct {
	context.Context
	calls int
	check func(int) error
}

func (c *bindingCheckContext) Err() error {
	c.calls++
	if c.check != nil {
		return c.check(c.calls)
	}
	return nil
}
func TestBindingContextOwnerAndLastDispatchChecks(t *testing.T) {
	for _, kind := range []string{"explicit", "fallback", "changed", "noncomparable", "copy", "nil", "zero", "cancel-last", "expiry-last", "noncomparable-executor"} {
		t.Run(kind, func(t *testing.T) {
			q, c, spy, m := bindingFixture(t, false)
			ctx := &bindingCheckContext{Context: context.Background()}
			var supplied context.Context = ctx
			if kind == "fallback" {
				q.WithContext(ctx)
				supplied = nil
			}
			if kind == "noncomparable" {
				supplied = bindingNonComparableContext{Context: ctx}
			}
			if kind == "noncomparable-executor" {
				q = NewWithSettings(bindingNonComparableExecutor{bindingSpy: spy}, "users", testDialect(false), c.Settings)
			}
			h, p, err := q.ValidateInsert(supplied, c, bindingExpiry(), bindingRow())
			if kind == "noncomparable" {
				if !errors.Is(err, ErrBindingUnavailable) || h != nil || spy.calls != [6]int{} {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx.calls = 0
			want := error(nil)
			switch kind {
			case "changed":
				q.WithContext(context.Background())
				want = ErrBindingOwner
			case "copy":
				copy := *q
				q = &copy
				want = ErrBindingOwner
			case "nil":
				h = nil
				want = ErrBindingUnavailable
			case "zero":
				h = &ValidatedPlan{}
				want = ErrBindingUnavailable
			case "cancel-last":
				ctx.check = func(n int) error {
					if n >= 3 {
						return context.Canceled
					}
					return nil
				}
				want = context.Canceled
			case "expiry-last":
				h.state.expires = time.Now().Add(time.Second)
				ctx.check = func(n int) error {
					if n == 3 {
						time.Sleep(time.Until(h.state.expires) + time.Millisecond)
					}
					return nil
				}
				want = ErrBindingExpired
			}
			if want == nil {
				m.ExpectExec(regexp.QuoteMeta(p.SQL)).WillReturnResult(sqlmock.NewResult(1, 1))
			}
			_, err = q.ExecuteValidatedInsert(supplied, c, h, bindingRow())
			if want == nil {
				if err != nil || spy.calls[1] != 1 {
					t.Fatal(err, spy.calls)
				}
			} else if !errors.Is(err, want) || spy.calls != [6]int{} {
				t.Fatal(err, spy.calls)
			}
		})
	}
}
func TestBindingDetachedKeyBatchOrderAndEffectiveConfig(t *testing.T) {
	q, c, spy, m := bindingFixture(t, false)
	key := bytes.Repeat([]byte{0xaa}, 32)
	input := BindingContextInput{Key: key, Scope: "fictional", Generation: "1", Target: "app-db", Dialect: "mysql"}
	bc, err := NewBindingContext(input)
	if err != nil {
		t.Fatal(err)
	}
	c.BindingContext = bc
	h, p, err := q.ValidateInsert(nil, c, bindingExpiry(), bindingRow())
	if err != nil {
		t.Fatal(err)
	}
	key[0]++
	m.ExpectExec(regexp.QuoteMeta(p.SQL)).WillReturnResult(sqlmock.NewResult(1, 1))
	if _, err = q.ExecuteValidatedInsert(nil, c, h, bindingRow()); err != nil {
		t.Fatal(err)
	}
	if spy.calls[0] != 1 {
		t.Fatal(spy.calls)
	}
	for _, change := range []string{"batch-order", "required", "deleted", "keys", "query-policy"} {
		t.Run(change, func(t *testing.T) {
			q, c, spy, _ := bindingFixture(t, false)
			batch := []map[string]any{bindingRow(), {"tenant_id": 1, "id": int64(4), "score": int32(5)}}
			h, _, err := q.ValidateInsertBatch(nil, c, bindingExpiry(), batch)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "batch-order":
				batch[0], batch[1] = batch[1], batch[0]
			case "required":
				q.RequirePredicates(RequiredPredicate{Column: "tenant_id"})
			case "deleted":
				q.WithDeleted()
			case "keys":
				q.WithWriteKeyContext(WriteKeyContext{Database: "app-db", Dialect: "mysql", Table: "users"})
			case "query-policy":
				p := *q.policy
				p.ImmutableColumns = append(p.ImmutableColumns, "score")
				q.policy = &p
			}
			if _, err = q.ExecuteValidatedInsertBatch(nil, c, h, batch); err == nil || spy.calls != [6]int{} {
				t.Fatal(err, spy.calls)
			}
		})
	}
}

func TestBindingSharedJSONFixture(t *testing.T) {
	b, err := os.ReadFile("../../tests/contracts/testdata/validated_binding_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name       string
			Diagnostic json.RawMessage
			Readable   bool
		}
	}
	if err = json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, tc := range fixture.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			var p QueryPlan
			err := json.Unmarshal(tc.Diagnostic, &p)
			if (err == nil) != tc.Readable || p.execution != nil {
				t.Fatal(err)
			}
			var h ValidatedPlan
			if json.Unmarshal(tc.Diagnostic, &h) == nil || h.state != nil {
				t.Fatal("decoded handle")
			}
			q, c, spy, _ := bindingFixture(t, false)
			var dest []struct{ ID int }
			if err = q.ExecuteValidatedSelect(nil, c, &h, &dest); !errors.Is(err, ErrBindingUnavailable) || spy.calls != [6]int{} {
				t.Fatal(err, spy.calls)
			}
		})
	}
}

func TestBindingExplicitExpiryAndDiagnosticDetachment(t *testing.T) {
	q, c, spy, _ := bindingFixture(t, false)
	q = NewWithSettings(spy, "users", testDialect(false), c.Settings).Select("id").Where("tenant_id", 1).Limit(5).RequireApproval("fixture reason")
	upper := time.Now().Add(30 * time.Second)
	q.approval.ExpiresAt = &upper
	h, p, err := q.ValidateSelect(nil, c, upper)
	if err != nil {
		t.Fatal(err)
	}
	if !h.state.expires.Equal(upper) || p.Approval == nil || p.Approval.ExpiresAt == nil {
		t.Fatal("missing approval ceiling")
	}
	*p.Approval.ExpiresAt = time.Now().Add(-time.Hour)
	if !q.approval.ExpiresAt.Equal(upper) {
		t.Fatal("diagnostic modified Query approval")
	}
	later := time.Now().Add(time.Hour)
	q.approval.ExpiresAt = &later
	plan, err := q.prepareBinding(nil, c, h, "select", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.execution.bindingExpires.Equal(upper) {
		t.Fatal("fresh approval extended handle")
	}
	expired := time.Now().Add(-time.Second)
	plan.execution.bindingExpires = &expired
	if _, err = q.executeOpenRows(plan); !errors.Is(err, ErrBindingExpired) || spy.calls != [6]int{} {
		t.Fatal(err, spy.calls)
	}
}

func TestBindingPrivateApprovalDeadlineAndHighRiskRefusal(t *testing.T) {
	explicit := time.Now().Add(time.Minute)
	earlier := explicit.Add(-time.Second)
	later := explicit.Add(time.Second)
	if !bindingDeadline(explicit, nil).Equal(explicit) || !bindingDeadline(explicit, &earlier).Equal(earlier) || !bindingDeadline(explicit, &later).Equal(explicit) {
		t.Fatal("approval deadline not capped")
	}
	q, c, spy, _ := bindingFixture(t, false)
	high := RiskHigh
	c.Settings = c.Settings.WithRiskConfig(RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &high}}})
	q = NewWithSettings(spy, "users", testDialect(false), c.Settings).Select("id").Where("tenant_id", 1).RequireApproval("fixture reason")
	if h, _, err := q.ValidateSelect(nil, c, explicit); err == nil || h != nil || spy.calls != [6]int{} {
		t.Fatal("reason became high-risk authority", err)
	}
}
