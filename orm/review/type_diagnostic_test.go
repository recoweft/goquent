package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/recoweft/goquent/orm/query"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"golang.org/x/tools/go/packages"
)

// diagnosticLoad is a test-only mirror; TestDiagnosticLoaderParity checks its body.
func diagnosticLoad(parent context.Context, paths []string, overlay map[string][]byte, capture *ciCapture, trace *loadDiagnostic) map[string]*typedFile {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, typeLoadTimeout)
	defer cancel()
	out := map[string]*typedFile{}
	groups := map[string]map[string]bool{}
	total, count := int64(0), 0
	for _, path := range paths {
		if filepath.Ext(path) != ".go" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, exists := out[abs]; exists {
			continue
		}
		tf := &typedFile{incomplete: true, fset: token.NewFileSet()}
		out[abs] = tf
		st, err := os.Stat(abs)
		count++
		if err != nil || count > maxTypeFiles || st.Size() > maxTypeFileBytes || total+st.Size() > maxTypeInputBytes || ctx.Err() != nil {
			continue
		}
		total += st.Size()
		var input any
		if b, ok := overlay[abs]; ok {
			input = b
		}
		tf.file, _ = parser.ParseFile(tf.fset, abs, input, parser.ParseComments)
		dir := filepath.Dir(abs)
		root := moduleRoot(dir)
		if root == "" {
			root = dir
		}
		if groups[root] == nil {
			groups[root] = map[string]bool{}
		}
		groups[root][dir] = true
	}
	roots := make([]string, 0, len(groups))
	for root := range groups {
		roots = append(roots, root)
	}
	sort.Strings(roots)
	var parseMu sync.Mutex
	parsedSources := map[string][]byte{}
	parsedBytes, parsedFiles := int64(0), 0
	parseBounded := func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
		parseMu.Lock()
		defer parseMu.Unlock()
		if ctx.Err() != nil {
			return nil, errors.New("type load canceled")
		}
		st, err := os.Stat(filename)
		if err != nil {
			return nil, errors.New("type input unavailable")
		}
		trace.parseCalls++
		parsedFiles++
		parsedBytes += st.Size()
		if st.Size() > maxTypeFileBytes || parsedBytes > maxTypeInputBytes || parsedFiles > maxTypeFiles {
			trace.budgetHits++
			return nil, errors.New("type input budget exceeded")
		}
		if b, ok := overlay[filename]; ok {
			src = b
		}
		if src == nil {
			src, err = os.ReadFile(filename)
			if err != nil {
				return nil, errors.New("type input unavailable")
			}
		}
		if len(src) > maxTypeFileBytes {
			trace.budgetHits++
			return nil, errors.New("type input budget exceeded")
		}
		if previous, seen := parsedSources[filename]; seen && !bytes.Equal(previous, src) {
			return nil, errors.New("type input changed")
		}
		parsedSources[filename] = append([]byte(nil), src...)
		return parser.ParseFile(fset, filename, src, parser.ParseComments)
	}
	dirs := 0
	roles := newTestMainRoles()
	native := nativePackageDriver()
	for _, root := range roots {
		var patterns []string
		for dir := range groups[root] {
			rel, _ := filepath.Rel(root, dir)
			patterns = append(patterns, "./"+filepath.ToSlash(rel))
		}
		sort.Strings(patterns)
		dirs += len(patterns)
		if dirs > maxTypeDirectories || ctx.Err() != nil {
			roles.valid = false
			continue
		}
		pkgs, err := packages.Load(&packages.Config{Context: ctx, Overlay: overlay, Dir: root, Mode: packages.LoadSyntax | packages.NeedForTest, Tests: true, ParseFile: parseBounded, BuildFlags: []string{"-mod=readonly"}}, patterns...)
		trace.record(root, patterns, pkgs, err)
		if err != nil {
			roles.valid = false
			continue
		}
		if capture != nil && native {
			// Request dependency file roles without expanding syntax loading. Never
			// mix type objects from separate loads. Metadata only narrows the exception.
			meta, err := packages.Load(&packages.Config{Context: ctx, Overlay: overlay, Dir: root, Mode: packages.LoadImports | packages.NeedDeps | packages.NeedForTest, Tests: true, BuildFlags: []string{"-mod=readonly"}}, patterns...)
			if err != nil {
				roles.valid = false
			} else {
				roles.add(pkgs, meta, root)
			}
		}
		// Prefer the test variant, which includes ordinary and internal test files.
		sort.SliceStable(pkgs, func(i, j int) bool { return len(pkgs[i].Syntax) < len(pkgs[j].Syntax) })
		for _, pkg := range pkgs {
			helpers := map[*types.Func]*ast.FuncDecl{}
			if pkg.TypesInfo != nil {
				for _, f := range pkg.Syntax {
					for _, d := range f.Decls {
						if fn, ok := d.(*ast.FuncDecl); ok {
							if obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func); ok {
								helpers[obj.Origin()] = fn
							}
						}
					}
				}
			}
			for _, f := range pkg.Syntax {
				abs := pkg.Fset.PositionFor(f.Pos(), false).Filename
				if _, wanted := out[abs]; !wanted {
					continue
				}
				trace.selected[abs] = pkg
				out[abs] = &typedFile{file: f, fset: pkg.Fset, info: pkg.TypesInfo, pkg: pkg.Types, helpers: helpers, incomplete: pkg.IllTyped || len(pkg.Errors) > 0 || pkg.TypesInfo == nil}
			}
		}
	}
	if capture != nil && native && ctx.Err() == nil {
		capture.unusedTestMain = roles.unused(out, overlay, parsedSources)
	}
	if overlay != nil {
		for path, b := range parsedSources {
			if _, ok := overlay[path]; !ok {
				overlay[path] = b
			}
		}
	}
	trace.canceled = ctx.Err() != nil
	return out
}

