package query

import (
	"context"
	sqldriver "database/sql/driver"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
)

func tenantTestSettings(t *testing.T, dialect string, auto bool) Settings {
	t.Helper()
	typ := "INT"
	if dialect == "postgres" {
		typ = "integer"
	}
	var cols []WriteKeyColumn
	for _, name := range []string{"tenant_id", "id", "score", "deleted", "secret", "fixed", "forbidden"} {
		cols = append(cols, WriteKeyColumn{Name: name, DBType: typ, Bits: 32})
	}
	input := ApplicationSchemaInput{Database: "app-db", Dialect: dialect, Tables: []ApplicationTable{{Table: "users", PlainTable: true, Columns: cols, CompleteUniqueConstraints: true, Constraints: []WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols[:2]}}}}}
	schema, err := NewApplicationSchema(input)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPolicySet(TablePolicy{Table: "users", TenantColumn: "tenant_id", PIIColumns: []string{"secret"}, ImmutableColumns: []string{"fixed"}, ForbiddenColumns: []string{"forbidden"}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 1})
	if err != nil {
		t.Fatal(err)
	}
	return NewSettings(p, RiskConfig{}, c).WithTenantPolicy("app-db", schema, auto)
}
func tenantPlanAllowed(q *Query) (bool, *QueryPlan, error) {
	p, err := q.Plan(context.Background())
	if err != nil {
		return false, p, err
	}
	return ensurePlanExecutable(p) == nil, p, nil
}
func TestTenantAllBranches(t *testing.T) {
	cases := []struct {
		name  string
		build func(*Query) *Query
		allow bool
	}{
		{"equal", func(q *Query) *Query { return q.Where("tenant_id", 1) }, true},
		{"and", func(q *Query) *Query { return q.Where("tenant_id", 1).Where("id", 2) }, true},
		{"all_or", func(q *Query) *Query { return q.Where("tenant_id", 1).OrWhere("tenant_id", int64(1)) }, true},
		{"one_or", func(q *Query) *Query { return q.Where("tenant_id", 1).OrWhere("id", 2) }, false},
		{"nested_or", func(q *Query) *Query {
			return q.WhereGroup(func(g *Query) { g.Where("tenant_id", 1).OrWhere("id", 2) }).Where("score", 3)
		}, false},
		{"outer_and", func(q *Query) *Query {
			return q.WhereGroup(func(g *Query) { g.Where("id", 1).OrWhere("id", 2) }).Where("tenant_id", 1)
		}, true},
		{"not", func(q *Query) *Query { return q.WhereNot(func(g *Query) { g.Where("tenant_id", 1) }) }, false},
		{"not_outer_and", func(q *Query) *Query { return q.WhereNot(func(g *Query) { g.Where("id", 1) }).Where("tenant_id", 1) }, true},
		{"ne", func(q *Query) *Query { return q.Where("tenant_id", "!=", 1) }, false},
		{"range", func(q *Query) *Query { return q.Where("tenant_id", ">", 0) }, false},
		{"in", func(q *Query) *Query { return q.WhereIn("tenant_id", []int{1}) }, false},
		{"null", func(q *Query) *Query { return q.WhereNull("tenant_id") }, false},
		{"not_null", func(q *Query) *Query { return q.WhereNotNull("tenant_id") }, false},
		{"nil", func(q *Query) *Query { return q.Where("tenant_id", nil) }, false},
		{"column", func(q *Query) *Query { return q.WhereColumn("tenant_id", "=", "id") }, false},
		{"other", func(q *Query) *Query { return q.Where("tenant_id", 2) }, false},
		{"coercion", func(q *Query) *Query { return q.Where("tenant_id", "1") }, false},
		{"raw", func(q *Query) *Query { return q.Where("tenant_id", 1).SafeWhereRaw("id > :n", map[string]any{"n": 0}) }, false},
		{"raw_projection", func(q *Query) *Query { return q.SelectRaw("1").Where("tenant_id", 1) }, false},
		{"pii", func(q *Query) *Query { return q.Select("secret").Where("tenant_id", 1).AccessReason("reason") }, false},
	}
	for _, d := range []struct {
		name    string
		dialect driver.Dialect
	}{{"mysql", driver.MySQLDialect{}}, {"postgres", driver.PostgresDialect{}}} {
		for _, tc := range cases {
			t.Run(d.name+"/"+tc.name, func(t *testing.T) {
				e := &recordingExec{}
				q := tc.build(NewWithSettings(e, "users", d.dialect, tenantTestSettings(t, d.name, false)).Select("id").Limit(5))
				allowed, p, err := tenantPlanAllowed(q)
				if allowed != tc.allow {
					t.Fatalf("allowed=%v error=%v result=%+v", allowed, err, p)
				}
				if e.calls != 0 {
					t.Fatal("plan executed")
				}
				if !tc.allow {
					q.RequireApproval("not authority").SuppressWarning("TENANT_POLICY_REJECTED", "not authority")
					var dest []map[string]any
					_ = q.Get(&dest)
					if e.calls != 0 {
						t.Fatal("rejected gate called executor")
					}
				}
			})
		}
	}
}
func TestTenantSupplyAndSchema(t *testing.T) {
	s := tenantTestSettings(t, "mysql", false)
	inputs := []ExecutionContext{{}}
	old, _ := NewExecutionContext(s.execution.Input())
	inputs = append(inputs, old)
	for _, v := range []any{nil, "", "1", float64(1), []int{1}, map[string]any{"tenant_verified": true}} {
		c, _ := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: v})
		inputs = append(inputs, c)
	}
	for _, c := range inputs {
		q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s.WithExecutionContext(c)).Select("id").Where("tenant_id", 1)
		if ok, _, _ := tenantPlanAllowed(q); ok {
			t.Fatal("unconfirmed/unsupported accepted")
		}
	}
	for _, bad := range []Settings{s.WithTenantPolicy("wrong", s.schema, false), s.WithTenantPolicy("app-db", ApplicationSchema{}, false)} {
		if ok, _, _ := tenantPlanAllowed(NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, bad).Select("id").Where("tenant_id", 1)); ok {
			t.Fatal("schema mismatch accepted")
		}
	}
	input := s.schema.Input()
	input.Tables[0].Columns[0].Bits = 16
	input.Tables[0].Constraints[0].Columns[0].Name = "changed"
	if s.schema.Input().Tables[0].Columns[0].Bits != 32 {
		t.Fatal("schema shared")
	}
	q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s).Select("id").Where("tenant_id", 1)
	ok, p, err := tenantPlanAllowed(q)
	if !ok || err != nil {
		t.Fatalf("valid %v %+v", err, p)
	}
	raw, _ := json.Marshal(p)
	var restored QueryPlan
	if err := json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.tenantEvidence != nil {
		t.Fatal("JSON promoted evidence")
	}
	p.Params[0] = 2
	p.Blocked = false
	p.RiskLevel = RiskLow
	p.Warnings = nil
	if ensurePlanExecutable(p) == nil {
		t.Fatal("changed values accepted")
	}
}
func TestTenantAutomaticOuterAnd(t *testing.T) {
	s := tenantTestSettings(t, "mysql", true)
	pset, _ := NewPolicySet(TablePolicy{Table: "users", TenantColumn: "tenant_id", SoftDeleteColumn: "deleted"})
	s = s.WithPolicySet(pset)
	q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s).Select("id").Where("id", 1).OrWhere("id", 2)
	before := q.builder.Snapshot()
	ok, p, err := tenantPlanAllowed(q)
	if !ok || err != nil {
		t.Fatalf("%v %+v", err, p)
	}
	if !strings.Contains(p.SQL, "(`id` = ? OR `id` = ?) AND") || !reflect.DeepEqual(p.Params, []any{1, 2, 1}) {
		t.Fatalf("outer AND mismatch %s", p.SQL)
	}
	if !reflect.DeepEqual(before, q.builder.Snapshot()) {
		t.Fatal("caller builder changed")
	}
	q.OrWhere("id", 3)
	ok, next, err := tenantPlanAllowed(q)
	if !ok || err != nil || len(next.Params) != 4 || len(p.Params) != 3 {
		t.Fatalf("replanning %v", err)
	}
	if p.WhereTree.Kind != "and" {
		t.Fatal("tree disagrees")
	}
}
func TestTenantWriteRowsAndReturning(t *testing.T) {
	s := tenantTestSettings(t, "postgres", false)
	e := &recordingExec{}
	q := NewWithSettings(e, "users", driver.PostgresDialect{}, s)
	for _, rows := range [][]map[string]any{{{"id": 1}}, {{"id": 1, "tenant_id": nil}}, {{"id": 1, "tenant_id": 2}}, {{"id": 1, "tenant_id": 1}, {"id": 2, "tenant_id": 2}}} {
		if _, err := q.InsertBatch(rows); err == nil {
			t.Fatal("bad batch accepted")
		}
	}
	for _, col := range []string{"tenant_id", "fixed", "forbidden", "secret"} {
		if _, err := q.Where("tenant_id", 1).Update(map[string]any{col: 1}); err == nil {
			t.Fatal("protected update accepted")
		}
	}
	p, err := q.PlanInsert(context.Background(), map[string]any{"tenant_id": 1, "id": 1})
	if err != nil || ensurePlanExecutable(p) != nil {
		t.Fatalf("insert %v %+v", err, p)
	}
	if err := q.planReturning(p, []string{"secret"}); err == nil {
		t.Fatal("protected returning accepted")
	}
	if err := q.planReturning(p, []string{"id"}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.PrimaryKey("secret").InsertGetId(map[string]any{"tenant_id": 1, "id": 9}); err == nil {
		t.Fatal("protected InsertGetId accepted")
	}
	if e.calls != 0 {
		t.Fatal("rejected write called executor")
	}
	for _, d := range []string{"mysql", "postgres"} {
		s := tenantTestSettings(t, d, false)
		var dialect driver.Dialect = driver.MySQLDialect{}
		if d == "postgres" {
			dialect = driver.PostgresDialect{}
		}
		q := NewWithSettings(e, "users", dialect, s)
		rows := []map[string]any{{"tenant_id": 1, "id": 1, "score": 4}}
		p, err := q.planUpsert(context.Background(), rows, []string{"tenant_id", "id"}, []string{"score"})
		if err != nil || ensurePlanExecutable(p) != nil {
			t.Fatalf("upsert %s %v %+v", d, err, p)
		}
		if _, err = q.planUpsert(context.Background(), rows, []string{"tenant_id", "id"}, []string{"tenant_id"}); err == nil {
			t.Fatal("tenant upsert accepted")
		}
		input := s.schema.Input()
		key := input.Tables[0].Constraints[0]
		key.Columns = key.Columns[1:]
		input.Tables[0].Constraints = append(input.Tables[0].Constraints, key)
		schema, _ := NewApplicationSchema(input)
		q.settings = s.WithTenantPolicy("app-db", schema, false)
		_, err = q.planUpsert(context.Background(), rows, []string{"tenant_id", "id"}, []string{"score"})
		if (err != nil) != (d == "mysql") {
			t.Fatalf("dialect conflict difference %s %v", d, err)
		}
	}
}

func TestTenantAliasesAndUnknown(t *testing.T) {
	s := tenantTestSettings(t, "mysql", false)
	cases := []struct {
		name  string
		build func(*Query) *Query
		allow bool
	}{
		{"self_both", func(q *Query) *Query {
			return q.Join("users as b", "a.id", "=", "b.id").Where("a.tenant_id", 1).Where("b.tenant_id", 1)
		}, true},
		{"self_one", func(q *Query) *Query { return q.Join("users as b", "a.id", "=", "b.id").Where("a.tenant_id", 1) }, false},
		{"ambiguous", func(q *Query) *Query { return q.Join("users as b", "a.id", "=", "b.id").Where("tenant_id", 1) }, false},
		{"left_on", func(q *Query) *Query {
			return q.LeftJoinQuery("users as b", func(j *JoinClause) { j.On("a.id", "=", "b.id").Where("b.tenant_id", "=", 1) }).Where("a.tenant_id", 1)
		}, true},
		{"left_on_or", func(q *Query) *Query {
			return q.LeftJoinQuery("users as b", func(j *JoinClause) { j.On("a.id", "=", "b.id").OrWhere("b.tenant_id", "=", 1) }).Where("a.tenant_id", 1)
		}, false},
		{"right", func(q *Query) *Query {
			return q.RightJoin("users as b", "a.id", "=", "b.id").Where("a.tenant_id", 1).Where("b.tenant_id", 1)
		}, false},
		{"subquery", func(q *Query) *Query {
			return q.WhereInSubQuery("a.id", NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s).Select("id")).Where("a.tenant_id", 1)
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := tc.build(NewWithSettings(&recordingExec{}, "users as a", driver.MySQLDialect{}, s).Select("a.id"))
			ok, p, err := tenantPlanAllowed(q)
			if ok != tc.allow {
				t.Fatalf("allowed=%v err=%v plan=%+v", ok, err, p)
			}
		})
	}
	p := NewRawPlanWithSettings(s, "WITH x AS (SELECT * FROM users) SELECT * FROM x")
	p.Blocked = false
	p.Warnings = nil
	p.Approval = &Approval{Reason: "reason"}
	if ensurePlanExecutable(p) == nil {
		t.Fatal("raw CTE bypass")
	}
}

