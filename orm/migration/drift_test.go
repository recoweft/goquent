package migration

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestCompareSchemaDriftReportsSteps(t *testing.T) {
	desired := Schema{Tables: []TableSchema{{
		Name: "users",
		Columns: []ColumnSchema{
			{Name: "id", Type: "uuid"},
			{Name: "email", Type: "text"},
		},
	}}}
	current := Schema{Tables: []TableSchema{{
		Name: "users",
		Columns: []ColumnSchema{
			{Name: "id", Type: "bigint"},
			{Name: "legacy_code", Type: "text"},
		},
	}}}

	report := CompareSchemaDrift(desired, current)
	if !report.Drifted {
		t.Fatalf("expected drift report, got %#v", report)
	}
	if report.DesiredTables != 1 || report.CurrentTables != 1 {
		t.Fatalf("unexpected table counts: %#v", report)
	}
	if !hasMigrationStep(report.Steps, AlterColumnType, "users", "id") ||
		!hasMigrationStep(report.Steps, AddColumn, "users", "email") ||
		!hasMigrationStep(report.Steps, DropColumn, "users", "legacy_code") {
		t.Fatalf("unexpected drift steps: %#v", report.Steps)
	}
}

func TestWriteDriftPrettyAndJSON(t *testing.T) {
	report := CompareSchemaDrift(
		Schema{Tables: []TableSchema{{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "uuid"}}}}},
		Schema{Tables: []TableSchema{{Name: "users", Columns: []ColumnSchema{{Name: "id", Type: "bigint"}}}}},
	)

	var pretty bytes.Buffer
	if err := WriteDriftPretty(&pretty, report); err != nil {
		t.Fatalf("write pretty: %v", err)
	}
	if !strings.Contains(pretty.String(), "Schema Drift") || !strings.Contains(pretty.String(), "alter_column_type") {
		t.Fatalf("unexpected pretty drift output: %s", pretty.String())
	}

	var jsonBuf bytes.Buffer
	if err := WriteDriftJSON(&jsonBuf, report); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var decoded DriftReport
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if !decoded.Drifted || len(decoded.Steps) != 1 {
		t.Fatalf("unexpected decoded drift report: %#v", decoded)
	}
}

func hasMigrationStep(steps []MigrationStep, typ MigrationStepType, table, column string) bool {
	for _, step := range steps {
		if step.Type == typ && step.Table == table && step.Column == column {
			return true
		}
	}
	return false
}
