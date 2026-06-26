package query_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/query"
)

func TestDeleteBuilder(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *query.DeleteBuilder
		expectedQuery  string
		expectedValues []interface{}
	}{
		{
			"Delete_all",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Delete()
			},
			"DELETE FROM `users`",
			[]interface{}{},
		},
		{
			"Delete_where",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Where("id", "=", 1).
					Delete()
			},
			"DELETE FROM `users` WHERE `id` = ?",
			[]interface{}{1},
		},
		{
			"Delete_where_not",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Where("id", "!=", 1).
					OrWhereNot(func(b *query.WhereBuilder[query.DeleteBuilder]) {
						b.Where("age", ">", 18).Where("name", "=", "John")
					}).
					Delete()
			},
			"DELETE FROM `users` WHERE `id` != ? OR NOT (`age` > ? AND `name` = ?)",
			[]interface{}{1, 18, "John"},
		},
		{
			"Delete_where_in",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereIn("id", []interface{}{1, 2, 3}).
					Delete()
			},
			"DELETE FROM `users` WHERE `id` IN (?, ?, ?)",
			[]interface{}{1, 2, 3},
		},
		{
			"Delete_where_not_in",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereNotIn("id", []interface{}{1, 2, 3}).
					Delete()
			},
			"DELETE FROM `users` WHERE `id` NOT IN (?, ?, ?)",
			[]interface{}{1, 2, 3},
		},
		{
			"Delete_where_any",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereAny([]string{"name", "note"}, "LIKE", "%test%").
					Delete()
			},
			"DELETE FROM `users` WHERE (`name` LIKE ? OR `note` LIKE ?)",
			[]interface{}{"%test%", "%test%"},
		},
		{
			"Delete_where_all",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Where("id", ">", 10000).
					WhereAll([]string{"firstname", "lastname"}, "LIKE", "%test%").
					Delete()
			},
			"DELETE FROM `users` WHERE `id` > ? AND (`firstname` LIKE ? AND `lastname` LIKE ?)",
			[]interface{}{10000, "%test%", "%test%"},
		},
		{
			"Delete_where_null",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereNull("name").
					Delete()
			},
			"DELETE FROM `users` WHERE `name` IS NULL",
			[]interface{}{},
		},
		{
			"Delete_where_not_null",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereNotNull("name").
					Delete()
			},
			"DELETE FROM `users` WHERE `name` IS NOT NULL",
			[]interface{}{},
		},
		{
			"Delete_where_column",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereColumn([]string{"name", "note"}, "name", "=", "note").
					Delete()
			},
			"DELETE FROM `users` WHERE `name` = `note`",
			[]interface{}{},
		},
		{
			"Delete_where_between",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereBetween("age", 18, 30).
					Delete()
			},
			"DELETE FROM `users` WHERE `age` BETWEEN ? AND ?",
			[]interface{}{18, 30},
		},
		{
			"Delete_where_not_between",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereNotBetween("age", 18, 30).
					Delete()
			},
			"DELETE FROM `users` WHERE `age` NOT BETWEEN ? AND ?",
			[]interface{}{18, 30},
		},
		{
			"Delete_where_between_columns",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					WhereBetweenColumns([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "updated_at", "deleted_at").
					Delete()
			},
			"DELETE FROM `users` WHERE `created_at` BETWEEN `updated_at` AND `deleted_at`",
			[]interface{}{},
		},
		{
			"Delete_JOINS",
			func() *query.DeleteBuilder {
				return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
					Table("users").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("age", ">", 18).
					Delete()
			},
			"DELETE `users` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `age` > ?",
			[]interface{}{18},
		},
		/*
			{
				"Delete_using",
				func() *query.DeleteBuilder {
					return query.NewDeleteBuilder(mysql.NewMySQLQueryBuilder()).
						Table("users").
						Using(query.NewBuilder(mysql.NewMySQLQueryBuilder()).
							Table("profiles").
							Select("name", "age").
							Where("age", ">", 18).GetQuery()).
						Delete()
				},
				"DELETE users FROM users USING (SELECT name, age FROM profiles WHERE age > ?)",
				[]interface{}{18},
			},
		*/
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
