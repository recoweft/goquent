package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/recoweft/goquent/orm/query"
	"strings"
	"testing"
)

const migrationCanary = "canary@example.test_password_TOKEN"

type migrationBomb struct{}

func (migrationBomb) MarshalJSON() ([]byte, error) { panic("custom marshal") }
func (migrationBomb) String() string               { panic("custom string") }
func TestMigrationPublicBudgetAndTampering(t *testing.T) {
	p := &MigrationPlan{SQL: migrationCanary, RiskLevel: query.RiskLevel(migrationCanary), AnalysisPrecision: query.AnalysisPrecision(migrationCanary), Metadata: map[string]any{"x": migrationBomb{}}}
	cycle := map[string]any{}
	cycle["self"] = cycle
	p.Metadata["cycle"] = cycle
	for range 140 {
		step := MigrationStep{Type: MigrationStepType(migrationCanary), Table: migrationCanary, SQL: migrationCanary}
		for range 140 {
			step.Warnings = append(step.Warnings, query.Warning{Code: migrationCanary, Message: migrationCanary, Hint: migrationCanary, Evidence: []query.Evidence{{Key: migrationCanary, Value: migrationBomb{}}}})
		}
		p.Steps = append(p.Steps, step)
	}
	v, err := p.PublicView()
	if err != nil {
		t.Fatal(err)
	}
	v.Risk = migrationCanary
	v.Steps[0].Type = migrationCanary
	b, err := v.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 1<<20 || !v.Truncated || len(v.Steps) != 128 {
		t.Fatal("bound")
	}
	count := 1 + len(v.Steps) + len(v.Warnings)
	for _, s := range v.Steps {
		count += len(s.Warnings)
	}
	if count > 1024 {
		t.Fatal("aggregate element budget")
	}
	decoded, err := DecodeMigrationPlanView(b)
	if err != nil || !decoded.Truncated {
		t.Fatal("serializer/reader boundary")
	}
	var out bytes.Buffer
	if WritePretty(&out, p) != nil {
		t.Fatal("pretty")
	}
	for _, text := range []string{string(b), v.String(), p.String(), out.String(), fmt.Sprintf("%#v", v), fmt.Sprintf("%+v", v.Steps[0])} {
		if strings.Contains(text, migrationCanary) {
			t.Fatal("leak")
		}
	}
	if p.SQL != migrationCanary || p.RiskLevel != query.RiskLevel(migrationCanary) {
		t.Fatal("source changed")
	}
	for _, bad := range []string{`null`, `{"kind":"goquent.migration_plan_view","version":1.0}`, `{"kind":"goquent.migration_plan_view","version":1,"version":1}`, `{"kind":"goquent.migration_plan_view","version":2}`, string(b) + `{}`, strings.Replace(string(b), `"kind"`, `"Kind"`, 1)} {
		if _, err := DecodeMigrationPlanView([]byte(bad)); err == nil {
			t.Fatal("invalid wire accepted")
		}
	}
	if err := WritePublicJSON(nil, v); err == nil {
		t.Fatal("nil writer")
	}
	if _, err := json.Marshal(v); err != nil {
		t.Fatal(err)
	}
}
