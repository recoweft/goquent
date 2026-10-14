package review

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/recoweft/goquent/orm/query"
)

type queryState struct {
	calls   []chainCall
	partial bool
}
type trackedValue struct {
	q          *queryState
	fn         *types.Func
	recv       *queryState
	expression bool
	uncertain  bool
	scope      ast.Expr
}
type goTracker struct {
	ctx         reviewContext
	path        string
	tf          *typedFile
	env         map[types.Object]trackedValue
	findings    []Finding
	uncertain   bool
	helperDepth int
	steps       int
}

func reviewTypedGo(ctx reviewContext, path string, tf *typedFile) ([]Finding, error) {
	t := &goTracker{ctx: ctx, path: path, tf: tf, env: map[types.Object]trackedValue{}}
	if tf == nil || tf.file == nil {
		return []Finding{typeUnavailable(&query.SourceLocation{File: path, Line: 1})}, nil
	}
	if tf.incomplete {
		t.findings = append(t.findings, typeUnavailable(&query.SourceLocation{File: path, Line: 1}))
	}
	for _, d := range tf.file.Decls {
		switch x := d.(type) {
		case *ast.FuncDecl:
			t.env = map[types.Object]trackedValue{}
			if x.Body != nil {
				t.block(x.Body)
			}
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				if v, ok := spec.(*ast.ValueSpec); ok {
					for _, e := range v.Values {
						t.eval(e)
					}
				}
			}
		}
	}
	fs, err := applyFileSuppressions(ctx, path, t.findings)
	if err != nil {
		return t.findings, err
	}
	return fs, nil
}
func typeUnavailable(loc *query.SourceLocation) Finding {
	return staticFinding(query.WarningStaticReviewUnsupported, query.RiskMedium, "type loading incomplete; database operations may be missing", "provide a buildable package and review unsupported inputs", loc, query.AnalysisUnsupported)
}
func (t *goTracker) partial(loc *query.SourceLocation, reason string) {
	f := staticReviewPartial(loc)
	f.Message = reason
	t.findings = append(t.findings, f)
}
func (t *goTracker) loc(n ast.Node) *query.SourceLocation {
	return sourceLocation(t.tf.fset, t.path, n.Pos())
}
func (t *goTracker) block(b *ast.BlockStmt) {
	if b != nil {
		for _, s := range b.List {
			t.stmt(s)
		}
	}
}
func (t *goTracker) stmt(s ast.Stmt) {
	t.steps++
	if t.steps > 100000 {
		if t.steps == 100001 {
			t.findings = append(t.findings, typeUnavailable(t.loc(s)))
		}
		return
	}
	switch x := s.(type) {
	case *ast.BlockStmt:
		t.block(x)
	case *ast.ExprStmt:
		t.eval(x.X)
	case *ast.AssignStmt:
		values := make([]trackedValue, len(x.Rhs))
		for i, e := range x.Rhs {
			values[i] = t.eval(e)
		}
		for i, e := range x.Lhs {
			if id, ok := e.(*ast.Ident); ok {
				v := trackedValue{}
				if i < len(values) {
					v = values[i]
				}
				if o := t.tf.object(id); o != nil {
					t.env[o] = v
				}
			} else {
				t.invalidate()
			}
		}
	case *ast.DeclStmt:
		if d, ok := x.Decl.(*ast.GenDecl); ok {
			for _, sp := range d.Specs {
				if v, ok := sp.(*ast.ValueSpec); ok {
					vals := make([]trackedValue, len(v.Values))
					for i, e := range v.Values {
						vals[i] = t.eval(e)
					}
					for i, id := range v.Names {
						if i < len(vals) {
							t.env[t.tf.object(id)] = vals[i]
						}
					}
				}
			}
		}
	case *ast.ReturnStmt:
		for _, e := range x.Results {
			v := t.eval(e)
			if v.q != nil {
				v.q.partial = true
			}
			if v.recv != nil {
				v.recv.partial = true
			}
		}
	case *ast.IfStmt:
		if x.Init != nil {
			t.stmt(x.Init)
		}
		t.eval(x.Cond)
		original := t.env
		left := cloneEnvironment(original)
		t.env = left
		t.withUncertainty(func() { t.block(x.Body) })
		right := cloneEnvironment(original)
		t.env = right
		t.withUncertainty(func() {
			if x.Else != nil {
				t.stmt(x.Else)
			}
		})
		t.env = joinEnvironments(original, left, right)
	case *ast.ForStmt:
		if x.Init != nil {
			t.stmt(x.Init)
		}
		t.eval(x.Cond)
		t.invalidate()
		t.withUncertainty(func() {
			t.block(x.Body)
			if x.Post != nil {
				t.stmt(x.Post)
			}
		})
		t.invalidate()
	case *ast.RangeStmt:
		t.eval(x.X)
		t.invalidate()
		t.withUncertainty(func() { t.block(x.Body) })
		t.invalidate()
	case *ast.SwitchStmt:
		if x.Init != nil {
			t.stmt(x.Init)
		}
		t.eval(x.Tag)
		t.invalidate()
		t.withUncertainty(func() { t.block(x.Body) })
		t.invalidate()
	case *ast.TypeSwitchStmt:
		t.invalidate()
		t.withUncertainty(func() {
			if x.Init != nil {
				t.stmt(x.Init)
			}
			t.stmt(x.Assign)
			t.block(x.Body)
		})
		t.invalidate()
	case *ast.CaseClause:
		for _, e := range x.List {
			t.eval(e)
		}
		for _, s := range x.Body {
			t.stmt(s)
		}
	case *ast.SelectStmt:
		t.invalidate()
		t.withUncertainty(func() { t.block(x.Body) })
		t.invalidate()
	case *ast.CommClause:
		if x.Comm != nil {
			t.stmt(x.Comm)
		}
		for _, s := range x.Body {
			t.stmt(s)
		}
	case *ast.GoStmt:
		t.invalidate()
		t.withUncertainty(func() { t.eval(x.Call) })
	case *ast.DeferStmt:
		t.invalidate()
		t.withUncertainty(func() { t.eval(x.Call) })
	case *ast.LabeledStmt:
		t.stmt(x.Stmt)
	case *ast.SendStmt:
		t.eval(x.Chan)
		t.eval(x.Value)
		t.invalidate()
	case *ast.BranchStmt:
		t.invalidate()
	}
}
func (t *goTracker) invalidate() {
	for _, v := range t.env {
		if v.q != nil {
			v.q.partial = true
		}
		if v.recv != nil {
			v.recv.partial = true
		}
	}
}
func (t *goTracker) withUncertainty(f func()) {
	old := t.uncertain
	t.uncertain = true
	f()
	t.uncertain = old
}
func cloneEnvironment(src map[types.Object]trackedValue) map[types.Object]trackedValue {
	dst := map[types.Object]trackedValue{}
	states := map[*queryState]*queryState{}
	clone := func(q *queryState) *queryState {
		if q == nil {
			return nil
		}
		if states[q] == nil {
			states[q] = &queryState{calls: append([]chainCall(nil), q.calls...), partial: q.partial}
		}
		return states[q]
	}
	for o, v := range src {
		v.q = clone(v.q)
		v.recv = clone(v.recv)
		dst[o] = v
	}
	return dst
}
func sameState(a, b *queryState) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.partial != b.partial || len(a.calls) != len(b.calls) {
		return false
	}
	for i := range a.calls {
		if a.calls[i] != b.calls[i] {
			return false
		}
	}
	return true
}
func joinEnvironments(base, a, b map[types.Object]trackedValue) map[types.Object]trackedValue {
	out := cloneEnvironment(base)
	for o, v := range out {
		av, bv := a[o], b[o]
		if !sameState(av.q, bv.q) || !sameState(v.q, av.q) {
			if v.q == nil {
				v.q = &queryState{partial: true}
			}
			v.q.partial = true
		}
		if av.fn != bv.fn || !sameState(av.recv, bv.recv) {
			for _, candidate := range []trackedValue{av, bv, v} {
				if databaseFunction(candidate.fn) {
					v.fn = candidate.fn
					v.recv = candidate.recv
					v.expression = candidate.expression
					break
				}
			}
			v.uncertain = true
			if v.recv != nil {
				v.recv.partial = true
			}
		}
		if av.scope != bv.scope {
			v.scope = nil
		}
		out[o] = v
	}
	return out
}

