package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/query"
)

// MigrationStepType classifies a structural schema migration step.
type MigrationStepType string

// ReviewMode tunes migration review for a specific rollout pattern.
type ReviewMode string

const (
	AddTable         MigrationStepType = "add_table"
	DropTable        MigrationStepType = "drop_table"
	AddColumn        MigrationStepType = "add_column"
	DropColumn       MigrationStepType = "drop_column"
	RenameColumn     MigrationStepType = "rename_column"
	AlterColumnType  MigrationStepType = "alter_column_type"
	AlterNullability MigrationStepType = "alter_nullability"
	AddIndex         MigrationStepType = "add_index"
	DropIndex        MigrationStepType = "drop_index"
	UnsupportedStep  MigrationStepType = "unsupported"
)

const (
	// ReviewModeBackfill adds expand/backfill/contract preflight checks.
	ReviewModeBackfill ReviewMode = "backfill"
)

const (
	WarningMigrationUnsupported           = "MIGRATION_UNSUPPORTED"
	WarningMigrationDropTable             = "MIGRATION_DROP_TABLE"
	WarningMigrationDropColumn            = "MIGRATION_DROP_COLUMN"
	WarningMigrationAddNotNullColumn      = "MIGRATION_ADD_NOT_NULL_COLUMN"
	WarningMigrationRenameColumn          = "MIGRATION_RENAME_COLUMN"
	WarningMigrationAlterColumnType       = "MIGRATION_ALTER_COLUMN_TYPE"
	WarningMigrationTypeNarrowing         = "MIGRATION_TYPE_NARROWING"
	WarningMigrationSetNotNull            = "MIGRATION_SET_NOT_NULL"
	WarningMigrationAddIndexNonConcurrent = "MIGRATION_ADD_INDEX_NON_CONCURRENT"
	WarningMigrationDropIndex             = "MIGRATION_DROP_INDEX"
	WarningMigrationBackfillReview        = "MIGRATION_BACKFILL_REVIEW"
)

// MigrationStatement is one executable SQL statement in a migration plan.
type MigrationStatement struct {
	SQL  string `json:"sql"`
	Line int    `json:"line,omitempty"`
}

// MigrationStep describes one schema change extracted from migration SQL.
type MigrationStep struct {
	Type              MigrationStepType       `json:"type"`
	Table             string                  `json:"table,omitempty"`
	Column            string                  `json:"column,omitempty"`
	FromName          string                  `json:"from_name,omitempty"`
	ToName            string                  `json:"to_name,omitempty"`
	Index             string                  `json:"index,omitempty"`
	ColumnType        string                  `json:"column_type,omitempty"`
	OldType           string                  `json:"old_type,omitempty"`
	NewType           string                  `json:"new_type,omitempty"`
	Nullable          *bool                   `json:"nullable,omitempty"`
	HasDefault        bool                    `json:"has_default,omitempty"`
	DefaultExpression string                  `json:"default_expression,omitempty"`
	Concurrent        bool                    `json:"concurrent,omitempty"`
	SQL               string                  `json:"sql,omitempty"`
	Line              int                     `json:"line,omitempty"`
	RiskLevel         query.RiskLevel         `json:"risk_level"`
	Warnings          []query.Warning         `json:"warnings,omitempty"`
	Preflight         []string                `json:"preflight,omitempty"`
	AnalysisPrecision query.AnalysisPrecision `json:"analysis_precision"`
}

// MigrationPlan explains a schema migration before it is applied.
type MigrationPlan struct {
	SQL               string                  `json:"sql"`
	Statements        []MigrationStatement    `json:"statements,omitempty"`
	Steps             []MigrationStep         `json:"steps,omitempty"`
	RiskLevel         query.RiskLevel         `json:"risk_level"`
	Warnings          []query.Warning         `json:"warnings,omitempty"`
	RequiredApproval  bool                    `json:"required_approval"`
	Blocked           bool                    `json:"blocked,omitempty"`
	Approval          *query.Approval         `json:"approval,omitempty"`
	AnalysisPrecision query.AnalysisPrecision `json:"analysis_precision"`
	Metadata          map[string]any          `json:"metadata,omitempty"`
}

// ToJSON returns stable, indented JSON for the migration plan.
func (p *MigrationPlan) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// RequiresApproval reports whether this migration needs an explicit approval reason.
func (p *MigrationPlan) RequiresApproval() bool {
	return p != nil && p.RequiredApproval
}

