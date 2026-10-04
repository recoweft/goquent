package query

import (
	"math"
	"reflect"
	"strings"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/internal/valueguard"
	"github.com/recoweft/goquent/orm/predicate"
)

// WriteKeyColumn declares a database integer column, not an inferred Go model type.
// Supported DBType names are documented in docs/write-scope.md.
type WriteKeyColumn struct {
	Name     string
	DBType   string
	Bits     int
	Unsigned bool
	Nullable bool
}

// WriteKeyConstraint is an application assertion about an entire unique key.
// AllRows, Valid and NotDeferrable must be explicitly asserted. Partial and
// expression indexes and deferred constraints cannot establish a bound.
type WriteKeyConstraint struct {
	Name          string
	Kind          string // "primary" or "unique"
	Columns       []WriteKeyColumn
	AllRows       bool
	Valid         bool
	NotDeferrable bool
	Partial       bool
	Expression    bool
}

// WriteKeyContext must originate in trusted application code for this Query's
// current executor/database. Database is the application's database identity;
// Goquent does not read or authenticate it. Table includes any required schema.
// Alias must match the target alias exactly (empty for an unaliased target).
// This assertion is not live schema verification, tenant binding or authorization.
type WriteKeyContext struct {
	invalid     string
	Database    string
	Dialect     string // "mysql" or "postgres"
	Table       string
	Alias       string
	Constraints []WriteKeyConstraint
}

// WithWriteKeyContext copies explicit, caller-asserted key facts for this query.
// Do not construct this context directly from untrusted requests or plan JSON.
// It never reads a database or invokes custom values/Executor methods.
func (q *Query) WithWriteKeyContext(ctx WriteKeyContext) *Query {
	q.writeKeys = copyWriteKeys(ctx)
	return q
}

func copyWriteKeys(ctx WriteKeyContext) *WriteKeyContext {
	// Bound work before copying. An invalid sentinel preserves unknown on overflow.
	if len(ctx.Constraints) > 64 || len(ctx.Database)+len(ctx.Dialect)+len(ctx.Table)+len(ctx.Alias) > 4096 {
		return &WriteKeyContext{invalid: "key_context_budget_exceeded"}
	}
	n := 0
	for _, k := range ctx.Constraints {
		n += len(k.Columns)
		if n > 256 || len(k.Name) > 256 {
			return &WriteKeyContext{invalid: "key_context_budget_exceeded"}
		}
		for _, c := range k.Columns {
			if len(c.Name)+len(c.DBType) > 512 {
				return &WriteKeyContext{invalid: "key_context_budget_exceeded"}
			}
		}
	}
	out := ctx
	out.Constraints = append([]WriteKeyConstraint(nil), ctx.Constraints...)
	for i := range out.Constraints {
		out.Constraints[i].Columns = append([]WriteKeyColumn(nil), ctx.Constraints[i].Columns...)
	}
	return &out
}

// WriteScopeResult is conditional structural analysis, never authorization or
// measured affected rows. Re-run AnalyzeWriteScope after editing public fields.
type WriteScopeResult struct {
	Status      string            `json:"status"` // at_most_one, broad, unknown
	Reason      string            `json:"reason"`
	Precision   AnalysisPrecision `json:"precision"`
	Provenance  string            `json:"provenance"`
	Assumptions []string          `json:"assumptions,omitempty"`
	Key         string            `json:"key,omitempty"`
	Database    string            `json:"asserted_database,omitempty"`
	Dialect     string            `json:"dialect,omitempty"`
	Table       string            `json:"table,omitempty"`
	Alias       string            `json:"alias,omitempty"`
	KeyColumns  []string          `json:"key_columns,omitempty"`
}

type writeEvidence struct {
	Operation  OperationType
	SQL        string
	Params     []any
	Tables     []TableRef
	Columns    []ColumnRef
	Joins      []JoinRef
	Where      *predicate.Node
	Having     *predicate.Node
	Unverified []string
	keys       *WriteKeyContext
	dialect    string
	failure    string
}

