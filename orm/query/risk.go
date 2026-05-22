package query

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrApprovalRequired       = errors.New("goquent: approval required")
	ErrApprovalReasonRequired = errors.New("goquent: approval reason required")
	ErrAccessReasonRequired   = errors.New("goquent: access reason required")
	ErrBlockedOperation       = errors.New("goquent: blocked operation")
)

// RiskEngine deterministically evaluates the structural DB risk of a query plan.
type RiskEngine interface {
	CheckQuery(plan *QueryPlan) RiskResult
}

// RiskResult is the result of applying risk rules to a query plan.
type RiskResult struct {
	Level            RiskLevel `json:"level"`
	Warnings         []Warning `json:"warnings,omitempty"`
	RequiredApproval bool      `json:"required_approval"`
	Blocked          bool      `json:"blocked"`
}

// RiskRuleConfig customizes a built-in warning rule.
type RiskRuleConfig struct {
	Enabled        *bool      `json:"enabled,omitempty"`
	Severity       *RiskLevel `json:"severity,omitempty"`
	Suppressible   *bool      `json:"suppressible,omitempty"`
	RequiresReason *bool      `json:"requires_reason,omitempty"`
}

// RiskConfig customizes risk rules for an environment or caller.
type RiskConfig struct {
	Environment string                    `json:"environment,omitempty"`
	Rules       map[string]RiskRuleConfig `json:"rules,omitempty"`
}

// DefaultRiskEngine is the built-in deterministic risk engine.
var DefaultRiskEngine RiskEngine = defaultRiskEngine{}

// NewRiskEngine creates a deterministic risk engine using config overrides.
func NewRiskEngine(config RiskConfig) RiskEngine {
	return defaultRiskEngine{config: config}
}

type defaultRiskEngine struct {
	config RiskConfig
}

func (d defaultRiskEngine) CheckQuery(plan *QueryPlan) RiskResult {
	if plan == nil {
		return RiskResult{Level: RiskLow}
	}

	var warnings []Warning
	add := func(w Warning) {
		warnings = append(warnings, w)
	}

	switch plan.Operation {
	case OperationSelect:
		if selectStarUsed(plan) {
			add(newWarning(WarningSelectStarUsed, RiskMedium,
				"SELECT * makes selected data harder to review",
				"select explicit columns",
				true,
				false,
			))
		}
		if plan.Limit == nil && !selectIsAggregateOnly(plan) {
			add(newWarning(WarningLimitMissing, RiskMedium,
				"SELECT query has no LIMIT",
				"add Limit(n) for list queries",
				true,
				false,
			))
		}
	case OperationUpdate:
		if hasNoPredicate(plan) {
			add(newWarning(WarningUpdateWithoutWhere, RiskBlocked,
				"UPDATE query has no WHERE predicate",
				"add a specific predicate before executing the update",
				false,
				false,
			))
		} else if !hasPrimaryKeyLikePredicate(plan) {
			add(newWarning(WarningBulkUpdateDetected, RiskMedium,
				"UPDATE predicate is not primary-key-like and may affect multiple rows",
				"confirm the intended row set or add a narrower predicate",
				true,
				false,
			))
		}
	case OperationDelete:
		if hasNoPredicate(plan) {
			add(newWarning(WarningDeleteWithoutWhere, RiskBlocked,
				"DELETE query has no WHERE predicate",
				"add a specific predicate before executing the delete",
				false,
				false,
			))
		} else if !hasPrimaryKeyLikePredicate(plan) {
			add(newWarning(WarningBulkDeleteDetected, RiskMedium,
				"DELETE predicate is not primary-key-like and may affect multiple rows",
				"confirm the intended row set or add a narrower predicate",
				true,
				false,
			))
		}
	case OperationRaw:
		add(newWarning(WarningRawSQLUsed, RiskHigh,
			"raw SQL was used; Goquent cannot fully inspect this query",
			"prefer Goquent query builders when possible, or review the raw SQL explicitly",
			true,
			true,
		))
	}

	if containsDangerousSQLToken(plan.SQL) {
		add(newWarning(WarningDestructiveSQL, RiskDestructive,
			"SQL contains a destructive DDL token",
			"review destructive SQL manually and require explicit approval before execution",
			false,
			false,
		))
	}
	if hasWeakPredicate(plan) {
		add(newWarning(WarningWeakPredicate, RiskHigh,
			"query contains a weak predicate such as 1=1",
			"replace weak predicates with a meaningful filter",
			true,
			false,
		))
	}

	warnings = applyRiskConfig(warnings, d.config)
	level, blocked := aggregateWarnings(warnings)
	return RiskResult{
		Level:            level,
		Warnings:         warnings,
		RequiredApproval: requiresApprovalLevel(level),
		Blocked:          blocked,
	}
}