// String returns a compact human-readable migration summary.
func (p *MigrationPlan) String() string {
	if p == nil {
		return "<nil migration plan>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "migration plan\n")
	fmt.Fprintf(&b, "risk: %s\n", p.RiskLevel)
	fmt.Fprintf(&b, "precision: %s\n", p.AnalysisPrecision)
	if p.RequiredApproval {
		b.WriteString("requires_approval: true\n")
	}
	for _, step := range p.Steps {
		fmt.Fprintf(&b, "step[%s]: %s", step.RiskLevel, step.Type)
		if step.Table != "" {
			fmt.Fprintf(&b, " table=%s", step.Table)
		}
		if step.Column != "" {
			fmt.Fprintf(&b, " column=%s", step.Column)
		}
		if step.Index != "" {
			fmt.Fprintf(&b, " index=%s", step.Index)
		}
		b.WriteByte('\n')
		for _, warning := range step.Warnings {
			fmt.Fprintf(&b, "  warning[%s]: %s - %s\n", warning.Level, warning.Code, warning.Message)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// Migrator builds and optionally applies a migration plan.
type Migrator struct {
	sql        string
	approval   *query.Approval
	reviewMode ReviewMode
}

// New creates a migration planner for SQL text.
func New(sqlText string) *Migrator {
	return &Migrator{sql: sqlText}
}

// RequireApproval records an explicit reason for applying a risky migration.
func (m *Migrator) RequireApproval(reason string) *Migrator {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return m
	}
	m.approval = &query.Approval{Reason: reason, CreatedAt: time.Now().UTC()}
	return m
}

// ReviewMode enables an additional migration review mode.
func (m *Migrator) ReviewMode(mode ReviewMode) *Migrator {
	m.reviewMode = mode
	return m
}

// Plan builds a migration plan without executing it.
func (m *Migrator) Plan(ctx context.Context) (*MigrationPlan, error) {
	_ = ctx
	plan, err := PlanSQL(m.sql)
	if err != nil {
		return nil, err
	}
	if err := ApplyReviewMode(plan, m.reviewMode); err != nil {
		return nil, err
	}
	if m.approval != nil {
		copied := *m.approval
		plan.Approval = &copied
	}
	return plan, nil
}

// DryRun builds and validates the migration plan without executing it.
func (m *Migrator) DryRun(ctx context.Context) (*MigrationPlan, error) {
	plan, err := m.Plan(ctx)
	if err != nil {
		return nil, err
	}
	return plan, EnsureExecutable(plan)
}

// Executor is the minimal execution surface used by Apply.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Apply validates and executes migration statements sequentially.
func (m *Migrator) Apply(ctx context.Context, exec Executor) (*MigrationPlan, error) {
	if exec == nil {
		return nil, fmt.Errorf("goquent: migration executor is required")
	}
	plan, err := m.Plan(ctx)
	if err != nil {
		return nil, err
	}
	if err := EnsureExecutable(plan); err != nil {
		return plan, err
	}
	for _, statement := range plan.Statements {
		if strings.TrimSpace(statement.SQL) == "" {
			continue
		}
		if _, err := exec.ExecContext(ctx, statement.SQL); err != nil {
			return plan, err
		}
	}
	return plan, nil
}

// PlanSQL builds a migration plan from raw migration SQL.
func PlanSQL(sqlText string) (*MigrationPlan, error) {
	statements := splitSQLStatements(sqlText)
	plan := &MigrationPlan{
		SQL:               sqlText,
		RiskLevel:         query.RiskLow,
		AnalysisPrecision: query.AnalysisPrecise,
		Metadata:          map[string]any{"source": "sql"},
	}
	for _, statement := range statements {
		plan.Statements = append(plan.Statements, MigrationStatement{SQL: statement.SQL, Line: statement.Line})
		step := parseMigrationStatement(statement)
		if step.Type == "" {
			continue
		}
		classifyStep(&step)
		plan.Steps = append(plan.Steps, step)
	}
	finalizePlan(plan)
	return plan, nil
}

// EnsureExecutable enforces migration approval requirements before execution.
func EnsureExecutable(plan *MigrationPlan) error {
	if plan == nil {
		return nil
	}
	if plan.Blocked {
		return fmt.Errorf("%w: %s", query.ErrBlockedOperation, warningCodes(plan.Warnings))
	}
	if !plan.RequiredApproval {
		return nil
	}
	if plan.Approval == nil || strings.TrimSpace(plan.Approval.Reason) == "" {
		return fmt.Errorf("%w: %s", query.ErrApprovalRequired, warningCodes(plan.Warnings))
	}
	if plan.Approval.ExpiresAt != nil && !plan.Approval.ExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("%w: approval expired", query.ErrApprovalRequired)
	}
	return nil
}

func finalizePlan(plan *MigrationPlan) {
	if plan == nil {
		return
	}
	precision := query.AnalysisPrecise
	for _, step := range plan.Steps {
		plan.Warnings = append(plan.Warnings, step.Warnings...)
		if step.AnalysisPrecision == query.AnalysisUnsupported {
			precision = query.AnalysisUnsupported
		} else if step.AnalysisPrecision == query.AnalysisPartial && precision == query.AnalysisPrecise {
			precision = query.AnalysisPartial
		}
	}
	plan.AnalysisPrecision = precision
	plan.RiskLevel, plan.Blocked = aggregateWarnings(plan.Warnings)
	plan.RequiredApproval = requiresApproval(plan.RiskLevel)
}

func newWarning(code string, level query.RiskLevel, message, hint string, suppressible bool, line int) query.Warning {
	return query.Warning{
		Code:         code,
		Level:        level,
		Message:      message,
		Location:     &query.SourceLocation{Line: line},
		Hint:         hint,
		Suppressible: suppressible,
	}
}

func aggregateWarnings(warnings []query.Warning) (query.RiskLevel, bool) {
	level := query.RiskLow
	blocked := false
	for _, warning := range warnings {
		if compareRisk(warning.Level, level) > 0 {
			level = warning.Level
		}
		if warning.Level == query.RiskBlocked {
			blocked = true
		}
	}
	return level, blocked
}

func requiresApproval(level query.RiskLevel) bool {
	return compareRisk(level, query.RiskHigh) >= 0 && level != query.RiskBlocked
}

func compareRisk(a, b query.RiskLevel) int {
	return riskRank(a) - riskRank(b)
}

func riskRank(level query.RiskLevel) int {
	switch level {
	case query.RiskLow, "":
		return 0
	case query.RiskMedium:
		return 1
	case query.RiskHigh:
		return 2
	case query.RiskDestructive:
		return 3
	case query.RiskBlocked:
		return 4
	default:
		return 0
	}
}

func warningCodes(warnings []query.Warning) string {
	if len(warnings) == 0 {
		return "no warnings"
	}
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return strings.Join(codes, ", ")
}
