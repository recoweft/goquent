package operation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/internal/planversion"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/query"
)

// UpdateState describes presence independently of the value. A zero assignment
// is deliberately invalid; generated zero patches emit no assignments.
type UpdateState string

const (
	UpdateUnchanged UpdateState = "unchanged"
	UpdateNull      UpdateState = "null"
	UpdateValue     UpdateState = "value"
)

var (
	ErrInvalidAssignment = errors.New("goquent operation: invalid update assignment")
	ErrEmptyPatch        = errors.New("goquent operation: empty update patch")
)

// UpdateAssignment is sensitive source data, not a display or execution permit.
type UpdateAssignment struct {
	Column       string      `json:"column"`
	State        UpdateState `json:"state"`
	Value        any         `json:"value,omitempty"`
	ValuePresent bool        `json:"-"`
}

func (a UpdateAssignment) hasValue() bool { return a.ValuePresent || a.Value != nil }
func (a *UpdateAssignment) UnmarshalJSON(b []byte) error {
	raw, err := inputjson.Object(b, map[string]bool{"column": true, "state": true, "value": true})
	if err != nil {
		return ErrInvalidAssignment
	}
	type plain UpdateAssignment
	var next plain
	if planversion.Decode(b, &next) != nil {
		return ErrInvalidAssignment
	}
	_, next.ValuePresent = raw["value"]
	*a = UpdateAssignment(next)
	return nil
}
func (a UpdateAssignment) MarshalJSON() ([]byte, error) {
	m := map[string]any{"column": a.Column, "state": a.State}
	if a.hasValue() {
		m["value"] = a.Value
	}
	return json.Marshal(m)
}

// UpdateSpec is a separate application-only source contract. OperationSpec,
// CLI and MCP remain SELECT-only. Compilation never executes a database call.
type UpdateSpec struct {
	Version      int                `json:"version"`
	Model        string             `json:"model"`
	Filters      []FilterSpec       `json:"filters,omitempty"`
	Assignments  []UpdateAssignment `json:"assignments"`
	Returning    []string           `json:"returning,omitempty"`
	AccessReason string             `json:"access_reason,omitempty"`
	sourceBytes  int
}

func (s *UpdateSpec) UnmarshalJSON(b []byte) error {
	if len(b) > inputjson.MaxBytes || inputjson.CheckJSON(b) != nil {
		return ErrInputLimit
	}
	if _, err := inputjson.Object(b, map[string]bool{"version": true, "model": true, "filters": true, "assignments": true, "returning": true, "access_reason": true}); err != nil {
		return ErrInvalidAssignment
	}
	type plain UpdateSpec
	var next plain
	if planversion.Decode(b, &next) != nil || next.Version != 1 {
		return ErrInvalidAssignment
	}
	*s = UpdateSpec(next)
	s.sourceBytes = len(b)
	return nil
}

func updateBudget(s UpdateSpec, values map[string]any) error {
	if len(s.Assignments)+len(s.Filters)+len(s.Returning)+len(values) > inputjson.MaxNodes {
		return ErrInputLimit
	}
	f := make([]any, len(s.Filters))
	for i, v := range s.Filters {
		m := map[string]any{"field": v.Field, "op": v.Op}
		if v.hasValue() {
			m["value"] = v.Value
		}
		if v.hasRef() {
			m["value_ref"] = v.ValueRef
		}
		f[i] = m
	}
	a := make([]any, len(s.Assignments))
	for i, v := range s.Assignments {
		m := map[string]any{"column": v.Column, "state": string(v.State)}
		if v.hasValue() {
			m["value"] = v.Value
		}
		a[i] = m
	}
	m := map[string]any{"version": s.Version, "model": s.Model, "filters": f, "assignments": a, "returning": s.Returning, "access_reason": s.AccessReason}
	n, err := inputjson.Size(map[string]any{"spec": m, "values": values})
	if err != nil {
		return ErrInputLimit
	}
	own, _ := inputjson.Size(m)
	if s.sourceBytes > own && s.sourceBytes-own > inputjson.MaxBytes-n {
		return ErrInputLimit
	}
	return nil
}

