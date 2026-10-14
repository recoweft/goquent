package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

func TestTypedIdentityAndTracking(t *testing.T) {
	dir := t.TempDir()
	fixtureModule(t, dir)
	source := `package fixture
import (
 "context"
 sqlalias "database/sql"
 g "github.com/recoweft/goquent/orm"
 qalias "github.com/recoweft/goquent/orm/query"
)
type unrelated struct{}
func (unrelated) Get(any) {}
func (unrelated) GetErr(any) error {return nil}
func invoke(f func(any)error){_ = f(nil)}
func (unrelated) Update(any) {}
func (u unrelated) Table(string) unrelated {return u}
func (unrelated) Exec(string) {}
func helper(q *qalias.Query) *qalias.Query { return q.Select("id").Limit(3) }
func nested(q *qalias.Query) *qalias.Query { return helper(q) }
func unknown(q *qalias.Query) *qalias.Query { for i:=0;i<2;i++ {q.Limit(i)}; return q }
func identity(db *g.DB, raw *sqlalias.DB, executor g.Executor, flag bool, dynamic string, scope g.Scope) {
 var rows []map[string]any
 unrelated{}.Table("users").Update(nil) // negative
 unrelated{}.Get(nil) // negative
 _ = g.UpdateColumns("name") // negative
 _ = g.InsertHook("hook",1) // negative
 g.SelectOperationBy[int](context.Background(),db,g.OperationSpec{},g.OperationOptions{}) // generic
 unrelated{}.Exec("SELECT * FROM secret") // negative
 db.Table("users").Update(nil) // update
 db.Table("users").Get(&rows) // read
 a:=db.Table("users"); b:=a; b.Select("id").Limit(3)
 a.Get(&rows) // alias_clean
 c:=db.Table("other")
 c.Get(&rows) // independent
 { a:=unrelated{};a.Get(nil) } // negative
 a.Get(&rows) // shadow_clean
 if flag {c.Select("id").Limit(2)}
 c.Get(&rows) // branch
 a.Get(&rows) // branch_unaffected
 helper(db.Table("users")).Get(&rows) // helper_clean
 nested(db.Table("users")).Get(&rows) // nested
 unknown(db.Table("users")).Get(&rows) // loop
 g.ApplyScopes(db.Table("users"),g.ComposeScopes(g.TenantScope(7),g.RequireTenantScope("users"))).Get(&rows) // guard
 g.ApplyScopes(db.Table("users"),scope).Get(&rows) // opaque
 g.ApplyScopes(db.Table("users"),g.TenantScope(7)).Select("id").Limit(3).Get(&rows) // tenant_clean
 m:=db.Table("users").Get; m(&rows) // method_value
 (*qalias.Query).Get(db.Table("users"),&rows) // method_expression
 n:=db.Table("users").Get; if flag {n=unrelated{}.GetErr};n(&rows) // branch_method
 invoke(db.Table("users").Get) // escaped_method
 g.SelectAll[int](context.Background(),db,"SELECT id FROM users") // generic
 raw.Exec("DELETE FROM users") // raw
 raw.Exec(dynamic) // dynamic
 raw.Exec("SELECT id FROM users WHERE id = ?",dynamic) // raw_binding
 executor.ExecContext(context.Background(),"DELETE FROM users") // raw
 db.Table(dynamic).Where("id",1).Delete() // dynamic_table
 db.Table("users").Where("id",dynamic).Get(&rows) // dynamic_binding
 d:=db.Table("users").Select("id").Limit(1)
 d.WhereGroup(func(x *qalias.Query){x.Where("id",1)})
 d.Get(&rows) // callback
 d= db.Table("new")
 d.Get(&rows) // reassignment
}
`
	path := filepath.Join(dir, "fixture.go")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	byLine := map[int][]Finding{}
	for _, f := range report.Findings {
		if f.Location != nil {
			byLine[f.Location.Line] = append(byLine[f.Location.Line], f)
		}
	}
	for i, line := range strings.Split(source, "\n") {
		_, label, ok := strings.Cut(line, "// ")
		if !ok {
			continue
		}
		fs := byLine[i+1]
		switch label {
		case "negative", "alias_clean", "shadow_clean", "helper_clean", "tenant_clean", "branch_unaffected":
			if len(fs) != 0 {
				t.Errorf("%s: unexpected findings: %d", label, len(fs))
			}
		case "update":
			if !hasFinding(fs, query.WarningUpdateWithoutWhere) {
				t.Errorf("missing update diagnostic")
			}
		case "read", "independent", "reassignment", "method_value", "method_expression":
			if !hasFinding(fs, query.WarningLimitMissing) {
				t.Errorf("%s: missing limit diagnostic", label)
			}
		case "raw":
			if !hasFinding(fs, query.WarningRawSQLUsed) {
				t.Error("raw SQL missing")
			}
		case "dynamic":
			if !hasFinding(fs, query.WarningStaticReviewUnsupported) {
				t.Error("dynamic SQL missing")
			}
		default:
			if !hasFinding(fs, query.WarningStaticReviewPartial) {
				t.Errorf("%s: missing partial marker", label)
			}
		}
	}
	for _, f := range byLine[1] {
		if f.Code == query.WarningStaticReviewUnsupported {
			t.Fatal("fixture must type check")
		}
	}
}

