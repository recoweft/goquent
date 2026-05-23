package query

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

const (
	WarningTenantFilterMissing     = "TENANT_FILTER_MISSING"
	WarningSoftDeleteFilterMissing = "SOFT_DELETE_FILTER_MISSING"
	WarningPIIColumnSelected       = "PII_COLUMN_SELECTED"
	WarningRequiredFilterMissing   = "REQUIRED_FILTER_MISSING"
)

// PolicyMode controls how policy violations are represented in a QueryPlan.
type PolicyMode string

const (
	PolicyModeWarn    PolicyMode = "warn"
	PolicyModeEnforce PolicyMode = "enforce"
	PolicyModeBlock   PolicyMode = "block"
)

// TablePolicy describes application-specific safety policy for a table.
type TablePolicy struct {
	Table                 string     `json:"table"`
	TenantColumn          string     `json:"tenant_column,omitempty"`
	TenantMode            PolicyMode `json:"tenant_mode,omitempty"`
	SoftDeleteColumn      string     `json:"soft_delete_column,omitempty"`
	SoftDeleteMode        PolicyMode `json:"soft_delete_mode,omitempty"`
	PIIColumns            []string   `json:"pii_columns,omitempty"`
	PIIMode               PolicyMode `json:"pii_mode,omitempty"`
	RequiredFilterColumns []string   `json:"required_filter_columns,omitempty"`
	RequiredFilterMode    PolicyMode `json:"required_filter_mode,omitempty"`
}

var policyRegistry = struct {
	sync.RWMutex
	byTable map[string]TablePolicy
}{byTable: make(map[string]TablePolicy)}

// RegisterTablePolicy registers or replaces a table policy.
func RegisterTablePolicy(policy TablePolicy) error {
	policy.Table = strings.TrimSpace(policy.Table)
	if policy.Table == "" {
		return fmt.Errorf("goquent: policy table is required")
	}
	policy.Table = normalizeTableName(policy.Table)
	policy = normalizeTablePolicy(policy)
	policyRegistry.Lock()
	defer policyRegistry.Unlock()
	policyRegistry.byTable[policy.Table] = cloneTablePolicy(policy)
	return nil
}

// PolicyForTable returns a registered policy for table.
func PolicyForTable(table string) (TablePolicy, bool) {
	policyRegistry.RLock()
	defer policyRegistry.RUnlock()
	policy, ok := policyRegistry.byTable[normalizeTableName(table)]
	if !ok {
		return TablePolicy{}, false
	}
	return cloneTablePolicy(policy), true
}

// RegisteredTablePolicies returns all registered table policies in stable order.
func RegisteredTablePolicies() []TablePolicy {
	policyRegistry.RLock()
	defer policyRegistry.RUnlock()
	policies := make([]TablePolicy, 0, len(policyRegistry.byTable))
	for _, policy := range policyRegistry.byTable {
		policies = append(policies, cloneTablePolicy(policy))
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Table < policies[j].Table
	})
	return policies
}

// ResetPolicyRegistry clears registered policies. Intended for tests.
func ResetPolicyRegistry() {
	policyRegistry.Lock()
	defer policyRegistry.Unlock()
	policyRegistry.byTable = make(map[string]TablePolicy)
}

func normalizeTablePolicy(policy TablePolicy) TablePolicy {
	policy.TenantColumn = strings.TrimSpace(policy.TenantColumn)
	policy.SoftDeleteColumn = strings.TrimSpace(policy.SoftDeleteColumn)
	policy.TenantMode = defaultPolicyMode(policy.TenantMode, PolicyModeEnforce)
	policy.SoftDeleteMode = defaultPolicyMode(policy.SoftDeleteMode, PolicyModeEnforce)
	policy.PIIMode = defaultPolicyMode(policy.PIIMode, PolicyModeWarn)
	policy.RequiredFilterMode = defaultPolicyMode(policy.RequiredFilterMode, PolicyModeEnforce)
	policy.PIIColumns = normalizeColumns(policy.PIIColumns)
	policy.RequiredFilterColumns = normalizeColumns(policy.RequiredFilterColumns)
	return policy
}

