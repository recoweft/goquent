package migration

import "github.com/recoweft/goquent/internal/inputjson"

func (v *Preflight) UnmarshalJSON(b []byte) error {
	v.receipt = nil
	if inputjson.Document(b) != nil {
		return ErrPreflightInput
	}
	type plain Preflight
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = Preflight(next)
	return nil
}
func (v *PreflightPlan) UnmarshalJSON(b []byte) error {
	type plain PreflightPlan
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = PreflightPlan(next)
	return nil
}
func (v *PreflightStep) UnmarshalJSON(b []byte) error {
	type plain PreflightStep
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = PreflightStep(next)
	return nil
}
func (v *PhaseContract) UnmarshalJSON(b []byte) error {
	type plain PhaseContract
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = PhaseContract(next)
	return nil
}
func (v *PreflightRequirement) UnmarshalJSON(b []byte) error {
	type plain PreflightRequirement
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = PreflightRequirement(next)
	return nil
}
func (v *PreflightResult) UnmarshalJSON(b []byte) error {
	type plain PreflightResult
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = PreflightResult(next)
	return nil
}
func (v *PreflightCollection) UnmarshalJSON(b []byte) error {
	type plain PreflightCollection
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = PreflightCollection(next)
	return nil
}