func TestLegacyTypeFailureAndCanceledLoad(t *testing.T) {
	for _, source := range []string{"package p\nfunc run(db any){db.Table(\"users\").Update(nil)}", "package p\nfunc run(){ unknown_CANARY_COMPILER_928() }"} {
		dir := t.TempDir()
		fixtureModule(t, dir)
		path := filepath.Join(dir, "source.go")
		os.WriteFile(path, []byte(source), 0600)
		report, err := Run(Options{Paths: []string{path}})
		if err != nil {
			t.Fatal(err)
		}
		if !HasFindingsAtOrAbovePrecision(report, query.AnalysisUnsupported) {
			t.Fatal("type failure silently omitted")
		}
		for _, f := range report.Findings {
			if f.AnalysisPrecision == query.AnalysisPrecise {
				t.Fatal("invalid source promoted")
			}
		}
		view, _ := report.PublicView()
		if strings.Contains(view.String(), "CANARY") || strings.Contains(view.String(), dir) {
			t.Fatal("loader output escaped")
		}
	}
	dir := t.TempDir()
	fixtureModule(t, dir)
	path := filepath.Join(dir, "source.go")
	os.WriteFile(path, []byte("package p\n"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := Run(Options{Paths: []string{path}, Context: ctx})
	if err != nil {
		t.Fatal(err)
	}
	if !HasFindingsAtOrAbovePrecision(report, query.AnalysisUnsupported) {
		t.Fatal("cancellation lost")
	}
	report, err = Run(Options{Paths: []string{path}})
	if err != nil || len(report.Findings) != 0 {
		t.Fatal("separate Run retained failed type state")
	}
}

func TestPartialTypesKeepKnownOperationsAndVariants(t *testing.T) {
	dir := t.TempDir()
	fixtureModule(t, dir)
	path := filepath.Join(dir, "source.go")
	os.WriteFile(path, []byte(`package fixture
import (g "github.com/recoweft/goquent/orm"; q "github.com/recoweft/goquent/orm/query")
func f(db *g.DB) { db.Table("users").Update(nil); q.New(nil); missing_CANARY_991() }
`), 0600)
	report, _ := Run(Options{Paths: []string{path}})
	if !hasFinding(report.Findings, query.WarningUpdateWithoutWhere) || !hasFinding(report.Findings, query.WarningStaticReviewUnsupported) {
		t.Fatal("partial load discarded known operation")
	}
	for _, f := range report.Findings {
		if f.AnalysisPrecision == query.AnalysisPrecise {
			t.Fatal("partial types became precise")
		}
	}
	// A different input at the same path must not reuse the old type universe.
	os.WriteFile(path, []byte("package fixture\n"), 0600)
	testPath := filepath.Join(dir, "source_test.go")
	os.WriteFile(testPath, []byte(`package fixture_test
import ("testing"; g "github.com/recoweft/goquent/orm")
func TestFixture(t *testing.T) { var db *g.DB; db.Table("users").Delete() }
`), 0600)
	report, err := Run(Options{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFinding(report.Findings, query.WarningDeleteWithoutWhere) || hasFinding(report.Findings, query.WarningStaticReviewUnsupported) {
		t.Fatal("external test variant not loaded")
	}
}

func TestTypeInputBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.go")
	if err := os.WriteFile(path, []byte("package p\n//"+strings.Repeat("x", maxTypeFileBytes)), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if !HasFindingsAtOrAbovePrecision(report, query.AnalysisUnsupported) {
		t.Fatal("input budget silently omitted file")
	}
}
