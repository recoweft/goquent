package query

import (
	"context"
	"errors"
	"testing"
	"time"
)

func warningCodeSet(warnings []Warning) map[string]bool {
	codes := make(map[string]bool, len(warnings))
	for _, warning := range warnings {
		codes[warning.Code] = true
	}
	return codes
}

func TestRiskEngineSelectWarnings(t *testing.T) {
	plan, err := newPlanTestQuery(&recordingExec{}).Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	codes := warningCodeSet(plan.Warnings)
	if !codes[WarningSelectStarUsed] {
		t.Fatalf("missing %s in %#v", WarningSelectStarUsed, plan.Warnings)
	}
	if !codes[WarningLimitMissing] {
		t.Fatalf("missing %s in %#v", WarningLimitMissing, plan.Warnings)
	}
	if plan.RiskLevel != RiskMedium {
		t.Fatalf("risk=%s", plan.RiskLevel)
	}
	if plan.RequiredApproval {
		t.Fatal("medium risk select should not require approval")
	}
}

func TestRiskEngineBlocksUpdateAndDeleteWithoutWhere(t *testing.T) {
	ctx := context.Background()

	updatePlan, err := newPlanTestQuery(&recordingExec{}).PlanUpdate(ctx, map[string]any{"age": 31})
	if err != nil {
		t.Fatalf("PlanUpdate: %v", err)
	}
	if !updatePlan.Blocked || updatePlan.RiskLevel != RiskBlocked {
		t.Fatalf("update blocked=%v risk=%s warnings=%#v", updatePlan.Blocked, updatePlan.RiskLevel, updatePlan.Warnings)
	}
	if !warningCodeSet(updatePlan.Warnings)[WarningUpdateWithoutWhere] {
		t.Fatalf("missing update warning: %#v", updatePlan.Warnings)
	}

	exec := &recordingExec{}
	_, err = newPlanTestQuery(exec).Delete()
	if !errors.Is(err, ErrBlockedOperation) {
		t.Fatalf("Delete error=%v", err)
	}
	if exec.calls != 0 {
		t.Fatalf("blocked delete executed database call count=%d", exec.calls)
	}
}

func TestRiskEngineDoesNotTreatForeignIDAsPrimaryKey(t *testing.T) {
	plan := &QueryPlan{
		Operation: OperationUpdate,
		SQL:       "UPDATE users SET name = ? WHERE tenant_id = ?",
		Tables:    []TableRef{{Name: "users"}},
		Predicates: []PredicateRef{{
			Column:   "tenant_id",
			Operator: "=",
		}},
	}
	result := DefaultRiskEngine.CheckQuery(plan)
	if !warningCodeSet(result.Warnings)[WarningBulkUpdateDetected] {
		t.Fatalf("expected tenant_id-only update to remain bulk, got %#v", result.Warnings)
	}
}

func TestRiskEngineUsesTableRiskMetadataForCompositeUniqueKeys(t *testing.T) {
	plan := &QueryPlan{
		Operation: OperationUpdate,
		SQL:       "UPDATE users SET name = ? WHERE tenant_id = ? AND external_id = ?",
		Tables:    []TableRef{{Name: "users"}},
		Predicates: []PredicateRef{
			{Column: "tenant_id", Operator: "="},
			{Column: "external_id", Operator: "="},
		},
	}
	AttachTableRiskMetadata(plan, []TableRiskMetadata{{
		Table:         "users",
		UniqueIndexes: [][]string{{"tenant_id", "external_id"}},
		TenantColumn:  "tenant_id",
	}})

	result := DefaultRiskEngine.CheckQuery(plan)
	if warningCodeSet(result.Warnings)[WarningBulkUpdateDetected] {
		t.Fatalf("expected composite unique predicate to be narrow, got %#v", result.Warnings)
	}
}

