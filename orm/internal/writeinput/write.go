// Package writeinput holds ORM-owned structural write inputs, never public plans.
package writeinput

import "github.com/recoweft/goquent/orm/internal/valuecopy"

type Conflict struct {
	Columns, Updates             []string
	Where, Constraint, RawTarget string
	ExplicitColumns              bool
	DoNothing                    bool
}

type Options struct {
	Columns     []string
	TableParts  []string
	Assignments []Assignment
	Conflict    *Conflict
	// Literal makes generic identifiers literal, preserving their existing contract.
	Literal bool
}

func (o Options) Clone() Options {
	o.TableParts = append([]string(nil), o.TableParts...)
	o.Columns = append([]string(nil), o.Columns...)
	o.Assignments = append([]Assignment(nil), o.Assignments...)
	for i := range o.Assignments {
		o.Assignments[i].Args = valuecopy.Slice(o.Assignments[i].Args)
	}
	if o.Conflict != nil {
		c := *o.Conflict
		c.Columns = append([]string(nil), c.Columns...)
		c.Updates = append([]string(nil), c.Updates...)
		o.Conflict = &c
	}
	return o
}
