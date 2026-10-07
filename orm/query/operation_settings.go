package query

// IsStrict reports the immutable profile, not authorization.
func (s Settings) IsStrict() bool { return s.strict }

// ApplicationTenantContext preserves application provenance and returns detached
// data through ExecutionContext.Input. It does not authenticate the application.
func (s Settings) ApplicationTenantContext() (ExecutionContext, error) {
	c := s.execution
	if !c.application || !c.input.TenantPresent || c.input.CurrentTenant == nil {
		return ExecutionContext{}, ErrUnsupportedExecutionContext
	}
	return c, nil
}
