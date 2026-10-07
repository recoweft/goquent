package query

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/predicate"
)

var viewCanaries = []string{"fictional-view@example.invalid", "gq_fake_token_06", "gq_fake_password_06", "gq_fake_person_06"}

type viewBomb struct{}

func (viewBomb) String() string               { panic("unexpected Stringer") }
func (viewBomb) MarshalJSON() ([]byte, error) { panic("unexpected Marshaler") }
func (viewBomb) Value() (driver.Value, error) { panic("unexpected Valuer") }
func (viewBomb) Error() string                { panic("unexpected error display") }
func assertViewSafe(t *testing.T, outputs ...string) {
	t.Helper()
	for _, out := range outputs {
		for _, secret := range viewCanaries {
			if strings.Contains(out, secret) {
				t.Fatal("public output contains fixed canary")
			}
		}
	}
}
func exercisePlanView(t *testing.T, v PlanView) {
	t.Helper()
	b, err := v.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	assertViewSafe(t, string(b), v.String(), fmt.Sprintf("%v %+v %#v %s %q %x", v, v, v, &v, &v, v))
	for _, x := range []any{v, &v, struct{ View PlanView }{v}, []PlanView{v}} {
		b, err := json.Marshal(x)
		if err != nil {
			t.Fatal(err)
		}
		assertViewSafe(t, string(b))
	}
	out, err := DecodePlanView(b)
	if err != nil {
		t.Fatal(err)
	}
	assertViewSafe(t, out.String())
}
func TestPublicPlanViewCanariesOpaqueAndMutation(t *testing.T) {
	for _, s := range viewCanaries {
		cycle := map[string]any{}
		cycle[s] = cycle
		var deep any = viewBomb{}
		for i := 0; i < 10000; i++ {
			deep = []any{deep}
		}
		tree := &predicate.Node{SQL: s, Raw: s, Column: s, Values: []predicate.Value{{Data: viewBomb{}}}, NamedValues: map[string]predicate.Value{s: {Data: s}}}
		tree.Children = []*predicate.Node{tree}
		p := &QueryPlan{Operation: OperationType(s), SQL: "WITH c AS (SELECT '" + s + "') SELECT $$" + s + "$$ /* " + s + " */", Params: []any{s, int16(7), json.Number("9007199254740993"), viewBomb{}}, Tables: []TableRef{{Name: s, Alias: s}}, Columns: []ColumnRef{{Name: s, Expression: s, Function: s}}, Joins: []JoinRef{{Table: s, OnTree: tree}}, Predicates: []PredicateRef{{Raw: s, Column: s}}, WhereTree: tree, HavingTree: tree, Unverified: []string{s}, Approval: &Approval{Reason: s, CreatedBy: s, Scope: s}, RiskLevel: RiskLevel(s), AnalysisPrecision: AnalysisPrecision(s), Metadata: map[string]any{s: cycle, "deep": deep, "huge": strings.Repeat(s, 100000), "nested": &QueryPlan{SQL: s}, "callback": func() { panic("callback") }, "error": viewBomb{}}, Warnings: []Warning{{Code: s, Level: RiskLevel(s), Message: s, Hint: s, Location: &SourceLocation{File: s, Line: 12, Column: 9}, Evidence: []Evidence{{Key: s, Value: viewBomb{}}}}}}
		p.SuppressedWarnings = p.Warnings
		v, err := p.PublicView()
		if err != nil {
			t.Fatal(err)
		}
		if v.Operation != "unknown" || v.Risk != "unknown" || v.Precision != "unknown" || v.Warnings[0].Code != "unknown" {
			t.Fatal("unknown became affirmative")
		}
		exercisePlanView(t, v)
		v.Operation = s
		v.Risk = s
		v.Precision = s
		v.Warnings[0].Code = s
		v.Warnings[0].Message = s
		v.Warnings[0].Level = s
		exercisePlanView(t, v)
		w := v.Warnings[0]
		b, _ := json.Marshal(w)
		assertViewSafe(t, string(b), fmt.Sprintf("%#v %+v %s", w, &w, w))
		if p.SQL == "" || p.Warnings[0].Message != s || p.WhereTree != tree || p.Params[0] != s {
			t.Fatal("source changed")
		}
		v.Kind = s
		_, err = v.ToJSON()
		if err == nil {
			t.Fatal("bad kind accepted")
		}
		assertViewSafe(t, err.Error(), v.String(), fmt.Sprintf("%#v", v))
	}
}
func TestPublicPlanViewBoundsAndLegacy(t *testing.T) {
	for _, n := range []int{0, 127, 128, 129, 10000} {
		p := &QueryPlan{Warnings: make([]Warning, n), SuppressedWarnings: make([]Warning, n)}
		v, err := p.PublicView()
		if err != nil {
			t.Fatal(err)
		}
		if len(v.Warnings) != min(n, 128) || len(v.SuppressedWarnings) != min(n, 128) || v.WarningCount != n || v.Truncated != (n > 128) {
			t.Fatal("incorrect bound")
		}
		exercisePlanView(t, v)
	}
	for _, version := range []int{0, 1, 2, -1} {
		_, err := (&QueryPlan{Version: version}).PublicView()
		if (err == nil) != (version == 0 || version == 1) {
			t.Fatal("source version")
		}
	}
	if _, err := (*QueryPlan)(nil).PublicView(); err == nil {
		t.Fatal("nil source")
	}
	var p QueryPlan
	if err := json.Unmarshal([]byte(`{"params":[9007199254740993,1.00,1e3]}`), &p); err != nil {
		t.Fatal(err)
	}
	before := append([]any(nil), p.Params...)
	v, err := p.PublicView()
	if err != nil {
		t.Fatal(err)
	}
	exercisePlanView(t, v)
	if !reflect.DeepEqual(before, p.Params) || p.Params[0] != json.Number("9007199254740993") {
		t.Fatal("numeric lexemes changed")
	}
}
func TestPublicPlanViewWireFixture(t *testing.T) {
	b, err := os.ReadFile("../../tests/contracts/testdata/public_views_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		PlanValid   []json.RawMessage `json:"plan_valid"`
		PlanInvalid []json.RawMessage `json:"plan_invalid"`
	}
	if err = json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, b := range fixture.PlanValid {
		v, e := DecodePlanView(b)
		if e != nil {
			t.Fatal(e)
		}
		exercisePlanView(t, v)
	}
	for _, b := range fixture.PlanInvalid {
		if _, e := DecodePlanView(b); e == nil {
			t.Fatal("invalid fixture accepted")
		}
	}
	valid := `{"kind":"goquent.plan_view","version":1}`
	for _, b := range []string{valid + valid, valid + " trailing", strings.Repeat(" ", 1<<20) + valid, `{"kind":"goquent.plan_view","version":1,"table_count":1.1}`, `{"kind":"goquent.plan_view","version":1,"table_count":9223372036854775808}`, `{"kind":"goquent.plan_view","version":1,"warnings":[{"code":"x","code":"y"}]}`, `{"kind":"goquent.plan_view","version":1,"warnings":[{"Message":"x"}]}`} {
		_, err := DecodePlanView([]byte(b))
		if err == nil {
			t.Fatal("invalid input accepted")
		}
		assertViewSafe(t, err.Error())
	}
	v, _ := (&QueryPlan{}).PublicView()
	if err := v.UnmarshalJSON([]byte("invalid")); err == nil || v.Kind != "" {
		t.Fatal("receiver not cleared")
	}
	// Decoding into the old diagnostic may succeed, but cannot reconstruct a seal.
	b, _ = (&QueryPlan{}).PublicViewJSONForTest(t)
	var old QueryPlan
	if err = json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	if old.execution != nil || old.SQL != "" || len(old.Params) != 0 {
		t.Fatal("view restored execution material")
	}
}
func (p *QueryPlan) PublicViewJSONForTest(t *testing.T) ([]byte, error) {
	t.Helper()
	v, e := p.PublicView()
	if e != nil {
		t.Fatal(e)
	}
	return v.ToJSON()
}

