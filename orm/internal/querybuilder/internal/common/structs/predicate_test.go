package structs

import (
	"errors"
	"github.com/recoweft/goquent/orm/predicate"
	"testing"
)

func TestPredicateSubqueryDepthAndCycles(t *testing.T) {
	for _, depth := range []int{63, 64, 65, 1000} {
		q := &Query{}
		for i := 0; i < depth; i++ {
			q = &Query{ConditionGroups: []WhereGroup{{IsDummyGroup: true, Conditions: []Where{{Query: q}}}}}
		}
		err := ValidateQuery(q)
		if depth > predicate.MaxDepth {
			if !errors.Is(err, predicate.ErrDepth) {
				t.Fatalf("depth %d: %v", depth, err)
			}
			if !errors.Is(CloneQuery(q).PredicateError, predicate.ErrDepth) {
				t.Fatal("clone discarded depth error")
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
	q := &Query{}
	q.Unions = []Union{{Query: q}}
	if !errors.Is(ValidateQuery(q), predicate.ErrDepth) {
		t.Fatal("cycle accepted")
	}
	groups := []WhereGroup{{IsDummyGroup: true, Conditions: make([]Where, 1)}}
	groups[0].Conditions[0].Nested = groups
	if _, err := PredicateTree(groups); !errors.Is(err, predicate.ErrDepth) {
		t.Fatal("condition cycle accepted")
	}
}
func TestPredicateFlatListDoesNotConsumeDepth(t *testing.T) {
	conditions := make([]Where, 10000)
	for i := range conditions {
		conditions[i] = Where{Column: "id", Condition: "=", Value: []any{i}, Operator: i % 2}
	}
	tree, err := PredicateTree([]WhereGroup{{IsDummyGroup: true, Conditions: conditions}})
	if err != nil || tree == nil {
		t.Fatalf("flat list: %v", err)
	}
}
