package orm

import (
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

type policyUser struct{}

func (policyUser) TableName() string { return "users" }

func TestModelPolicyBuilderRegistersPolicy(t *testing.T) {
	ResetModelPolicies()
	t.Cleanup(ResetModelPolicies)

	err := Model(policyUser{}).
		TenantScoped("tenant_id").
		SoftDelete("deleted_at").
		PII("email", "phone").
		RequiredFilter("tenant_id").
		Register()
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	policy, ok := query.PolicyForTable("users")
	if !ok {
		t.Fatal("policy not registered")
	}
	if policy.TenantColumn != "tenant_id" {
		t.Fatalf("tenant column=%q", policy.TenantColumn)
	}
	if policy.SoftDeleteColumn != "deleted_at" {
		t.Fatalf("soft delete column=%q", policy.SoftDeleteColumn)
	}
	if len(policy.PIIColumns) != 2 || policy.PIIColumns[0] != "email" || policy.PIIColumns[1] != "phone" {
		t.Fatalf("pii=%#v", policy.PIIColumns)
	}
	if len(policy.RequiredFilterColumns) != 1 || policy.RequiredFilterColumns[0] != "tenant_id" {
		t.Fatalf("required filters=%#v", policy.RequiredFilterColumns)
	}
}
