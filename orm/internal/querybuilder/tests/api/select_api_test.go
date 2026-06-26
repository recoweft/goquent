package api_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/api"
	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/database/postgres"
)

func TestSelectApiBuilder(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *api.SelectQueryBuilder
		expectedQuery  string
		expectedValues []interface{}
	}{
		{
			"Select",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id", "name")
			},
			"SELECT `id`, `name` FROM ``",
			nil,
		},
		{
			"SelectRaw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).SelectRaw("COUNT(*) as total")
			},
			"SELECT COUNT(*) as total FROM ``",
			nil,
		},
		{
			"Count",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Count()
			},
			"SELECT COUNT(*) FROM ``",
			nil,
		},
		{
			"Count_Columns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Count("id")
			},
			"SELECT COUNT(`id`) FROM ``",
			nil,
		},
		{
			"Count_Distinct",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Count("id").Distinct("id")
			},
			"SELECT COUNT(DISTINCT `id`) FROM ``",
			nil,
		},
		{
			"Count_Distinct_Columns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Distinct("id", "name").Count("id", "name")
			},
			"SELECT COUNT(DISTINCT `id`), COUNT(DISTINCT `name`) FROM ``",
			nil,
		},
		{
			"Distincts",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Distinct("id", "name")
			},
			"SELECT DISTINCT `id`, `name` FROM ``",
			nil,
		},
		{
			"Max",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Max("price")
			},
			"SELECT MAX(`price`) FROM ``",
			nil,
		},
		{
			"Min",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Min("price")
			},
			"SELECT MIN(`price`) FROM ``",
			nil,
		},
		{
			"Sum",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Sum("price")
			},
			"SELECT SUM(`price`) FROM ``",
			nil,
		},
		{
			"Avg",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Avg("price")
			},
			"SELECT AVG(`price`) FROM ``",
			nil,
		},
		{
			"SelectRaw_With_Value",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).SelectRaw("`price` * ? as `price_with_tax`", 1.0825)
			},
			"SELECT `price` * ? as `price_with_tax` FROM ``",
			[]interface{}{1.0825},
		},
		{
			"From",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Table("users")
			},
			"SELECT * FROM `users`",
			nil,
		},
		{
			"From_With_Alias",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Table("users as u")
			},
			"SELECT * FROM `users` as `u`",
			nil,
		},
		{
			"Inner_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Join("orders", "users.id", "=", "orders.user_id")
			},
			"SELECT `orders`.*, ``.* FROM `` INNER JOIN `orders` ON `users`.`id` = `orders`.`user_id`",
			nil,
		},
		{
			"Left_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).LeftJoin("orders", "users.id", "=", "orders.user_id")
			},
			"SELECT `orders`.*, ``.* FROM `` LEFT JOIN `orders` ON `users`.`id` = `orders`.`user_id`",
			nil,
		},
		{
			"Right_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).RightJoin("orders", "users.id", "=", "orders.user_id")
			},
			"SELECT `orders`.*, ``.* FROM `` RIGHT JOIN `orders` ON `users`.`id` = `orders`.`user_id`",
			nil,
		},
		{
			"Cross_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).CrossJoin("orders")
			},
			"SELECT `orders`.*, ``.* FROM `` CROSS JOIN `orders`",
			nil,
		},
		{
			"Join_With_Alias",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Join("orders as o", "users.id", "=", "o.user_id")
			},
			"SELECT `o`.*, ``.* FROM `` INNER JOIN `orders` as `o` ON `users`.`id` = `o`.`user_id`",
			nil,
		},
		{
			"Join_and_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Join("orders", "users.id", "=", "orders.user_id").Join("products", "orders.product_id", "=", "products.id")
			},
			"SELECT `orders`.*, ``.*, `products`.* FROM `` INNER JOIN `orders` ON `users`.`id` = `orders`.`user_id` INNER JOIN `products` ON `orders`.`product_id` = `products`.`id`",
			nil,
		},
		{
			"JoinQuery",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).JoinQuery("users", func(b *api.JoinClauseQueryBuilder) {
					b.On("users.id", "=", "profiles.user_id").OrOn("users.id", "=", "profiles.alter_user_id").Where("profiles.age", ">", 18)
				})
			},
			"SELECT `users`.* FROM `` INNER JOIN `users` ON `users`.`id` = `profiles`.`user_id` OR `users`.`id` = `profiles`.`alter_user_id` AND `profiles`.`age` > ?",
			[]interface{}{18},
		},
		{
			"LeftJoinQuery",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).LeftJoinQuery("users", func(b *api.JoinClauseQueryBuilder) {
					b.On("users.id", "=", "profiles.user_id").OrOn("users.id", "=", "profiles.alter_user_id").Where("profiles.age", ">", 18)
				})
			},
			"SELECT `users`.* FROM `` LEFT JOIN `users` ON `users`.`id` = `profiles`.`user_id` OR `users`.`id` = `profiles`.`alter_user_id` AND `profiles`.`age` > ?",
			[]interface{}{18},
		},
		{
			"RightJoinQuery",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).RightJoinQuery("users", func(b *api.JoinClauseQueryBuilder) {
					b.On("users.id", "=", "profiles.user_id").OrOn("users.id", "=", "profiles.alter_user_id").Where("profiles.age", ">", 18)
				})
			},
			"SELECT `users`.* FROM `` RIGHT JOIN `users` ON `users`.`id` = `profiles`.`user_id` OR `users`.`id` = `profiles`.`alter_user_id` AND `profiles`.`age` > ?",
			[]interface{}{18},
		},
		{
			"JoinQuery_With_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Select("id", "users.name as name").
					JoinQuery("profiles", func(b *api.JoinClauseQueryBuilder) {
						b.On("users.id", "=", "profiles.user_id").
							Where("profiles.age", ">", 18)
					}).
					Join("addresses", "users.id", "=", "addresses.user_id").
					OrderBy("users.name", "ASC")
			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` AND `profiles`.`age` > ? INNER JOIN `addresses` ON `users`.`id` = `addresses`.`user_id` ORDER BY `users`.`name` ASC",
			[]interface{}{18},
		},
		{
			"JoinSubQuery_with_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Select("id", "users.name as name").
					JoinSubQuery(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("profiles").Where("age", ">", 18), "profiles", "users.id", "=", "profiles.user_id").
					Join("addresses", "users.id", "=", "addresses.user_id").
					OrderBy("users.name", "ASC")
			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN (SELECT `id` FROM `profiles` WHERE `age` > ?) as `profiles` ON `users`.`id` = `profiles`.`user_id` INNER JOIN `addresses` ON `users`.`id` = `addresses`.`user_id` ORDER BY `users`.`name` ASC",
			[]interface{}{18},
		},
		{
			"Multiple_JoinSubQuery_with_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Select("id", "users.name as name").
					JoinSubQuery(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("profiles").Where("age", ">", 18), "profiles", "users.id", "=", "profiles.user_id").
					JoinSubQuery(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("addresses").Where("city", "=", "New York"), "addresses", "users.id", "=", "addresses.user_id").
					OrderBy("users.name", "ASC")
			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN (SELECT `id` FROM `profiles` WHERE `age` > ?) as `profiles` ON `users`.`id` = `profiles`.`user_id` INNER JOIN (SELECT `id` FROM `addresses` WHERE `city` = ?) as `addresses` ON `users`.`id` = `addresses`.`user_id` ORDER BY `users`.`name` ASC",
			[]interface{}{18, "New York"},
		},
		{
			"JoinSub",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).JoinSubQuery(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("profiles").Where("age", ">", 18), "profiles", "users.id", "=", "profiles.user_id")
			},
			"SELECT `profiles`.*, ``.* FROM `` INNER JOIN (SELECT `id` FROM `profiles` WHERE `age` > ?) as `profiles` ON `users`.`id` = `profiles`.`user_id`",
			[]interface{}{18},
		},
		{
			"Lateral_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).JoinLateral(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("profiles").Where("age", ">", 18), "profiles")
			},
			"SELECT `profiles`.*, ``.* FROM `` ,LATERAL(SELECT `id` FROM `profiles` WHERE `age` > ?) as `profiles`",
			[]interface{}{18},
		},
		{
			"LeftLateral_Join",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).LeftJoinLateral(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("profiles").Where("age", ">", 18), "profiles")
			},
			"SELECT `profiles`.*, ``.* FROM `` ,LEFT LATERAL(SELECT `id` FROM `profiles` WHERE `age` > ?) as `profiles`",
			[]interface{}{18},
		},
		{
			"OrderBy",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).OrderBy("name", "asc")
			},
			"SELECT * FROM `` ORDER BY `name` ASC",
			nil,
		},
		{
			"OrderByDesc_And_ReOrder",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).OrderBy("name", "asc").ReOrder().OrderBy("name", "desc")
			},
			"SELECT * FROM `` ORDER BY `name` DESC",
			nil,
		},
		{
			"OrderByRaw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).OrderByRaw("RAND()")
			},
			"SELECT * FROM `` ORDER BY RAND()",
			nil,
		},
		{
			"GroupBy",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).GroupBy("name", "age")
			},
			"SELECT * FROM `` GROUP BY `name`, `age`",
			nil,
		},
		{
			"GroupBy_Having",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).GroupBy("name", "age").Having("age", ">", 18)
			},
			"SELECT * FROM `` GROUP BY `name`, `age` HAVING `age` > ?",
			[]interface{}{18},
		},
		{
			"GroupBy_Having_OR",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).GroupBy("name", "age").Having("age", ">", 18).OrHaving("name", "=", "John")
			},
			"SELECT * FROM `` GROUP BY `name`, `age` HAVING `age` > ? OR `name` = ?",
			[]interface{}{18, "John"},
		},
		{
			"GroupBy_Having_Raw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).GroupBy("name", "age").HavingRaw("`age` > 18")
			},
			"SELECT * FROM `` GROUP BY `name`, `age` HAVING `age` > 18",
			nil,
		},
		{
			"GroupBy_HavingRaw_OR",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).GroupBy("name", "age").HavingRaw("`age` > 18").OrHavingRaw("`name` = 'John'")
			},
			"SELECT * FROM `` GROUP BY `name`, `age` HAVING `age` > 18 OR `name` = 'John'",
			nil,
		},
		{
			"Limit",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Limit(10)
			},
			"SELECT * FROM `` LIMIT 10",
			nil,
		},
		{
			"Offset",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Offset(10)
			},
			"SELECT * FROM `` OFFSET 10",
			nil,
		},
		{
			"Limit_And_Offset",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Limit(10).Offset(5)
			},
			"SELECT * FROM `` LIMIT 10 OFFSET 5",
			nil,
		},
		{
			"Lock FOR UPDATE",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).LockForUpdate()
			},
			"SELECT * FROM `` FOR UPDATE",
			nil,
		},
		{
			"Lock_Shared",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).SharedLock()
			},
			"SELECT * FROM `` LOCK IN SHARE MODE",
			nil,
		},
		{
			"Union",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Union(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")).Where("age", ">", 18)
			},
			"SELECT `id` FROM `users` WHERE `name` = ? UNION SELECT * FROM `` WHERE `age` > ?",
			[]interface{}{"John", 18},
		},
		{
			"Union_All",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).UnionAll(api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")).Where("age", ">", 18)
			},
			"SELECT `id` FROM `users` WHERE `name` = ? UNION ALL SELECT * FROM `` WHERE `age` > ?",
			[]interface{}{"John", 18},
		},
		{
			"Complex_Query",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Select("id", "name").
					Table("users").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("age", ">", 18).
					OrderBy("name", "ASC")
			},
			"SELECT `id`, `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `age` > ? ORDER BY `name` ASC",
			[]interface{}{18},
		},
		{
			"Complex_Query_With_Subquery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					SelectRaw("`id`, `name`, `profiles`.`point` * ? as `profiles_point`", 1.05).
					Table("users").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("status", "=", "active").
					WhereSubQuery("user_id", "IN", sq).
					Where("age", ">", 18).
					OrderBy("name", "ASC")
			},
			"SELECT `id`, `name`, `profiles`.`point` * ? as `profiles_point` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `status` = ? AND `user_id` IN (SELECT `id` FROM `users` WHERE `name` = ?) AND `age` > ? ORDER BY `name` ASC",
			[]interface{}{1.05, "active", "John", 18},
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

