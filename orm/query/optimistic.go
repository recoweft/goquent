package query

import (
	"math"

	qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"
	"github.com/recoweft/goquent/orm/internal/writecheck"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"github.com/recoweft/goquent/orm/predicate"
)

// optimisticInput operates on the same private builder used for final SQL.
func (q *Query) optimisticInput(source *qbapi.SelectQueryBuilder, row map[string]any, o *writeinput.Options) (*qbapi.SelectQueryBuilder, map[string]any, error) {
	v := q.writeResult.Version
	if v == nil {
		return source, row, nil
	}
	fail := func() (*qbapi.SelectQueryBuilder, map[string]any, error) {
		return nil, nil, writecheck.ErrOptions
	}
	if q.writeResult.Validate() != nil || !q.settings.strict || len(o.Assignments) != 0 || !scopeIdent(v.Column) {
		return fail()
	}
	ts, err := q.tenantTargets(&QueryPlan{Tables: []TableRef{{Name: q.tableName()}}})
	if err != nil {
		return nil, nil, err
	}
	t := ts[0]
	if t.alias != "" || t.policy.TenantColumn == "" {
		return fail()
	}
	c, ok := tenantColumn(t, v.Column)
	key := WriteKeyConstraint{Kind: "primary", Columns: []WriteKeyColumn{c}, AllRows: true, Valid: true, NotDeferrable: true}
	if !ok || !validWriteKey(key, q.settings.schema.input.Dialect) || c.Nullable || c.Name == t.policy.TenantColumn || v.Expected == math.MaxInt64 {
		return fail()
	}
	if _, ok = scopeInteger(v.Expected+1, c.Bits); !ok {
		return fail()
	}
	probe := newUpdateBuilder(q.dialect)
	probe.Table(q.tableName()).Update(row)
	probe.WriteOptions(*o)
	copyBuilderState(source, probe)
	_, _, snapshot, err := probe.BuildSnapshot()
	if err != nil {
		return nil, nil, err
	}
	if len(snapshot.Joins) != 0 || snapshot.HavingTree != nil || len(snapshot.Unverified) != 0 {
		return fail()
	}
	equals := map[string]any{}
	var walk func(*predicate.Node) bool
	walk = func(n *predicate.Node) bool {
		if n == nil || n.Correspondence != "generated" || n.OpaqueReason != "" {
			return false
		}
		if n.Kind == "and" || n.Kind == "group" {
			if len(n.Children) == 0 {
				return false
			}
			for _, ch := range n.Children {
				if !walk(ch) {
					return false
				}
			}
			return true
		}
		if n.Kind != "comparison" || n.Operator != "=" || len(n.Values) != 1 || n.Values[0].Isolation != "detached" {
			return false
		}
		_, col, ok := tenantResolve(n.Column, ts)
		if !ok || col == v.Column {
			return false
		}
		if _, exists := equals[col]; exists {
			return false
		}
		equals[col] = n.Values[0].Data
		return true
	}
	if !walk(snapshot.WhereTree) {
		return fail()
	}
	tenant, _ := tenantColumn(t, t.policy.TenantColumn)
	got, ok := scopeInteger(equals[tenant.Name], tenant.Bits)
	want, _ := scopeInteger(q.settings.execution.input.CurrentTenant, tenant.Bits)
	if !ok || got != want {
		return fail()
	}
	protected := map[string]bool{tenant.Name: true, v.Column: true}
	found := false
	for _, k := range t.schema.Constraints {
		if k.Kind != "primary" {
			continue
		}
		if found || !validWriteKey(k, q.settings.schema.input.Dialect) {
			return fail()
		}
		found = true
		for _, kc := range k.Columns {
			actual, ok := tenantColumn(t, kc.Name)
			if !ok || actual != kc || kc.Name == v.Column {
				return fail()
			}
			if _, ok := scopeInteger(equals[kc.Name], kc.Bits); !ok {
				return fail()
			}
			protected[kc.Name] = true
		}
	}
	if !found {
		return fail()
	}
	out := make(map[string]any, len(row)+1)
	for col, val := range row {
		if protected[col] {
			return fail()
		}
		out[col] = val
	}
	out[v.Column] = v.Expected + 1
	if len(o.Columns) > 0 {
		o.Columns = append(o.Columns, v.Column)
	}
	copied := newSelectBuilder(q.dialect)
	source.CopyStateToSelect(copied)
	copied.WhereLiteral(v.Column, v.Expected)
	return copied, out, nil
}
