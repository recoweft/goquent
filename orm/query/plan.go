package query

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"
)

// OperationType describes the structural SQL operation represented by a plan.
type OperationType string

const (
	OperationSelect OperationType = "select"
	OperationInsert OperationType = "insert"
	OperationUpdate OperationType = "update"
	OperationDelete OperationType = "delete"
	OperationRaw    OperationType = "raw"
)

// RiskLevel is structural database risk, not a business-safety guarantee.
type RiskLevel string

const (
	RiskLow         RiskLevel = "low"
	RiskMedium      RiskLevel = "medium"
	RiskHigh        RiskLevel = "high"
	RiskDestructive RiskLevel = "destructive"
	RiskBlocked     RiskLevel = "blocked"
)

// AnalysisPrecision describes how precisely Goquent could explain a query.
type AnalysisPrecision string

const (
	AnalysisPrecise     AnalysisPrecision = "precise"
	AnalysisPartial     AnalysisPrecision = "partial"
	AnalysisUnsupported AnalysisPrecision = "unsupported"
)

const (
	WarningUpdateWithoutWhere       = "UPDATE_WITHOUT_WHERE"
	WarningDeleteWithoutWhere       = "DELETE_WITHOUT_WHERE"
	WarningSelectStarUsed           = "SELECT_STAR_USED"
	WarningLimitMissing             = "LIMIT_MISSING"
	WarningRawSQLUsed               = "RAW_SQL_USED"
	WarningBulkUpdateDetected       = "BULK_UPDATE_DETECTED"
	WarningBulkDeleteDetected       = "BULK_DELETE_DETECTED"
	WarningDestructiveSQL           = "DESTRUCTIVE_SQL_DETECTED"
	WarningWeakPredicate            = "WEAK_PREDICATE"
	WarningSuppressionExpired       = "SUPPRESSION_EXPIRED"
	WarningSuppressionNotAllowed    = "SUPPRESSION_NOT_ALLOWED"
	WarningStaticReviewPartial      = "STATIC_REVIEW_PARTIAL"
	WarningStaticReviewUnsupported  = "STATIC_REVIEW_UNSUPPORTED"
	WarningRequiredPredicateMissing = "REQUIRED_PREDICATE_MISSING"
)

// SourceLocation points at source code when a plan/finding is derived from static analysis.
type SourceLocation struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// Evidence stores machine-readable supporting details for a warning.
type Evidence struct {
	Key   string `json:"key"`
	Value any    `json:"value,omitempty"`
}

// Warning is a reviewable issue attached to a plan.
type Warning struct {
	Code           string          `json:"code"`
	Level          RiskLevel       `json:"level"`
	Message        string          `json:"message"`
	Location       *SourceLocation `json:"location,omitempty"`
	Hint           string          `json:"hint,omitempty"`
	Evidence       []Evidence      `json:"evidence,omitempty"`
	Suppressible   bool            `json:"suppressible"`
	RequiresReason bool            `json:"requires_reason"`
}

