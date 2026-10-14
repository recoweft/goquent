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

func TestTypedCallableEscapeBoundaries(t *testing.T) {
	dir := t.TempDir()
	fixtureModule(t, dir)
	source := `package fixture
import ( sqlalias "database/sql"; g "github.com/recoweft/goquent/orm" )
type execFn = func(string, ...any) (sqlalias.Result,error)
type holder struct { F execFn }
type unrelated struct{}
func (unrelated) Exec(string,...any)(sqlalias.Result,error){return nil,nil}
func take(execFn){}
func ret(db *sqlalias.DB) execFn { return db.Exec } // escape
func retQuery(db *sqlalias.DB) func(string,...any)(*sqlalias.Rows,error) { return db.Query } // escape
func retTerminal(db *g.DB) func(any)error { f:=db.Table("CANARY_ESCAPE_481").Get; alias:=f; return alias } // escape
func negative() execFn { return unrelated{}.Exec } // negative
func containers(db *sqlalias.DB, qdb *g.DB) {
 f:=db.Exec; alias:=f
 h:=holder{F:alias}; h.F("DELETE FROM CANARY_ESCAPE_481") // escape
 s:=[]execFn{db.Exec}; s[0]("DELETE FROM CANARY_ESCAPE_481") // escape
 m:=map[string]execFn{"x":db.Exec}; m["x"]("DELETE FROM CANARY_ESCAPE_481") // escape
 terminals:=[]func(any)error{qdb.Table("CANARY_ESCAPE_481").Get}; terminals[0](nil) // escape
 h.F=db.Exec // escape
 s[0]=alias // escape
 m["x"]=db.Exec // escape
 take(db.Exec) // escape
 ch:=make(chan execFn,1); ch<-db.Exec // escape
 maps:=[]any{qdb.SelectMap,qdb.SelectMaps} // escape
 _=maps
 n:=holder{F:unrelated{}.Exec}; n.F("anything") // negative
 ns:=[]execFn{unrelated{}.Exec}; ns[0]("anything") // negative
 nm:=map[string]execFn{"x":unrelated{}.Exec}; nm["x"]("anything") // negative
 db.Exec("DELETE FROM CANARY_ESCAPE_481") // direct
 local:=db.Exec; local("DELETE FROM CANARY_ESCAPE_481") // direct
 (*sqlalias.DB).Exec(db,"DELETE FROM CANARY_ESCAPE_481") // direct
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
		if f.Code == query.WarningStaticReviewUnsupported {
			t.Fatal("fixture must type check")
		}
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
		case "escape":
			if !hasFinding(fs, query.WarningStaticReviewPartial) {
				t.Errorf("line %d: escape marker missing", i+1)
			}
			for _, f := range fs {
				if f.AnalysisPrecision == query.AnalysisPrecise {
					t.Errorf("line %d: escape promoted", i+1)
				}
			}
		case "negative":
			if len(fs) != 0 {
				t.Errorf("line %d: unrelated callable diagnosed", i+1)
			}
		case "direct":
			if !hasFinding(fs, query.WarningRawSQLUsed) {
				t.Errorf("line %d: direct diagnostic missing", i+1)
			}
		}
	}
	view, _ := report.PublicView()
	if strings.Contains(view.String(), "CANARY_ESCAPE") || strings.Contains(view.String(), dir) {
		t.Fatal("escape source published")
	}
}

func TestTypedBranchCallableReceiverAndValidatedOperations(t *testing.T) {
	dir := t.TempDir()
	fixtureModule(t, dir)
	source := `package fixture
import ("context";"time"; g "github.com/recoweft/goquent/orm"; q "github.com/recoweft/goquent/orm/query")
func branch(db *g.DB,flag bool) {
 safe:=db.Table("first").Select("id").Limit(1)
 other:=db.Table("second")
 f:=safe.Get
 if flag {f=other.Get} else {f=other.Get}
 f(nil) // partial
 independent:=db.Table("third").Select("id").Limit(1)
 independent.Get(nil) // clean
 a:=db.Table("fourth"); b:=db.Table("fifth"); alias:=a
 if flag {a=b}
 a.Select("id").Limit(1)
 alias.Get(nil) // partial
 b.Get(nil) // partial
}
type unrelated struct{}
func (unrelated) Get(any)error{return nil}
func opaque(q *q.Query){}
func boundaries(db *g.DB, flag bool) {
 a:=db.Table("first").Select("id").Limit(1)
 f:=a.Get
 for flag {a.Limit(2)}
 f(nil) // partial
 b:=db.Table("second").Select("id").Limit(1)
 g:=b.Get
 _=func(){b.Limit(2)}
 g(nil) // partial
 c:=db.Table("third").Select("id").Limit(1)
 opaque(c)
 (*q.Query).Get(c,nil) // partial
 f2:=db.Table("fourth").Get
 for flag {f2=unrelated{}.Get}
 f2(nil) // partial
}
func validated(x *q.Query, ctx context.Context, c q.BindingCurrent, h *q.ValidatedPlan) {
 x.ValidateSelect(ctx,c,time.Time{}) // partial
 x.ValidateCount(ctx,c,time.Time{}) // partial
 x.ValidateInsert(ctx,c,time.Time{},nil) // partial
 x.ValidateInsertBatch(ctx,c,time.Time{},nil) // partial
 x.ValidateUpdate(ctx,c,time.Time{},nil) // partial
 x.ValidateDelete(ctx,c,time.Time{}) // partial
 x.ExecuteValidatedSelect(ctx,c,h,nil) // partial
 x.ExecuteValidatedCount(ctx,c,h) // partial
 x.ExecuteValidatedInsert(ctx,c,h,nil) // partial
 x.ExecuteValidatedInsertBatch(ctx,c,h,nil) // partial
 x.ExecuteValidatedUpdate(ctx,c,h,nil) // partial
 x.ExecuteValidatedDelete(ctx,c,h) // partial
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
		if f.Code == query.WarningStaticReviewUnsupported {
			t.Fatal("fixture must type check")
		}
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
		if label == "clean" {
			if len(fs) != 0 {
				t.Errorf("line %d: independent query changed", i+1)
			}
			continue
		}
		if !hasFinding(fs, query.WarningStaticReviewPartial) {
			t.Errorf("line %d: operation uncertainty lost", i+1)
		}
		for _, f := range fs {
			if f.AnalysisPrecision == query.AnalysisPrecise {
				t.Errorf("line %d: uncertainty promoted", i+1)
			}
		}
	}
}
