package manifest

import "github.com/recoweft/goquent/orm/publicoutput"

// PublicView omits names, fingerprints and all opaque or free-form data.
func (m *Manifest) PublicView() publicoutput.SummaryView {
	v := publicoutput.SummaryView{Kind: "goquent.manifest_view", Version: 1, DetailsOmitted: true}
	if m == nil {
		return v
	}
	v.Present = true
	v.Count = len(m.Tables)
	v.Truncated = len(m.Tables) > 128
	for _, t := range m.Tables[:min(128, len(m.Tables))] {
		v.Items = append(v.Items, publicoutput.ItemView{Ordinal: len(v.Items) + 1, Columns: len(t.Columns), Indexes: len(t.Indexes), Relations: len(t.Relations), Policies: len(t.Policies), Examples: len(t.QueryExamples)})
	}
	if m.Verification != nil {
		v.Known = true
		v.Fresh = m.Verification.Fresh
	}
	return v
}

// PublicView describes supplied verification claims, not live database evidence.
func (v Verification) PublicView() publicoutput.SummaryView {
	out := publicoutput.SummaryView{Kind: "goquent.verification_view", Version: 1, Present: true, Known: true, Fresh: v.Fresh, Count: len(v.Checks), Truncated: len(v.Checks) > 128, DetailsOmitted: true}
	for _, c := range v.Checks[:min(128, len(v.Checks))] {
		out.Items = append(out.Items, publicoutput.ItemView{Check: c.Name, Status: c.Status})
	}
	// Normalize before returning so projection itself never contains arbitrary text.
	b, _ := out.ToJSON()
	out, _ = publicoutput.DecodeSummaryView(b)
	return out
}
