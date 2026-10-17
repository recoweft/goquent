package migration

import "slices"

// CoverageState describes supplied observations, never authority or freshness.
// The zero value and unrecognized values are unknown, not a known empty set.
type CoverageState string

const (
	CoverageUnknown     CoverageState = "unknown"
	CoverageComplete    CoverageState = "complete"
	CoveragePartial     CoverageState = "partial"
	CoverageUnsupported CoverageState = "unsupported"
)

// SchemaCoverage separates object definitions from collection completeness.
type SchemaCoverage struct {
	Tables      CoverageState `json:"tables,omitempty"`
	Columns     CoverageState `json:"columns,omitempty"`
	Indexes     CoverageState `json:"indexes,omitempty"`
	ForeignKeys CoverageState `json:"foreign_keys,omitempty"`
	Checks      CoverageState `json:"checks,omitempty"`
	Identity    CoverageState `json:"identity,omitempty"`
	Version     CoverageState `json:"version,omitempty"`
}

// ObjectIdentity retains namespace components without parsing qualified SQL.
// MySQL's catalog component is the literal "def" returned by information_schema.
type ObjectIdentity struct {
	Catalog string        `json:"catalog,omitempty"`
	Schema  string        `json:"schema,omitempty"`
	Name    string        `json:"name,omitempty"`
	State   CoverageState `json:"state,omitempty"`
}

// SchemaContext is sensitive source data. Reader labels and coverage declarations
// are not authenticated evidence, including after a source JSON roundtrip.
type SchemaContext struct {
	observedVersion string
	Dialect         string         `json:"dialect,omitempty"`
	Catalog         string         `json:"catalog,omitempty"`
	Schema          string         `json:"schema,omitempty"`
	ServerVersion   string         `json:"server_version,omitempty"`
	VersionSource   string         `json:"version_source,omitempty"`
	Scope           string         `json:"scope,omitempty"`
	Tables          []string       `json:"tables,omitempty"`
	Visibility      string         `json:"visibility,omitempty"`
	Coverage        SchemaCoverage `json:"coverage,omitempty"`
}

// ExpressionDefinition distinguishes absent SQL from an unobserved expression.
type ExpressionDefinition struct {
	State string `json:"state,omitempty"`
	SQL   string `json:"sql,omitempty"`
}

// IndexKey is a finite column/expression union. Empty attributes are unknown.
type IndexKey struct {
	Column     string `json:"column,omitempty"`
	Expression string `json:"expression,omitempty"`
	Order      string `json:"order,omitempty"`
	Nulls      string `json:"nulls,omitempty"`
	Prefix     int    `json:"prefix,omitempty"`
}

// ForeignKeySchema preserves ordered reference pairs and catalog declarations.
// Referenced objects outside the export scope have not been inspected.
type ForeignKeySchema struct {
	Name              string         `json:"name"`
	Columns           []string       `json:"columns,omitempty"`
	Reference         ObjectIdentity `json:"reference"`
	ReferencedColumns []string       `json:"referenced_columns,omitempty"`
	OnUpdate          string         `json:"on_update,omitempty"`
	OnDelete          string         `json:"on_delete,omitempty"`
	Match             string         `json:"match,omitempty"`
	Validation        string         `json:"validation,omitempty"`
	Enforcement       string         `json:"enforcement,omitempty"`
	Deferrability     string         `json:"deferrability,omitempty"`
	Definition        CoverageState  `json:"definition,omitempty"`
	RawDefinition     string         `json:"raw_definition,omitempty"`
}

// CheckSchema compares catalog SQL byte for byte, not SQL meaning.
type CheckSchema struct {
	Inheritance   string        `json:"inheritance,omitempty"`
	Name          string        `json:"name"`
	Expression    string        `json:"expression,omitempty"`
	Validation    string        `json:"validation,omitempty"`
	Enforcement   string        `json:"enforcement,omitempty"`
	Definition    CoverageState `json:"definition,omitempty"`
	RawDefinition string        `json:"raw_definition,omitempty"`
}

func copyContext(c *SchemaContext) *SchemaContext {
	if c == nil {
		return nil
	}
	n := *c
	n.Tables = slices.Clone(c.Tables)
	return &n
}
func copySchema(s Schema) *Schema {
	n := s
	n.Context = copyContext(s.Context)
	n.Tables = slices.Clone(s.Tables)
	for i := range n.Tables {
		n.Tables[i] = *copyTable(n.Tables[i])
	}
	return &n
}
func identityKnown(i ObjectIdentity) bool {
	return i.State == CoverageComplete && simpleSchemaName.MatchString(i.Catalog) && simpleSchemaName.MatchString(i.Schema) && simpleSchemaName.MatchString(i.Name)
}
func expressionKnown(e ExpressionDefinition) bool {
	return e.State == "none" && e.SQL == "" || e.State == "expression" && e.SQL != ""
}
func coverageKnown(c SchemaCoverage) bool {
	return c.Columns == CoverageComplete && c.Indexes == CoverageComplete && c.ForeignKeys == CoverageComplete && c.Checks == CoverageComplete && c.Identity == CoverageComplete
}
