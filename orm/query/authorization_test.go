package query

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type authorizationSpy struct{ calls int }

func (s *authorizationSpy) Authorize(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
	s.calls++
	return r.AllowUntil(r.ExpiresAt()), nil
}

type authorizationHostileError struct{}

func (authorizationHostileError) Error() string          { panic("error callback") }
func (authorizationHostileError) Is(error) bool          { panic("is callback") }
func (authorizationHostileError) Unwrap() error          { panic("unwrap callback") }
func (authorizationHostileError) As(any) bool            { panic("as callback") }
func (authorizationHostileError) Format(fmt.State, rune) { panic("format callback") }

func authorizationFixture(t *testing.T) (authorizationMaterial, time.Time) {
	t.Helper()
	q, c, _, _ := bindingFixture(t, false)
	p, err := q.bindingPlan(nil, "select", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewAuthorization(&authorizationSpy{}, "subject-canary")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	settings, err := bindingSettings(c.Settings)
	if err != nil {
		t.Fatal(err)
	}
	op, err := q.bindingOperation("select", nil)
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := bindingInspection(p.execution.inspection)
	if err != nil {
		t.Fatal(err)
	}
	return authorizationMaterial{config: a, owner: q, executor: q.exec, binding: c.BindingContext, snapshot: p.execution.identity, settings: settings, operation: op, inspection: inspection, family: "select", tenant: int64(1), tenantPresent: true, ceiling: now.Add(time.Hour)}, now
}
func TestAuthorizationValidation(t *testing.T) {
	m, now := authorizationFixture(t)
	r, err := newAuthorizationRequest(m, now)
	if err != nil {
		t.Fatal(err)
	}
	d := r.AllowUntil(now.Add(time.Minute))
	if err := validateAuthorization(r, m, d, nil, now); err != nil {
		t.Fatal(err)
	}
	// Validation is deliberately non-consuming; execution consumption is PR2.
	if err := validateAuthorization(r, m, d, nil, now); err != nil {
		t.Fatal(err)
	}
	other, _ := newAuthorizationRequest(m, now)
	for _, tc := range []struct {
		name        string
		decision    AuthorizationDecision
		providerErr error
		at          time.Time
		want        error
	}{
		{"deny", r.Deny(), nil, now, ErrAuthorizationDenied},
		{"zero", AuthorizationDecision{}, nil, now, ErrAuthorizationInvalid},
		{"unknown", AuthorizationDecision{request: r.state, status: 99}, nil, now, ErrAuthorizationInvalid},
		{"other request", other.AllowUntil(d.expires), nil, now, ErrAuthorizationMismatch},
		{"error and allow", d, authorizationHostileError{}, now, ErrAuthorizationProvider},
		{"missing expiry", r.AllowUntil(time.Time{}), nil, now, ErrAuthorizationInvalid},
		{"over ceiling", r.AllowUntil(m.ceiling.Add(time.Nanosecond)), nil, now, ErrAuthorizationInvalid},
		{"before", d, nil, d.expires.Add(-time.Nanosecond), nil},
		{"equal", d, nil, d.expires, ErrAuthorizationExpired},
		{"after", d, nil, d.expires.Add(time.Nanosecond), ErrAuthorizationExpired},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateAuthorization(r, m, tc.decision, tc.providerErr, tc.at); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	for name, change := range map[string]func(*authorizationMaterial){
		"registration": func(m *authorizationMaterial) {
			m.config, _ = NewAuthorization(m.config.state.provider, m.config.state.subject)
		},
		"subject":    func(m *authorizationMaterial) { m.config, _ = NewAuthorization(m.config.state.provider, "other") },
		"owner":      func(m *authorizationMaterial) { q := *m.owner; m.owner = &q },
		"executor":   func(m *authorizationMaterial) { m.executor = &bindingSpy{} },
		"tenant":     func(m *authorizationMaterial) { m.tenant = int64(2) },
		"family":     func(m *authorizationMaterial) { m.family = "delete" },
		"settings":   func(m *authorizationMaterial) { m.settings = []byte("other") },
		"operation":  func(m *authorizationMaterial) { m.operation = []byte("other") },
		"inspection": func(m *authorizationMaterial) { m.inspection = []byte("other") },
		"snapshot": func(m *authorizationMaterial) {
			q := m.owner.Where("id", 3)
			p, e := q.bindingPlan(nil, "select", nil, nil, nil)
			if e != nil {
				t.Fatal(e)
			}
			m.snapshot = p.execution.identity
		},
		"scope": func(m *authorizationMaterial) {
			v := *m.binding.input
			v.Scope = "other"
			m.binding, _ = NewBindingContext(v)
		},
		"target": func(m *authorizationMaterial) {
			v := *m.binding.input
			v.Target = "other"
			m.binding, _ = NewBindingContext(v)
		},
		"dialect": func(m *authorizationMaterial) {
			v := *m.binding.input
			v.Dialect = "postgres"
			m.binding, _ = NewBindingContext(v)
		},
		"generation": func(m *authorizationMaterial) {
			v := *m.binding.input
			v.Generation = "other"
			m.binding, _ = NewBindingContext(v)
		},
		"context": func(m *authorizationMaterial) { m.ctx = context.Background() },
	} {
		t.Run(name, func(t *testing.T) {
			current := m
			change(&current)
			if got := validateAuthorization(r, current, d, nil, now); got != ErrAuthorizationMismatch {
				t.Fatalf("got %v", got)
			}
		})
	}
	missing := m
	missing.config = Authorization{}
	if got := validateAuthorization(r, missing, d, nil, now); got != ErrAuthorizationMissing {
		t.Fatal(got)
	}
}
func TestAuthorizationContextAndOwnership(t *testing.T) {
	m, now := authorizationFixture(t)
	tenant := map[string]any{"nested": []byte("tenant-canary")}
	m.tenant = tenant
	r, err := newAuthorizationRequest(m, now)
	if err != nil {
		t.Fatal(err)
	}
	tenant["nested"].([]byte)[0] = 'X'
	m.settings[0] ^= 1
	v, present := r.Tenant()
	if !present {
		t.Fatal("missing")
	}
	v.(map[string]any)["nested"].([]byte)[0] = 'Y'
	v, _ = r.Tenant()
	if string(v.(map[string]any)["nested"].([]byte)) != "tenant-canary" {
		t.Fatal("aliased tenant")
	}
	if r.Subject() != "subject-canary" || r.Scope() == "" || r.Target() != "app-db" || r.Dialect() != "mysql" || r.Operation() != "select" || r.ExpiresAt() != m.ceiling {
		t.Fatal("descriptor")
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	r, err = newAuthorizationRequest(m, now)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if got := validateAuthorization(r, m, r.AllowUntil(m.ceiling), nil, now); got != ErrAuthorizationCanceled {
		t.Fatal(got)
	}
	m.ctx = nil
	m.tenantPresent = false
	if _, err = newAuthorizationRequest(m, now); err != ErrAuthorizationInvalid {
		t.Fatal(err)
	}
	m.tenantPresent = true
	m.tenant = &authorizationHostileError{}
	if _, err = newAuthorizationRequest(m, now); err != ErrAuthorizationInvalid {
		t.Fatal(err)
	}
	m.tenant = nil
	if _, err = newAuthorizationRequest(m, now); err != nil {
		t.Fatal("explicit nil presence must remain distinct", err)
	}
	// Real future context deadline, deterministic validator time.
	deadline := time.Now().Add(time.Hour)
	m.ctx, cancel = context.WithDeadline(context.Background(), deadline)
	defer cancel()
	m.ceiling = deadline.Add(time.Hour)
	r, err = newAuthorizationRequest(m, deadline.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !r.ExpiresAt().Equal(deadline) {
		t.Fatal("deadline not capped")
	}
	if got := validateAuthorization(r, m, r.AllowUntil(deadline.Add(time.Nanosecond)), nil, deadline.Add(-time.Minute)); got != ErrAuthorizationInvalid {
		t.Fatal(got)
	}
}
func TestAuthorizationOpaqueAndSettings(t *testing.T) {
	m, now := authorizationFixture(t)
	r, _ := newAuthorizationRequest(m, now)
	d := r.AllowUntil(m.ceiling)
	a := m.config
	for _, v := range []any{a, &a, r, &r, d, &d, struct {
		R AuthorizationRequest
		D AuthorizationDecision
		A Authorization
	}{r, d, a}} {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
			s := fmt.Sprintf(verb, v)
			if strings.Contains(s, "canary") || strings.Contains(s, "app-db") {
				t.Fatal("secret leaked")
			}
		}
		if _, err := json.Marshal(v); err == nil {
			t.Fatal("serialized")
		}
	}
	if json.Unmarshal([]byte(`{}`), &a) == nil || a.state != nil {
		t.Fatal("config decode")
	}
	if json.Unmarshal([]byte(`{}`), &r) == nil || r.state != nil {
		t.Fatal("request decode")
	}
	if json.Unmarshal([]byte(`{}`), &d) == nil || d.request != nil {
		t.Fatal("decision decode")
	}
	p := &authorizationSpy{}
	for _, subject := range []string{"", "   ", strings.Repeat("x", 257)} {
		if _, err := NewAuthorization(p, subject); err != ErrAuthorizationInvalid {
			t.Fatal(err)
		}
	}
	var nilProvider *authorizationSpy
	for _, p := range []AuthorizationProvider{nil, nilProvider} {
		if _, err := NewAuthorization(p, "subject"); err != ErrAuthorizationInvalid {
			t.Fatal(err)
		}
	}
	one, _ := NewAuthorization(p, " a ")
	two, _ := NewAuthorization(p, " a ")
	if one == two || one.state.subject != " a " {
		t.Fatal("registration identity or normalization")
	}
	s := Settings{}.WithAuthorization(one)
	copy := s
	if copy.Authorization() != one || s.WithAuthorization(two).Authorization() == one || s.Authorization() != one {
		t.Fatal("settings mutation")
	}
}
func TestAuthorizationPlanningAndLowRiskRemainDisconnected(t *testing.T) {
	for _, pg := range []bool{false, true} {
		q, c, spy, mock := bindingFixture(t, pg)
		provider := &authorizationSpy{}
		a, _ := NewAuthorization(provider, "subject")
		c.Settings = c.Settings.WithAuthorization(a)
		q.settings = c.Settings
		if _, err := q.Plan(nil); err != nil {
			t.Fatal(err)
		}
		h, _, err := q.ValidateSelect(nil, c, bindingExpiry())
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
		var rows []struct {
			ID int `db:"id"`
		}
		if err = q.ExecuteValidatedSelect(nil, c, h, &rows); err != nil {
			t.Fatal(err)
		}
		spy.calls = [6]int{}
		high := RiskHigh
		c.Settings = c.Settings.WithRiskConfig(RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &high}}})
		q = NewWithSettings(spy, "users", testDialect(pg), c.Settings).Select("id").Where("tenant_id", 1).RequireApproval("intent")
		_, _ = q.Plan(nil)
		if h, p, err := q.ValidateSelect(nil, c, bindingExpiry()); err != nil || h == nil || !p.Blocked {
			t.Fatal("pending high risk candidate unavailable", err)
		}
		if err = q.Get(&rows); err == nil {
			t.Fatal("high risk executed")
		}
		if spy.calls != [6]int{} || provider.calls != 0 {
			t.Fatal("provider or executor called")
		}
		if err = mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

// Identical adapter descriptors do not imply identical private operations or
// statement/value-specific external approval. Neither operation executes here.
func TestAuthorizationSameDescriptorDifferentUpdate(t *testing.T) {
	for _, pg := range []bool{false, true} {
		t.Run(driverName(pg), func(t *testing.T) {
			q, c, spy, _ := bindingFixture(t, pg)
			m, now := authorizationFixture(t)
			m.owner, m.executor, m.binding, m.family = q, q.exec, c.BindingContext, "update"
			var err error
			m.settings, err = bindingSettings(c.Settings)
			if err != nil {
				t.Fatal(err)
			}
			m.operation, err = q.bindingOperation("update", nil)
			if err != nil {
				t.Fatal(err)
			}
			material := func(score int32) authorizationMaterial {
				p, err := q.bindingPlan(nil, "update", map[string]any{"score": score}, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				n := m
				n.snapshot = p.execution.identity
				n.inspection, err = bindingInspection(p.execution.inspection)
				if err != nil {
					t.Fatal(err)
				}
				return n
			}
			one, two := material(3), material(4)
			r1, err := newAuthorizationRequest(one, now)
			if err != nil {
				t.Fatal(err)
			}
			r2, err := newAuthorizationRequest(two, now)
			if err != nil {
				t.Fatal(err)
			}
			descriptor := func(r AuthorizationRequest) []any {
				v, present := r.Tenant()
				return []any{r.Operation(), r.Subject(), r.Scope(), r.Target(), r.Dialect(), v, present, r.ExpiresAt()}
			}
			if !reflect.DeepEqual(descriptor(r1), descriptor(r2)) || one.snapshot.Equal(two.snapshot) {
				t.Fatal("fixture must have identical descriptors and different private values")
			}
			d := r1.AllowUntil(one.ceiling)
			if err := validateAuthorization(r1, one, d, nil, now); err != nil {
				t.Fatal(err)
			}
			if err := validateAuthorization(r1, two, d, nil, now); err != ErrAuthorizationMismatch {
				t.Fatal("changed current values accepted")
			}
			if err := validateAuthorization(r2, two, d, nil, now); err != ErrAuthorizationMismatch {
				t.Fatal("old decision accepted for new request")
			}
			if err := validateAuthorization(r2, two, r2.AllowUntil(two.ceiling), nil, now); err != nil {
				t.Fatal(err)
			}
			if spy.calls != [6]int{} {
				t.Fatal("validation executed SQL")
			}
		})
	}
}

func TestAuthorizationDecodeLifecycle(t *testing.T) {
	m, now := authorizationFixture(t)
	r, err := newAuthorizationRequest(m, now)
	if err != nil {
		t.Fatal(err)
	}
	d, a := r.AllowUntil(m.ceiling), m.config
	rAlias, dAlias, aAlias := r, d, a
	// encoding/json rejects malformed syntax before calling UnmarshalJSON.
	for _, receiver := range []any{&r, &d, &a} {
		if json.Unmarshal([]byte(`{"unfinished":`), receiver) == nil {
			t.Fatal("syntax accepted")
		}
	}
	if r != rAlias || d != dAlias || a != aAlias {
		t.Fatal("unexpected pre-method mutation")
	}
	// Discard failed receivers. Copies are not globally revoked by decoding.
	r, d, a = AuthorizationRequest{}, AuthorizationDecision{}, Authorization{}
	for _, receiver := range []any{&rAlias, &dAlias, &aAlias} {
		if json.Unmarshal([]byte(`null`), receiver) != ErrAuthorizationInvalid {
			t.Fatal("method decode accepted")
		}
	}
	if rAlias.state != nil || dAlias.request != nil || aAlias.state != nil {
		t.Fatal("method did not clear receiver")
	}
	r, err = newAuthorizationRequest(m, now)
	if err != nil {
		t.Fatal(err)
	}
	rAlias = r
	if json.Unmarshal([]byte(`{}`), &r) != ErrAuthorizationInvalid || r.state != nil || rAlias.state == nil {
		t.Fatal("receiver reset must not revoke independent aliases")
	}
	var rp *AuthorizationRequest
	var dp *AuthorizationDecision
	var ap *Authorization
	for _, v := range []any{rp, dp, ap} {
		b, err := json.Marshal(v)
		if err != nil || string(b) != "null" {
			t.Fatal("nil pointer JSON contract")
		}
	}
}

func TestAuthorizationShortenedCurrentDeadline(t *testing.T) {
	m, now := authorizationFixture(t)
	r, err := newAuthorizationRequest(m, now)
	if err != nil {
		t.Fatal(err)
	}
	d := r.AllowUntil(now.Add(time.Minute))
	current := m
	current.ceiling = d.expires.Add(-time.Nanosecond)
	if err := validateAuthorization(r, current, d, nil, now); err != ErrAuthorizationInvalid {
		t.Fatal("decision exceeded shortened current bound")
	}
	if err := validateAuthorization(r, current, r.AllowUntil(current.ceiling), nil, now); err != nil {
		t.Fatal(err)
	}
	if err := validateAuthorization(r, current, d, nil, current.ceiling); err != ErrAuthorizationExpired {
		t.Fatal("expired current bound accepted")
	}
}
