package review

import (
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// nativePackageDriver mirrors x/tools v0.43.0's driver selection without
// overriding it. Unknown/custom drivers retain exact external correspondence.
func nativePackageDriver() bool {
	switch os.Getenv("GOPACKAGESDRIVER") {
	case "off":
		return true
	case "":
		_, err := exec.LookPath("gopackagesdriver")
		return err != nil
	default:
		return false
	}
}

type testMainRoles struct {
	valid      bool
	candidates map[string][]*packages.Package
	retained   map[string]bool
}

func newTestMainRoles() *testMainRoles {
	return &testMainRoles{valid: true, candidates: map[string][]*packages.Package{}, retained: map[string]bool{}}
}

func packageFiles(p *packages.Package) []string {
	files := append([]string{}, p.GoFiles...)
	files = append(files, p.CompiledGoFiles...)
	files = append(files, p.OtherFiles...)
	return append(files, p.IgnoredFiles...)
}

// Metadata is a second, bounded-in-time go list snapshot, never a substitute
// for consumed bytes. Require agreement with every syntax-load root, including
// import identities, before using its transitive file ownership information.
func samePackageMetadata(a, b *packages.Package) bool {
	if a == nil || b == nil || a.ID != b.ID || a.Name != b.Name || a.PkgPath != b.PkgPath || a.ForTest != b.ForTest || a.Dir != b.Dir || len(a.Errors) != 0 || len(b.Errors) != 0 || !slices.Equal(a.GoFiles, b.GoFiles) || !slices.Equal(a.CompiledGoFiles, b.CompiledGoFiles) || !slices.Equal(a.OtherFiles, b.OtherFiles) || !slices.Equal(a.IgnoredFiles, b.IgnoredFiles) || len(a.Imports) != len(b.Imports) {
		return false
	}
	for key, imp := range a.Imports {
		other := b.Imports[key]
		if imp == nil || other == nil || imp.ID != other.ID {
			return false
		}
	}
	return true
}

func (r *testMainRoles) add(loaded, metadata []*packages.Package, root string) {
	graph := map[string]*packages.Package{}
	incoming := map[string]bool{}
	var visit func(*packages.Package)
	visit = func(p *packages.Package) {
		if p == nil || p.ID == "" {
			r.valid = false
			return
		}
		if prev := graph[p.ID]; prev != nil {
			if prev != p {
				r.valid = false
			}
			return
		}
		graph[p.ID] = p
		if len(graph) > maxTypeFiles || len(p.Errors) != 0 {
			r.valid = false
			return
		}
		for _, dep := range p.Imports {
			if dep == nil {
				r.valid = false
				continue
			}
			incoming[dep.ID] = true
			visit(dep)
		}
	}
	for _, p := range metadata {
		visit(p)
	}
	if len(loaded) != len(metadata) {
		r.valid = false
	}
	rootIDs := map[string]bool{}
	for _, p := range metadata {
		if p != nil {
			rootIDs[p.ID] = true
		}
	}
	for _, p := range loaded {
		if p == nil || !rootIDs[p.ID] || !samePackageMetadata(p, graph[p.ID]) {
			r.valid = false
		}
	}
	candidates := map[string]string{}
	for _, p := range loaded {
		if unusedTestExecutable(p, graph, incoming, root) {
			file := p.CompiledGoFiles[0]
			candidates[p.ID] = file
			r.candidates[file] = append(r.candidates[file], p)
		}
	}
	// Roles are unioned across all roots and dependencies. Any ordinary role wins.
	for _, p := range graph {
		for _, file := range packageFiles(p) {
			if candidates[p.ID] != file {
				r.retained[file] = true
			}
		}
	}
}

// This recognizes the native go list test-executable graph, not a filename or
// generated comment. Test variants remain ordinary analysis inputs. The
// executable must be a root with no incoming edge, one external syntax file,
// and direct imports of a related test variant and the standard test harness.
func unusedTestExecutable(p *packages.Package, graph map[string]*packages.Package, incoming map[string]bool, root string) bool {
	if p == nil || p.Name != "main" || p.ForTest != "" || p.ID != p.PkgPath || !strings.HasSuffix(p.ID, ".test") || incoming[p.ID] || p.IllTyped || len(p.Errors) != 0 || p.Types == nil || !p.Types.Complete() || p.Types.Name() != "main" || p.Types.Path() != p.PkgPath || p.TypesInfo == nil || p.Fset == nil || len(p.GoFiles) != 1 || len(p.CompiledGoFiles) != 1 || len(p.Syntax) != 1 || p.GoFiles[0] != p.CompiledGoFiles[0] {
		return false
	}
	file := p.CompiledGoFiles[0]
	if !filepath.IsAbs(p.Dir) || !filepath.IsAbs(file) || filepath.Clean(file) != file || relativeFile(root, file) != "" || relativeFile(p.Dir, file) != "" || p.Syntax[0] == nil || p.Syntax[0].Name == nil || p.Syntax[0].Name.Name != "main" || p.Fset.PositionFor(p.Syntax[0].Pos(), false).Filename != file {
		return false
	}
	original := strings.TrimSuffix(p.ID, ".test")
	base := graph[original]
	if base == nil || base.ForTest != "" || base.ID != base.PkgPath || base.PkgPath != original || base.Dir != p.Dir || len(base.GoFiles) == 0 {
		return false
	}
	for _, name := range []string{"os", "testing", "testing/internal/testdeps"} {
		dep := p.Imports[name]
		if dep == nil || dep.ID != name || graph[dep.ID] == nil || graph[dep.ID].PkgPath != name {
			return false
		}
	}
	related := false
	for _, dep := range p.Imports {
		if dep == nil {
			return false
		}
		q := graph[dep.ID]
		if q == nil {
			return false
		}
		if q.ForTest == original {
			if q.Dir != base.Dir || (q.PkgPath != original && q.PkgPath != original+"_test") || q.ID != q.PkgPath+" ["+p.ID+"]" || len(q.CompiledGoFiles) == 0 {
				return false
			}
			related = true
		}
	}
	return related
}

func (r *testMainRoles) unused(out map[string]*typedFile, overlay, parsed map[string][]byte) map[string]bool {
	result := map[string]bool{}
	if !r.valid {
		return result
	}
	// Build the consumer closure once; many source files share a package.
	usedTypes := map[*types.Package]bool{}
	usedPaths := map[string]bool{}
	var visitTypes func(*types.Package)
	visitTypes = func(p *types.Package) {
		if p == nil || usedTypes[p] {
			return
		}
		usedTypes[p] = true
		usedPaths[p.Path()] = true
		for _, dep := range p.Imports() {
			visitTypes(dep)
		}
	}
	for _, tf := range out {
		if tf == nil {
			continue
		}
		visitTypes(tf.pkg)
		for fn := range tf.helpers {
			visitTypes(fn.Pkg())
		}
	}
	for file, owners := range r.candidates {
		if r.retained[file] || out[file] != nil || overlay[file] != nil || parsed[file] == nil {
			continue
		}
		used := false
		for _, owner := range owners {
			if usedTypes[owner.Types] || usedPaths[owner.Types.Path()] {
				used = true
				break
			}
			for _, tf := range out {
				if tf == nil {
					continue
				}
				if tf.file == owner.Syntax[0] || tf.info == owner.TypesInfo {
					used = true
					break
				}
			}
		}
		if !used {
			result[file] = true
		}
	}
	return result
}
