package operation

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/internal/publicview"
	"github.com/recoweft/goquent/orm/query"
)

const DiagnosticViewKind = "goquent.operation_diagnostics"
const DiagnosticViewVersion = 1
const MaxDiagnostics = 128
const MaxDiagnosticElements = 1024

// DiagnosticView is detached display data, never execution or authorization input.
// Compiled includes blocked plans. Coverage describes only the declared subset.
type DiagnosticView struct {
	Kind            string            `json:"kind"`
	Version         int               `json:"version"`
	Outcome         string            `json:"outcome"`
	Coverage        string            `json:"coverage"`
	DiagnosticCount int               `json:"diagnostic_count"`
	Diagnostics     []DiagnosticEntry `json:"diagnostics"`
	Truncated       bool              `json:"truncated"`
	DetailsOmitted  bool              `json:"details_omitted"`
	Plan            *query.PlanView   `json:"plan,omitempty"`
}

// DiagnosticEntry contains only a fixed classification and logical location.
type DiagnosticEntry struct {
	Ordinal  int                `json:"ordinal"`
	Code     string             `json:"code"`
	Status   string             `json:"status"`
	Message  string             `json:"message"`
	Location DiagnosticLocation `json:"location"`
}

// DiagnosticLocation is a logical position, not a source path or byte offset.
// Unknown indexes are zero with their corresponding Known flag false.
type DiagnosticLocation struct {
	Source       string `json:"source"`
	Section      string `json:"section"`
	Member       string `json:"member"`
	IndexKnown   bool   `json:"index_known"`
	Index        int    `json:"index"`
	ElementKnown bool   `json:"element_known"`
	Element      int    `json:"element"`
	Origin       string `json:"origin"`
}

func vocabulary(s string, allowed ...string) string {
	for _, a := range allowed {
		if s == a {
			return a
		}
	}
	return "unknown"
}