func applyRiskConfig(warnings []Warning, config RiskConfig) []Warning {
	if len(warnings) == 0 || len(config.Rules) == 0 {
		return warnings
	}
	out := make([]Warning, 0, len(warnings))
	for _, warning := range warnings {
		rule, ok := config.Rules[warning.Code]
		if !ok {
			out = append(out, warning)
			continue
		}
		if rule.Enabled != nil && !*rule.Enabled {
			continue
		}
		if rule.Severity != nil {
			warning.Level = *rule.Severity
		}
		if rule.Suppressible != nil {
			warning.Suppressible = *rule.Suppressible
		}
		if rule.RequiresReason != nil {
			warning.RequiresReason = *rule.RequiresReason
		}
		out = append(out, warning)
	}
	return out
}

func finalizePlan(plan *QueryPlan, approval *Approval, suppressions []Suppression) {
	if plan == nil {
		return
	}
	result := DefaultRiskEngine.CheckQuery(plan)
	allWarnings := append([]Warning(nil), result.Warnings...)
	allWarnings = append(allWarnings, checkPolicies(plan, nil)...)
	warnings, suppressed, suppressionWarnings := applySuppressions(allWarnings, suppressions, time.Now().UTC())
	warnings = append(warnings, suppressionWarnings...)

	level, blocked := aggregateWarnings(warnings)
	if len(warnings) == 0 && len(suppressed) > 0 {
		level = RiskLow
	}

	plan.Warnings = warnings
	plan.SuppressedWarnings = suppressed
	plan.RiskLevel = level
	plan.Blocked = blocked || level == RiskBlocked
	plan.RequiredApproval = requiresApprovalLevel(level)
	if approval != nil {
		copied := *approval
		plan.Approval = &copied
	}
}

func finalizePlanWithPolicy(plan *QueryPlan, approval *Approval, suppressions []Suppression, policy *TablePolicy) {
	if plan == nil {
		return
	}
	result := DefaultRiskEngine.CheckQuery(plan)
	allWarnings := append([]Warning(nil), result.Warnings...)
	allWarnings = append(allWarnings, checkPolicies(plan, policy)...)
	warnings, suppressed, suppressionWarnings := applySuppressions(allWarnings, suppressions, time.Now().UTC())
	warnings = append(warnings, suppressionWarnings...)

	level, blocked := aggregateWarnings(warnings)
	if len(warnings) == 0 && len(suppressed) > 0 {
		level = RiskLow
	}

	plan.Warnings = warnings
	plan.SuppressedWarnings = suppressed
	plan.RiskLevel = level
	plan.Blocked = blocked || level == RiskBlocked
	plan.RequiredApproval = requiresApprovalLevel(level)
	if approval != nil {
		copied := *approval
		plan.Approval = &copied
	}
}

// EnsurePlanExecutable enforces approval and block rules for a finalized plan.
func EnsurePlanExecutable(plan *QueryPlan) error {
	return ensurePlanExecutable(plan)
}

func ensurePlanExecutable(plan *QueryPlan) error {
	if plan == nil {
		return nil
	}
	if plan.Blocked {
		return fmt.Errorf("%w: %s", ErrBlockedOperation, warningCodes(plan.Warnings))
	}
	if !plan.RequiredApproval {
		return nil
	}
	if plan.Approval == nil || strings.TrimSpace(plan.Approval.Reason) == "" {
		return fmt.Errorf("%w: %s", ErrApprovalRequired, warningCodes(plan.Warnings))
	}
	if plan.Approval.ExpiresAt != nil && !plan.Approval.ExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("%w: approval expired", ErrApprovalRequired)
	}
	return nil
}

