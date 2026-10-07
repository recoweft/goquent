package query

import (
	"encoding/json"
	"fmt"

	"github.com/recoweft/goquent/orm/internal/publicview"
)

const PlanViewKind = "goquent.plan_view"
const PlanViewVersion = 1

// MaxPlanViewWarnings applies independently to active and suppressed warnings.
const MaxPlanViewWarnings = 128

// PlanView is detached, non-executable display data. SQL and all names/values
// are omitted. Its claims are not authorization or a complete inspection.
type PlanView struct {
	Kind                   string        `json:"kind"`
	Version                int           `json:"version"`
	Operation              string        `json:"operation"`
	Risk                   string        `json:"risk"`
	Precision              string        `json:"precision"`
	RequiredApproval       bool          `json:"required_approval"`
	Blocked                bool          `json:"blocked"`
	DetailsOmitted         bool          `json:"details_omitted"`
	TableCount             int           `json:"table_count"`
	ColumnCount            int           `json:"column_count"`
	JoinCount              int           `json:"join_count"`
	PredicateCount         int           `json:"predicate_count"`
	WarningCount           int           `json:"warning_count"`
	SuppressedWarningCount int           `json:"suppressed_warning_count"`
	Warnings               []WarningView `json:"warnings"`
	SuppressedWarnings     []WarningView `json:"suppressed_warnings"`
	Truncated              bool          `json:"truncated"`
}

// WarningView contains only fixed vocabulary, local ordinal and source position.
// Message is regenerated from Code by every provided display method.
type WarningView struct {
	Ordinal        int    `json:"ordinal"`
	Code           string `json:"code"`
	Level          string `json:"level"`
	Message        string `json:"message"`
	Line           int    `json:"line"`
	Column         int    `json:"column"`
	Suppressible   bool   `json:"suppressible"`
	RequiresReason bool   `json:"requires_reason"`
}

// PublicView does not walk payloads, copy private evidence, or call value methods.
func (p *QueryPlan) PublicView() (PlanView, error) {
	if p == nil || (p.Version != 0 && p.Version != 1) {
		return PlanView{}, publicview.ErrSource
	}
	v := PlanView{Kind: PlanViewKind, Version: PlanViewVersion,
		Operation: string(p.Operation), Risk: string(p.RiskLevel), Precision: string(p.AnalysisPrecision),
		RequiredApproval: p.RequiredApproval, Blocked: p.Blocked, DetailsOmitted: true,
		TableCount: len(p.Tables), ColumnCount: len(p.Columns), JoinCount: len(p.Joins), PredicateCount: len(p.Predicates),
		WarningCount: len(p.Warnings), SuppressedWarningCount: len(p.SuppressedWarnings),
		Warnings: projectWarnings(p.Warnings), SuppressedWarnings: projectWarnings(p.SuppressedWarnings),
		Truncated: len(p.Warnings) > MaxPlanViewWarnings || len(p.SuppressedWarnings) > MaxPlanViewWarnings}
	return v.safe(), nil
}
func projectWarnings(src []Warning) []WarningView {
	out := make([]WarningView, min(len(src), MaxPlanViewWarnings))
	for i := range out {
		w := src[i]
		out[i] = WarningView{Ordinal: i + 1, Code: w.Code, Level: string(w.Level), Suppressible: w.Suppressible, RequiresReason: w.RequiresReason}
		if w.Location != nil {
			out[i].Line, out[i].Column = w.Location.Line, w.Location.Column
		}
	}
	return out
}
func (v WarningView) safe() WarningView {
	v.Code = publicview.Code(v.Code)
	v.Level = publicview.Risk(v.Level)
	v.Message = publicview.Message(v.Code)
	v.Ordinal = publicview.Nonnegative(v.Ordinal)
	v.Line = publicview.Nonnegative(v.Line)
	v.Column = publicview.Nonnegative(v.Column)
	return v
}
func safeWarnings(src []WarningView) []WarningView {
	out := make([]WarningView, min(len(src), MaxPlanViewWarnings))
	for i := range out {
		out[i] = src[i].safe()
		out[i].Ordinal = i + 1
	}
	return out
}
func (v PlanView) safe() PlanView {
	v.Operation = publicview.Operation(v.Operation)
	v.Risk = publicview.Risk(v.Risk)
	v.Precision = publicview.Precision(v.Precision)
	v.DetailsOmitted = true
	v.TableCount = publicview.Nonnegative(v.TableCount)
	v.ColumnCount = publicview.Nonnegative(v.ColumnCount)
	v.JoinCount = publicview.Nonnegative(v.JoinCount)
	v.PredicateCount = publicview.Nonnegative(v.PredicateCount)
	v.WarningCount = max(v.WarningCount, len(v.Warnings))
	v.SuppressedWarningCount = max(v.SuppressedWarningCount, len(v.SuppressedWarnings))
	v.Warnings, v.SuppressedWarnings = safeWarnings(v.Warnings), safeWarnings(v.SuppressedWarnings)
	v.Truncated = v.Truncated || v.WarningCount > len(v.Warnings) || v.SuppressedWarningCount > len(v.SuppressedWarnings)
	return v
}

// MarshalJSON reapplies policy even to caller-modified exported fields.
func (v PlanView) MarshalJSON() ([]byte, error) {
	if v.Kind != PlanViewKind || v.Version != PlanViewVersion {
		return nil, publicview.ErrInvalid
	}
	type wire PlanView
	b, err := json.Marshal(wire(v.safe()))
	if err != nil {
		return nil, publicview.ErrInvalid
	}
	return b, nil
}

// ToJSON is the safe JSON entry, including its returned errors.
func (v PlanView) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v PlanView) String() string {
	b, err := v.ToJSON()
	if err != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}

// Format covers all fmt verbs, including %#v, for values and pointers.
func (v PlanView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

// DecodePlanView reads one bounded document into a fresh, display-only value.
func DecodePlanView(b []byte) (PlanView, error) {
	type wire PlanView
	var v wire
	if publicview.Decode(b, PlanViewKind, &v) != nil {
		return PlanView{}, publicview.ErrInvalid
	}
	return PlanView(v).safe(), nil
}
func (v *PlanView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = PlanView{}
	out, err := DecodePlanView(b)
	if err == nil {
		*v = out
	}
	return err
}
func (v WarningView) MarshalJSON() ([]byte, error) {
	type wire WarningView
	return json.Marshal(wire(v.safe()))
}
func (v WarningView) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v WarningView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
