package manifest

import "encoding/json"

// JSONSchema returns a JSON Schema for manifest version 1.
func JSONSchema() ([]byte, error) {
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://faciam.dev/goquent/manifest.schema.json",
		"title":                "Goquent Manifest",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"version", "generated_at", "generator_version", "schema_fingerprint", "policy_fingerprint", "tables"},
		"properties": map[string]any{
			"version":                    map[string]any{"const": Version},
			"generated_at":               map[string]any{"type": "string", "format": "date-time"},
			"generator_version":          map[string]any{"type": "string"},
			"dialect":                    map[string]any{"type": "string"},
			"schema_fingerprint":         map[string]any{"type": "string", "pattern": "^sha256:"},
			"policy_fingerprint":         map[string]any{"type": "string", "pattern": "^sha256:"},
			"generated_code_fingerprint": map[string]any{"type": "string"},
			"database_fingerprint":       map[string]any{"type": "string"},
			"tables":                     tableArraySchema(),
			"verification":               verificationSchema(),
		},
	}
	return json.MarshalIndent(schema, "", "  ")
}

func tableArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name"},
			"properties": map[string]any{
				"name":           map[string]any{"type": "string"},
				"model":          map[string]any{"type": "string"},
				"columns":        columnArraySchema(),
				"indexes":        indexArraySchema(),
				"relations":      relationArraySchema(),
				"policies":       policyArraySchema(),
				"query_examples": queryExampleArraySchema(),
			},
		},
	}
}

func columnArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name"},
			"properties": map[string]any{
				"name":            map[string]any{"type": "string"},
				"type":            map[string]any{"type": "string"},
				"type_source":     map[string]any{"type": "string", "enum": []string{"sql", "go"}},
				"primary":         map[string]any{"type": "boolean"},
				"nullable":        map[string]any{"type": "boolean"},
				"default":         map[string]any{"type": "string"},
				"generated":       map[string]any{"type": "boolean"},
				"enum_values":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"pii":             map[string]any{"type": "boolean"},
				"forbidden":       map[string]any{"type": "boolean"},
				"tenant_scope":    map[string]any{"type": "boolean"},
				"soft_delete":     map[string]any{"type": "boolean"},
				"required_filter": map[string]any{"type": "boolean"},
			},
		},
	}
}

func indexArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name"},
			"properties": map[string]any{
				"name":    map[string]any{"type": "string"},
				"columns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"unique":  map[string]any{"type": "boolean"},
			},
		},
	}
}

func policyArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"type"},
			"properties": map[string]any{
				"type":   map[string]any{"type": "string"},
				"column": map[string]any{"type": "string"},
				"mode":   map[string]any{"enum": []string{"", "warn", "enforce", "block"}},
			},
		},
	}
}

func relationArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name"},
			"properties": map[string]any{
				"name":       map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"table":      map[string]any{"type": "string"},
				"column":     map[string]any{"type": "string"},
				"ref_table":  map[string]any{"type": "string"},
				"ref_column": map[string]any{"type": "string"},
			},
		},
	}
}

func queryExampleArraySchema() map[string]any {
	return map[string]any{
		"type": "array",
		"items": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"name", "operation"},
			"properties": map[string]any{
				"name":        map[string]any{"type": "string"},
				"operation":   map[string]any{"type": "string"},
				"select":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"required_by": map[string]any{"type": "string"},
				"description": map[string]any{"type": "string"},
			},
		},
	}
}

func verificationSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"fresh", "checked_at"},
		"properties": map[string]any{
			"fresh":      map[string]any{"type": "boolean"},
			"checked_at": map[string]any{"type": "string", "format": "date-time"},
			"checks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"name", "status"},
					"properties": map[string]any{
						"name":     map[string]any{"type": "string"},
						"status":   map[string]any{"enum": []string{"ok", "stale", "skipped"}},
						"expected": map[string]any{"type": "string"},
						"actual":   map[string]any{"type": "string"},
						"message":  map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}