func writeView(p *QueryPlan) *writeEvidence {
	return &writeEvidence{Operation: p.Operation, SQL: p.SQL, Params: p.Params, Tables: p.Tables, Columns: p.Columns, Joins: p.Joins, Where: p.WhereTree, Having: p.HavingTree, Unverified: p.Unverified}
}

// sealWriteEvidence is called only at the builder boundary, before risk checking.
func (q *Query) sealWriteEvidence(p *QueryPlan) {
	if p.Operation != OperationUpdate && p.Operation != OperationDelete {
		return
	}
	e := writeView(p)
	switch q.dialect.(type) {
	case driver.MySQLDialect:
		e.dialect = "mysql"
	case driver.PostgresDialect:
		e.dialect = "postgres"
	}
	if q.writeKeys != nil {
		e.keys = copyWriteKeys(*q.writeKeys)
	}
	if reason := comparableWriteView(e); reason != "" {
		e.failure = reason
		p.writeEvidence = e
		return
	}
	e.Params = valuecopy.Slice(e.Params)
	e.Where = valuecopy.Node(e.Where)
	e.Having = valuecopy.Node(e.Having)
	e.Tables = append([]TableRef(nil), e.Tables...)
	e.Columns = append([]ColumnRef(nil), e.Columns...)
	e.Joins = append([]JoinRef(nil), e.Joins...)
	e.Unverified = append([]string(nil), e.Unverified...)
	p.writeEvidence = e
}

func comparableWriteView(e *writeEvidence) string {
	if valueguard.Check(e) != nil {
		return "correspondence_budget_or_cycle"
	}
	for _, v := range e.Params {
		if !scopeScalar(v) {
			return "parameter_not_comparable"
		}
	}
	var walk func(*predicate.Node) bool
	walk = func(n *predicate.Node) bool {
		if n == nil {
			return true
		}
		if len(n.Parameters) != len(n.Values) {
			return false
		}
		for i, v := range n.Values {
			index := n.Parameters[i]
			if index < 0 || index >= len(e.Params) || !reflect.DeepEqual(v.Data, e.Params[index]) {
				return false
			}
			if v.Isolation != "detached" || !scopeScalar(v.Data) {
				return false
			}
		}
		for _, v := range n.NamedValues {
			if v.Isolation != "detached" || !scopeScalar(v.Data) {
				return false
			}
		}
		for _, c := range n.Children {
			if !walk(c) {
				return false
			}
		}
		return true
	}
	if !walk(e.Where) || !walk(e.Having) {
		return "condition_value_not_comparable"
	}
	return ""
}
func scopeScalar(v any) bool {
	switch v.(type) {
	case nil, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, string, bool, float32, float64:
		return true
	}
	return false
}

