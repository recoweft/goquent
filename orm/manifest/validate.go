package manifest

import (
	"fmt"
	"strings"
)

// Validate checks the minimal versioned manifest contract.
func Validate(m *Manifest) error {
	if m == nil {
		return fmt.Errorf("goquent: manifest is nil")
	}
	if m.Version != Version {
		return fmt.Errorf("goquent: unsupported manifest version %q", m.Version)
	}
	if m.GeneratedAt.IsZero() {
		return fmt.Errorf("goquent: manifest generated_at is required")
	}
	if strings.TrimSpace(m.GeneratorVersion) == "" {
		return fmt.Errorf("goquent: manifest generator_version is required")
	}
	if !strings.HasPrefix(m.SchemaFingerprint, "sha256:") {
		return fmt.Errorf("goquent: manifest schema_fingerprint is required")
	}
	if !strings.HasPrefix(m.PolicyFingerprint, "sha256:") {
		return fmt.Errorf("goquent: manifest policy_fingerprint is required")
	}
	for _, table := range m.Tables {
		if strings.TrimSpace(table.Name) == "" {
			return fmt.Errorf("goquent: manifest table name is required")
		}
		for _, column := range table.Columns {
			if column.TypeSource != "" && column.TypeSource != "sql" && column.TypeSource != "go" {
				return fmt.Errorf("goquent: invalid manifest type source")
			}
			if strings.TrimSpace(column.Name) == "" {
				return fmt.Errorf("goquent: manifest column name is required for table %s", table.Name)
			}
		}
	}
	return nil
}
