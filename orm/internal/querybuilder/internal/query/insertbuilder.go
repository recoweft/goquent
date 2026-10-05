package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

type InsertBuilder struct {
	BaseBuilder
	dbBuilder  interfaces.QueryBuilderStrategy
	query      *structs.InsertQuery
	BuiltQuery *structs.InsertQuery
}

func NewInsertBuilder(dbBuilder interfaces.QueryBuilderStrategy) *InsertBuilder {
	return &InsertBuilder{
		dbBuilder: dbBuilder,
		query:     &structs.InsertQuery{},
	}
}

func (ib *InsertBuilder) Table(table string) *InsertBuilder {
	ib.query.Table = table
	return ib
}

func (ib *InsertBuilder) Insert(data map[string]interface{}) *InsertBuilder {
	ib.query.Values = data
	return ib
}

func (ib *InsertBuilder) InsertBatch(data []map[string]interface{}) *InsertBuilder {
	ib.query.ValuesBatch = data
	return ib
}

func (ib *InsertBuilder) InsertOrIgnore(data []map[string]interface{}) *InsertBuilder {
	ib.query.ValuesBatch = data
	ib.query.Ignore = true
	return ib
}

func (ib *InsertBuilder) Upsert(data []map[string]interface{}, unique []string, updateColumns []string) *InsertBuilder {
	ib.query.ValuesBatch = data
	ib.query.Upsert = &structs.Upsert{UniqueColumns: unique, UpdateColumns: updateColumns}
	return ib
}

func (ib *InsertBuilder) UpdateOrInsert(condition map[string]interface{}, values map[string]interface{}) *InsertBuilder {
	merged := make(map[string]interface{})
	for k, v := range condition {
		merged[k] = v
	}
	for k, v := range values {
		merged[k] = v
	}
	unique := make([]string, 0, len(condition))
	for k := range condition {
		unique = append(unique, k)
	}
	updateCols := make([]string, 0, len(values))
	for k := range values {
		updateCols = append(updateCols, k)
	}
	ib.query.ValuesBatch = []map[string]interface{}{merged}
	ib.query.Upsert = &structs.Upsert{UniqueColumns: unique, UpdateColumns: updateCols}
	return ib
}

func (ib *InsertBuilder) InsertUsing(columns []string, b *SelectBuilder) *InsertBuilder {
	ib.query.Columns = columns

	// If there are conditions, add them to the query
	if b.WhereBuilder.query.Conditions != nil && len(*b.WhereBuilder.query.Conditions) > 0 {
		b.WhereBuilder.query.ConditionGroups = append(b.WhereBuilder.query.ConditionGroups, structs.WhereGroup{
			Conditions:   *b.WhereBuilder.query.Conditions,
			Operator:     consts.LogicalOperator_AND,
			IsDummyGroup: true,
		})
		b.WhereBuilder.query.Conditions = &[]structs.Where{}
	}

	b.buildQuery()
	ib.query.Query = b.GetQuery()

	return ib
}

func (ib *InsertBuilder) Build() (string, []interface{}, error) {
	// Render and inspect the same detached input, including INSERT SELECT.
	frozen := *ib.query
	copier := valuecopy.New()
	copyRow := func(row map[string]any) map[string]any {
		if row == nil {
			return nil
		}
		out := make(map[string]any, len(row))
		for k, v := range row {
			out[k], _ = copier.Copy(v)
		}
		return out
	}
	frozen.Values = copyRow(ib.query.Values)
	if ib.query.ValuesBatch != nil {
		frozen.ValuesBatch = make([]map[string]any, len(ib.query.ValuesBatch))
		for i, row := range ib.query.ValuesBatch {
			frozen.ValuesBatch[i] = copyRow(row)
		}
	}
	frozen.Columns = append([]string(nil), ib.query.Columns...)
	frozen.Query = structs.CloneQuery(ib.query.Query)
	if ib.query.Upsert != nil {
		frozen.Upsert = &structs.Upsert{UniqueColumns: append([]string(nil), ib.query.Upsert.UniqueColumns...), UpdateColumns: append([]string(nil), ib.query.Upsert.UpdateColumns...)}
	}
	ib.BuiltQuery = &frozen
	ib.dbBuilder.ResetPlaceholderCounter()
	query, values, err := ib.dbBuilder.BuildInsert(&frozen)
	return query, values, err
}
