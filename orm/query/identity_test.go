package query

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/planidentity"
)

func identityTestKey() planidentity.Key {
	return planidentity.Key{Secret: bytes.Repeat([]byte{0xA5}, 32), Scope: "fictional-test", Generation: "1"}
}
func TestPrivatePlannerIdentity(t *testing.T) {
	for _, d := range []struct {
		name    string
		dialect driver.Dialect
	}{{"mysql", driver.MySQLDialect{}}, {"postgres", driver.PostgresDialect{}}} {
		t.Run(d.name, func(t *testing.T) {
			e := &recordingExec{}
			settings := tenantTestSettings(t, d.name, false)
			plan := func(value any) *QueryPlan {
				p, err := NewWithSettings(e, "users", d.dialect, settings).Select("id").Where("tenant_id", 1).Where("id", value).OrderBy("id", "asc").Limit(5).Plan(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				return p
			}
			p := plan(int64(3))
			if p.execution.identityErr != nil {
				t.Fatalf("identity unavailable for %s: %v", p.SQL, p.execution.identityErr)
			}
			first, err := p.execution.identity.Identify(identityTestKey())
			if err != nil {
				t.Fatal(err)
			}
			same, _ := plan(int64(3)).execution.identity.Identify(identityTestKey())
			if first != same {
				t.Fatal("unstable")
			}
			changed, _ := plan(int64(4)).execution.identity.Identify(identityTestKey())
			if first.Shape != changed.Shape || first.Execution == changed.Execution {
				t.Fatal("shape/value domain")
			}
			typed, _ := plan(int32(3)).execution.identity.Identify(identityTestKey())
			if first.Execution == typed.Execution {
				t.Fatal("lost native width")
			}
			p.SQL = "tampered"
			p.Params = []any{"tampered"}
			p.RiskLevel = RiskLow
			p.Blocked = false
			p.Approval = &Approval{Reason: "tampered", CreatedAt: time.Now()}
			p.Metadata = map[string]any{"source": "tampered"}
			p.Tables = []TableRef{{Name: "other"}}
			p.Warnings = []Warning{{Code: "tampered"}}
			after, _ := p.execution.identity.Identify(identityTestKey())
			if first != after {
				t.Fatal("public mutation affected identity")
			}
			out, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{`"identity":`, "fictional-test", `"generation":`, `"execution_id":`, `"shape_id":`} {
				if bytes.Contains(out, []byte(bad)) {
					t.Fatalf("private field leaked: %s", bad)
				}
			}
			if e.calls != 0 {
				t.Fatal("planning executed")
			}
		})
	}
}

// These cases reach the method; syntax prevalidation is tested separately.
func TestDecodeMethodInvalidatesReceiverEvidence(t *testing.T) {
	for _, input := range []string{`{"version":1,"risk_level":"low","blocked":false,"approval":{"reason":"forged"}}`, `{"version":0}`, `{}`, `{"version":2}`, `{"version":1,"params":false}`, `{"version":null}`, `{"version":2,"version":1}`, `{"version":2,"Version":1}`, `{"version":true}`, `{"metadata":{"changed":1},"params":false}`} {
		t.Run(input, func(t *testing.T) {
			for _, contextual := range []bool{false, true} {
				spy := &recordingExec{}
				q := NewWithSettings(spy, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", false)).Select("id").Where("tenant_id", 1).Limit(5)
				var ctx context.Context
				if contextual {
					ctx = t.Context()
				}
				p, err := q.Plan(ctx)
				if err != nil {
					t.Fatal(err)
				}
				p.Metadata["old"] = []any{"retained on error"}
				before, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				alias := *p
				err = json.Unmarshal([]byte(input), p)
				if p.execution != nil || p.tenantEvidence != nil || p.writeEvidence != nil || p.conditionSource != nil || p.generatedConditions {
					t.Fatal("private evidence retained")
				}
				if alias.execution == nil {
					t.Fatal("unexpected alias revocation")
				}
				after, marshalErr := json.Marshal(p)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				if err != nil && !bytes.Equal(before, after) {
					t.Fatal("partial public mutation")
				}
				if err == nil && (p.SQL != "" || p.Params != nil || p.Metadata != nil) {
					t.Fatal("old public fields survived replacement")
				}
				if _, err = q.executeResult(p); !errors.Is(err, ErrBlockedOperation) {
					t.Fatal(err)
				}
				if err = q.executeRows(p, nil); !errors.Is(err, ErrBlockedOperation) {
					t.Fatal(err)
				}
				if _, err = q.executeOpenRow(p); !errors.Is(err, ErrBlockedOperation) {
					t.Fatal(err)
				}
				if spy.calls != 0 {
					t.Fatal("decode/refusal dispatched", spy.calls)
				}
			}
		})
	}
}

func TestDecodeSyntaxPrevalidationLeavesOriginalPlanUnchanged(t *testing.T) {
	for _, input := range []string{"", `{"version":`, `{} garbage`, `{} {}`} {
		t.Run(input, func(t *testing.T) {
			q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", false)).Select("id").Where("tenant_id", 1).Limit(5)
			p, err := q.Plan(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			old := *p
			if err = json.Unmarshal([]byte(input), p); err == nil {
				t.Fatal("syntax accepted")
			}
			if !reflect.DeepEqual(*p, old) || p.execution != old.execution {
				t.Fatal("prevalidation changed receiver")
			}
			if err = q.checkExecution(p); err != nil {
				t.Fatal("original plan lost its original lifecycle", err)
			}
			// A fresh receiver never inherits an earlier plan, even on failure.
			var fresh QueryPlan
			if json.Unmarshal([]byte(input), &fresh) == nil || fresh.execution != nil {
				t.Fatal("fresh syntax decode")
			}
			// Direct method calls bypass standard-library prevalidation.
			if p.UnmarshalJSON([]byte(input)) == nil || p.execution != nil {
				t.Fatal("direct method did not invalidate")
			}
		})
	}
}

func TestDecodeDecoderAndNullBoundaries(t *testing.T) {
	q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", false)).Select("id").Where("tenant_id", 1).Limit(5)
	p, err := q.Plan(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	old := p.execution
	d := json.NewDecoder(bytes.NewBufferString(`{"version":`))
	d.UseNumber()
	if d.Decode(p) == nil || p.execution != old {
		t.Fatal("decoder prevalidation boundary")
	}
	d = json.NewDecoder(bytes.NewBufferString(`{"version":2}`))
	d.UseNumber()
	if d.Decode(p) == nil || p.execution != nil {
		t.Fatal("decoder method boundary")
	}
	p, err = q.Plan(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	alias := p
	if err = json.Unmarshal([]byte(`null`), &p); err != nil || p != nil || alias.execution == nil {
		t.Fatal("pointer null contract", err)
	}
}

func TestIdentityUnavailableDoesNotChangeCompat(t *testing.T) {
	p, err := New(&recordingExec{}, "users", driver.MySQLDialect{}).Select("id").Limit(5).Plan(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if p.execution.identity != nil || !errors.Is(p.execution.identityErr, planidentity.ErrUnavailable) {
		t.Fatal("missing context accepted")
	}
	for _, build := range []func(*Query) *Query{func(q *Query) *Query { return q.SelectRaw("1") }, func(q *Query) *Query { return q.WhereRaw("id = 3", nil) }, func(q *Query) *Query { return q.OrderByRaw("id") }} {
		q := build(NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", false)).Select("id").Where("tenant_id", 1))
		p, e := q.Plan(t.Context())
		if e == nil && p.execution.identity != nil {
			t.Fatal("opaque accepted")
		}
	}
}

func TestPlannerIdentityContextAndOrdering(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			spy := &recordingExec{}
			base := tenantTestSettings(t, dialect, false)
			build := func(s Settings, change func(*Query) *Query) planidentity.IDs {
				q := NewWithSettings(spy, "users", testDialect(dialect == "postgres"), s).Select("id", "score").Where("tenant_id", s.execution.input.CurrentTenant).Where("id", int64(3)).Where("score", int64(4)).Limit(5)
				if change != nil {
					q = change(q)
				}
				p, e := q.Plan(t.Context())
				if e != nil {
					t.Fatal(e)
				}
				ids, e := p.execution.identity.Identify(identityTestKey())
				if e != nil {
					t.Fatalf("%s: %v (%v)", p.SQL, e, p.execution.identityErr)
				}
				return ids
			}
			first := build(base, nil)
			for name, change := range map[string]func(*Settings){
				"target": func(s *Settings) {
					input := s.schema.Input()
					input.Database = "other-db"
					s.schema, _ = NewApplicationSchema(input)
					s.database = input.Database
				},
				"tenant": func(s *Settings) {
					s.execution, _ = NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
				},
				"policy": func(s *Settings) {
					p := s.policies.Policies()
					p[0].RequiredFilterColumns = []string{"id"}
					s.policies, _ = NewPolicySet(p...)
				},
				"schema": func(s *Settings) {
					input := s.schema.Input()
					input.Tables[0].Columns = append(input.Tables[0].Columns, WriteKeyColumn{Name: "extra", DBType: "integer", Bits: 32})
					s.schema, _ = NewApplicationSchema(input)
				},
				"config": func(s *Settings) { s.risk.Environment = "other" },
			} {
				t.Run(name, func(t *testing.T) {
					s := base
					change(&s)
					if got := build(s, nil); got.Execution == first.Execution {
						t.Fatal("owner context ignored")
					}
				})
			}
			for name, change := range map[string]func(*Query) *Query{
				"projection": func(q *Query) *Query { return q.Select("score", "id") },
				"order":      func(q *Query) *Query { return q.OrderBy("id", "desc") },
				"condition":  func(q *Query) *Query { return q.WhereNot(func(g *Query) { g.Where("id", 9).OrWhere("score", 10) }) },
				"limit":      func(q *Query) *Query { return q.Limit(6) },
			} {
				t.Run(name, func(t *testing.T) {
					if build(base, change).Execution == first.Execution {
						t.Fatal("ordered structure ignored")
					}
				})
			}
			// Labels and diagnostic clocks do not confer provenance or affect semantics.
			s := base
			s.execution.input.Source = "other-label"
			if build(s, nil) != first {
				t.Fatal("source label became semantic")
			}
			// Rule maps are canonical regardless of insertion order.
			low := RiskLow
			a, b := base, base
			a.risk.Rules = map[string]RiskRuleConfig{}
			a.risk.Rules["A"] = RiskRuleConfig{Severity: &low}
			a.risk.Rules["B"] = RiskRuleConfig{Severity: &low}
			b.risk.Rules = map[string]RiskRuleConfig{}
			b.risk.Rules["B"] = RiskRuleConfig{Severity: &low}
			b.risk.Rules["A"] = RiskRuleConfig{Severity: &low}
			if build(a, nil) != build(b, nil) {
				t.Fatal("rule insertion order")
			}
			if spy.calls != 0 {
				t.Fatal("planning called executor")
			}
		})
	}
}

func TestPlannerWriteIdentityAndMapOrder(t *testing.T) {
	for _, pg := range []bool{false, true} {
		spy := &recordingExec{}
		q := NewWithSettings(spy, "users", testDialect(pg), tenantTestSettings(t, driverName(pg), false))
		a := map[string]any{}
		a["tenant_id"] = 1
		a["id"] = 2
		a["score"] = 3
		b := map[string]any{}
		b["score"] = 3
		b["id"] = 2
		b["tenant_id"] = 1
		p, e := q.PlanInsert(t.Context(), a)
		if e != nil {
			t.Fatal(e)
		}
		first, e := p.execution.identity.Identify(identityTestKey())
		if e != nil {
			t.Fatal(e, p.execution.identityErr)
		}
		p, e = q.PlanInsert(t.Context(), b)
		if e != nil {
			t.Fatal(e)
		}
		second, e := p.execution.identity.Identify(identityTestKey())
		if e != nil || first != second {
			t.Fatal("insert map order", e)
		}
		b["score"] = 4
		p, e = q.PlanInsert(t.Context(), b)
		if e != nil {
			t.Fatal(e)
		}
		changed, e := p.execution.identity.Identify(identityTestKey())
		if e != nil || first.Shape != changed.Shape || first.Execution == changed.Execution {
			t.Fatal("insert values", e)
		}
		if spy.calls != 0 {
			t.Fatal("insert plan executed")
		}
	}
}

func TestIdentityDoesNotExtendPrivateExpiry(t *testing.T) {
	spy := &recordingExec{}
	q := NewWithSettings(spy, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", false)).Select("id").Where("tenant_id", 1).Limit(5)
	p, e := q.Plan(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	before, e := p.execution.identity.Identify(identityTestKey())
	if e != nil {
		t.Fatal(e)
	}
	expired := time.Now().Add(-time.Hour)
	p.execution.expires = &expired
	p.Approval = &Approval{Reason: "display only", ExpiresAt: &expired}
	after, e := p.execution.identity.Identify(identityTestKey())
	if e != nil || before != after {
		t.Fatal("expiry became a semantic ID", e)
	}
	if _, e = q.executeResult(p); !errors.Is(e, ErrApprovalRequired) {
		t.Fatal("identity bypassed expiry", e)
	}
	if spy.calls != 0 {
		t.Fatal("expired dispatch")
	}
}

func TestInvalidWriteContextHasNoIdentity(t *testing.T) {
	q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", false)).Select("id").Where("tenant_id", 1).Limit(5).WithWriteKeyContext(WriteKeyContext{Constraints: make([]WriteKeyConstraint, 65)})
	p, e := q.Plan(t.Context())
	if e != nil {
		t.Fatal(e)
	}
	if p.execution.identity != nil || !errors.Is(p.execution.identityErr, planidentity.ErrUnavailable) {
		t.Fatal("invalid private key assertion was omitted from correspondence")
	}
}