func newWarning(code string, level RiskLevel, message, hint string, suppressible, requiresReason bool) Warning {
	return Warning{
		Code:           code,
		Level:          level,
		Message:        message,
		Hint:           hint,
		Evidence:       nil,
		Suppressible:   suppressible,
		RequiresReason: requiresReason,
	}
}

func aggregateWarnings(warnings []Warning) (RiskLevel, bool) {
	level := RiskLow
	blocked := false
	for _, warning := range warnings {
		if compareRisk(warning.Level, level) > 0 {
			level = warning.Level
		}
		if warning.Level == RiskBlocked {
			blocked = true
		}
	}
	return level, blocked
}

func compareRisk(a, b RiskLevel) int {
	return riskRank(a) - riskRank(b)
}

func riskRank(level RiskLevel) int {
	switch level {
	case RiskLow, "":
		return 0
	case RiskMedium:
		return 1
	case RiskHigh:
		return 2
	case RiskDestructive:
		return 3
	case RiskBlocked:
		return 4
	default:
		return 0
	}
}

func requiresApprovalLevel(level RiskLevel) bool {
	return compareRisk(level, RiskHigh) >= 0 && level != RiskBlocked
}

func warningCodes(warnings []Warning) string {
	if len(warnings) == 0 {
		return "no warnings"
	}
	codes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		codes = append(codes, warning.Code)
	}
	return strings.Join(codes, ", ")
}

func selectStarUsed(plan *QueryPlan) bool {
	if len(plan.Columns) == 0 {
		return true
	}
	for _, column := range plan.Columns {
		if column.Count {
			continue
		}
		if strings.TrimSpace(column.Name) == "*" || strings.TrimSpace(column.Expression) == "*" {
			return true
		}
	}
	return false
}

func selectIsAggregateOnly(plan *QueryPlan) bool {
	if len(plan.Columns) == 0 {
		return false
	}
	for _, column := range plan.Columns {
		if !column.Count && column.Function == "" {
			return false
		}
	}
	return true
}

func hasNoPredicate(plan *QueryPlan) bool {
	return len(plan.Predicates) == 0 && !strings.Contains(strings.ToUpper(plan.SQL), " WHERE ")
}

func hasPrimaryKeyLikePredicate(plan *QueryPlan) bool {
	for _, predicate := range plan.Predicates {
		col := strings.ToLower(strings.TrimSpace(predicate.Column))
		col = strings.Trim(col, "`\"")
		if col == "id" || strings.HasSuffix(col, ".id") || strings.HasSuffix(col, "_id") {
			return true
		}
	}
	return false
}

func hasWeakPredicate(plan *QueryPlan) bool {
	if normalizedContainsWeakPredicate(plan.SQL) {
		return true
	}
	for _, predicate := range plan.Predicates {
		if normalizedContainsWeakPredicate(predicate.Raw) {
			return true
		}
		if predicate.Column == "1" && predicate.Operator == "=" {
			return true
		}
	}
	return false
}

func normalizedContainsWeakPredicate(s string) bool {
	normalized := strings.ToLower(s)
	normalized = strings.ReplaceAll(normalized, "`", "")
	normalized = strings.ReplaceAll(normalized, `"`, "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "\t", "")
	normalized = strings.ReplaceAll(normalized, "\n", "")
	return strings.Contains(normalized, "where1=1") || normalized == "1=1" || strings.Contains(normalized, "(1=1)")
}

func containsDangerousSQLToken(sql string) bool {
	upper := strings.ToUpper(sql)
	for _, token := range []string{"DROP", "TRUNCATE", "ALTER"} {
		if containsSQLWord(upper, token) {
			return true
		}
	}
	return false
}

func containsSQLWord(upperSQL, token string) bool {
	for i := 0; i+len(token) <= len(upperSQL); i++ {
		if upperSQL[i:i+len(token)] != token {
			continue
		}
		beforeOK := i == 0 || !isSQLWordByte(upperSQL[i-1])
		after := i + len(token)
		afterOK := after >= len(upperSQL) || !isSQLWordByte(upperSQL[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isSQLWordByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
