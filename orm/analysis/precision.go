// Package analysis describes bounded review coverage, never execution authority.
package analysis

import (
	"encoding/json"
	"errors"
	"github.com/recoweft/goquent/orm/internal/planversion"
)

const Version = 1

var ErrVersion = errors.New("goquent: unsupported analysis version")

type State string

const (
	Unknown     State = "unknown"
	Known       State = "known"
	Partial     State = "partial"
	Unsupported State = "unsupported"
)

// Detail separates static knowledge from sample-plan correspondence.
// Policy describes declared checks only; live authorization is always unverified.
type Detail struct {
	Version  int      `json:"version"`
	SQL      State    `json:"sql"`
	Columns  State    `json:"columns"`
	Bindings State    `json:"bindings"`
	Policy   State    `json:"policy"`
	Reasons  []string `json:"reasons"`
	Missing  []string `json:"missing"`
}

func NormalizeState(s State) State {
	switch s {
	case Known, Partial, Unsupported:
		return s
	}
	return Unknown
}
func reason(s string) string {
	switch s {
	case "static_unknown", "dynamic_value", "opaque_operation", "type_unavailable", "unverified_input", "runtime_policy", "database_specific", "evidence_missing", "evidence_mismatch", "source_changed", "duplicate_identity", "budget_exceeded", "canceled", "declaration_only", "planner_sample", "unsupported_version":
		return s
	}
	return "static_unknown"
}
func words(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for i, s := range in {
		if i >= 32 {
			break
		}
		s = reason(s)
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
		if len(out) == 16 {
			break
		}
	}
	return out
}
func (d Detail) Normalize() Detail {
	if d.Version != 0 && d.Version != Version {
		d = Detail{Reasons: []string{"unsupported_version"}}
	}
	d.Version = Version
	d.SQL = NormalizeState(d.SQL)
	d.Columns = NormalizeState(d.Columns)
	d.Bindings = NormalizeState(d.Bindings)
	d.Policy = NormalizeState(d.Policy)
	d.Reasons = words(d.Reasons)
	d.Missing = words(d.Missing)
	return d
}
func (d Detail) MarshalJSON() ([]byte, error) {
	if d.Version != 0 && d.Version != Version {
		return nil, ErrVersion
	}
	type wire Detail
	return json.Marshal(wire(d.Normalize()))
}
func (d *Detail) UnmarshalJSON(b []byte) error {
	if len(b) > 1<<20 || planversion.Read(b) != nil {
		return ErrVersion
	}
	type wire Detail
	var v wire
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if v.Version != 0 && v.Version != Version {
		return ErrVersion
	}
	*d = Detail(v).Normalize()
	return nil
}

// Precision is a conservative gate adapter. Missing axes cannot be precise.
func (d Detail) Precision() string {
	d = d.Normalize()
	all := []State{d.SQL, d.Columns, d.Bindings, d.Policy}
	complete := true
	for _, s := range all {
		if s == Unsupported {
			return "unsupported"
		}
		complete = complete && s == Known
	}
	if complete {
		return "precise"
	}
	return "partial"
}

type Match string

const (
	Unconfirmed Match = "unconfirmed"
	Matched     Match = "matched"
	Mismatched  Match = "mismatched"
)

func NormalizeMatch(m Match) Match {
	if m == Matched || m == Mismatched {
		return m
	}
	return Unconfirmed
}

// Correspondence reports independent comparisons, not a runtime safety verdict.
type Correspondence struct {
	Operation Match `json:"operation"`
	Target    Match `json:"target"`
	Structure Match `json:"structure"`
	Arguments Match `json:"arguments"`
	Source    Match `json:"source"`
	Context   Match `json:"context"`
}

func (c Correspondence) Normalize() Correspondence {
	c.Operation = NormalizeMatch(c.Operation)
	c.Target = NormalizeMatch(c.Target)
	c.Structure = NormalizeMatch(c.Structure)
	c.Arguments = NormalizeMatch(c.Arguments)
	c.Source = NormalizeMatch(c.Source)
	c.Context = NormalizeMatch(c.Context)
	return c
}

// Coverage distinguishes entirely unknown input from partial static knowledge.
func (d Detail) Coverage() State {
	d = d.Normalize()
	allUnknown, allKnown := true, true
	for _, s := range []State{d.SQL, d.Columns, d.Bindings, d.Policy} {
		if s == Unsupported {
			return Unsupported
		}
		allUnknown = allUnknown && s == Unknown
		allKnown = allKnown && s == Known
	}
	if allUnknown {
		return Unknown
	}
	if allKnown {
		return Known
	}
	return Partial
}
