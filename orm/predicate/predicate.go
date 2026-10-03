// Package predicate describes generated condition structure, not authorization.
package predicate

import "errors"

// MaxDepth limits nested condition groups and subqueries. Flat lists do not
// consume nesting depth. Builders return ErrDepth rather than omit a condition.
const MaxDepth = 64

// ErrDepth indicates that a condition or subquery exceeded MaxDepth.
var ErrDepth = errors.New("predicate nesting exceeds 64 levels")

// Node is a detached inspection view of the structure used to render SQL.
// Parameter indexes in rendered views are zero based in the enclosing
// statement's Params; unrendered snapshots omit them. Opaque nodes preserve SQL but are not parsed.
// Exported/serialized fields are descriptive and never trusted proof.
type Node struct {
	NamedValues    map[string]Value `json:"named_values,omitempty"`
	BoundColumns   []string         `json:"bound_columns,omitempty"`
	Kind           string           `json:"kind"`
	Children       []*Node          `json:"children,omitempty"`
	Column         string           `json:"column,omitempty"`
	Operator       string           `json:"operator,omitempty"`
	ValueColumn    string           `json:"value_column,omitempty"`
	Raw            string           `json:"raw,omitempty"`
	Function       string           `json:"function,omitempty"`
	SQL            string           `json:"sql,omitempty"`
	Parameters     []int            `json:"parameters,omitempty"`
	Values         []Value          `json:"values,omitempty"`
	OpaqueReason   string           `json:"opaque_reason,omitempty"`
	Correspondence string           `json:"correspondence"`
}

// Value records isolation separately from SQL parameter position. Unverified
// payloads are deliberately omitted: serializing them could call user code.
type Value struct {
	Isolation string `json:"isolation"`
	Reason    string `json:"reason,omitempty"`
	Data      any    `json:"data,omitempty"`
}
