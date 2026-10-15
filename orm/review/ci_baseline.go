package review

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/internal/publicview"
	"github.com/recoweft/goquent/orm/query"
)

const BaselineKind = "goquent.review_baseline"
const MaxBaselineBytes = 256 << 10
const MaxBaselineEntries = 64
const MaxBaselineAge = 30 * 24 * time.Hour

// Baseline is sensitive source configuration, never evidence or authorization.
// Only DecodeBaseline and RunCI consume it; no serialized report is accepted.
type Baseline struct {
	Kind    string          `json:"kind"`
	Version int             `json:"version"`
	Entries []BaselineEntry `json:"entries"`
}
type BaselineEntry struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	Operation string `json:"operation"`
	Code      string `json:"code"`
	Risk      string `json:"risk"`
	Precision string `json:"precision"`
	SQL       string `json:"sql"`
	Columns   string `json:"columns"`
	Bindings  string `json:"bindings"`
	Policy    string `json:"policy"`
	Owner     string `json:"owner"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

// DecodeBaseline rejects unknown/duplicate fields and unsupported envelopes.
// Entry semantic failures are retained by RunCI as invalid, not silently removed.
func DecodeBaseline(b []byte) (Baseline, error) {
	var v Baseline
	if len(b) > MaxBaselineBytes || publicview.Decode(b, BaselineKind, &v) != nil || v.Entries == nil || len(v.Entries) > MaxBaselineEntries {
		return Baseline{}, errCIInput
	}
	var raw struct {
		Entries []map[string]json.RawMessage `json:"entries"`
	}
	if json.Unmarshal(b, &raw) != nil {
		return Baseline{}, errCIInput
	}
	required := []string{"file", "line", "column", "operation", "code", "risk", "precision", "sql", "columns", "bindings", "policy", "owner", "reason", "created_at", "expires_at"}
	for _, entry := range raw.Entries {
		for _, key := range required {
			if _, ok := entry[key]; !ok {
				return Baseline{}, errCIInput
			}
		}
	}
	return v, nil
}
func baselineCode(c string) bool {
	switch c {
	case query.WarningSelectStarUsed, query.WarningLimitMissing, query.WarningWeakPredicate:
		return findingSuppressible(c)
	}
	return false
}
func baselineState(s string) bool {
	return s == "known" || s == "partial" || s == "unknown" || s == "unsupported"
}
func (e BaselineEntry) state(now time.Time) string {
	if !exactPath(e.File) || e.Line < 1 || e.Column < 0 || e.Operation != "select" || !baselineCode(e.Code) || strings.TrimSpace(e.Owner) == "" || len(e.Owner) > 256 || configSuppressionReasonWeak(e.Reason) || len(e.Reason) > 1024 {
		return "invalid"
	}
	if publicview.Risk(e.Risk) != e.Risk || e.Risk == "unknown" || publicview.Precision(e.Precision) != e.Precision || e.Precision == "unknown" {
		return "invalid"
	}
	for _, s := range []string{e.SQL, e.Columns, e.Bindings, e.Policy} {
		if !baselineState(s) {
			return "invalid"
		}
	}
	created, err := time.Parse(time.RFC3339, e.CreatedAt)
	expiry, err2 := time.Parse(time.RFC3339, e.ExpiresAt)
	if err != nil || err2 != nil || !strings.HasSuffix(e.CreatedAt, "Z") || !strings.HasSuffix(e.ExpiresAt, "Z") || !expiry.After(created) || expiry.Sub(created) > MaxBaselineAge || created.After(now) {
		return "invalid"
	}
	if !expiry.After(now) {
		return "expired"
	}
	return "valid"
}
func (e BaselineEntry) key() string {
	return e.File + "\x00" + itoa(e.Line) + "\x00" + itoa(e.Column) + "\x00" + e.Code
}
func (e BaselineEntry) matches(f ciFinding) bool {
	return f.file == e.File && f.f.Location != nil && f.f.Location.Line == e.Line && f.f.Location.Column == e.Column && f.f.Code == e.Code && f.operation == e.Operation && string(f.f.Level) == e.Risk && string(f.f.AnalysisPrecision) == e.Precision && string(f.f.Detail.SQL) == e.SQL && string(f.f.Detail.Columns) == e.Columns && string(f.f.Detail.Bindings) == e.Bindings && string(f.f.Detail.Policy) == e.Policy
}

// EntryCounts counts baseline entries. Invalid=1 with zero Entries denotes an
// invalid document. FindingCounts independently classifies head diagnostics.
type EntryCounts struct {
	Entries int `json:"entries"`
	Matched int `json:"matched"`
	Expired int `json:"expired"`
	Invalid int `json:"invalid"`
	Unused  int `json:"unused"`
	Unknown int `json:"unknown"`
}
type FindingCounts struct {
	Matched   int `json:"matched"`
	New       int `json:"new"`
	Worsened  int `json:"worsened"`
	Unknown   int `json:"unknown"`
	Unchanged int `json:"unchanged"`
}

func applyBaseline(b []byte, now time.Time, old, head []ciFinding, contextEqual bool) (EntryCounts, []bool) {
	counts := EntryCounts{}
	matched := make([]bool, len(head))
	if b == nil {
		return counts, matched
	}
	v, err := DecodeBaseline(b)
	if err != nil {
		counts.Invalid = 1
		return counts, matched
	}
	counts.Entries = len(v.Entries)
	keys := map[string]int{}
	for _, e := range v.Entries {
		keys[e.key()]++
	}
	for _, e := range v.Entries {
		state := e.state(now)
		if keys[e.key()] > 1 {
			state = "invalid"
		}
		switch state {
		case "invalid":
			counts.Invalid++
			continue
		case "expired":
			counts.Expired++
			continue
		}
		oi, hi := -1, -1
		oc, hc := 0, 0
		for i, f := range old {
			if e.matches(f) {
				oc++
				oi = i
			}
		}
		for i, f := range head {
			if e.matches(f) {
				hc++
				hi = i
			}
		}
		if !contextEqual || oc > 1 || hc > 1 {
			counts.Unknown++
			continue
		}
		if oc == 0 || hc == 0 {
			counts.Unused++
			continue
		}
		if !contextEqual || !old[oi].known || !head[hi].known || !sameDiagnostic(old[oi], head[hi]) {
			counts.Unknown++
			continue
		}
		matched[hi] = true
		counts.Matched++
	}
	return counts, matched
}
func equalDetail(a, b analysis.Detail) bool {
	a = a.Normalize()
	b = b.Normalize()
	return a.SQL == b.SQL && a.Columns == b.Columns && a.Bindings == b.Bindings && a.Policy == b.Policy && strings.Join(a.Reasons, "\x00") == strings.Join(b.Reasons, "\x00") && strings.Join(a.Missing, "\x00") == strings.Join(b.Missing, "\x00")
}
func stateRank(s analysis.State) int {
	switch s {
	case analysis.Known:
		return 0
	case analysis.Partial:
		return 1
	case analysis.Unknown:
		return 2
	default:
		return 3
	}
}
func worsened(a, b ciFinding) bool {
	if riskRank(b.f.Level) > riskRank(a.f.Level) || precisionRank(b.f.AnalysisPrecision) > precisionRank(a.f.AnalysisPrecision) {
		return true
	}
	x, y := a.f.Detail.Normalize(), b.f.Detail.Normalize()
	return stateRank(y.SQL) > stateRank(x.SQL) || stateRank(y.Columns) > stateRank(x.Columns) || stateRank(y.Bindings) > stateRank(x.Bindings) || stateRank(y.Policy) > stateRank(x.Policy)
}
func sameDiagnostic(a, b ciFinding) bool {
	return a.key() == b.key() && a.operation == b.operation && a.f.Level == b.f.Level && a.f.AnalysisPrecision == b.f.AnalysisPrecision && a.f.Suppressed == b.f.Suppressed && equalDetail(a.f.Detail, b.f.Detail) && bytes.Equal(a.source, b.source)
}
