package db_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

func TestBaseDeleteQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		input    *structs.DeleteQuery
		expected QueryBuilderExpected
	}{
		{
			"Delete",
			"Delete",
			&structs.DeleteQuery{
				Table: "users",
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
				Expected: "DELETE FROM `users` WHERE `id` = ?",
				Values:   []interface{}{1},
			},
		},
		{
			"Delete_where_not",
			"Delete",
			&structs.DeleteQuery{
				Table: "users",
				Query: &structs.Query{
					ConditionGroups: []structs.WhereGroup{
						{
							Conditions: []structs.Where{
								{
									Column:    "id",
									Condition: "!=",
									Value:     []interface{}{1},
								},
							},
							Operator:     consts.LogicalOperator_OR,
							IsDummyGroup: false,
							IsNot:        true,
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
				Expected: "DELETE FROM `users` WHERE NOT (`id` != ?)",
				Values:   []interface{}{1},
			},
		},
		{
			"Delete_JOINS",
			"Delete",
			&structs.DeleteQuery{
				Table: "users",
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
				Expected: "DELETE `users` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `age` > ?",
				Values:   []interface{}{18},
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
			case "Delete":
				got, gotValues, _ = builder.BuildDelete(tt.input)
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
