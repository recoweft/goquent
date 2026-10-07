// Package publicoutput provides detached display summaries and fixed public errors.
// Internal data APIs and runtime error identities remain separate contracts.
package publicoutput

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/recoweft/goquent/orm/internal/publicview"
)

// ErrOutput retains neither caller data nor an underlying error.
var ErrOutput = Error{}

// Error has no payload or cause. Its zero value is a safe public error.
type Error struct{}

func (Error) Error() string { return "PUBLIC_OUTPUT: operation failed; details omitted" }

// Write writes library-owned public bytes and discards sink error details.
func Write(w io.Writer, b []byte) error { return publicview.Write(w, b) }

// SummaryView is a bounded, non-executable resource/status display. Counts and
// booleans are supplied snapshot claims, never authorization or live evidence.
type SummaryView struct {
	Kind           string     `json:"kind"`
	Version        int        `json:"version"`
	Present        bool       `json:"present"`
	Known          bool       `json:"known"`
	Fresh          bool       `json:"fresh"`
	Exists         bool       `json:"exists"`
	Dirty          bool       `json:"dirty"`
	Unknown        bool       `json:"unknown"`
	Drifted        bool       `json:"drifted"`
	Count          int        `json:"count"`
	PendingCount   int        `json:"pending_count"`
	WarningCount   int        `json:"warning_count"`
	Items          []ItemView `json:"items"`
	Truncated      bool       `json:"truncated"`
	DetailsOmitted bool       `json:"details_omitted"`
}

// ItemView contains structural counts and fixed check vocabulary only.
type ItemView struct {
	Ordinal   int    `json:"ordinal"`
	Columns   int    `json:"columns"`
	Indexes   int    `json:"indexes"`
	Relations int    `json:"relations"`
	Policies  int    `json:"policies"`
	Examples  int    `json:"examples"`
	Check     string `json:"check"`
	Status    string `json:"status"`
}

func validKind(k string) bool {
	switch k {
	case "goquent.manifest_view", "goquent.schema_view", "goquent.models_view", "goquent.relations_view", "goquent.policies_view", "goquent.query_examples_view", "goquent.verification_view", "goquent.migration_status_view", "goquent.drift_view":
		return true
	}
	return false
}
func (v ItemView) safe() ItemView {
	v.Ordinal = publicview.Nonnegative(v.Ordinal)
	v.Columns = publicview.Nonnegative(v.Columns)
	v.Indexes = publicview.Nonnegative(v.Indexes)
	v.Relations = publicview.Nonnegative(v.Relations)
	v.Policies = publicview.Nonnegative(v.Policies)
	v.Examples = publicview.Nonnegative(v.Examples)
	switch v.Check {
	case "manifest", "schema", "policy", "generated_code", "database", "":
	default:
		v.Check = "unknown"
	}
	switch v.Status {
	case "ok", "stale", "skipped", "":
	default:
		v.Status = "unknown"
	}
	return v
}
func (v SummaryView) safe() (SummaryView, error) {
	if !validKind(v.Kind) || v.Version != 1 {
		return SummaryView{}, publicview.ErrInvalid
	}
	v.Count = max(publicview.Nonnegative(v.Count), len(v.Items))
	v.PendingCount = publicview.Nonnegative(v.PendingCount)
	v.WarningCount = publicview.Nonnegative(v.WarningCount)
	if !v.Known {
		v.Fresh = false
	}
	v.DetailsOmitted = true
	v.Truncated = v.Truncated || len(v.Items) > 128
	items := make([]ItemView, min(128, len(v.Items)))
	for i := range items {
		items[i] = v.Items[i].safe()
		items[i].Ordinal = i + 1
	}
	v.Items = items
	return v, nil
}
func (v SummaryView) MarshalJSON() ([]byte, error) {
	v, err := v.safe()
	if err != nil {
		return nil, err
	}
	type wire SummaryView
	return json.Marshal(wire(v))
}
func (v SummaryView) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v SummaryView) String() string {
	b, err := v.ToJSON()
	if err != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v SummaryView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func (v ItemView) MarshalJSON() ([]byte, error) {
	type wire ItemView
	return json.Marshal(wire(v.safe()))
}
func (v ItemView) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v ItemView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

// DecodeSummaryView accepts only a known display kind and strict integer v1.
func DecodeSummaryView(b []byte) (SummaryView, error) {
	var envelope struct {
		Kind string `json:"kind"`
	}
	if len(b) > publicview.MaxBytes || json.Unmarshal(b, &envelope) != nil || !validKind(envelope.Kind) {
		return SummaryView{}, publicview.ErrInvalid
	}
	type wire SummaryView
	var v wire
	if publicview.Decode(b, envelope.Kind, &v) != nil {
		return SummaryView{}, publicview.ErrInvalid
	}
	return SummaryView(v).safe()
}
func (v *SummaryView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = SummaryView{}
	out, err := DecodeSummaryView(b)
	if err == nil {
		*v = out
	}
	return err
}
func WriteSummary(w io.Writer, v SummaryView) error {
	b, err := v.ToJSON()
	if err != nil {
		return err
	}
	return Write(w, append(b, '\n'))
}
