package migration

import (
	"reflect"
	"slices"
)

// A tuple preserves component boundaries; SQL quoting is deliberately not parsed.
type objectKey struct{ catalog, schema, name string }

func objectTableKey(t TableSchema) objectKey {
	if identityKnown(t.Identity) {
		return objectKey{t.Identity.Catalog, t.Identity.Schema, t.Name}
	}
	return objectKey{name: t.Name}
}
func tableIdentityValid(t TableSchema, c *SchemaContext) bool {
	if !identityKnown(t.Identity) || t.Identity.Name != t.Name || !simpleSchemaName.MatchString(t.Name) {
		return false
	}
	return c == nil || (c.Catalog == t.Identity.Catalog && c.Schema == t.Identity.Schema && c.Coverage.Identity == CoverageComplete)
}
func contextKnown(c *SchemaContext) bool {
	if c == nil || c.Coverage.Identity != CoverageComplete || !simpleSchemaName.MatchString(c.Catalog) || !simpleSchemaName.MatchString(c.Schema) {
		return false
	}
	if c.Dialect != "mysql" && c.Dialect != "postgres" {
		return false
	}
	if c.Scope == "namespace" {
		return len(c.Tables) == 0
	}
	if c.Scope != "tables" || len(c.Tables) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, n := range c.Tables {
		if !simpleSchemaName.MatchString(n) || seen[n] {
			return false
		}
		seen[n] = true
	}
	return true
}
func sameScope(a, b *SchemaContext) bool {
	if !contextKnown(a) || !contextKnown(b) || a.Dialect != b.Dialect || a.Catalog != b.Catalog || a.Schema != b.Schema || a.Scope != b.Scope {
		return false
	}
	x, y := slices.Clone(a.Tables), slices.Clone(b.Tables)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}
