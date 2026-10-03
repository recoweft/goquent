package valuecopy

import "github.com/recoweft/goquent/orm/predicate"

func Node(n *predicate.Node) *predicate.Node {
	if n == nil {
		return nil
	}
	out := *n
	out.BoundColumns = append([]string(nil), n.BoundColumns...)
	if n.NamedValues != nil {
		out.NamedValues = make(map[string]predicate.Value, len(n.NamedValues))
		for k, v := range n.NamedValues {
			v.Data, _ = Copy(v.Data)
			out.NamedValues[k] = v
		}
	}
	out.Children = make([]*predicate.Node, len(n.Children))
	for i, c := range n.Children {
		out.Children[i] = Node(c)
	}
	out.Parameters = append([]int(nil), n.Parameters...)
	out.Values = append([]predicate.Value(nil), n.Values...)
	for i := range out.Values {
		out.Values[i].Data, _ = Copy(out.Values[i].Data)
	}
	return &out
}

func Shift(n *predicate.Node, offset int) {
	if n == nil {
		return
	}
	for i := range n.Parameters {
		n.Parameters[i] += offset
	}
	for _, c := range n.Children {
		Shift(c, offset)
	}
}