// AnalyzeWriteScope rechecks correspondence against private builder evidence.
// Public result fields, Metadata, PrimaryKey and JSON cannot supply that evidence.
// Built-in scalar parameters are compared without invoking driver.Valuer or any
// user method. Custom payloads and concurrent mutation are outside this proof.
func AnalyzeWriteScope(p *QueryPlan) WriteScopeResult {
	r := WriteScopeResult{Status: "unknown", Precision: AnalysisPartial, Provenance: "none"}
	unknown := func(reason string) WriteScopeResult { r.Reason = reason; return r }
	if p == nil || p.writeEvidence == nil {
		return unknown("builder_evidence_missing")
	}
	e := p.writeEvidence
	r.Provenance = "builder_and_application_assertion"
	r.Assumptions = []string{"caller asserts current database identity and valid uniqueness over all target rows", "schema freshness is not verified", "executor preserves database/sql integer parameter semantics"}
	if e.failure != "" {
		return unknown(e.failure)
	}
	view := writeView(p)
	if reason := comparableWriteView(view); reason != "" {
		return unknown(reason)
	}
	expected := *e
	expected.keys = nil
	expected.dialect = ""
	expected.failure = ""
	if !reflect.DeepEqual(view, &expected) {
		return unknown("sql_values_tree_or_target_changed")
	}
	if e.Operation != OperationUpdate && e.Operation != OperationDelete {
		return unknown("not_a_write")
	}
	if len(e.Unverified) > 0 || len(e.Joins) > 0 || e.Having != nil {
		return unknown("statement_not_fully_inspected")
	}
	c := e.keys
	if c == nil {
		return unknown("key_context_missing")
	}
	if c.invalid != "" {
		return unknown(c.invalid)
	}
	if strings.TrimSpace(c.Database) == "" || c.Dialect != e.dialect || e.dialect == "" || len(e.Tables) != 1 {
		return unknown("database_dialect_or_target_context_invalid")
	}
	table, alias := splitTableAlias(e.Tables[0].Name)
	if e.Tables[0].Alias != "" {
		alias = e.Tables[0].Alias
	}
	if !scopePath(table) || (alias != "" && !scopeIdent(alias)) || c.Table != table || c.Alias != alias || len(c.Constraints) == 0 {
		return unknown("table_alias_or_key_context_invalid")
	}
	r.Database, r.Dialect, r.Table, r.Alias = c.Database, c.Dialect, c.Table, c.Alias
	// Validate all supplied facts before choosing any key. Invalid assertions cannot
	// be hidden by another usable key.
	for _, k := range c.Constraints {
		if !validWriteKey(k, c.Dialect) {
			return unknown("constraint_or_integer_type_unsupported")
		}
	}
	if e.Where == nil {
		r.Status = "broad"
		r.Reason = "no_where"
		r.Precision = AnalysisPrecise
		return r
	}
	best := ""
	var bestColumns []string
	for _, k := range c.Constraints {
		bindings, reason := scopeBindings(e.Where, k, table, alias)
		if reason != "" {
			return unknown(reason)
		}
		if len(bindings) == len(k.Columns) {
			bestColumns = nil
			for _, col := range k.Columns {
				bestColumns = append(bestColumns, col.Name)
			}
			best = k.Name
			if best == "" {
				best = k.Kind
			}
		}
	}
	r.Precision = AnalysisPrecise
	if best != "" {
		r.Status = "at_most_one"
		r.Reason = "complete_integer_key_binding"
		r.Key = best
		r.KeyColumns = bestColumns
	} else {
		r.Status = "broad"
		r.Reason = "no_complete_common_key_binding"
	}
	return r
}

func scopeIdent(s string) bool {
	if s == "" || len(s) > 256 {
		return false
	}
	for i, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
func scopePath(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if !scopeIdent(p) {
			return false
		}
	}
	return true
}
func scopeColumn(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if !scopeIdent(p) {
			return false
		}
	}
	return true
}

func scopeTargetColumn(s, table, alias string) bool {
	if !scopeColumn(s) {
		return false
	}
	if i := strings.LastIndex(s, "."); i >= 0 {
		target := table
		if alias != "" {
			target = alias
		}
		return s[:i] == target
	}
	return true
}

