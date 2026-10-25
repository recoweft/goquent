package query

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/internal/observe"
)

// Exercise input changes in the adapter and after it returns, across all six
// families. Existing tests cover identity swaps, deadlines and concurrent CAS.
func TestSecurityAuditAuthorizationSixFamilyMutation(t *testing.T) {
	for _, pg := range []bool{false, true} {
		for _, family := range []string{"select", "count", "insert", "batch", "update", "delete"} {
			for _, phase := range []string{"unchanged", "provider", "before-dispatch"} {
				t.Run(fmt.Sprintf("%v/%s/%s", pg, family, phase), func(t *testing.T) {
					q, current, spy, mock := bindingFixture(t, pg)
					// A quoted reserved table name supplies the existing conservative
					// high-risk classification, including INSERT and COUNT.
					input := current.Settings.schema.Input()
					input.Tables[0].Table = "alter"
					schema, err := NewApplicationSchema(input)
					if err != nil {
						t.Fatal(err)
					}
					policies, err := NewPolicySet(TablePolicy{Table: "alter", TenantColumn: "tenant_id"})
					if err != nil {
						t.Fatal(err)
					}
					data := bindingRow()
					if family == "update" {
						data = map[string]any{"score": int32(3)}
					}
					batch := []map[string]any{bindingRow(), {"tenant_id": 1, "id": int64(4), "score": int32(5)}}
					columns := []string{"id"}
					mutate := func() {
						switch family {
						case "select", "delete":
							q.Where("score", int32(9))
						case "count":
							columns[0] = "score"
						case "insert", "update":
							data["score"] = int32(9)
						case "batch":
							batch[0], batch[1] = batch[1], batch[0]
						}
					}
					calls := 0
					auth, err := NewAuthorization(executionAuthorizer(func(_ context.Context, r AuthorizationRequest) (AuthorizationDecision, error) {
						calls++
						if r.Operation() != family {
							t.Fatal("wrong family")
						}
						if phase == "provider" {
							mutate()
						}
						return r.AllowUntil(r.ExpiresAt()), nil
					}), "audit-subject")
					if err != nil {
						t.Fatal(err)
					}
					current.Settings = NewSettings(policies, RiskConfig{}, current.Settings.execution).
						WithTenantPolicy("app-db", schema, false).WithAuthorization(auth)
					q = NewWithSettings(spy, "alter", testDialect(pg), current.Settings).
						Select("id").Where("tenant_id", 1).Where("id", int64(2)).RequireApproval("fixture intent")
					h, diagnostic, err := q.validateBinding(nil, current, bindingExpiry(), family, data, batch, columns)
					if err != nil || h == nil {
						t.Fatal("validation", err)
					}
					if calls != 0 || spy.calls != [6]int{} || !diagnostic.Blocked {
						t.Fatal("planning effects or missing pending authority")
					}
					statement := diagnostic.SQL
					// Explicit type-sensitive expectations at the Executor boundary;
					// sqlmock alone would coerce the integers to driver values.
					wantArgs := []any{1, int64(2)}
					switch family {
					case "insert":
						wantArgs = []any{int64(2), int32(3), 1}
					case "batch":
						wantArgs = []any{int64(2), int32(3), 1, int64(4), int32(5), 1}
					case "update":
						wantArgs = []any{int32(3), 1, int64(2)}
					}
					p, err := q.prepareBinding(nil, current, h, family, data, batch, columns)
					if phase == "provider" {
						if err != ErrAuthorizationMismatch {
							t.Fatal("provider mutation accepted", err)
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
						if phase == "before-dispatch" {
							mutate()
						} else if family == "select" || family == "count" {
							mock.ExpectQuery(regexp.QuoteMeta(statement)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2)).RowsWillBeClosed()
						} else {
							mock.ExpectExec(regexp.QuoteMeta(statement)).WillReturnResult(sqlmock.NewResult(2, 1))
						}
						switch family {
						case "select":
							err = q.executeRows(p, func(observe.Reader) error { return nil })
						case "count":
							err = q.executeRow(p, new(int64))
						default:
							_, err = q.executeResult(p)
						}
						if phase == "unchanged" {
							if err != nil || spy.sql != statement || !reflect.DeepEqual(spy.args, wantArgs) {
								t.Fatal("typed dispatch mismatch", err)
							}
						} else if err != ErrAuthorizationMismatch {
							t.Fatal("late mutation accepted", err)
						}
					}
					if phase != "unchanged" && spy.calls != [6]int{} {
						t.Fatal("rejected dispatch")
					}
					if calls != 1 {
						t.Fatal("provider count")
					}
					if _, err = q.prepareBinding(nil, current, h, family, data, batch, columns); err != ErrBindingConsumed {
						t.Fatal("reusable attempt", err)
					}
					if err = mock.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
