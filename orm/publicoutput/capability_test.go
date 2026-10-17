package publicoutput

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCapabilityViewsCloseVocabulary(t *testing.T) {
	canary := "private_capability_canary"
	c := CapabilityView{Operation: canary, Source: canary, LockFree: CapabilityAssessment{canary, canary}}
	for _, v := range []any{c, c.LockFree, SummaryView{Kind: "goquent.drift_view", Version: 1, Capabilities: []CapabilityView{c}}} {
		b, e := json.Marshal(v)
		if e != nil || strings.Contains(string(b), canary) || strings.Contains(fmt.Sprintf("%+v", v), canary) {
			t.Fatal("capability source leak")
		}
	}
	v := SummaryView{Kind: "goquent.drift_view", Version: 1, Capabilities: make([]CapabilityView, 200)}
	b, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	out, e := DecodeSummaryView(b)
	if e != nil || len(out.Capabilities) != 128 || !out.Truncated {
		t.Fatal("capability bounds/roundtrip")
	}
}
