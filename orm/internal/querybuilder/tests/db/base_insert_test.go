package db_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

func TestBaseInsertQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		input    *structs.InsertQuery
		expected QueryBuilderExpected
	}{
		{
			"Insert",
			"Insert",
			&structs.InsertQuery{
				Table: "users",
				Values: map[string]interface{}{
					"name": "John",
					"age":  30,
				},
			},
			QueryBuilderExpected{
				Expected: "INSERT INTO `users` (`age`, `name`) VALUES (?, ?)",
				Values:   []interface{}{"John", 30},
			},
		},
		{
			"InsertBatch",
			"InsertBatch",
			&structs.InsertQuery{
				Table: "users",
				ValuesBatch: []map[string]interface{}{
					{
						"name": "John",
						"age":  30,
					},
					{
						"name": "Mike",
						"age":  25,
					},
				},
			},
			QueryBuilderExpected{
				Expected: "INSERT INTO `users` (`age`, `name`) VALUES (?, ?), (?, ?)",
				Values:   []interface{}{30, "John", 25, "Mike"},
			},
		},
		{
			"InsertUsing",
			"InsertUsing",
			&structs.InsertQuery{
				Table:   "users",
				Columns: []string{"name", "age"},
				Query: &structs.Query{
					Table: structs.Table{Name: "profiles"},
					Joins: &structs.Joins{
						Joins:        &[]structs.Join{},
						LateralJoins: &[]structs.Join{},
					},
					Columns: &[]structs.Column{
						{Name: "name"},
						{Name: "age"},
					},
					Conditions: &[]structs.Where{},
					ConditionGroups: []structs.WhereGroup{
						{
							Conditions: []structs.Where{
								{
									Column:    "age",
									Condition: ">",
									Value:     []interface{}{18},
								},
							},
							IsDummyGroup: true,
						},
					},
					Order: &[]structs.Order{},
					Lock:  &structs.Lock{},
				},
			},
			QueryBuilderExpected{
				Expected: "INSERT INTO `users` (`name`, `age`) SELECT `name`, `age` FROM `profiles` WHERE `age` > ?",
				Values:   []interface{}{18},
			},
		},
		{
			"InsertIgnore",
			"InsertIgnore",
			&structs.InsertQuery{
				Table:  "users",
				Values: map[string]interface{}{"name": "John", "age": 30},
				Ignore: true,
			},
			QueryBuilderExpected{
				Expected: "INSERT IGNORE INTO `users` (`age`, `name`) VALUES (?, ?)",
				Values:   []interface{}{30, "John"},
			},
		},
		{
			"Upsert",
			"Upsert",
			&structs.InsertQuery{
				Table:       "flights",
				ValuesBatch: []map[string]interface{}{{"departure": "Oakland", "destination": "San Diego", "price": 99}},
				Upsert:      &structs.Upsert{UniqueColumns: []string{"departure", "destination"}, UpdateColumns: []string{"price"}},
			},
			QueryBuilderExpected{
				Expected: "INSERT INTO `flights` (`departure`, `destination`, `price`) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE `price` = VALUES(`price`)",
				Values:   []interface{}{"Oakland", "San Diego", 99},
			},
		},
		{
			"UpdateOrInsert",
			"Upsert",
			&structs.InsertQuery{
				Table:       "users",
				ValuesBatch: []map[string]interface{}{{"email": "john@example.com", "name": "John"}},
				Upsert:      &structs.Upsert{UniqueColumns: []string{"email"}, UpdateColumns: []string{"name"}},
			},
			QueryBuilderExpected{
				Expected: "INSERT INTO `users` (`email`, `name`) VALUES (?, ?) ON DUPLICATE KEY UPDATE `name` = VALUES(`name`)",
				Values:   []interface{}{"john@example.com", "John"},
			},
		},
	}

	builder := mysql.NewMySQLQueryBuilder()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got string
			var gotValues []interface{} = nil
			switch tt.method {
			case "Insert":
				got, gotValues, _ = builder.Insert(tt.input)
			case "InsertBatch":
				got, gotValues, _ = builder.InsertBatch(tt.input)
			case "InsertUsing":
				got, gotValues, _ = builder.InsertUsing(tt.input)
			case "InsertIgnore":
				got, gotValues, _ = builder.InsertIgnore(tt.input)
			case "Upsert":
				got, gotValues, _ = builder.Upsert(tt.input)
			}
			if got != tt.expected.Expected {
				t.Errorf("expected '%s' but got '%s'", tt.expected, got)
			}

			if len(gotValues) != len(tt.expected.Values) {
				t.Errorf("expected '%v' but got '%v'", tt.expected.Values, gotValues)
			}

		})
	}
}
