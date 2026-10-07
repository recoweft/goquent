package query

import (
	"errors"
	"strings"
	"testing"
)

func TestLimitExactSQLSnapshotAndSetterOrder(t *testing.T) {
	for _, pg := range []bool{false, true} {
		q := New(nil, "users", testDialect(pg)).Select("id")
		for _, tc := range []struct {
			exact bool
			n     int
		}{{false, 0}, {true, 0}, {false, 5}, {true, 2}, {false, 0}, {true, 0}} {
			if tc.exact {
				q.LimitExact(tc.n)
			} else {
				q.Take(tc.n)
			}
			p, e := q.Plan(t.Context())
			if e != nil {
				t.Fatal(e)
			}
			want := tc.exact || tc.n > 0
			if (p.Limit != nil) != want || strings.Contains(p.SQL, " LIMIT ") != want {
				t.Fatal("SQL/plan disagree")
			}
			if want && *p.Limit != int64(tc.n) {
				t.Fatal("limit changed")
			}
			if (p.execution.inspection.Limit != nil) != want {
				t.Fatal("seal lost limit")
			}
			for _, w := range p.Warnings {
				if tc.exact && w.Code == WarningLimitMissing {
					t.Fatal("explicit zero became missing")
				}
			}
			b := newSelectBuilder(testDialect(pg))
			q.builder.CopyStateToSelect(b)
			copy, err := q.planSelectBuilder(t.Context(), b)
			if err != nil || copy.SQL != p.SQL {
				t.Fatal("copy lost state")
			}
		}
		q.LimitExact(-1).Limit(5).LimitExact(0)
		if _, e := q.Plan(t.Context()); !errors.Is(e, ErrInvalidLimit) {
			t.Fatal("error not sticky", e)
		}
		if _, _, e := q.Build(); !errors.Is(e, ErrInvalidLimit) {
			t.Fatal(e)
		}
	}
}
func TestLimitExactWriteRefusalAndBindingMismatch(t *testing.T) {
	for _, pg := range []bool{false, true} {
		q, c, spy, _ := bindingFixture(t, pg)
		q.LimitExact(0)
		for _, kind := range []string{"insert", "update", "delete"} {
			var e error
			switch kind {
			case "insert":
				_, e = q.PlanInsert(t.Context(), bindingRow())
			case "update":
				_, e = q.PlanUpdate(t.Context(), map[string]any{"score": 3})
			case "delete":
				_, e = q.PlanDelete(t.Context())
			}
			if !errors.Is(e, ErrBlockedOperation) {
				t.Fatal(kind, e)
			}
		}
		for _, change := range []func(*Query){func(q *Query) { q.LimitExact(1) }, func(q *Query) { q.Limit(0) }} {
			q.LimitExact(0)
			h, p, e := q.ValidateSelect(t.Context(), c, bindingExpiry())
			if e != nil {
				t.Fatal("fixture cannot bind", e)
			}
			*p.Limit = 99
			change(q)
			var dest []struct{ ID int }
			e = q.ExecuteValidatedSelect(t.Context(), c, h, &dest)
			if !errors.Is(e, ErrBindingMismatch) {
				t.Fatal("limit mutation not detected", e)
			}
		}
		if spy.calls != [6]int{} {
			t.Fatal("rejection dispatched")
		}
	}
}