func (t *goTracker) eval(e ast.Expr) trackedValue {
	if e == nil {
		return trackedValue{}
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return t.eval(x.X)
	case *ast.Ident:
		o := t.tf.object(x)
		if v, ok := t.env[o]; ok {
			return v
		}
		if fn, ok := o.(*types.Func); ok {
			return trackedValue{fn: fn.Origin()}
		}
		if o != nil && queryType(o.Type()) {
			v := trackedValue{q: &queryState{partial: true}}
			t.env[o] = v
			return v
		}
	case *ast.SelectorExpr:
		fn := t.tf.function(x)
		if fn != nil {
			v := trackedValue{fn: fn}
			if t.tf.info != nil {
				if selection := t.tf.info.Selections[x]; selection != nil {
					v.expression = selection.Kind() == types.MethodExpr
					if !v.expression {
						v.recv = t.eval(x.X).q
					}
				}
			}
			return v
		}
		t.eval(x.X)
	case *ast.IndexExpr:
		t.eval(x.Index)
		return t.eval(x.X)
	case *ast.StarExpr:
		return t.eval(x.X)
	case *ast.IndexListExpr:
		return t.eval(x.X)
	case *ast.CallExpr:
		return t.call(x)
	case *ast.FuncLit:
		t.invalidate()
		old := t.env
		t.env = cloneEnvironment(old)
		t.withUncertainty(func() { t.block(x.Body) })
		t.env = old
	case *ast.UnaryExpr:
		v := t.eval(x.X)
		if v.q != nil {
			v.q.partial = true
		}
		return v
	case *ast.BinaryExpr:
		t.eval(x.X)
		t.eval(x.Y)
	case *ast.CompositeLit:
		for _, e := range x.Elts {
			v := t.eval(e)
			if v.q != nil {
				v.q.partial = true
			}
			if v.recv != nil {
				v.recv.partial = true
			}
		}
	case *ast.KeyValueExpr:
		t.eval(x.Key)
		return t.eval(x.Value)
	case *ast.SliceExpr:
		t.eval(x.X)
		t.eval(x.Low)
		t.eval(x.High)
		t.eval(x.Max)
	case *ast.TypeAssertExpr:
		t.eval(x.X)
	}
	return trackedValue{}
}
func (t *goTracker) call(c *ast.CallExpr) trackedValue {
	value := t.eval(c.Fun)
	args := make([]trackedValue, len(c.Args))
	for i, e := range c.Args {
		args[i] = t.eval(e)
	}
	fn := value.fn
	if value.uncertain {
		old := t.uncertain
		t.uncertain = true
		defer func() { t.uncertain = old }()
	}
	loc := t.loc(c.Fun)
	if s, ok := unparen(c.Fun).(*ast.SelectorExpr); ok {
		loc = t.loc(s.Sel)
	}
	if fn == nil {
		t.escapeCallables(args, loc)
		if t.tf.incomplete {
			if s, ok := unparen(c.Fun).(*ast.SelectorExpr); ok && (isGoquentTerminal(s.Sel.Name) || isRawSQLMethod(s.Sel.Name) || isRawBuilderMethod(s.Sel.Name)) {
				t.findings = append(t.findings, typeUnavailable(loc))
			}
		}
		for _, v := range args {
			if v.q != nil {
				v.q.partial = true
			}
			if v.recv != nil {
				v.recv.partial = true
			}
		}
		return trackedValue{}
	}
	path := ""
	if fn.Pkg() != nil {
		path = fn.Pkg().Path()
	}
	name := fn.Name()
	recv := declaredReceiverName(fn)
	offset := 0
	q := value.recv
	if value.expression {
		offset = 1
		if len(args) > 0 {
			q = args[0].q
		}
	}
	callArgs := c.Args
	if offset <= len(callArgs) {
		callArgs = callArgs[offset:]
	}
	normalized := &ast.CallExpr{Fun: c.Fun, Args: callArgs, Ellipsis: c.Ellipsis}
	if path == queryPath && recv == "" && name == "NewRawPlan" {
		t.raw(name, normalized, loc)
		return trackedValue{}
	}
	if (path == "database/sql" && (recv == "DB" || recv == "Tx" || recv == "Conn" || recv == "Stmt")) || (path == ormPath && (recv == "DB" || recv == "Tx" || recv == "Executor")) ||
		(path == queryPath && recv == "executor") ||
		(path == ormPath+"/migration" && (recv == "Executor" || recv == "StatusExecutor")) ||
		(path == ormPath+"/internal/querybridge" && recv == "Executor") {
		if isRawSQLMethod(name) || name == "QueryRowE" || name == "SelectMap" || name == "SelectMaps" || name == "Prepare" || name == "PrepareContext" {
			if recv == "Stmt" {
				t.partial(loc, "prepared statement SQL is not tracked")
			} else {
				t.raw(name, normalized, loc)
			}
			return trackedValue{}
		}
	}
	if (path == ormPath && (recv == "DB" || recv == "Tx") && (name == "Table" || name == "TablePath" || name == "Model")) || (path == queryPath && recv == "" && (name == "New" || name == "NewWithSettings")) {
		root := chainCall{Method: name, Call: normalized}
		if name == "New" || name == "NewWithSettings" {
			if len(callArgs) < 2 {
				t.findings = append(t.findings, typeUnavailable(loc))
				return trackedValue{q: &queryState{partial: true}}
			}
			root = chainCall{Method: "Table", Call: &ast.CallExpr{Args: callArgs[1:2]}}
		}
		return trackedValue{q: &queryState{calls: []chainCall{root}, partial: name == "Model"}}
	}
	if path == queryPath && recv == "Query" {
		if q == nil {
			q = &queryState{partial: true}
		}
		if isQueryOperation(name) {
			t.terminal(name, q, normalized, loc)
			return trackedValue{}
		}
		if isRawBuilderMethod(name) {
			sel := &ast.SelectorExpr{Sel: &ast.Ident{Name: name}}
			t.findings = append(t.findings, reviewRawBuilderCall(sel, normalized, loc)...)
		}
		sig := fn.Type().(*types.Signature)
		if sig.Results().Len() > 0 && queryType(sig.Results().At(0).Type()) {
			if !simpleModifier(name, normalized) {
				q.partial = true
			}
			if t.uncertain {
				q.partial = true
			}
			q.calls = append([]chainCall{{Method: name, Call: normalized}}, q.calls...)
			return trackedValue{q: q}
		}
	}
	if path == ormPath && recv == "" {
		switch name {
		case "ComposeScopes", "TenantScope", "RequirePredicates", "RequireTenantScope":
			return trackedValue{scope: c}
		case "ApplyScopes":
			if len(args) > 0 {
				q = args[0].q
			}
			if q == nil {
				q = &queryState{partial: true}
			}
			for i := 1; i < len(args); i++ {
				if id, ok := c.Args[i].(*ast.Ident); ok && id.Name == "nil" {
					continue
				}
				t.scope(q, args[i].scope)
			}
			if c.Ellipsis.IsValid() {
				q.partial = true
			}
			return trackedValue{q: q}
		}
		if genericOperation(name) {
			for _, v := range args {
				if v.q != nil {
					v.q.partial = true
				}
			}
			t.partial(loc, "generic or composite database operation requires runtime plan evidence")
			return trackedValue{}
		}
	}
	if t.helperDepth == 0 && fn.Pkg() == t.tf.pkg {
		if result, ok := t.helper(fn, args); ok {
			return result
		}
	}
	t.escapeCallables(args, loc)
	for _, v := range args {
		if v.q != nil {
			v.q.partial = true
		}
	}
	if q != nil {
		q.partial = true
	}
	if t.tf.info != nil && queryType(t.tf.info.TypeOf(c)) {
		return trackedValue{q: &queryState{partial: true}}
	}
	return trackedValue{}
}
func databaseFunction(fn *types.Func) bool {
	if fn == nil || fn.Pkg() == nil {
		return false
	}
	path, name, recv := fn.Pkg().Path(), fn.Name(), declaredReceiverName(fn)
	if path == queryPath {
		return (recv == "Query" && isQueryOperation(name)) || name == "NewRawPlan"
	}
	if path == ormPath {
		return genericOperation(name) || isRawSQLMethod(name) || name == "QueryRowE"
	}
	return path == "database/sql" && (isRawSQLMethod(name) || name == "Prepare" || name == "PrepareContext")
}
func (t *goTracker) escapeCallables(args []trackedValue, loc *query.SourceLocation) {
	for _, v := range args {
		if v.recv != nil {
			v.recv.partial = true
		}
		if databaseFunction(v.fn) {
			t.partial(loc, "database callable escapes local tracking")
		}
	}
}