func defaultPolicyMode(mode, fallback PolicyMode) PolicyMode {
	switch mode {
	case PolicyModeWarn, PolicyModeEnforce, PolicyModeBlock:
		return mode
	default:
		return fallback
	}
}

func normalizeColumns(cols []string) []string {
	seen := make(map[string]struct{}, len(cols))
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		col = strings.TrimSpace(col)
		if col == "" {
			continue
		}
		key := normalizeColumnName(col)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, col)
	}
	return out
}

func cloneTablePolicy(policy TablePolicy) TablePolicy {
	policy.PIIColumns = append([]string(nil), policy.PIIColumns...)
	policy.RequiredFilterColumns = append([]string(nil), policy.RequiredFilterColumns...)
	return policy
}

type policyCheckContext struct {
	ambiguousPredicateColumns map[string]bool
}

func checkPolicies(plan *QueryPlan, extra *TablePolicy) []Warning {
	if plan == nil {
		return nil
	}

	byTable := make(map[string]TablePolicy)
	for _, policy := range RegisteredTablePolicies() {
		byTable[normalizeTableName(policy.Table)] = policy
	}
	if extra != nil && strings.TrimSpace(extra.Table) != "" {
		byTable[normalizeTableName(extra.Table)] = cloneTablePolicy(*extra)
	}

	keys := make([]string, 0, len(byTable))
	for key := range byTable {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	policies := make([]TablePolicy, 0, len(keys))
	for _, key := range keys {
		policy := byTable[key]
		if planTouchesTable(plan, policy.Table) {
			policies = append(policies, policy)
		}
	}

	ctx := newPolicyCheckContext(policies)
	var warnings []Warning
	for i := range policies {
		policy := policies[i]
		warnings = append(warnings, checkPolicyWithContext(plan, &policy, ctx)...)
	}
	return warnings
}

func newPolicyCheckContext(policies []TablePolicy) policyCheckContext {
	columnOwners := make(map[string]map[string]struct{})
	addOwner := func(table, column string) {
		column = normalizeColumnName(column)
		if column == "" {
			return
		}
		table = normalizeTableName(table)
		if columnOwners[column] == nil {
			columnOwners[column] = make(map[string]struct{})
		}
		columnOwners[column][table] = struct{}{}
	}

	for _, policy := range policies {
		addOwner(policy.Table, policy.TenantColumn)
		addOwner(policy.Table, policy.SoftDeleteColumn)
		for _, col := range policy.RequiredFilterColumns {
			addOwner(policy.Table, col)
		}
	}

	ambiguous := make(map[string]bool)
	for col, owners := range columnOwners {
		if len(owners) > 1 {
			ambiguous[col] = true
		}
	}
	return policyCheckContext{ambiguousPredicateColumns: ambiguous}
}

func checkPolicy(plan *QueryPlan, policy *TablePolicy) []Warning {
	return checkPolicyWithContext(plan, policy, policyCheckContext{})
}

func checkPolicyWithContext(plan *QueryPlan, policy *TablePolicy, ctx policyCheckContext) []Warning {
	if plan == nil || policy == nil || policy.Table == "" {
		return nil
	}
	if !planTouchesTable(plan, policy.Table) {
		return nil
	}

	var warnings []Warning
	if policy.TenantColumn != "" && policyAppliesToOperation(plan.Operation) && !hasPolicyPredicateColumn(plan, policy.Table, policy.TenantColumn, !ctx.ambiguousPredicateColumns[normalizeColumnName(policy.TenantColumn)]) {
		warnings = append(warnings, policyWarning(
			WarningTenantFilterMissing,
			policyModeLevel(policy.TenantMode, RiskHigh),
			fmt.Sprintf("%s is tenant-scoped but %s filter is missing", policy.Table, policy.TenantColumn),
			"add a tenant filter before executing this query",
			false,
		))
	}
	for _, col := range policy.RequiredFilterColumns {
		if policyAppliesToOperation(plan.Operation) && !hasPolicyPredicateColumn(plan, policy.Table, col, !ctx.ambiguousPredicateColumns[normalizeColumnName(col)]) {
			warnings = append(warnings, policyWarning(
				WarningRequiredFilterMissing,
				policyModeLevel(policy.RequiredFilterMode, RiskHigh),
				fmt.Sprintf("%s requires a filter on %s", policy.Table, col),
				"add the required filter before executing this query",
				false,
			))
		}
	}
	if policy.SoftDeleteColumn != "" && policyAppliesToOperation(plan.Operation) && shouldRequireSoftDeleteFilter(plan, policy.Table) && !hasPolicyPredicateColumn(plan, policy.Table, policy.SoftDeleteColumn, !ctx.ambiguousPredicateColumns[normalizeColumnName(policy.SoftDeleteColumn)]) {
		warnings = append(warnings, policyWarning(
			WarningSoftDeleteFilterMissing,
			policyModeLevel(policy.SoftDeleteMode, RiskMedium),
			fmt.Sprintf("%s has soft delete policy but %s filter is missing", policy.Table, policy.SoftDeleteColumn),
			"use the default soft delete filter or explicitly call WithDeleted",
			true,
		))
	}
	if plan.Operation == OperationSelect {
		for _, col := range selectedPIIColumnsForPolicy(plan, policy.Table, policy.PIIColumns) {
			w := policyWarning(
				WarningPIIColumnSelected,
				policyModeLevel(policy.PIIMode, RiskMedium),
				fmt.Sprintf("PII column selected: %s.%s", policy.Table, col),
				"avoid selecting PII or include a narrow access reason",
				true,
			)
			w.RequiresReason = true
			if reason, ok := plan.Metadata["access_reason"].(string); ok && reason != "" {
				w.Evidence = append(w.Evidence, Evidence{Key: "access_reason", Value: reason})
			}
			warnings = append(warnings, w)
		}
	}
	return warnings
}

func policyWarning(code string, level RiskLevel, message, hint string, suppressible bool) Warning {
	return Warning{
		Code:         code,
		Level:        level,
		Message:      message,
		Hint:         hint,
		Suppressible: suppressible && level != RiskBlocked,
	}
}

func policyModeLevel(mode PolicyMode, fallback RiskLevel) RiskLevel {
	switch mode {
	case PolicyModeWarn:
		return fallback
	case PolicyModeEnforce:
		if compareRisk(fallback, RiskHigh) < 0 {
			return RiskHigh
		}
		return fallback
	case PolicyModeBlock:
		return RiskBlocked
	default:
		return fallback
	}
}

func policyAppliesToOperation(op OperationType) bool {
	return op == OperationSelect || op == OperationUpdate || op == OperationDelete
}

func planTouchesTable(plan *QueryPlan, table string) bool {
	target := normalizeTableName(table)
	for _, ref := range plan.Tables {
		if normalizeTableName(ref.Name) == target {
			return true
		}
	}
	return false
}

func hasPredicateColumn(plan *QueryPlan, column string) bool {
	return hasPolicyPredicateColumn(plan, "", column, true)
}

func hasPolicyPredicateColumn(plan *QueryPlan, table, column string, allowUnqualified bool) bool {
	target := normalizeColumnName(column)
	qualifiers := tableQualifierSet(plan, table)
	for _, predicate := range plan.Predicates {
		if predicateColumnMatches(predicate.Column, target, qualifiers, allowUnqualified) {
			return true
		}
		if predicateColumnMatches(predicate.ValueColumn, target, qualifiers, allowUnqualified) {
			return true
		}
	}
	return false
}

func selectedPIIColumns(plan *QueryPlan, piiColumns []string) []string {
	return selectedPIIColumnsForPolicy(plan, "", piiColumns)
}

func selectedPIIColumnsForPolicy(plan *QueryPlan, table string, piiColumns []string) []string {
	if len(piiColumns) == 0 {
		return nil
	}
	qualifiers := tableQualifierSet(plan, table)
	selectedAll := false
	selected := make([]string, 0, len(plan.Columns))
	for _, column := range plan.Columns {
		if strings.TrimSpace(column.Name) == "*" || strings.TrimSpace(column.Expression) == "*" {
			selectedAll = true
			continue
		}
		if column.Name != "" {
			selected = append(selected, column.Name)
		}
	}
	var out []string
	for _, pii := range piiColumns {
		if selectedAll {
			out = append(out, pii)
			continue
		}
		target := normalizeColumnName(pii)
		for _, column := range selected {
			if predicateColumnMatches(column, target, qualifiers, true) {
				out = append(out, pii)
				break
			}
		}
	}
	return out
}

func shouldRequireSoftDeleteFilter(plan *QueryPlan, table string) bool {
	if plan.Metadata != nil {
		if v, ok := plan.Metadata["soft_delete"].(string); ok && v == "with_deleted" {
			if policyTable, ok := plan.Metadata["policy_table"].(string); !ok || normalizeTableName(policyTable) == normalizeTableName(table) {
				return false
			}
		}
	}
	return true
}

func predicateColumnMatches(reference, target string, qualifiers map[string]struct{}, allowUnqualified bool) bool {
	qualifier, name := splitColumnReference(reference)
	if name == "" || name != target {
		return false
	}
	if len(qualifiers) == 0 {
		return true
	}
	if qualifier == "" {
		return allowUnqualified
	}
	_, ok := qualifiers[qualifier]
	return ok
}

func splitColumnReference(reference string) (string, string) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return "", ""
	}
	idx := strings.LastIndex(reference, ".")
	if idx < 0 {
		return "", normalizeColumnName(reference)
	}
	return normalizeIdentifierToken(reference[:idx]), normalizeColumnName(reference)
}

