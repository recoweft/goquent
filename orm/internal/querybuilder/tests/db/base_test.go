package db_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

type QueryBuilderExpected struct {
	Expected string
	Values   []interface{}
}

func TestBaseQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		input    structs.Query
		expected QueryBuilderExpected
	}{
		{
			"Select",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Name: "id"},
					{Name: "name"},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT `id`, `name`",
				Values:   nil,
			},
		},
		{
			"SelectRaw",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "COUNT(*) as total"},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT COUNT(*) as total",
				Values:   nil,
			},
		},
		{
			"SelectRaw_With_Value",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "price * ? as price_with_tax", Values: []interface{}{1.0825}},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT price * ? as price_with_tax",
				Values:   []interface{}{1.0825},
			},
		},
		{
			"Count",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "COUNT(*)", Values: nil},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT COUNT(*)",
				Values:   nil,
			},
		},
		{
			"Max",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "MAX(price)", Values: nil},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT MAX(price)",
				Values:   nil,
			},
		},
		{
			"Min",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "MIN(price)", Values: nil},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT MIN(price)",
				Values:   nil,
			},
		},
		{
			"Sum",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "SUM(price)", Values: nil},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT SUM(price)",
				Values:   nil,
			},
		},
		{
			"Avg",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Raw: "AVG(price)", Values: nil},
				},
			},
			QueryBuilderExpected{
				Expected: "SELECT AVG(price)",
				Values:   nil,
			},
		},
		{
			"From",
			"From",
			structs.Query{
				Table: structs.Table{Name: "users"},
			},
			QueryBuilderExpected{
				Expected: "FROM `users`",
				Values:   nil,
			}},

		{
			"Where",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:    "age",
								Condition: ">",
								Value:     []interface{}{18},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE `age` > ?",
				Values:   []interface{}{18},
			},
		},
		{
			"WhereQuery",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:    "age",
								Condition: ">",
								Query: &structs.Query{
									Columns: &[]structs.Column{
										{Name: "id"},
									},
									Table:           structs.Table{Name: "users"},
									ConditionGroups: []structs.WhereGroup{},
									Conditions:      &[]structs.Where{},
									Joins: &structs.Joins{
										Joins:        &[]structs.Join{},
										LateralJoins: &[]structs.Join{},
									},
									Order: &[]structs.Order{},
									Group: &structs.GroupBy{},
								},
								Operator: consts.LogicalOperator_AND,
							},
							{
								Column:    "name",
								Condition: "=",
								Value:     []interface{}{"John"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
					},
					{
						Conditions: []structs.Where{
							{
								Column:    "city",
								Condition: "=",
								Value:     []interface{}{"New York"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE (`age` > (SELECT `id` FROM `users`) AND `name` = ?) AND `city` = ?",
				Values:   []interface{}{"John", "New York"},
			},
		},
		{
			"WhereGroup_Or",
			"WhereGroup",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:    "age",
								Condition: ">",
								Value:     []interface{}{18},
								Operator:  consts.LogicalOperator_AND,
							},
							{
								Column:    "name",
								Condition: "=",
								Value:     []interface{}{"John"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						Operator: consts.LogicalOperator_AND,
					},
					{
						Conditions: []structs.Where{
							{
								Column:    "age",
								Condition: ">",
								Value:     []interface{}{18},
								Operator:  consts.LogicalOperator_AND,
							},
							{
								Column:    "name",
								Condition: "=",
								Value:     []interface{}{"John"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						Operator: consts.LogicalOperator_OR,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE (`age` > ? AND `name` = ?) OR (`age` > ? AND `name` = ?)",
				Values:   []interface{}{18, "John", 18, "John"},
			},
		},
		{
			"WhereRaw",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Raw:      "age > 18",
								Operator: consts.LogicalOperator_AND,
							},
							{
								Column:    "name",
								Condition: "=",
								Value:     []interface{}{"John"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE age > 18 AND `name` = ?",
				Values:   []interface{}{"John"},
			},
		},
		{
			"WhereRaw_Or",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Raw:      "age > 18",
								Operator: consts.LogicalOperator_AND,
							},
							{
								Column:    "name",
								Condition: "=",
								Value:     []interface{}{"John"},
								Operator:  consts.LogicalOperator_OR,
							},
							{
								Raw:      "city = 'New York'",
								Operator: consts.LogicalOperator_OR,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE age > 18 OR `name` = ? OR city = 'New York'",
				Values:   []interface{}{"John"},
			},
		},
		{
			"SafeWhereRaw",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Raw:      "age > :age",
								ValueMap: map[string]interface{}{"age": 25},
								Operator: consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE age > ?",
				Values:   []interface{}{25},
			},
		},
		{
			"SafeWhereRaw_PrefixKeys",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Raw:      "col = :test AND col2 = :test1",
								ValueMap: map[string]interface{}{"test": 1, "test1": 2},
								Operator: consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE col = ? AND col2 = ?",
				Values:   []interface{}{1, 2},
			},
		},
		{
			"WhereGroup",
			"WhereGroup",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:    "age",
								Condition: ">",
								Value:     []interface{}{18},
								Operator:  consts.LogicalOperator_AND,
							},
							{
								Column:    "name",
								Condition: "=",
								Value:     []interface{}{"John"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
					},
					{
						Conditions: []structs.Where{
							{
								Column:    "city",
								Condition: "=",
								Value:     []interface{}{"New York"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE (`age` > ? AND `name` = ?) AND `city` = ?",
				Values:   []interface{}{18, "John", "New York"},
			},
		},
		{
			"WhereNull",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:    "name",
								Condition: "IS NULL",
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE `name` IS NULL",
				Values:   nil,
			},
		},
		{
			"WhereColumn",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:      "name",
								Condition:   "=",
								ValueColumn: "users.name",
								Operator:    consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE `name` = `users`.`name`",
				Values:   nil,
			},
		},
		{
			"WhereBetween",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column: "age",
								Between: &structs.WhereBetween{
									From:  18,
									To:    30,
									IsNot: false,
								},
								Condition: consts.Condition_BETWEEN,
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE `age` BETWEEN ? AND ?",
				Values:   []interface{}{18, 30},
			},
		},
		{
			"WhereNotBetween",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column: "age",
								Between: &structs.WhereBetween{
									From:  18,
									To:    30,
									IsNot: true,
								},
								Condition: consts.Condition_NOT_BETWEEN,
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE `age` NOT BETWEEN ? AND ?",
				Values:   []interface{}{18, 30},
			},
		},
		{
			"WhereNotBetweenColumns",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column: "age",
								Between: &structs.WhereBetween{
									From:     "users.age",
									To:       "users.age",
									IsColumn: true,
									IsNot:    true,
								},
								Condition: consts.Condition_NOT_BETWEEN,
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE `age` NOT BETWEEN `users`.`age` AND `users`.`age`",
				Values:   nil,
			},
		},
		{
			"WhereExists",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Exists: &structs.Exists{
									Query: &structs.Query{
										Columns: &[]structs.Column{
											{Name: "id"},
										},
										Table:           structs.Table{Name: "users"},
										ConditionGroups: []structs.WhereGroup{},
										Conditions:      &[]structs.Where{},
										Joins: &structs.Joins{
											Joins:        &[]structs.Join{},
											LateralJoins: &[]structs.Join{},
										},
										Order: &[]structs.Order{},
										Group: &structs.GroupBy{},
									},
									IsNot: false,
								},
								Condition: consts.Condition_EXISTS,
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE EXISTS (SELECT `id` FROM `users`)",
				Values:   nil,
			},
		},
		{
			"WhereNotExists",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Exists: &structs.Exists{
									Query: &structs.Query{
										Columns: &[]structs.Column{
											{Name: "id"},
										},
										Table:           structs.Table{Name: "users"},
										ConditionGroups: []structs.WhereGroup{},
										Conditions:      &[]structs.Where{},
										Joins: &structs.Joins{
											Joins:        &[]structs.Join{},
											LateralJoins: &[]structs.Join{},
										},
										Order: &[]structs.Order{},
										Group: &structs.GroupBy{},
									},
									IsNot: true,
								},
								Condition: consts.Condition_NOT_EXISTS,
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE NOT EXISTS (SELECT `id` FROM `users`)",
				Values:   nil,
			},
		},
		{
			"WhereDate",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Function:  "DATE",
								Column:    "created_at",
								Condition: "=",
								Value:     []interface{}{"2021-01-01"},
								Operator:  consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " WHERE DATE(`created_at`) = ?",
				Values:   []interface{}{"2021-01-01"},
			},
		},
		{
			"Join",
			"Join",
			structs.Query{
				Table: structs.Table{Name: "users"},
				Joins: &structs.Joins{
					Joins: &[]structs.Join{
						{
							Name:               "orders",
							TargetNameMap:      map[string]string{"inner": "orders"},
							SearchColumn:       "users.id",
							SearchCondition:    "=",
							SearchTargetColumn: "orders.user_id",
						},
					},
					LateralJoins: &[]structs.Join{},
				},
			},
			QueryBuilderExpected{
				Expected: " INNER JOIN `orders` ON `users`.`id` = `orders`.`user_id`",
				Values:   nil,
			},
		},
		{
			"Join_Left",
			"Join",
			structs.Query{
				Table: structs.Table{Name: "users"},
				Joins: &structs.Joins{
					Joins: &[]structs.Join{
						{
							Name:               "orders",
							TargetNameMap:      map[string]string{"left": "orders"},
							SearchColumn:       "users.id",
							SearchCondition:    "=",
							SearchTargetColumn: "orders.user_id",
						},
					},
					LateralJoins: &[]structs.Join{},
				},
			},
			QueryBuilderExpected{
				Expected: " LEFT JOIN `orders` ON `users`.`id` = `orders`.`user_id`",
				Values:   nil,
			},
		},
		{
			"Join_Multiple",
			"Join",
			structs.Query{
				Table: structs.Table{Name: "users"},
				Joins: &structs.Joins{
					Joins: &[]structs.Join{
						{
							Name:               "orders",
							TargetNameMap:      map[string]string{"inner": "orders"},
							SearchColumn:       "users.id",
							SearchCondition:    "=",
							SearchTargetColumn: "orders.user_id",
						},
						{
							Name:               "products",
							TargetNameMap:      map[string]string{"inner": "products"},
							SearchColumn:       "users.id",
							SearchCondition:    "=",
							SearchTargetColumn: "products.user_id",
						},
					},
					LateralJoins: &[]structs.Join{},
				},
			},
			QueryBuilderExpected{
				Expected: " INNER JOIN `orders` ON `users`.`id` = `orders`.`user_id` INNER JOIN `products` ON `users`.`id` = `products`.`user_id`",
				Values:   nil,
			},
		},
		{
			"OrderBy",
			"OrderBy",
			structs.Query{
				Order: &[]structs.Order{
					{
						Column: "name",
						IsAsc:  true,
					},
				},
			},
			QueryBuilderExpected{
				Expected: " ORDER BY `name` ASC",
				Values:   nil,
			},
		},
		{
			"Lateral Join",
			"Join",
			structs.Query{
				Table: structs.Table{Name: "users"},
				Joins: &structs.Joins{
					Joins: &[]structs.Join{},
					LateralJoins: &[]structs.Join{
						{
							Name:          "orders",
							TargetNameMap: map[string]string{"lateral": "orders"},
							Query: &structs.Query{
								Columns: &[]structs.Column{
									{Name: "id"},
								},
								Table:           structs.Table{Name: "users"},
								ConditionGroups: []structs.WhereGroup{},
								Conditions:      &[]structs.Where{},
								Joins: &structs.Joins{
									Joins:        &[]structs.Join{},
									LateralJoins: &[]structs.Join{},
								},
								Order: &[]structs.Order{},
								Group: &structs.GroupBy{},
							},
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: " ,LATERAL(SELECT `id` FROM `users`) as `orders`",
				Values:   nil,
			},
		},
		{
			"LeftLateral Join",
			"Join",
			structs.Query{
				Table: structs.Table{Name: "users"},
				Joins: &structs.Joins{
					Joins: &[]structs.Join{},
					LateralJoins: &[]structs.Join{
						{
							Name:          "orders",
							TargetNameMap: map[string]string{"left_lateral": "orders"},
							Query: &structs.Query{
								Columns: &[]structs.Column{
									{Name: "id"},
								},
								Table: structs.Table{Name: "users"},
								ConditionGroups: []structs.WhereGroup{
									{
										Conditions: []structs.Where{
											{
												Column:    "age",
												Condition: ">",
												Value:     []interface{}{18},
												Operator:  consts.LogicalOperator_AND,
											},
										},
										IsDummyGroup: true,
									},
								},
								Conditions: &[]structs.Where{},
								Joins: &structs.Joins{
									Joins:        &[]structs.Join{},
									LateralJoins: &[]structs.Join{},
								},
								Order: &[]structs.Order{},
								Group: &structs.GroupBy{},
							},
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: " ,LEFT LATERAL(SELECT `id` FROM `users` WHERE `age` > ?) as `orders`",
				Values:   []interface{}{18},
			},
		},
		{
			"OrderByRaw",
			"OrderBy",
			structs.Query{
				Order: &[]structs.Order{
					{
						Column: "name",
						IsAsc:  true,
						Raw:    "name DESC",
					},
				},
			},
			QueryBuilderExpected{
				Expected: " ORDER BY name DESC",
				Values:   nil,
			},
		},
		{
			"GroupBy",
			"GroupBy",
			structs.Query{
				Group: &structs.GroupBy{
					Columns: []string{"name"},
					Having:  &[]structs.Having{},
				},
			},
			QueryBuilderExpected{
				Expected: " GROUP BY `name`",
				Values:   nil,
			},
		},
		{
			"GroupBy_Having",
			"GroupBy",
			structs.Query{
				Group: &structs.GroupBy{
					Columns: []string{"name"},
					Having: &[]structs.Having{
						{
							Column:    "age",
							Condition: ">",
							Value:     18,
							Operator:  consts.LogicalOperator_AND,
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: " GROUP BY `name` HAVING `age` > ?",
				Values:   []interface{}{18},
			},
		},
		{
			"GroupBy_HavingRaw",
			"GroupBy",
			structs.Query{
				Group: &structs.GroupBy{
					Columns: []string{"name"},
					Having: &[]structs.Having{
						{
							Raw:      "age > 18",
							Operator: consts.LogicalOperator_AND,
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: " GROUP BY `name` HAVING age > 18",
				Values:   nil,
			},
		},
		{
			"GroupBy_HavingRaw_OR",
			"GroupBy",
			structs.Query{
				Group: &structs.GroupBy{
					Columns: []string{"name"},
					Having: &[]structs.Having{
						{
							Raw:      "birthday > '2000-01-01'",
							Operator: consts.LogicalOperator_AND,
						},
						{
							Raw:      "city = 'New York'",
							Operator: consts.LogicalOperator_OR,
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: " GROUP BY `name` HAVING birthday > '2000-01-01' OR city = 'New York'",
				Values:   nil,
			},
		},
		{
			"Limit",
			"Limit",
			structs.Query{
				Limit: structs.Limit{
					Limit: 10,
				},
			},
			QueryBuilderExpected{
				Expected: " LIMIT 10",
				Values:   nil,
			},
		},
		{
			"Offset",
			"Offset",
			structs.Query{
				Offset: structs.Offset{
					Offset: 10,
				},
			},
			QueryBuilderExpected{
				Expected: " OFFSET 10",
				Values:   nil,
			},
		},
		{
			"Limit_And_Offset",
			"Limit_And_Offset",
			structs.Query{
				Limit: structs.Limit{
					Limit: 10,
				},
				Offset: structs.Offset{
					Offset: 10,
				},
			},
			QueryBuilderExpected{
				Expected: " LIMIT 10 OFFSET 10",
				Values:   nil,
			},
		},
		{
			"SHARED_LOCK",
			"Lock",
			structs.Query{
				Lock: &structs.Lock{
					LockType: consts.Lock_SHARE_MODE,
				},
			},
			QueryBuilderExpected{
				Expected: " LOCK IN SHARE MODE",
				Values:   nil,
			},
		},
		{
			"FOR_UPDATE",
			"Lock",
			structs.Query{
				Lock: &structs.Lock{
					LockType: consts.Lock_FOR_UPDATE,
				},
			},
			QueryBuilderExpected{
				Expected: " FOR UPDATE",
				Values:   nil,
			},
		},
	}

	builder := mysql.NewMySQLQueryBuilder()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			//sb := &strings.Builder{}
			sb := make([]byte, 0, 1000)

			var got string
			var gotValues []interface{} = nil
			switch tt.method {
			case "Select":
				values, _ := builder.Select(&sb, tt.input.Columns, "", nil)
				columns := string(sb)
				got = got + "SELECT " + columns
				gotValues = values
			case "From":
				builder.From(&sb, tt.input.Table.Name)
				got = string(sb)
			case "Where":
				values, _ := builder.Where(&sb, tt.input.ConditionGroups)
				got = string(sb)
				gotValues = values
			case "WhereGroup":
				values, _ := builder.Where(&sb, tt.input.ConditionGroups)
				got = string(sb)
				gotValues = values
			case "Join":
				values := builder.Join(&sb, tt.input.Joins)
				got = string(sb)
				gotValues = values
			case "OrderBy":
				builder.OrderBy(&sb, tt.input.Order)
				got = string(sb)
			case "GroupBy":
				values := builder.GroupBy(&sb, tt.input.Group)
				got = string(sb)
				gotValues = values
			case "Limit":
				builder.Limit(&sb, tt.input.Limit)
				got = string(sb)
			case "Offset":
				builder.Offset(&sb, tt.input.Offset)
				got = string(sb)
			case "Limit_And_Offset":
				builder.Limit(&sb, tt.input.Limit)
				gotLimit := string(sb)
				sb = sb[:0]
				builder.Offset(&sb, tt.input.Offset)
				gotOffset := string(sb)
				got = gotLimit + gotOffset
			case "Lock":
				builder.Lock(&sb, tt.input.Lock)
				got = string(sb)
			}
			if got != tt.expected.Expected {
				t.Errorf("expected '%s' but got '%s'", tt.expected.Expected, got)
			}

			if len(gotValues) != len(tt.expected.Values) {
				t.Errorf("expected '%v' but got '%v'", tt.expected.Values, gotValues)
			}

		})
	}
}
