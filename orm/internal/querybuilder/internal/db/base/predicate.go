package base

import (
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/predicate"
)

type LeafRenderer func(*[]byte, structs.Where) ([]any, error)

// RenderPredicates generates SQL, ordered parameters and inspection together.
// No SQL parser or second value evaluation is involved.
func RenderPredicates(sb *[]byte, groups []structs.WhereGroup, offset int, render LeafRenderer) ([]any, *predicate.Node, error) {
	tree, err := structs.PredicateTree(groups)
	if err != nil {
		return nil, nil, err
	}
	if tree == nil {
		return nil, nil, nil
	}
	*sb = append(*sb, " WHERE "...)
	values := []any{}
	copier := valuecopy.New()
	var walk func(*structs.Predicate) (*predicate.Node, error)
	walk = func(n *structs.Predicate) (*predicate.Node, error) {
		start := len(*sb)
		out := &predicate.Node{Kind: n.Kind, Correspondence: "generated"}
		switch n.Kind {
		case "and", "or", "group", "not":
			if n.Kind == "group" {
				*sb = append(*sb, '(')
			}
			if n.Kind == "not" {
				*sb = append(*sb, "NOT "...)
			}
			for i, c := range n.Children {
				if i > 0 {
					*sb = append(*sb, " "+strings.ToUpper(n.Kind)+" "...)
				}
				child, err := walk(c)
				if err != nil {
					return nil, err
				}
				out.Children = append(out.Children, child)
				if child.Correspondence != "generated" {
					out.Correspondence = "unverified"
				}
			}
			if n.Kind == "group" {
				*sb = append(*sb, ')')
			}
		default:
			c := n.Leaf
			out.Column = c.Column
			out.Operator = c.Condition
			out.ValueColumn = c.ValueColumn
			out.Raw = c.Raw
			out.Function = c.Function
			if c.Between != nil && c.Between.IsColumn {
				out.BoundColumns = []string{c.Between.From.(string), c.Between.To.(string)}
			}
			if n.Kind == "comparison" && len(c.Value) != 1 {
				out.OpaqueReason = "unsupported_comparison_arity"
			}
			if n.Kind == "opaque" {
				out.OpaqueReason = "unparsed_expression"
			}
			if c.Query != nil || c.Exists != nil {
				out.OpaqueReason = "subquery"
			}
			if n.Kind == "in" && len(c.Value) == 0 {
				out.OpaqueReason = "empty_in_legacy_sql"
			}
			if out.OpaqueReason != "" {
				out.Correspondence = "unverified"
			}
			args, err := render(sb, c)
			if err != nil {
				return nil, err
			}
			for _, arg := range args {
				out.Parameters = append(out.Parameters, offset+len(values))
				copied, ok := copier.Copy(arg)
				v := predicate.Value{Isolation: "detached", Data: copied}
				if !ok {
					out.Correspondence = "unverified"
					v = predicate.Value{Isolation: "unverified", Reason: valuecopy.UnverifiedReason}
				}
				out.Values = append(out.Values, v)
				values = append(values, copied)
			}
		}
		out.SQL = string((*sb)[start:])
		return out, nil
	}
	view, err := walk(tree)
	return values, view, err
}

func (wb *WhereBaseBuilder) RenderLeaf(sb *[]byte, c structs.Where) ([]any, error) {
	switch {
	case c.Query != nil:
		return wb.ProcessSubQuery(sb, c)
	case c.Exists != nil:
		return wb.ProcessExistsQuery(sb, c)
	case c.Between != nil:
		return wb.ProcessBetweenCondition(sb, c), nil
	case c.FullText != nil:
		return wb.ProcessFullText(sb, c)
	case c.Function != "":
		return wb.ProcessFunction(sb, c), nil
	default:
		return wb.ProcessRawCondition(sb, c)
	}
}