func inventoryComplete(c *SchemaContext) bool {
	return contextKnown(c) && c.Coverage.Tables == CoverageComplete && c.Visibility == "declared"
}
func inScope(t TableSchema, c *SchemaContext) bool {
	return contextKnown(c) && tableIdentityValid(t, c) && (c.Scope == "namespace" || slices.Contains(c.Tables, t.Name))
}
func compareExtendedSchemas(current, desired Schema) *MigrationPlan {
	var steps []MigrationStep
	scope := sameScope(current.Context, desired.Context)
	if !scope || !inventoryComplete(current.Context) || !inventoryComplete(desired.Context) || !contextCoverageKnown(current.Context) || !contextCoverageKnown(desired.Context) {
		steps = append(steps, comparisonUnknown("", "", ""))
	}
	if contextKnown(current.Context) && contextKnown(desired.Context) && (current.Context.Catalog != desired.Context.Catalog || current.Context.Schema != desired.Context.Schema || current.Context.Dialect != desired.Context.Dialect) {
		steps = append(steps, MigrationStep{Type: SchemaIdentityDifference})
	}
	a, b := map[objectKey][]TableSchema{}, map[objectKey][]TableSchema{}
	for _, t := range current.Tables {
		k := objectTableKey(t)
		a[k] = append(a[k], t)
	}
	for _, t := range desired.Tables {
		k := objectTableKey(t)
		b[k] = append(b[k], t)
	}
	handled := map[objectKey]bool{}
	for _, next := range desired.Tables {
		k := objectTableKey(next)
		if handled[k] {
			continue
		}
		handled[k] = true
		old := a[k]
		if len(b[k]) != 1 || len(old) > 1 || !simpleSchemaName.MatchString(next.Name) {
			steps = append(steps, tableUnknown(nil, &next))
			continue
		}
		if len(old) == 1 {
			prev := old[0]
			knownID := inScope(prev, current.Context) && inScope(next, desired.Context)
			// Legacy local matching is allowed only when neither side claims an identity.
			legacy := prev.Identity == (ObjectIdentity{}) && next.Identity == (ObjectIdentity{})
			if !knownID && !legacy {
				steps = append(steps, tableUnknown(&prev, &next))
				continue
			}
			if !knownID || !scope || !coverageKnown(prev.Coverage) || !coverageKnown(next.Coverage) {
				steps = append(steps, tableUnknown(&prev, &next))
			}
			for _, t := range []TableSchema{prev, next} {
				for _, i := range t.Indexes {
					if i.Attributes != CoverageComplete {
						steps = append(steps, tableUnknown(&prev, &next))
					}
				}
			}
			ac, bc := effectiveCoverage(prev.Coverage, current.Context), effectiveCoverage(next.Coverage, desired.Context)
			if ambiguousConstraints(prev) || ambiguousConstraints(next) {
				steps = append(steps, tableUnknown(&prev, &next))
			}
			local := diffTable(prev, next)
			for _, s := range local {
				s.CurrentTable = copyTable(prev)
				s.DesiredTable = copyTable(next)
				// An absent row is not a proved collection difference without coverage.
				absent := false
				switch s.Type {
				case AddColumn:
					absent = ac.Columns != CoverageComplete
				case DropColumn:
					absent = bc.Columns != CoverageComplete
				case AddIndex:
					absent = ac.Indexes != CoverageComplete
				case DropIndex:
					absent = bc.Indexes != CoverageComplete
				}
				if absent || (!scope && (s.Type == AddColumn || s.Type == DropColumn || s.Type == AddIndex || s.Type == DropIndex)) {
					s.ComparisonUnknown = true
					s.Type = UnsupportedStep
				}
				steps = append(steps, s)
			}
			steps = append(steps, diffConstraints(prev, next, scope, ac, bc)...)
		} else {
			s := MigrationStep{Type: AddTable, Table: next.Name, DesiredTable: copyTable(next)}
			if !scope || !inventoryComplete(current.Context) || !inventoryComplete(desired.Context) || !inScope(next, desired.Context) {
				s.Type = UnsupportedStep
				s.ComparisonUnknown = true
			}
			steps = append(steps, s)
			steps = append(steps, extendedTableDiagnostics(next)...)
		}
	}
	for _, prev := range current.Tables {
		k := objectTableKey(prev)
		if len(b[k]) > 0 {
			continue
		}
		if handled[k] {
			continue
		}
		handled[k] = true
		s := MigrationStep{Type: DropTable, Table: prev.Name, CurrentTable: copyTable(prev)}
		if len(a[k]) != 1 || !scope || !inventoryComplete(desired.Context) || !inventoryComplete(current.Context) || !inScope(prev, current.Context) {
			s.Type = UnsupportedStep
			s.ComparisonUnknown = true
		}
		steps = append(steps, s)
		steps = append(steps, extendedTableDiagnostics(prev)...)
	}
	p := schemaDiffPlan(steps)
	p.CurrentSchema = copySchema(current)
	p.DesiredSchema = copySchema(desired)
	p.Capabilities = capabilitiesFor(current.Context, p.Steps)
	return p
}
func tableUnknown(a, b *TableSchema) MigrationStep {
	s := comparisonUnknown("", "", "")
	if a != nil {
		s.CurrentTable = copyTable(*a)
		s.Table = a.Name
	}
	if b != nil {
		s.DesiredTable = copyTable(*b)
		s.Table = b.Name
	}
	return s
}
func extendedTableDiagnostics(t TableSchema) []MigrationStep {
	out := tableComparisonDiagnostics(t)
	if ambiguousConstraints(t) {
		out = append(out, tableUnknown(nil, &t))
	}
	for _, i := range t.Indexes {
		if i.Attributes != CoverageComplete {
			out = append(out, tableUnknown(nil, &t))
		}
	}
	if !identityKnown(t.Identity) || !coverageKnown(t.Coverage) {
		out = append(out, tableUnknown(nil, &t))
	}
	for _, f := range t.ForeignKeys {
		if !validFK(f, t) || f.Definition != CoverageComplete || f.Validation == "unknown" {
			out = append(out, tableUnknown(nil, &t))
		}
	}
	for _, c := range t.Checks {
		if !validCheck(c) || c.Definition != CoverageComplete || c.Validation == "unknown" {
			out = append(out, tableUnknown(nil, &t))
		}
	}
	return out
}
func knownColumns(cols []string, t TableSchema) bool {
	if len(cols) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, n := range cols {
		count := 0
		for _, c := range t.Columns {
			if c.Name == n {
				count++
			}
		}
		if !simpleSchemaName.MatchString(n) || seen[n] || count != 1 {
			return false
		}
		seen[n] = true
	}
	return true
}
func validFK(f ForeignKeySchema, t TableSchema) bool {
	if (f.Definition != CoverageComplete && f.Definition != CoveragePartial) || !simpleSchemaName.MatchString(f.Name) || !knownColumns(f.Columns, t) || !identityKnown(f.Reference) || len(f.Columns) != len(f.ReferencedColumns) {
		return false
	}
	if identityKnown(t.Identity) && f.Reference.Catalog != t.Identity.Catalog {
		return false
	}
	if f.Reference.Catalog == t.Identity.Catalog && f.Reference.Schema == t.Identity.Schema && f.Reference.Name == t.Name && !knownColumns(f.ReferencedColumns, t) {
		return false
	}
	seen := map[string]bool{}
	for _, n := range f.ReferencedColumns {
		if !simpleSchemaName.MatchString(n) || seen[n] {
			return false
		}
		seen[n] = true
	}
	actions := []string{"no_action", "restrict", "cascade", "set_null", "set_default"}
	return slices.Contains(actions, f.OnUpdate) && slices.Contains(actions, f.OnDelete) && slices.Contains([]string{"simple", "full", "partial"}, f.Match) && slices.Contains([]string{"validated", "not_validated", "unknown"}, f.Validation) && slices.Contains([]string{"enforced", "not_enforced"}, f.Enforcement) && slices.Contains([]string{"not_deferrable", "immediate", "deferred"}, f.Deferrability)
}
func validCheck(c CheckSchema) bool {
	return (c.Definition == CoverageComplete || c.Definition == CoveragePartial) && simpleSchemaName.MatchString(c.Name) && slices.Contains([]string{"inheritable", "no_inherit", "not_applicable"}, c.Inheritance) && c.Expression != "" && slices.Contains([]string{"validated", "not_validated", "unknown"}, c.Validation) && slices.Contains([]string{"enforced", "not_enforced"}, c.Enforcement)
}
func diffConstraints(a, b TableSchema, scope bool, aCoverage, bCoverage SchemaCoverage) []MigrationStep {
	var out []MigrationStep
	for _, t := range []TableSchema{a, b} {
		for _, f := range t.ForeignKeys {
			if f.Definition != CoverageComplete || f.Validation == "unknown" {
				out = append(out, tableUnknown(&a, &b))
			}
		}
		for _, c := range t.Checks {
			if c.Definition != CoverageComplete || c.Validation == "unknown" {
				out = append(out, tableUnknown(&a, &b))
			}
		}
	}
	// Names are scoped to a table, never globally across namespaces.
	type item struct {
		name  string
		valid bool
		value any
	}
	compare := func(old, next []item, ac, bc CoverageState, add, drop, alter MigrationStepType) {
		am, bm := map[string][]item{}, map[string][]item{}
		for _, v := range old {
			am[v.name] = append(am[v.name], v)
		}
		for _, v := range next {
			bm[v.name] = append(bm[v.name], v)
		}
		seen := map[string]bool{}
		for _, v := range append(slices.Clone(next), old...) {
			if seen[v.name] {
				continue
			}
			seen[v.name] = true
			x, y := am[v.name], bm[v.name]
			s := MigrationStep{Constraint: v.name, Table: b.Name, CurrentTable: copyTable(a), DesiredTable: copyTable(b)}
			invalidDefinition := (len(x) == 1 && !x[0].valid) || (len(y) == 1 && !y[0].valid)
			ambiguous := len(x) > 1 || len(y) > 1 || !simpleSchemaName.MatchString(v.name) || constraintNameCount(a, v.name) > 1 || constraintNameCount(b, v.name) > 1
			if invalidDefinition && !ambiguous && ((len(x) == 0 && ac == CoverageComplete && scope) || (len(y) == 0 && bc == CoverageComplete && scope)) {
				marker := s
				marker.Type = UnsupportedStep
				marker.ComparisonUnknown = true
				out = append(out, marker)
				if len(x) == 0 {
					s.Type = add
				} else {
					s.Type = drop
				}
				out = append(out, s)
				continue
			}
			if ambiguous || invalidDefinition {
				s.Type = UnsupportedStep
				s.ComparisonUnknown = true
			} else if len(x) == 0 {
				s.Type = add
				if ac != CoverageComplete || !scope {
					s.Type = UnsupportedStep
					s.ComparisonUnknown = true
				}
			} else if len(y) == 0 {
				s.Type = drop
				if bc != CoverageComplete || !scope {
					s.Type = UnsupportedStep
					s.ComparisonUnknown = true
				}
			} else if !equalConstraintValue(x[0].value, y[0].value) {
				s.Type = alter
			} else {
				continue
			}
			out = append(out, s)
		}
	}
	af, bf := []item{}, []item{}
	for _, f := range a.ForeignKeys {
		v := f
		v.RawDefinition = ""
		af = append(af, item{f.Name, validFK(f, a), v})
	}
	for _, f := range b.ForeignKeys {
		v := f
		v.RawDefinition = ""
		bf = append(bf, item{f.Name, validFK(f, b), v})
	}
	compare(af, bf, aCoverage.ForeignKeys, bCoverage.ForeignKeys, AddForeignKey, DropForeignKey, AlterForeignKey)
	ac, bc := []item{}, []item{}
	for _, c := range a.Checks {
		v := c
		v.RawDefinition = ""
		ac = append(ac, item{c.Name, validCheck(c), v})
	}
	for _, c := range b.Checks {
		v := c
		v.RawDefinition = ""
		bc = append(bc, item{c.Name, validCheck(c), v})
	}
	compare(ac, bc, aCoverage.Checks, bCoverage.Checks, AddCheck, DropCheck, AlterCheck)
	return out
}

