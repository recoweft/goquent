package valuecopy

import "github.com/recoweft/goquent/orm/predicate"

func Node(n *predicate.Node) *predicate.Node { return New().Node(n) }

type nodeEntry struct {
	node   *predicate.Node
	active bool
}

func (c *Copier) Node(n *predicate.Node) *predicate.Node {
	if n == nil {
		return nil
	}
	if c.nodes == nil {
		c.nodes = make(map[*predicate.Node]*nodeEntry)
	}
	if e := c.nodes[n]; e != nil {
		if e.active {
			e.node.Correspondence = "unverified"
			e.node.OpaqueReason = "recursive_condition_view"
		}
		return e.node
	}
	out := *n
	e := &nodeEntry{node: &out, active: true}
	c.nodes[n] = e
	out.BoundColumns = append([]string(nil), n.BoundColumns...)
	if n.NamedValues != nil {
		out.NamedValues = make(map[string]predicate.Value, len(n.NamedValues))
		for k, v := range n.NamedValues {
			v = c.inspectionValue(v)
			out.NamedValues[k] = v
		}
	}
	out.Children = make([]*predicate.Node, len(n.Children))
	for i, child := range n.Children {
		out.Children[i] = c.Node(child)
	}
	out.Parameters = append([]int(nil), n.Parameters...)
	out.Values = append([]predicate.Value(nil), n.Values...)
	for i := range out.Values {
		out.Values[i] = c.inspectionValue(out.Values[i])
	}
	for _, v := range out.Values {
		if v.Isolation != "detached" {
			out.Correspondence = "unverified"
		}
	}
	for _, v := range out.NamedValues {
		if v.Isolation != "detached" {
			out.Correspondence = "unverified"
		}
	}
	for _, child := range out.Children {
		if child != nil && child.Correspondence != "generated" {
			out.Correspondence = "unverified"
		}
	}
	e.active = false
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

func (c *Copier) inspectionValue(v predicate.Value) predicate.Value {
	if v.Isolation != "detached" {
		return v
	}
	data, ok := c.Copy(v.Data)
	if !ok {
		return predicate.Value{Isolation: "unverified", Reason: UnverifiedReason}
	}
	v.Data = data
	return v
}
