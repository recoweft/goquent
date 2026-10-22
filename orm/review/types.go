package review

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"golang.org/x/tools/go/packages"
)

const (
	maxTypeFiles       = 4096
	maxTypeFileBytes   = 2 << 20
	maxTypeInputBytes  = 64 << 20
	maxTypeDirectories = 256
	typeLoadTimeout    = 60 * time.Second
)

type typedFile struct {
	file       *ast.File
	fset       *token.FileSet
	info       *types.Info
	pkg        *types.Package
	helpers    map[*types.Func]*ast.FuncDecl
	incomplete bool
}

// loadReviewTypes owns all type identities for one Run. It never publishes
// loader diagnostics or reuses objects across runs, modules or package variants.
func loadReviewTypes(parent context.Context, paths []string) map[string]*typedFile {
	return loadReviewTypesSnapshot(parent, paths, nil)
}
func loadReviewTypesSnapshot(parent context.Context, paths []string, overlay map[string][]byte) map[string]*typedFile {
	return loadReviewTypesCaptured(parent, paths, overlay, nil)
}
func loadReviewTypesCaptured(parent context.Context, paths []string, overlay map[string][]byte, capture *ciCapture) map[string]*typedFile {
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
		parsedFiles++
		parsedBytes += st.Size()
		if st.Size() > maxTypeFileBytes || parsedBytes > maxTypeInputBytes || parsedFiles > maxTypeFiles {
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
	return out
}

func moduleRoot(dir string) string {
	dir, _ = filepath.Abs(dir)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			return ""
		}
		dir = next
	}
}

func unparen(e ast.Expr) ast.Expr {
	for {
		switch x := e.(type) {
		case *ast.ParenExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		default:
			return e
		}
	}
}
func (f *typedFile) object(id *ast.Ident) types.Object {
	if f.info == nil {
		return nil
	}
	return f.info.ObjectOf(id)
}
func (f *typedFile) function(e ast.Expr) *types.Func {
	if f.info == nil {
		return nil
	}
	var obj types.Object
	switch x := unparen(e).(type) {
	case *ast.Ident:
		obj = f.object(x)
	case *ast.SelectorExpr:
		obj = f.info.Uses[x.Sel]
	}
	fn, _ := obj.(*types.Func)
	if fn != nil {
		return fn.Origin()
	}
	return nil
}
func namedType(t types.Type, path, name string) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	n, ok := t.(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == path && n.Obj().Name() == name
}

const ormPath = "github.com/recoweft/goquent/orm"
const queryPath = ormPath + "/query"

func queryType(t types.Type) bool { return namedType(t, queryPath, "Query") }
func declaredReceiverName(fn *types.Func) string {
	sig, _ := fn.Type().(*types.Signature)
	if sig == nil || sig.Recv() == nil {
		return ""
	}
	t := types.Unalias(sig.Recv().Type())
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}
