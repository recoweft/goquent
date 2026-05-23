package operation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/manifest"
	"github.com/faciam-dev/goquent/orm/query"
)

const (
	OperationSelect = "select"

	WarningOperationPIISelected    = "OPERATION_SPEC_PII_SELECTED"
	WarningOperationRequiredFilter = "OPERATION_SPEC_REQUIRED_FILTER_MISSING"
	WarningOperationMissingLimit   = query.WarningLimitMissing
	WarningOperationStaleManifest  = manifest.WarningStale
)

var (
	ErrManifestRequired        = errors.New("goquent operation: manifest is required")
	ErrUnsupportedOperation    = errors.New("goquent operation: unsupported operation")
	ErrModelRequired           = errors.New("goquent operation: model is required")
	ErrUnknownModel            = errors.New("goquent operation: unknown model")
	ErrSelectRequired          = errors.New("goquent operation: explicit select fields are required")
	ErrUnknownField            = errors.New("goquent operation: unknown field")
	ErrForbiddenField          = errors.New("goquent operation: forbidden field")
	ErrInvalidFilter           = errors.New("goquent operation: invalid filter")
	ErrInvalidOrder            = errors.New("goquent operation: invalid order")
	ErrValueRefMissing         = errors.New("goquent operation: value_ref missing")
	ErrRequiredFilterMissing   = errors.New("goquent operation: required filter missing")
	ErrPIIAccessReasonRequired = errors.New("goquent operation: PII access reason required")
	ErrStaleManifest           = errors.New("goquent operation: stale manifest")
)

// OperationSpec is the read-only structured interface for AI-generated DB intent.
type OperationSpec struct {
	Operation    string       `json:"operation"`
	Model        string       `json:"model"`
	Select       []string     `json:"select,omitempty"`
	Filters      []FilterSpec `json:"filters,omitempty"`
	OrderBy      []OrderSpec  `json:"order_by,omitempty"`
	Limit        *int64       `json:"limit,omitempty"`
	AccessReason string       `json:"access_reason,omitempty"`
}

// UnmarshalJSON rejects non-MVP fields such as join, aggregate, mutation, or raw SQL hints.
func (s *OperationSpec) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	allowed := map[string]struct{}{
		"operation":     {},
		"model":         {},
		"select":        {},
		"filters":       {},
		"order_by":      {},
		"limit":         {},
		"access_reason": {},
	}
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%w: unsupported field %q", ErrUnsupportedOperation, key)
		}
	}
	type alias OperationSpec
	var decoded alias
	if err := json.Unmarshal(b, &decoded); err != nil {
		return err
	}
	*s = OperationSpec(decoded)
	return nil
}

// FilterSpec describes a single field predicate.
type FilterSpec struct {
	Field    string `json:"field"`
	Op       string `json:"op"`
	Value    any    `json:"value,omitempty"`
	ValueRef string `json:"value_ref,omitempty"`
}

// OrderSpec describes a single ordering term.
type OrderSpec struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

// Options controls validation and compilation.
type Options struct {
	Manifest             *manifest.Manifest
	Dialect              driver.Dialect
	Values               map[string]any
	RequireFreshManifest bool
	AccessReason         string
}

type validationResult struct {
	table    manifest.Table
	columns  map[string]manifest.Column
	warnings []query.Warning
}

// Validate checks an OperationSpec against a manifest and policy metadata.
func Validate(spec OperationSpec, opts Options) ([]query.Warning, error) {
	result, err := validate(spec, opts)
	if err != nil {
		return nil, err
	}
	return result.warnings, nil
}

// Compile validates spec and compiles it to a read-only QueryPlan.
func Compile(ctx context.Context, spec OperationSpec, opts Options) (*query.QueryPlan, error) {
	result, err := validate(spec, opts)
	if err != nil {
		return nil, err
	}
	dialect := opts.Dialect
	if dialect == nil {
		dialect = dialectFromManifest(opts.Manifest)
	}

	q := query.New(nil, result.table.Name, dialect)
	if reason := accessReason(spec, opts); reason != "" {
		q.AccessReason(reason)
	}
	q.Select(spec.Select...)

	if softDeleteColumn := tableSoftDeleteColumn(result.table); softDeleteColumn != "" && !specHasFilter(spec, softDeleteColumn) {
		q.WhereNull(softDeleteColumn)
	}
	for _, filter := range spec.Filters {
		if err := applyFilter(q, filter, opts); err != nil {
			return nil, err
		}
	}
	for _, order := range spec.OrderBy {
		q.OrderBy(order.Field, normalizeDirection(order.Direction))
	}
	if spec.Limit != nil {
		if *spec.Limit < 0 {
			return nil, fmt.Errorf("%w: limit must be non-negative", ErrInvalidFilter)
		}
		q.Limit(int(*spec.Limit))
	}

	plan, err := q.Plan(ctx)
	if err != nil {
		return nil, err
	}
	mergeWarningsIntoPlan(plan, result.warnings)
	return plan, nil
}

