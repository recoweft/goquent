package review

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/query"
)

type chainCall struct {
	Method string
	Call   *ast.CallExpr
}

func reviewGoFile(ctx reviewContext, path string) ([]Finding, error) {
	abs, _ := filepath.Abs(path)
	tf := ctx.types[abs]
	if tf == nil {
		tf = loadReviewTypes(nil, []string{path})[abs]
	}
	return reviewTypedGo(ctx, path, tf)
}

func staticPlanFromChain(ctx reviewContext, method string, calls []chainCall) (*query.QueryPlan, bool) {
	op, ok := operationForTerminal(method)
	if !ok {
		return nil, false
	}
	table, ok := tableFromChain(calls)
	if !ok {
		return nil, false
	}
	plan := &query.QueryPlan{
		Operation:         op,
		SQL:               staticSQLShape(op, table, calls),
		Tables:            []query.TableRef{{Name: table}},
		Predicates:        predicatesFromChain(calls),
		RiskLevel:         query.RiskLow,
		AnalysisPrecision: query.AnalysisPrecise,
	}
	if op == query.OperationSelect {
		plan.Columns = selectedColumnsFromChain(calls)
		if len(plan.Columns) == 0 {
			plan.Columns = []query.ColumnRef{{Name: "*"}}
		}
	}
	if limit, ok := limitFromChain(calls); ok {
		plan.Limit = &limit
	}
	for _, joinTable := range joinedTablesFromChain(calls) {
		plan.Tables = append(plan.Tables, query.TableRef{Name: joinTable})
	}
	if hasChainMethod(calls, "WithDeleted") || hasChainMethod(calls, "OnlyDeleted") {
		plan.Metadata = map[string]any{"soft_delete": "with_deleted"}
	}
	if len(ctx.riskMetadata) > 0 {
		query.AttachTableRiskMetadata(plan, ctx.riskMetadata)
	}
	return plan, true
}

func operationForTerminal(method string) (query.OperationType, bool) {
	switch method {
	case "Update", "PlanUpdate":
		return query.OperationUpdate, true
	case "Delete", "PlanDelete":
		return query.OperationDelete, true
	case "Get", "GetMaps", "First", "FirstMap", "Plan":
		return query.OperationSelect, true
	default:
		return "", false
	}
}

func tableFromChain(calls []chainCall) (string, bool) {
	for _, call := range calls {
		switch call.Method {
		case "Table":
			if call.Call == nil || len(call.Call.Args) == 0 {
				return "", false
			}
			table, ok := stringLiteralValue(call.Call.Args[0])
			return table, ok && strings.TrimSpace(table) != ""
		case "TablePath":
			if call.Call == nil || len(call.Call.Args) == 0 {
				return "", false
			}
			parts := make([]string, 0, len(call.Call.Args))
			for _, arg := range call.Call.Args {
				part, ok := stringLiteralValue(arg)
				if !ok || strings.TrimSpace(part) == "" {
					return "", false
				}
				parts = append(parts, part)
			}
			return strings.Join(parts, "."), true
		}
	}
	return "", false
}

func staticSQLShape(op query.OperationType, table string, calls []chainCall) string {
	hasWhere := len(predicatesFromChain(calls)) > 0
	switch op {
	case query.OperationUpdate:
		if hasWhere {
			return "UPDATE " + table + " SET ..."
		}
		return "UPDATE " + table + " SET ..."
	case query.OperationDelete:
		if hasWhere {
			return "DELETE FROM " + table + " WHERE ..."
		}
		return "DELETE FROM " + table
	default:
		return "SELECT ... FROM " + table
	}
}

func selectedColumnsFromChain(calls []chainCall) []query.ColumnRef {
	var columns []query.ColumnRef
	for _, call := range calls {
		if call.Call == nil {
			continue
		}
		switch call.Method {
		case "Select":
			for _, arg := range call.Call.Args {
				col, ok := stringLiteralValue(arg)
				if !ok {
					continue
				}
				columns = append(columns, query.ColumnRef{Name: col})
			}
		case "SelectRaw":
			if len(call.Call.Args) > 0 {
				raw, ok := stringLiteralValue(call.Call.Args[0])
				if ok {
					columns = append(columns, query.ColumnRef{Expression: raw, Raw: true})
				}
			}
		case "Distinct", "Count", "Max", "Min", "Sum", "Avg":
			name := ""
			if len(call.Call.Args) > 0 {
				name, _ = stringLiteralValue(call.Call.Args[0])
			}
			columns = append(columns, query.ColumnRef{Name: name, Function: call.Method, Count: call.Method == "Count"})
		}
	}
	return columns
}

