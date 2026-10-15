package review

import (
	"encoding/json"
	"fmt"

	"github.com/recoweft/goquent/orm/internal/publicview"
)

const CIViewKind = "goquent.review_ci_view"

// MarshalJSON closes all classifications. A modified public view cannot change
// the private CIResult or its gate, and is never an input to RunCI.
func (v CIView) MarshalJSON() ([]byte, error) {
	if v.Kind != CIViewKind || v.Version != 1 {
		return nil, publicview.ErrInvalid
	}
	v.DetailsOmitted = true
	v.BaselineReason = "review_annotation_only"
	v.GitTracking = "not_assessed"
	counts := []*int{&v.FullFindingCount, &v.FullSuppressedFindingCount,
		&v.Entries.Entries, &v.Entries.Matched, &v.Entries.Expired, &v.Entries.Invalid, &v.Entries.Unused, &v.Entries.Unknown,
		&v.Findings.Matched, &v.Findings.New, &v.Findings.Worsened, &v.Findings.Unknown, &v.Findings.Unchanged,
		&v.Diff.Modified, &v.Diff.Added, &v.Diff.Deleted, &v.Diff.Renamed, &v.Diff.Binary, &v.Diff.Invalid, &v.Diff.Unknown, &v.Diff.Unlisted, &v.Diff.ReadFailed}
	for _, n := range counts {
		*n = max(0, *n)
	}
	type wire CIView
	b, e := json.Marshal(wire(v))
	if e != nil {
		return nil, publicview.ErrInvalid
	}
	return b, nil
}
func (v CIView) String() string {
	b, e := v.MarshalJSON()
	if e != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v CIView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }

// DecodeCIView reads only the new strict display envelope, not source evidence.
func DecodeCIView(b []byte) (CIView, error) {
	type wire CIView
	var v wire
	if publicview.Decode(b, CIViewKind, &v) != nil {
		return CIView{}, publicview.ErrInvalid
	}
	// Reuse the publication normalization for the independent display reader.
	safe, e := CIView(v).MarshalJSON()
	if e != nil {
		return CIView{}, e
	}
	var normalized wire
	if json.Unmarshal(safe, &normalized) != nil {
		return CIView{}, publicview.ErrInvalid
	}
	return CIView(normalized), nil
}

// UnmarshalJSON enforces the same independent strict reader for standard JSON use.
func (v *CIView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = CIView{}
	out, e := DecodeCIView(b)
	if e == nil {
		*v = out
	}
	return e
}