func equalConstraintValue(a, b any) bool {
	switch x := a.(type) {
	case ForeignKeySchema:
		y := b.(ForeignKeySchema)
		if x.Validation == "unknown" || y.Validation == "unknown" {
			x.Validation = ""
			y.Validation = ""
		}
		x.Definition = ""
		y.Definition = ""
		return reflect.DeepEqual(x, y)
	case CheckSchema:
		y := b.(CheckSchema)
		if x.Validation == "unknown" || y.Validation == "unknown" {
			x.Validation = ""
			y.Validation = ""
		}
		x.Definition = ""
		y.Definition = ""
		return reflect.DeepEqual(x, y)
	}
	return false
}

func contextCoverageKnown(c *SchemaContext) bool { return c != nil && coverageKnown(c.Coverage) }
func effectiveCoverage(t SchemaCoverage, c *SchemaContext) SchemaCoverage {
	if c == nil {
		return SchemaCoverage{}
	}
	intersect := func(a, b CoverageState) CoverageState {
		if a == CoverageComplete && b == CoverageComplete {
			return CoverageComplete
		}
		return CoverageUnknown
	}
	t.Columns = intersect(t.Columns, c.Coverage.Columns)
	t.Indexes = intersect(t.Indexes, c.Coverage.Indexes)
	t.ForeignKeys = intersect(t.ForeignKeys, c.Coverage.ForeignKeys)
	t.Checks = intersect(t.Checks, c.Coverage.Checks)
	return t
}

