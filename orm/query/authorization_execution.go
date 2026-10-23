package query

import (
	"context"
	"time"
)

// authorizationBindingMaterial uses only a freshly rebuilt private seal. The
// public diagnostic returned by Validate is never an input to this boundary.
func (q *Query) authorizationBindingMaterial(c BindingCurrent, s *bindingState, p *QueryPlan, family string) authorizationMaterial {
	e := p.execution
	return authorizationMaterial{config: q.settings.authorization, owner: q, executor: q.exec,
		ctx: e.ctx, queryContext: q.ctx, binding: c.BindingContext, snapshot: e.identity,
		settings: s.settings, operation: s.operation, inspection: s.inspection, family: family,
		tenant: q.settings.execution.input.CurrentTenant, tenantPresent: q.settings.execution.input.TenantPresent,
		ceiling: bindingDeadline(s.expires, e.expires)}
}

func (q *Query) authorizeBinding(ctx context.Context, c BindingCurrent, s *bindingState, p *QueryPlan, family string, data any, batch []map[string]any, columns []string) error {
	e := p.execution
	// An owned atomic recipe changes executor after Begin and performs storage
	// probes. This finite authorization path cannot authorize that second boundary.
	if (e.results.Atomic || e.results.Version != nil) && !e.managedTx {
		return ErrAuthorizationInvalid
	}
	m := q.authorizationBindingMaterial(c, s, p, family)
	request, err := newAuthorizationRequest(m, time.Now())
	if err != nil {
		return err
	}
	providerContext := m.ctx
	if providerContext == nil {
		providerContext = context.Background()
	}
	decision, providerErr := m.config.state.provider.Authorize(providerContext, request)
	// Discard the source error immediately, without calling any of its methods.
	if providerErr != nil {
		return ErrAuthorizationProvider
	}
	check := func() error {
		if err := authorizationContext(m.ctx); err != nil {
			return err
		}
		if err := authorizationContext(q.ctx); err != nil {
			return err
		}
		if !s.expires.After(time.Now()) || (e.expires != nil && !e.expires.After(time.Now())) {
			return ErrAuthorizationExpired
		}
		current, err := q.rebuildBinding(ctx, c, s, family, data, batch, columns)
		if err != nil {
			return ErrAuthorizationMismatch
		}
		return validateAuthorization(request, q.authorizationBindingMaterial(c, s, current, family), decision, nil, time.Now())
	}
	if err := check(); err != nil {
		return err
	}
	e.authorizationCheck = check
	return nil
}
