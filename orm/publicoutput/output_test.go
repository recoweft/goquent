package publicoutput

import (
	"fmt"
	"strings"
	"testing"
)

func TestSummaryViewsSanitizeAndBound(t *testing.T) {
	for _, kind := range []string{"goquent.manifest_view", "goquent.schema_view", "goquent.models_view", "goquent.relations_view", "goquent.policies_view", "goquent.query_examples_view", "goquent.verification_view", "goquent.migration_status_view", "goquent.drift_view"} {
		v := SummaryView{Kind: kind, Version: 1, Fresh: true, Items: make([]ItemView, 1025)}
		for i := range v.Items {
			v.Items[i] = ItemView{Check: "canary@example.test", Status: "canary@example.test"}
		}
		b, err := v.ToJSON()
		if err != nil {
			t.Fatal(err)
		}
		out, err := DecodeSummaryView(b)
		if err != nil || out.Fresh || !out.Truncated || !out.DetailsOmitted || len(out.Items) != 128 || len(b) > 1<<20 {
			t.Fatal("unsafe summary bounds/unknown freshness")
		}
		for _, s := range []string{string(b), fmt.Sprintf("%+v %#v", v, v), fmt.Sprint(v.Items[0])} {
			if strings.Contains(s, "canary@example.test") {
				t.Fatal("tampering leaked")
			}
		}
		for _, bad := range []string{`null`, strings.Replace(string(b), `"version":1`, `"version":1e0`, 1), strings.Replace(string(b), `"known":false`, `"known":null`, 1), strings.Replace(string(b), `"kind"`, `"Kind"`, 1), string(b) + ` {}`} {
			if _, err := DecodeSummaryView([]byte(bad)); err == nil {
				t.Fatal("invalid wire")
			}
		}
	}
	if WriteSummary(nil, SummaryView{Kind: "goquent.schema_view", Version: 1}) == nil {
		t.Fatal("nil writer")
	}
}
