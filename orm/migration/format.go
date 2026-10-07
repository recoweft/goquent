package migration

import (
	"encoding/json"
	"github.com/recoweft/goquent/orm/publicoutput"
	"io"
)

// WriteJSON writes sensitive internal migration data, not public display JSON.
func WriteJSON(w io.Writer, plan *MigrationPlan) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

// WritePretty writes a human-readable migration plan.
func WritePretty(w io.Writer, plan *MigrationPlan) error {
	v, err := plan.PublicView()
	if err != nil {
		return err
	}
	return WritePublicJSON(w, v)
}

// WriteStatusJSON writes sensitive internal status data; use PublicView for display.
func WriteStatusJSON(w io.Writer, status Status) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}

// WriteStatusPretty writes a human-readable migration status.
func WriteStatusPretty(w io.Writer, status Status) error {
	return publicoutput.WriteSummary(w, status.PublicView())
}

// WriteSchemaJSON writes sensitive internal schema data; use PublicView for display.
func WriteSchemaJSON(w io.Writer, schema Schema) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(schema)
}

// WriteSchemaPretty writes a compact human-readable schema export summary.
func WriteSchemaPretty(w io.Writer, schema Schema) error {
	return publicoutput.WriteSummary(w, schema.PublicView())
}

// WriteDriftJSON writes sensitive internal drift data; use PublicView for display.
func WriteDriftJSON(w io.Writer, report DriftReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteDriftPretty writes a human-readable schema drift report.
func WriteDriftPretty(w io.Writer, report DriftReport) error {
	return publicoutput.WriteSummary(w, report.PublicView())
}
