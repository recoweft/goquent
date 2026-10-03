package structs

import (
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/predicate"
)

// InspectPredicates is the unrendered snapshot view. Parameter positions and
// dialect transformations are intentionally unavailable until BuildSnapshot.
func InspectPredicates(groups []WhereGroup) (*predicate.Node, error) {
	tree, err := PredicateTree(groups)
	if err != nil {
		return nil, err
	}
	var walk func(*Predicate) *predicate.Node
	val := func(v any) predicate.Value {
		data, ok := valuecopy.Copy(v)
		if !ok {
			return predicate.Value{Isolation: "unverified", Reason: "unsupported_or_recursive_value"}
		}
		return predicate.Value{Isolation: "detached", Data: data}
	}
	walk = func(n *Predicate) *predicate.Node {
		if n == nil {
			return nil
		}
		c := n.Leaf
		out := &predicate.Node{Kind: n.Kind, Column: c.Column, Operator: c.Condition, ValueColumn: c.ValueColumn, Raw: c.Raw, Function: c.Function, Correspondence: "unverified"}
		for _, child := range n.Children {
			out.Children = append(out.Children, walk(child))
		}
		for _, v := range c.Value {
			out.Values = append(out.Values, val(v))
		}
		if c.ValueMap != nil {
			out.NamedValues = map[string]predicate.Value{}
			for k, v := range c.ValueMap {
				out.NamedValues[k] = val(v)
			}
		}
		if c.Between != nil {
			if c.Between.IsColumn {
				out.BoundColumns = []string{c.Between.From.(string), c.Between.To.(string)}
			} else {
				out.Values = []predicate.Value{val(c.Between.From), val(c.Between.To)}
			}
		}
		if c.JsonContains != nil {
			for _, v := range c.JsonContains.Values {
				out.Values = append(out.Values, val(v))
			}
		}
		if c.JsonLength != nil {
			out.Values = append(out.Values, val(c.JsonLength.Value))
		}
		if c.FullText != nil {
			out.Values = append(out.Values, val(c.FullText.Search))
		}
		if n.Kind == "opaque" {
			out.OpaqueReason = "unparsed_expression"
		}
		if c.Query != nil || c.Exists != nil {
			out.OpaqueReason = "subquery"
		}
		return out
	}
	return walk(tree), nil
}

// HavingGroups is shared by rendering and unrendered inspection. Preserve the
// existing omission of empty column/operator/string-value clauses.
func HavingGroups(group *GroupBy) []WhereGroup {
	if group == nil || len(group.Columns) == 0 || group.Having == nil {
		return nil
	}
	var conditions []Where
	for _, h := range *group.Having {
		if h.Raw != "" {
			conditions = append(conditions, Where{Raw: h.Raw, Operator: h.Operator})
			continue
		}
		empty, ok := h.Value.(string)
		if h.Column == "" || h.Condition == "" || (ok && empty == "") {
			continue
		}
		conditions = append(conditions, Where{Column: h.Column, Condition: h.Condition, Value: []any{h.Value}, Operator: h.Operator})
	}
	return []WhereGroup{{Conditions: conditions, IsDummyGroup: true}}
}