// Diagnostics contain source identities and compiler errors and are local-only.
// No diagnostic type is reachable from a production entry point.
type loadDiagnostic struct {
	selected               map[string]*packages.Package
	candidates             map[string][]*packages.Package
	Groups                 []diagnosticGroup
	parseCalls, budgetHits int
	canceled               bool
}
type diagnosticGroup struct {
	Root      string
	Patterns  []string
	LoadError string
	Packages  []diagnosticPackage
}
type diagnosticPackage struct {
	ID          string
	ForTest     string
	IllTyped    bool
	MissingInfo bool
	Errors      []packages.Error
	Files       []string
}

func newLoadDiagnostic() *loadDiagnostic {
	return &loadDiagnostic{selected: map[string]*packages.Package{}, candidates: map[string][]*packages.Package{}}
}
func (d *loadDiagnostic) record(root string, patterns []string, pkgs []*packages.Package, err error) {
	g := diagnosticGroup{Root: root, Patterns: patterns}
	if err != nil {
		g.LoadError = err.Error()
	}
	for _, p := range pkgs {
		row := diagnosticPackage{ID: p.ID, ForTest: p.ForTest, IllTyped: p.IllTyped, MissingInfo: p.TypesInfo == nil, Errors: p.Errors}
		for _, f := range p.Syntax {
			path := p.Fset.PositionFor(f.Pos(), false).Filename
			row.Files = append(row.Files, path)
			d.candidates[path] = append(d.candidates[path], p)
		}
		g.Packages = append(g.Packages, row)
	}
	d.Groups = append(d.Groups, g)
}