// CompileUpdateWithDiagnostics validates and plans one update without execution.
func CompileUpdateWithDiagnostics(ctx context.Context, s UpdateSpec, opts Options) (*query.QueryPlan, DiagnosticView, error) {
	r := newRecorder(OperationSpec{sourceBytes: s.sourceBytes})
	p, err := prepareUpdate(ctx, s, opts, r, nil, nil)
	return p, r.view(p, err != nil), err
}

func prepareUpdate(ctx context.Context, s UpdateSpec, opts Options, d *diagnosticRecorder, executor querybridge.Executor, out *querybridge.Planned) (plan *query.QueryPlan, failure error) {
	defer func() {
		if failure != nil {
			d.refusal()
			failure = &validationFailure{cause: failure, diagnostics: append([]querybridge.OperationDiagnostic(nil), d.records...)}
		}
	}()
	d.at("spec", "root", "", -1, "")
	d.expect("INPUT_LIMIT")
	if err := updateBudget(s, opts.Values); err != nil {
		return nil, err
	}
	d.at("spec", "version", "", -1, "")
	d.expect("VERSION_UNSUPPORTED")
	if s.Version != 1 {
		return nil, ErrInvalidAssignment
	}
	opts = operationSettings(opts)
	read := OperationSpec{Version: 1, Operation: OperationSelect, Model: s.Model, Select: s.Returning, Filters: s.Filters, AccessReason: s.AccessReason}
	r, err := validateOperation(read, opts, d, true)
	if err != nil {
		return nil, err
	}
	// Validation above shares read declaration checks, not a SELECT plan. Retain
	// the original RETURNING positions in both internal and public diagnostics.
	for i := range d.records {
		if d.records[i].Section == "select" {
			d.records[i].Section = "returning"
		}
	}
	check := columnChecker(opts, &r, d)
	values := map[string]any{}
	seen := map[string]bool{}
	policy, _ := opts.Settings.PolicySet().PolicyForTable(r.table.Name)
	for i, a := range s.Assignments {
		d.at("spec", "assignments", "column", i, a.Column)
		d.target(r.table.Name, a.Column)
		d.expect("ASSIGNMENT_INVALID")
		name := normalizeName(a.Column)
		if seen[name] {
			return nil, ErrInvalidAssignment
		}
		seen[name] = true
		c, err := diagnosticField(r.columns, a.Column, d)
		if err != nil {
			return nil, err
		}
		// Assignments use literal unqualified identifiers, never paths/expressions.
		if a.Column != c.Name || strings.ContainsAny(c.Name, ".->") {
			return nil, ErrInvalidAssignment
		}
		d.expect("FIELD_FORBIDDEN")
		if !manifest.RepositoryUpdateCandidate(r.table, c) || c.Name == policy.TenantColumn || c.Name == policy.SoftDeleteColumn {
			return nil, ErrForbiddenField
		}
		protected := append(append(append([]string(nil), policy.ForbiddenColumns...), policy.PIIColumns...), policy.ImmutableColumns...)
		for _, name := range protected {
			if normalizeName(name) == normalizeName(c.Name) {
				return nil, ErrForbiddenField
			}
		}
		d.current.Member = "state"
		d.expect("ASSIGNMENT_INVALID")
		if a.State != UpdateUnchanged && a.State != UpdateNull && a.State != UpdateValue {
			return nil, ErrInvalidAssignment
		}
		if a.State == UpdateValue {
			if !a.hasValue() || a.Value == nil {
				return nil, ErrInvalidAssignment
			}
		} else if a.hasValue() {
			return nil, ErrInvalidAssignment
		}
		// Even Unchanged records must satisfy target/declaration/presence checks.
		if a.State == UpdateNull && (!c.NullableKnown || !c.Nullable || c.TypeSource != "sql") {
			d.expect("TYPE_MISMATCH")
			return nil, ErrTypeMismatch
		}
		declaration, err := parseColumnType(c, knownDialect(opts))
		if err != nil {
			return nil, err
		}
		if declaration.array {
			d.expect("ARRAY_UNSUPPORTED")
			return nil, ErrArrayBinding
		}
		op := ""
		if a.State == UpdateValue {
			op = "="
		}
		d.current.Member = "value"
		if err := check(c.Name, i, op, a.Value, a.hasValue(), false, "assignment"); err != nil {
			return nil, err
		}
		if !c.NullableKnown {
			d.partial = true
			d.expect("TYPE_UNVERIFIED")
			d.add("OPERATION_TYPE_UNVERIFIED", "unverified", "", "nullability_unknown", "supply_supported_explicit_declarations")
			if opts.Settings.IsStrict() {
				return nil, ErrTypeUnverified
			}
			r.typed.Unknown = append(r.typed.Unknown, "nullability_unknown")
		}
		if a.State == UpdateNull {
			d.add("OPERATION_CHECK_NULLABILITY", "checked", "nullable_assignment", "", "")
			values[c.Name] = nil
		} else if a.State == UpdateValue {
			values[c.Name] = a.Value
		}
	}
	d.at("spec", "assignments", "", -1, "")
	d.expect("PATCH_EMPTY")
	if len(values) == 0 {
		return nil, ErrEmptyPatch
	}
	d.at("planner", "root", "", -1, "")
	d.expect("PLANNER_REFUSED")
	dialect := opts.Dialect
	if dialect == nil {
		dialect = dialectFromManifest(opts.Manifest)
	}
	q := query.NewWithSettings(executor, r.table.Name, dialect, *opts.Settings).WithContext(ctx)
	for _, w := range r.warnings {
		r.typed.WarningCodes = append(r.typed.WarningCodes, w.Code)
	}
	d.add("OPERATION_UNVERIFIED_LIVE", "unverified", "", "live_database_session_collation_physical_identity", "verify_live_conditions_separately")
	r.typed.Diagnostics = append([]querybridge.OperationDiagnostic(nil), d.records...)
	if err := querybridge.ConfigureOperation(q, r.typed); err != nil {
		return nil, err
	}
	if reason := accessReason(read, opts); reason != "" {
		q.AccessReason(reason)
	}
	if soft := tableSoftDeleteColumn(r.table); soft != "" && !specHasFilter(read, soft) {
		q.WhereNull(soft)
	}
	for _, f := range r.filters {
		if normalizeFilterOp(f.Op) == "=" && !strings.Contains(f.Field, ".") {
			if err := querybridge.OperationEquality(q, f.Field, f.Value); err != nil {
				return nil, err
			}
			continue
		}
		if err := applyFilter(q, f); err != nil {
			return nil, err
		}
	}
	prepared, err := querybridge.PlanOperationUpdate(ctx, q, values, s.Returning)
	if err != nil {
		return nil, err
	}
	plan = prepared.Diagnostic.(*query.QueryPlan)
	if opts.Settings.IsStrict() && plan.Blocked {
		return nil, query.ErrBlockedOperation
	}
	known := map[string]bool{}
	for _, record := range d.records {
		if record.Status == "warning" {
			known[record.Code] = true
		}
	}
	for _, w := range plan.Warnings {
		if !known[w.Code] {
			d.warning(w.Code)
		}
	}
	if out != nil {
		*out = prepared
	}
	return plan, nil
}

func init() {
	querybridge.PrepareUpdate = func(ctx context.Context, source, options any, executor querybridge.Executor) (querybridge.Planned, error) {
		s, ok := source.(UpdateSpec)
		opts, valid := options.(Options)
		if !ok || !valid {
			return querybridge.Planned{}, ErrInvalidAssignment
		}
		var p querybridge.Planned
		_, err := prepareUpdate(ctx, s, opts, newRecorder(OperationSpec{sourceBytes: s.sourceBytes}), executor, &p)
		return p, err
	}
}
