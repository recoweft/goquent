package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/recoweft/goquent/orm/driver"
	qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/predicate"
)

// TenantPolicyResult explains a conditional result. Public fields are not evidence.
type TenantPolicyResult struct {
	Status           string            `json:"status"`
	Reason           string            `json:"reason"`
	Precision        AnalysisPrecision `json:"precision"`
	Provenance       string            `json:"provenance"`
	SchemaFreshness  string            `json:"schema_freshness"`
	PolicyFreshness  string            `json:"policy_freshness"`
	ExecutorIdentity string            `json:"executor_identity"`
}

type tenantEvidence struct {
	view    *writeEvidence
	failure string
}

type tenantTarget struct {
	table, alias string
	schema       ApplicationTable
	policy       TablePolicy
}

func tenantError(reason string) error {
	return fmt.Errorf("%w: tenant_policy/%s", ErrBlockedOperation, reason)
}
func (q *Query) tenantTargets(p *QueryPlan) ([]tenantTarget, error) {
	s := q.settings
	if s.schema.input == nil || s.database == "" || s.database != s.schema.input.Database {
		return nil, tenantError("application_schema_missing_or_database_mismatch")
	}
	dialect := ""
	switch q.dialect.(type) {
	case driver.MySQLDialect:
		dialect = "mysql"
	case driver.PostgresDialect:
		dialect = "postgres"
	}
	if dialect == "" || dialect != s.schema.input.Dialect {
		return nil, tenantError("dialect_mismatch")
	}
	if len(p.Tables) == 0 {
		return nil, tenantError("target_missing")
	}
	if len(p.Joins) > 0 && len(p.Tables) != len(p.Joins)+1 {
		return nil, tenantError("duplicate_join_target")
	}
	var targets []tenantTarget
	seen := map[string]bool{}
	for _, ref := range p.Tables {
		table, alias := splitTableAlias(ref.Name)
		if ref.Alias != "" {
			alias = ref.Alias
		}
		if !scopePath(table) || (alias != "" && !scopeIdent(alias)) {
			return nil, tenantError("table_or_alias_unsupported")
		}
		qualifier := table
		if alias != "" {
			qualifier = alias
		}
		if seen[qualifier] {
			return nil, tenantError("ambiguous_alias")
		}
		seen[qualifier] = true
		t := tenantTarget{table: table, alias: alias}
		found := false
		for _, schema := range s.schema.input.Tables {
			if schema.Table == table {
				t.schema = schema
				found = true
				break
			}
		}
		if !found {
			return nil, tenantError("table_schema_missing")
		}
		if !t.schema.PlainTable {
			return nil, tenantError("table_rewrite_semantics_unconfirmed")
		}
		t.policy, _ = s.policies.PolicyForTable(table)
		if t.policy.Table != "" && t.policy.Table != table {
			return nil, tenantError("policy_identifier_normalization_unsupported")
		}
		for _, col := range append(append(append([]string{}, t.policy.PIIColumns...), t.policy.ForbiddenColumns...), t.policy.ImmutableColumns...) {
			if _, ok := tenantColumn(t, col); !ok {
				return nil, tenantError("policy_column_unresolved")
			}
		}
		if t.policy.TenantColumn != "" {
			c, ok := tenantColumn(t, t.policy.TenantColumn)
			k := WriteKeyConstraint{Kind: "unique", Columns: []WriteKeyColumn{c}, AllRows: true, Valid: true, NotDeferrable: true}
			if !ok || c.Nullable || !validWriteKey(k, dialect) {
				return nil, tenantError("tenant_schema_unsupported")
			}
			e := s.execution
			if !e.application || !e.input.TenantPresent || e.input.CurrentTenant == nil {
				return nil, tenantError("application_tenant_missing")
			}
			if _, ok := scopeInteger(e.input.CurrentTenant, c.Bits); !ok {
				return nil, tenantError("tenant_value_unsupported")
			}
		}
		targets = append(targets, t)
	}
	return targets, nil
}
func tenantColumn(t tenantTarget, name string) (WriteKeyColumn, bool) {
	for _, c := range t.schema.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return WriteKeyColumn{}, false
}
func tenantResolve(ref string, ts []tenantTarget) (int, string, bool) {
	if !scopeColumn(ref) {
		return 0, "", false
	}
	qualifier, col := "", ref
	if i := strings.LastIndex(ref, "."); i >= 0 {
		qualifier, col = ref[:i], ref[i+1:]
	}
	found := -1
	for i, t := range ts {
		name := t.table
		if t.alias != "" {
			name = t.alias
		}
		if qualifier != "" && name != qualifier {
			continue
		}
		if _, ok := tenantColumn(t, col); ok {
			if found >= 0 {
				return 0, "", false
			}
			found = i
		}
	}
	return found, col, found >= 0
}
func tenantTreeKnown(n *predicate.Node, ts []tenantTarget) bool {
	if n == nil {
		return true
	}
	if n.Correspondence != "generated" || n.OpaqueReason != "" {
		return false
	}
	switch n.Kind {
	case "and", "or", "group", "not":
		if len(n.Children) == 0 {
			return false
		}
		for _, c := range n.Children {
			if !tenantTreeKnown(c, ts) {
				return false
			}
		}
	case "comparison", "in", "between", "null", "column":
		if _, _, ok := tenantResolve(n.Column, ts); !ok {
			return false
		}
		if n.Kind == "column" {
			if _, _, ok := tenantResolve(n.ValueColumn, ts); !ok {
				return false
			}
		}
		for _, c := range n.BoundColumns {
			if _, _, ok := tenantResolve(c, ts); !ok {
				return false
			}
		}
		for _, v := range n.Values {
			if v.Isolation != "detached" || !scopeScalar(v.Data) {
				return false
			}
		}
		switch strings.ToUpper(n.Operator) {
		case "=", "!=", "<>", ">", ">=", "<", "<=", "LIKE", "NOT LIKE", "IN", "NOT IN", "BETWEEN", "NOT BETWEEN", "IS NULL", "IS NOT NULL":
		default:
			return false
		}
	default:
		return false
	}
	return true
}
func tenantBound(n *predicate.Node, ts []tenantTarget, target int, want int64) bool {
	if n == nil {
		return false
	}
	switch n.Kind {
	case "group":
		return len(n.Children) == 1 && tenantBound(n.Children[0], ts, target, want)
	case "and":
		for _, c := range n.Children {
			if tenantBound(c, ts, target, want) {
				return true
			}
		}
	case "or":
		if len(n.Children) == 0 {
			return false
		}
		for _, c := range n.Children {
			if !tenantBound(c, ts, target, want) {
				return false
			}
		}
		return true
	case "comparison":
		i, col, ok := tenantResolve(n.Column, ts)
		if !ok || i != target || col != ts[target].policy.TenantColumn || n.Operator != "=" || len(n.Values) != 1 {
			return false
		}
		schema, _ := tenantColumn(ts[target], col)
		got, ok := scopeInteger(n.Values[0].Data, schema.Bits)
		return ok && got == want
	}
	return false
}
func (q *Query) strictPolicyCheck(p *QueryPlan) error {
	ts, err := q.tenantTargets(p)
	if err != nil {
		return err
	}
	if p.Operation == OperationRaw {
		return tenantError("raw_unsupported")
	}
	for _, u := range p.Unverified {
		if u != "join_conditions_not_inspected" {
			return tenantError("statement_unsupported")
		}
	}
	if p.HavingTree != nil || !tenantTreeKnown(p.WhereTree, ts) {
		return tenantError("condition_unknown")
	}
	if p.Operation != OperationSelect && len(p.Joins) > 0 {
		return tenantError("write_join_unsupported")
	}
	for _, j := range p.Joins {
		if j.Subquery || (j.Type != "INNER" && j.Type != "LEFT") {
			return tenantError("join_type_or_subquery_unsupported")
		}
		if j.OnTree != nil {
			if !tenantTreeKnown(j.OnTree, ts) {
				return tenantError("join_condition_unknown")
			}
		} else {
			if _, _, ok := tenantResolve(j.LeftColumn, ts); !ok {
				return tenantError("join_column_unknown")
			}
			if _, _, ok := tenantResolve(j.RightColumn, ts); !ok {
				return tenantError("join_column_unknown")
			}
			if j.Operator != "=" {
				return tenantError("join_operator_unsupported")
			}
		}
	}
	for i, t := range ts {
		if i > 0 && t.policy.SoftDeleteColumn != "" {
			return tenantError("joined_soft_delete_unsupported")
		}
	}
	if p.Operation != OperationInsert {
		for i, t := range ts {
			for _, col := range t.policy.RequiredFilterColumns {
				if !requiredTenantFilter(p.WhereTree, ts, i, col) {
					return tenantError("required_filter_unproven")
				}
			}
			if t.policy.TenantColumn == "" {
				continue
			}
			c, _ := tenantColumn(t, t.policy.TenantColumn)
			want, _ := scopeInteger(q.settings.execution.input.CurrentTenant, c.Bits)
			bound := tenantBound(p.WhereTree, ts, i, want)
			if i > 0 {
				for _, j := range p.Joins {
					jt, ja := splitTableAlias(j.Table)
					if j.Alias != "" {
						ja = j.Alias
					}
					if jt == t.table && ja == t.alias {
						bound = bound || tenantBound(j.OnTree, ts, i, want)
					}
				}
			}
			if !bound {
				return tenantError("all_branch_binding_missing:" + t.table + ":" + t.alias)
			}
		}
	}
	if p.Operation == OperationSelect {
		if err := tenantProjection(p.Columns, ts); err != nil {
			return err
		}
	}
	// External high-risk permits are not implemented. Reasons/suppression and
	// caller-selected risk thresholds cannot replace that missing authority.
	risk := (defaultRiskEngine{}).CheckQuery(p)
	configured := (defaultRiskEngine{config: q.settings.risk}).CheckQuery(p)
	if risk.Blocked || configured.Blocked || requiresApprovalLevel(risk.Level) || requiresApprovalLevel(configured.Level) {
		return tenantError("external_high_risk_permit_missing")
	}
	return nil
}
func tenantProjection(cols []ColumnRef, ts []tenantTarget) error {
	for _, c := range cols {
		if c.Raw || c.Expression != "" || c.Function != "" {
			return tenantError("projection_unknown")
		}
		if c.Count && c.Name == "*" {
			continue
		}
		i, col, ok := tenantResolve(c.Name, ts)
		if !ok {
			return tenantError("projection_unknown")
		}
		t := ts[i]
		for _, bad := range append(append([]string{}, t.policy.PIIColumns...), t.policy.ForbiddenColumns...) {
			if col == bad {
				return tenantError("protected_projection")
			}
		}
	}
	return nil
}
func (q *Query) finalizeTenantPolicy(p *QueryPlan) {
	if !q.settings.strict {
		return
	}
	err := q.strictPolicyCheck(p)
	r := &TenantPolicyResult{Status: "conditional_pass", Precision: AnalysisPrecise, Provenance: "builder_application_tenant_and_application_asserted_schema", SchemaFreshness: "unknown", PolicyFreshness: "unknown", ExecutorIdentity: "unverified"}
	if err != nil {
		r.Status = "unknown_or_rejected"
		r.Precision = AnalysisPartial
		r.Reason = err.Error()
		p.Blocked = true
		p.RiskLevel = RiskBlocked
		p.Warnings = append(p.Warnings, Warning{Code: "TENANT_POLICY_REJECTED", Level: RiskBlocked, Message: r.Reason})
	}
	p.TenantPolicy = r
	e := &tenantEvidence{}
	if err != nil {
		e.failure = err.Error()
	} else if failure := comparableWriteView(writeView(p)); failure != "" {
		e.failure = failure
		p.Blocked = true
		r.Status = "unknown_or_rejected"
		r.Reason = failure
		r.Precision = AnalysisPartial
	}
	if e.failure == "" {
		v := writeView(p)
		v.Params = valuecopy.Slice(v.Params)
		v.Where = valuecopy.Node(v.Where)
		v.Having = valuecopy.Node(v.Having)
		v.Tables = append([]TableRef(nil), v.Tables...)
		v.Columns = append([]ColumnRef(nil), v.Columns...)
		v.Joins = append([]JoinRef(nil), v.Joins...)
		for i := range v.Joins {
			v.Joins[i].OnTree = valuecopy.Node(v.Joins[i].OnTree)
		}
		v.Unverified = append([]string(nil), v.Unverified...)
		e.view = v
	}
	p.tenantEvidence = e
}
func checkTenantEvidence(p *QueryPlan) error {
	e := p.tenantEvidence
	if e == nil {
		return nil
	}
	if e.failure != "" {
		return tenantError(e.failure)
	}
	v := writeView(p)
	if comparableWriteView(v) != "" || !reflect.DeepEqual(v, e.view) {
		return tenantError("sql_values_tree_or_target_changed")
	}
	return nil
}

