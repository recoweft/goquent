package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/recoweft/goquent/internal/inputjson"
)

// RepositoryRegistry is sensitive source configuration, rooted at its own
// directory. It contains no executable commands. All fields must be explicit.
type RepositoryRegistry struct {
	Version      int               `json:"version"`
	ManagedRoots []string          `json:"managed_roots"`
	Entries      []RepositoryEntry `json:"entries"`
}

type RepositoryEntry struct {
	SnapshotKind     string `json:"snapshot_kind"`
	Input            string `json:"input"`
	Target           string `json:"target"`
	GeneratorVersion string `json:"generator_version"`
	// CodePaths documents the fixed snapshot's original code hash scope. It may
	// not overlap any managed output. The checker never rebuilds that snapshot.
	CodePaths []string               `json:"code_paths"`
	Options   RepositoryCheckOptions `json:"options"`
}

type RepositoryCheckOptions struct {
	PackageName        string                      `json:"package_name"`
	TableName          string                      `json:"table_name"`
	RowTypeName        string                      `json:"row_type_name"`
	RepositoryTypeName string                      `json:"repository_type_name"`
	ORMImportPath      string                      `json:"orm_import_path"`
	Typed              bool                        `json:"typed"`
	Dialect            string                      `json:"dialect"`
	Projections        []RepositoryCheckProjection `json:"projections"`
}
type RepositoryCheckProjection struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
}

// RepositoryTableSnapshot keeps table-scoped fingerprints separate from a full
// manifest. Supplying it does not create whole-manifest freshness evidence.
type RepositoryTableSnapshot struct {
	Version           int    `json:"version"`
	SnapshotKind      string `json:"snapshot_kind"`
	Dialect           string `json:"dialect"`
	SchemaFingerprint string `json:"schema_fingerprint"`
	PolicyFingerprint string `json:"policy_fingerprint"`
	Table             Table  `json:"table"`
}

// RepositoryCheckResult contains only fixed classifications and bounded counts.
// Success means source equality against supplied input, not live verification.
type RepositoryCheckResult struct {
	Status  string `json:"status"`
	Checked int    `json:"checked"`
}

const RepositoryCheckMaxFiles = 4096
const RepositoryCheckMaxEntries = 256
const RepositoryCheckMaxFileBytes = 4 << 20
const RepositoryCheckMaxTotalBytes = 64 << 20

type repositoryReader struct {
	root  *os.Root
	total int
}

func repositoryPath(p string) bool {
	return p != "" && filepath.IsLocal(p) && filepath.Clean(p) == p && !strings.Contains(p, "\\")
}
func (r *repositoryReader) info(p string) (fs.FileInfo, error) {
	if !repositoryPath(p) {
		return nil, fs.ErrInvalid
	}
	parts := strings.Split(p, string(filepath.Separator))
	current := ""
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := r.root.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fs.ErrInvalid
		}
	}
	return r.root.Lstat(p)
}
func (r *repositoryReader) read(p string, limit int) ([]byte, error) {
	i, err := r.info(p)
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() || i.Size() > int64(limit) {
		return nil, fs.ErrInvalid
	}
	f, err := r.root.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(i, actual) {
		return nil, fs.ErrInvalid
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit+1)))
	if err != nil {
		return nil, err
	}
	r.total += len(b)
	if len(b) > limit || r.total > RepositoryCheckMaxTotalBytes {
		return nil, fs.ErrInvalid
	}
	return b, nil
}

// closedRepositoryJSON refuses unknown, ambiguous and case-misspelled members
// recursively, including schema metadata. It never returns source parser errors.
func closedRepositoryJSON(b []byte, out any, required bool) bool {
	if inputjson.CheckJSON(b) != nil {
		return false
	}
	var check func(json.RawMessage, reflect.Type) bool
	check = func(raw json.RawMessage, t reflect.Type) bool {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t == reflect.TypeOf(time.Time{}) {
			return true
		}
		switch t.Kind() {
		case reflect.Struct:
			allow := map[string]bool{}
			fields := map[string]reflect.Type{}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if !f.IsExported() {
					continue
				}
				tag := strings.Split(f.Tag.Get("json"), ",")[0]
				if tag == "-" {
					continue
				}
				if tag == "" {
					tag = f.Name
				}
				allow[tag] = true
				fields[tag] = f.Type
			}
			obj, err := inputjson.Object(raw, allow)
			if err != nil {
				return false
			}
			for k, v := range obj {
				if !check(v, fields[k]) {
					return false
				}
			}
			if required {
				for k := range allow {
					if _, ok := obj[k]; !ok {
						return false
					}
				}
			}
		case reflect.Slice:
			var list []json.RawMessage
			if json.Unmarshal(raw, &list) != nil {
				return false
			}
			for _, v := range list {
				if !check(v, t.Elem()) {
					return false
				}
			}
		}
		return true
	}
	if !check(b, reflect.TypeOf(out)) {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out) == nil
}

