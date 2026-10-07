package migration

import (
	"encoding/json"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/publicview"
	"github.com/recoweft/goquent/orm/publicoutput"
	"github.com/recoweft/goquent/orm/query"
	"io"
)

const MigrationPlanViewKind = "goquent.migration_plan_view"

// MigrationPlanView is detached display data, never an apply or approval input.
type MigrationPlanView struct {
	Kind             string              `json:"kind"`
	Version          int                 `json:"version"`
	Risk             string              `json:"risk"`
	Precision        string              `json:"precision"`
	RequiredApproval bool                `json:"required_approval"`
	Blocked          bool                `json:"blocked"`
	StepCount        int                 `json:"step_count"`
	WarningCount     int                 `json:"warning_count"`
	Steps            []StepView          `json:"steps"`
	Warnings         []query.WarningView `json:"warnings"`
	Truncated        bool                `json:"truncated"`
	DetailsOmitted   bool                `json:"details_omitted"`
}

// StepView omits all executable statements, identifiers, preflight and defaults.
type StepView struct {
	Ordinal      int                 `json:"ordinal"`
	Type         string              `json:"type"`
	Risk         string              `json:"risk"`
	Precision    string              `json:"precision"`
	Line         int                 `json:"line"`
	WarningCount int                 `json:"warning_count"`
	Warnings     []query.WarningView `json:"warnings"`
	Truncated    bool                `json:"truncated"`
}

