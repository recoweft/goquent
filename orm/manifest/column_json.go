package manifest

import (
	"encoding/json"
	"github.com/recoweft/goquent/internal/inputjson"
)

// UnmarshalJSON preserves nullable presence and refuses ambiguous declarations.
func (c *Column) UnmarshalJSON(b []byte) error {
	fields, err := inputjson.Object(b, map[string]bool{"name": true, "type": true, "type_source": true, "primary": true, "nullable": true, "default": true, "readonly": true, "generated": true, "enum_values": true, "pii": true, "forbidden": true, "tenant_scope": true, "soft_delete": true, "required_filter": true})
	if err != nil {
		return err
	}
	nullable, known, err := inputjson.Nullable(fields)
	if err != nil {
		return err
	}
	if b, ok := fields["type_source"]; ok {
		var v string
		if json.Unmarshal(b, &v) != nil || (v != "sql" && v != "go") {
			return inputjson.ErrInvalid
		}
	}
	if raw, ok := fields["readonly"]; ok && string(raw) != "true" && string(raw) != "false" {
		return inputjson.ErrInvalid
	}
	type plain Column
	var next plain
	if err := json.Unmarshal(b, &next); err != nil {
		return inputjson.ErrInvalid
	}
	next.Nullable = nullable
	next.NullableKnown = known
	*c = Column(next)
	return nil
}

// MarshalJSON never promotes missing nullability to a declaration.
func (c Column) MarshalJSON() ([]byte, error) {
	var n *bool
	if c.NullableKnown {
		v := c.Nullable
		n = &v
	}
	type plain Column
	return json.Marshal(struct {
		Name           string   `json:"name"`
		Type           string   `json:"type,omitempty"`
		TypeSource     string   `json:"type_source,omitempty"`
		Primary        bool     `json:"primary,omitempty"`
		Nullable       *bool    `json:"nullable,omitempty"`
		Default        string   `json:"default,omitempty"`
		Readonly       bool     `json:"readonly,omitempty"`
		Generated      bool     `json:"generated,omitempty"`
		EnumValues     []string `json:"enum_values,omitempty"`
		PII            bool     `json:"pii,omitempty"`
		Forbidden      bool     `json:"forbidden,omitempty"`
		TenantScope    bool     `json:"tenant_scope,omitempty"`
		SoftDelete     bool     `json:"soft_delete,omitempty"`
		RequiredFilter bool     `json:"required_filter,omitempty"`
	}{c.Name, c.Type, c.TypeSource, c.Primary, n, c.Default, c.Readonly, c.Generated, c.EnumValues, c.PII, c.Forbidden, c.TenantScope, c.SoftDelete, c.RequiredFilter})
}
