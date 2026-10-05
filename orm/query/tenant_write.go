package query

import (
	"reflect"

	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/internal/valueguard"
)

func (q *Query) tenantRows(rows []map[string]any, update bool, unique, updateCols []string) ([]map[string]any, error) {
	if !q.settings.strict {
		return rows, nil
	}
	if valueguard.Check(rows) != nil {
		return nil, tenantError("write_value_budget")
	}
	p := &QueryPlan{Tables: []TableRef{{Name: q.tableName()}}}
	ts, err := q.tenantTargets(p)
	if err != nil {
		return nil, err
	}
	t := ts[0]
	if t.alias != "" || len(rows) == 0 {
		return nil, tenantError("write_target_or_rows_unsupported")
	}
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = make(map[string]any, len(row)+1)
		for col, v := range row {
			if !scopeIdent(col) {
				return nil, tenantError("assignment_expression_unsupported")
			}
			if _, ok := tenantColumn(t, col); !ok {
				return nil, tenantError("assignment_column_unknown")
			}
			if !scopeScalar(v) {
				return nil, tenantError("assignment_value_unsupported")
			}
			if err := tenantWriteColumn(t, col, update); err != nil {
				return nil, err
			}
			out[i][col] = v
		}
		if !update && t.policy.TenantColumn != "" {
			col := t.policy.TenantColumn
			if _, ok := out[i][col]; !ok && q.settings.autoTenant {
				out[i][col] = q.settings.execution.input.CurrentTenant
			}
			c, _ := tenantColumn(t, col)
			got, ok := scopeInteger(out[i][col], c.Bits)
			want, _ := scopeInteger(q.settings.execution.input.CurrentTenant, c.Bits)
			if !ok || got != want {
				return nil, tenantError("insert_tenant_missing_or_mismatched")
			}
		}
		if i > 0 && !reflect.DeepEqual(sortedMapKeys(out[0]), sortedMapKeys(out[i])) {
			return nil, tenantError("batch_columns_mismatch")
		}
	}
	if unique != nil || updateCols != nil {
		if len(unique) == 0 || len(updateCols) == 0 || !t.schema.CompleteUniqueConstraints || len(t.schema.Constraints) == 0 {
			return nil, tenantError("conflict_constraints_incomplete")
		}
		for _, col := range updateCols {
			if err := tenantWriteColumn(t, col, true); err != nil {
				return nil, err
			}
			if !scopeIdent(col) {
				return nil, tenantError("conflict_update_unknown")
			}
			if _, ok := out[0][col]; !ok {
				return nil, tenantError("conflict_update_missing_value")
			}
		}
		found := false
		for _, k := range t.schema.Constraints {
			if !validWriteKey(k, q.settings.schema.input.Dialect) {
				return nil, tenantError("conflict_constraint_unsupported")
			}
			var names []string
			tenant := false
			for _, c := range k.Columns {
				actual, ok := tenantColumn(t, c.Name)
				if !ok || actual != c || c.Nullable {
					return nil, tenantError("conflict_column_schema_mismatch")
				}
				names = append(names, c.Name)
				tenant = tenant || c.Name == t.policy.TenantColumn
			}
			matches := sameTenantNames(names, unique)
			found = found || matches
			if q.settings.schema.input.Dialect == "postgres" && !matches {
				continue
			}
			if t.policy.TenantColumn != "" && !tenant {
				return nil, tenantError("cross_tenant_conflict")
			}
			for _, row := range out {
				for _, c := range k.Columns {
					if _, ok := scopeInteger(row[c.Name], c.Bits); !ok {
						return nil, tenantError("conflict_value_missing_or_unsupported")
					}
				}
			}
		}
		if !found {
			return nil, tenantError("conflict_target_unknown")
		}
	}
	// All entries are supported immutable scalar values; containers are detached.
	return out, nil
}
func sameTenantNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]bool{}
	for _, v := range a {
		seen[v] = true
	}
	for _, v := range b {
		if !seen[v] {
			return false
		}
		delete(seen, v)
	}
	return len(seen) == 0
}
func tenantWriteColumn(t tenantTarget, col string, update bool) error {
	for _, bad := range append(append([]string{}, t.policy.ForbiddenColumns...), t.policy.PIIColumns...) {
		if col == bad {
			return tenantError("protected_write_column")
		}
	}
	if update {
		if col == t.policy.TenantColumn {
			return tenantError("tenant_assignment_forbidden")
		}
		for _, bad := range t.policy.ImmutableColumns {
			if col == bad {
				return tenantError("immutable_assignment")
			}
		}
	}
	return nil
}

// planReturning checks the final generated projection before resealing this
// internally generated plan. Generic/scoped RETURNING remains GQ-AI-04.
func (q *Query) planReturning(p *QueryPlan, cols []string) error {
	if q.settings.strict {
		if err := checkTenantEvidence(p); err != nil {
			return err
		}
		ts, err := q.tenantTargets(p)
		if err != nil {
			return err
		}
		if err := tenantProjection(columnRefsFromNames(cols), ts); err != nil {
			return err
		}
	}
	for i, col := range cols {
		if i == 0 {
			p.SQL += " RETURNING "
		} else {
			p.SQL += ", "
		}
		p.SQL += q.dialect.QuoteIdent(col)
	}
	if q.settings.strict {
		p.tenantEvidence.view.SQL = p.SQL
		p.tenantEvidence.view.Params = valuecopy.Slice(p.Params)
	}
	return ensurePlanExecutable(p)
}
