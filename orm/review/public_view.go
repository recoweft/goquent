package review

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/recoweft/goquent/orm/internal/publicview"
)

const ReportViewKind = "goquent.report_view"
const ReportViewVersion = 1

// MaxReportViewFindings applies independently to each finding list.
const MaxReportViewFindings = 256

// ReportView is display-only. Counts refer to each entire source list, which
// can overlap when ShowSuppressed is enabled. Summary maps are not copied.
type ReportView struct {
	Kind                   string        `json:"kind"`
	Version                int           `json:"version"`
	FindingCount           int           `json:"finding_count"`
	SuppressedFindingCount int           `json:"suppressed_finding_count"`
	Findings               []FindingView `json:"findings"`
	SuppressedFindings     []FindingView `json:"suppressed_findings"`
	Truncated              bool          `json:"truncated"`
	DetailsOmitted         bool          `json:"details_omitted"`
}

// FindingView omits paths, reasons, evidence and nested plans completely.
type FindingView struct {
	Ordinal    int    `json:"ordinal"`
	Code       string `json:"code"`
	Level      string `json:"level"`
	Precision  string `json:"precision"`
	Message    string `json:"message"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	Suppressed bool   `json:"suppressed"`
}

// PublicView projects bounded outer structure without visiting opaque contents.
func (r ReviewReport) PublicView() (ReportView, error) {
	if r.Version != 0 && r.Version != 1 {
		return ReportView{}, publicview.ErrSource
	}
	v := ReportView{Kind: ReportViewKind, Version: ReportViewVersion,
		FindingCount: len(r.Findings), SuppressedFindingCount: len(r.SuppressedFindings),
		Findings: projectFindings(r.Findings), SuppressedFindings: projectFindings(r.SuppressedFindings),
		Truncated: len(r.Findings) > MaxReportViewFindings || len(r.SuppressedFindings) > MaxReportViewFindings, DetailsOmitted: true}
	return v.safe(), nil
}
func projectFindings(src []Finding) []FindingView {
	out := make([]FindingView, min(len(src), MaxReportViewFindings))
	for i := range out {
		f := src[i]
		out[i] = FindingView{Ordinal: i + 1, Code: f.Code, Level: string(f.Level), Precision: string(f.AnalysisPrecision), Suppressed: f.Suppressed}
		if f.Location != nil {
			out[i].Line, out[i].Column = f.Location.Line, f.Location.Column
		}
	}
	return out
}
func (v FindingView) safe() FindingView {
	v.Code = publicview.Code(v.Code)
	v.Level = publicview.Risk(v.Level)
	v.Precision = publicview.Precision(v.Precision)
	v.Message = publicview.Message(v.Code)
	v.Ordinal = publicview.Nonnegative(v.Ordinal)
	v.Line = publicview.Nonnegative(v.Line)
	v.Column = publicview.Nonnegative(v.Column)
	return v
}
func safeFindings(src []FindingView) []FindingView {
	out := make([]FindingView, min(len(src), MaxReportViewFindings))
	for i := range out {
		out[i] = src[i].safe()
		out[i].Ordinal = i + 1
	}
	return out
}
func (v ReportView) safe() ReportView {
	v.DetailsOmitted = true
	v.FindingCount = max(v.FindingCount, len(v.Findings))
	v.SuppressedFindingCount = max(v.SuppressedFindingCount, len(v.SuppressedFindings))
	v.Findings, v.SuppressedFindings = safeFindings(v.Findings), safeFindings(v.SuppressedFindings)
	v.Truncated = v.Truncated || v.FindingCount > len(v.Findings) || v.SuppressedFindingCount > len(v.SuppressedFindings)
	return v
}
func (v ReportView) MarshalJSON() ([]byte, error) {
	if v.Kind != ReportViewKind || v.Version != ReportViewVersion {
		return nil, publicview.ErrInvalid
	}
	type wire ReportView
	b, err := json.Marshal(wire(v.safe()))
	if err != nil {
		return nil, publicview.ErrInvalid
	}
	return b, nil
}

// ToJSON returns policy-filtered JSON and only fixed safe errors.
func (v ReportView) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v ReportView) String() string {
	b, err := v.ToJSON()
	if err != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v ReportView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

// DecodeReportView accepts one strict bounded v1 document, never an execution input.
func DecodeReportView(b []byte) (ReportView, error) {
	type wire ReportView
	var v wire
	if publicview.Decode(b, ReportViewKind, &v) != nil {
		return ReportView{}, publicview.ErrInvalid
	}
	return ReportView(v).safe(), nil
}
func (v *ReportView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = ReportView{}
	out, err := DecodeReportView(b)
	if err == nil {
		*v = out
	}
	return err
}
func (v FindingView) MarshalJSON() ([]byte, error) {
	type wire FindingView
	return json.Marshal(wire(v.safe()))
}
func (v FindingView) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v FindingView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

// WritePublicJSON writes only a ReportView. Source reports require PublicView first.
func WritePublicJSON(w io.Writer, v ReportView) error {
	b, err := v.ToJSON()
	if err != nil {
		return publicview.ErrInvalid
	}
	return publicview.Write(w, append(b, '\n'))
}

// WritePublicPretty labels omitted and truncated detail instead of "No findings".
func WritePublicPretty(w io.Writer, v ReportView) error {
	if v.Kind != ReportViewKind || v.Version != ReportViewVersion {
		return publicview.ErrInvalid
	}
	v = v.safe()
	var b strings.Builder
	fmt.Fprintf(&b, "Public database review (details omitted; not authorization)\nFindings: %d; suppressed list: %d; truncated: %t\n", v.FindingCount, v.SuppressedFindingCount, v.Truncated)
	for _, list := range [][]FindingView{v.Findings, v.SuppressedFindings} {
		for _, f := range list {
			fmt.Fprintf(&b, "[%s] %s precision=%s ordinal=%d line=%d column=%d suppressed=%t\n", f.Level, f.Message, f.Precision, f.Ordinal, f.Line, f.Column, f.Suppressed)
		}
	}
	return publicview.Write(w, []byte(b.String()))
}

// WritePublicGitHub emits path-free annotations and an explicit omission summary.
// It does not make CI decisions or treat unknown risk as a passing verdict.
func WritePublicGitHub(w io.Writer, v ReportView) error {
	if v.Kind != ReportViewKind || v.Version != ReportViewVersion {
		return publicview.ErrInvalid
	}
	v = v.safe()
	var b strings.Builder
	fmt.Fprintf(&b, "::notice::Public review: details omitted; findings=%d suppressed_list=%d truncated=%t; not authorization\n", v.FindingCount, v.SuppressedFindingCount, v.Truncated)
	for _, list := range [][]FindingView{v.Findings, v.SuppressedFindings} {
		for _, f := range list {
			level := "warning"
			if f.Level == "high" || f.Level == "destructive" || f.Level == "blocked" || f.Level == "unknown" {
				level = "error"
			}
			if f.Suppressed {
				level = "notice"
			}
			fmt.Fprintf(&b, "::%s::%s precision=%s ordinal=%d line=%d column=%d\n", level, f.Message, f.Precision, f.Ordinal, f.Line, f.Column)
		}
	}
	return publicview.Write(w, []byte(b.String()))
}