// Approval records an explicit approval reason for a risky operation.
type Approval struct {
	Reason    string     `json:"reason"`
	Scope     string     `json:"scope,omitempty"`
	CreatedBy string     `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// TableRef describes a table touched by the query.
type TableRef struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

// ColumnRef describes a selected, inserted, or updated column.
type ColumnRef struct {
	Table      string `json:"table,omitempty"`
	Name       string `json:"name,omitempty"`
	Expression string `json:"expression,omitempty"`
	Raw        bool   `json:"raw,omitempty"`
	Distinct   bool   `json:"distinct,omitempty"`
	Count      bool   `json:"count,omitempty"`
	Function   string `json:"function,omitempty"`
}

// JoinRef describes a JOIN visible in the query builder metadata.
type JoinRef struct {
	Type        string `json:"type,omitempty"`
	Table       string `json:"table,omitempty"`
	Alias       string `json:"alias,omitempty"`
	LeftColumn  string `json:"left_column,omitempty"`
	Operator    string `json:"operator,omitempty"`
	RightColumn string `json:"right_column,omitempty"`
	Subquery    bool   `json:"subquery,omitempty"`
}

// PredicateRef describes a WHERE-like predicate visible in the query builder metadata.
type PredicateRef struct {
	Group       int    `json:"group,omitempty"`
	Connector   string `json:"connector,omitempty"`
	Column      string `json:"column,omitempty"`
	Operator    string `json:"operator,omitempty"`
	ValueCount  int    `json:"value_count,omitempty"`
	ValueColumn string `json:"value_column,omitempty"`
	Raw         string `json:"raw,omitempty"`
	Function    string `json:"function,omitempty"`
	Subquery    bool   `json:"subquery,omitempty"`
	Negated     bool   `json:"negated,omitempty"`
}

// QueryPlan explains SQL and metadata before the query is executed.
type QueryPlan struct {
	Operation          OperationType     `json:"operation"`
	SQL                string            `json:"sql"`
	Params             []any             `json:"params"`
	Tables             []TableRef        `json:"tables,omitempty"`
	Columns            []ColumnRef       `json:"columns,omitempty"`
	Joins              []JoinRef         `json:"joins,omitempty"`
	Predicates         []PredicateRef    `json:"predicates,omitempty"`
	Limit              *int64            `json:"limit,omitempty"`
	Offset             *int64            `json:"offset,omitempty"`
	EstimatedRows      *int64            `json:"estimated_rows,omitempty"`
	UsesIndex          *bool             `json:"uses_index,omitempty"`
	RiskLevel          RiskLevel         `json:"risk_level"`
	Warnings           []Warning         `json:"warnings,omitempty"`
	SuppressedWarnings []Warning         `json:"suppressed_warnings,omitempty"`
	RequiredApproval   bool              `json:"required_approval"`
	Blocked            bool              `json:"blocked,omitempty"`
	Approval           *Approval         `json:"approval,omitempty"`
	AnalysisPrecision  AnalysisPrecision `json:"analysis_precision"`
	Metadata           map[string]any    `json:"metadata,omitempty"`
}

// MetadataTableRisk stores []TableRiskMetadata in QueryPlan.Metadata.
const MetadataTableRisk = "table_risk_metadata"

// MetadataRequiredPredicates stores []RequiredPredicate in QueryPlan.Metadata.
const MetadataRequiredPredicates = "required_predicates"

// RequiredPredicate describes a repository-level predicate guard.
type RequiredPredicate struct {
	Table  string `json:"table,omitempty"`
	Column string `json:"column"`
}

// TableRiskMetadata gives the risk engine table key context without depending
// on the manifest package.
type TableRiskMetadata struct {
	Table                 string     `json:"table"`
	PrimaryKeyColumns     []string   `json:"primary_key_columns,omitempty"`
	UniqueIndexes         [][]string `json:"unique_indexes,omitempty"`
	TenantColumn          string     `json:"tenant_column,omitempty"`
	SoftDeleteColumn      string     `json:"soft_delete_column,omitempty"`
	RequiredFilterColumns []string   `json:"required_filter_columns,omitempty"`
}

// AttachTableRiskMetadata attaches table key metadata used by risk checks.
func AttachTableRiskMetadata(plan *QueryPlan, metadata []TableRiskMetadata) {
	if plan == nil || len(metadata) == 0 {
		return
	}
	if plan.Metadata == nil {
		plan.Metadata = make(map[string]any, 1)
	}
	plan.Metadata[MetadataTableRisk] = append([]TableRiskMetadata(nil), metadata...)
}

// PlanHasPredicateColumn reports whether plan contains a predicate for column.
//
// If table is non-empty and the plan touches multiple tables, the predicate must
// be qualified by that table or its alias. For single-table plans, unqualified
// predicates are accepted.
func PlanHasPredicateColumn(plan *QueryPlan, table, column string) bool {
	table = strings.TrimSpace(table)
	allowUnqualified := table == "" || countPlanTables(plan) <= 1
	return hasPolicyPredicateColumn(plan, table, column, allowUnqualified)
}

// MissingRequiredPredicates returns required predicates not present in plan.
func MissingRequiredPredicates(plan *QueryPlan, required []RequiredPredicate) []RequiredPredicate {
	out := make([]RequiredPredicate, 0)
	seen := make(map[string]struct{}, len(required))
	for _, req := range required {
		req.Table = strings.TrimSpace(req.Table)
		req.Column = strings.TrimSpace(req.Column)
		if req.Column == "" {
			continue
		}
		key := normalizeTableName(req.Table) + "." + normalizeColumnName(req.Column)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if !PlanHasPredicateColumn(plan, req.Table, req.Column) {
			out = append(out, req)
		}
	}
	return out
}

// RequiresApproval reports whether this plan needs explicit approval.
func (p *QueryPlan) RequiresApproval() bool {
	return p != nil && p.RequiredApproval
}

// ToJSON returns stable, indented JSON for the plan.
func (p *QueryPlan) ToJSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// String returns a compact pretty format suitable for logs and CLI output.
func (p *QueryPlan) String() string {
	if p == nil {
		return "<nil query plan>"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s query plan\n", p.Operation)
	fmt.Fprintf(&b, "risk: %s\n", p.RiskLevel)
	fmt.Fprintf(&b, "precision: %s\n", p.AnalysisPrecision)
	if p.RequiredApproval {
		b.WriteString("requires_approval: true\n")
	}
	fmt.Fprintf(&b, "sql: %s\n", p.SQL)
	if len(p.Params) > 0 {
		fmt.Fprintf(&b, "params: %v\n", p.Params)
	}
	if len(p.Tables) > 0 {
		fmt.Fprintf(&b, "tables: %s\n", tableRefsString(p.Tables))
	}
	if len(p.Columns) > 0 {
		fmt.Fprintf(&b, "columns: %s\n", columnRefsString(p.Columns))
	}
	if len(p.Predicates) > 0 {
		fmt.Fprintf(&b, "predicates: %s\n", predicateRefsString(p.Predicates))
	}
	if p.Limit != nil {
		fmt.Fprintf(&b, "limit: %d\n", *p.Limit)
	}
	if p.Offset != nil {
		fmt.Fprintf(&b, "offset: %d\n", *p.Offset)
	}
	for _, w := range p.Warnings {
		fmt.Fprintf(&b, "warning[%s]: %s", w.Level, w.Code)
		if w.Message != "" {
			fmt.Fprintf(&b, " - %s", w.Message)
		}
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func newQueryPlan(op OperationType, sqlStr string, args []any) *QueryPlan {
	return &QueryPlan{
		Operation:         op,
		SQL:               sqlStr,
		Params:            append([]any(nil), args...),
		RiskLevel:         RiskLow,
		AnalysisPrecision: AnalysisPrecise,
	}
}

// NewRawPlan creates a plan for caller-supplied SQL. It does not execute SQL.
func NewRawPlan(sqlStr string, args ...any) *QueryPlan {
	plan := newQueryPlan(OperationRaw, sqlStr, args)
	finalizePlan(plan, nil, nil)
	return plan
}

// Plan builds a QueryPlan for the current SELECT query without executing it.
func (q *Query) Plan(ctx context.Context) (*QueryPlan, error) {
	return q.planSelectBuilder(ctx, q.builder)
}

func (q *Query) planSelectBuilder(ctx context.Context, builder *qbapi.SelectQueryBuilder) (*QueryPlan, error) {
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	if builder == q.builder {
		q.applyPolicyPredicates()
	}
	sqlStr, args, err := builder.Build()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationSelect, sqlStr, args)
	appendSelectBuilderMetadata(plan, builder)
	q.finalizePlan(plan)
	return plan, nil
}

func appendSelectBuilderMetadata(plan *QueryPlan, builder *qbapi.SelectQueryBuilder) {
	src := builder.Snapshot()
	appendTableRef(plan, src.Table, "")

	if len(src.Columns) > 0 {
		for _, c := range src.Columns {
			plan.Columns = append(plan.Columns, ColumnRef{
				Name:       c.Name,
				Expression: c.Expression,
				Raw:        c.Raw,
				Distinct:   c.Distinct,
				Count:      c.Count,
				Function:   c.Function,
			})
		}
	} else {
		plan.Columns = append(plan.Columns, ColumnRef{Name: "*"})
	}

	if src.Limit > 0 {
		v := src.Limit
		plan.Limit = &v
	}
	if src.Offset > 0 {
		v := src.Offset
		plan.Offset = &v
	}

	appendJoinMetadata(plan, src.Joins)
	appendPredicateMetadata(plan, src.Predicates)
}

func appendSelectBuilderWriteMetadata(plan *QueryPlan, builder *qbapi.SelectQueryBuilder) {
	src := builder.Snapshot()
	appendJoinMetadata(plan, src.Joins)
	appendPredicateMetadata(plan, src.Predicates)
}

func columnRefsFromNames(names []string) []ColumnRef {
	refs := make([]ColumnRef, 0, len(names))
	for _, name := range names {
		refs = append(refs, ColumnRef{Name: name})
	}
	return refs
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedBatchMapKeys(rows []map[string]any) []string {
	seen := make(map[string]struct{})
	for _, row := range rows {
		for k := range row {
			seen[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func appendTableRef(plan *QueryPlan, name, alias string) {
	if name == "" {
		return
	}
	for _, table := range plan.Tables {
		if table.Name == name && table.Alias == alias {
			return
		}
	}
	plan.Tables = append(plan.Tables, TableRef{Name: name, Alias: alias})
}

func appendJoinMetadata(plan *QueryPlan, joins []qbapi.JoinSnapshot) {
	for _, join := range joins {
		ref := JoinRef{
			Type:        join.Type,
			Table:       join.Table,
			Alias:       join.Alias,
			LeftColumn:  join.LeftColumn,
			Operator:    join.Operator,
			RightColumn: join.RightColumn,
			Subquery:    join.Subquery,
		}
		if ref.Table == "" && ref.Alias == "" {
			continue
		}
		plan.Joins = append(plan.Joins, ref)
		appendTableRef(plan, ref.Table, ref.Alias)
	}
}

func appendPredicateMetadata(plan *QueryPlan, predicates []qbapi.PredicateSnapshot) {
	for _, predicate := range predicates {
		plan.Predicates = append(plan.Predicates, PredicateRef{
			Group:       predicate.Group,
			Negated:     predicate.Negated,
			Connector:   predicate.Connector,
			Column:      predicate.Column,
			Operator:    predicate.Operator,
			ValueColumn: predicate.ValueColumn,
			Raw:         predicate.Raw,
			Function:    predicate.Function,
			Subquery:    predicate.Subquery,
			ValueCount:  predicate.ValueCount,
		})
	}
}

func tableRefsString(refs []TableRef) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Alias != "" {
			parts = append(parts, ref.Name+" as "+ref.Alias)
			continue
		}
		parts = append(parts, ref.Name)
	}
	return strings.Join(parts, ", ")
}

func columnRefsString(refs []ColumnRef) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		switch {
		case ref.Expression != "":
			parts = append(parts, ref.Expression)
		case ref.Function != "":
			parts = append(parts, ref.Function+"("+ref.Name+")")
		default:
			parts = append(parts, ref.Name)
		}
	}
	return strings.Join(parts, ", ")
}

func predicateRefsString(refs []PredicateRef) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Raw != "" {
			parts = append(parts, ref.Raw)
			continue
		}
		part := strings.TrimSpace(ref.Column + " " + ref.Operator)
		if ref.ValueCount > 0 {
			part += fmt.Sprintf(" [%d value(s)]", ref.ValueCount)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}
