package migration

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/recoweft/goquent/orm/query"
)

// WriteJSON writes a machine-readable migration plan.
func WriteJSON(w io.Writer, plan *MigrationPlan) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(plan)
}

// WritePretty writes a human-readable migration plan.
func WritePretty(w io.Writer, plan *MigrationPlan) error {
	if plan == nil {
		_, err := fmt.Fprintln(w, "Migration Plan\n\nNo migration plan.")
		return err
	}
	if _, err := fmt.Fprintln(w, "Migration Plan"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nrisk: %s\nprecision: %s\n", plan.RiskLevel, plan.AnalysisPrecision); err != nil {
		return err
	}
	if plan.RequiredApproval {
		if _, err := fmt.Fprintln(w, "requires_approval: true"); err != nil {
			return err
		}
	}
	if len(plan.Steps) == 0 {
		_, err := fmt.Fprintln(w, "\nNo migration steps detected.")
		return err
	}

	for _, step := range plan.Steps {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "[%s] %s", riskLabel(step.RiskLevel), step.Type); err != nil {
			return err
		}
		if step.Table != "" {
			if _, err := fmt.Fprintf(w, " table=%s", step.Table); err != nil {
				return err
			}
		}
		if step.Column != "" {
			if _, err := fmt.Fprintf(w, " column=%s", step.Column); err != nil {
				return err
			}
		}
		if step.Index != "" {
			if _, err := fmt.Fprintf(w, " index=%s", step.Index); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if step.Line > 0 {
			if _, err := fmt.Fprintf(w, "  line: %d\n", step.Line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "  precision: %s\n", step.AnalysisPrecision); err != nil {
			return err
		}
		for _, warning := range step.Warnings {
			if _, err := fmt.Fprintf(w, "  warning[%s]: %s - %s\n", warning.Level, warning.Code, warning.Message); err != nil {
				return err
			}
			if warning.Hint != "" {
				if _, err := fmt.Fprintf(w, "    hint: %s\n", warning.Hint); err != nil {
					return err
				}
			}
		}
		if len(step.Preflight) > 0 {
			if _, err := fmt.Fprintln(w, "  suggested_preflight:"); err != nil {
				return err
			}
			for _, item := range step.Preflight {
				if _, err := fmt.Fprintf(w, "    - %s\n", item); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// WriteStatusJSON writes a machine-readable migration status.
func WriteStatusJSON(w io.Writer, status Status) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}

// WriteStatusPretty writes a human-readable migration status.
func WriteStatusPretty(w io.Writer, status Status) error {
	if _, err := fmt.Fprintln(w, "Migration Status"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\ntable: %s\nexists: %t\n", status.Table, status.Exists); err != nil {
		return err
	}
	if status.LatestApplied != "" {
		if _, err := fmt.Fprintf(w, "latest_applied: %s\n", status.LatestApplied); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "dirty: %t\nunknown: %t\n", status.Dirty, status.Unknown); err != nil {
		return err
	}
	if len(status.Pending) > 0 {
		if _, err := fmt.Fprintln(w, "\npending:"); err != nil {
			return err
		}
		for _, version := range status.Pending {
			if _, err := fmt.Fprintf(w, "  - %s\n", version); err != nil {
				return err
			}
		}
	}
	if len(status.Applied) > 0 {
		if _, err := fmt.Fprintln(w, "\napplied:"); err != nil {
			return err
		}
		for _, row := range status.Applied {
			if _, err := fmt.Fprintf(w, "  - %s", row.Version); err != nil {
				return err
			}
			if row.AppliedAt != nil {
				if _, err := fmt.Fprintf(w, " applied_at=%s", row.AppliedAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
					return err
				}
			}
			if row.Dirty {
				if _, err := fmt.Fprint(w, " dirty"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	if len(status.Warnings) > 0 {
		if _, err := fmt.Fprintln(w, "\nwarnings:"); err != nil {
			return err
		}
		for _, warning := range status.Warnings {
			if _, err := fmt.Fprintf(w, "  - %s\n", warning); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteSchemaJSON writes a machine-readable migration schema export.
func WriteSchemaJSON(w io.Writer, schema Schema) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(schema)
}

// WriteSchemaPretty writes a compact human-readable schema export summary.
func WriteSchemaPretty(w io.Writer, schema Schema) error {
	if _, err := fmt.Fprintln(w, "Schema Export"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\ntables: %d\n", len(schema.Tables)); err != nil {
		return err
	}
	for _, table := range schema.Tables {
		if _, err := fmt.Fprintf(w, "\n- %s columns=%d indexes=%d\n", table.Name, len(table.Columns), len(table.Indexes)); err != nil {
			return err
		}
		for _, column := range table.Columns {
			nullable := "not null"
			if column.Nullable {
				nullable = "nullable"
			}
			if _, err := fmt.Fprintf(w, "  column %s %s %s", column.Name, column.Type, nullable); err != nil {
				return err
			}
			if column.HasDefault {
				if _, err := fmt.Fprintf(w, " default=%s", column.DefaultExpression); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		for _, index := range table.Indexes {
			unique := ""
			if index.Unique {
				unique = " unique"
			}
			if _, err := fmt.Fprintf(w, "  index%s %s (%v)\n", unique, index.Name, index.Columns); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteDriftJSON writes a machine-readable schema drift report.
func WriteDriftJSON(w io.Writer, report DriftReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WriteDriftPretty writes a human-readable schema drift report.
func WriteDriftPretty(w io.Writer, report DriftReport) error {
	if _, err := fmt.Fprintln(w, "Schema Drift"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		w,
		"\ndrifted: %t\nrisk: %s\ndesired_tables: %d\ncurrent_tables: %d\n",
		report.Drifted,
		report.RiskLevel,
		report.DesiredTables,
		report.CurrentTables,
	); err != nil {
		return err
	}
	if len(report.Steps) == 0 {
		_, err := fmt.Fprintln(w, "\nNo schema drift detected.")
		return err
	}
	if _, err := fmt.Fprintln(w, "\nsteps:"); err != nil {
		return err
	}
	for _, step := range report.Steps {
		if _, err := fmt.Fprintf(w, "  - [%s] %s", riskLabel(step.RiskLevel), step.Type); err != nil {
			return err
		}
		if step.Table != "" {
			if _, err := fmt.Fprintf(w, " table=%s", step.Table); err != nil {
				return err
			}
		}
		if step.Column != "" {
			if _, err := fmt.Fprintf(w, " column=%s", step.Column); err != nil {
				return err
			}
		}
		if step.Index != "" {
			if _, err := fmt.Fprintf(w, " index=%s", step.Index); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func riskLabel(level query.RiskLevel) string {
	switch level {
	case query.RiskLow:
		return "Low"
	case query.RiskMedium:
		return "Medium"
	case query.RiskHigh:
		return "High"
	case query.RiskDestructive:
		return "Destructive"
	case query.RiskBlocked:
		return "Blocked"
	default:
		return string(level)
	}
}
