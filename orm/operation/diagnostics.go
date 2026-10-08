package operation

import (
	"context"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/query"
)

type diagnosticRecorder struct {
	records      []querybridge.OperationDiagnostic
	current      querybridge.OperationDiagnostic
	origin       string
	partial      bool
	typedReached bool
	returning    bool
}

func newRecorder(spec OperationSpec) *diagnosticRecorder {
	origin := "go"
	if spec.sourceBytes > 0 {
		origin = "json"
	}
	if spec.inputOrigin != "" {
		origin = spec.inputOrigin
	}
	return &diagnosticRecorder{origin: origin}
}
func (r *diagnosticRecorder) at(source, section, member string, index int, field string) {
	// Shared SELECT validation retains the logical UpdateSpec source position.
	if r.returning && source == "spec" && section == "select" {
		section = "returning"
	}
	r.current = querybridge.OperationDiagnostic{Source: source, Section: section, Member: member, Origin: r.origin, IndexKnown: index >= 0, Index: max(index, 0), Field: field}
	if source != "spec" {
		r.current.Origin = "unknown"
	}
	if section == "implicit" {
		r.current.Origin = "implicit"
	}
}
func (r *diagnosticRecorder) expect(code string) { r.current.Code = "OPERATION_" + code }
func (r *diagnosticRecorder) add(code, status, checked, missing, action string) {
	d := r.current
	d.Code = code
	d.Status = status
	d.Checked = checked
	d.Missing = missing
	d.Action = action
	if d.Evidence == "" {
		d.Evidence = "compiler_structural_check"
	}
	r.records = append(r.records, d)
}
func (r *diagnosticRecorder) refusal() {
	code := r.current.Code
	if code == "" {
		code = "OPERATION_INPUT_INVALID"
	}
	r.add(code, "refused", r.current.Checked, r.current.Missing, repairAction(code))
}
func repairAction(code string) string {
	switch code {
	case "OPERATION_TYPE_MISMATCH":
		return "supply_value_matching_declaration"
	case "OPERATION_TYPE_UNVERIFIED", "OPERATION_MANIFEST_INVALID", "OPERATION_MANIFEST_REQUIRED":
		return "supply_supported_explicit_declarations"
	case "OPERATION_RESERVED_BINDING":
		return "use_trusted_application_context"
	case "OPERATION_VALUE_REF_MISSING":
		return "supply_referenced_value"
	case "OPERATION_ARRAY_UNSUPPORTED":
		return "use_supported_scalar_operation"
	case "OPERATION_REQUIRED_FILTER_MISSING":
		return "add_required_predicate"
	case "OPERATION_FIELD_UNKNOWN", "OPERATION_FIELD_FORBIDDEN", "OPERATION_MODEL_UNKNOWN":
		return "use_declared_allowed_target"
	case "OPERATION_PLANNER_REFUSED", "OPERATION_SETTINGS_INVALID":
		return "review_application_settings_and_plan_gates"
	case "OPERATION_ACCESS_REASON_REQUIRED":
		return "supply_application_access_reason"
	case "OPERATION_MANIFEST_STALE":
		return "verify_supplied_manifest"
	default:
		return "correct_input_structure"
	}
}
func (r *diagnosticRecorder) warning(code string) {
	r.add(code, "warning", "", "", "review_fixed_warning_catalog")
}

// CompileWithDiagnostics uses the same single validation/planning pass as Compile.
// The returned view is detached; the error and original plan retain their meaning.
func CompileWithDiagnostics(ctx context.Context, spec OperationSpec, opts Options) (*query.QueryPlan, DiagnosticView, error) {
	r := newRecorder(spec)
	plan, err := compileOperation(ctx, spec, opts, r)
	return plan, r.view(plan, err != nil), err
}

// ValidateWithDiagnostics shares CompileWithDiagnostics, including planning gates.
func ValidateWithDiagnostics(spec OperationSpec, opts Options) ([]query.Warning, DiagnosticView, error) {
	plan, view, err := CompileWithDiagnostics(context.Background(), spec, opts)
	if err != nil {
		return nil, view, err
	}
	return plan.Warnings, view, nil
}
func (r *diagnosticRecorder) view(plan *query.QueryPlan, rejected bool) DiagnosticView {
	v := DiagnosticView{Kind: DiagnosticViewKind, Version: 1, Outcome: "rejected", Coverage: "unavailable", DetailsOmitted: true, DiagnosticCount: len(r.records)}
	if r.typedReached {
		v.Coverage = "partial"
	}
	if !rejected {
		v.Outcome = "compiled"
		v.Coverage = "checked_subset"
		if r.partial {
			v.Coverage = "partial"
		}
		if plan != nil {
			p, e := plan.PublicView()
			if e == nil {
				v.Plan = &p
			}
		}
	}
	// A refusal is retained even when it occurs after the display budget.
	appendEntry := func(d querybridge.OperationDiagnostic) {
		if len(v.Diagnostics) >= MaxDiagnostics {
			return
		}
		v.Diagnostics = append(v.Diagnostics, DiagnosticEntry{Code: d.Code, Status: d.Status, Location: DiagnosticLocation{Source: d.Source, Section: d.Section, Member: d.Member, Origin: d.Origin, IndexKnown: d.IndexKnown, Index: d.Index, ElementKnown: d.ElementKnown, Element: d.Element}})
	}
	for _, d := range r.records {
		if d.Status == "refused" {
			appendEntry(d)
		}
	}
	for _, d := range r.records {
		if d.Status != "refused" {
			appendEntry(d)
		}
	}
	out, _ := v.safe()
	return out
}
func (r *diagnosticRecorder) target(table, field string) {
	r.current.Table = table
	r.current.Field = field
	if dot := strings.LastIndex(field, "."); dot >= 0 {
		r.current.Qualifier = field[:dot]
	}
}

// Only in-repository adapters can label reconstructed input acquisition.
// This label never affects validation, budget accounting or trusted provenance.
func init() {
	querybridge.OperationInputOrigin = func(v any, origin string) {
		if s, ok := v.(*OperationSpec); ok && s != nil {
			s.inputOrigin = vocabulary(origin, "json", "go")
		}
	}
}
