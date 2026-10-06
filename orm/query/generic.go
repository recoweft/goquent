package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

func init() { querybridge.Prepare = prepareGeneric }

// prepareGeneric is the sole internal facade adapter. It creates a fresh Query
// owned by the destination settings/executor, then uses the private lifecycle.
func prepareGeneric(r querybridge.Request) (querybridge.Planned, error) {
	var out querybridge.Planned
	settings, ok := r.Settings.(Settings)
	if !ok {
		return out, fmt.Errorf("%w: invalid internal settings", ErrBlockedOperation)
	}
	q := NewWithSettings(r.Executor, r.Table, r.Dialect, settings).WithContext(r.Context)
	if r.Base != nil {
		base, ok := r.Base.(*Query)
		if !ok || base == nil {
			return out, fmt.Errorf("base query is nil")
		}
		copy := *base
		q = &copy
		q.settings = settings
		q.exec = r.Executor
		q.ctx = r.Context
		q.dialect = r.Dialect
		q.builder = newSelectBuilder(r.Dialect)
		base.builder.CopyStateToSelect(q.builder)
		q.policy = nil
		q.writeKeys = nil
		if p, ok := settings.PolicySet().PolicyForTable(q.tableName()); ok {
			q.policy = &p
		}
		if settings.Err() != nil {
			q.err = settings.Err()
		}
	}
	if q.err != nil {
		return out, q.err
	}
	for i, col := range r.WhereColumns {
		if q.settings.strict && !scopeIdent(col) {
			return out, tenantError("literal_predicate_identifier_unsupported")
		}
		q.builder.WhereLiteral(col, r.WhereValues[i])
	}
	var p *QueryPlan
	var err error
	switch r.Operation {
	case "select":
		p, err = q.Plan(r.Context)
	case "update":
		if len(r.Rows) != 1 {
			return out, fmt.Errorf("invalid update rows")
		}
		p, err = q.planUpdateValues(r.Context, r.Rows[0], r.Options)
	case "insert", "upsert":
		p, err = q.planInsertRows(r.Context, r.Rows, r.Options, r.Operation, r.Batch || len(r.Rows) > 1)
	default:
		return out, fmt.Errorf("unsupported internal operation")
	}
	if err != nil {
		return out, err
	}
	if len(r.Returning) > 0 {
		if err = q.planReturning(p, r.Returning); err != nil {
			return out, err
		}
	}
	out.Diagnostic = p
	out.Exec = func() (sql.Result, error) { return q.executeResult(p) }
	out.Scan = func(scan func(*sql.Rows) error) error { return q.executeRows(p, scan) }
	return out, nil
}

func (q *Query) planInsertRows(ctx context.Context, rows []map[string]any, o writeinput.Options, mode string, batch bool) (*QueryPlan, error) {
	if q.err != nil {
		return nil, q.err
	}
	o = o.Clone()
	if q.settings.strict {
		for _, part := range o.TableParts {
			if !scopeIdent(part) {
				return nil, tenantError("table_or_alias_unsupported")
			}
		}
		if len(o.Assignments) > 0 {
			return nil, tenantError("assignment_expression_unsupported")
		}
		if c := o.Conflict; c != nil && (c.Where != "" || c.Constraint != "" || c.RawTarget != "") {
			return nil, tenantError("conflict_target_unsupported")
		}
	}
	var unique, updates []string
	if c := o.Conflict; c != nil && !c.DoNothing {
		unique, updates = c.Columns, c.Updates
		if q.settings.strict && (len(unique) == 0 || len(updates) == 0) {
			return nil, tenantError("conflict_target_or_updates_missing")
		}
	}
	rows, err := q.tenantRows(rows, false, unique, updates)
	if err != nil {
		return nil, err
	}
	if c := o.Conflict; c != nil && c.ExplicitColumns && len(rows) > 0 {
		for _, col := range c.Columns {
			if _, ok := rows[0][col]; !ok {
				return nil, fmt.Errorf("ConflictColumns requires column %s", col)
			}
		}
	}
	// Automatic tenant fill changes the final column list as well as candidates.
	if len(o.Columns) > 0 && len(rows) > 0 {
		for _, col := range sortedMapKeys(rows[0]) {
			found := false
			for _, old := range o.Columns {
				found = found || old == col
			}
			if !found {
				o.Columns = append(o.Columns, col)
			}
		}
	}
	ib := newInsertBuilder(q.dialect)
	ib.Table(q.tableName())
	ib.WriteOptions(o)
	if mode == "ignore" {
		ib.InsertOrIgnore(rows)
	} else if len(rows) == 1 && !batch && o.Conflict == nil {
		ib.Insert(rows[0])
	} else {
		ib.InsertBatch(rows)
	}
	sql, args, snapshot, err := ib.BuildSnapshot()
	if err != nil {
		return nil, err
	}
	p := newQueryPlan(OperationInsert, sql, args)
	p.Tables = []TableRef{{Name: snapshot.Table}}
	p.Columns = columnRefsFromNames(snapshot.Columns)
	p.Metadata = map[string]any{}
	if batch {
		p.Metadata["batch_size"] = snapshot.BatchSize
	}
	if snapshot.Mode != "" {
		p.Metadata["insert_mode"] = snapshot.Mode
	}
	if snapshot.Mode == "upsert" {
		p.Metadata["unique_columns"] = snapshot.UniqueColumns
		p.Metadata["update_columns"] = snapshot.UpdateColumns
	}
	p.Unverified = append(p.Unverified, snapshot.Unverified...)
	q.finalizePlan(p)
	return p, nil
}

func (q *Query) planUpdateValues(ctx context.Context, m map[string]any, o writeinput.Options) (*QueryPlan, error) {
	if q.err != nil {
		return nil, q.err
	}
	o = o.Clone()
	if q.settings.strict {
		for _, part := range o.TableParts {
			if !scopeIdent(part) {
				return nil, tenantError("table_or_alias_unsupported")
			}
		}
		if len(o.Assignments) > 0 {
			return nil, tenantError("assignment_expression_unsupported")
		}
	}
	rows, err := q.tenantRows([]map[string]any{m}, true, nil, nil)
	if err != nil {
		return nil, err
	}
	ub := newUpdateBuilder(q.dialect)
	ub.Table(q.tableName()).Update(rows[0])
	ub.WriteOptions(o)
	source, err := q.policyBuilder(q.builder)
	if err != nil {
		return nil, err
	}
	copyBuilderState(source, ub)
	sql, args, snapshot, err := ub.BuildSnapshot()
	if err != nil {
		return nil, err
	}
	p := newQueryPlan(OperationUpdate, sql, args)
	appendTableRef(p, snapshot.Table, "")
	p.Columns = columnRefsFromNames(snapshot.AssignmentColumns)
	appendWriteSnapshotMetadata(p, snapshot)
	// Literal generic identifiers are not interpreted as JSON paths or expressions.
	if o.Literal {
		for _, col := range snapshot.AssignmentColumns {
			if strings.Contains(col, "->") {
				p.Unverified = append(p.Unverified, "literal_assignment_identifier")
			}
		}
	}
	q.finalizePlan(p)
	return p, nil
}