func TestTenantCompatibilityAndReplacement(t *testing.T) {
	s := tenantTestSettings(t, "mysql", false)
	compat := NewSettings(s.policies, RiskConfig{}, s.execution)
	q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, compat).Select("id").Where("tenant_id", 2)
	if ok, _, _ := tenantPlanAllowed(q); !ok {
		t.Fatal("compatibility changed")
	}
	old, _ := NewExecutionContext(s.execution.Input())
	newS := s.WithExecutionContext(old)
	if newS.execution.application || !s.execution.application {
		t.Fatal("replacement mixed provenance")
	}
	newer, _ := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
	q = NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s.WithExecutionContext(newer)).Select("id").Where("tenant_id", 1)
	if ok, _, _ := tenantPlanAllowed(q); ok {
		t.Fatal("new tenant not rechecked")
	}
}

func TestTenantMalformedSchemaAndConstraints(t *testing.T) {
	s := tenantTestSettings(t, "mysql", false)
	for _, change := range []func(*ApplicationSchemaInput){
		func(i *ApplicationSchemaInput) { i.Dialect = "postgres" },
		func(i *ApplicationSchemaInput) { i.Tables[0].Columns[0].Nullable = true },
		func(i *ApplicationSchemaInput) { i.Tables[0].Columns[0].Unsigned = true },
		func(i *ApplicationSchemaInput) { i.Tables[0].Columns[0].Bits = 16 },
		func(i *ApplicationSchemaInput) { i.Tables[0].PlainTable = false },
		func(i *ApplicationSchemaInput) { i.Tables[0].Table = "other" },
	} {
		input := s.schema.Input()
		change(&input)
		schema, err := NewApplicationSchema(input)
		if err != nil {
			continue
		}
		q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s.WithTenantPolicy("app-db", schema, false)).Select("id").Where("tenant_id", 1)
		if ok, _, _ := tenantPlanAllowed(q); ok {
			t.Fatal("bad schema accepted")
		}
	}
	for _, change := range []func(*ApplicationTable){
		func(t *ApplicationTable) { t.CompleteUniqueConstraints = false },
		func(t *ApplicationTable) { t.Constraints[0].Partial = true },
		func(t *ApplicationTable) { t.Constraints[0].Expression = true },
		func(t *ApplicationTable) { t.Constraints[0].NotDeferrable = false },
		func(t *ApplicationTable) { t.Constraints[0].Columns[1].Nullable = true },
	} {
		input := s.schema.Input()
		change(&input.Tables[0])
		schema, _ := NewApplicationSchema(input)
		q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s.WithTenantPolicy("app-db", schema, false))
		if _, err := q.planUpsert(context.Background(), []map[string]any{{"id": 1, "tenant_id": 1, "score": 2}}, []string{"tenant_id", "id"}, []string{"score"}); err == nil {
			t.Fatal("unsupported conflict accepted")
		}
	}
	input := s.schema.Input()
	input.Tables = make([]ApplicationTable, 65)
	if _, err := NewApplicationSchema(input); err == nil {
		t.Fatal("schema budget accepted")
	}
}

