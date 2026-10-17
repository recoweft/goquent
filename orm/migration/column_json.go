package migration

import (
	"bytes"
	"encoding/json"
	"github.com/recoweft/goquent/internal/inputjson"
	"strings"
)

// UnmarshalJSON preserves nullable presence and refuses ambiguous declarations.
func (c *ColumnSchema) UnmarshalJSON(b []byte) error {
	fields, err := inputjson.Object(b, nil)
	if err != nil {
		return err
	}
	for _, key := range []string{"has_default", "default_expression", "default_state", "default_source"} {
		for k := range fields {
			if strings.EqualFold(k, key) && k != key {
				return inputjson.ErrInvalid
			}
		}
		if raw, ok := fields[key]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return inputjson.ErrInvalid
		}
	}
	nullable, known, err := inputjson.Nullable(fields)
	if err != nil {
		return err
	}

	type plain ColumnSchema
	var next plain
	if err := json.Unmarshal(b, &next); err != nil {
		return inputjson.ErrInvalid
	}
	next.Nullable = nullable
	next.NullableKnown = known
	*c = ColumnSchema(next)
	return nil
}

// MarshalJSON never promotes missing nullability to a declaration.
func (c ColumnSchema) MarshalJSON() ([]byte, error) {
	var n *bool
	if c.NullableKnown {
		v := c.Nullable
		n = &v
	}
	return json.Marshal(struct {
		Name              string       `json:"name"`
		Type              string       `json:"type,omitempty"`
		Nullable          *bool        `json:"nullable,omitempty"`
		HasDefault        bool         `json:"has_default,omitempty"`
		DefaultExpression string       `json:"default_expression,omitempty"`
		DefaultState      DefaultState `json:"default_state,omitempty"`
		DefaultSource     string       `json:"default_source,omitempty"`
	}{c.Name, c.Type, n, c.HasDefault, c.DefaultExpression, c.DefaultState, c.DefaultSource})
}