func predicatesFromChain(calls []chainCall) []query.PredicateRef {
	var predicates []query.PredicateRef
	for _, call := range calls {
		if call.Call == nil || !isPredicateMethod(call.Method) {
			continue
		}
		p := query.PredicateRef{Connector: "AND"}
		if strings.HasPrefix(call.Method, "Or") {
			p.Connector = "OR"
		}
		if strings.Contains(call.Method, "Raw") {
			predicates = append(predicates, p)
			continue
		}
		if len(call.Call.Args) > 0 {
			p.Column, _ = stringLiteralValue(call.Call.Args[0])
		}
		switch call.Method {
		case "WhereNull":
			p.Operator = "IS NULL"
		case "WhereNotNull":
			p.Operator = "IS NOT NULL"
		default:
			if len(call.Call.Args) > 2 {
				p.Operator, _ = stringLiteralValue(call.Call.Args[1])
				p.ValueCount = 1
			} else if len(call.Call.Args) > 1 {
				p.Operator = "="
				p.ValueCount = 1
			}
		}
		predicates = append(predicates, p)
	}
	return predicates
}

func limitFromChain(calls []chainCall) (int64, bool) {
	for _, call := range calls {
		switch call.Method {
		case "Limit", "Take":
			if call.Call == nil || len(call.Call.Args) != 1 {
				return 0, false
			}
			lit, ok := call.Call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.INT {
				return 0, false
			}
			n, err := strconv.ParseInt(lit.Value, 0, 64)
			return n, err == nil && n > 0
		}
	}
	return 0, false
}

func joinedTablesFromChain(calls []chainCall) []string {
	var tables []string
	for _, call := range calls {
		if call.Call == nil || len(call.Call.Args) == 0 {
			continue
		}
		switch call.Method {
		case "Join", "LeftJoin", "RightJoin", "CrossJoin":
			table, ok := stringLiteralValue(call.Call.Args[0])
			if ok && strings.TrimSpace(table) != "" {
				tables = append(tables, table)
			}
		}
	}
	return tables
}

func hasChainMethod(calls []chainCall, method string) bool {
	for _, call := range calls {
		if call.Method == method {
			return true
		}
	}
	return false
}

func manifestPolicyFindings(ctx reviewContext, plan *query.QueryPlan, loc *query.SourceLocation) []Finding {
	if ctx.manifest == nil || plan == nil {
		return nil
	}
	policies := touchedManifestTables(ctx, plan)
	ambiguous := ambiguousPolicyColumns(policies)
	var findings []Finding
	for _, table := range policies {
		findings = append(findings, manifestPolicyFindingsForTable(ctx, table, plan, loc, ambiguous)...)
	}
	return findings
}

