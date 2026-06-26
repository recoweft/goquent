package api_test

import (
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/api"
	"github.com/recoweft/goquent/orm/internal/querybuilder/database/mysql"
	"github.com/recoweft/goquent/orm/internal/querybuilder/database/postgres"
)

func TestSelectDebugApiRawSqlTest(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *api.SelectQueryBuilder
		expectedQuery string
	}{
		{
			"Complex_Query_With_Union",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				usq := api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("profiles.age", ">", 18)

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("area", "=", "Jakarta").
					WhereBetween("profiles.age", 18, 30).
					OrderBy("users.name", "ASC").
					OrderBy("profiles.age", "DESC").
					GroupBy("users.id").
					HavingRaw("COUNT(`profiles`.`id`) > 1").
					WhereIn("users.id", usq).
					Union(
						api.NewSelectQueryBuilder(dbStrategy).
							Table("users").
							Select("id", "users.name as name").
							Join("profiles", "users.id", "=", "profiles.user_id").
							Where("area", "=", "Jakarta").
							WhereBetween("profiles.age", 18, 30).
							OrderBy("users.name", "ASC").
							OrderBy("profiles.age", "DESC").
							GroupBy("users.id").
							HavingRaw("COUNT(`profiles`.`id`) > 1").
							WhereIn("users.id", usq),
					)

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = 'Jakarta' AND `profiles`.`age` BETWEEN 18 AND 30 AND `users`.`id` IN (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > 18) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC UNION SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = 'Jakarta' AND `profiles`.`age` BETWEEN 18 AND 30 AND `users`.`id` IN (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > 18) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC",
		},

		{
			"Complex_Query_With_WhereExists",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("area", "=", "Jakarta").
					WhereExists(func(q *api.SelectQueryBuilder) {
						q.Table("users").
							Select("id").
							Join("profiles", "users.id", "=", "profiles.user_id").
							Where("profiles.age", ">", 18)
					}).
					OrderBy("users.name", "ASC").
					OrderBy("profiles.age", "DESC").
					GroupBy("users.id").
					HavingRaw("COUNT(`profiles`.`id`) > 1")

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = 'Jakarta' AND EXISTS (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > 18) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC",
		},
		{
			"Complex_Query_With_OrWhereNotExists",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("area", "=", "Jakarta").
					OrWhereNotExists(func(q *api.SelectQueryBuilder) {
						q.Table("users").
							Select("id").
							Join("profiles", "users.id", "=", "profiles.user_id").
							Where("profiles.age", ">", 18)
					}).
					OrderBy("users.name", "ASC").
					OrderBy("profiles.age", "DESC").
					GroupBy("users.id").
					HavingRaw("COUNT(`profiles`.`id`) > 1")

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = 'Jakarta' OR NOT EXISTS (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > 18) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC",
		},
		{
			"Complex_Query",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				usq := api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("profiles.age", ">", 18)

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("area", "=", "Jakarta").
					WhereBetween("profiles.age", 18, 30).
					OrderBy("users.name", "ASC").
					OrderBy("profiles.age", "DESC").
					GroupBy("users.id").
					HavingRaw("COUNT(`profiles`.`id`) > 1").
					WhereIn("users.id", usq)

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = 'Jakarta' AND `profiles`.`age` BETWEEN 18 AND 30 AND `users`.`id` IN (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > 18) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC",
		},
		{
			"Normal_Query",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("profiles.age", ">", 18).
					Where("deleted_at", "IS", nil).
					OrderBy("users.name", "ASC")

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > 18 AND `deleted_at` IS NULL ORDER BY `users`.`name` ASC",
		},
		{
			"Normal_Query_With_WhereGroup",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					WhereGroup(func(w *api.WhereSelectQueryBuilder) {
						w.Where("profiles.age", ">", 18).Where("profiles.age", "<", 30)
					}).OrderBy("users.name", "ASC")
			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE (`profiles`.`age` > 18 AND `profiles`.`age` < 30) ORDER BY `users`.`name` ASC",
		},
		{
			"Simple_Query",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name")

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tt.setup()
			query, _ := builder.RawSql()

			if query != tt.expectedQuery {
				t.Errorf("expected '%s' but got '%s'", tt.expectedQuery, query)
			}

		})
	}
}

