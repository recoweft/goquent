package review

import (
	"encoding/json"
	"fmt"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/internal/publicview"
	"github.com/recoweft/goquent/orm/query"
)

// DetailView contains only closed classifications. No source evidence is copied.
type DetailView struct {
	Coverage string   `json:"coverage"`
	SQL      string   `json:"sql"`
	Columns  string   `json:"columns"`
	Bindings string   `json:"bindings"`
	Policy   string   `json:"policy"`
	Reasons  []string `json:"reasons"`
	Missing  []string `json:"missing"`
}

func detailView(d analysis.Detail) DetailView {
	d = d.Normalize()
	return DetailView{Coverage: string(d.Coverage()), SQL: string(d.SQL), Columns: string(d.Columns), Bindings: string(d.Bindings), Policy: string(d.Policy), Reasons: d.Reasons, Missing: d.Missing}
}
func (d DetailView) safe() DetailView {
	return detailView(analysis.Detail{SQL: analysis.State(d.SQL), Columns: analysis.State(d.Columns), Bindings: analysis.State(d.Bindings), Policy: analysis.State(d.Policy), Reasons: d.Reasons, Missing: d.Missing})
}

type OperationView struct {
	Suppressed     bool                    `json:"suppressed"`
	Ordinal        int                     `json:"ordinal"`
	Precision      string                  `json:"precision"`
	Detail         DetailView              `json:"detail"`
	Correspondence analysis.Correspondence `json:"correspondence"`
}

// ReportViewV2 is an opt-in display contract. V1 defaults and readers are unchanged.
type ReportViewV2 struct {
	Kind                   string          `json:"kind"`
	Version                int             `json:"version"`
	FindingCount           int             `json:"finding_count"`
	SuppressedFindingCount int             `json:"suppressed_finding_count"`
	OperationCount         int             `json:"operation_count"`
	Findings               []FindingView   `json:"findings"`
	SuppressedFindings     []FindingView   `json:"suppressed_findings"`
	Operations             []OperationView `json:"operations"`
	Truncated              bool            `json:"truncated"`
	DetailsOmitted         bool            `json:"details_omitted"`
}

func (r ReviewReport) PublicViewV2() (ReportViewV2, error) {
	if r.privateOperations != nil {
		r.Operations = r.privateOperations
	}
	v1, err := r.PublicView()
	if err != nil {
		return ReportViewV2{}, err
	}
	v := ReportViewV2{Kind: ReportViewKind, Version: 2, FindingCount: v1.FindingCount, SuppressedFindingCount: v1.SuppressedFindingCount, OperationCount: max(r.OperationCount, len(r.Operations)), Findings: v1.Findings, SuppressedFindings: v1.SuppressedFindings, Operations: []OperationView{}, Truncated: v1.Truncated, DetailsOmitted: true}
	for i, op := range r.Operations {
		if i == MaxReportViewFindings {
			break
		}
		v.Operations = append(v.Operations, OperationView{Suppressed: op.Suppressed, Ordinal: i + 1, Precision: op.Detail.Precision(), Detail: detailView(op.Detail), Correspondence: op.Correspondence.Normalize()})
	}
	return v.safe(), nil
}
func (v ReportViewV2) safe() ReportViewV2 {
	v.FindingCount = max(v.FindingCount, len(v.Findings))
	v.SuppressedFindingCount = max(v.SuppressedFindingCount, len(v.SuppressedFindings))
	v.Findings = safeFindings(v.Findings)
	v.SuppressedFindings = safeFindings(v.SuppressedFindings)
	v.FindingCount = max(v.FindingCount, len(v.Findings))
	v.SuppressedFindingCount = max(v.SuppressedFindingCount, len(v.SuppressedFindings))
	v.OperationCount = max(v.OperationCount, len(v.Operations))
	ops := make([]OperationView, min(len(v.Operations), MaxReportViewFindings))
	for i := range ops {
		o := v.Operations[i]
		o.Ordinal = i + 1
		o.Precision = publicview.Precision(o.Precision)
		o.Detail = o.Detail.safe()
		o.Correspondence = o.Correspondence.Normalize()
		ops[i] = o
	}
	v.Operations = ops
	v.Truncated = v.Truncated || v.OperationCount > len(ops) || v.FindingCount > len(v.Findings) || v.SuppressedFindingCount > len(v.SuppressedFindings)
	v.DetailsOmitted = true
	return v
}
func (v ReportViewV2) MarshalJSON() ([]byte, error) {
	if v.Kind != ReportViewKind || v.Version != 2 {
		return nil, publicview.ErrInvalid
	}
	type wire ReportViewV2
	return json.Marshal(wire(v.safe()))
}
func (v ReportViewV2) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v ReportViewV2) String() string {
	b, e := v.ToJSON()
	if e != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v ReportViewV2) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func DecodeReportViewV2(b []byte) (ReportViewV2, error) {
	type wire ReportViewV2
	var v wire
	if publicview.DecodeVersion(b, ReportViewKind, 2, &v) != nil {
		return ReportViewV2{}, publicview.ErrInvalid
	}
	return ReportViewV2(v).safe(), nil
}
func (v *ReportViewV2) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = ReportViewV2{}
	out, err := DecodeReportViewV2(b)
	if err == nil {
		*v = out
	}
	return err
}

// ReviewSQL reports structural raw-SQL findings without execution or evidence.
// This source API is sensitive; publication must use PublicView or PublicViewV2.
func ReviewSQL(sql string) ReviewReport {
	p := query.NewRawPlan(sql)
	d := analysis.Detail{SQL: analysis.Partial, Reasons: []string{"unverified_input"}, Missing: []string{"runtime_policy", "evidence_missing", "database_specific"}}.Normalize()
	r := ReviewReport{Findings: findingsFromPlan(p, query.AnalysisPartial, nil), Operations: []OperationReview{{Detail: d, Correspondence: analysis.Correspondence{}.Normalize()}}}
	for i := range r.Findings {
		r.Findings[i].Detail = d
	}
	r.Summary = summarize(r)
	return r
}
