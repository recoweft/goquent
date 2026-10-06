package structs

import (
	"strings"

	"github.com/recoweft/goquent/orm/predicate"
)

// Predicate is the sole logical structure traversed by SQL rendering and its
// inspection trace. Leaves retain existing dialect-specific payloads.
type Predicate struct {
	Kind     string
	Children []*Predicate
	Leaf     Where
}

func PredicateTree(groups []WhereGroup) (*Predicate, error) {
	if err := validateGroups(groups, 0); err != nil {
		return nil, err
	}
	return predicateTree(groups, 0)
}
func predicateTree(groups []WhereGroup, depth int) (*Predicate, error) {
	if depth > predicate.MaxDepth {
		return nil, predicate.ErrDepth
	}
	var nodes []*Predicate
	var ops []int
	for _, g := range groups {
		var children []*Predicate
		var childOps []int
		for _, c := range g.Conditions {
			n := &Predicate{Kind: leafKind(c), Leaf: c}
			if c.Nested != nil {
				var err error
				n, err = predicateTree(c.Nested, depth)
				if err != nil {
					return nil, err
				}
			}
			if n != nil {
				children = append(children, n)
				childOps = append(childOps, c.Operator)
			}
		}
		if len(children) == 0 {
			continue
		}
		if g.IsDummyGroup {
			nodes = append(nodes, children...)
			ops = append(ops, childOps...)
			continue
		}
		// An explicit group consumes one level; nested lists are validated separately.
		n := &Predicate{Kind: "group", Children: []*Predicate{combine(children, childOps)}}
		if g.IsNot {
			n = &Predicate{Kind: "not", Children: []*Predicate{n}}
		}
		nodes = append(nodes, n)
		ops = append(ops, g.Operator)
	}
	return combine(nodes, ops), nil
}

// SQL AND binds more tightly than OR. Keep n-ary lists so a long flat query
// does not consume recursive stack proportional to its predicate count.
func combine(nodes []*Predicate, ops []int) *Predicate {
	if len(nodes) == 0 {
		return nil
	}
	var disjunction []*Predicate
	start := 0
	and := func(ns []*Predicate) *Predicate {
		if len(ns) == 1 {
			return ns[0]
		}
		return &Predicate{Kind: "and", Children: ns}
	}
	for i := 1; i < len(nodes); i++ {
		if ops[i] == 1 {
			disjunction = append(disjunction, and(nodes[start:i]))
			start = i
		}
	}
	disjunction = append(disjunction, and(nodes[start:]))
	if len(disjunction) == 1 {
		return disjunction[0]
	}
	return &Predicate{Kind: "or", Children: disjunction}
}

func leafKind(c Where) string {
	if c.LiteralColumn {
		for i, b := range []byte(c.Column) {
			if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_' || i > 0 && b >= '0' && b <= '9') {
				return "opaque"
			}
		}
	}

	switch {
	case c.Raw != "", c.Query != nil, c.Exists != nil, c.Function != "", c.FullText != nil, c.JsonContains != nil, c.JsonLength != nil:
		return "opaque"
	case c.Between != nil:
		return "between"
	case c.ValueColumn != "":
		return "column"
	}
	switch strings.ToUpper(strings.TrimSpace(c.Condition)) {
	case "IN", "NOT IN":
		return "in"
	case "IS NULL", "IS NOT NULL":
		return "null"
	case "=", "!=", "<>", "<", ">", "<=", ">=", "LIKE", "NOT LIKE":
		return "comparison"
	default:
		return "opaque"
	}
}

// ValidateQuery bounds nested groups/subqueries before cloning or rendering.
// Cycles are also rejected by this bounded traversal.
func ValidateQuery(q *Query) error { return validateQuery(q, 0) }
func validateQuery(q *Query, depth int) error {
	if q == nil {
		return nil
	}
	if depth > predicate.MaxDepth {
		return predicate.ErrDepth
	}
	for _, u := range q.Unions {
		if err := validateQuery(u.Query, depth+1); err != nil {
			return err
		}
	}
	if q.PredicateError != nil {
		return q.PredicateError
	}
	if err := validateGroups(q.ConditionGroups, depth); err != nil {
		return err
	}
	if q.Conditions != nil {
		if err := validateGroups([]WhereGroup{{Conditions: *q.Conditions, IsDummyGroup: true}}, depth); err != nil {
			return err
		}
	}
	if q.Joins != nil {
		for _, js := range []*[]Join{q.Joins.Joins, q.Joins.LateralJoins} {
			if js != nil {
				for _, j := range *js {
					if err := validateQuery(j.Query, depth+1); err != nil {
						return err
					}
				}
			}
		}
		if q.Joins.JoinClauses != nil {
			for _, j := range *q.Joins.JoinClauses {
				if err := validateQuery(j.Query, depth+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func validateGroups(groups []WhereGroup, depth int) error {
	for _, g := range groups {
		d := depth
		if !g.IsDummyGroup {
			d++
		}
		if d > predicate.MaxDepth {
			return predicate.ErrDepth
		}
		for _, c := range g.Conditions {
			nestedDepth := d
			if g.IsDummyGroup {
				nestedDepth++
			}
			if err := validateGroups(c.Nested, nestedDepth); err != nil {
				return err
			}
			if err := validateQuery(c.Query, d+1); err != nil {
				return err
			}
			if c.Exists != nil {
				if err := validateQuery(c.Exists.Query, d+1); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