func literalInput(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return x.Name == "nil" || x.Name == "true" || x.Name == "false"
	case *ast.CompositeLit:
		for _, e := range x.Elts {
			if !literalInput(e) {
				return false
			}
		}
		return true
	case *ast.KeyValueExpr:
		return literalInput(x.Key) && literalInput(x.Value)
	case *ast.ParenExpr:
		return literalInput(x.X)
	default:
		return false
	}
}

func simpleModifier(name string, c *ast.CallExpr) bool {
	switch name {
	case "Select", "Table", "TablePath", "Join", "LeftJoin", "RightJoin", "CrossJoin", "OrderBy", "GroupBy":
		for _, a := range c.Args {
			if _, ok := stringLiteralValue(a); !ok {
				return false
			}
		}
		return !c.Ellipsis.IsValid()
	case "Where", "OrWhere", "WhereNull", "WhereNotNull":
		for _, a := range c.Args {
			if _, ok := a.(*ast.BasicLit); !ok {
				return false
			}
		}
		return !c.Ellipsis.IsValid()
	case "Limit", "Offset":
		if len(c.Args) != 1 {
			return false
		}
		_, ok := c.Args[0].(*ast.BasicLit)
		return ok
	case "WithContext", "RequireApproval", "AccessReason":
		return true
	default:
		return false
	}
}
func isQueryOperation(name string) bool {
	if isGoquentTerminal(name) {
		return true
	}
	switch name {
	case "Count", "Insert", "InsertBatch", "InsertGetId", "InsertOrIgnore", "InsertUsing", "Upsert", "UpdateOrInsert", "PlanInsert", "PlanInsertBatch", "Increment", "Decrement", "UpdateReturning", "DeleteReturning":
		return true
	}
	return false
}

