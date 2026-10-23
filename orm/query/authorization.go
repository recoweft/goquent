package query

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/internal/planidentity"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

var (
	ErrAuthorizationMissing  = errors.New("goquent: authorization missing")
	ErrAuthorizationDenied   = errors.New("goquent: authorization denied")
	ErrAuthorizationProvider = errors.New("goquent: authorization provider failed")
	ErrAuthorizationInvalid  = errors.New("goquent: authorization invalid")
	ErrAuthorizationMismatch = errors.New("goquent: authorization mismatch")
	ErrAuthorizationExpired  = errors.New("goquent: authorization expired")
	ErrAuthorizationCanceled = errors.New("goquent: authorization canceled")
)

// AuthorizationProvider is an explicitly trusted application adapter. It must
// authenticate its application context and verify external authority before
// returning an allow candidate. PR1 does not invoke it from production paths.
// Implementations own their concurrency safety and external side effects.
type AuthorizationProvider interface {
	Authorize(context.Context, AuthorizationRequest) (AuthorizationDecision, error)
}

// Authorization is an immutable, process-local registration, not authority by
// itself. Copies share registration identity; each constructor creates a new one.
// Provider internals are not copied or made immutable by Goquent.
type Authorization struct{ state *authorizationConfig }
type authorizationConfig struct {
	provider AuthorizationProvider
	subject  string
}

// NewAuthorization registers an adapter and exact application subject (1..256
// bytes, not whitespace-only). Subject is not normalized or authenticated here.
func NewAuthorization(provider AuthorizationProvider, subject string) (Authorization, error) {
	if bindingNil(provider) || len(subject) > 256 || strings.TrimSpace(subject) == "" {
		return Authorization{}, ErrAuthorizationInvalid
	}
	return Authorization{&authorizationConfig{provider: provider, subject: subject}}, nil
}

func (s Settings) WithAuthorization(a Authorization) Settings { s.authorization = a; return s }
func (s Settings) Authorization() Authorization               { return s.authorization }

// AuthorizationRequest has no public constructor. Accessors are sensitive inputs
// for the trusted adapter, not display data. No SQL, key or digest is exposed.
type AuthorizationRequest struct{ state *authorizationRequest }
type authorizationRequest struct{ material authorizationMaterial }

// authorizationMaterial is supplied only by a private planner/binding boundary.
// The PR1 independent validator has no production caller. It does not bypass or
// replace structural gates, current-input inspection or execution consumption.
type authorizationMaterial struct {
	config                          Authorization
	owner                           *Query
	executor                        executor
	ctx, queryContext               context.Context
	binding                         BindingContext
	snapshot                        *planidentity.Snapshot
	settings, operation, inspection []byte
	family                          string
	tenant                          any
	tenantPresent                   bool
	ceiling                         time.Time
}

func (r AuthorizationRequest) Operation() string {
	if r.state == nil {
		return ""
	}
	return r.state.material.family
}
func (r AuthorizationRequest) Subject() string {
	if r.state == nil {
		return ""
	}
	return r.state.material.config.state.subject
}
func (r AuthorizationRequest) Scope() string {
	if r.state == nil {
		return ""
	}
	return r.state.material.binding.input.Scope
}
func (r AuthorizationRequest) Target() string {
	if r.state == nil {
		return ""
	}
	return r.state.material.binding.input.Target
}
func (r AuthorizationRequest) Dialect() string {
	if r.state == nil {
		return ""
	}
	return r.state.material.binding.input.Dialect
}
func (r AuthorizationRequest) Tenant() (any, bool) {
	if r.state == nil {
		return nil, false
	}
	v, _ := valuecopy.Copy(r.state.material.tenant)
	return v, r.state.material.tenantPresent
}
func (r AuthorizationRequest) ExpiresAt() time.Time {
	if r.state == nil {
		return time.Time{}
	}
	return r.state.material.ceiling
}

// AuthorizationDecision is a request-bound candidate, never an executable or
// serialized permission. Copies do not create another request or attempt.
type AuthorizationDecision struct {
	request *authorizationRequest
	status  uint8
	expires time.Time
}

// AllowUntil records the trusted adapter's decision. Validation still checks the
// exact request, current private material, mandatory expiry and context. This
// method does not authenticate the adapter or independently check external ACLs.
func (r AuthorizationRequest) AllowUntil(expiresAt time.Time) AuthorizationDecision {
	return AuthorizationDecision{r.state, 1, expiresAt}
}
func (r AuthorizationRequest) Deny() AuthorizationDecision {
	return AuthorizationDecision{request: r.state, status: 2}
}

