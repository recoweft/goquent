package db_test

import (
	"testing"

	"github.com/recoweft/goquent/orm/internal/querybuilder/database/postgres"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

func TestPostgreSQLQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		input    structs.Query
		expected QueryBuilderExpected
	}{
		{
			"FromSchemaQualifiedAliasUppercaseAS",
			"From",
			structs.Query{
				Table: structs.Table{Name: "app.feed_entries AS feed_entries"},
			},
			QueryBuilderExpected{
				Expected: `FROM "app"."feed_entries" as "feed_entries"`,
				Values:   nil,
			},
		},
		{
			"SelectAliasedThreePartReference",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{Name: "app.feed_entries.id AS feed_entry_id"},
				},
			},
			QueryBuilderExpected{
				Expected: `SELECT "app"."feed_entries"."id" as "feed_entry_id"`,
				Values:   nil,
			},
		},
		{
			"SelectRaw_With_Case_When_Placeholder",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{
						Raw:    `CASE WHEN viewer_sub.id IS NOT NULL AND viewer_sub.status::text <> ? THEN TRUE ELSE FALSE END AS muted_source`,
						Values: []interface{}{"active"},
					},
				},
			},
			QueryBuilderExpected{
				Expected: `SELECT CASE WHEN viewer_sub.id IS NOT NULL AND viewer_sub.status::text <> $1 THEN TRUE ELSE FALSE END AS muted_source`,
				Values:   []interface{}{"active"},
			},
		},
		{
			"SelectRaw_With_Subquery_And_Multiple_Placeholders",
			"Select",
			structs.Query{
				Columns: &[]structs.Column{
					{
						Name: "app.feed_entries.id",
					},
					{
						Raw:    `(SELECT recommendations.reason_text FROM app.recommendations AS recommendations WHERE recommendations.user_id = ? AND recommendations.content_item_id = feed_entries.content_item_id AND recommendations.slot = ? ORDER BY recommendations.score DESC, recommendations.generated_at DESC LIMIT 1) AS recommendation_reason`,
						Values: []interface{}{42, "home"},
					},
				},
			},
			QueryBuilderExpected{
				Expected: `SELECT "app"."feed_entries"."id", (SELECT recommendations.reason_text FROM app.recommendations AS recommendations WHERE recommendations.user_id = $1 AND recommendations.content_item_id = feed_entries.content_item_id AND recommendations.slot = $2 ORDER BY recommendations.score DESC, recommendations.generated_at DESC LIMIT 1) AS recommendation_reason`,
				Values:   []interface{}{42, "home"},
			},
		},
		{
			"JoinSchemaQualifiedAliasUppercaseAS",
			"Join",
			structs.Query{
				Joins: &structs.Joins{
					Joins: &[]structs.Join{
						{
							TargetNameMap:      map[string]string{consts.Join_INNER: "app.feed_entries AS feed_entries"},
							SearchColumn:       "users.feed_entry_id",
							SearchCondition:    "=",
							SearchTargetColumn: "feed_entries.id",
						},
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` INNER JOIN "app"."feed_entries" as "feed_entries" ON "users"."feed_entry_id" = "feed_entries"."id"`,
				Values:   nil,
			},
		},
		{
			"OrderByThreePartReference",
			"OrderBy",
			structs.Query{
				Order: &[]structs.Order{
					{
						Column: "app.feed_entries.created_at",
						IsAsc:  false,
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` ORDER BY "app"."feed_entries"."created_at" DESC`,
				Values:   nil,
			},
		},
		{
			"WhereFullText",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								FullText: &structs.FullText{
									Columns: []string{"name", "description"},
									Search:  "search",
									Options: map[string]interface{}{"mode": "websearch"},
								},
								Operator: consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` WHERE (to_tsvector($1, "name") || to_tsvector($2, "description")) @@ websearch_to_tsquery($3, $4)`,
				Values:   []interface{}{"english", "english", "english", "search"},
			},
		},
		{
			"WhereJsonContains",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:       "options->languages",
								JsonContains: &structs.JsonContains{Values: []interface{}{"en"}},
								Operator:     consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` WHERE ("options"->'languages')::jsonb @> $1`,
				Values:   []interface{}{"\"en\""},
			},
		},
		{
			"WhereJsonLength",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Column:     "options->languages",
								JsonLength: &structs.JsonLength{Operator: ">", Value: 1},
								Operator:   consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` WHERE jsonb_array_length(("options"->'languages')::jsonb) > $1`,
				Values:   []interface{}{1},
			},
		},
		{
			"WhereRaw_NamedPlaceholders_AvoidPrefixCollision",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Raw: `coalesce(user_content_states.inbox_state::text, :archived_default) <> :archived`,
								ValueMap: map[string]any{
									"archived_default": "active",
									"archived":         "archived",
								},
								Operator: consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` WHERE coalesce(user_content_states.inbox_state::text, $1) <> $2`,
				Values:   []interface{}{"active", "archived"},
			},
		},
		{
			"WhereRaw_NamedPlaceholders_SharedPrefixes",
			"Where",
			structs.Query{
				ConditionGroups: []structs.WhereGroup{
					{
						Conditions: []structs.Where{
							{
								Raw: `coalesce(feed_entries.state::text, :state_default) <> :state AND coalesce(feed_entries.previous_state::text, :state_default_extra) <> :state_default`,
								ValueMap: map[string]any{
									"state_default":       "active",
									"state":               "archived",
									"state_default_extra": "muted",
								},
								Operator: consts.LogicalOperator_AND,
							},
						},
						IsDummyGroup: true,
						Operator:     consts.LogicalOperator_AND,
					},
				},
			},
			QueryBuilderExpected{
				Expected: ` WHERE coalesce(feed_entries.state::text, $1) <> $2 AND coalesce(feed_entries.previous_state::text, $3) <> $4`,
				Values:   []interface{}{"active", "archived", "muted", "active"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			builder := postgres.NewPostgreSQLQueryBuilder()
			sb := make([]byte, 0, consts.StringBuffer_Middle_Query_Grow)

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
				t.Errorf("expected '%s' but got '%s'", tt.expected, got)
			}

			if len(gotValues) != len(tt.expected.Values) {
				t.Errorf("expected '%v' but got '%v'", tt.expected.Values, gotValues)
			}
			for i := range gotValues {
				if gotValues[i] != tt.expected.Values[i] {
					t.Errorf("expected value %v at index %d but got %v", tt.expected.Values[i], i, gotValues[i])
				}
			}

		})
	}
}

func TestPostgreSQLQueryBuilder_InvalidTableReferenceFallsBackWithoutPanic(t *testing.T) {
	t.Parallel()

	builder := postgres.NewPostgreSQLQueryBuilder()
	sb := make([]byte, 0, consts.StringBuffer_Middle_Query_Grow)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	builder.From(&sb, "app.feed.entries AS feed_entries")

	got := string(sb)
	want := `FROM "app.feed.entries AS feed_entries"`
	if got != want {
		t.Fatalf("expected %q but got %q", want, got)
	}
}