// Normalize only trace statements and the additional parameter. Any drift in
// production selection, budgets, flags, overlays or fallback fails this guard.
func TestDiagnosticLoaderParity(t *testing.T) {
	read := func(path, name string, diagnostic bool) string {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal("parity source unavailable")
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			fn.Name.Name = "loader"
			if diagnostic {
				fn.Type.Params.List = fn.Type.Params.List[:len(fn.Type.Params.List)-1]
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					block, ok := n.(*ast.BlockStmt)
					if !ok {
						return true
					}
					var kept []ast.Stmt
					for _, stmt := range block.List {
						var b bytes.Buffer
						_ = format.Node(&b, fset, stmt)
						if strings.HasPrefix(b.String(), "trace.") {
							continue
						}
						kept = append(kept, stmt)
					}
					block.List = kept
					return true
				})
			}
			// Reparse to remove source-position-dependent blank lines.
			var b bytes.Buffer
			_ = format.Node(&b, fset, fn)
			normalized := strings.Join(strings.Fields(b.String()), " ")
			return normalized
		}
		t.Fatal("parity declaration unavailable")
		return ""
	}
	if read("types.go", "loadReviewTypesCaptured", false) != read("type_diagnostic_test.go", "diagnosticLoad", true) {
		t.Fatal("diagnostic loader drift")
	}
}

func diagnosticState(files map[string]*typedFile) map[string][4]bool {
	out := map[string][4]bool{}
	for path, f := range files {
		out[path] = [4]bool{f.file != nil, f.info != nil, f.pkg != nil, f.incomplete}
	}
	return out
}
func hasTypeReason(reasons []string) bool {
	for _, s := range reasons {
		if s == "type_unavailable" {
			return true
		}
	}
	return false
}

