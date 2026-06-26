package bench_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/api"
	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
)

func BenchmarkSimpleSelectQuery(b *testing.B) {

	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewSelectQueryBuilder(dbStrategy).
		Table("users").
		Select("id", "users.name as name")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}
}

func BenchmarkSimple2SelectQuery(b *testing.B) {

	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewSelectQueryBuilder(dbStrategy).
		Table("models").
		Where("id", ">", 0).
		Limit(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}
}

func BenchmarkNormalSelectQuery(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewSelectQueryBuilder(dbStrategy).
		Table("users").
		Select("id", "users.name as name").
		Join("profiles", "users.id", "=", "profiles.user_id").
		Where("profiles.age", ">", 18).
		OrderBy("users.name", "ASC")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}
}

func BenchmarkComplexSelectQuery(b *testing.B) {

	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewSelectQueryBuilder(dbStrategy).
		Table("users").
		Select("id", "users.name as name").
		Join("profiles", "users.id", "=", "profiles.user_id").
		Where("profiles.age", ">", 18).
		OrderBy("users.name", "ASC").
		OrderBy("profiles.age", "DESC").
		GroupBy("users.id").
		Having("COUNT(profiles.id)", ">", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}
}

func BenchmarkComplexSelectQueryWithUsingSubQuery(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewSelectQueryBuilder(dbStrategy).
		Table("users").
		Select("id", "users.name as name").
		Join("profiles", "users.id", "=", "profiles.user_id").
		Where("profiles.age", ">", 18).
		OrderBy("users.name", "ASC").
		OrderBy("profiles.age", "DESC").
		GroupBy("users.id").
		Having("COUNT(profiles.id)", ">", 1).
		WhereIn("users.id", func(qb *api.SelectQueryBuilder) {
			qb.Table("users").
				Select("id").
				Join("profiles", "users.id", "=", "profiles.user_id").
				Where("profiles.age", ">", 18)
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

}

func BenchmarkSimpleInsert(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewInsertQueryBuilder(dbStrategy).
		Table("users").
		Insert(map[string]interface{}{
			"name": "John",
			"age":  30,
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}
}

func BenchmarkInsertBatch(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewInsertQueryBuilder(dbStrategy).
		Table("users").
		InsertBatch([]map[string]interface{}{
			{
				"name": "John",
				"age":  30,
			},
			{
				"name": "Mike",
				"age":  25,
			},
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

	// go test -benchmem -run=^$ -bench BenchmarkInsertBatch -benchtime=1s
	// before refactor
	//  2263813               520.9 ns/op           472 B/op        17 allocs/op
	// after refactor
	//  3401209               352.6 ns/op          1184 B/op         5 allocs/op
}

func BenchmarkInsertUsing(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewInsertQueryBuilder(dbStrategy).
		Table("users").
		InsertBatch([]map[string]interface{}{
			{
				"name": "John",
				"age":  30,
			},
			{
				"name": "Mike",
				"age":  25,
			},
		}).
		InsertUsing([]string{"name", "age"}, api.NewSelectQueryBuilder(dbStrategy).
			Table("users").
			Select("id").
			Join("profiles", "users.id", "=", "profiles.user_id").
			Where("profiles.age", ">", 18).
			OrderBy("users.name", "ASC"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}
}

func BenchmarkSimpleUpdate(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewUpdateQueryBuilder(dbStrategy).
		Table("users").
		Update(map[string]interface{}{
			"name": "Joe",
			"age":  31,
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

}

func BenchmarkUpdateWhere(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewUpdateQueryBuilder(dbStrategy).
		Table("users").
		Where("id", "=", 1).
		Update(map[string]interface{}{
			"name": "Joe",
			"age":  31,
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

}

func BenchmarkJoinUpdate(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewUpdateQueryBuilder(dbStrategy).
		Table("users").
		Join("profiles", "users.id", "=", "profiles.user_id").
		Where("profiles.age", ">", 18).
		Update(map[string]interface{}{
			"name": "Joe",
			"age":  31,
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

}

func BenchmarkDelete(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewDeleteQueryBuilder(dbStrategy).
		Table("users").
		Where("id", "=", 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

}

func BenchmarkDeleteJoin(b *testing.B) {
	dbStrategy := mysql.NewMySQLQueryBuilder()

	qb := api.NewDeleteQueryBuilder(dbStrategy).
		Table("users").
		Join("profiles", "users.id", "=", "profiles.user_id").
		Where("profiles.age", ">", 18)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qb.Build()
	}

}
