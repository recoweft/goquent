package operation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/faciam-dev/goquent/orm/manifest"
	"github.com/faciam-dev/goquent/orm/query"
)

func TestCompileValidSelectSpec(t *testing.T) {
	limit := int64(50)
	m := testManifest(false)
	spec := OperationSpec{
		Operation: OperationSelect,
		Model:     "Order",
		Select:    []string{"id", "total"},
		Filters: []FilterSpec{
			{Field: "tenant_id", Op: "=", ValueRef: "current_tenant"},
			{Field: "created_at", Op: ">=", ValueRef: "start_date"},
		},
		OrderBy: []OrderSpec{{Field: "created_at", Direction: "desc"}},
		Limit:   &limit,
	}

	plan, err := Compile(context.Background(), spec, Options{
		Manifest: m,
		Values: map[string]any{
			"current_tenant": "tenant-1",
			"start_date":     "2026-04-01",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Operation != query.OperationSelect {
		t.Fatalf("expected select plan, got %s", plan.Operation)
	}
	if len(plan.Params) != 2 || plan.Params[0] != "tenant-1" || plan.Params[1] != "2026-04-01" {
		t.Fatalf("unexpected params: %#v", plan.Params)
	}
	if !strings.Contains(plan.SQL, "deleted_at") {
		t.Fatalf("expected soft delete predicate in SQL, got %s", plan.SQL)
	}
	if plan.RiskLevel != query.RiskLow {
		t.Fatalf("expected low risk plan, got %s warnings=%#v", plan.RiskLevel, plan.Warnings)
	}
}

func TestValidateRejectsUnknownModelAndField(t *testing.T) {
	m := testManifest(false)
	_, err := Validate(OperationSpec{Operation: OperationSelect, Model: "Missing", Select: []string{"id"}}, Options{Manifest: m})
	if !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("expected unknown model, got %v", err)
	}
	_, err = Validate(OperationSpec{Operation: OperationSelect, Model: "Order", Select: []string{"missing"}, Filters: []FilterSpec{{Field: "tenant_id", Op: "=", Value: "t"}}}, Options{Manifest: m})
	if !errors.Is(err, ErrUnknownField) {
		t.Fatalf("expected unknown field, got %v", err)
	}
}

func TestValidateRejectsUnsafeSpec(t *testing.T) {
	m := testManifest(false)
	_, err := Validate(OperationSpec{Operation: "update", Model: "Order", Select: []string{"id"}}, Options{Manifest: m})
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported operation, got %v", err)
	}
	_, err = Validate(OperationSpec{Operation: OperationSelect, Model: "Order", Select: []string{"sum(total)"}, Filters: []FilterSpec{{Field: "tenant_id", Op: "=", Value: "t"}}}, Options{Manifest: m})
	if !errors.Is(err, ErrUnknownField) {
		t.Fatalf("expected aggregate-like field rejection, got %v", err)
	}
	_, err = Validate(OperationSpec{Operation: OperationSelect, Model: "Order", Select: []string{"secret"}, Filters: []FilterSpec{{Field: "tenant_id", Op: "=", Value: "t"}}}, Options{Manifest: m})
	if !errors.Is(err, ErrForbiddenField) {
		t.Fatalf("expected forbidden field rejection, got %v", err)
	}
	_, err = Validate(OperationSpec{
		Operation: OperationSelect,
		Model:     "Order",
		Select:    []string{"id"},
		Filters:   []FilterSpec{{Field: "tenant_id", Op: "=", ValueRef: "current_tenant"}},
	}, Options{Manifest: m})
	if !errors.Is(err, ErrValueRefMissing) {
		t.Fatalf("expected missing value_ref rejection, got %v", err)
	}
}

func TestValidatePolicyAndPII(t *testing.T) {
	m := testManifest(false)
	_, err := Validate(OperationSpec{Operation: OperationSelect, Model: "Order", Select: []string{"id"}}, Options{Manifest: m})
	if !errors.Is(err, ErrRequiredFilterMissing) {
		t.Fatalf("expected missing tenant filter rejection, got %v", err)
	}

	_, err = Validate(OperationSpec{
		Operation: OperationSelect,
		Model:     "Order",
		Select:    []string{"email"},
		Filters:   []FilterSpec{{Field: "tenant_id", Op: "=", Value: "t"}},
	}, Options{Manifest: m})
	if !errors.Is(err, ErrPIIAccessReasonRequired) {
		t.Fatalf("expected PII access reason error, got %v", err)
	}

	warnings, err := Validate(OperationSpec{
		Operation:    OperationSelect,
		Model:        "Order",
		Select:       []string{"email"},
		Filters:      []FilterSpec{{Field: "tenant_id", Op: "=", Value: "t"}},
		AccessReason: "support lookup",
	}, Options{Manifest: m})
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(warnings, WarningOperationPIISelected) {
		t.Fatalf("expected PII warning, got %#v", warnings)
	}
}

func TestStaleManifestBehavior(t *testing.T) {
	m := testManifest(true)
	limit := int64(1)
	spec := OperationSpec{
		Operation: OperationSelect,
		Model:     "Order",
		Select:    []string{"id"},
		Filters:   []FilterSpec{{Field: "tenant_id", Op: "=", Value: "t"}},
		Limit:     &limit,
	}

	_, err := Compile(context.Background(), spec, Options{Manifest: m, RequireFreshManifest: true})
	if !errors.Is(err, ErrStaleManifest) {
		t.Fatalf("expected stale manifest error, got %v", err)
	}

	plan, err := Compile(context.Background(), spec, Options{Manifest: m})
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(plan.Warnings, WarningOperationStaleManifest) {
		t.Fatalf("expected stale manifest warning, got %#v", plan.Warnings)
	}
}

func TestWarnModeRequiredFilterAndMissingLimit(t *testing.T) {
	m := testManifest(false)
	m.Tables[0].Columns[1].RequiredFilter = false
	m.Tables[0].Columns[1].TenantScope = false
	m.Tables[0].Policies = []manifest.Policy{{Type: "tenant_scope", Column: "tenant_id", Mode: query.PolicyModeWarn}}
	warnings, err := Validate(OperationSpec{Operation: OperationSelect, Model: "Order", Select: []string{"id"}}, Options{Manifest: m})
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(warnings, WarningOperationRequiredFilter) {
		t.Fatalf("expected required filter warning, got %#v", warnings)
	}
	if !hasWarning(warnings, WarningOperationMissingLimit) {
		t.Fatalf("expected missing limit warning, got %#v", warnings)
	}
}

func TestJSONSchema(t *testing.T) {
	b, err := JSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["title"] != "Goquent OperationSpec" {
		t.Fatalf("unexpected schema title: %#v", decoded["title"])
	}

	var spec OperationSpec
	err = json.Unmarshal([]byte(`{"operation":"select","model":"Order","select":["id"],"join":[{"model":"User"}]}`), &spec)
	if !errors.Is(err, ErrUnsupportedOperation) {
		t.Fatalf("expected unsupported join field rejection, got %v", err)
	}
}

func testManifest(stale bool) *manifest.Manifest {
	m := &manifest.Manifest{
		Version:          manifest.Version,
		GeneratedAt:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: manifest.Generator,
		Dialect:          "mysql",
		Tables: []manifest.Table{{
			Name:  "orders",
			Model: "Order",
			Columns: []manifest.Column{
				{Name: "created_at", Type: "time.Time"},
				{Name: "tenant_id", Type: "string", TenantScope: true, RequiredFilter: true},
				{Name: "deleted_at", Type: "time.Time", Nullable: true, SoftDelete: true},
				{Name: "email", Type: "string", PII: true},
				{Name: "id", Type: "int64", Primary: true},
				{Name: "secret", Type: "string", Forbidden: true},
				{Name: "total", Type: "int64"},
			},
			Policies: []manifest.Policy{
				{Type: "tenant_scope", Column: "tenant_id", Mode: query.PolicyModeEnforce},
				{Type: "soft_delete", Column: "deleted_at", Mode: query.PolicyModeEnforce},
				{Type: "pii", Column: "email", Mode: query.PolicyModeWarn},
			},
		}},
	}
	if stale {
		m.Verification = &manifest.Verification{Fresh: false, CheckedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	}
	return m
}

func hasWarning(warnings []query.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
