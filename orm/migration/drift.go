package migration

import "github.com/recoweft/goquent/orm/query"

// DriftReport reports whether the current database schema differs from the
// desired schema representation.
type DriftReport struct {
	CurrentSchema        *Schema               `json:"current_schema,omitempty"`
	DesiredSchema        *Schema               `json:"desired_schema,omitempty"`
	Capabilities         []OperationCapability `json:"capabilities,omitempty"`
	ComparisonIncomplete bool                  `json:"comparison_incomplete"`
	Drifted              bool                  `json:"drifted"`
	DesiredTables        int                   `json:"desired_tables"`
	CurrentTables        int                   `json:"current_tables"`
	RiskLevel            query.RiskLevel       `json:"risk_level"`
	Steps                []MigrationStep       `json:"steps,omitempty"`
}

// CompareSchemaDrift compares current database schema against desired schema.
//
// The returned steps are the migration steps that would transform current into
// desired. This is a structural drift report over migration.Schema values; it
// does not introspect a live database by itself.
func CompareSchemaDrift(desired, current Schema) DriftReport {
	plan := DiffSchemas(current, desired)
	report := DriftReport{
		DesiredTables: len(desired.Tables),
		CurrentTables: len(current.Tables),
		RiskLevel:     query.RiskLow,
	}
	if plan != nil {
		report.CurrentSchema = plan.CurrentSchema
		report.DesiredSchema = plan.DesiredSchema
		report.Capabilities = plan.Capabilities
		report.ComparisonIncomplete = plan.ComparisonIncomplete
		report.RiskLevel = plan.RiskLevel
		report.Steps = append([]MigrationStep(nil), plan.Steps...)
	}
	for _, step := range report.Steps {
		if !step.ComparisonUnknown {
			report.Drifted = true
		}
	}
	return report
}
