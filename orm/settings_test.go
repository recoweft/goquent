package orm

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

func settingsFixture(t *testing.T, tenant string, level RiskLevel) Settings {
	t.Helper()
	p, err := NewPolicySet(TablePolicy{Table: "settings_rows", TenantColumn: tenant, TenantMode: PolicyModeBlock, PIIColumns: []string{"secret"}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewExecutionContext(ExecutionContextInput{Source: "application", TenantPresent: true, CurrentTenant: map[string]any{"value": []byte(tenant)}})
	if err != nil {
		t.Fatal(err)
	}
	return NewSettings(p, RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &level}}}, c)
}

func assertSettings(t *testing.T, db *DB, want Settings) {
	t.Helper()
	if !reflect.DeepEqual(db.Settings(), want) {
		t.Fatal("settings changed")
	}
	q := db.Table("settings_rows")
	if !reflect.DeepEqual(q.Settings(), want) {
		t.Fatal("query did not inherit settings")
	}
	p, err := q.Select("id").Plan(context.Background())
	if err != nil || !p.Blocked {
		t.Fatalf("missing tenant presence should block: %v %+v", err, p)
	}
}

func TestDBSettingsIsolationAndOwnership(t *testing.T) {
	cols := []string{"secret"}
	policy, err := NewPolicySet(TablePolicy{Table: "settings_rows", TenantColumn: "tenant_a", TenantMode: PolicyModeBlock, PIIColumns: cols})
	if err != nil {
		t.Fatal(err)
	}
	level := RiskHigh
	enabled, suppressible, reason := true, false, true
	risk := RiskConfig{Environment: "a", Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &level, Enabled: &enabled, Suppressible: &suppressible, RequiresReason: &reason}}}
	payload := map[string]any{"value": []byte("a")}
	execution, err := NewExecutionContext(ExecutionContextInput{Source: "app-a", TenantPresent: true, CurrentTenant: payload})
	if err != nil {
		t.Fatal(err)
	}
	option := WithRiskConfig(risk)
	a := NewDBWithExecutor(nil, driver.MySQLDialect{}, WithPolicySet(policy), option, WithExecutionContext(execution))
	b := NewDBWithExecutor(nil, driver.PostgresDialect{}, WithSettings(settingsFixture(t, "tenant_b", RiskMedium)))
	oldQuery := a.Table("settings_rows").Select("id").Where("tenant_a", "a")
	want := a.Settings()
	cols[0] = "id"
	level, enabled, suppressible, reason = RiskLow, false, true, false
	delete(risk.Rules, WarningLimitMissing)
	payload["value"].([]byte)[0] = 'x'
	got := a.Settings().RiskConfig()
	*got.Rules[WarningLimitMissing].Severity = RiskLow
	*got.Rules[WarningLimitMissing].Enabled = false
	*got.Rules[WarningLimitMissing].Suppressible = true
	*got.Rules[WarningLimitMissing].RequiresReason = false
	delete(got.Rules, WarningLimitMissing)
	policies := a.Settings().PolicySet().Policies()
	policies[0].PIIColumns[0] = "id"
	single, _ := a.Settings().PolicySet().PolicyForTable("settings_rows")
	single.PIIColumns[0] = "id"
	c := a.Settings().ExecutionContext().Input()
	c.CurrentTenant.(map[string]any)["value"].([]byte)[0] = 'z'
	if !reflect.DeepEqual(want, a.Settings()) || a.Settings().ExecutionContext().Input().CurrentTenant.(map[string]any)["value"].([]byte)[0] != 'a' {
		t.Fatal("input or getter mutated snapshot")
	}
	// Options also own their input before application to another DB.
	later := NewDBWithExecutor(nil, driver.MySQLDialect{}, option)
	if later.Settings().RiskConfig().Rules[WarningLimitMissing].Severity == nil || *later.Settings().RiskConfig().Rules[WarningLimitMissing].Severity != RiskHigh {
		t.Fatal("option captured live risk input")
	}
	derived := a.WithOptions(WithSettings(b.Settings()))
	if !reflect.DeepEqual(derived.Settings(), b.Settings()) {
		t.Fatal("explicit replacement ignored")
	}
	p, err := oldQuery.Plan(context.Background())
	if err != nil || p.Blocked || p.RiskLevel != RiskHigh {
		t.Fatalf("existing query changed: %v %+v", err, p)
	}
	var wg sync.WaitGroup
	for _, item := range []struct {
		db    *DB
		col   string
		level RiskLevel
	}{{a, "tenant_a", RiskHigh}, {b, "tenant_b", RiskMedium}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				p, err := item.db.Clone().Table("settings_rows").Select("id").Where(item.col, "own").Plan(context.Background())
				if err != nil || p.Blocked || p.RiskLevel != item.level {
					t.Errorf("mixed snapshot: %v %+v", err, p)
					return
				}
				raw, err := item.db.RawPlan(context.Background(), "SELECT 1")
				if err != nil || raw.RiskLevel != RiskHigh {
					t.Errorf("raw defaults: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestDBLegacySettingsSnapshotAndExplicitPrecedence(t *testing.T) {
	old := query.DefaultRiskEngine
	oldPolicies := RegisteredTablePolicies()
	ResetModelPolicies()
	t.Cleanup(func() {
		query.DefaultRiskEngine = old
		ResetModelPolicies()
		for _, p := range oldPolicies {
			_ = RegisterTablePolicy(p)
		}
	})
	high := RiskHigh
	query.DefaultRiskEngine = NewRiskEngine(RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &high}}})
	if err := RegisterTablePolicy(TablePolicy{Table: "settings_rows", TenantColumn: "old", TenantMode: PolicyModeBlock}); err != nil {
		t.Fatal(err)
	}
	a := NewDBWithExecutor(nil, driver.MySQLDialect{})
	q := a.Table("settings_rows").Select("id").Where("old", 1)
	if err := RegisterTablePolicy(TablePolicy{Table: "settings_rows", TenantColumn: "new", TenantMode: PolicyModeBlock}); err != nil {
		t.Fatal(err)
	}
	query.DefaultRiskEngine = NewRiskEngine(RiskConfig{})
	b := NewDBWithExecutor(nil, driver.MySQLDialect{})
	p, err := q.Plan(context.Background())
	if err != nil || p.Blocked || p.RiskLevel != RiskHigh {
		t.Fatalf("old query read new globals: %v %+v", err, p)
	}
	p, err = a.Table("settings_rows").Select("id").Where("old", 1).Plan(context.Background())
	if err != nil || p.Blocked || p.RiskLevel != RiskHigh {
		t.Fatalf("old DB read new globals: %v %+v", err, p)
	}
	for _, db := range []*DB{b, a.WithOptions(WithSettings(SnapshotDefaultSettings()))} {
		p, err = db.Table("settings_rows").Select("id").Where("old", 1).Plan(context.Background())
		if err != nil || !p.Blocked {
			t.Fatalf("snapshot did not import new defaults: %v %+v", err, p)
		}
	}
	explicit := a.WithOptions(WithSettings(Settings{}))
	ResetModelPolicies()
	_ = RegisterTablePolicy(TablePolicy{Table: "settings_rows", TenantColumn: "third", TenantMode: PolicyModeBlock})
	p, err = explicit.Table("settings_rows").Select("id").Limit(1).Plan(context.Background())
	if err != nil || p.Blocked || p.RiskLevel != RiskLow {
		t.Fatalf("explicit empty settings fell back to global: %v %+v", err, p)
	}
	// Existing DB reads never access global engines, even during legacy assignment.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			query.DefaultRiskEngine = NewRiskEngine(RiskConfig{})
			_ = RegisterTablePolicy(TablePolicy{Table: "other"})
		}
	}()
	for i := 0; i < 100; i++ {
		if _, err := a.Table("settings_rows").Plan(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	<-done
}

func TestDBSettingsTransactionAndExecutorInheritance(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	settings := settingsFixture(t, "tenant_a", RiskHigh)
	db := NewDB(sqlDB, driver.MySQLDialect{}, WithSettings(settings), WithBoolScanPolicy(BoolStrict)).RequireRawApproval("reviewed").TouchedTables("settings_rows")
	for _, child := range []*DB{db.Clone(), db.WithOptions(), db.RequireRawApproval("other"), db.TouchedTables("other"), db.WrapExecutor(&rawQueryRowExecutor{})} {
		assertSettings(t, child, settings)
		if child.scanOpts != db.scanOpts {
			t.Fatal("scan options lost")
		}
	}
	external := db.WrapExecutor(&rawQueryRowExecutor{})
	if external.SQLDB() != nil || external.Close() != nil {
		t.Fatal("external wrapper acquired ownership")
	}
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	assertSettings(t, tx.DB, settings)
	assertSettings(t, tx.Clone(), settings)
	// Parent changes create a new wrapper, leaving the active transaction intact.
	_ = db.WithOptions(WithSettings(Settings{}))
	assertSettings(t, tx.DB, settings)
	wrapped := db.WrapTx(tx.Tx.Tx)
	if wrapped.SQLDB() != sqlDB {
		t.Fatal("legacy WrapTx ownership changed")
	}
	assertSettings(t, wrapped, settings)
	standalone := NewTxDB(tx.Tx.Tx, driver.MySQLDialect{}, WithSettings(settings))
	assertSettings(t, standalone, settings)
	if standalone.SQLDB() != nil || standalone.Close() != nil {
		t.Fatal("standalone Tx acquired ownership")
	}
	missing := NewTxDB(tx.Tx.Tx, driver.MySQLDialect{})
	if missing.Settings().ExecutionContext().Input().TenantPresent {
		t.Fatal("standalone Tx inferred parent context")
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectCommit()
	if err := db.Transaction(func(tx Tx) error { assertSettings(t, tx.DB, settings); return nil }); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectRollback()
	sentinel := errors.New("callback failed")
	if err := db.TransactionContext(context.Background(), func(tx Tx) error { assertSettings(t, tx.DB, settings); return sentinel }); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	child, err := db.BeginTx(context.Background(), &sql.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertSettings(t, child.DB, settings)
	mock.ExpectCommit()
	if err := child.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type settingsOpaque struct{}

func (*settingsOpaque) Value() (sqldriver.Value, error) { panic("user Value must not run") }
func (*settingsOpaque) MarshalJSON() ([]byte, error)    { panic("user marshaler must not run") }
func (*settingsOpaque) String() string                  { panic("user String must not run") }

type settingsCustomEngine struct{}

func (settingsCustomEngine) CheckQuery(*query.QueryPlan) query.RiskResult {
	panic("custom engine must not run while snapshotting/planning")
}

func TestDBSettingsUnsupportedAndMissingContext(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, value := range []any{&settingsOpaque{}, map[string]any{"nested": &settingsOpaque{}}, cycle, make([]byte, (8<<20)+1)} {
		if _, err := NewExecutionContext(ExecutionContextInput{Source: "app", TenantPresent: true, CurrentTenant: value}); !errors.Is(err, ErrUnsupportedExecutionContext) {
			t.Fatalf("expected unsupported for %T: %v", value, err)
		}
	}
	for _, input := range []ExecutionContextInput{{}, {Source: "unconfirmed"}, {Source: "app", TenantPresent: true}, {TenantPresent: true, CurrentTenant: ""}, {TenantPresent: true, CurrentTenant: "T1"}} {
		c, err := NewExecutionContext(input)
		if err != nil || !reflect.DeepEqual(input, c.Input()) {
			t.Fatalf("absence/provenance changed: %v", err)
		}
	}
	if _, err := NewExecutionContext(ExecutionContextInput{CurrentTenant: "T1"}); err == nil {
		t.Fatal("inconsistent presence accepted")
	}
	old := query.DefaultRiskEngine
	query.DefaultRiskEngine = settingsCustomEngine{}
	defer func() { query.DefaultRiskEngine = old }()
	db := NewDBWithExecutor(nil, driver.MySQLDialect{})
	if !errors.Is(db.Settings().Err(), ErrUnsupportedRiskEngine) {
		t.Fatal("custom engine silently accepted")
	}
	if _, err := db.Table("rows").Plan(context.Background()); !errors.Is(err, ErrUnsupportedRiskEngine) {
		t.Fatal(err)
	}
	if _, err := db.RawPlan(context.Background(), "SELECT 1"); !errors.Is(err, ErrUnsupportedRiskEngine) {
		t.Fatal(err)
	}
	// An explicit built-in config is a supported migration, not a silent fallback.
	explicit := db.WithOptions(WithRiskConfig(RiskConfig{}))
	if _, err := explicit.Table("rows").Plan(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDBSettingsRawRiskAndSoftDeleteUseLocalSnapshots(t *testing.T) {
	high, blocked := RiskHigh, RiskBlocked
	for _, tc := range []struct {
		column  string
		level   *RiskLevel
		blocked bool
	}{{"deleted_a", &high, false}, {"deleted_b", &blocked, true}} {
		exec := &rawQueryRowExecutor{}
		policies, err := NewPolicySet(TablePolicy{Table: "rows", SoftDeleteColumn: tc.column})
		if err != nil {
			t.Fatal(err)
		}
		db := NewDBWithExecutor(exec, driver.MySQLDialect{}, WithPolicySet(policies), WithRiskConfig(RiskConfig{Rules: map[string]RiskRuleConfig{query.WarningRawSQLUsed: {Severity: tc.level}}})).RequireRawApproval("test reviewed raw SQL")
		q := db.Table("rows").Select("id").Limit(1)
		p, err := q.Plan(context.Background())
		if err != nil || !query.PlanHasPredicateColumn(p, "rows", tc.column) {
			t.Fatalf("local soft delete not applied: %v", err)
		}
		raw, err := db.RawPlan(context.Background(), "SELECT 1")
		if err != nil || raw.RiskLevel != *tc.level {
			t.Fatalf("raw ignored DB risk: %v", err)
		}
		_, err = db.QueryRowE(context.Background(), "SELECT 1")
		if tc.blocked {
			if !errors.Is(err, query.ErrBlockedOperation) || len(exec.queryRowsContext) != 0 {
				t.Fatal("blocked local risk did not gate executor")
			}
		} else if err != nil || len(exec.queryRowsContext) != 1 {
			t.Fatalf("approved local raw query not delegated: %v", err)
		}
	}
}
