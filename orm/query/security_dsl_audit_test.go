package query

import (
	"fmt"
	"strings"
	"testing"
)

func TestSecurityAuditStructuredDSLRefusal(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, kind := range []string{"operator", "direction", "column", "alias", "json-path", "assignment-path", "column-set"} {
			t.Run(fmt.Sprintf("%v/%s", pg, kind), func(t *testing.T) {
				q, _, spy, _ := bindingFixture(t, pg)
				data := map[string]any{"score": int32(3)}
				switch kind {
				case "operator":
					q.Where("score", "fixture-operator", 3)
				case "direction":
					q.OrderBy("id", "fixture-direction")
				case "column":
					q.Select("fixture\"`column")
				case "alias":
					q.Select("id as fixture_alias")
				case "json-path":
					q.Where("score->fixture'path", 3)
				case "assignment-path":
					data = map[string]any{"score->fixture": int32(3)}
				case "column-set":
					q.Select("id", "forbidden")
				}
				var err error
				if kind == "assignment-path" {
					_, err = q.Update(data)
				} else {
					var dest []struct{ ID int }
					err = q.Get(&dest)
				}
				if err == nil || spy.calls != [6]int{} {
					t.Fatal("unsupported structure dispatched")
				}
			})
		}
	}
}

func TestSecurityAuditCompatibilityIdentifierRendering(t *testing.T) {
	for _, pg := range []bool{false, true} {
		// Compatibility quoting is not an authorization or identifier allow-list.
		// These harmless punctuation fixtures are planned only, never sent to a DB.
		q, _, spy, _ := bindingFixture(t, pg)
		q = NewWithSettings(spy, "fixture\"`table", testDialect(pg), Settings{}).
			Select("id").Where("fixture\"`key", int32(7)).OrderBy("fixture\"`order", "desc").Limit(1)
		p, err := q.Plan(nil)
		if err != nil {
			t.Fatal(err)
		}
		quote := "`"
		if pg {
			quote = "\""
		}
		for _, part := range []string{"table", "key", "order"} {
			identifier := "fixture\"`" + part
			quoted := quote + strings.ReplaceAll(identifier, quote, quote+quote) + quote
			if !strings.Contains(p.SQL, quoted) {
				t.Fatal("identifier quote boundary changed")
			}
		}
		if len(p.Params) != 1 || p.Params[0] != int32(7) || spy.calls != [6]int{} {
			t.Fatal("plan changed typed value or dispatched")
		}
		for _, bad := range []*Query{
			NewWithSettings(spy, "users", testDialect(pg), Settings{}).Where("id", "fixture-operator", 1),
			NewWithSettings(spy, "users", testDialect(pg), Settings{}).OrderBy("id", "fixture-direction"),
		} {
			if _, err := bad.Plan(nil); err == nil {
				t.Fatal("invalid structural token accepted")
			}
		}
	}
}