// policyBuilder applies mandatory predicates to a detached builder on every
// plan, so later OR additions cannot escape a previously appended predicate.
func (q *Query) policyBuilder(src *qbapi.SelectQueryBuilder) (*qbapi.SelectQueryBuilder, error) {
	if !q.settings.strict {
		if q.policy == nil || q.policy.SoftDeleteColumn == "" || q.withDeleted {
			return src, nil
		}
		b := newSelectBuilder(q.dialect)
		src.CopyStateToSelect(b)
		if q.onlyDeleted {
			b.WhereNotNull(q.policy.SoftDeleteColumn)
		} else {
			b.WhereNull(q.policy.SoftDeleteColumn)
		}
		return b, nil
	}
	b := newSelectBuilder(q.dialect)
	src.CopyStateToSelect(b)
	snapshot := b.Snapshot()
	if snapshot.Error != nil {
		return nil, snapshot.Error
	}
	p := &QueryPlan{}
	appendTableRef(p, snapshot.Table, "")
	appendJoinMetadata(p, snapshot.Joins)
	ts, err := q.tenantTargets(p)
	if err != nil {
		return nil, err
	}
	t := ts[0]
	prefix := t.table
	if t.alias != "" {
		prefix = t.alias
	}
	b.GroupWhere()
	if q.settings.autoTenant && t.policy.TenantColumn != "" {
		b.Where(prefix+"."+t.policy.TenantColumn, "=", q.settings.execution.input.CurrentTenant)
	}
	if t.policy.SoftDeleteColumn != "" && !q.withDeleted {
		col := prefix + "." + t.policy.SoftDeleteColumn
		if q.onlyDeleted {
			b.WhereNotNull(col)
		} else {
			b.WhereNull(col)
		}
	}
	return b, nil
}

func (q *Query) strictInValue(v any) bool {
	if !q.settings.strict {
		return true
	}
	copy, ok := valuecopy.Copy(v)
	if !ok || copy == nil || reflect.TypeOf(copy).Kind() != reflect.Slice {
		q.err = tenantError("in_value_unsupported")
		return false
	}
	return true
}

func requiredTenantFilter(n *predicate.Node, ts []tenantTarget, target int, col string) bool {
	if n == nil {
		return false
	}
	switch n.Kind {
	case "group":
		return len(n.Children) == 1 && requiredTenantFilter(n.Children[0], ts, target, col)
	case "and":
		for _, c := range n.Children {
			if requiredTenantFilter(c, ts, target, col) {
				return true
			}
		}
	case "or":
		if len(n.Children) == 0 {
			return false
		}
		for _, c := range n.Children {
			if !requiredTenantFilter(c, ts, target, col) {
				return false
			}
		}
		return true
	case "comparison":
		i, name, ok := tenantResolve(n.Column, ts)
		return ok && i == target && name == col && n.Operator == "=" && len(n.Values) == 1 && n.Values[0].Data != nil
	}
	return false
}