func TestSelectDebugApiRawSqlPostgreSQLConsistency(t *testing.T) {
	t.Parallel()

	builder := api.NewSelectQueryBuilder(postgres.NewPostgreSQLQueryBuilder()).
		Table("app.feed_entries AS feed_entries").
		SelectRaw(`CASE WHEN viewer_sub.id IS NOT NULL AND viewer_sub.status::text <> ? THEN TRUE ELSE FALSE END AS muted_source`, "active").
		SelectRaw(`(SELECT recommendations.reason_text FROM app.recommendations AS recommendations WHERE recommendations.user_id = ? AND recommendations.content_item_id = feed_entries.content_item_id AND recommendations.slot = ? ORDER BY recommendations.score DESC, recommendations.generated_at DESC LIMIT 1) AS recommendation_reason`, 42, "home").
		WhereRaw(`coalesce(user_content_states.inbox_state::text, :archived_default) <> :archived`, map[string]any{
			"archived_default": "active",
			"archived":         "archived",
		})

	expectedQuery := `SELECT CASE WHEN viewer_sub.id IS NOT NULL AND viewer_sub.status::text <> $1 THEN TRUE ELSE FALSE END AS muted_source, (SELECT recommendations.reason_text FROM app.recommendations AS recommendations WHERE recommendations.user_id = $2 AND recommendations.content_item_id = feed_entries.content_item_id AND recommendations.slot = $3 ORDER BY recommendations.score DESC, recommendations.generated_at DESC LIMIT 1) AS recommendation_reason FROM "app"."feed_entries" as "feed_entries" WHERE coalesce(user_content_states.inbox_state::text, $4) <> $5`
	expectedValues := []interface{}{"active", 42, "home", "active", "archived"}
	expectedRawSQL := `SELECT CASE WHEN viewer_sub.id IS NOT NULL AND viewer_sub.status::text <> 'active' THEN TRUE ELSE FALSE END AS muted_source, (SELECT recommendations.reason_text FROM app.recommendations AS recommendations WHERE recommendations.user_id = 42 AND recommendations.content_item_id = feed_entries.content_item_id AND recommendations.slot = 'home' ORDER BY recommendations.score DESC, recommendations.generated_at DESC LIMIT 1) AS recommendation_reason FROM "app"."feed_entries" as "feed_entries" WHERE coalesce(user_content_states.inbox_state::text, 'active') <> 'archived'`

	dumpQuery, dumpValues, err := builder.Dump()
	if err != nil {
		t.Fatalf("Dump returned error: %v", err)
	}
	if dumpQuery != expectedQuery {
		t.Fatalf("expected dump query %q but got %q", expectedQuery, dumpQuery)
	}
	if len(dumpValues) != len(expectedValues) {
		t.Fatalf("expected dump values %v but got %v", expectedValues, dumpValues)
	}
	for i := range dumpValues {
		if dumpValues[i] != expectedValues[i] {
			t.Fatalf("expected dump value %v at index %d but got %v", expectedValues[i], i, dumpValues[i])
		}
	}

	buildQuery, buildValues, err := builder.Build()
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if buildQuery != expectedQuery {
		t.Fatalf("expected build query %q but got %q", expectedQuery, buildQuery)
	}
	if len(buildValues) != len(expectedValues) {
		t.Fatalf("expected build values %v but got %v", expectedValues, buildValues)
	}
	for i := range buildValues {
		if buildValues[i] != expectedValues[i] {
			t.Fatalf("expected build value %v at index %d but got %v", expectedValues[i], i, buildValues[i])
		}
	}

	rawQuery, err := builder.RawSql()
	if err != nil {
		t.Fatalf("RawSql returned error: %v", err)
	}
	if rawQuery != expectedRawSQL {
		t.Fatalf("expected raw SQL %q but got %q", expectedRawSQL, rawQuery)
	}
}

