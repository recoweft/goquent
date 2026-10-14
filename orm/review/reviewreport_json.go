package review

import (
	"encoding/json"
	"github.com/recoweft/goquent/orm/analysis"

	"github.com/recoweft/goquent/orm/internal/planversion"
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
	p.privateOperations = cloneOperations(p.Operations)
	for i := range p.privateOperations {
		p.privateOperations[i].Detail = analysis.Detail{Reasons: []string{"unverified_input"}}.Normalize()
		p.privateOperations[i].Suppressed = false
	}
	return nil
}