func TestTenantHighRiskCannotUseReasons(t *testing.T) {
	s := tenantTestSettings(t, "mysql", false)
	high := RiskHigh
	s = s.WithRiskConfig(RiskConfig{Rules: map[string]RiskRuleConfig{WarningLimitMissing: {Severity: &high}}})
	e := &recordingExec{}
	q := NewWithSettings(e, "users", driver.MySQLDialect{}, s).Select("id").Where("tenant_id", 1).RequireApproval("intent").SuppressWarning(WarningLimitMissing, "intent")
	if ok, _, _ := tenantPlanAllowed(q); ok {
		t.Fatal("configured high risk bypass")
	}
	var rows []map[string]any
	_ = q.Get(&rows)
	if e.calls != 0 {
		t.Fatal("missing permit called executor")
	}
}

type tenantMethodTrap struct{}

func (tenantMethodTrap) Value() (sqldriver.Value, error) { panic("Valuer called") }
func (tenantMethodTrap) String() string                  { panic("Stringer called") }
func (tenantMethodTrap) MarshalJSON() ([]byte, error)    { panic("Marshaler called") }
func TestTenantNoUserMethods(t *testing.T) {
	if _, err := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: tenantMethodTrap{}}); err == nil {
		t.Fatal("custom tenant accepted")
	}
	s := tenantTestSettings(t, "mysql", false)
	e := &recordingExec{}
	q := NewWithSettings(e, "users", driver.MySQLDialect{}, s).Select("id").Where("tenant_id", 1).Where("score", tenantMethodTrap{})
	if ok, _, _ := tenantPlanAllowed(q); ok {
		t.Fatal("custom predicate accepted")
	}
	if _, err := q.Insert(map[string]any{"tenant_id": 1, "score": tenantMethodTrap{}}); err == nil {
		t.Fatal("custom assignment accepted")
	}
	if e.calls != 0 {
		t.Fatal("unsupported executed")
	}
}
func TestTenantConcurrentDifferentPolicies(t *testing.T) {
	first := tenantTestSettings(t, "mysql", true)
	p, _ := NewPolicySet(TablePolicy{Table: "users", TenantColumn: "score"})
	c, _ := NewApplicationTenantContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: 2})
	second := first.WithPolicySet(p).WithExecutionContext(c)
	var wg sync.WaitGroup
	for index, s := range []Settings{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				q := NewWithSettings(&recordingExec{}, "users", driver.MySQLDialect{}, s).Select("id")
				ok, p, err := tenantPlanAllowed(q)
				if !ok || err != nil || len(p.Params) != 1 || p.Params[0] != index+1 {
					t.Errorf("context/policy mixed: %v", err)
					return
				}
				col := []string{"tenant_id", "score"}[index]
				if !strings.Contains(p.SQL, "`"+col+"`") {
					t.Error("policy mixed")
					return
				}
			}
		}()
	}
	wg.Wait()
}