func (Authorization) MarshalJSON() ([]byte, error) { return nil, ErrAuthorizationInvalid }
func (a *Authorization) UnmarshalJSON([]byte) error {
	if a != nil {
		*a = Authorization{}
	}
	return ErrAuthorizationInvalid
}
func (Authorization) Format(s fmt.State, _ rune)          { _, _ = s.Write([]byte("<authorization>")) }
func (AuthorizationRequest) MarshalJSON() ([]byte, error) { return nil, ErrAuthorizationInvalid }
func (r *AuthorizationRequest) UnmarshalJSON([]byte) error {
	if r != nil {
		*r = AuthorizationRequest{}
	}
	return ErrAuthorizationInvalid
}
func (AuthorizationRequest) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("<authorization request>"))
}
func (AuthorizationDecision) MarshalJSON() ([]byte, error) { return nil, ErrAuthorizationInvalid }
func (d *AuthorizationDecision) UnmarshalJSON([]byte) error {
	if d != nil {
		*d = AuthorizationDecision{}
	}
	return ErrAuthorizationInvalid
}
func (AuthorizationDecision) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("<authorization decision>"))
}

func authorizationContext(ctx context.Context) error {
	if !bindingComparable(ctx) || (ctx != nil && bindingNil(ctx)) {
		return ErrAuthorizationInvalid
	}
	if ctx != nil && ctx.Err() != nil {
		return ErrAuthorizationCanceled
	}
	return nil
}

func authorizationValid(m authorizationMaterial, now time.Time) error {
	if m.config.state == nil {
		return ErrAuthorizationMissing
	}
	if m.owner == nil || bindingNil(m.executor) || m.binding.input == nil || m.snapshot == nil || len(m.settings) == 0 || len(m.operation) == 0 || len(m.inspection) == 0 || !m.tenantPresent {
		return ErrAuthorizationInvalid
	}
	switch m.family {
	case "select", "count", "insert", "batch", "update", "delete":
	default:
		return ErrAuthorizationInvalid
	}
	if err := authorizationContext(m.ctx); err != nil {
		return err
	}
	if err := authorizationContext(m.queryContext); err != nil {
		return err
	}
	if m.ceiling.IsZero() {
		return ErrAuthorizationInvalid
	}
	if !m.ceiling.After(now) {
		return ErrAuthorizationExpired
	}
	return nil
}

func newAuthorizationRequest(m authorizationMaterial, now time.Time) (AuthorizationRequest, error) {
	if err := authorizationValid(m, now); err != nil {
		return AuthorizationRequest{}, err
	}
	v, ok := valuecopy.Copy(m.tenant)
	if !ok {
		return AuthorizationRequest{}, ErrAuthorizationInvalid
	}
	m.tenant = v
	m.settings = bytes.Clone(m.settings)
	m.operation = bytes.Clone(m.operation)
	m.inspection = bytes.Clone(m.inspection)
	for _, ctx := range []context.Context{m.ctx, m.queryContext} {
		if ctx != nil {
			if deadline, ok := ctx.Deadline(); ok && deadline.Before(m.ceiling) {
				m.ceiling = deadline
			}
		}
	}
	if !m.ceiling.After(now) {
		return AuthorizationRequest{}, ErrAuthorizationExpired
	}
	return AuthorizationRequest{&authorizationRequest{m}}, nil
}

func authorizationMatches(a, b authorizationMaterial) bool {
	if a.config.state != b.config.state || a.owner != b.owner || !bindingSame(a.executor, b.executor) || !bindingSame(a.ctx, b.ctx) || !bindingSame(a.queryContext, b.queryContext) || !bindingContextEqual(a.binding, b.binding) || a.family != b.family || a.tenantPresent != b.tenantPresent || !a.snapshot.Equal(b.snapshot) || !bytes.Equal(a.settings, b.settings) || !bytes.Equal(a.operation, b.operation) || !bytes.Equal(a.inspection, b.inspection) {
		return false
	}
	x, e := planidentity.Canonical(a.tenant)
	y, f := planidentity.Canonical(b.tenant)
	return e == nil && f == nil && bytes.Equal(x, y)
}

// validateAuthorization is a non-consuming check, not a dispatch gate. No error
// methods on providerErr are called and that error is never retained or unwrapped.
func validateAuthorization(r AuthorizationRequest, current authorizationMaterial, d AuthorizationDecision, providerErr error, now time.Time) error {
	if r.state == nil {
		return ErrAuthorizationMissing
	}
	if err := authorizationValid(current, now); err != nil {
		return err
	}
	if err := authorizationValid(r.state.material, now); err != nil {
		return err
	}
	if providerErr != nil {
		return ErrAuthorizationProvider
	}
	if d.request == nil || (d.status != 1 && d.status != 2) {
		return ErrAuthorizationInvalid
	}
	if d.request != r.state || !authorizationMatches(r.state.material, current) {
		return ErrAuthorizationMismatch
	}
	if d.status == 2 {
		return ErrAuthorizationDenied
	}
	if d.expires.IsZero() {
		return ErrAuthorizationInvalid
	}
	if d.expires.After(r.state.material.ceiling) || d.expires.After(current.ceiling) {
		return ErrAuthorizationInvalid
	}
	for _, ctx := range []context.Context{current.ctx, current.queryContext} {
		if ctx != nil {
			if deadline, ok := ctx.Deadline(); ok && d.expires.After(deadline) {
				return ErrAuthorizationInvalid
			}
		}
	}
	if !d.expires.After(now) {
		return ErrAuthorizationExpired
	}
	return nil
}
