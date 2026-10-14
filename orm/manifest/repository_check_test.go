package manifest

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func checkFixture(t *testing.T) (string, *RepositoryRegistry, *Manifest) {
	t.Helper()
	dir := t.TempDir()
	m := &Manifest{Version: Version, GeneratedAt: time.Unix(1, 0).UTC(), GeneratorVersion: "fixture", Dialect: "postgres", Tables: []Table{{Name: "records", Columns: []Column{{Name: "id", Type: "bigint", TypeSource: "sql", NullableKnown: true, Primary: true}, {Name: "note", Type: "text", TypeSource: "sql", NullableKnown: true, Nullable: true}}}}}
	m.SchemaFingerprint = fingerprintSchema(m.Tables)
	m.PolicyFingerprint = fingerprintPolicies(m.Tables)
	o := RepositorySkeletonOptions{Typed: true, Dialect: "postgres", PackageName: "fixture", TableName: "records", RowTypeName: "RecordRow", RepositoryTypeName: "RecordRepository", ORMImportPath: "github.com/recoweft/goquent/orm"}
	s, e := GenerateRepositorySkeleton(m, o)
	if e != nil {
		t.Fatal("fixture generation failed")
	}
	if os.WriteFile(filepath.Join(dir, "generated.go"), s, 0600) != nil {
		t.Fatal("fixture write failed")
	}
	r := &RepositoryRegistry{Version: 1, ManagedRoots: []string{"."}, Entries: []RepositoryEntry{{SnapshotKind: "manifest", Input: "input.json", Target: "generated.go", GeneratorVersion: RepositoryGeneratorVersion, CodePaths: []string{}, Options: RepositoryCheckOptions{Typed: true, Dialect: o.Dialect, PackageName: o.PackageName, TableName: o.TableName, RowTypeName: o.RowTypeName, RepositoryTypeName: o.RepositoryTypeName, ORMImportPath: o.ORMImportPath, Projections: []RepositoryCheckProjection{}}}}}
	writeCheckJSON(t, filepath.Join(dir, "input.json"), m)
	writeCheckJSON(t, filepath.Join(dir, "registry.json"), r)
	return dir, r, m
}
func writeCheckJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil || os.WriteFile(path, b, 0600) != nil {
		t.Fatal("fixture write refused")
	}
}