// TestLocalTypeLoadingCensus is opt-in. It writes complete source reports and
// package identities only into a pre-existing protected directory outside the
// input root. It emits no source content or compiler error in test output.
func TestLocalTypeLoadingCensus(t *testing.T) {
	output := os.Getenv("GOQUENT_TYPE_DIAGNOSTIC_OUTPUT")
	if output == "" {
		t.Skip("local diagnostic not selected")
	}
	output, err := filepath.Abs(output)
	if err != nil {
		t.Fatal("invalid diagnostic output")
	}
	root := moduleRoot(".")
	rel, err := filepath.Rel(root, output)
	if err != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		t.Fatal("output must be outside input root")
	}
	st, err := os.Lstat(output)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		t.Fatal("output must be a protected directory")
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal("working directory unavailable")
	}
	if os.Chdir(root) != nil {
		t.Fatal("input root unavailable")
	}
	defer os.Chdir(old)
	write := func(name string, v any) {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal("diagnostic encoding failed")
		}
		f, err := os.OpenFile(filepath.Join(output, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("diagnostic output unavailable")
		}
		_, err = f.Write(b)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatal("diagnostic write failed")
		}
	}
	paths, err := discoverFiles("./...")
	if err != nil {
		t.Fatal("discovery failed")
	}
	opts := Options{Paths: []string{"./..."}}
	session := newSession(opts, paths)
	defer session.token.Close()
	overlay := func() map[string][]byte {
		out := map[string][]byte{}
		for p, b := range session.inputs {
			out[p] = append([]byte(nil), b...)
		}
		return out
	}
	report, runErr := Run(opts)
	write("source-report.json", report)
	if runErr != nil {
		write("source-error.json", runErr.Error())
		t.Fatal("source report incomplete")
	}
	trace := newLoadDiagnostic()
	diagnostic := diagnosticLoad(nil, paths, overlay(), nil, trace)
	existing := loadReviewTypesSnapshot(nil, paths, overlay())
	write("loader-diagnostic.json", trace.Groups)
	write("source-inputs.json", session.inputs)
	write("selected-paths.json", paths)
	write("loader-state.json", diagnosticState(diagnostic))
	// Only fixed aggregate keys are suitable for publication.
	counts := map[string]int{"findings": len(report.Findings), "operations": len(report.Operations), "parse_calls": trace.parseCalls, "budget_hits": trace.budgetHits}
	seen := map[string]bool{}
	replay := map[string]bool{}
	for path, tf := range diagnostic {
		fs, err := reviewTypedGo(newReviewContext(opts), path, tf)
		if err != nil {
			t.Fatal("diagnostic replay failed")
		}
		for _, f := range fs {
			if hasTypeReason(f.Detail.Reasons) && f.Location != nil {
				replay[locationKey(f.Location)] = true
			}
		}
	}
	for _, key := range []string{"duplicate_locations", "multiple_variants", "ill_typed", "package_errors", "missing_types_info", "no_selected_syntax", "selected_incomplete", "unknown", "unknown_complete_loader", "unmatched", "loader_state_mismatch", "replay_unmatched", "canceled"} {
		counts[key] = 0
	}
	for _, f := range report.Findings {
		if !hasTypeReason(f.Detail.Reasons) {
			continue
		}
		counts["type_findings"]++
		if f.Location == nil {
			counts["unknown"]++
			continue
		}
		key := locationKey(f.Location)
		if seen[key] {
			counts["duplicate_locations"]++
		}
		seen[key] = true
		path, _ := filepath.Abs(f.Location.File)
		tf := diagnostic[path]
		local := *f.Location
		local.File = path
		if !replay[locationKey(&local)] {
			counts["replay_unmatched"]++
		}
		if len(trace.candidates[path]) > 1 {
			counts["multiple_variants"]++
		}
		if tf == nil {
			counts["unmatched"]++
			continue
		}
		if !tf.incomplete && tf.file != nil {
			if diagnosticDynamicRaw(tf, f.Location) {
				counts["typed_dynamic_raw"]++
			} else {
				counts["unknown_complete_loader"]++
			}
			continue
		}
		p := trace.selected[path]
		if p == nil {
			counts["no_selected_syntax"]++
			continue
		}
		if p.IllTyped {
			counts["ill_typed"]++
		}
		if len(p.Errors) > 0 {
			counts["package_errors"]++
		}
		if p.TypesInfo == nil {
			counts["missing_types_info"]++
		}
		counts["selected_incomplete"]++
	}
	for _, op := range report.Operations {
		if hasTypeReason(op.Detail.Reasons) {
			counts["type_operations"]++
		}
	}
	if trace.canceled {
		counts["canceled"] = 1
	}
	if !reflect.DeepEqual(diagnosticState(existing), diagnosticState(diagnostic)) {
		counts["loader_state_mismatch"] = 1
	}
	write("counts.json", counts)
	write("context.json", map[string]any{"toolchain": runtime.Version(), "root": root, "flags": []string{"-mod=readonly"}, "tests": true, "mode": packages.LoadSyntax | packages.NeedForTest, "environment": map[string]string{"GOFLAGS": os.Getenv("GOFLAGS"), "GOWORK": os.Getenv("GOWORK"), "GOOS": os.Getenv("GOOS"), "GOARCH": os.Getenv("GOARCH"), "CGO_ENABLED": os.Getenv("CGO_ENABLED"), "GOPACKAGESDRIVER": os.Getenv("GOPACKAGESDRIVER"), "GOPROXY": os.Getenv("GOPROXY"), "GOTOOLCHAIN": os.Getenv("GOTOOLCHAIN")}})
	if !session.unchanged() || counts["loader_state_mismatch"] != 0 || counts["unmatched"] != 0 || counts["replay_unmatched"] != 0 {
		t.Fatal("diagnostic correspondence unavailable")
	}
}