func TestPublicPlanViewPrivateSealDispatch(t *testing.T) {
	for _, pg := range []bool{false, true} {
		q, c, spy, m := bindingFixture(t, pg)
		data := map[string]any{"score": int32(3)}
		p, err := q.PlanUpdate(nil, data)
		if err != nil {
			t.Fatal(err)
		}
		e := p.execution
		sqlText := e.sql
		args := append([]any(nil), e.args...)
		inspection, err := bindingInspection(e.inspection)
		if err != nil {
			t.Fatal(err)
		}
		original := *p
		v, err := p.PublicView()
		if err != nil {
			t.Fatal(err)
		}
		v.Operation = viewCanaries[0]
		v.Warnings = append(v.Warnings, WarningView{Code: viewCanaries[1]})
		exercisePlanView(t, v)
		after, err := bindingInspection(e.inspection)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(original, *p) || !bytes.Equal(inspection, after) || spy.calls != [6]int{} || e.used.Load() {
			t.Fatal("projection changed private state")
		}
		m.ExpectExec(regexp.QuoteMeta(sqlText)).WillReturnResult(sqlmock.NewResult(0, 1))
		if _, err = q.executeResult(p); err != nil {
			t.Fatal(err)
		}
		if spy.sql != sqlText || !reflect.DeepEqual(spy.args, args) {
			t.Fatal("typed dispatch changed")
		}
		if _, err = q.executeResult(p); err == nil {
			t.Fatal("one-use changed")
		}
		if err = m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		_ = c
	}
}

func TestPublicPlanViewSecretDispatchAndRefusal(t *testing.T) {
	for _, pg := range []bool{false, true} {
		db, m, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		spy := &bindingSpy{executor: db}
		q := NewWithSettings(spy, "users", testDialect(pg), Settings{})
		p, err := q.PlanInsert(nil, map[string]any{"email": viewCanaries[0], "token": viewCanaries[1], "password": viewCanaries[2], "person": viewCanaries[3], "n": int16(19)})
		if err != nil {
			t.Fatal(err)
		}
		originalArgs := append([]any(nil), p.execution.args...)
		originalSQL := p.execution.sql
		v, err := p.PublicView()
		if err != nil {
			t.Fatal(err)
		}
		exercisePlanView(t, v)
		m.ExpectExec(regexp.QuoteMeta(originalSQL)).WillReturnResult(sqlmock.NewResult(1, 1))
		if _, err = q.executeResult(p); err != nil {
			t.Fatal(err)
		}
		if spy.sql != originalSQL || !reflect.DeepEqual(spy.args, originalArgs) {
			t.Fatal("canary values or types changed")
		}
		if err = m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		spy.calls = [6]int{}
		blocked, err := q.PlanDelete(nil)
		if err != nil {
			t.Fatal(err)
		}
		v, err = blocked.PublicView()
		if err != nil {
			t.Fatal(err)
		}
		v.Blocked = false
		v.Risk = "low"
		exercisePlanView(t, v)
		if _, err = q.executeResult(blocked); err == nil || spy.calls != [6]int{} {
			t.Fatal("view bypassed private gate")
		}
	}
}
