package query

import (
	"fmt"
	"strings"
)

// NewApplicationTenantContext records a trusted application's explicit supply
// after authentication and authorization. It does not authenticate that caller.
// Never call it merely to deserialize an operation/request's tenant claim.
func NewApplicationTenantContext(input ExecutionContextInput) (ExecutionContext, error) {
	c, err := NewExecutionContext(input)
	if err != nil {
		return c, err
	}
	c.application = true
	return c, nil
}

// ApplicationSchemaInput is a trusted application's assertion for the executor
// configured under Database. Neither identity nor schema freshness is read.
// It must not be reconstructed from operation JSON or manifest verdicts.
type ApplicationSchemaInput struct {
	Database string
	Dialect  string
	Tables   []ApplicationTable
}

// ApplicationTable declares the column inventory and conflict constraints.
// Tenant and conflict columns currently support signed SQL integers only.
// Other DBType values may describe non-tenant, non-conflict columns.
type ApplicationTable struct {
	// PlainTable asserts a base table with no trigger/rule/generated-column rewrite
	// of supplied writes or hidden conflict constraints. Views are unsupported.
	PlainTable                bool
	Table                     string
	Columns                   []WriteKeyColumn
	Constraints               []WriteKeyConstraint
	CompleteUniqueConstraints bool
}

// ApplicationSchema is an immutable, explicitly supplied assertion, not live evidence.
type ApplicationSchema struct{ input *ApplicationSchemaInput }

func NewApplicationSchema(input ApplicationSchemaInput) (ApplicationSchema, error) {
	fail := func() (ApplicationSchema, error) {
		return ApplicationSchema{}, fmt.Errorf("goquent: invalid or oversized application schema")
	}
	if strings.TrimSpace(input.Database) == "" || len(input.Database) > 256 || (input.Dialect != "mysql" && input.Dialect != "postgres") || len(input.Tables) == 0 || len(input.Tables) > 64 {
		return fail()
	}
	out := input
	out.Tables = append([]ApplicationTable(nil), input.Tables...)
	seen := map[string]bool{}
	total := 0
	for i, t := range input.Tables {
		if !scopePath(t.Table) || seen[t.Table] || len(t.Columns) == 0 || len(t.Columns) > 256 {
			return fail()
		}
		seen[t.Table] = true
		total += len(t.Columns)
		if total > 4096 {
			return fail()
		}
		names := map[string]bool{}
		for _, c := range t.Columns {
			if !scopeIdent(c.Name) || names[c.Name] || len(c.DBType) > 64 {
				return fail()
			}
			names[c.Name] = true
		}
		keys := copyWriteKeys(WriteKeyContext{Constraints: t.Constraints})
		if keys.invalid != "" {
			return fail()
		}
		out.Tables[i].Columns = append([]WriteKeyColumn(nil), t.Columns...)
		out.Tables[i].Constraints = keys.Constraints
	}
	return ApplicationSchema{input: &out}, nil
}

func (s ApplicationSchema) Input() ApplicationSchemaInput {
	if s.input == nil {
		return ApplicationSchemaInput{}
	}
	c, _ := NewApplicationSchema(*s.input)
	return *c.input
}

// WithTenantPolicy enables PR2's conditional strict gate on documented query
// paths. Database is an application identity, never a connection attestation.
// Automatic binding affects the base WHERE only; JOIN ON remains caller-owned.
func (s Settings) WithTenantPolicy(database string, schema ApplicationSchema, automatic bool) Settings {
	s.strict = true
	s.autoTenant = automatic
	s.database = database
	s.schema = schema
	return s
}
