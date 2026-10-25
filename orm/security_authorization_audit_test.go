package orm

import (
	"reflect"
	"strings"
	"testing"
)

// A supplied model is not an allow-list. Only effective assignments are checked;
// excluding a caller's tenant field does not make that field tenant authority.
func TestSecurityAuditMassAssignmentSelection(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		for _, selection := range []string{"all", "columns", "omit", "tenant"} {
			t.Run(dialect+"/"+selection, func(t *testing.T) {
				spy := &genericSpy{}
				db := NewDBWithExecutor(spy, genericDialect(dialect), WithSettings(genericSettings(t, dialect, true)))
				data := map[string]any{"id": int64(3), "score": int32(7), "tenant_id": int64(2), "fixed": int32(9)}
				opts := []WriteOpt{Table("users"), PK("id"), WherePK()}
				switch selection {
				case "columns":
					opts = append(opts, Columns("score"))
				case "omit":
					opts = append(opts, Omit("tenant_id", "fixed"))
				case "tenant":
					opts = append(opts, Columns("tenant_id"))
				}
				_, err := Update(nil, db, data, opts...)
				if selection == "all" || selection == "tenant" {
					if err == nil || len(spy.calls) != 0 {
						t.Fatal("protected assignment dispatched")
					}
					return
				}
				if err != nil || !reflect.DeepEqual(spy.calls, []string{"exec"}) {
					t.Fatal("allowed assignment refused", err)
				}
				if !reflect.DeepEqual(spy.args, []any{int32(7), int64(3), 1}) {
					t.Fatal("caller tenant replaced application tenant or typed arguments changed")
				}
				quote := "`"
				if dialect == "postgres" {
					quote = "\""
				}
				if !strings.Contains(spy.sql, "SET "+quote+"score"+quote) || strings.Contains(spy.sql, quote+"fixed"+quote) {
					t.Fatal("assignment selection changed")
				}
				if data["tenant_id"] != int64(2) {
					t.Fatal("caller payload mutated")
				}
			})
		}
	}
}
