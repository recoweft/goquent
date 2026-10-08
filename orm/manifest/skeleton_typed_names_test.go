package manifest_test

import (
	"bytes"
	"errors"
	"fmt"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
)

// Each accepted fixture is compiled in an external module. Compiler diagnostics
// stay private: they can contain source identifiers and enum values.
func TestTypedRepositoryDeclarationCompile(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal("module root unavailable")
	}
	compile := func(t *testing.T, src []byte) bool {
		t.Helper()
		dir := t.TempDir()
		mod := "module declarationfixture\n\ngo 1.25.0\n\nrequire github.com/recoweft/goquent v0.0.0\nreplace github.com/recoweft/goquent => " + filepath.ToSlash(root) + "\n"
		for name, data := range map[string][]byte{"go.mod": []byte(mod), "generated.go": src} {
			if os.WriteFile(filepath.Join(dir, name), data, 0600) != nil {
				t.Fatal("fixture write failed")
			}
		}
		cmd := exec.Command("go", "test", "-mod=mod", "-run=^$", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		if err != nil {
			switch {
			case bytes.Contains(out, []byte("imported and not used")):
				t.Log("actual compile: unused import")
			case bytes.Contains(out, []byte("redeclared")):
				t.Log("actual compile: redeclared identifier")
			default:
				t.Log("actual compile: type check failed (diagnostics omitted)")
			}
		}
		return err == nil
	}
	type fixture struct {
		name   string
		change func(*manifest.Table, *manifest.RepositorySkeletonOptions)
		refuse bool
	}
	fixtures := []fixture{
		{"no_primary", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) { m.Columns[0].Primary = false }, false},
		{"unknown_primary", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) { m.Columns[0].TypeSource = "" }, false},
		{"forbidden_primary", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) { m.Columns[0].Forbidden = true }, false},
		{"empty", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) { m.Columns = nil }, false},
		{"unknown_only", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns[0].Primary = false
			m.Columns[0].TypeSource = "go"
		}, false},
		{"forbidden_only", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns[0].Primary = false
			m.Columns[0].Forbidden = true
		}, false},
		{"key", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {}, false},
		{"projection", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns[0].Primary = false
			o.Projections = []manifest.RepositoryProjection{{Name: "Identity", Columns: []string{"id"}}}
		}, false},
		{"enum_repository", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns = append(m.Columns, manifest.Column{Name: "status", Type: "varchar(12)", TypeSource: "sql", EnumValues: []string{"ready", "closed"}})
			o.RepositoryTypeName = "RecordStatusValueChoice1"
		}, false},
		{"enum_only_repository", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns = []manifest.Column{{Name: "status", Type: "varchar(12)", TypeSource: "sql", EnumValues: []string{"ready", "closed"}}}
			o.RepositoryTypeName = "RecordStatusValueChoice1"
		}, false},
		{"enum_key_repository", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns[0].Type = "varchar(12)"
			m.Columns[0].EnumValues = []string{"ready"}
			o.RepositoryTypeName = "RecordKeyChoice1"
		}, false},
		{"field_names", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			for _, name := range []string{"a_b", "a__b", "type", "_", "table_name", "string", "format", "marshal_json"} {
				m.Columns = append(m.Columns, manifest.Column{Name: name, Type: "text", TypeSource: "sql"})
			}
		}, false},
		{"duplicate_column", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns = append(m.Columns, m.Columns[0])
		}, true},
		{"duplicate_projection", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			o.Projections = []manifest.RepositoryProjection{{Name: "A_b", Columns: []string{"id"}}, {Name: "a__b", Columns: []string{"id"}}}
		}, true},
		{"projection_type", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			o.RepositoryTypeName = "RecordIdentityRow"
			o.Projections = []manifest.RepositoryProjection{{Name: "Identity", Columns: []string{"id"}}}
		}, false},
		{"column_type", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			o.RepositoryTypeName = "RecordIDColumn"
		}, false},
		{"clock_type", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns = append(m.Columns, manifest.Column{Name: "clock", Type: "time", TypeSource: "sql"})
			o.RepositoryTypeName = "RecordTimeText"
		}, false},
		{"tenant_helper", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns[0].TenantScope = true
			o.RepositoryTypeName = "RecordCurrentTenantKey"
		}, true},
		{"multiple_tenant_helpers", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
			m.Columns[0].TenantScope = true
			m.Columns = append(m.Columns, manifest.Column{Name: "tenant", Type: "integer", TypeSource: "sql", NullableKnown: true, Primary: true, TenantScope: true})
		}, true},
	}
	for _, name := range []string{"RecordRow", "RecordPredicate", "RecordOrder", "RecordRead", "RecordKey", "RecordColumns", "RecordColumnSet", "recordSnapshot", "recordKeyInput"} {
		fixtures = append(fixtures, fixture{"fixed_" + name, func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) { o.RepositoryTypeName = name }, true})
	}
	fixtures = append(fixtures, fixture{"constructor", func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) { o.RowTypeName = "NewRecordRepository" }, true})
	// Reserve the standard library's universe, including every width, builtin
	// function and constant; test both caller-controlled top-level type names.
	for _, name := range append(types.Universe.Names(), "orm", "context", "sql", "json", "fmt", "time", "operation", "manifest", "init", "db", "ctx", "input", "key", "r", "rows", "err") {
		for _, row := range []bool{false, true} {
			fixtures = append(fixtures, fixture{fmt.Sprintf("reserved_%s_%t", name, row), func(m *manifest.Table, o *manifest.RepositorySkeletonOptions) {
				if row {
					o.RowTypeName = name
				} else {
					o.RepositoryTypeName = name
				}
			}, true})
		}
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		for _, entry := range []string{"manifest", "table"} {
			for _, f := range fixtures {
				t.Run(dialect+"/"+entry+"/"+f.name, func(t *testing.T) {
					m := manifest.Table{Name: "records", Columns: []manifest.Column{{Name: "id", Type: "integer", TypeSource: "sql", NullableKnown: true, Primary: true}}}
					o := manifest.RepositorySkeletonOptions{Typed: true, Dialect: dialect, PackageName: "fixture", RowTypeName: "RecordRow", RepositoryTypeName: "RecordRepository"}
					f.change(&m, &o)
					generate := func() ([]byte, error) {
						if entry == "table" {
							return manifest.GenerateRepositorySkeletonForTable(m, o)
						}
						return manifest.GenerateRepositorySkeleton(&manifest.Manifest{Version: manifest.Version, Dialect: dialect, Tables: []manifest.Table{m}}, o)
					}
					src, err := generate()
					if err != nil {
						if !f.refuse || err != manifest.ErrRepositoryGeneration || !errors.Is(err, manifest.ErrRepositoryGeneration) || len(src) != 0 || fmt.Sprint(err) != "goquent: repository generation refused" {
							t.Fatal("unexpected generation refusal")
						}
						return
					}
					again, err := generate()
					if err != nil || !bytes.Equal(src, again) {
						t.Fatal("nondeterministic generation")
					}
					if !compile(t, src) {
						t.Fatal("accepted source failed actual external module compile")
					}
					if f.refuse {
						t.Fatal("reserved declaration was accepted")
					}
					if strings.Contains(f.name, "primary") || f.name == "unknown_only" || f.name == "forbidden_only" || f.name == "empty" {
						if bytes.Contains(src, []byte("FindByKey(")) {
							t.Fatal("unsupported key was inferred")
						}
					}
				})
			}
		}
	}
}