func stepType(s string) string {
	switch MigrationStepType(s) {
	case AddTable, DropTable, AddColumn, DropColumn, RenameColumn, AlterColumnType, AlterNullability, AddIndex, DropIndex, UnsupportedStep:
		return s
	}
	return "unknown"
}
func warnings(src []query.Warning) []query.WarningView {
	out := make([]query.WarningView, min(128, len(src)))
	for i, w := range src[:len(out)] {
		out[i] = query.WarningView{Ordinal: i + 1, Code: publicview.Code(w.Code), Level: publicview.Risk(string(w.Level)), Suppressible: w.Suppressible, RequiresReason: w.RequiresReason}
		out[i].Message = publicview.Message(out[i].Code)
		if w.Location != nil {
			out[i].Line = publicview.Nonnegative(w.Location.Line)
			out[i].Column = publicview.Nonnegative(w.Location.Column)
		}
	}
	return out
}
func (p *MigrationPlan) PublicView() (MigrationPlanView, error) {
	if p == nil {
		return MigrationPlanView{}, publicview.ErrSource
	}
	v := MigrationPlanView{Kind: MigrationPlanViewKind, Version: 1, Risk: string(p.RiskLevel), Precision: string(p.AnalysisPrecision), RequiredApproval: p.RequiredApproval, Blocked: p.Blocked, StepCount: len(p.Steps), WarningCount: len(p.Warnings), Warnings: warnings(p.Warnings), Truncated: len(p.Steps) > 128 || len(p.Warnings) > 128}
	// Reserve root and every visible step before allocating warning lists.
	n := min(128, len(p.Steps))
	budget := 1024 - 1 - n - len(v.Warnings)
	for i, s := range p.Steps[:n] {
		w := warnings(s.Warnings[:min(len(s.Warnings), budget)])
		budget -= len(w)
		v.Steps = append(v.Steps, StepView{Ordinal: i + 1, Type: string(s.Type), Risk: string(s.RiskLevel), Precision: string(s.AnalysisPrecision), Line: s.Line, WarningCount: len(s.Warnings), Warnings: w, Truncated: len(w) < len(s.Warnings)})
	}
	return v.safe()
}
func boundWarnings(src []query.WarningView, budget *int) []query.WarningView {
	n := min(len(src), 128, *budget)
	*budget -= n
	out := make([]query.WarningView, n)
	for i := range out {
		w := src[i]
		w.Ordinal = i + 1
		w.Code = publicview.Code(w.Code)
		w.Level = publicview.Risk(w.Level)
		w.Message = publicview.Message(w.Code)
		w.Line = publicview.Nonnegative(w.Line)
		w.Column = publicview.Nonnegative(w.Column)
		out[i] = w
	}
	return out
}
func (v StepView) safe(budget *int) StepView {
	v.Type = stepType(v.Type)
	v.Risk = publicview.Risk(v.Risk)
	v.Precision = publicview.Precision(v.Precision)
	v.Ordinal = publicview.Nonnegative(v.Ordinal)
	v.Line = publicview.Nonnegative(v.Line)
	v.WarningCount = max(publicview.Nonnegative(v.WarningCount), len(v.Warnings))
	v.Warnings = boundWarnings(v.Warnings, budget)
	v.Truncated = v.Truncated || v.WarningCount > len(v.Warnings)
	return v
}
func (v MigrationPlanView) safe() (MigrationPlanView, error) {
	if v.Kind != MigrationPlanViewKind || v.Version != 1 {
		return MigrationPlanView{}, publicview.ErrInvalid
	}
	v.Risk = publicview.Risk(v.Risk)
	v.Precision = publicview.Precision(v.Precision)
	v.DetailsOmitted = true
	v.StepCount = max(publicview.Nonnegative(v.StepCount), len(v.Steps))
	v.WarningCount = max(publicview.Nonnegative(v.WarningCount), len(v.Warnings))
	n := min(128, len(v.Steps))
	budget := 1024 - 1 - n
	v.Warnings = boundWarnings(v.Warnings, &budget)
	steps := make([]StepView, n)
	for i := range steps {
		steps[i] = v.Steps[i].safe(&budget)
		steps[i].Ordinal = i + 1
		v.Truncated = v.Truncated || steps[i].Truncated
	}
	v.Steps = steps
	v.Truncated = v.Truncated || v.StepCount > n || v.WarningCount > len(v.Warnings)
	return v, nil
}
func (v MigrationPlanView) MarshalJSON() ([]byte, error) {
	v, err := v.safe()
	if err != nil {
		return nil, err
	}
	type wire MigrationPlanView
	return json.Marshal(wire(v))
}
func (v MigrationPlanView) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v MigrationPlanView) String() string {
	b, err := v.ToJSON()
	if err != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v MigrationPlanView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func (v StepView) MarshalJSON() ([]byte, error) {
	budget := 128
	type wire StepView
	return json.Marshal(wire(v.safe(&budget)))
}
func (v StepView) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v StepView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func DecodeMigrationPlanView(b []byte) (MigrationPlanView, error) {
	type wire MigrationPlanView
	var v wire
	if publicview.Decode(b, MigrationPlanViewKind, &v) != nil {
		return MigrationPlanView{}, publicview.ErrInvalid
	}
	return MigrationPlanView(v).safe()
}
func (v *MigrationPlanView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = MigrationPlanView{}
	out, err := DecodeMigrationPlanView(b)
	if err == nil {
		*v = out
	}
	return err
}
func WritePublicJSON(w io.Writer, v MigrationPlanView) error {
	b, err := v.ToJSON()
	if err != nil {
		return err
	}
	return publicview.Write(w, append(b, '\n'))
}

func (s Schema) PublicView() publicoutput.SummaryView {
	v := publicoutput.SummaryView{Kind: "goquent.schema_view", Version: 1, Present: true, Count: len(s.Tables), Truncated: len(s.Tables) > 128, DetailsOmitted: true}
	for _, t := range s.Tables[:min(128, len(s.Tables))] {
		v.Items = append(v.Items, publicoutput.ItemView{Ordinal: len(v.Items) + 1, Columns: len(t.Columns), Indexes: len(t.Indexes)})
	}
	return v
}
func (s Status) PublicView() publicoutput.SummaryView {
	return publicoutput.SummaryView{Kind: "goquent.migration_status_view", Version: 1, Present: true, Known: !s.Unknown, Exists: s.Exists, Dirty: s.Dirty, Unknown: s.Unknown, Count: len(s.Applied), PendingCount: len(s.Pending), WarningCount: len(s.Warnings), DetailsOmitted: true}
}
func (s DriftReport) PublicView() publicoutput.SummaryView {
	return publicoutput.SummaryView{Kind: "goquent.drift_view", Version: 1, Present: true, Drifted: s.Drifted, Count: len(s.Steps), DetailsOmitted: true}
}