func validate(spec OperationSpec, opts Options) (validationResult, error) {
	if opts.Manifest == nil {
		return validationResult{}, ErrManifestRequired
	}
	op := strings.ToLower(strings.TrimSpace(spec.Operation))
	if op == "" {
		op = OperationSelect
	}
	if op != OperationSelect {
		return validationResult{}, fmt.Errorf("%w: %s", ErrUnsupportedOperation, spec.Operation)
	}
	if strings.TrimSpace(spec.Model) == "" {
		return validationResult{}, ErrModelRequired
	}
	if len(spec.Select) == 0 {
		return validationResult{}, ErrSelectRequired
	}

	table, ok := findTable(opts.Manifest, spec.Model)
	if !ok {
		return validationResult{}, fmt.Errorf("%w: %s", ErrUnknownModel, spec.Model)
	}
	result := validationResult{table: table, columns: columnMap(table)}
	if opts.Manifest.Verification != nil && !opts.Manifest.Verification.Fresh {
		if opts.RequireFreshManifest {
			return validationResult{}, ErrStaleManifest
		}
		result.warnings = append(result.warnings, warning(
			manifest.WarningStale,
			query.RiskHigh,
			"manifest is stale",
			"verify or regenerate the manifest before compiling operation specs",
		))
	}

	for _, field := range spec.Select {
		column, err := validateField(result.columns, field)
		if err != nil {
			return validationResult{}, err
		}
		if column.Forbidden {
			return validationResult{}, fmt.Errorf("%w: %s", ErrForbiddenField, field)
		}
		if column.PII {
			if accessReason(spec, opts) == "" {
				return validationResult{}, fmt.Errorf("%w: %s", ErrPIIAccessReasonRequired, field)
			}
			result.warnings = append(result.warnings, warning(
				WarningOperationPIISelected,
				query.RiskMedium,
				fmt.Sprintf("PII field selected: %s.%s", table.Name, column.Name),
				"avoid selecting PII unless the access reason is necessary and narrow",
			))
		}
	}
	for _, filter := range spec.Filters {
		if strings.TrimSpace(filter.Field) == "" {
			return validationResult{}, fmt.Errorf("%w: filter field is required", ErrInvalidFilter)
		}
		if _, err := validateField(result.columns, filter.Field); err != nil {
			return validationResult{}, err
		}
		if !supportedFilterOp(filter.Op) {
			return validationResult{}, fmt.Errorf("%w: unsupported operator %q", ErrInvalidFilter, filter.Op)
		}
		if strings.TrimSpace(filter.ValueRef) != "" {
			if opts.Values == nil {
				return validationResult{}, fmt.Errorf("%w: %s", ErrValueRefMissing, filter.ValueRef)
			}
			if _, ok := opts.Values[filter.ValueRef]; !ok {
				return validationResult{}, fmt.Errorf("%w: %s", ErrValueRefMissing, filter.ValueRef)
			}
		}
	}
	for _, order := range spec.OrderBy {
		if _, err := validateField(result.columns, order.Field); err != nil {
			return validationResult{}, err
		}
		dir := normalizeDirection(order.Direction)
		if dir != "asc" && dir != "desc" {
			return validationResult{}, fmt.Errorf("%w: unsupported direction %q", ErrInvalidOrder, order.Direction)
		}
	}

	for _, required := range requiredFilterColumns(table) {
		if specHasFilter(spec, required.column) {
			continue
		}
		if required.mode == query.PolicyModeWarn {
			result.warnings = append(result.warnings, warning(
				WarningOperationRequiredFilter,
				query.RiskHigh,
				fmt.Sprintf("%s requires a filter on %s", table.Name, required.column),
				"add the required filter before compiling this operation",
			))
			continue
		}
		return validationResult{}, fmt.Errorf("%w: %s.%s", ErrRequiredFilterMissing, table.Name, required.column)
	}
	if spec.Limit == nil {
		result.warnings = append(result.warnings, warning(
			WarningOperationMissingLimit,
			query.RiskMedium,
			"SELECT operation has no LIMIT",
			"add limit for list operations",
		))
	}
	return result, nil
}

func applyFilter(q *query.Query, filter FilterSpec, opts Options) error {
	op := normalizeFilterOp(filter.Op)
	switch op {
	case "is_null":
		q.WhereNull(filter.Field)
	case "is_not_null":
		q.WhereNotNull(filter.Field)
	case "in":
		q.WhereIn(filter.Field, filterValue(filter, opts))
	default:
		q.Where(filter.Field, op, filterValue(filter, opts))
	}
	return nil
}

func filterValue(filter FilterSpec, opts Options) any {
	if filter.ValueRef != "" {
		if opts.Values != nil {
			if value, ok := opts.Values[filter.ValueRef]; ok {
				return value
			}
		}
		return "$ref:" + filter.ValueRef
	}
	return filter.Value
}