func TestApprovalRequiredAndProvided(t *testing.T) {
	exec := &recordingExec{}
	_, err := newPlanTestQuery(exec).
		WhereRaw("1 = 1", map[string]any{}).
		Update(map[string]any{"age": 31})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("Update error=%v", err)
	}
	if exec.calls != 0 {
		t.Fatalf("unapproved update executed database call count=%d", exec.calls)
	}

	exec = &recordingExec{}
	_, err = newPlanTestQuery(exec).
		WhereRaw("1 = 1", map[string]any{}).
		RequireApproval("operator reviewed weak predicate").
		Update(map[string]any{"age": 31})
	if err == nil || errors.Is(err, ErrApprovalRequired) || errors.Is(err, ErrBlockedOperation) {
		t.Fatalf("expected fake executor error after approval, got %v", err)
	}
	if exec.calls != 1 {
		t.Fatalf("approved update database call count=%d", exec.calls)
	}

	plan, err := newPlanTestQuery(&recordingExec{}).
		WhereRaw("1 = 1", map[string]any{}).
		RequireApproval("operator reviewed weak predicate").
		PlanUpdate(context.Background(), map[string]any{"age": 31})
	if err != nil {
		t.Fatalf("PlanUpdate with approval: %v", err)
	}
	if plan.Approval == nil || plan.Approval.Reason == "" {
		t.Fatalf("approval not stored on plan: %#v", plan.Approval)
	}
	if !plan.RequiredApproval {
		t.Fatal("weak predicate should still indicate approval is required")
	}
}

func TestRequireApprovalReasonRequired(t *testing.T) {
	_, err := newPlanTestQuery(&recordingExec{}).
		RequireApproval("  ").
		Plan(context.Background())
	if !errors.Is(err, ErrApprovalReasonRequired) {
		t.Fatalf("Plan error=%v", err)
	}
}

func TestUnsafeSQLStructureInputsRejected(t *testing.T) {
	if _, err := newPlanTestQuery(&recordingExec{}).
		Where("id", "= 1 OR 1=1 --", 1).
		Plan(context.Background()); err == nil {
		t.Fatal("expected invalid operator error")
	}
	if _, err := newPlanTestQuery(&recordingExec{}).
		OrderByRaw("id; DROP TABLE users").
		Plan(context.Background()); err == nil {
		t.Fatal("expected invalid raw SQL fragment error")
	}
	if _, err := newPlanTestQuery(&recordingExec{}).
		SafeWhereRaw("name = :name; DROP TABLE users", map[string]any{"name": "alice"}).
		Plan(context.Background()); err == nil {
		t.Fatal("expected invalid safe raw SQL fragment error")
	}
}

func TestSuppressionMVP(t *testing.T) {
	ctx := context.Background()

	t.Run("suppresses warning with reason", func(t *testing.T) {
		plan, err := newPlanTestQuery(&recordingExec{}).
			Select("id").
			SuppressWarning(WarningLimitMissing, "batch job intentionally scans all users").
			Plan(ctx)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		if len(plan.Warnings) != 0 {
			t.Fatalf("warnings=%#v", plan.Warnings)
		}
		if len(plan.SuppressedWarnings) != 1 || plan.SuppressedWarnings[0].Code != WarningLimitMissing {
			t.Fatalf("suppressed=%#v", plan.SuppressedWarnings)
		}
		if plan.RiskLevel != RiskLow {
			t.Fatalf("risk=%s", plan.RiskLevel)
		}
	})

	t.Run("expired suppression keeps original warning", func(t *testing.T) {
		expires := time.Now().UTC().Add(-time.Hour)
		plan, err := newPlanTestQuery(&recordingExec{}).
			Select("id").
			SuppressWarning(WarningLimitMissing, "old exception", SuppressionExpiresAt(expires)).
			Plan(ctx)
		if err != nil {
			t.Fatalf("Plan: %v", err)
		}
		codes := warningCodeSet(plan.Warnings)
		if !codes[WarningLimitMissing] || !codes[WarningSuppressionExpired] {
			t.Fatalf("warnings=%#v", plan.Warnings)
		}
	})

	t.Run("non suppressible warning is kept", func(t *testing.T) {
		plan, err := newPlanTestQuery(&recordingExec{}).
			SuppressWarning(WarningUpdateWithoutWhere, "legacy code path").
			PlanUpdate(ctx, map[string]any{"age": 31})
		if err != nil {
			t.Fatalf("PlanUpdate: %v", err)
		}
		codes := warningCodeSet(plan.Warnings)
		if !codes[WarningUpdateWithoutWhere] || !codes[WarningSuppressionNotAllowed] {
			t.Fatalf("warnings=%#v", plan.Warnings)
		}
		if !plan.Blocked {
			t.Fatal("non-suppressible blocked warning should remain blocked")
		}
	})
}

