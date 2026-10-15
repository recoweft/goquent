package review

import (
	"encoding/json"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/internal/planversion"
	"github.com/recoweft/goquent/orm/query"
)

func (p ReviewReport) MarshalJSON() ([]byte, error) {
	if err := planversion.Check(p.Version); err != nil {
		return nil, err
	}
	p.Version = planversion.Current
	type plain ReviewReport
	return json.Marshal(plain(p))
}

// UnmarshalJSON replaces diagnostic data atomically. No execution evidence is decoded.
func (p *ReviewReport) UnmarshalJSON(b []byte) error {
	if err := planversion.Read(b); err != nil {
		return err
	}
	type plain ReviewReport
	var v plain
	if err := planversion.Decode(b, &v); err != nil {
		return err
	}
	for i := range v.Operations {
		v.Operations[i].Correspondence = analysis.Correspondence{}.Normalize()
	}
	*p = ReviewReport(v)
	p.privateFindings = unverifiedFindings(p.Findings)
	p.privateSuppressedFindings = unverifiedFindings(p.SuppressedFindings)
	p.privateOperations = cloneOperations(p.Operations)
	if p.privateOperations == nil {
		p.privateOperations = []OperationReview{}
	}
	for i := range p.privateOperations {
		p.privateOperations[i].Detail = unverifiedDetail(p.privateOperations[i].Detail)
		p.privateOperations[i].Suppressed = false
	}
	return nil
}

// precisionFindings selects the decoded diagnostic snapshot, not mutable wire claims.
// In-process reports retain their original analysis and suppression semantics.
func (p ReviewReport) precisionFindings() ReviewReport {
	if p.privateFindings != nil {
		p.Findings = p.privateFindings
		p.SuppressedFindings = p.privateSuppressedFindings
	}
	return p
}

func unverifiedDetail(src analysis.Detail) analysis.Detail {
	d := analysis.Detail{Reasons: []string{"unverified_input"}}.Normalize()
	// Unsupported coverage must not be weakened to unknown on re-input.
	if src.SQL == analysis.Unsupported {
		d.SQL = analysis.Unsupported
	}
	if src.Columns == analysis.Unsupported {
		d.Columns = analysis.Unsupported
	}
	if src.Bindings == analysis.Unsupported {
		d.Bindings = analysis.Unsupported
	}
	if src.Policy == analysis.Unsupported {
		d.Policy = analysis.Unsupported
	}
	return d
}

func unverifiedFindings(src []Finding) []Finding {
	out := make([]Finding, len(src))
	for i, f := range src {
		f.Detail = unverifiedDetail(f.Detail)
		if f.AnalysisPrecision != query.AnalysisUnsupported {
			f.AnalysisPrecision = query.AnalysisPrecision(f.Detail.Precision())
		}
		f.Location = cloneLocation(f.Location)
		out[i] = f
	}
	return out
}
