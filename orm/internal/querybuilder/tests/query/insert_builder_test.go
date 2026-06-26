package query_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

func TestInsertBuilder(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *query.InsertBuilder
		expectedQuery  string
		expectedValues []interface{}
	}{
		{
			"Insert",
			func() *query.InsertBuilder {
				return query.NewInsertBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Insert(map[string]interface{}{
						"name": "John Doe",
						"age":  30,
					})
			},
			"INSERT INTO `users` (`age`, `name`) VALUES (?, ?)",
			[]interface{}{30, "John Doe"},
		},
		{
			"InsertBatch",
			func() *query.InsertBuilder {
				return query.NewInsertBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					InsertBatch([]map[string]interface{}{
						{
							"name": "John Doe",
							"age":  30,
						},
						{
							"name": "Jane Doe",
							"age":  25,
						},
					})
			},
			"INSERT INTO `users` (`age`, `name`) VALUES (?, ?), (?, ?)",
			[]interface{}{30, "John Doe", 25, "Jane Doe"},
		},
		{
			"InsertUsing",
			func() *query.InsertBuilder {
				return query.NewInsertBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					InsertUsing([]string{"name", "age"}, query.NewSelectBuilder(mysql.NewMySQLQueryBuilder()).
						Table("profiles").
						Select("name", "age").
						Where("age", ">", 18))
			},
			"INSERT INTO `users` (`name`, `age`) SELECT `name`, `age` FROM `profiles` WHERE `age` > ?",
			[]interface{}{18},
		},
		{
			"InsertIgnore",
			func() *query.InsertBuilder {
				return query.NewInsertBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					InsertOrIgnore([]map[string]interface{}{
						{"name": "John", "age": 30},
					})
			},
			"INSERT IGNORE INTO `users` (`age`, `name`) VALUES (?, ?)",
			[]interface{}{30, "John"},
		},
		{
			"Upsert",
			func() *query.InsertBuilder {
				return query.NewInsertBuilder(mysql.NewMySQLQueryBuilder()).
					Table("flights").
					Upsert([]map[string]interface{}{{"departure": "Oakland", "destination": "San Diego", "price": 99}},
						[]string{"departure", "destination"}, []string{"price"})
			},
			"INSERT INTO `flights` (`departure`, `destination`, `price`) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE `price` = VALUES(`price`)",
			[]interface{}{"Oakland", "San Diego", 99},
		},
		{
			"UpdateOrInsert",
			func() *query.InsertBuilder {
				return query.NewInsertBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					UpdateOrInsert(map[string]interface{}{"email": "john@example.com"}, map[string]interface{}{"name": "John"})
			},
			"INSERT INTO `users` (`email`, `name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)",
			[]interface{}{"john@example.com", "John"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tt.setup()
			query, values, _ := builder.Build()

			if query != tt.expectedQuery {
				t.Errorf("expected '%s' but got '%s'", tt.expectedQuery, query)
			}

			if len(values) != len(tt.expectedValues) {
				t.Errorf("expected values %v but got %v", tt.expectedValues, values)
			}

			for i := range values {
				if values[i] != tt.expectedValues[i] {
					t.Errorf("expected value %v at index %d but got %v", tt.expectedValues[i], i, values[i])
				}
			}
		})
	}
}