// Only declared operation entry points belong here. Option and hook factories
// such as UpdateColumns and InsertHook do not themselves execute an operation.
func genericOperation(name string) bool {
	switch name {
	case "SelectOne", "SelectAll", "SelectStruct", "SelectStructs",
		"Insert", "InsertReturning", "InsertMany", "InsertManyReturning",
		"Upsert", "UpsertReturning", "UpsertMany", "UpsertManyReturning",
		"Update", "UpdateReturning", "InsertOnceReturning",
		"PlanInsert", "PlanUpdate", "PlanUpsert", "PlanInsertMany", "PlanUpsertMany",
		"PlanSelectBy", "PlanUpdateBy", "PlanDeleteBy", "SelectOneBy", "SelectAllBy",
		"UpdateBy", "UpdateByReturning", "UpdateByReturningWithOptions", "DeleteBy",
		"SelectOperationBy", "UpdateOperationBy", "UpdateOperationReturningBy",
		"ReplaceNestedCollection", "ReplaceNestedCollectionTx", "RunIdempotentCommand", "RunTransactionWithHooks":
		return true
	default:
		return false
	}
}

func (t *goTracker) raw(name string, c *ast.CallExpr, loc *query.SourceLocation) {
	i := rawSQLArgIndex(name)
	if name == "QueryRowE" || name == "SelectMap" || name == "SelectMaps" || name == "PrepareContext" {
		i = 1
	}
	if name == "Prepare" || name == "NewRawPlan" {
		i = 0
	}
	if i < 0 || i >= len(c.Args) {
		t.findings = append(t.findings, typeUnavailable(loc))
		return
	}
	sql, ok := stringLiteralValue(c.Args[i])
	if !ok {
		f := typeUnavailable(loc)
		f.Message = "dynamic SQL cannot be reconstructed"
		t.findings = append(t.findings, f)
		return
	}
	incomplete := t.tf.incomplete || t.uncertain || c.Ellipsis.IsValid()
	for _, arg := range c.Args[i+1:] {
		if !literalInput(arg) {
			incomplete = true
		}
	}
	fs := findingsFromPlan(query.NewRawPlan(sql), query.AnalysisPrecise, loc)
	if incomplete {
		for i := range fs {
			fs[i].AnalysisPrecision = query.AnalysisPartial
		}
	}
	t.findings = append(t.findings, fs...)
	if incomplete {
		t.partial(loc, "raw operation has incomplete type, binding or control flow evidence")
	}
}
func (t *goTracker) terminal(name string, q *queryState, c *ast.CallExpr, loc *query.SourceLocation) {
	calls := append([]chainCall{{Method: name, Call: c}}, q.calls...)
	partial := q.partial || t.uncertain || t.tf.incomplete
	if name == "Update" || name == "PlanUpdate" {
		i := 0
		if name == "PlanUpdate" {
			i = 1
		}
		if i >= len(c.Args) || !literalInput(c.Args[i]) {
			partial = true
		}
	}
	if _, ok := tableFromChain(calls); !ok {
		partial = true
	}
	if !isGoquentTerminal(name) {
		partial = true
	}
	if p, ok := staticPlanFromChain(t.ctx, name, calls); ok {
		fs := warningsToFindings(query.DefaultRiskEngine.CheckQuery(p).Warnings, query.AnalysisPrecise, loc)
		fs = append(fs, manifestPolicyFindings(t.ctx, p, loc)...)
		if partial {
			for i := range fs {
				if precisionRank(fs[i].AnalysisPrecision) < 1 {
					fs[i].AnalysisPrecision = query.AnalysisPartial
				}
			}
		}
		t.findings = append(t.findings, fs...)
	}
	if _, ok := tableFromChain(calls); !ok && (name == "Update" || name == "Delete" || name == "PlanUpdate" || name == "PlanDelete") {
		code := query.WarningBulkUpdateDetected
		if name == "Delete" || name == "PlanDelete" {
			code = query.WarningBulkDeleteDetected
		}
		level := query.RiskMedium
		hasRoot, hasPredicate := false, false
		for _, c := range calls {
			if c.Method == "Table" || c.Method == "TablePath" || c.Method == "Model" {
				hasRoot = true
			}
			if isPredicateMethod(c.Method) {
				hasPredicate = true
			}
		}
		if hasRoot && !hasPredicate {
			level = query.RiskBlocked
			if name == "Delete" || name == "PlanDelete" {
				code = query.WarningDeleteWithoutWhere
			} else {
				code = query.WarningUpdateWithoutWhere
			}
		}
		f := staticFinding(code, level, "write scope is unknown", "provide query structure and runtime evidence", loc, query.AnalysisPartial)
		f.Evidence = []query.Evidence{{Key: "write_scope", Value: query.AnalyzeWriteScope(nil)}}
		t.findings = append(t.findings, f)
	}
	if _, ok := tableFromChain(calls); !ok && (name == "Get" || name == "GetMaps" || name == "First" || name == "FirstMap" || name == "Plan") {
		hasRoot := false
		for _, c := range calls {
			if c.Method == "Table" || c.Method == "TablePath" || c.Method == "Model" {
				hasRoot = true
			}
		}
		if hasRoot {
			if len(selectedColumnsFromChain(calls)) == 0 {
				t.findings = append(t.findings, staticFinding(query.WarningSelectStarUsed, query.RiskMedium, "SELECT projection is not explicit", "select explicit columns", loc, query.AnalysisPartial))
			}
			if _, limited := limitFromChain(calls); !limited {
				t.findings = append(t.findings, staticFinding(query.WarningLimitMissing, query.RiskMedium, "SELECT has no known positive limit", "add a bounded limit", loc, query.AnalysisPartial))
			}
		}
	}
	if partial {
		t.partial(loc, "query structure, bindings or control flow are incomplete")
	}
}
func (t *goTracker) scope(q *queryState, e ast.Expr) {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		q.partial = true
		return
	}
	fn := t.tf.function(c.Fun)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != ormPath {
		q.partial = true
		return
	}
	switch fn.Name() {
	case "ComposeScopes":
		for _, arg := range c.Args {
			t.scope(q, t.eval(arg).scope)
		}
	case "TenantScope":
		col := ast.Expr(&ast.BasicLit{Kind: token.STRING, Value: `"tenant_id"`})
		if len(c.Args) > 1 {
			col = c.Args[1]
			if text, ok := stringLiteralValue(col); ok {
				if text = strings.TrimSpace(text); text != "" {
					col = &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(text)}
				} else {
					col = &ast.BasicLit{Kind: token.STRING, Value: `"tenant_id"`}
				}
			}
		}
		if len(c.Args) == 0 {
			q.partial = true
			return
		}
		call := &ast.CallExpr{Args: []ast.Expr{col, c.Args[0]}}
		q.calls = append([]chainCall{{Method: "Where", Call: call}}, q.calls...)
		if !simpleModifier("Where", call) {
			q.partial = true
		}
	case "RequirePredicates", "RequireTenantScope":
		q.partial = true // A guard is not predicate or tenant-value evidence.
	default:
		q.partial = true
	}
	if c.Ellipsis.IsValid() {
		q.partial = true
	}
}

