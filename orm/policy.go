package orm

import (
	"strings"

	"github.com/recoweft/goquent/orm/model"
	"github.com/recoweft/goquent/orm/query"
)

// ModelPolicyBuilder registers table policy metadata for a model/table.
type ModelPolicyBuilder struct {
	policy query.TablePolicy
	err    error
}

// Model starts a policy declaration for v's table.
func Model(v any) *ModelPolicyBuilder {
	b := &ModelPolicyBuilder{policy: query.TablePolicy{Table: model.TableName(v)}}
	b.register()
	return b
}

// Table overrides the table name for this policy declaration.
func (b *ModelPolicyBuilder) Table(name string) *ModelPolicyBuilder {
	b.policy.Table = strings.TrimSpace(name)
	b.register()
	return b
}

// TenantScoped marks column as the tenant scope column.
func (b *ModelPolicyBuilder) TenantScoped(column string, mode ...PolicyMode) *ModelPolicyBuilder {
	b.policy.TenantColumn = strings.TrimSpace(column)
	if len(mode) > 0 {
		b.policy.TenantMode = mode[0]
	}
	b.register()
	return b
}

// SoftDelete marks column as the soft delete column.
func (b *ModelPolicyBuilder) SoftDelete(column string, mode ...PolicyMode) *ModelPolicyBuilder {
	b.policy.SoftDeleteColumn = strings.TrimSpace(column)
	if len(mode) > 0 {
		b.policy.SoftDeleteMode = mode[0]
	}
	b.register()
	return b
}

// PII marks columns as personally identifiable information.
func (b *ModelPolicyBuilder) PII(columns ...string) *ModelPolicyBuilder {
	b.policy.PIIColumns = append(b.policy.PIIColumns, columns...)
	b.register()
	return b
}

// RequiredFilter requires filters on the provided columns.
func (b *ModelPolicyBuilder) RequiredFilter(columns ...string) *ModelPolicyBuilder {
	b.policy.RequiredFilterColumns = append(b.policy.RequiredFilterColumns, columns...)
	b.register()
	return b
}

// PolicyMode sets all policy modes for this model declaration.
func (b *ModelPolicyBuilder) PolicyMode(mode PolicyMode) *ModelPolicyBuilder {
	b.policy.TenantMode = mode
	b.policy.SoftDeleteMode = mode
	b.policy.PIIMode = mode
	b.policy.RequiredFilterMode = mode
	b.register()
	return b
}

// Register explicitly registers the current policy.
func (b *ModelPolicyBuilder) Register() error {
	b.register()
	return b.err
}

// Err returns the last registration error.
func (b *ModelPolicyBuilder) Err() error {
	return b.err
}

func (b *ModelPolicyBuilder) register() {
	b.err = query.RegisterTablePolicy(b.policy)
}

// RegisterTablePolicy registers a table policy directly.
func RegisterTablePolicy(policy TablePolicy) error {
	return query.RegisterTablePolicy(policy)
}

// RegisteredTablePolicies returns all registered table policies in stable order.
func RegisteredTablePolicies() []TablePolicy {
	return query.RegisteredTablePolicies()
}

// ResetModelPolicies clears registered model policies. Intended for tests.
func ResetModelPolicies() {
	query.ResetPolicyRegistry()
}