func insideRepositoryRoot(path, root string) bool {
	return root == "." || path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// CheckRepositories reads a bounded registry and compares actual regenerated
// source bytes. It does not write, run a compiler, read a DB or refresh globals.
func CheckRepositories(configPath string) RepositoryCheckResult {
	result := RepositoryCheckResult{Status: "invalid_config"}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return result
	}
	// The registry and each ancestor must themselves be nonsymlink paths.
	for p := abs; ; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if e != nil {
			result.Status = "missing_input"
			return result
		}
		if i.Mode()&os.ModeSymlink != 0 {
			return result
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	root, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return result
	}
	defer root.Close()
	r := repositoryReader{root: root}
	b, err := r.read(filepath.Base(abs), inputjson.MaxBytes)
	if err != nil {
		return result
	}
	var registry RepositoryRegistry
	if !closedRepositoryJSON(b, &registry, true) || registry.Version != 1 || len(registry.ManagedRoots) == 0 || len(registry.ManagedRoots) > RepositoryCheckMaxEntries || len(registry.Entries) == 0 || len(registry.Entries) > RepositoryCheckMaxEntries {
		return result
	}
	for i, p := range registry.ManagedRoots {
		info, e := r.info(p)
		if e != nil || !info.IsDir() {
			result.Status = "missing_root"
			return result
		}
		for _, other := range registry.ManagedRoots[:i] {
			if insideRepositoryRoot(p, other) || insideRepositoryRoot(other, p) {
				return result
			}
		}
	}
	targets := map[string]bool{}
	for _, e := range registry.Entries {
		if !repositoryPath(e.Input) || !repositoryPath(e.Target) || filepath.Ext(e.Target) != ".go" || targets[e.Target] || e.Input == e.Target {
			return result
		}
		managed := false
		for _, p := range registry.ManagedRoots {
			managed = managed || insideRepositoryRoot(e.Target, p)
		}
		if !managed {
			return result
		}
		targets[e.Target] = true
	}
	for _, e := range registry.Entries {
		seenCode := map[string]bool{}
		for _, p := range e.CodePaths {
			info, err := r.info(p)
			if err != nil || !info.Mode().IsRegular() || seenCode[p] || p == e.Input || p == filepath.Base(abs) {
				return result
			}
			seenCode[p] = true
			for target := range targets {
				if insideRepositoryRoot(target, p) || insideRepositoryRoot(p, target) {
					return result
				}
			}
		}
	}
	// Walk with bounded streaming directory batches, not an unbounded WalkDir.
	owned := map[string]bool{}
	count := 0
	var walk func(string, int) bool
	walk = func(path string, depth int) bool {
		if depth > 32 {
			return false
		}
		info, err := r.info(path)
		if err != nil {
			return false
		}
		count++
		if count > RepositoryCheckMaxFiles {
			return false
		}
		if info.IsDir() {
			d, err := r.root.Open(path)
			if err != nil {
				return false
			}
			defer d.Close()
			for {
				entries, e := d.ReadDir(64)
				for _, child := range entries {
					if !walk(filepath.Join(path, child.Name()), depth+1) {
						return false
					}
				}
				if e == io.EOF {
					break
				}
				if e != nil {
					return false
				}
			}
			return true
		}
		if !info.Mode().IsRegular() {
			return false
		}
		if filepath.Ext(path) != ".go" {
			return true
		}
		b, err := r.read(path, RepositoryCheckMaxFileBytes)
		if err != nil {
			return false
		}
		if bytes.HasPrefix(b, []byte(RepositoryOwnershipHeader+"\n")) {
			owned[path] = true
		}
		return true
	}
	for _, p := range registry.ManagedRoots {
		if !walk(p, 0) {
			result.Status = "invalid_target"
			return result
		}
	}
	for p := range owned {
		if !targets[p] {
			result.Status = "unregistered_target"
			return result
		}
	}
	for _, e := range registry.Entries {
		if !owned[e.Target] {
			result.Status = "missing_target"
			return result
		}
		if e.GeneratorVersion != RepositoryGeneratorVersion {
			result.Status = "stale_version"
			return result
		}
		if !e.Options.Typed || (e.Options.Dialect != "mysql" && e.Options.Dialect != "postgres") || e.Options.PackageName == "" || e.Options.TableName == "" || e.Options.RowTypeName == "" || e.Options.RepositoryTypeName == "" || e.Options.ORMImportPath == "" {
			return result
		}
		opts := RepositorySkeletonOptions{PackageName: e.Options.PackageName, TableName: e.Options.TableName, RowTypeName: e.Options.RowTypeName, RepositoryTypeName: e.Options.RepositoryTypeName, ORMImportPath: e.Options.ORMImportPath, Typed: e.Options.Typed, Dialect: e.Options.Dialect}
		for _, p := range e.Options.Projections {
			opts.Projections = append(opts.Projections, RepositoryProjection{Name: p.Name, Columns: p.Columns})
		}
		input, err := r.read(e.Input, inputjson.MaxBytes)
		if err != nil {
			result.Status = "missing_input"
			return result
		}
		result.Status = "invalid_snapshot"
		var source []byte
		switch e.SnapshotKind {
		case "manifest":
			var m Manifest
			if !closedRepositoryJSON(input, &m, false) || Validate(&m) != nil || !repositoryDeclarationsValid(m.Tables) || m.SchemaFingerprint != fingerprintSchema(m.Tables) || m.PolicyFingerprint != fingerprintPolicies(m.Tables) {
				return result
			}
			if (m.GeneratedCodeFingerprint != "") != (len(e.CodePaths) > 0) {
				return result
			}
			if len(e.CodePaths) > 0 {
				names := append([]string(nil), e.CodePaths...)
				sort.Strings(names)
				hash := sha256.New()
				for _, path := range names {
					data, e := r.read(path, RepositoryCheckMaxFileBytes)
					if e != nil {
						return result
					}
					hash.Write([]byte(path))
					hash.Write([]byte{0})
					hash.Write(data)
					hash.Write([]byte{0})
				}
				if m.GeneratedCodeFingerprint != "sha256:"+hex.EncodeToString(hash.Sum(nil)) {
					return result
				}
			}
			source, err = GenerateRepositorySkeleton(&m, opts)
		case "table":
			var s RepositoryTableSnapshot
			if !closedRepositoryJSON(input, &s, false) || s.Version != 1 || s.SnapshotKind != "table" || s.Dialect != opts.Dialect || s.Table.Name != opts.TableName || len(e.CodePaths) != 0 {
				return result
			}
			if !repositoryDeclarationsValid([]Table{s.Table}) {
				return result
			}
			// Table generation sorts columns before computing its scoped snapshot.
			cols := append([]Column(nil), s.Table.Columns...)
			sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })
			s.Table.Columns = cols
			if s.SchemaFingerprint != fingerprintSchema([]Table{s.Table}) || s.PolicyFingerprint != fingerprintPolicies([]Table{s.Table}) {
				return result
			}
			source, err = GenerateRepositorySkeletonForTable(s.Table, opts)
		default:
			return result
		}
		if err != nil || len(source) > RepositoryCheckMaxFileBytes {
			return result
		}
		stored, err := r.read(e.Target, RepositoryCheckMaxFileBytes)
		if err != nil {
			result.Status = "missing_target"
			return result
		}
		if !bytes.Equal(source, stored) {
			result.Status = "stale_source"
			return result
		}
		result.Checked++
	}
	result.Status = "current"
	return result
}

func repositoryDeclarationsValid(tables []Table) bool {
	seen := map[string]bool{}
	for _, t := range tables {
		name := normalizeName(t.Name)
		if name == "" || seen[name] {
			return false
		}
		seen[name] = true
		columns := map[string]bool{}
		for _, c := range t.Columns {
			key := normalizeName(c.Name)
			if key == "" || columns[key] {
				return false
			}
			columns[key] = true
		}
	}
	return len(tables) > 0
}