// Synthetic cases establish selection mechanisms without classifying a real
// compiler error by its text. Calls are never executed.
func TestDiagnosticSelectionMechanisms(t *testing.T) {
	cases := []struct {
		name           string
		files          map[string]string
		wantIncomplete bool
		wantMultiple   bool
	}{
		{"ordinary", map[string]string{"source.go": "package fixture\nimport \"database/sql\"\nfunc use(db *sql.DB){db.Exec(\"SELECT 1\")}"}, false, false},
		{"internal_test", map[string]string{"source.go": "package fixture\nimport \"database/sql\"\nfunc use(db *sql.DB){db.Exec(\"SELECT 1\")}", "source_test.go": "package fixture\nimport \"testing\"\nfunc TestExample(t *testing.T){}"}, false, true},
		{"external_test", map[string]string{"source.go": "package fixture\n", "source_test.go": "package fixture_test\nimport (\"testing\";\"database/sql\")\nfunc TestExample(t *testing.T){var db *sql.DB;db.Exec(\"SELECT 1\")}"}, false, false},
		{"invalid_internal_test", map[string]string{"source.go": "package fixture\nimport \"database/sql\"\nfunc use(db *sql.DB){db.Exec(\"SELECT 1\")}", "source_test.go": "package fixture\nfunc broken(){missingTypeLoadingCanary()}"}, true, true},
		{"missing_import", map[string]string{"source.go": "package fixture\nimport _ \"missing.invalid/type-loading-canary\""}, true, false},
		{"missing_type", map[string]string{"source.go": "package fixture\nvar value missingTypeLoadingCanary"}, true, false},
		{"excluded", map[string]string{"source.go": "//go:build diagnostic_excluded\n\npackage fixture\n"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			fixtureModule(t, dir)
			paths := []string{}
			for name, src := range tc.files {
				p := filepath.Join(dir, name)
				if os.WriteFile(p, []byte(src), 0600) != nil {
					t.Fatal("fixture write failed")
				}
				paths = append(paths, p)
			}
			sort.Strings(paths)
			trace := newLoadDiagnostic()
			got := diagnosticLoad(nil, paths, nil, nil, trace)
			existing := loadReviewTypes(nil, paths)
			if !reflect.DeepEqual(diagnosticState(got), diagnosticState(existing)) {
				t.Fatal("selection correspondence failed")
			}
			incomplete, multiple := false, false
			for _, p := range paths {
				f := got[p]
				if f == nil || f.incomplete || f.file == nil {
					incomplete = true
				}
				if len(trace.candidates[p]) > 1 {
					multiple = true
				}
			}
			if incomplete != tc.wantIncomplete || multiple != tc.wantMultiple {
				t.Fatal("unexpected selection mechanism")
			}
			report, err := Run(Options{Paths: paths})
			if err != nil {
				t.Fatal("fixture review failed")
			}
			unavailable, raw := false, false
			for _, f := range report.Findings {
				unavailable = unavailable || hasTypeReason(f.Detail.Reasons)
				raw = raw || f.Code == "RAW_SQL_USED"
				if f.AnalysisPrecision == "precise" {
					t.Fatal("missing runtime evidence promoted")
				}
			}
			if unavailable != tc.wantIncomplete {
				t.Fatal("unsupported fallback changed")
			}
			if (!tc.wantIncomplete || tc.name == "invalid_internal_test") && !raw {
				t.Fatal("supported typed call lost")
			}
			view, err := report.PublicView()
			if err != nil {
				t.Fatal("public view unavailable")
			}
			if strings.Contains(view.String(), "Canary") || strings.Contains(view.String(), dir) {
				t.Fatal("private diagnostic escaped")
			}
		})
	}
}

