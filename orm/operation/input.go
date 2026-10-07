package operation

import (
	"encoding/json"
	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/internal/planversion"
)

func (f FilterSpec) hasValue() bool { return f.ValuePresent || f.Value != nil }
func (f FilterSpec) hasRef() bool   { return f.refPresent || f.ValueRef != "" }

// UnmarshalJSON preserves even null and empty declarations for arity validation.
func (f *FilterSpec) UnmarshalJSON(b []byte) error {
	raw, err := inputjson.Object(b, map[string]bool{"field": true, "op": true, "value": true, "value_ref": true})
	if err != nil {
		return ErrInvalidFilter
	}
	type plain FilterSpec
	var next plain
	if err := planversion.Decode(b, &next); err != nil {
		return ErrInvalidFilter
	}
	_, next.ValuePresent = raw["value"]
	_, next.refPresent = raw["value_ref"]
	if v, ok := raw["value_ref"]; ok && string(v) == "null" {
		return ErrInvalidFilter
	}
	*f = FilterSpec(next)
	return nil
}

func (f FilterSpec) MarshalJSON() ([]byte, error) {
	m := map[string]any{"field": f.Field, "op": f.Op}
	if f.hasValue() {
		m["value"] = f.Value
	}
	if f.hasRef() {
		m["value_ref"] = f.ValueRef
	}
	return json.Marshal(m)
}

func (o *OrderSpec) UnmarshalJSON(b []byte) error {
	if _, err := inputjson.Object(b, map[string]bool{"field": true, "direction": true}); err != nil {
		return ErrInvalidOrder
	}
	type plain OrderSpec
	var next plain
	if json.Unmarshal(b, &next) != nil {
		return ErrInvalidOrder
	}
	*o = OrderSpec(next)
	return nil
}

func inputBudget(spec OperationSpec, values map[string]any) error {
	if len(spec.Filters)+len(spec.Select)+len(spec.OrderBy)+len(values) > inputjson.MaxNodes {
		return ErrInputLimit
	}
	// Library fields are projected explicitly; arbitrary value methods never run.
	s := map[string]any{"version": spec.Version, "operation": spec.Operation, "model": spec.Model}
	if spec.Select != nil {
		s["select"] = spec.Select
	}
	if spec.Filters != nil {
		filters := make([]any, len(spec.Filters))
		for i, f := range spec.Filters {
			m := map[string]any{"field": f.Field, "op": f.Op}
			if f.hasValue() {
				m["value"] = f.Value
			}
			if f.hasRef() {
				m["value_ref"] = f.ValueRef
			}
			filters[i] = m
		}
		s["filters"] = filters
	}
	if spec.OrderBy != nil {
		orders := make([]any, len(spec.OrderBy))
		for i, o := range spec.OrderBy {
			orders[i] = map[string]any{"field": o.Field, "direction": o.Direction}
		}
		s["order_by"] = orders
	}
	if spec.Limit != nil {
		s["limit"] = *spec.Limit
	}
	if spec.AccessReason != "" {
		s["access_reason"] = spec.AccessReason
	}
	n, err := inputjson.Size(map[string]any{"spec": s, "values": values})
	if err != nil {
		return ErrInputLimit
	}
	own, _ := inputjson.Size(s)
	if spec.sourceBytes > own && spec.sourceBytes-own > inputjson.MaxBytes-n {
		return ErrInputLimit
	}
	return nil
}