// Public summaries inspect completeness without allocating sensitive diff copies.
func schemaDefinitionComplete(s Schema) bool {
	if !inventoryComplete(s.Context) || !contextCoverageKnown(s.Context) {
		return false
	}
	seen := map[objectKey]bool{}
	for _, t := range s.Tables {
		k := objectTableKey(t)
		if seen[k] || !inScope(t, s.Context) || !coverageKnown(t.Coverage) || len(ambiguousColumns(t, TableSchema{})) > 0 || ambiguousConstraints(t) {
			return false
		}
		seen[k] = true
		for _, c := range t.Columns {
			if columnDefault(c).State == DefaultUnknown {
				return false
			}
		}
		for _, i := range t.Indexes {
			if i.Attributes != CoverageComplete || !validIndex(i, t) {
				return false
			}
		}
		names := map[string]bool{}
		for _, f := range t.ForeignKeys {
			if names[f.Name] || !validFK(f, t) || f.Definition != CoverageComplete || f.Validation == "unknown" {
				return false
			}
			names[f.Name] = true
		}
		names = map[string]bool{}
		for _, c := range t.Checks {
			if names[c.Name] || !validCheck(c) || c.Definition != CoverageComplete || c.Validation == "unknown" {
				return false
			}
			names[c.Name] = true
		}
	}
	return true
}

func ambiguousConstraints(t TableSchema) bool {
	seen := map[string]bool{}
	for _, f := range t.ForeignKeys {
		if seen[f.Name] {
			return true
		}
		seen[f.Name] = true
	}
	for _, c := range t.Checks {
		if seen[c.Name] {
			return true
		}
		seen[c.Name] = true
	}
	return false
}

func constraintNameCount(t TableSchema, name string) int {
	n := 0
	for _, f := range t.ForeignKeys {
		if f.Name == name {
			n++
		}
	}
	for _, c := range t.Checks {
		if c.Name == name {
			n++
		}
	}
	return n
}
