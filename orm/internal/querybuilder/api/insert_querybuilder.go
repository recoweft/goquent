package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
	"sort"
)

type InsertQueryBuilder struct {
	builder *query.InsertBuilder
}

func NewInsertQueryBuilder(strategy interfaces.QueryBuilderStrategy) *InsertQueryBuilder {
	return &InsertQueryBuilder{
		builder: query.NewInsertBuilder(strategy),
	}
}

func (ib *InsertQueryBuilder) Table(table string) *InsertQueryBuilder {
	ib.builder.Table(table)
	return ib
}

func (ib *InsertQueryBuilder) Insert(data map[string]interface{}) *InsertQueryBuilder {
	ib.builder.Insert(data)
	return ib
}

func (ib *InsertQueryBuilder) InsertBatch(data []map[string]interface{}) *InsertQueryBuilder {
	ib.builder.InsertBatch(data)
	return ib
}

func (ib *InsertQueryBuilder) InsertOrIgnore(data []map[string]interface{}) *InsertQueryBuilder {
	ib.builder.InsertOrIgnore(data)
	return ib
}

func (ib *InsertQueryBuilder) Upsert(data []map[string]interface{}, unique []string, updateColumns []string) *InsertQueryBuilder {
	ib.builder.Upsert(data, unique, updateColumns)
	return ib
}

func (ib *InsertQueryBuilder) UpdateOrInsert(condition map[string]interface{}, values map[string]interface{}) *InsertQueryBuilder {
	ib.builder.UpdateOrInsert(condition, values)
	return ib
}

func (ib *InsertQueryBuilder) InsertUsing(columns []string, qb *SelectQueryBuilder) *InsertQueryBuilder {
	ib.builder.InsertUsing(columns, qb.builder)
	return ib
}

func (ib *InsertQueryBuilder) Dump() (string, []interface{}, error) {
	b := query.NewDebugBuilder[*query.InsertBuilder, InsertQueryBuilder](ib.builder)

	return b.Dump()
}

func (ib *InsertQueryBuilder) RawSql() (string, error) {
	b := query.NewDebugBuilder[*query.InsertBuilder, InsertQueryBuilder](ib.builder)

	return b.RawSql()
}

func (ib *InsertQueryBuilder) Build() (string, []interface{}, error) {
	return ib.builder.Build()
}

// InsertSnapshot describes the exact input used by BuildSnapshot.
type InsertSnapshot struct {
	Table         string
	Columns       []string
	BatchSize     int
	UniqueColumns []string
	UpdateColumns []string
}

func (ib *InsertQueryBuilder) BuildSnapshot() (string, []any, InsertSnapshot, error) {
	sql, args, err := ib.Build()
	if err != nil {
		return "", nil, InsertSnapshot{}, err
	}
	src := ib.builder.BuiltQuery
	out := InsertSnapshot{Table: src.Table, BatchSize: len(src.ValuesBatch)}
	names := make(map[string]bool)
	for k := range src.Values {
		names[k] = true
	}
	for _, row := range src.ValuesBatch {
		for k := range row {
			names[k] = true
		}
	}
	for _, col := range src.Columns {
		names[col] = true
	}
	for k := range names {
		out.Columns = append(out.Columns, k)
	}
	sort.Strings(out.Columns)
	if src.Query != nil {
		out.Columns = append([]string(nil), src.Columns...)
	}
	if src.Upsert != nil {
		out.UniqueColumns = append([]string(nil), src.Upsert.UniqueColumns...)
		out.UpdateColumns = append([]string(nil), src.Upsert.UpdateColumns...)
	}
	return sql, args, out, nil
}
