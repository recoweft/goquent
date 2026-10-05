package query

import (
	"context"
	"errors"
	"github.com/recoweft/goquent/orm/driver"
	"testing"
)

func TestQuerySettingsSnapshotIncludesJoinedPolicies(t *testing.T) {
	old := RegisteredTablePolicies()
	ResetPolicyRegistry()
	defer func() {
		ResetPolicyRegistry()
		for _, p := range old {
			_ = RegisterTablePolicy(p)
		}
	}()
	_ = RegisterTablePolicy(TablePolicy{Table: "joined", TenantColumn: "tenant_old", TenantMode: PolicyModeBlock})
	q := New(nil, "root", driver.MySQLDialect{}).Select("root.id").Join("joined", "root.id", "=", "joined.id").Where("joined.tenant_old", 1).Limit(1)
	_ = RegisterTablePolicy(TablePolicy{Table: "joined", TenantColumn: "tenant_new", TenantMode: PolicyModeBlock})
	p, err := q.Plan(context.Background())
	if err != nil || p.Blocked {
		t.Fatalf("joined policy read global at finalization: %v %+v", err, p)
	}
	later := New(nil, "root", driver.MySQLDialect{}).Select("root.id").Join("joined", "root.id", "=", "joined.id").Where("joined.tenant_old", 1).Limit(1)
	p, err = later.Plan(context.Background())
	if err != nil || !p.Blocked {
		t.Fatalf("new query failed to snapshot new joined policy: %v %+v", err, p)
	}
}

type opaqueSettingsEngine struct{}

func (opaqueSettingsEngine) CheckQuery(*QueryPlan) RiskResult { panic("must not call opaque engine") }

func TestQuerySettingsUnknownEngineBlocksRawAndDSL(t *testing.T) {
	old := DefaultRiskEngine
	DefaultRiskEngine = opaqueSettingsEngine{}
	defer func() { DefaultRiskEngine = old }()
	if _, err := New(nil, "root", driver.MySQLDialect{}).Plan(context.Background()); !errors.Is(err, ErrUnsupportedRiskEngine) {
		t.Fatal(err)
	}
	p := NewRawPlan("SELECT 1")
	if !p.Blocked || !errors.Is(EnsurePlanExecutable(p), ErrBlockedOperation) {
		t.Fatal("unsupported engine raw plan allowed")
	}
	p = NewRawPlanWithSettings(Settings{}, "SELECT 1")
	if p.Blocked || p.RiskLevel != RiskHigh {
		t.Fatal("explicit raw risk settings ignored")
	}
}

func TestRiskEngineConfigOwnsAllPointers(t *testing.T) {
	level := RiskHigh
	yes, no := true, false
	config := RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &level, Enabled: &yes, Suppressible: &no, RequiresReason: &yes}}}
	engine := NewRiskEngine(config)
	level = RiskLow
	yes, no = false, true
	delete(config.Rules, WarningLimitMissing)
	result := engine.CheckQuery(&QueryPlan{Operation: OperationSelect, Columns: []ColumnRef{{Name: "id"}}})
	if result.Level != RiskHigh || len(result.Warnings) != 1 || result.Warnings[0].Suppressible || !result.Warnings[0].RequiresReason {
		t.Fatalf("config remained live: %+v", result)
	}
}