func TestInsertDebugApiRawSqlTest(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *api.InsertQueryBuilder
		expectedQuery string
	}{
		{
			"Complex_Query",
			func() *api.InsertQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewInsertQueryBuilder(dbStrategy).
					Table("users").
					Insert(map[string]interface{}{
						"name": "Joe",
						"age":  31,
					})
			},
			"INSERT INTO `users` (`age`, `name`) VALUES (31, 'Joe')",
		},
		{
			"InsertUsing",
			func() *api.InsertQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewInsertQueryBuilder(dbStrategy).
					Table("users").
					InsertUsing([]string{"name", "age"}, api.NewSelectQueryBuilder(dbStrategy).
						Table("profiles").
						Select("name", "age").
						Where("age", ">", 18)).
					Insert(map[string]interface{}{
						"name": "Joe",
						"age":  31,
					})
			},
			"INSERT INTO `users` (`name`, `age`) SELECT `name`, `age` FROM `profiles` WHERE `age` > 18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tt.setup()
			query, _ := builder.RawSql()

			if query != tt.expectedQuery {
				t.Errorf("expected '%s' but got '%s'", tt.expectedQuery, query)
			}

		})
	}
}

func TestUpdateDebugApiRawSqlTest(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *api.UpdateQueryBuilder
		expectedQuery string
	}{
		{
			"Complex_Query",
			func() *api.UpdateQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewUpdateQueryBuilder(dbStrategy).
					Table("users").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("age", ">", 18).
					Update(map[string]interface{}{
						"name": "Joe",
						"age":  31,
					})
			},
			"UPDATE `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` SET `age` = 31, `name` = 'Joe' WHERE `age` > 18",
		},
		{
			"Update_ORDER_BY",
			func() *api.UpdateQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewUpdateQueryBuilder(dbStrategy).
					Table("users").
					OrderBy("name", "ASC").
					Update(map[string]interface{}{
						"name": "Joe",
						"age":  31,
					})
			},
			"UPDATE `users` SET `age` = 31, `name` = 'Joe' ORDER BY `name` ASC",
		},
		{
			"Update_Where_Date",
			func() *api.UpdateQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewUpdateQueryBuilder(dbStrategy).
					Table("users").
					WhereDate("created_at", "=", "2021-01-01").
					Update(map[string]interface{}{
						"name": "Joe",
						"age":  31,
					})
			},
			"UPDATE `users` SET `age` = 31, `name` = 'Joe' WHERE DATE(`created_at`) = '2021-01-01'",
		},
		{
			"Update_Where_Between_Columns",
			func() *api.UpdateQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewUpdateQueryBuilder(dbStrategy).
					Table("users").
					WhereBetweenColumns([]string{"age", "min_age", "max_age"}, "age", "min_age", "max_age").
					Update(map[string]interface{}{
						"name": "Joe",
						"age":  31,
					})
			},
			"UPDATE `users` SET `age` = 31, `name` = 'Joe' WHERE `age` BETWEEN `min_age` AND `max_age`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tt.setup()
			query, _ := builder.RawSql()

			if query != tt.expectedQuery {
				t.Errorf("expected '%s' but got '%s'", tt.expectedQuery, query)
			}

		})
	}
}

func TestDeleteDebugApiRawSqlTest(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *api.DeleteQueryBuilder
		expectedQuery string
	}{
		{
			"Complex_Query",
			func() *api.DeleteQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewDeleteQueryBuilder(dbStrategy).
					Table("users").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("age", ">", 18).
					Delete()
			},
			"DELETE `users` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `age` > 18",
		},
		{
			"Delete_Where_Between",
			func() *api.DeleteQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewDeleteQueryBuilder(dbStrategy).
					Table("users").
					WhereNotBetween("age", 18, 30).
					Delete()
			},
			"DELETE FROM `users` WHERE `age` NOT BETWEEN 18 AND 30",
		},
		{
			"Delete_Where_Between_Columns",
			func() *api.DeleteQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewDeleteQueryBuilder(dbStrategy).
					Table("users").
					WhereBetweenColumns([]string{"created_at", "updated_at", "deleted_at"}, "created_at", "updated_at", "deleted_at").
					Delete()
			},
			"DELETE FROM `users` WHERE `created_at` BETWEEN `updated_at` AND `deleted_at`",
		},
		{
			"Delete_Where_Columns",
			func() *api.DeleteQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewDeleteQueryBuilder(dbStrategy).
					Table("users").
					WhereColumns([]string{"name", "age"},
						[][]string{
							{"name", "age"},
						}).
					Delete()
			},
			"DELETE FROM `users` WHERE `name` = `age`",
		},
		{
			"Delete_Where_Columns_With_WhereGroup",
			func() *api.DeleteQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewDeleteQueryBuilder(dbStrategy).
					Table("users").
					WhereGroup(func(w *api.WhereDeleteQueryBuilder) {
						w.Where("name", "=", "Joe").Where("age", "=", 31)
					}).
					Delete()
			},
			"DELETE FROM `users` WHERE (`name` = 'Joe' AND `age` = 31)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tt.setup()
			query, _ := builder.RawSql()

			if query != tt.expectedQuery {
				t.Errorf("expected '%s' but got '%s'", tt.expectedQuery, query)
			}

		})
	}
}