func TestInlineSuppressionParser(t *testing.T) {
	s, ok, err := ParseInlineSuppression(`// goquent:suppress LIMIT_MISSING reason="batch export" expires="2026-07-01" owner="platform"`)
	if err != nil {
		t.Fatalf("ParseInlineSuppression: %v", err)
	}
	if !ok {
		t.Fatal("expected suppression")
	}
	if s.Code != WarningLimitMissing || s.Reason != "batch export" || s.Owner != "platform" {
		t.Fatalf("suppression=%#v", s)
	}
	if s.ExpiresAt == nil || s.ExpiresAt.Format("2006-01-02") != "2026-07-01" {
		t.Fatalf("expires=%v", s.ExpiresAt)
	}

	_, ok, err = ParseInlineSuppression("// ordinary comment")
	if err != nil || ok {
		t.Fatalf("ordinary comment ok=%v err=%v", ok, err)
	}

	_, _, err = ParseInlineSuppression("// goquent:suppress LIMIT_MISSING")
	if err == nil {
		t.Fatal("expected reason required error")
	}
}

func TestRawAndDangerousSQLRisk(t *testing.T) {
	raw := NewRawPlan("DELETE FROM users WHERE id = ?", 10)
	if raw.RiskLevel != RiskHigh || !raw.RequiredApproval {
		t.Fatalf("raw risk=%s required=%v", raw.RiskLevel, raw.RequiredApproval)
	}
	if len(raw.Warnings) != 1 || raw.Warnings[0].Code != WarningRawSQLUsed {
		t.Fatalf("raw warnings=%#v", raw.Warnings)
	}

	drop := NewRawPlan("DROP TABLE users")
	codes := warningCodeSet(drop.Warnings)
	if drop.RiskLevel != RiskDestructive || !drop.RequiredApproval {
		t.Fatalf("drop risk=%s required=%v warnings=%#v", drop.RiskLevel, drop.RequiredApproval, drop.Warnings)
	}
	if !codes[WarningRawSQLUsed] || !codes[WarningDestructiveSQL] {
		t.Fatalf("drop warnings=%#v", drop.Warnings)
	}
}

func TestRiskEngineConfigOverrides(t *testing.T) {
	plan, err := newPlanTestQuery(&recordingExec{}).Select("id").Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	disabled := false
	engine := NewRiskEngine(RiskConfig{
		Rules: map[string]RiskRuleConfig{
			WarningLimitMissing: {Enabled: &disabled},
		},
	})
	result := engine.CheckQuery(plan)
	if warningCodeSet(result.Warnings)[WarningLimitMissing] {
		t.Fatalf("expected limit warning disabled: %#v", result.Warnings)
	}

	high := RiskHigh
	notSuppressible := false
	engine = NewRiskEngine(RiskConfig{
		Rules: map[string]RiskRuleConfig{
			WarningLimitMissing: {
				Severity:     &high,
				Suppressible: &notSuppressible,
			},
		},
	})
	result = engine.CheckQuery(plan)
	if result.Level != RiskHigh || !result.RequiredApproval {
		t.Fatalf("level=%s required=%v warnings=%#v", result.Level, result.RequiredApproval, result.Warnings)
	}
	for _, warning := range result.Warnings {
		if warning.Code == WarningLimitMissing && warning.Suppressible {
			t.Fatalf("expected suppressible override: %#v", warning)
		}
	}
}
