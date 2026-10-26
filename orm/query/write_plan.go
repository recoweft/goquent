package query

import (
	"context"

	"github.com/recoweft/goquent/orm/conv"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

func dataToMap(data any) (map[string]any, error) {
	if m, ok := data.(map[string]any); ok {
		return m, nil
	}
	return conv.StructToMap(data)
}

// PlanInsert builds an INSERT plan for data without executing it.
func (q *Query) PlanInsert(ctx context.Context, data any) (*QueryPlan, error) {
	if err := q.checkExactWrite(); err != nil {
		return nil, err
	}
	if q.err != nil {
		return nil, q.err
	}
	m, err := dataToMap(data)
	if err != nil {
		return nil, err
	}
	return q.planInsertRows(ctx, []map[string]any{m}, writeinput.Options{}, "insert", false)
}

// PlanInsertBatch builds a batch INSERT plan without executing it.
func (q *Query) PlanInsertBatch(ctx context.Context, data []map[string]any) (*QueryPlan, error) {
	return q.planInsertRows(ctx, data, writeinput.Options{}, "insert", true)
}

func (q *Query) planInsertOrIgnore(ctx context.Context, data []map[string]any) (*QueryPlan, error) {
	return q.planInsertRows(ctx, data, writeinput.Options{}, "ignore", true)
}

func (q *Query) planUpsert(ctx context.Context, data []map[string]any, unique []string, updateCols []string) (*QueryPlan, error) {
	return q.planInsertRows(ctx, data, writeinput.Options{Conflict: &writeinput.Conflict{Columns: unique, Updates: updateCols}}, "upsert", false)
}

func (q *Query) planUpdateOrInsert(ctx context.Context, cond map[string]any, values map[string]any) (*QueryPlan, error) {
	if err := q.checkExactWrite(); err != nil {
		return nil, err
	}
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	if q.settings.strict {
		merged := make(map[string]any, len(cond)+len(values))
		for k, v := range cond {
			merged[k] = v
		}
		for k, v := range values {
			merged[k] = v
		}
		return q.planUpsert(ctx, []map[string]any{merged}, sortedMapKeys(cond), sortedMapKeys(values))
	}
	ib.Table(q.tableName()).UpdateOrInsert(cond, values)
	sqlStr, args, snapshot, err := ib.BuildSnapshot()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: snapshot.Table})
	plan.Columns = columnRefsFromNames(snapshot.Columns)
	plan.Metadata = map[string]any{"insert_mode": "update_or_insert", "condition_columns": snapshot.UniqueColumns, "update_columns": snapshot.UpdateColumns}
	q.finalizePlan(plan)
	return plan, nil
}

func (q *Query) planInsertUsing(ctx context.Context, columns []string, sub *Query) (*QueryPlan, error) {
	if err := q.checkExactWrite(); err != nil {
		return nil, err
	}
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	ib := newInsertBuilder(q.dialect)
	if q.settings.strict {
		return nil, tenantError("insert_using_unsupported")
	}
	ib.Table(q.tableName()).InsertUsing(columns, sub.builder)
	sqlStr, args, snapshot, err := ib.BuildSnapshot()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationInsert, sqlStr, args)
	plan.Tables = append(plan.Tables, TableRef{Name: snapshot.Table})
	plan.Columns = columnRefsFromNames(snapshot.Columns)
	plan.Metadata = map[string]any{"insert_mode": "insert_using"}
	q.finalizePlan(plan)
	return plan, nil
}

// PlanUpdate builds an UPDATE plan for data without executing it.
func (q *Query) PlanUpdate(ctx context.Context, data any) (*QueryPlan, error) {
	if err := q.checkExactWrite(); err != nil {
		return nil, err
	}
	if q.err != nil {
		return nil, q.err
	}
	m, err := dataToMap(data)
	if err != nil {
		return nil, err
	}
	return q.planUpdateValues(ctx, m, writeinput.Options{})
}

// PlanDelete builds a DELETE plan without executing it.
func (q *Query) PlanDelete(ctx context.Context) (*QueryPlan, error) {
	if err := q.checkExactWrite(); err != nil {
		return nil, err
	}
	_ = ctx
	if q.err != nil {
		return nil, q.err
	}
	delBuilder := newDeleteBuilder(q.dialect)
	delBuilder.Table(q.tableName()).Delete()
	source, err := q.policyBuilder(q.builder)
	if err != nil {
		return nil, err
	}
	copyBuilderStateDelete(source, delBuilder)
	sqlStr, args, snapshot, err := delBuilder.BuildSnapshot()
	if err != nil {
		return nil, err
	}
	plan := newQueryPlan(OperationDelete, sqlStr, args)
	appendTableRef(plan, snapshot.Table, "")
	appendWriteSnapshotMetadata(plan, snapshot)
	q.finalizePlan(plan)
	return plan, nil
}
