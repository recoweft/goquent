package migration

import (
	"encoding/json"
	"fmt"
	"github.com/recoweft/goquent/internal/inputjson"
	"io"
)

// PreflightView is bounded, non-reusable display data. Counts here count checks,
// never database rows. It omits timestamps, fingerprints, versions and names.
type PreflightView struct {
	Kind    string               `json:"kind"`
	Version int                  `json:"version"`
	Status  string               `json:"status"`
	Code    string               `json:"code"`
	Checks  []PreflightCheckView `json:"checks"`
}
type PreflightCheckView struct {
	Status string `json:"status"`
	Code   string `json:"code"`
}

func preflightStatus(s string) string {
	switch s {
	case "satisfied", "unsatisfied", "unknown", "unsupported":
		return s
	}
	return "unknown"
}
func preflightCode(s string) string {
	switch s {
	case "PREFLIGHT_OK", "PREFLIGHT_INPUT", "PREFLIGHT_FAILED", "PREFLIGHT_MISSING", "PREFLIGHT_ORIGIN", "PREFLIGHT_EXPIRED", "PREFLIGHT_TIME", "PREFLIGHT_BINDING", "PREFLIGHT_SCOPE", "PREFLIGHT_PHASE", "PREFLIGHT_COMPATIBILITY", "PREFLIGHT_DEPENDENCY", "PREFLIGHT_EXTERNAL_WORK", "PREFLIGHT_DEFINITION", "PREFLIGHT_UNSUPPORTED", "PREFLIGHT_NONZERO", "PREFLIGHT_VISIBILITY", "PREFLIGHT_SCHEMA_CHANGED":
		return s
	}
	return "PREFLIGHT_DEFINITION"
}
func (v PreflightView) MarshalJSON() ([]byte, error) {
	type plain PreflightView
	v.Kind = "goquent.preflight_view"
	v.Version = 1
	v.Status = preflightStatus(v.Status)
	v.Code = preflightCode(v.Code)
	v.Checks = append([]PreflightCheckView{}, v.Checks[:min(64, len(v.Checks))]...)
	for i := range v.Checks {
		v.Checks[i].Status = preflightStatus(v.Checks[i].Status)
		v.Checks[i].Code = preflightCode(v.Checks[i].Code)
	}
	return json.Marshal(plain(v))
}
func (v PreflightView) String() string             { b, _ := json.Marshal(v); return string(b) }
func (v PreflightView) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, v.String()) }

// EvaluatePreflight evaluates necessary conditions, never migration permission.
// Without the originating open collector this is offline consistency inspection.
func EvaluatePreflight(p *Preflight, opts PreflightOptions) (PreflightView, error) {
	return evaluatePreflight(p, opts, nil)
}
func evaluatePreflight(p *Preflight, opts PreflightOptions, owner *Collector) (PreflightView, error) {
	v := PreflightView{Kind: "goquent.preflight_view", Version: 1, Status: "unknown", Code: "PREFLIGHT_MISSING", Checks: []PreflightCheckView{}}
	if opts.Clock == nil || opts.MaxAge <= 0 || opts.MaxAge > PreflightMaxAge || ValidatePreflightSource(p) != nil {
		v.Code = "PREFLIGHT_INPUT"
		return v, ErrPreflightInput
	}
	for _, r := range p.Plan.Requirements {
		v.Checks = append(v.Checks, PreflightCheckView{preflightStatus(r.Status), preflightCode(r.Code)})
	}
	c := p.Collection
	if c == nil {
		return v, nil
	}
	if !c.Completed || c.Code != "PREFLIGHT_OK" {
		v.Code = preflightCode(c.Code)
		if v.Code == "PREFLIGHT_OK" {
			v.Code = "PREFLIGHT_FAILED"
		}
		return v, ErrPreflightRead
	}
	now := opts.Clock().UTC()
	if now.IsZero() || now.Year() < 1 || now.Year() > 9999 || c.StartedAt.IsZero() || c.FinishedAt.IsZero() || c.FinishedAt.Before(c.StartedAt) || c.FinishedAt.After(now) {
		v.Code = "PREFLIGHT_TIME"
		return v, nil
	}
	if now.Sub(c.StartedAt) > opts.MaxAge {
		v.Code = "PREFLIGHT_EXPIRED"
		return v, nil
	}
	if owner == nil || owner.closed || p.receipt == nil || p.receipt.owner != owner || p.receipt.digest != c.Digest || p.receipt.plan != p.Plan.ID {
		v.Code = "PREFLIGHT_ORIGIN"
		return v, nil
	}
	if len(c.Results) != len(p.Plan.Requirements) {
		v.Code = "PREFLIGHT_MISSING"
		return v, nil
	}
	v.Status = "satisfied"
	v.Code = "PREFLIGHT_OK"
	for i, r := range c.Results {
		req := p.Plan.Requirements[i]
		check := PreflightCheckView{preflightStatus(req.Status), preflightCode(req.Code)}
		if req.Status == "pending" {
			check = PreflightCheckView{preflightStatus(r.Status), preflightCode(r.Code)}
			if r.Completed && r.Count != nil && r.Status == "satisfied" && r.Code == "PREFLIGHT_OK" {
				if *r.Count > 0 {
					check = PreflightCheckView{"unsatisfied", "PREFLIGHT_NONZERO"}
				}
			} else if check.Status == "satisfied" {
				check = PreflightCheckView{"unknown", "PREFLIGHT_MISSING"}
			}
		}
		v.Checks[i] = check
		// Unknown > unsupported > unsatisfied > satisfied; collection errors precede all.
		if verdictRank(check.Status) > verdictRank(v.Status) {
			v.Status = check.Status
			v.Code = check.Code
		}
	}
	// A prerequisite with unobserved application work cannot disappear just because
	// a later data count is zero. Requirements for all steps remain in the aggregate.
	return v, nil
}
func verdictRank(s string) int {
	switch s {
	case "satisfied":
		return 0
	case "unsatisfied":
		return 1
	case "unsupported":
		return 2
	}
	return 3
}

func (v PreflightCheckView) MarshalJSON() ([]byte, error) {
	type plain PreflightCheckView
	return json.Marshal(plain{Status: preflightStatus(v.Status), Code: preflightCode(v.Code)})
}
func (v PreflightCheckView) String() string             { b, _ := json.Marshal(v); return string(b) }
func (v PreflightCheckView) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, v.String()) }
func (v *PreflightCheckView) UnmarshalJSON(b []byte) error {
	if _, err := inputjson.Object(b, map[string]bool{"status": true, "code": true}); err != nil {
		return ErrPreflightInput
	}
	type plain PreflightCheckView
	var n plain
	if sourceObject(b, &n) != nil || preflightStatus(n.Status) != n.Status || preflightCode(n.Code) != n.Code {
		return ErrPreflightInput
	}
	*v = PreflightCheckView(n)
	return nil
}
func (v *PreflightView) UnmarshalJSON(b []byte) error {
	if _, err := inputjson.Object(b, map[string]bool{"kind": true, "version": true, "status": true, "code": true, "checks": true}); err != nil {
		return ErrPreflightInput
	}
	type plain PreflightView
	var n plain
	if sourceObject(b, &n) != nil || n.Kind != "goquent.preflight_view" || n.Version != 1 || len(n.Checks) > 64 || preflightStatus(n.Status) != n.Status || preflightCode(n.Code) != n.Code {
		return ErrPreflightInput
	}
	*v = PreflightView(n)
	return nil
}
