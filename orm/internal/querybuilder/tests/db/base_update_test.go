package db_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

func TestBaseUpdateQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		input    *structs.UpdateQuery
		expected QueryBuilderExpected
	}{
		{
			"Update",
			"Update",
			&structs.UpdateQuery{
				Table: "users",
				Values: map[string]interface{}{
					"name": "Joe",
					"age":  30,
				},
				Query: &structs.Query{
					ConditionGroups: []structs.WhereGroup{
						{
							Conditions: []structs.Where{
								{
									Column:    "id",
									Condition: "=",
									Value:     []interface{}{1},
								},
							},
							IsDummyGroup: true,
						},
					},
					Joins: &structs.Joins{
						Joins:        &[]structs.Join{},
						LateralJoins: &[]structs.Join{},
					},
					Order: &[]structs.Order{},
				},
			},
			QueryBuilderExpected{
				Expected: "UPDATE `users` SET `age` = ?, `name` = ? WHERE `id` = ?",
				Values:   []interface{}{30, "Joe", 1},
			},
		},
		{
			"Update Where Not",
			"Update",
			&structs.UpdateQuery{
				Table: "users",
				Values: map[string]interface{}{
					"name": "Joe",
					"age":  30,
				},
				Query: &structs.Query{
					ConditionGroups: []structs.WhereGroup{
						{
							Conditions: []structs.Where{
								{
									Column:    "id",
									Condition: "!=",
									Value:     []interface{}{1},
								},
								{
									Column:    "age",
									Condition: ">",
									Value:     []interface{}{18},
								},
								{
									Column:    "name",
									Condition: "=",
									Value:     []interface{}{"John"},
								},
							},
							IsNot:        true,
							IsDummyGroup: false,
						},
					},
					Order: &[]structs.Order{},
				},
			},
			QueryBuilderExpected{
				Expected: "UPDATE `users` SET `age` = ?, `name` = ? WHERE NOT (`id` != ? AND `age` > ? AND `name` = ?)",
				Values:   []interface{}{30, "Joe", 1, 18, "John"},
			},
		},
		{
			"Update Where Between",
			"Update",
			&structs.UpdateQuery{
				Table: "users",
				Values: map[string]interface{}{
					"name": "Joe",
					"age":  30,
				},
				Query: &structs.Query{
					ConditionGroups: []structs.WhereGroup{
						{
							Conditions: []structs.Where{
								{
									Column:    "id",
									Condition: consts.Condition_BETWEEN,
									Value:     []interface{}{1, 10},
									Between: &structs.WhereBetween{
										To:       10,
										From:     1,
										IsColumn: false,
										IsNot:    false,
									},
								},
							},
							IsDummyGroup: true,
						},
					},
					Order: &[]structs.Order{},
				},
			},
			QueryBuilderExpected{
				Expected: "UPDATE `users` SET `age` = ?, `name` = ? WHERE `id` BETWEEN ? AND ?",
				Values:   []interface{}{30, "Joe", 1, 10},
			},
		},
		{
			"Update JOIN",
			"Update",
			&structs.UpdateQuery{
				Table: "users",
				Values: map[string]interface{}{
					"name": "Joe",
					"age":  30,
				},
				Query: &structs.Query{
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
					Joins: &structs.Joins{
						Joins: &[]structs.Join{
							{
								Name:               "profiles",
								TargetNameMap:      map[string]string{consts.Join_INNER: "profiles"},
								SearchColumn:       "users.id",
								SearchCondition:    "=",
								SearchTargetColumn: "profiles.user_id",
							},
						},
						LateralJoins: &[]structs.Join{},
					},
					Order: &[]structs.Order{},
				},
			},
			QueryBuilderExpected{
				Expected: "UPDATE `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` SET `age` = ?, `name` = ? WHERE `age` > ?",
				Values:   []interface{}{18, "Joe", 30},
			},
		},
		{
			"Update ORDER BY",
			"Update",
			&structs.UpdateQuery{
				Table: "users",
				Values: map[string]interface{}{
					"name": "Joe",
					"age":  30,
				},
				Query: &structs.Query{
					ConditionGroups: []structs.WhereGroup{
						{
							Conditions: []structs.Where{
								{
									Column:    "id",
									Condition: "=",
									Value:     []interface{}{1},
								},
							},
							IsDummyGroup: true,
						},
					},
					Joins: &structs.Joins{
						Joins:        &[]structs.Join{},
						LateralJoins: &[]structs.Join{},
					},
					Order: &[]structs.Order{
						{
							Column: "name",
							IsAsc:  true,
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: "UPDATE `users` SET `age` = ?, `name` = ? WHERE `id` = ? ORDER BY `name` ASC",
				Values:   []interface{}{30, "Joe", 1},
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
			case "Update":
				got, gotValues, _ = builder.BuildUpdate(tt.input)
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