func TestRepositoryCheckRegeneratesReadonly(t *testing.T) {
	dir, _, _ := checkFixture(t)
	p := filepath.Join(dir, "generated.go")
	before, _ := os.ReadFile(p)
	info, _ := os.Stat(p)
	for range 2 {
		r := CheckRepositories(filepath.Join(dir, "registry.json"))
		if r.Status != "current" || r.Checked != 1 {
			t.Fatal("fresh source refused")
		}
	}
	after, _ := os.ReadFile(p)
	last, _ := os.Stat(p)
	if !bytes.Equal(before, after) || !last.ModTime().Equal(info.ModTime()) {
		t.Fatal("checker wrote source")
	}
	// An embedded string is not an ownership header.
	if os.WriteFile(filepath.Join(dir, "manual.go"), []byte("package fixture\nvar s = `"+RepositoryOwnershipHeader+"`\n"), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	if CheckRepositories(filepath.Join(dir, "registry.json")).Status != "current" {
		t.Fatal("handwritten file misclassified")
	}
}

func TestRepositoryCheckDriftAndMissing(t *testing.T) {
	for _, which := range []string{"source", "schema", "policy", "version", "options", "projection", "scope", "fingerprint", "empty", "missing_input", "missing_target", "unregistered", "handwritten_target", "unknown_option", "duplicate", "escape", "symlink", "root", "oversize", "self_code_scope", "ambiguous"} {
		t.Run(which, func(t *testing.T) {
			dir, r, m := checkFixture(t)
			registry := filepath.Join(dir, "registry.json")
			source := filepath.Join(dir, "generated.go")
			switch which {
			case "source":
				f, e := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0600)
				if e != nil {
					t.Fatal("fixture failed")
				}
				f.WriteString("\n// drift\n")
				f.Close()
			case "schema":
				m.Tables[0].Columns[1].Type = "varchar(10)"
				m.SchemaFingerprint = fingerprintSchema(m.Tables)
			case "policy":
				m.Tables[0].Columns[1].PII = true
				m.SchemaFingerprint = fingerprintSchema(m.Tables)
				m.PolicyFingerprint = fingerprintPolicies(m.Tables)
			case "version":
				r.Entries[0].GeneratorVersion = "typed-repository-v0"
			case "options":
				r.Entries[0].Options.RowTypeName = "ChangedRow"
			case "projection":
				r.Entries[0].Options.Projections = []RepositoryCheckProjection{{Name: "Identity", Columns: []string{"id"}}}
			case "scope":
				r.Entries[0].SnapshotKind = "table"
			case "fingerprint":
				m.SchemaFingerprint = ""
			case "empty":
				r.Entries = nil
			case "missing_target":
				os.Remove(source)
			case "unregistered":
				b, _ := os.ReadFile(source)
				os.WriteFile(filepath.Join(dir, "other.go"), b, 0600)
			case "handwritten_target":
				os.WriteFile(source, []byte("package fixture\n"), 0600)
			case "duplicate":
				r.Entries = append(r.Entries, r.Entries[0])
			case "escape":
				r.Entries[0].Target = "../generated.go"
			case "symlink":
				os.Remove(source)
				os.Symlink("input.json", source)
			case "root":
				r.ManagedRoots = []string{"missing"}
			case "oversize":
				os.WriteFile(source, []byte(strings.Repeat("x", RepositoryCheckMaxFileBytes+1)), 0600)
			case "self_code_scope":
				r.Entries[0].CodePaths = []string{"."}
				m.GeneratedCodeFingerprint = "sha256:fixture"
			}
			writeCheckJSON(t, filepath.Join(dir, "input.json"), m)
			writeCheckJSON(t, registry, r)
			if which == "missing_input" {
				os.Remove(filepath.Join(dir, "input.json"))
			}
			if which == "unknown_option" || which == "ambiguous" {
				b, _ := os.ReadFile(registry)
				k := `"unknown_option":true,`
				if which == "ambiguous" {
					k = `"Version":1,`
				}
				b = bytes.Replace(b, []byte("{"), []byte("{"+k), 1)
				os.WriteFile(registry, b, 0600)
			}
			if CheckRepositories(registry).Status == "current" {
				t.Fatal("drift or invalid input accepted")
			}
		})
	}
}

func TestRepositoryCheckTableScope(t *testing.T) {
	dir, r, m := checkFixture(t)
	table := m.Tables[0]
	snapshot := RepositoryTableSnapshot{Version: 1, SnapshotKind: "table", Dialect: "postgres", Table: table, SchemaFingerprint: fingerprintSchema([]Table{table}), PolicyFingerprint: fingerprintPolicies([]Table{table})}
	r.Entries[0].SnapshotKind = "table"
	o := r.Entries[0].Options
	source, e := GenerateRepositorySkeletonForTable(table, RepositorySkeletonOptions{Typed: true, Dialect: o.Dialect, PackageName: o.PackageName, TableName: o.TableName, RowTypeName: o.RowTypeName, RepositoryTypeName: o.RepositoryTypeName, ORMImportPath: o.ORMImportPath})
	if e != nil {
		t.Fatal("table generation failed")
	}
	os.WriteFile(filepath.Join(dir, "generated.go"), source, 0600)
	writeCheckJSON(t, filepath.Join(dir, "input.json"), snapshot)
	writeCheckJSON(t, filepath.Join(dir, "registry.json"), r)
	if CheckRepositories(filepath.Join(dir, "registry.json")).Status != "current" {
		t.Fatal("table-scoped source refused")
	}
	r.Entries[0].SnapshotKind = "manifest"
	writeCheckJSON(t, filepath.Join(dir, "registry.json"), r)
	if CheckRepositories(filepath.Join(dir, "registry.json")).Status == "current" {
		t.Fatal("table snapshot promoted")
	}
}
