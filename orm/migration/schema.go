package migration

import (
	"strings"

	"github.com/recoweft/goquent/orm/query"
)

// Schema is a minimal desired/current schema representation for diff planning.
type Schema struct {
	Tables []TableSchema `json:"tables,omitempty"`
}

// TableSchema describes a table in a schema diff.
type TableSchema struct {
	Name    string         `json:"name"`
	Columns []ColumnSchema `json:"columns,omitempty"`
	Indexes []IndexSchema  `json:"indexes,omitempty"`
}

// ColumnSchema describes a column in a schema diff.
type ColumnSchema struct {
	Name              string `json:"name"`
	Type              string `json:"type,omitempty"`
	Nullable          bool   `json:"nullable"`
	HasDefault        bool   `json:"has_default,omitempty"`
	DefaultExpression string `json:"default_expression,omitempty"`
}

// IndexSchema describes an index in a schema diff.
type IndexSchema struct {
	Name       string   `json:"name"`
	Columns    []string `json:"columns,omitempty"`
	Unique     bool     `json:"unique,omitempty"`
	Concurrent bool     `json:"concurrent,omitempty"`
}

// PlanSteps builds a MigrationPlan from structured steps.
func PlanSteps(steps []MigrationStep) *MigrationPlan {
	plan := &MigrationPlan{
		RiskLevel:         query.RiskLow,
		AnalysisPrecision: query.AnalysisPrecise,
		Metadata:          map[string]any{"source": "structured_steps"},
	}
	for _, step := range steps {
		if step.AnalysisPrecision == "" {
			step.AnalysisPrecision = query.AnalysisPrecise
		}
		classifyStep(&step)
		plan.Steps = append(plan.Steps, step)
	}
	finalizePlan(plan)
	return plan
}

// DiffSchemas creates a migration plan that transforms current into desired.
func DiffSchemas(current, desired Schema) *MigrationPlan {
	currentTables := tableMap(current.Tables)
	desiredTables := tableMap(desired.Tables)

	var steps []MigrationStep
	for _, table := range desired.Tables {
		if _, ok := currentTables[tableKey(table.Name)]; !ok {
			steps = append(steps, MigrationStep{Type: AddTable, Table: table.Name})
			continue
		}
		steps = append(steps, diffTable(currentTables[tableKey(table.Name)], table)...)
	}
	for _, table := range current.Tables {
		if _, ok := desiredTables[tableKey(table.Name)]; !ok {
			steps = append(steps, MigrationStep{Type: DropTable, Table: table.Name})
		}
	}

	plan := PlanSteps(steps)
	plan.Metadata["source"] = "schema_diff"
	return plan
}

func diffTable(current, desired TableSchema) []MigrationStep {
	currentColumns := columnMap(current.Columns)
	desiredColumns := columnMap(desired.Columns)
	var steps []MigrationStep

	for _, column := range desired.Columns {
		currentColumn, ok := currentColumns[columnKey(column.Name)]
		if !ok {
			nullable := column.Nullable
			steps = append(steps, MigrationStep{
				Type:              AddColumn,
				Table:             desired.Name,
				Column:            column.Name,
				ColumnType:        column.Type,
				Nullable:          &nullable,
				HasDefault:        column.HasDefault,
				DefaultExpression: column.DefaultExpression,
			})
			continue
		}
		if normalizeType(currentColumn.Type) != normalizeType(column.Type) {
			steps = append(steps, MigrationStep{
				Type:    AlterColumnType,
				Table:   desired.Name,
				Column:  column.Name,
				OldType: currentColumn.Type,
				NewType: column.Type,
			})
		}
		if currentColumn.Nullable && !column.Nullable {
			steps = append(steps, MigrationStep{
				Type:     AlterNullability,
				Table:    desired.Name,
				Column:   column.Name,
				OldType:  currentColumn.Type,
				NewType:  column.Type,
				Nullable: boolPtr(false),
			})
		}
		if !currentColumn.Nullable && column.Nullable {
			steps = append(steps, MigrationStep{
				Type:     AlterNullability,
				Table:    desired.Name,
				Column:   column.Name,
				OldType:  currentColumn.Type,
				NewType:  column.Type,
				Nullable: boolPtr(true),
			})
		}
	}
	for _, column := range current.Columns {
		if _, ok := desiredColumns[columnKey(column.Name)]; !ok {
			steps = append(steps, MigrationStep{Type: DropColumn, Table: desired.Name, Column: column.Name})
		}
	}

	currentIndexes := indexMap(current.Indexes)
	desiredIndexes := indexMap(desired.Indexes)
	for _, index := range desired.Indexes {
		if _, ok := currentIndexes[indexKey(index.Name)]; !ok {
			steps = append(steps, MigrationStep{
				Type:       AddIndex,
				Table:      desired.Name,
				Index:      index.Name,
				Concurrent: index.Concurrent,
			})
		}
	}
	for _, index := range current.Indexes {
		if _, ok := desiredIndexes[indexKey(index.Name)]; !ok {
			steps = append(steps, MigrationStep{Type: DropIndex, Table: desired.Name, Index: index.Name})
		}
	}
	return steps
}

func tableMap(tables []TableSchema) map[string]TableSchema {
	out := make(map[string]TableSchema, len(tables))
	for _, table := range tables {
		out[tableKey(table.Name)] = table
	}
	return out
}

func columnMap(columns []ColumnSchema) map[string]ColumnSchema {
	out := make(map[string]ColumnSchema, len(columns))
	for _, column := range columns {
		out[columnKey(column.Name)] = column
	}
	return out
}

func indexMap(indexes []IndexSchema) map[string]IndexSchema {
	out := make(map[string]IndexSchema, len(indexes))
	for _, index := range indexes {
		out[indexKey(index.Name)] = index
	}
	return out
}

func tableKey(name string) string  { return strings.ToLower(strings.TrimSpace(name)) }
func columnKey(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
func indexKey(name string) string  { return strings.ToLower(strings.TrimSpace(name)) }

func boolPtr(v bool) *bool { return &v }
