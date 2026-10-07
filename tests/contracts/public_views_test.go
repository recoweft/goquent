package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/orm/review"
)

func TestPublicViewsOperationValuesAndLegacyBoundary(t *testing.T) {
	const secret = "fictional-view@example.invalid gq_fake_token_06 gq_fake_password_06 gq_fake_person_06"
	m, err := manifest.Load("../../examples/ai-safe-orm/goquent.manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	// This canary checks an ordinary value ref; tenant claims now refuse.
	for i := range m.Tables[0].Columns {
		m.Tables[0].Columns[i].TenantScope = false
	}
	for i := range m.Tables[0].Policies {
		if m.Tables[0].Policies[i].Type == "tenant_scope" {
			m.Tables[0].Policies[i].Type = "required_filter"
		}
	}
	spec := operation.OperationSpec{Model: "users", Select: []string{"id"}, Filters: []operation.FilterSpec{{Field: "tenant_id", Op: "=", ValueRef: secret}}, AccessReason: secret}
	p, err := operation.Compile(context.Background(), spec, operation.Options{Manifest: m, Values: map[string]any{secret: secret}})
	if err != nil {
		t.Fatal("compile failed")
	}
	var retained bool
	for _, v := range p.Params {
		if v == secret {
			retained = true
		}
	}
	if !retained {
		t.Fatal("value_ref not retained")
	}
	p.Metadata = map[string]any{"reason": secret, "nested": p}
	v, err := p.PublicView()
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), secret) {
		t.Fatal("value reference leak")
	}
	report := review.ReviewReport{Findings: []review.Finding{{Code: query.WarningRawSQLUsed, Evidence: []query.Evidence{{Key: secret, Value: p}}, Message: secret}}}
	rv, err := report.PublicView()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = review.WritePublicJSON(&out, rv); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), secret) {
		t.Fatal("nested plan leak")
	}
	// PR1 deliberately leaves the old diagnostic serializer unchanged.
	legacy := &query.QueryPlan{SQL: secret, Params: []any{secret}}
	old, err := legacy.ToJSON()
	if err != nil || !bytes.Contains(old, []byte(secret)) {
		t.Fatal("legacy compatibility changed")
	}
	var handle query.ValidatedPlan
	if err = json.Unmarshal(b, &handle); err == nil {
		t.Fatal("public view restored handle")
	}
}