func TestDebugDumpTest(t *testing.T) {
	tests := []struct {
		name           string
		setup          func() *api.SelectQueryBuilder
		expectedQuery  string
		expectedValues []interface{}
	}{
		{
			"Complex_Query_With_Union",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				usq := api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("profiles.age", ">", 18)

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("area", "=", "Jakarta").
					WhereBetween("profiles.age", 18, 30).
					OrderBy("users.name", "ASC").
					OrderBy("profiles.age", "DESC").
					GroupBy("users.id").
					HavingRaw("COUNT(`profiles`.`id`) > 1").
					WhereIn("users.id", usq).
					Union(
						api.NewSelectQueryBuilder(dbStrategy).
							Table("users").
							Select("id", "users.name as name").
							Join("profiles", "users.id", "=", "profiles.user_id").
							Where("area", "=", "Jakarta").
							WhereBetween("profiles.age", 18, 30).
							OrderBy("users.name", "ASC").
							OrderBy("profiles.age", "DESC").
							GroupBy("users.id").
							HavingRaw("COUNT(`profiles`.`id`) > 1").
							WhereIn("users.id", usq),
					)

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = ? AND `profiles`.`age` BETWEEN ? AND ? AND `users`.`id` IN (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > ?) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC UNION SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = ? AND `profiles`.`age` BETWEEN ? AND ? AND `users`.`id` IN (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > ?) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC",
			[]interface{}{
				"Jakarta", 18, 30, 18, "Jakarta", 18, 30, 18,
			},
		},
		{
			"Complex_Query",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				usq := api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("profiles.age", ">", 18)

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name").
					Join("profiles", "users.id", "=", "profiles.user_id").
					Where("area", "=", "Jakarta").
					WhereBetween("profiles.age", 18, 30).
					OrderBy("users.name", "ASC").
					OrderBy("profiles.age", "DESC").
					GroupBy("users.id").
					HavingRaw("COUNT(`profiles`.`id`) > 1").
					WhereIn("users.id", usq)

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `area` = ? AND `profiles`.`age` BETWEEN ? AND ? AND `users`.`id` IN (SELECT `id` FROM `users` INNER JOIN `profiles` ON `users`.`id` = `profiles`.`user_id` WHERE `profiles`.`age` > ?) GROUP BY `users`.`id` HAVING COUNT(`profiles`.`id`) > 1 ORDER BY `users`.`name` ASC, `profiles`.`age` DESC",
			[]interface{}{
				"Jakarta", 18, 30, 18,
			},
		},
		{
			"Simple_Query",
			func() *api.SelectQueryBuilder {
				dbStrategy := mysql.NewMySQLQueryBuilder()

				return api.NewSelectQueryBuilder(dbStrategy).
					Table("users").
					Select("id", "users.name as name")

			},
			"SELECT `id`, `users`.`name` as `name` FROM `users`",
			[]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			builder := tt.setup()
			query, values, _ := builder.Dump()

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

			builder.Where("debug", "=", 1)

			query, values, _ = builder.Build()

			tt.expectedValues = append(tt.expectedValues, 1)

			if !strings.Contains(query, "`debug` = ?") {
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