func findTable(m *manifest.Manifest, modelName string) (manifest.Table, bool) {
	target := normalizeName(modelName)
	for _, table := range m.Tables {
		if normalizeName(table.Model) == target || normalizeName(table.Name) == target {
			return table, true
		}
	}
	return manifest.Table{}, false
}

func columnMap(table manifest.Table) map[string]manifest.Column {
	out := make(map[string]manifest.Column, len(table.Columns))
	for _, column := range table.Columns {
		out[normalizeName(column.Name)] = column
	}
	return out
}

func validateField(columns map[string]manifest.Column, field string) (manifest.Column, error) {
	field = strings.TrimSpace(field)
	if field == "" {
		return manifest.Column{}, fmt.Errorf("%w: empty field", ErrUnknownField)
	}
	if strings.ContainsAny(field, "()* ") {
		return manifest.Column{}, fmt.Errorf("%w: %s", ErrUnknownField, field)
	}
	column, ok := columns[normalizeName(field)]
	if !ok {
		return manifest.Column{}, fmt.Errorf("%w: %s", ErrUnknownField, field)
	}
	if column.Forbidden {
		return manifest.Column{}, fmt.Errorf("%w: %s", ErrForbiddenField, field)
	}
	return column, nil
}

type requiredFilter struct {
	column string
	mode   query.PolicyMode
}

func requiredFilterColumns(table manifest.Table) []requiredFilter {
	seen := map[string]requiredFilter{}
	for _, column := range table.Columns {
		if column.RequiredFilter || column.TenantScope {
			seen[normalizeName(column.Name)] = requiredFilter{column: column.Name, mode: query.PolicyModeEnforce}
		}
	}
	for _, policy := range table.Policies {
		switch policy.Type {
		case "tenant_scope", "required_filter":
			mode := policy.Mode
			if mode == "" {
				mode = query.PolicyModeEnforce
			}
			seen[normalizeName(policy.Column)] = requiredFilter{column: policy.Column, mode: mode}
		}
	}
	out := make([]requiredFilter, 0, len(seen))
	for _, required := range seen {
		out = append(out, required)
	}
	return out
}

func tableSoftDeleteColumn(table manifest.Table) string {
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

func specHasFilter(spec OperationSpec, field string) bool {
	target := normalizeName(field)
	for _, filter := range spec.Filters {
		if normalizeName(filter.Field) == target {
			return true
		}
	}
	return false
}

func supportedFilterOp(op string) bool {
	switch normalizeFilterOp(op) {
	case "=", "!=", "<>", ">", ">=", "<", "<=", "like", "in", "is_null", "is_not_null":
		return true
	default:
		return false
	}
}

func normalizeFilterOp(op string) string {
	op = strings.ToLower(strings.TrimSpace(op))
	switch op {
	case "":
		return "="
	case "eq":
		return "="
	case "ne":
		return "!="
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	default:
		return op
	}
}

func normalizeDirection(direction string) string {
	direction = strings.ToLower(strings.TrimSpace(direction))
	if direction == "" {
		return "asc"
	}
	return direction
}

func accessReason(spec OperationSpec, opts Options) string {
	if strings.TrimSpace(spec.AccessReason) != "" {
		return strings.TrimSpace(spec.AccessReason)
	}
	return strings.TrimSpace(opts.AccessReason)
}

func dialectFromManifest(m *manifest.Manifest) driver.Dialect {
	if m != nil && strings.Contains(strings.ToLower(m.Dialect), "postgres") {
		return driver.PostgresDialect{}
	}
	return driver.MySQLDialect{}
}

func warning(code string, level query.RiskLevel, message, hint string) query.Warning {
	return query.Warning{
		Code:         code,
		Level:        level,
		Message:      message,
		Hint:         hint,
		Suppressible: true,
	}
}

func mergeWarningsIntoPlan(plan *query.QueryPlan, warnings []query.Warning) {
	if plan == nil || len(warnings) == 0 {
		return
	}
	existing := map[string]struct{}{}
	for _, w := range plan.Warnings {
		existing[w.Code] = struct{}{}
	}
	for _, w := range warnings {
		if _, ok := existing[w.Code]; ok {
			continue
		}
		plan.Warnings = append(plan.Warnings, w)
		existing[w.Code] = struct{}{}
	}
	plan.RiskLevel, plan.Blocked = aggregateWarnings(plan.Warnings)
	plan.RequiredApproval = requiresApproval(plan.RiskLevel)
}

func aggregateWarnings(warnings []query.Warning) (query.RiskLevel, bool) {
	level := query.RiskLow
	blocked := false
	for _, w := range warnings {
		if compareRisk(w.Level, level) > 0 {
			level = w.Level
		}
		if w.Level == query.RiskBlocked {
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

func normalizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"")
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.ToLower(s)
}