func TestWhereSelectBuilder(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *api.SelectQueryBuilder
		expectedQuery  string
		expectedValues []interface{}
	}{
		{
			"Where",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 18)
			},
			"SELECT * FROM `` WHERE `age` > ?",
			[]interface{}{18},
		},
		{
			"OrWhere",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("email", "LIKE", "%@gmail.com%").OrWhere("email", "LIKE", "%@yahoo.com%").OrWhere("age", ">", 18)
			},
			"SELECT * FROM `` WHERE `email` LIKE ? OR `email` LIKE ? OR `age` > ?",
			[]interface{}{"%@gmail.com%", "%@yahoo.com%", 18},
		},
		{
			"WhereRaw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereRaw("`age` > :age", map[string]interface{}{"age": 18})
			},
			"SELECT * FROM `` WHERE `age` > ?",
			[]interface{}{18},
		},
		{
			"OrWhereRaw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereRaw("`age` > :age", map[string]interface{}{"age": 18}).OrWhereRaw("`name`= :name", map[string]interface{}{"name": "John"})
			},
			"SELECT * FROM `` WHERE `age` > ? OR `name`= ?",
			[]interface{}{18, "John"},
		},
		{
			"SafeWhereRaw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).SafeWhereRaw("`age` > :age", map[string]any{"age": 30})
			},
			"SELECT * FROM `` WHERE `age` > ?",
			[]interface{}{30},
		},
		{
			"SafeOrWhereRaw",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).SafeWhereRaw("`age` > :age", map[string]any{"age": 30}).SafeOrWhereRaw("`name`= :name", map[string]any{"name": "Ann"})
			},
			"SELECT * FROM `` WHERE `age` > ? OR `name`= ?",
			[]interface{}{30, "Ann"},
		},
		{
			"WhereQuery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereSubQuery("user_id", "IN", sq).Where("city", "=", "New York")
			},
			"SELECT * FROM `` WHERE `user_id` IN (SELECT `id` FROM `users` WHERE `name` = ?) AND `city` = ?",
			[]interface{}{"John", "New York"},
		},
		{
			"OrWhereQuery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereSubQuery("user_id", "IN", sq)
			},
			"SELECT * FROM `` WHERE `city` = ? OR `user_id` IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"New York", "John"},
		},
		{
			"WhereGroup",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					WhereGroup(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					})
			},
			"SELECT * FROM `` WHERE (`age` > ? AND `name` = ?)",
			[]interface{}{18, "John"},
		},
		{
			"WhereGroup_And",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					WhereGroup(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					}).Where("city", "=", "New York")
			},
			"SELECT * FROM `` WHERE (`age` > ? AND `name` = ?) AND `city` = ?",
			[]interface{}{18, "John", "New York"},
		},
		{
			"WhereGroup_And_2",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Where("city", "=", "New York").
					WhereGroup(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					})
			},
			"SELECT * FROM `` WHERE `city` = ? AND (`age` > ? AND `name` = ?)",
			[]interface{}{"New York", 18, "John"},
		},
		{
			"WhereGroup_Or",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					WhereGroup(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					}).OrWhere("city", "=", "New York")
			},
			"SELECT * FROM `` WHERE (`age` > ? AND `name` = ?) OR `city` = ?",
			[]interface{}{18, "John", "New York"},
		},
		{
			"WhereGroup_Or_2",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Where("city", "=", "New York").
					OrWhereGroup(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					})
			},
			"SELECT * FROM `` WHERE `city` = ? OR (`age` > ? AND `name` = ?)",
			[]interface{}{"New York", 18, "John"},
		},
		{
			"WhereNot",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					WhereNot(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					})
			},
			"SELECT * FROM `` WHERE NOT (`age` > ? AND `name` = ?)",
			[]interface{}{18, "John"},
		},
		{
			"OrWhereNot",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Where("city", "=", "New York").
					OrWhereNot(func(b *api.WhereSelectQueryBuilder) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					})
			},
			"SELECT * FROM `` WHERE `city` = ? OR NOT (`age` > ? AND `name` = ?)",
			[]interface{}{"New York", 18, "John"},
		},
		{
			"WhereAll",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Where("age", ">", 18).
					WhereAll([]string{"name", "city"}, "LIKE", "%test%")
			},
			"SELECT * FROM `` WHERE `age` > ? AND (`name` LIKE ? AND `city` LIKE ?)",
			[]interface{}{18, "%test%", "%test%"},
		},
		{
			"WhereAny",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).
					Where("age", ">", 18).
					WhereAny([]string{"name", "city"}, "LIKE", "%test%")
			},
			"SELECT * FROM `` WHERE `age` > ? AND (`name` LIKE ? OR `city` LIKE ?)",
			[]interface{}{18, "%test%", "%test%"},
		},
		{
			"WhereIn",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereIn("id", []int64{1, 2, 3})
			},
			"SELECT * FROM `` WHERE `id` IN (?, ?, ?)",
			[]interface{}{int64(1), int64(2), int64(3)},
		},
		{
			"WhereIn (Subquery)",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereIn("id", sq)
			},
			"SELECT * FROM `` WHERE `id` IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"WhereNotIn",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotIn("id", []int64{1, 2, 3})
			},
			"SELECT * FROM `` WHERE `id` NOT IN (?, ?, ?)",
			[]interface{}{int64(1), int64(2), int64(3)},
		},
		{
			"WhereNotIn (Subquery)",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotIn("id", sq)
			},
			"SELECT * FROM `` WHERE `id` NOT IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"OrWhereIn",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 19).OrWhereIn("id", []int64{1, 2, 3})
			},
			"SELECT * FROM `` WHERE `age` > ? OR `id` IN (?, ?, ?)",
			[]interface{}{19, int64(1), int64(2), int64(3)},
		},
		{
			"OrWhereIn (Subquery)",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 19).OrWhereIn("id", sq)
			},
			"SELECT * FROM `` WHERE `age` > ? OR `id` IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{19, "John"},
		},
		{
			"OrWhereNotIn",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 19).OrWhereNotIn("id", []int64{1, 2, 3})
			},
			"SELECT * FROM `` WHERE `age` > ? OR `id` NOT IN (?, ?, ?)",
			[]interface{}{19, int64(1), int64(2), int64(3)},
		},
		{
			"OrWhereNotIn (Subquery)",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 19).OrWhereNotIn("id", sq)
			},
			"SELECT * FROM `` WHERE `age` > ? OR `id` NOT IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{19, "John"},
		},
		{
			"WhereInSubquery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereInSubQuery("id", sq)
			},
			"SELECT * FROM `` WHERE `id` IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},

		{
			"WhereNotInSubquery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotInSubQuery("id", sq)
			},
			"SELECT * FROM `` WHERE `id` NOT IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"OrWhereInSubquery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 19).OrWhereInSubQuery("id", sq)
			},
			"SELECT * FROM `` WHERE `age` > ? OR `id` IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{19, "John"},
		},
		{
			"OrWhereNotInSubquery",
			func() *api.SelectQueryBuilder {
				sq := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 19).OrWhereNotInSubQuery("id", sq)
			},
			"SELECT * FROM `` WHERE `age` > ? OR `id` NOT IN (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{19, "John"},
		},
		{
			"WhereNull",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNull("deleted_at")
			},
			"SELECT * FROM `` WHERE `deleted_at` IS NULL",
			nil,
		},
		{
			"WhereNotNull",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotNull("deleted_at")
			},
			"SELECT * FROM `` WHERE `deleted_at` IS NOT NULL",
			nil,
		},
		{
			"OrWhereNull",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 18).OrWhereNull("deleted_at")
			},
			"SELECT * FROM `` WHERE `age` > ? OR `deleted_at` IS NULL",
			[]interface{}{18},
		},
		{
			"OrWhereNotNull",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 18).OrWhereNotNull("deleted_at")
			},
			"SELECT * FROM `` WHERE `age` > ? OR `deleted_at` IS NOT NULL",
			[]interface{}{18},
		},
		{
			"WhereColumn",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereColumn([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "=", "updated_at")
			},
			"SELECT * FROM `` WHERE `created_at` = `updated_at`",
			nil,
		},
		{
			"WhereColumn_With_Operator",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereColumn([]string{"created_at", "updated_at", "deleted_at"}, "created_at", ">", "updated_at")
			},
			"SELECT * FROM `` WHERE `created_at` > `updated_at`",
			nil,
		},
		{
			"OrWhereColumn",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 18).OrWhereColumn([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "=", "updated_at")
			},
			"SELECT * FROM `` WHERE `age` > ? OR `created_at` = `updated_at`",
			[]interface{}{18},
		},
		{
			"OrWhereColumn_With_Operator",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("age", ">", 18).OrWhereColumn([]string{"created_at", "updated_at", "deleted_at"}, "created_at", ">", "updated_at")
			},
			"SELECT * FROM `` WHERE `age` > ? OR `created_at` > `updated_at`",
			[]interface{}{18},
		},
		{
			"WhereColumns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereColumns([]string{"created_at", "updated_at", "deleted_at"}, [][]string{{"created_at", "=", "updated_at"}, {"deleted_at", "=", "updated_at"}})
			},
			"SELECT * FROM `` WHERE `created_at` = `updated_at` AND `deleted_at` = `updated_at`",
			nil,
		},
		{
			"OrWhereColumns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).OrWhereColumns([]string{"created_at", "updated_at", "deleted_at"}, [][]string{{"created_at", "=", "updated_at"}, {"deleted_at", "=", "updated_at"}})
			},
			"SELECT * FROM `` WHERE `created_at` = `updated_at` OR `deleted_at` = `updated_at`",
			nil,
		},
		{
			"WhereBetween",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereBetween("age", 18, 30)
			},
			"SELECT * FROM `` WHERE `age` BETWEEN ? AND ?",
			[]interface{}{18, 30},
		},
		{
			"OrWhereBetween",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereBetween("age", 18, 30)
			},
			"SELECT * FROM `` WHERE `city` = ? OR `age` BETWEEN ? AND ?",
			[]interface{}{"New York", 18, 30},
		},
		{
			"WhereNotBetween",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotBetween("age", 18, 30)
			},
			"SELECT * FROM `` WHERE `age` NOT BETWEEN ? AND ?",
			[]interface{}{18, 30},
		},
		{
			"OrWhereNotBetween",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereNotBetween("age", 18, 30)
			},
			"SELECT * FROM `` WHERE `city` = ? OR `age` NOT BETWEEN ? AND ?",
			[]interface{}{"New York", 18, 30},
		},
		{
			"WhereBetweenColumns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereBetweenColumns([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "updated_at", "deleted_at")
			},
			"SELECT * FROM `` WHERE `created_at` BETWEEN `updated_at` AND `deleted_at`",
			nil,
		},
		{
			"OrWhereBetweenColumns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).OrWhereBetweenColumns([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "updated_at", "deleted_at")
			},
			"SELECT * FROM `` WHERE `created_at` BETWEEN `updated_at` AND `deleted_at`",
			nil,
		},
		{
			"WhereNotBetweenColumns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotBetweenColumns([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "updated_at", "deleted_at")
			},
			"SELECT * FROM `` WHERE `created_at` NOT BETWEEN `updated_at` AND `deleted_at`",
			nil,
		},
		{
			"OrWhereNotBetweenColumns",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).OrWhereNotBetweenColumns([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "updated_at", "deleted_at")
			},
			"SELECT * FROM `` WHERE `created_at` NOT BETWEEN `updated_at` AND `deleted_at`",
			nil,
		},
		{
			"WhereExists",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereExists(func(b *api.SelectQueryBuilder) {
					b.Select("id").Table("users").Where("name", "=", "John")
				})
			},
			"SELECT * FROM `` WHERE EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"OrWhereExists",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereExists(func(b *api.SelectQueryBuilder) {
					b.Select("id").Table("users").Where("name", "=", "John")
				})
			},
			"SELECT * FROM `` WHERE `city` = ? OR EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"New York", "John"},
		},
		{
			"WhereNotExists",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotExists(func(b *api.SelectQueryBuilder) {
					b.Select("id").Table("users").Where("name", "=", "John")
				})
			},
			"SELECT * FROM `` WHERE NOT EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"OrWhereNotExists",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereNotExists(func(b *api.SelectQueryBuilder) {
					b.Select("id").Table("users").Where("name", "=", "John")
				})
			},
			"SELECT * FROM `` WHERE `city` = ? OR NOT EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"New York", "John"},
		},
		{
			"WhereExistsQuery",
			func() *api.SelectQueryBuilder {
				q := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereExistsSubQuery(q)
			},
			"SELECT * FROM `` WHERE EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"OrWhereExistsQuery",
			func() *api.SelectQueryBuilder {
				q := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereExistsSubQuery(q)
			},
			"SELECT * FROM `` WHERE `city` = ? OR EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"New York", "John"},
		},
		{
			"WhereNotExistsQuery",
			func() *api.SelectQueryBuilder {
				q := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereNotExistsQuery(q)
			},
			"SELECT * FROM `` WHERE NOT EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"John"},
		},
		{
			"OrWhereNotExistsQuery",
			func() *api.SelectQueryBuilder {
				q := api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Select("id").Table("users").Where("name", "=", "John")
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereNotExistsQuery(q)
			},
			"SELECT * FROM `` WHERE `city` = ? OR NOT EXISTS (SELECT `id` FROM `users` WHERE `name` = ?)",
			[]interface{}{"New York", "John"},
		},
		{
			"WhereFullText_MySQL",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereFullText([]string{"name", "note"}, "John Doe", map[string]interface{}{"mode": "boolean"})
			},
			"SELECT * FROM `` WHERE MATCH (`name`, `note`) AGAINST (? IN BOOLEAN MODE)",
			[]interface{}{"John Doe"},
		},
		{
			"WhereFullText_PostgreSQL",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(postgres.NewPostgreSQLQueryBuilder()).WhereFullText([]string{"name", "note"}, "John Doe", map[string]interface{}{"language": "english"})
			},
			`SELECT * FROM "" WHERE (to_tsvector($1, "name") || to_tsvector($2, "note")) @@ plainto_tsquery($3, $4)`,
			[]interface{}{"english", "english", "english", "John Doe"},
		},
		{
			"OrWhereFullText_MySQL",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).Where("city", "=", "New York").OrWhereFullText([]string{"name", "note"}, "John Doe", map[string]interface{}{"mode": "boolean"})
			},
			"SELECT * FROM `` WHERE `city` = ? OR MATCH (`name`, `note`) AGAINST (? IN BOOLEAN MODE)",
			[]interface{}{"New York", "John Doe"},
		},
		{

			"OrWhereFullText_PostgreSQL",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(postgres.NewPostgreSQLQueryBuilder()).Where("city", "=", "New York").OrWhereFullText([]string{"name", "note"}, "John Doe", map[string]interface{}{"language": "english"})
			},
			`SELECT * FROM "" WHERE "city" = $1 OR (to_tsvector($2, "name") || to_tsvector($3, "note")) @@ plainto_tsquery($4, $5)`,
			[]interface{}{"New York", "english", "english", "english", "John Doe"},
		},
		{
			"WhereDate",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereDate("created_at", "=", "2021-01-01")
			},
			"SELECT * FROM `` WHERE DATE(`created_at`) = ?",
			[]interface{}{"2021-01-01"},
		},
		{
			"WhereTime",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereTime("created_at", "=", "12:00:00")
			},
			"SELECT * FROM `` WHERE TIME(`created_at`) = ?",
			[]interface{}{"12:00:00"},
		},
		{
			"WhereDay",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereDay("created_at", "=", "1")
			},
			"SELECT * FROM `` WHERE DAY(`created_at`) = ?",
			[]interface{}{"1"},
		},
		{
			"WhereMonth",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereMonth("created_at", "=", "1")
			},
			"SELECT * FROM `` WHERE MONTH(`created_at`) = ?",
			[]interface{}{"1"},
		},
		{
			"WhereYear",
			func() *api.SelectQueryBuilder {
				return api.NewSelectQueryBuilder(mysql.NewMySQLQueryBuilder()).WhereYear("created_at", "=", "2021")
			},
			"SELECT * FROM `` WHERE YEAR(`created_at`) = ?",
			[]interface{}{"2021"},
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
