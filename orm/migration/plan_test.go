package migration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestPlanSQLClassifiesAddColumnRisk(t *testing.T) {
	plan, err := PlanSQL(`
ALTER TABLE users ADD COLUMN nickname text;
ALTER TABLE users ADD COLUMN status text NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN tenant_id uuid NOT NULL;
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].RiskLevel != query.RiskLow {
		t.Fatalf("expected nullable add column low risk, got %s", plan.Steps[0].RiskLevel)
	}
	if plan.Steps[1].RiskLevel != query.RiskMedium {
		t.Fatalf("expected not null with default medium risk, got %s", plan.Steps[1].RiskLevel)
	}
	if plan.Steps[2].RiskLevel != query.RiskHigh {
		t.Fatalf("expected not null without default high risk, got %s", plan.Steps[2].RiskLevel)
	}
	if !plan.RequiredApproval {
		t.Fatalf("expected high-risk migration to require approval")
	}
	if len(plan.Steps[2].Preflight) == 0 {
		t.Fatalf("expected preflight suggestions for high-risk add column")
	}
}

func TestPlanSQLClassifiesDestructiveMigrationAndApproval(t *testing.T) {
	plan, err := PlanSQL(`
DROP TABLE users;
ALTER TABLE accounts DROP COLUMN legacy_id;
`)
	if err != nil {
		t.Fatal(err)
	}
	if plan.RiskLevel != query.RiskDestructive {
		t.Fatalf("expected destructive plan, got %s", plan.RiskLevel)
	}
	if !plan.RequiredApproval {
		t.Fatalf("expected destructive plan to require approval")
	}
	if err := EnsureExecutable(plan); !errors.Is(err, query.ErrApprovalRequired) {
		t.Fatalf("expected approval required, got %v", err)
	}

	approved, err := New(plan.SQL).RequireApproval("legacy schema cleanup approved").Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := EnsureExecutable(approved); err != nil {
		t.Fatalf("expected approved destructive plan to be executable, got %v", err)
	}
	if len(approved.Steps[0].Preflight) == 0 || len(approved.Steps[1].Preflight) == 0 {
		t.Fatalf("expected destructive steps to include preflight suggestions")
	}
}

func TestPlanSQLClassifiesIndexAndUnsupportedDDL(t *testing.T) {
	plan, err := PlanSQL(`
CREATE INDEX users_email_idx ON users (email);
CREATE INDEX CONCURRENTLY users_name_idx ON users (name);
DROP INDEX users_old_idx;
GRANT SELECT ON users TO app_reader;
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Type != AddIndex || plan.Steps[0].RiskLevel != query.RiskMedium {
		t.Fatalf("expected non-concurrent add index medium risk, got %#v", plan.Steps[0])
	}
	if plan.Steps[1].Type != AddIndex || plan.Steps[1].RiskLevel != query.RiskLow {
		t.Fatalf("expected concurrent add index low risk, got %#v", plan.Steps[1])
	}
	if plan.Steps[2].Type != DropIndex || plan.Steps[2].RiskLevel != query.RiskMedium {
		t.Fatalf("expected drop index medium risk, got %#v", plan.Steps[2])
	}
	if plan.Steps[3].Type != UnsupportedStep || plan.Steps[3].AnalysisPrecision != query.AnalysisUnsupported {
		t.Fatalf("expected unsupported DDL warning, got %#v", plan.Steps[3])
	}
	if !plan.Blocked || plan.Steps[3].RiskLevel != query.RiskBlocked {
		t.Fatalf("expected unsupported DDL to block apply, got blocked=%v step=%#v", plan.Blocked, plan.Steps[3])
	}
}

func TestPlanSQLBlocksUnclassifiedDML(t *testing.T) {
	plan, err := PlanSQL(`
	DELETE FROM users;
	TRUNCATE TABLE users;
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 unsupported steps, got %#v", plan.Steps)
	}
	if !plan.Blocked || plan.RiskLevel != query.RiskBlocked {
		t.Fatalf("expected unclassified DML to block apply, risk=%s blocked=%v warnings=%#v", plan.RiskLevel, plan.Blocked, plan.Warnings)
	}
	if err := EnsureExecutable(plan); !errors.Is(err, query.ErrBlockedOperation) {
		t.Fatalf("expected blocked operation, got %v", err)
	}
}

func TestClassifyTypeChange(t *testing.T) {
	step := MigrationStep{Type: AlterColumnType, Table: "users", Column: "name", OldType: "varchar(32)", NewType: "varchar(255)", Line: 1}
	classifyStep(&step)
	if step.RiskLevel != query.RiskMedium {
		t.Fatalf("expected type expansion medium risk, got %s", step.RiskLevel)
	}

	step = MigrationStep{Type: AlterColumnType, Table: "users", Column: "name", OldType: "varchar(255)", NewType: "varchar(32)", Line: 1}
	classifyStep(&step)
	if step.RiskLevel != query.RiskDestructive {
		t.Fatalf("expected type narrowing destructive risk, got %s", step.RiskLevel)
	}
	if step.Warnings[0].Code != WarningMigrationTypeNarrowing {
		t.Fatalf("expected narrowing warning, got %s", step.Warnings[0].Code)
	}
}

func TestPlanSQLClassifiesSetNotNull(t *testing.T) {
	plan, err := PlanSQL(`ALTER TABLE users ALTER COLUMN email SET NOT NULL;`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}
	step := plan.Steps[0]
	if step.Type != AlterNullability {
		t.Fatalf("expected alter nullability step, got %#v", step)
	}
	if step.Nullable == nil || *step.Nullable {
		t.Fatalf("expected nullable=false, got %#v", step.Nullable)
	}
	if step.RiskLevel != query.RiskHigh || !plan.RequiredApproval {
		t.Fatalf("expected high-risk approval gate, risk=%s required=%v", step.RiskLevel, plan.RequiredApproval)
	}
	if len(step.Warnings) == 0 || step.Warnings[0].Code != WarningMigrationSetNotNull {
		t.Fatalf("expected set-not-null warning, got %#v", step.Warnings)
	}
	if len(step.Preflight) == 0 {
		t.Fatalf("expected set-not-null preflight suggestions")
	}
}

func TestBackfillReviewModeAddsEvidenceGuidance(t *testing.T) {
	plan, err := New(`ALTER TABLE users ALTER COLUMN email SET NOT NULL;`).
		ReviewMode(ReviewModeBackfill).
		Plan(context.Background())
	if err != nil {
		t.Fatalf("plan backfill review: %v", err)
	}
	if plan.Metadata["review_mode"] != string(ReviewModeBackfill) {
		t.Fatalf("expected backfill metadata, got %#v", plan.Metadata)
	}
	if !hasWarning(plan.Warnings, WarningMigrationBackfillReview) {
		t.Fatalf("expected backfill review warning, got %#v", plan.Warnings)
	}
	if !strings.Contains(strings.Join(plan.Steps[0].Preflight, "\n"), "zero-NULL") {
		t.Fatalf("expected zero-NULL preflight, got %#v", plan.Steps[0].Preflight)
	}
}

func TestReviewModeRejectsUnsupportedMode(t *testing.T) {
	_, err := New(`ALTER TABLE users ADD COLUMN nickname text;`).
		ReviewMode(ReviewMode("shadow")).
		Plan(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported migration review mode") {
		t.Fatalf("expected unsupported mode error, got %v", err)
	}
}

func TestPlanSQLStatementSplittingAndJSON(t *testing.T) {
	plan, err := PlanSQL(`
-- comments should not become statements
CREATE TABLE users (id bigint primary key);
ALTER TABLE users ADD COLUMN note text DEFAULT 'a;b';
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Statements) != 2 {
		t.Fatalf("expected two statements, got %#v", plan.Statements)
	}
	if plan.Statements[0].Line != 3 {
		t.Fatalf("expected first statement line 3, got %d", plan.Statements[0].Line)
	}
	b, err := plan.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"steps"`) {
		t.Fatalf("expected JSON to contain steps, got %s", string(b))
	}
	if !strings.Contains(plan.String(), "migration plan") {
		t.Fatalf("expected String summary, got %s", plan.String())
	}
}

func TestDiffSchemasBuildsMigrationPlan(t *testing.T) {
	current := Schema{Tables: []TableSchema{{
		Name: "users",
		Columns: []ColumnSchema{
			{Name: "id", Type: "bigint", Nullable: false},
			{Name: "legacy_id", Type: "text", Nullable: true},
			{Name: "name", Type: "varchar(255)", Nullable: true},
		},
		Indexes: []IndexSchema{{Name: "users_legacy_id_idx"}},
	}}}
	desired := Schema{Tables: []TableSchema{{
		Name: "users",
		Columns: []ColumnSchema{
			{Name: "id", Type: "bigint", Nullable: false},
			{Name: "name", Type: "varchar(64)", Nullable: true},
			{Name: "tenant_id", Type: "uuid", Nullable: false},
		},
		Indexes: []IndexSchema{{Name: "users_tenant_id_idx", Concurrent: true}},
	}}}

	plan := DiffSchemas(current, desired)
	if plan.Metadata["source"] != "schema_diff" {
		t.Fatalf("expected schema_diff metadata, got %#v", plan.Metadata)
	}
	if !hasStep(plan.Steps, AddColumn, "tenant_id") {
		t.Fatalf("expected add tenant_id step, got %#v", plan.Steps)
	}
	if !hasStep(plan.Steps, DropColumn, "legacy_id") {
		t.Fatalf("expected drop legacy_id step, got %#v", plan.Steps)
	}
	if !hasStep(plan.Steps, AlterColumnType, "name") {
		t.Fatalf("expected alter type step, got %#v", plan.Steps)
	}
	if !hasStep(plan.Steps, AddIndex, "users_tenant_id_idx") {
		t.Fatalf("expected add index step, got %#v", plan.Steps)
	}
	if !hasStep(plan.Steps, DropIndex, "users_legacy_id_idx") {
		t.Fatalf("expected drop index step, got %#v", plan.Steps)
	}
	if plan.RiskLevel != query.RiskDestructive {
		t.Fatalf("expected destructive diff due to drop/type narrowing, got %s", plan.RiskLevel)
	}
}

func TestDiffSchemasBuildsNullabilityStep(t *testing.T) {
	current := Schema{Tables: []TableSchema{{
		Name:    "users",
		Columns: []ColumnSchema{{Name: "email", Type: "text", Nullable: true}},
	}}}
	desired := Schema{Tables: []TableSchema{{
		Name:    "users",
		Columns: []ColumnSchema{{Name: "email", Type: "text", Nullable: false}},
	}}}

	plan := DiffSchemas(current, desired)
	if len(plan.Steps) != 1 {
		t.Fatalf("expected one step, got %#v", plan.Steps)
	}
	step := plan.Steps[0]
	if step.Type != AlterNullability {
		t.Fatalf("expected alter nullability, got %#v", step)
	}
	if step.Column != "email" || step.Nullable == nil || *step.Nullable {
		t.Fatalf("unexpected nullability step: %#v", step)
	}
	if hasStep(plan.Steps, AddColumn, "email") {
		t.Fatalf("nullable->not-null should not be represented as add column: %#v", plan.Steps)
	}
	if step.RiskLevel != query.RiskHigh {
		t.Fatalf("expected high-risk set-not-null diff, got %s", step.RiskLevel)
	}
}

func hasStep(steps []MigrationStep, typ MigrationStepType, name string) bool {
	for _, step := range steps {
		if step.Type != typ {
			continue
		}
		if step.Column == name || step.Index == name || step.Table == name {
			return true
		}
	}
	return false
}

func hasWarning(warnings []query.Warning, code string) bool {
	for _, warning := range warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