func (l DiagnosticLocation) safe() DiagnosticLocation {
	l.Source = vocabulary(l.Source, "spec", "values", "manifest", "settings", "planner")
	l.Section = vocabulary(l.Section, "root", "version", "operation", "model", "select", "filters", "order_by", "limit", "implicit")
	l.Member = vocabulary(l.Member, "field", "op", "value", "value_ref", "direction")
	l.Origin = vocabulary(l.Origin, "json", "go", "implicit")
	if !l.IndexKnown || l.Index < 0 {
		l.IndexKnown = false
		l.Index = 0
	}
	if !l.ElementKnown || l.Element < 0 {
		l.ElementKnown = false
		l.Element = 0
	}
	return l
}
func diagnosticCode(code string) string {
	switch code {
	case "OPERATION_INPUT_INVALID":
		return code
	case "OPERATION_INPUT_LIMIT":
		return code
	case "OPERATION_VERSION_UNSUPPORTED":
		return code
	case "OPERATION_MANIFEST_REQUIRED":
		return code
	case "OPERATION_MANIFEST_INVALID":
		return code
	case "OPERATION_MANIFEST_STALE":
		return code
	case "OPERATION_UNSUPPORTED":
		return code
	case "OPERATION_MODEL_REQUIRED":
		return code
	case "OPERATION_MODEL_UNKNOWN":
		return code
	case "OPERATION_SELECT_REQUIRED":
		return code
	case "OPERATION_FIELD_UNKNOWN":
		return code
	case "OPERATION_FIELD_FORBIDDEN":
		return code
	case "OPERATION_FILTER_INVALID":
		return code
	case "OPERATION_OPERATOR_UNSUPPORTED":
		return code
	case "OPERATION_ARITY_INVALID":
		return code
	case "OPERATION_ORDER_INVALID":
		return code
	case "OPERATION_VALUE_REF_MISSING":
		return code
	case "OPERATION_REQUIRED_FILTER_MISSING":
		return code
	case "OPERATION_ACCESS_REASON_REQUIRED":
		return code
	case "OPERATION_TYPE_MISMATCH":
		return code
	case "OPERATION_TYPE_UNVERIFIED":
		return code
	case "OPERATION_ARRAY_UNSUPPORTED":
		return code
	case "OPERATION_RESERVED_BINDING":
		return code
	case "OPERATION_SETTINGS_INVALID":
		return code
	case "OPERATION_PLANNER_REFUSED":
		return code
	case "OPERATION_CHECK_TYPE":
		return code
	case "OPERATION_CHECK_NULLABILITY":
		return code
	case "OPERATION_CHECK_ENUM":
		return code
	case "OPERATION_CHECK_INPUT":
		return code
	case "OPERATION_CHECK_OPERATOR":
		return code
	case "OPERATION_CHECK_BINDING":
		return code
	case "OPERATION_UNVERIFIED_DRIVER":
		return code
	case "OPERATION_UNVERIFIED_BINDING":
		return code
	case "OPERATION_UNVERIFIED_LIVE":
		return code

	}
	return publicview.Code(code)
}
func (d DiagnosticEntry) safe() DiagnosticEntry {
	d.Ordinal = publicview.Nonnegative(d.Ordinal)
	d.Code = diagnosticCode(d.Code)
	d.Status = vocabulary(d.Status, "checked", "refused", "unverified", "warning")
	switch {
	case d.Code == "unknown":
		d.Status = "unknown"
	case strings.HasPrefix(d.Code, "OPERATION_CHECK_"):
		if d.Status != "checked" {
			d.Status = "unknown"
		}
	case strings.HasPrefix(d.Code, "OPERATION_UNVERIFIED_"):
		if d.Status != "unverified" {
			d.Status = "unknown"
		}
	case d.Code == "OPERATION_TYPE_UNVERIFIED":
		if d.Status != "unverified" && d.Status != "refused" && d.Status != "warning" {
			d.Status = "unknown"
		}
	case publicview.Code(d.Code) != "unknown":
		if d.Status != "warning" {
			d.Status = "unknown"
		}
	default:
		if d.Status != "refused" {
			d.Status = "unknown"
		}
	}
	d.Message = "Unknown diagnostic; details omitted."
	if d.Code != "unknown" {
		d.Message = d.Code + ": diagnostic details omitted."
	}
	d.Location = d.Location.safe()
	return d
}
func (v DiagnosticView) safe() (DiagnosticView, error) {
	if v.Kind != DiagnosticViewKind || v.Version != DiagnosticViewVersion {
		return DiagnosticView{}, publicview.ErrInvalid
	}
	v.Outcome = vocabulary(v.Outcome, "compiled", "rejected")
	v.Coverage = vocabulary(v.Coverage, "checked_subset", "partial", "unavailable")
	v.DetailsOmitted = true
	v.DiagnosticCount = max(v.DiagnosticCount, len(v.Diagnostics))
	entries := make([]DiagnosticEntry, min(len(v.Diagnostics), MaxDiagnostics))
	for i := range entries {
		entries[i] = v.Diagnostics[i].safe()
		entries[i].Ordinal = i + 1
	}
	v.Diagnostics = entries
	v.Truncated = v.Truncated || v.DiagnosticCount > len(entries)
	if v.Outcome != "compiled" {
		v.Plan = nil
	}
	if v.Plan != nil {
		// Roundtrip only the closed public plan, never the source plan or its payload.
		b, err := v.Plan.ToJSON()
		if err != nil {
			return DiagnosticView{}, publicview.ErrInvalid
		}
		p, err := query.DecodePlanView(b)
		if err != nil {
			return DiagnosticView{}, publicview.ErrInvalid
		}
		v.Plan = &p
	}
	// Root + entries + locations + optional plan + its warnings <= 514.
	// The separate 1024 element ceiling leaves no unbounded nested field.
	return v, nil
}
func (v DiagnosticView) MarshalJSON() ([]byte, error) {
	s, err := v.safe()
	if err != nil {
		return nil, err
	}
	type wire DiagnosticView
	b, err := json.Marshal(wire(s))
	if err != nil || len(b) > inputjson.MaxBytes {
		return nil, publicview.ErrInvalid
	}
	return b, nil
}
func (v DiagnosticView) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v DiagnosticView) String() string {
	b, e := v.ToJSON()
	if e != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v DiagnosticView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

// DecodeDiagnosticView rejects ambiguous, unknown and oversized wire input.
// It returns a fresh zero view on error; no private evidence can be restored.
func DecodeDiagnosticView(b []byte) (DiagnosticView, error) {
	type wire DiagnosticView
	var v wire
	if inputjson.CheckJSON(b) != nil || publicview.Decode(b, DiagnosticViewKind, &v) != nil {
		return DiagnosticView{}, publicview.ErrInvalid
	}
	// Count raw structural objects before nested PlanView normalization truncates.
	var raw struct {
		Diagnostics []json.RawMessage
		Plan        *struct {
			Warnings           []json.RawMessage
			SuppressedWarnings []json.RawMessage `json:"suppressed_warnings"`
		}
	}
	if json.Unmarshal(b, &raw) != nil {
		return DiagnosticView{}, publicview.ErrInvalid
	}
	elements := 1 + 2*len(raw.Diagnostics)
	if raw.Plan != nil {
		elements += 1 + len(raw.Plan.Warnings) + len(raw.Plan.SuppressedWarnings)
	}
	if elements > MaxDiagnosticElements {
		return DiagnosticView{}, publicview.ErrInvalid
	}
	return DiagnosticView(v).safe()
}
func (v *DiagnosticView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = DiagnosticView{}
	out, e := DecodeDiagnosticView(b)
	if e == nil {
		*v = out
	}
	return e
}
func (v DiagnosticEntry) MarshalJSON() ([]byte, error) {
	type wire DiagnosticEntry
	return json.Marshal(wire(v.safe()))
}
func (v DiagnosticEntry) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v DiagnosticEntry) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func (v DiagnosticLocation) MarshalJSON() ([]byte, error) {
	type wire DiagnosticLocation
	return json.Marshal(wire(v.safe()))
}
func (v DiagnosticLocation) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v DiagnosticLocation) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

func WriteDiagnosticJSON(w io.Writer, v DiagnosticView) error {
	b, e := v.ToJSON()
	if e != nil {
		return e
	}
	return publicview.Write(w, append(b, '\n'))
}
func WriteDiagnosticPretty(w io.Writer, v DiagnosticView) error {
	b, e := v.ToJSON()
	if e != nil {
		return e
	}
	return publicview.Write(w, append(append([]byte("Operation diagnostics (details omitted): "), b...), '\n'))
}
