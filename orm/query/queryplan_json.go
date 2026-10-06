package query

import (
	"github.com/recoweft/goquent/orm/internal/planversion"
)

// UnmarshalJSON replaces diagnostic data atomically. No execution evidence is decoded.
func (p *QueryPlan) UnmarshalJSON(b []byte) error {
	p.execution = nil
	p.tenantEvidence = nil
	p.writeEvidence = nil
	p.conditionSource = nil
	p.generatedConditions = false
	if err := planversion.Read(b); err != nil {
		return err
	}
	type plain QueryPlan
	var v plain
	if err := planversion.Decode(b, &v); err != nil {
		return err
	}
	*p = QueryPlan(v)
	return nil
}
