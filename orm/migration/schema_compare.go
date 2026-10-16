package migration

import (
	"regexp"
	"slices"

	"github.com/recoweft/goquent/orm/query"
)

// DefaultState describes a declaration, never live verification or authorization.
type DefaultState string

const (
	DefaultUnknown    DefaultState = "unknown"
	DefaultNone       DefaultState = "none"
	DefaultExpression DefaultState = "expression"
)

// DefaultDefinition retains sensitive source data. It is not a public view.
type DefaultDefinition struct {
	State DefaultState `json:"state"`
	// DeclaredState and HasDefault retain the original, possibly contradictory input.
	DeclaredState DefaultState `json:"declared_state,omitempty"`
	HasDefault    bool         `json:"has_default"`
	Expression    string       `json:"expression,omitempty"`
	Source        string       `json:"source,omitempty"`
}

func columnDefault(c ColumnSchema) DefaultDefinition {
	d := DefaultDefinition{State: DefaultUnknown, DeclaredState: c.DefaultState, HasDefault: c.HasDefault, Expression: c.DefaultExpression, Source: c.DefaultSource}
	switch c.DefaultState {
	case "":
		if c.HasDefault && c.DefaultExpression != "" {
			d.State = DefaultExpression
		}
	case DefaultNone:
		if !c.HasDefault && c.DefaultExpression == "" {
			d.State = DefaultNone
		}
	case DefaultExpression:
		if c.HasDefault && c.DefaultExpression != "" {
			d.State = DefaultExpression
		}
	}
	return d
}

func diffDefault(table string, old, next ColumnSchema) []MigrationStep {
	a, b := columnDefault(old), columnDefault(next)
	if a.State == DefaultUnknown || b.State == DefaultUnknown {
		s := comparisonUnknown(table, next.Name, "")
		s.CurrentDefault = &a
		s.DesiredDefault = &b
		return []MigrationStep{s}
	}
	if a.State == b.State && a.Expression == b.Expression {
		return nil
	}
	return []MigrationStep{{Type: AlterColumnDefault, Table: table, Column: next.Name, CurrentDefault: &a, DesiredDefault: &b}}
}

func comparisonUnknown(table, column, index string) MigrationStep {
	return MigrationStep{Type: UnsupportedStep, Table: table, Column: column, Index: index, ComparisonUnknown: true, AnalysisPrecision: query.AnalysisUnsupported}
}
func schemaDiffPlan(steps []MigrationStep) *MigrationPlan {
	p := PlanSteps(steps)
	p.Metadata["source"] = "schema_diff"
	for _, s := range steps {
		if s.ComparisonUnknown {
			p.ComparisonIncomplete = true
		}
	}
	return p
}

// Only unqualified lowercase identifiers have dialect-independent identity here.
// Reject ambiguous identities before maps can overwrite declarations.
var simpleSchemaName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func validSchemaNames(s Schema) bool {
	tables := map[string]bool{}
	for _, t := range s.Tables {
		if !simpleSchemaName.MatchString(t.Name) || tables[t.Name] {
			return false
		}
		tables[t.Name] = true
	}
	return true
}
func validIndex(i IndexSchema, t TableSchema) bool {
	if !simpleSchemaName.MatchString(i.Name) || i.DefinitionUnknown || len(i.Columns) == 0 {
		return false
	}
	count := 0
	for _, other := range t.Indexes {
		if indexKey(other.Name) == indexKey(i.Name) {
			count++
		}
	}
	if count != 1 {
		return false
	}
	columns := columnMap(t.Columns)
	seen := map[string]bool{}
	for _, c := range i.Columns {
		if !simpleSchemaName.MatchString(c) || seen[c] {
			return false
		}
		matches := 0
		for _, col := range t.Columns {
			if columnKey(col.Name) == columnKey(c) {
				matches++
			}
		}
		if matches != 1 {
			return false
		}
		if _, ok := columns[c]; !ok {
			return false
		}
		seen[c] = true
	}
	return true
}
func equalIndex(a, b IndexSchema) bool {
	return a.Unique == b.Unique && slices.Equal(a.Columns, b.Columns)
}
func copyIndex(i IndexSchema) *IndexSchema { i.Columns = slices.Clone(i.Columns); return &i }

// Remove only tables with ambiguous identities, preserving other known changes.
func comparableSchemas(a, b Schema) (Schema, Schema, bool) {
	bad := map[string]bool{}
	for _, s := range []Schema{a, b} {
		seen := map[string]bool{}
		for _, t := range s.Tables {
			k := tableKey(t.Name)
			if seen[k] || !validSchemaNames(Schema{Tables: []TableSchema{t}}) {
				bad[k] = true
			}
			seen[k] = true
		}
	}
	filter := func(s Schema) Schema {
		out := Schema{}
		for _, t := range s.Tables {
			if !bad[tableKey(t.Name)] {
				out.Tables = append(out.Tables, t)
			}
		}
		return out
	}
	return filter(a), filter(b), len(bad) > 0
}

func ambiguousColumns(a, b TableSchema) map[string]bool {
	bad := map[string]bool{}
	for _, t := range []TableSchema{a, b} {
		seen := map[string]bool{}
		for _, c := range t.Columns {
			k := columnKey(c.Name)
			if !simpleSchemaName.MatchString(c.Name) || seen[k] {
				bad[k] = true
			}
			seen[k] = true
		}
	}
	return bad
}
