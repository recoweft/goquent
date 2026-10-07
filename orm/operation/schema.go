package operation

import "encoding/json"

// JSONSchema returns the OperationSpec MVP JSON Schema.
func JSONSchema() ([]byte, error) {
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://faciam.dev/goquent/operation-spec.schema.json",
		"title":                "Goquent OperationSpec",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"operation", "model", "select"},
		"properties": map[string]any{
			"version":       map[string]any{"type": "integer", "enum": []int{0, JSONVersion}, "description": "Missing/0 is legacy format; 1 is current. Neither grants execution authority."},
			"operation":     map[string]any{"const": OperationSelect},
			"model":         map[string]any{"type": "string", "minLength": 1},
			"select":        map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}},
			"filters":       filterArraySchema(),
			"order_by":      orderArraySchema(),
			"limit":         map[string]any{"type": "integer", "minimum": 0, "maximum": 10000},
			"access_reason": map[string]any{"type": "string"},
		},
	}
	return json.MarshalIndent(schema, "", "  ")
}

func filterArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"field", "op"},
			"allOf": []any{
				map[string]any{"if": map[string]any{"properties": map[string]any{"op": map[string]any{"not": map[string]any{"const": "in"}}}}, "then": map[string]any{"properties": map[string]any{"value": map[string]any{"type": []string{"string", "number", "boolean"}}}}},
				map[string]any{"if": map[string]any{"properties": map[string]any{"op": map[string]any{"enum": []string{"is_null", "is_not_null"}}}}, "then": map[string]any{"not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"value"}}, map[string]any{"required": []string{"value_ref"}}}}}, "else": map[string]any{"oneOf": []any{map[string]any{"required": []string{"value"}, "not": map[string]any{"required": []string{"value_ref"}}}, map[string]any{"required": []string{"value_ref"}, "not": map[string]any{"required": []string{"value"}}}}}},
				map[string]any{"if": map[string]any{"properties": map[string]any{"op": map[string]any{"const": "in"}}, "required": []string{"value"}}, "then": map[string]any{"properties": map[string]any{"value": map[string]any{"type": "array", "minItems": 1, "maxItems": 1000, "items": map[string]any{"type": []string{"string", "number", "boolean"}}}}}},
			},
			"properties": map[string]any{
				"field":     map[string]any{"type": "string", "minLength": 1},
				"op":        map[string]any{"enum": []string{"=", "!=", "<>", ">", ">=", "<", "<=", "like", "in", "is_null", "is_not_null", "eq", "ne", "gt", "gte", "lt", "lte"}},
				"value":     map[string]any{},
				"value_ref": map[string]any{"type": "string", "minLength": 1},
			},
		},
	}
}

func orderArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"field"},
			"properties": map[string]any{
				"field":     map[string]any{"type": "string", "minLength": 1},
				"direction": map[string]any{"enum": []string{"", "asc", "desc"}},
			},
		},
	}
}
