package query

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type InsertBuilder struct {
	BaseBuilder
	dbBuilder interfaces.QueryBuilderStrategy
	query     *structs.InsertQuery
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
	ib.dbBuilder.ResetPlaceholderCounter()
	query, values, err := ib.dbBuilder.BuildInsert(ib.query)
	return query, values, err
}