// helper expands exactly one same-package straight-line Query -> Query helper.
func (t *goTracker) helper(fn *types.Func, args []trackedValue) (trackedValue, bool) {
	decl := t.tf.helpers[fn]
	sig, _ := fn.Type().(*types.Signature)
	if decl == nil || decl.Body == nil || sig == nil || sig.Recv() != nil || sig.Params().Len() != 1 || sig.Results().Len() != 1 || !queryType(sig.Params().At(0).Type()) || !queryType(sig.Results().At(0).Type()) || len(args) != 1 || args[0].q == nil {
		return trackedValue{}, false
	}
	// Only modifier expressions and a final return are eligible. No calls to
	// helpers, closures, loops, escapes, assignment or multiple Query inputs.
	valid := true
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			f := t.tf.function(x.Fun)
			if f == nil || f.Pkg() == nil || f.Pkg().Path() != queryPath || declaredReceiverName(f) != "Query" || isQueryOperation(f.Name()) {
				valid = false
			}
		case *ast.Ident:
			if obj := t.tf.object(x); obj != nil && queryType(obj.Type()) && obj != sig.Params().At(0) {
				valid = false
			}
		case *ast.FuncLit:
			valid = false
			return false
		}
		return true
	})
	for i, s := range decl.Body.List {
		switch s.(type) {
		case *ast.ExprStmt:
		case *ast.ReturnStmt:
			if i != len(decl.Body.List)-1 {
				valid = false
			}
		default:
			valid = false
		}
	}
	if !valid {
		return trackedValue{}, false
	}
	old := t.env
	t.env = map[types.Object]trackedValue{sig.Params().At(0): args[0]}
	t.helperDepth++
	defer func() { t.env = old; t.helperDepth-- }()
	var result trackedValue
	for _, s := range decl.Body.List {
		if ret, ok := s.(*ast.ReturnStmt); ok {
			if len(ret.Results) == 1 {
				result = t.eval(ret.Results[0])
			}
		} else {
			t.stmt(s)
		}
	}
	if result.q == nil {
		args[0].q.partial = true
		result = args[0]
	}
	return result, true
}