func touchedManifestTables(ctx reviewContext, plan *query.QueryPlan) []manifest.Table {
	var tables []manifest.Table
	seen := map[string]struct{}{}
	for _, ref := range plan.Tables {
		table, ok := ctx.table(ref.Name)
		if !ok {
			continue
		}
		key := normalizeReviewName(table.Name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tables = append(tables, table)
	}
	return tables
}

func ambiguousPolicyColumns(tables []manifest.Table) map[string]bool {
	owners := map[string]map[string]struct{}{}
	add := func(table, col string) {
		col = normalizeReviewName(col)
		if col == "" {
			return
		}
		if owners[col] == nil {
			owners[col] = map[string]struct{}{}
		}
		owners[col][normalizeReviewName(table)] = struct{}{}
	}
	for _, table := range tables {
		for _, required := range requiredColumnsForManifestTable(table) {
			add(table.Name, required.column)
		}
		if softDeleteColumnForManifestTable(table) != "" {
			add(table.Name, softDeleteColumnForManifestTable(table))
		}
	}
	out := map[string]bool{}
	for col, ownerSet := range owners {
		if len(ownerSet) > 1 {
			out[col] = true
		}
	}
	return out
}

func manifestPolicyFindingsForTable(ctx reviewContext, table manifest.Table, plan *query.QueryPlan, loc *query.SourceLocation, ambiguous map[string]bool) []Finding {
	if plan.Operation != query.OperationSelect && plan.Operation != query.OperationUpdate && plan.Operation != query.OperationDelete {
		return nil
	}
	var findings []Finding
	for _, required := range requiredColumnsForManifestTable(table) {
		allowUnqualified := !ambiguous[normalizeReviewName(required.column)]
		if planHasPredicateForTable(plan, table.Name, required.column, allowUnqualified) {
			continue
		}
		code := query.WarningRequiredFilterMissing
		message := table.Name + " requires a filter on " + required.column
		if required.policyType == "tenant_scope" {
			code = query.WarningTenantFilterMissing
			message = table.Name + " is tenant-scoped but " + required.column + " filter is missing"
		}
		findings = append(findings, staticFinding(
			code,
			policyModeLevel(required.mode, query.RiskHigh),
			message,
			"add the required filter before executing this query",
			loc,
			query.AnalysisPrecise,
		))
	}
	if softDelete := softDeleteColumnForManifestTable(table); softDelete != "" && !staticPlanWithDeleted(plan) {
		allowUnqualified := !ambiguous[normalizeReviewName(softDelete)]
		if !planHasPredicateForTable(plan, table.Name, softDelete, allowUnqualified) {
			findings = append(findings, staticFinding(
				query.WarningSoftDeleteFilterMissing,
				policyModeLevel(policyModeForManifestPolicy(table, "soft_delete", softDelete), query.RiskMedium),
				table.Name+" has soft delete policy but "+softDelete+" filter is missing",
				"use the default soft delete filter or explicitly call WithDeleted",
				loc,
				query.AnalysisPrecise,
			))
		}
	}
	if plan.Operation == query.OperationSelect {
		for _, col := range selectedPIIColumnsForManifestTable(table, plan) {
			findings = append(findings, staticFinding(
				query.WarningPIIColumnSelected,
				policyModeLevel(policyModeForManifestPolicy(table, "pii", col), query.RiskMedium),
				"PII column selected: "+table.Name+"."+col,
				"avoid selecting PII or include a narrow access reason",
				loc,
				query.AnalysisPrecise,
			))
		}
	}
	return findings
}

type manifestRequiredColumn struct {
	column     string
	mode       query.PolicyMode
	policyType string
}

func requiredColumnsForManifestTable(table manifest.Table) []manifestRequiredColumn {
	seen := map[string]manifestRequiredColumn{}
	for _, column := range table.Columns {
		if column.TenantScope || column.RequiredFilter {
			policyType := "required_filter"
			if column.TenantScope {
				policyType = "tenant_scope"
			}
			seen[normalizeReviewName(column.Name)] = manifestRequiredColumn{column: column.Name, mode: query.PolicyModeEnforce, policyType: policyType}
		}
	}
	for _, policy := range table.Policies {
		if policy.Type != "tenant_scope" && policy.Type != "required_filter" {
			continue
		}
		mode := policy.Mode
		if mode == "" {
			mode = query.PolicyModeEnforce
		}
		seen[normalizeReviewName(policy.Column)] = manifestRequiredColumn{column: policy.Column, mode: mode, policyType: policy.Type}
	}
	out := make([]manifestRequiredColumn, 0, len(seen))
	for _, required := range seen {
		out = append(out, required)
	}
	return out
}

func softDeleteColumnForManifestTable(table manifest.Table) string {
	for _, policy := range table.Policies {
		if policy.Type == "soft_delete" && strings.TrimSpace(policy.Column) != "" {
			return policy.Column
		}
	}
	for _, column := range table.Columns {
		if column.SoftDelete {
			return column.Name
		}
	}
	return ""
}

func policyModeForManifestPolicy(table manifest.Table, policyType, column string) query.PolicyMode {
	for _, policy := range table.Policies {
		if policy.Type == policyType && normalizeReviewName(policy.Column) == normalizeReviewName(column) {
			if policy.Mode != "" {
				return policy.Mode
			}
		}
	}
	return query.PolicyModeWarn
}

func staticPlanWithDeleted(plan *query.QueryPlan) bool {
	if plan.Metadata == nil {
		return false
	}
	v, _ := plan.Metadata["soft_delete"].(string)
	return v == "with_deleted"
}

func selectedPIIColumnsForManifestTable(table manifest.Table, plan *query.QueryPlan) []string {
	selectedAll := false
	selected := map[string]struct{}{}
	for _, column := range plan.Columns {
		if strings.TrimSpace(column.Name) == "*" || strings.TrimSpace(column.Expression) == "*" {
			selectedAll = true
			continue
		}
		if column.Name != "" {
			selected[normalizeReviewName(column.Name)] = struct{}{}
		}
	}
	var out []string
	for _, column := range table.Columns {
		if !column.PII {
			continue
		}
		if selectedAll {
			out = append(out, column.Name)
			continue
		}
		if _, ok := selected[normalizeReviewName(column.Name)]; ok {
			out = append(out, column.Name)
		}
	}
	return out
}

func planHasPredicateForTable(plan *query.QueryPlan, table, column string, allowUnqualified bool) bool {
	target := normalizeReviewName(column)
	for _, predicate := range plan.Predicates {
		if reviewColumnMatches(predicate.Column, table, target, allowUnqualified) ||
			reviewColumnMatches(predicate.ValueColumn, table, target, allowUnqualified) {
			return true
		}
	}
	return false
}

func reviewColumnMatches(reference, table, column string, allowUnqualified bool) bool {
	qualifier, name := splitReviewColumn(reference)
	if name == "" || name != column {
		return false
	}
	if qualifier == "" {
		return allowUnqualified
	}
	return qualifier == normalizeReviewName(table)
}

func splitReviewColumn(reference string) (string, string) {
	reference = strings.TrimSpace(reference)
	reference = strings.Trim(reference, "`\"")
	if reference == "" {
		return "", ""
	}
	idx := strings.LastIndex(reference, ".")
	if idx < 0 {
		return "", normalizeReviewName(reference)
	}
	return normalizeReviewName(reference[:idx]), normalizeReviewName(reference[idx+1:])
}

func policyModeLevel(mode query.PolicyMode, fallback query.RiskLevel) query.RiskLevel {
	switch mode {
	case query.PolicyModeWarn:
		return fallback
	case query.PolicyModeEnforce:
		if compareRisk(fallback, query.RiskHigh) < 0 {
			return query.RiskHigh
		}
		return fallback
	case query.PolicyModeBlock:
		return query.RiskBlocked
	default:
		return fallback
	}
}

func reviewRawBuilderCall(sel *ast.SelectorExpr, call *ast.CallExpr, loc *query.SourceLocation) []Finding {
	if len(call.Args) == 0 {
		return nil
	}
	sqlText, ok := stringLiteralValue(call.Args[0])
	if !ok {
		return []Finding{staticFinding(
			query.WarningStaticReviewUnsupported,
			query.RiskMedium,
			"dynamic raw SQL fragment could not be inspected",
			"prefer structured predicates or keep raw SQL fragments literal and reviewed",
			loc,
			query.AnalysisUnsupported,
		)}
	}
	if normalizedContainsWeakPredicate(sqlText) {
		return []Finding{staticFinding(
			query.WarningWeakPredicate,
			query.RiskHigh,
			"query contains a weak predicate such as 1=1",
			"replace weak predicates with a meaningful filter",
			loc,
			query.AnalysisPrecise,
		)}
	}
	return nil
}

func staticReviewPartial(loc *query.SourceLocation) Finding {
	return staticFinding(
		query.WarningStaticReviewPartial,
		query.RiskMedium,
		"could not fully reconstruct Goquent query chain",
		"run query.Plan(ctx) in tests or keep the query chain inline for review",
		loc,
		query.AnalysisPartial,
	)
}

func staticFinding(code string, level query.RiskLevel, message, hint string, loc *query.SourceLocation, precision query.AnalysisPrecision) Finding {
	return Finding{
		Code:              code,
		Level:             level,
		Message:           message,
		Location:          cloneLocation(loc),
		Hint:              hint,
		AnalysisPrecision: precision,
	}
}

func sourceLocation(fset *token.FileSet, path string, pos token.Pos) *query.SourceLocation {
	p := fset.PositionFor(pos, false)
	return &query.SourceLocation{File: path, Line: p.Line, Column: p.Column}
}

func isPredicateMethod(method string) bool {
	if strings.Contains(method, "Where") {
		return true
	}
	switch method {
	case "Having", "HavingRaw", "OrHaving", "OrHavingRaw":
		return true
	default:
		return false
	}
}

func isRawSQLMethod(method string) bool {
	switch method {
	case "Exec", "ExecContext", "Query", "QueryContext", "QueryRow", "QueryRowContext", "RawPlan":
		return true
	default:
		return false
	}
}

func rawSQLArgIndex(method string) int {
	switch method {
	case "Exec", "Query", "QueryRow":
		return 0
	case "ExecContext", "QueryContext", "QueryRowContext", "RawPlan":
		return 1
	default:
		return -1
	}
}

func isGoquentTerminal(method string) bool {
	switch method {
	case "Get", "GetMaps", "First", "FirstMap", "Plan", "Update", "PlanUpdate", "Delete", "PlanDelete":
		return true
	default:
		return false
	}
}

func isRawBuilderMethod(method string) bool {
	switch method {
	case "SelectRaw", "WhereRaw", "OrWhereRaw", "SafeWhereRaw", "SafeOrWhereRaw", "HavingRaw", "OrHavingRaw", "OrderByRaw":
		return true
	default:
		return false
	}
}

func stringLiteralValue(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(e.Value)
		if err != nil {
			return "", false
		}
		return value, true
	case *ast.BinaryExpr:
		if e.Op != token.ADD {
			return "", false
		}
		left, ok := stringLiteralValue(e.X)
		if !ok {
			return "", false
		}
		right, ok := stringLiteralValue(e.Y)
		if !ok {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return stringLiteralValue(e.X)
	default:
		return "", false
	}
}

func looksLikeSQL(s string) bool {
	upper := strings.ToUpper(s)
	for _, token := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "TRUNCATE", "ALTER", "WITH"} {
		if containsSQLWord(upper, token) {
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
