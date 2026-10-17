package publicoutput

import (
	"encoding/json"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/publicview"
)

// CapabilityAssessment separates general support from supplied target conditions.
// Neither axis is authorization, freshness, or an execution guarantee.
type CapabilityAssessment struct {
	Support    string `json:"support"`
	Conditions string `json:"conditions"`
}

// CapabilityView contains closed, non-executable operation classifications.
type CapabilityView struct {
	Operation           string               `json:"operation"`
	Source              string               `json:"source"`
	LockFree            CapabilityAssessment `json:"lock_free"`
	NoRebuild           CapabilityAssessment `json:"no_rebuild"`
	Transactional       CapabilityAssessment `json:"transactional"`
	Concurrent          CapabilityAssessment `json:"concurrent"`
	ConcurrentRequested bool                 `json:"concurrent_requested"`
}

func safeAssessment(a CapabilityAssessment) CapabilityAssessment {
	switch a.Support {
	case "supported", "conditional", "unsupported":
	default:
		a.Support = "unknown"
	}
	switch a.Conditions {
	case "satisfied", "not_satisfied":
	default:
		a.Conditions = "unknown"
	}
	return a
}

// PublicView maps all arbitrary strings to closed vocabulary. The returned
// display claims cannot establish whether any observation actually occurred.
func (v CapabilityView) PublicView() CapabilityView {
	v.Operation = publicview.MigrationStep(v.Operation)
	if v.Source != "live_observation" {
		v.Source = "unverified"
	}
	v.LockFree = safeAssessment(v.LockFree)
	v.NoRebuild = safeAssessment(v.NoRebuild)
	v.Transactional = safeAssessment(v.Transactional)
	v.Concurrent = safeAssessment(v.Concurrent)
	return v
}
func (v CapabilityView) MarshalJSON() ([]byte, error) {
	type wire CapabilityView
	return json.Marshal(wire(v.PublicView()))
}
func (v CapabilityView) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v CapabilityView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func (a CapabilityAssessment) MarshalJSON() ([]byte, error) {
	type wire CapabilityAssessment
	return json.Marshal(wire(safeAssessment(a)))
}
func (a CapabilityAssessment) String() string             { b, _ := a.MarshalJSON(); return string(b) }
func (a CapabilityAssessment) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(a.String())) }
