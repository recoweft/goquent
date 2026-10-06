package query

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/planidentity"
	"github.com/recoweft/goquent/orm/internal/planversion"
	"github.com/recoweft/goquent/orm/predicate"
)

// sealIdentity uses only the final private statement/inspection and owner input.
// Unavailable identity does not change the existing compatibility execution gate.
func (q *Query) sealIdentity(e *plannedExecution) {
	e.identity, e.identityErr = q.identitySnapshot(e)
}

func (q *Query) identitySnapshot(e *plannedExecution) (*planidentity.Snapshot, error) {
	fail := func() (*planidentity.Snapshot, error) { return nil, planidentity.ErrUnavailable }
	p, s := e.inspection, q.settings
	if p == nil || p.tenantEvidence == nil || p.tenantEvidence.failure != "" || p.tenantEvidence.view == nil || p.Operation == OperationRaw || len(p.Unverified) > 0 || s.err != nil || s.database == "" || s.schema.input == nil || s.database != s.schema.input.Database || !s.execution.application || !s.execution.input.TenantPresent {
		return fail()
	}
	dialect := ""
	switch q.dialect.(type) {
	case driver.MySQLDialect:
		dialect = "mysql"
	case driver.PostgresDialect:
		dialect = "postgres"
	default:
		return fail()
	}
	if dialect != s.schema.input.Dialect {
		return fail()
	}
	// Only generated trees are accepted. Values stay in ordered execution args;
	// opaque/raw conditions are never turned into a value-free shape by stripping.
	if !identityTree(p.WhereTree, 0) || !identityTree(p.HavingTree, 0) {
		return fail()
	}
	for _, j := range p.Joins {
		if j.Subquery || !identityTree(j.OnTree, 0) {
			return fail()
		}
	}
	shape, ok := identitySQLShape(e.sql, dialect)
	if !ok {
		return fail()
	}
	policies := s.policies.Policies()
	// Effective extra policy is already an owner snapshot, not public metadata.
	if q.policy != nil {
		found := false
		for i := range policies {
			if policies[i].Table == q.policy.Table {
				policies[i] = cloneTablePolicy(*q.policy)
				found = true
			}
		}
		if !found {
			policies = append(policies, cloneTablePolicy(*q.policy))
		}
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Table < policies[j].Table })
	for i := range policies {
		if planversion.Check(policies[i].Version) != nil {
			return fail()
		}
		policies[i].Version = JSONVersion
	}
	// These structs contain only known library fields. No arbitrary value,
	// marshaler, Valuer, Stringer or application callback is serialized here.
	policy, err := identityFields(policies)
	if err != nil {
		return fail()
	}
	schema, err := identityFields(s.schema.Input())
	if err != nil {
		return fail()
	}
	config, err := identityFields(struct {
		Strict, AutoTenant, WithDeleted, OnlyDeleted bool
		Risk                                         RiskConfig
		Required                                     []RequiredPredicate
		Keys                                         *WriteKeyContext
	}{s.strict, s.autoTenant, q.withDeleted, q.onlyDeleted, s.risk, q.requiredPredicates, q.writeKeys})
	if err != nil {
		return fail()
	}
	return planidentity.Seal(planidentity.Material{Dialect: dialect, Target: s.database, Operation: string(p.Operation), SQL: e.sql, Shape: shape, Args: e.args, Tenant: s.execution.input.CurrentTenant, TenantPresent: s.execution.input.TenantPresent, Policy: policy, Schema: schema, Config: config})
}

func identityFields(v any) (any, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return nil, e
	}
	var out any
	e = planversion.Decode(b, &out)
	return out, e
}

func identityTree(n *predicate.Node, depth int) bool {
	if n == nil {
		return true
	}
	if depth > planidentity.MaxDepth || n.Correspondence != "generated" || n.OpaqueReason != "" || n.Raw != "" || n.Function != "" || len(n.NamedValues) > 0 {
		return false
	}
	switch n.Kind {
	case "and", "or", "not", "group", "comparison", "in", "null", "between", "column":
	default:
		return false
	}
	for _, c := range n.Children {
		if !identityTree(c, depth+1) {
			return false
		}
	}
	return true
}

// identitySQLShape accepts a conservative generated SQL lexical domain only.
// Quoted literals, comments, expressions with unknown tokens, raw parameters and
// opaque snapshots are unavailable. LIMIT/OFFSET magnitudes are execution values;
// their position remains in the shape. No arbitrary SQL parser is claimed.
func identitySQLShape(sql, dialect string) (string, bool) {
	var out strings.Builder
	last := ""
	for i := 0; i < len(sql); {
		c := sql[i]
		if c == ' ' || c == '\n' || c == '\t' || c == '\r' {
			out.WriteByte(c)
			i++
			continue
		}
		if c == '`' || c == '"' {
			quote := c
			j := i + 1
			for j < len(sql) && sql[j] != quote {
				if !(sql[j] == '_' || sql[j] >= 'a' && sql[j] <= 'z' || sql[j] >= 'A' && sql[j] <= 'Z' || sql[j] >= '0' && sql[j] <= '9') {
					return "", false
				}
				j++
			}
			if j == len(sql) || j == i+1 {
				return "", false
			}
			out.WriteString(sql[i : j+1])
			i = j + 1
			last = "identifier"
			continue
		}
		if c == '?' && dialect == "mysql" {
			out.WriteByte(c)
			i++
			last = "parameter"
			continue
		}
		if c == '$' && dialect == "postgres" {
			j := i + 1
			for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
				j++
			}
			if j == i+1 {
				return "", false
			}
			out.WriteString(sql[i:j])
			i = j
			last = "parameter"
			continue
		}
		if c >= '0' && c <= '9' {
			if last != "LIMIT" && last != "OFFSET" {
				return "", false
			}
			j := i
			for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
				j++
			}
			out.WriteString("#value")
			i = j
			last = "value"
			continue
		}
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
			j := i
			for j < len(sql) && (sql[j] >= 'A' && sql[j] <= 'Z' || sql[j] >= 'a' && sql[j] <= 'z') {
				j++
			}
			word := sql[i:j]
			switch word {
			case "JOIN", "INNER", "LEFT", "SELECT", "DISTINCT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "IS", "NULL", "BETWEEN", "LIKE", "ORDER", "BY", "ASC", "DESC", "GROUP", "HAVING", "LIMIT", "OFFSET", "INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE", "ON", "CONFLICT", "DUPLICATE", "KEY", "DO", "NOTHING", "RETURNING", "AS", "EXCLUDED", "excluded", "IGNORE", "FOR", "SHARE", "NOWAIT", "SKIP", "LOCKED", "COUNT", "MIN", "MAX", "SUM", "AVG":
			default:
				return "", false
			}
			out.WriteString(word)
			i = j
			last = word
			continue
		}
		if strings.ContainsRune("(),.*=<>!", rune(c)) {
			out.WriteByte(c)
			i++
			continue
		}
		return "", false
	}
	return out.String(), true
}