func tableQualifierSet(plan *QueryPlan, table string) map[string]struct{} {
	if plan == nil || strings.TrimSpace(table) == "" {
		return nil
	}
	target := normalizeTableName(table)
	out := make(map[string]struct{})
	for _, ref := range plan.Tables {
		if normalizeTableName(ref.Name) != target {
			continue
		}
		addTableQualifier(out, ref.Name)
		addTableQualifier(out, ref.Alias)
		name, alias := splitTableAlias(ref.Name)
		addTableQualifier(out, name)
		addTableQualifier(out, alias)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func addTableQualifier(out map[string]struct{}, value string) {
	value = normalizeIdentifierToken(value)
	if value == "" {
		return
	}
	out[value] = struct{}{}
	if idx := strings.LastIndex(value, "."); idx >= 0 && idx+1 < len(value) {
		out[value[idx+1:]] = struct{}{}
	}
}

func normalizeColumnName(column string) string {
	column = strings.TrimSpace(column)
	column = strings.Trim(column, "`\"")
	if idx := strings.LastIndex(column, "."); idx >= 0 {
		column = column[idx+1:]
	}
	column = strings.Trim(column, "`\"")
	return strings.ToLower(column)
}

func normalizeTableName(table string) string {
	table = strings.TrimSpace(table)
	if table == "" {
		return ""
	}
	table, _ = splitTableAlias(table)
	return normalizeIdentifierToken(table)
}

func splitTableAlias(table string) (string, string) {
	table = strings.TrimSpace(table)
	fields := strings.Fields(table)
	if len(fields) >= 3 && strings.EqualFold(fields[1], "as") {
		return fields[0], fields[2]
	} else if len(fields) == 2 && isSimpleIdentifierToken(fields[0]) && isSimpleIdentifierToken(fields[1]) {
		return fields[0], fields[1]
	}
	return table, ""
}

func normalizeIdentifierToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"")
	value = strings.ReplaceAll(value, "`", "")
	value = strings.ReplaceAll(value, `"`, "")
	return strings.ToLower(value)
}

func isSimpleIdentifierToken(value string) bool {
	value = strings.Trim(value, "`\"")
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '$' || ch == '.' {
			continue
		}
		return false
	}
	return true
}