func validWriteKey(k WriteKeyConstraint, dialect string) bool {
	if (k.Kind != "primary" && k.Kind != "unique") || !k.AllRows || !k.Valid || !k.NotDeferrable || k.Partial || k.Expression || len(k.Columns) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, c := range k.Columns {
		if !scopeIdent(c.Name) || seen[c.Name] || c.Unsigned || k.Kind == "primary" && c.Nullable {
			return false
		}
		seen[c.Name] = true
		bits := 0
		switch dialect {
		case "mysql":
			switch c.DBType {
			case "SMALLINT":
				bits = 16
			case "INT":
				bits = 32
			case "BIGINT":
				bits = 64
			}
		case "postgres":
			switch c.DBType {
			case "smallint":
				bits = 16
			case "integer":
				bits = 32
			case "bigint":
				bits = 64
			}
		}
		if bits == 0 || bits != c.Bits {
			return false
		}
	}
	return true
}
func scopeInteger(v any, bits int) (int64, bool) {
	var x int64
	switch n := v.(type) {
	case int:
		x = int64(n)
	case int8:
		x = int64(n)
	case int16:
		x = int64(n)
	case int32:
		x = int64(n)
	case int64:
		x = n
	case uint:
		if uint64(n) > math.MaxInt64 {
			return 0, false
		}
		x = int64(n)
	case uint8:
		x = int64(n)
	case uint16:
		x = int64(n)
	case uint32:
		x = int64(n)
	case uint64:
		if n > math.MaxInt64 {
			return 0, false
		}
		x = int64(n)
	default:
		return 0, false
	}
	if bits < 64 && (x < -(int64(1)<<(bits-1)) || x > (int64(1)<<(bits-1))-1) {
		return 0, false
	}
	return x, true
}

func scopeBindings(n *predicate.Node, k WriteKeyConstraint, table, alias string) (map[string]int64, string) {
	out := map[string]int64{}
	if n == nil || n.Correspondence != "generated" || n.OpaqueReason != "" {
		return out, "condition_unverified"
	}
	switch n.Kind {
	case "and", "or", "group", "not":
		for i, c := range n.Children {
			b, reason := scopeBindings(c, k, table, alias)
			if reason != "" {
				return out, reason
			}
			if n.Kind == "not" {
				continue
			}
			if n.Kind == "or" {
				if i == 0 {
					if len(b) == len(k.Columns) {
						out = b
					}
				} else {
					for col, v := range out {
						if w, ok := b[col]; !ok || w != v {
							delete(out, col)
						}
					}
				}
			} else {
				for col, v := range b {
					if old, ok := out[col]; ok && old != v {
						return nil, "conflicting_key_bindings"
					}
					out[col] = v
				}
			}
		}
		if n.Kind == "or" && len(out) != len(k.Columns) {
			clear(out)
		}
		return out, ""
	case "null", "column":
		if !scopeTargetColumn(n.Column, table, alias) || n.ValueColumn != "" && !scopeTargetColumn(n.ValueColumn, table, alias) {
			return out, "column_expression_unsupported"
		}
		return out, ""
	case "comparison", "in", "between":
		for _, bound := range n.BoundColumns {
			if !scopeTargetColumn(bound, table, alias) {
				return out, "column_target_unresolved"
			}
		}
		if !scopeColumn(n.Column) {
			return out, "column_expression_unsupported"
		}
		qualifier, col := "", n.Column
		if i := strings.LastIndex(col, "."); i >= 0 {
			qualifier, col = col[:i], col[i+1:]
		}
		target := table
		if alias != "" {
			target = alias
		}
		if qualifier != "" && qualifier != target {
			return out, "column_target_unresolved"
		}
		var kc *WriteKeyColumn
		for i := range k.Columns {
			if k.Columns[i].Name == col {
				kc = &k.Columns[i]
				break
			}
		}
		if kc == nil {
			return out, ""
		}
		for _, v := range n.Values {
			if v.Data != nil {
				if _, ok := scopeInteger(v.Data, kc.Bits); !ok {
					return out, "key_value_type_or_range_unsupported"
				}
			}
		}
		if len(n.Values) != 1 || n.Values[0].Data == nil {
			return out, ""
		}
		if n.Kind == "comparison" && n.Operator == "=" || n.Kind == "in" && strings.EqualFold(n.Operator, "IN") {
			v, ok := scopeInteger(n.Values[0].Data, kc.Bits)
			if ok {
				out[col] = v
			}
		}
		return out, ""
	default:
		return out, "condition_kind_unsupported"
	}
}