// Match a physical selector location to type-confirmed raw recognition and its
// nonliteral SQL operand. This does not infer causes from diagnostic messages.
func diagnosticDynamicRaw(tf *typedFile, loc *query.SourceLocation) bool {
	matched := false
	ast.Inspect(tf.file, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := unparen(c.Fun).(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pos := tf.fset.PositionFor(sel.Sel.Pos(), false)
		if pos.Line != loc.Line || pos.Column != loc.Column {
			return true
		}
		fn := tf.function(c.Fun)
		if fn == nil {
			return true
		}
		isNew := fn.Pkg() != nil && fn.Pkg().Path() == queryPath && declaredReceiverName(fn) == "" && fn.Name() == "NewRawPlan"
		if !rawDatabaseFunction(fn) && !isNew {
			return true
		}
		if declaredReceiverName(fn) == "Stmt" {
			return true
		}
		i := rawSQLArgIndex(fn.Name())
		switch fn.Name() {
		case "QueryRowE", "SelectMap", "SelectMaps", "PrepareContext":
			i = 1
		case "Prepare", "NewRawPlan":
			i = 0
		}
		if selection := tf.info.Selections[sel]; selection != nil && selection.Kind() == types.MethodExpr {
			i++
		}
		if i < 0 || i >= len(c.Args) {
			return true
		}
		_, literal := stringLiteralValue(c.Args[i])
		matched = !literal
		return true
	})
	return matched
}

func TestDiagnosticDynamicRawCounterexample(t *testing.T) {
	dir := t.TempDir()
	fixtureModule(t, dir)
	path := filepath.Join(dir, "source.go")
	source := `package fixture
import "database/sql"
func example(db *sql.DB, text string) {
 db.Exec(text)
 db.Exec("SELECT 1")
}
`
	if os.WriteFile(path, []byte(source), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	trace := newLoadDiagnostic()
	loaded := diagnosticLoad(nil, []string{path}, nil, nil, trace)
	tf := loaded[path]
	if tf == nil || tf.incomplete || tf.info == nil {
		t.Fatal("typed counterexample unavailable")
	}
	r, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal("counterexample review failed")
	}
	dynamic, literal := 0, 0
	for _, f := range r.Findings {
		if hasTypeReason(f.Detail.Reasons) {
			dynamic++
			if f.Location == nil || !diagnosticDynamicRaw(tf, f.Location) {
				t.Fatal("dynamic cause not matched")
			}
		}
		if f.Code == "RAW_SQL_USED" {
			literal++
		}
		if f.AnalysisPrecision == query.AnalysisPrecise {
			t.Fatal("unsupported evidence promoted")
		}
	}
	if dynamic != 1 || literal != 1 {
		t.Fatal("literal counterexample failed")
	}
}

func TestDiagnosticRefusalBoundaries(t *testing.T) {
	for _, mode := range []string{"cancel", "budget", "custom_driver"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			fixtureModule(t, dir)
			path := filepath.Join(dir, "source.go")
			source := "package fixture\n"
			if mode == "budget" {
				source += "//" + strings.Repeat("x", maxTypeFileBytes)
			}
			if os.WriteFile(path, []byte(source), 0600) != nil {
				t.Fatal("fixture write failed")
			}
			ctx := context.Background()
			if mode == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if mode == "custom_driver" {
				driver := filepath.Join(dir, "driver")
				// A controlled empty custom driver must not be mistaken for native success.
				if os.WriteFile(driver, []byte("#!/bin/sh\nprintf '%s\\n' '{\"Roots\":[],\"Packages\":[]}'\n"), 0700) != nil {
					t.Fatal("driver fixture write failed")
				}
				t.Setenv("GOPACKAGESDRIVER", driver)
				if nativePackageDriver() {
					t.Fatal("custom driver treated as native")
				}
			}
			trace := newLoadDiagnostic()
			got := diagnosticLoad(ctx, []string{path}, nil, nil, trace)
			if f := got[path]; f == nil || !f.incomplete {
				t.Fatal("refusal lost")
			}
			report, err := Run(Options{Paths: []string{path}, Context: ctx})
			if err != nil {
				t.Fatal("refusal review failed")
			}
			if !hasFinding(report.Findings, query.WarningStaticReviewUnsupported) {
				t.Fatal("refusal promoted")
			}
			for _, f := range report.Findings {
				if f.AnalysisPrecision == query.AnalysisPrecise {
					t.Fatal("refusal became precise")
				}
			}
		})
	}
}
