package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/faciam-dev/goquent/orm/internal/stringutil"
	"github.com/faciam-dev/goquent/orm/migration"
	"github.com/faciam-dev/goquent/orm/model"
	"github.com/faciam-dev/goquent/orm/query"
)

const (
	Version           = "1"
	Generator         = "goquent"
	WarningStale      = "MANIFEST_STALE"
	WarningUnverified = "MANIFEST_UNVERIFIED"
	WarningRequired   = "MANIFEST_REQUIRED"
	WarningUnreadable = "MANIFEST_UNREADABLE"
)

// Manifest is the AI-readable schema/policy export.
type Manifest struct {
	Version                  string        `json:"version"`
	GeneratedAt              time.Time     `json:"generated_at"`
	GeneratorVersion         string        `json:"generator_version"`
	Dialect                  string        `json:"dialect,omitempty"`
	SchemaFingerprint        string        `json:"schema_fingerprint,omitempty"`
	PolicyFingerprint        string        `json:"policy_fingerprint,omitempty"`
	GeneratedCodeFingerprint string        `json:"generated_code_fingerprint,omitempty"`
	DatabaseFingerprint      string        `json:"database_fingerprint,omitempty"`
	Tables                   []Table       `json:"tables,omitempty"`
	Verification             *Verification `json:"verification,omitempty"`
}

// ToJSON returns stable, indented JSON for the manifest.
func (m *Manifest) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// Table describes an application-visible table/model.
type Table struct {
	Name          string         `json:"name"`
	Model         string         `json:"model,omitempty"`
	Columns       []Column       `json:"columns,omitempty"`
	Indexes       []Index        `json:"indexes,omitempty"`
	Relations     []Relation     `json:"relations,omitempty"`
	Policies      []Policy       `json:"policies,omitempty"`
	QueryExamples []QueryExample `json:"query_examples,omitempty"`
}

// Column describes a table column.
type Column struct {
	Name           string   `json:"name"`
	Type           string   `json:"type,omitempty"`
	Primary        bool     `json:"primary,omitempty"`
	Nullable       bool     `json:"nullable"`
	Default        string   `json:"default,omitempty"`
	Generated      bool     `json:"generated,omitempty"`
	EnumValues     []string `json:"enum_values,omitempty"`
	PII            bool     `json:"pii,omitempty"`
	Forbidden      bool     `json:"forbidden,omitempty"`
	TenantScope    bool     `json:"tenant_scope,omitempty"`
	SoftDelete     bool     `json:"soft_delete,omitempty"`
	RequiredFilter bool     `json:"required_filter,omitempty"`
}

// Index describes a database index.
type Index struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns,omitempty"`
	Unique  bool     `json:"unique,omitempty"`
}

// Relation is reserved for relationship metadata.
type Relation struct {
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	Table     string `json:"table,omitempty"`
	Column    string `json:"column,omitempty"`
	RefTable  string `json:"ref_table,omitempty"`
	RefColumn string `json:"ref_column,omitempty"`
}

// Policy describes a manifest policy entry for a table.
type Policy struct {
	Type   string           `json:"type"`
	Column string           `json:"column,omitempty"`
	Mode   query.PolicyMode `json:"mode,omitempty"`
}

// QueryExample gives tools a safe query-shape hint.
type QueryExample struct {
	Name        string   `json:"name"`
	Operation   string   `json:"operation"`
	Select      []string `json:"select,omitempty"`
	RequiredBy  string   `json:"required_by,omitempty"`
	Description string   `json:"description,omitempty"`
}

// Verification reports whether a manifest matches current inputs.
type Verification struct {
	Fresh     bool             `json:"fresh"`
	CheckedAt time.Time        `json:"checked_at"`
	Checks    []FreshnessCheck `json:"checks,omitempty"`
}

// FreshnessCheck describes one fingerprint comparison.
type FreshnessCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Options controls manifest generation.
type Options struct {
	Dialect            string
	GeneratedAt        time.Time
	GeneratorVersion   string
	Models             []any
	Schema             *migration.Schema
	Policies           []query.TablePolicy
	GeneratedCodePaths []string
	DatabaseSchema     *migration.Schema
}

// Generate builds a stable manifest from known schema/model/policy inputs.
func Generate(opts Options) (*Manifest, error) {
	generatedAt := opts.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
	}
	generatorVersion := strings.TrimSpace(opts.GeneratorVersion)
	if generatorVersion == "" {
		generatorVersion = Generator
	}

	tables := map[string]Table{}
	if opts.Schema != nil {
		for _, table := range opts.Schema.Tables {
			tables[table.Name] = tableFromSchema(table)
		}
	}
	for _, v := range opts.Models {
		table, err := tableFromModel(v)
		if err != nil {
			return nil, err
		}
		existing := tables[table.Name]
		tables[table.Name] = mergeTables(existing, table)
	}

	policies := opts.Policies
	if len(policies) == 0 {
		policies = query.RegisteredTablePolicies()
	}
	for _, policy := range policies {
		table := tables[policy.Table]
		if table.Name == "" {
			table.Name = policy.Table
		}
		applyPolicy(&table, policy)
		tables[table.Name] = table
	}

	out := &Manifest{
		Version:          Version,
		GeneratedAt:      generatedAt,
		GeneratorVersion: generatorVersion,
		Dialect:          strings.TrimSpace(opts.Dialect),
		Tables:           sortedTables(tables),
	}
	out.SchemaFingerprint = fingerprintSchema(out.Tables)
	out.PolicyFingerprint = fingerprintPolicies(out.Tables)
	codeFingerprint, err := fingerprintPaths(opts.GeneratedCodePaths)
	if err != nil {
		return nil, err
	}
	out.GeneratedCodeFingerprint = codeFingerprint
	if opts.DatabaseSchema != nil {
		out.DatabaseFingerprint = fingerprintMigrationSchema(*opts.DatabaseSchema)
	}
	return out, nil
}

// Load reads a manifest JSON file.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Verify compares a stored manifest with a freshly generated one.
func Verify(stored, current *Manifest, checkedAt time.Time) Verification {
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	v := Verification{Fresh: true, CheckedAt: checkedAt}
	add := func(name, expected, actual string) {
		check := FreshnessCheck{Name: name, Expected: expected, Actual: actual}
		switch {
		case expected == "" && actual == "":
			check.Status = "skipped"
			check.Message = "fingerprint not present"
		case expected == actual:
			check.Status = "ok"
		default:
			check.Status = "stale"
			check.Message = "fingerprint mismatch"
			v.Fresh = false
		}
		v.Checks = append(v.Checks, check)
	}
	if stored == nil || current == nil {
		return Verification{
			Fresh:     false,
			CheckedAt: checkedAt,
			Checks: []FreshnessCheck{{
				Name:    "manifest",
				Status:  "stale",
				Message: "manifest or current state is missing",
			}},
		}
	}
	add("schema", stored.SchemaFingerprint, current.SchemaFingerprint)
	add("policy", stored.PolicyFingerprint, current.PolicyFingerprint)
	add("generated_code", stored.GeneratedCodeFingerprint, current.GeneratedCodeFingerprint)
	add("database", stored.DatabaseFingerprint, current.DatabaseFingerprint)
	return v
}

// AttachVerification stores a freshness result on a copy of m.
func AttachVerification(m *Manifest, v Verification) *Manifest {
	if m == nil {
		return nil
	}
	copied := *m
	copied.Tables = append([]Table(nil), m.Tables...)
	copied.Verification = &v
	return &copied
}

func tableFromSchema(table migration.TableSchema) Table {
	out := Table{Name: table.Name}
	for _, column := range table.Columns {
		out.Columns = append(out.Columns, Column{
			Name:     column.Name,
			Type:     column.Type,
			Nullable: column.Nullable,
			Default:  column.DefaultExpression,
		})
	}
	for _, index := range table.Indexes {
		out.Indexes = append(out.Indexes, Index{Name: index.Name, Columns: append([]string(nil), index.Columns...), Unique: index.Unique})
	}
	sortTable(&out)
	return out
}

func tableFromModel(v any) (Table, error) {
	if v == nil {
		return Table{}, fmt.Errorf("goquent: manifest model is nil")
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Table{}, fmt.Errorf("goquent: manifest model must be a struct, got %s", t.Kind())
	}
	table := Table{Name: model.TableName(v), Model: t.Name()}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		column, ok := columnFromField(field)
		if !ok {
			continue
		}
		table.Columns = append(table.Columns, column)
	}
	sortTable(&table)
	return table, nil
}

func columnFromField(field reflect.StructField) (Column, bool) {
	dbTag := field.Tag.Get("db")
	name, opts := splitTag(dbTag)
	if name == "-" {
		return Column{}, false
	}
	tag := dbTag
	if name == "" {
		ormTag := field.Tag.Get("orm")
		name = parseORMColumn(ormTag)
		tag = ormTag
	}
	if name == "" {
		name = stringutil.ToSnake(field.Name)
	}
	column := Column{
		Name:     name,
		Type:     goTypeString(field.Type),
		Nullable: isNullableType(field.Type),
	}
	for _, opt := range opts {
		switch strings.TrimSpace(opt) {
		case "pk", "primary":
			column.Primary = true
			column.Nullable = false
		case "pii":
			column.PII = true
		case "forbidden":
			column.Forbidden = true
		case "tenant":
			column.TenantScope = true
		case "soft_delete":
			column.SoftDelete = true
		case "required_filter":
			column.RequiredFilter = true
		}
	}
	if strings.Contains(tag, "pk") {
		column.Primary = true
		column.Nullable = false
	}
	if defaultValue := field.Tag.Get("default"); defaultValue != "" {
		column.Default = defaultValue
	}
	return column, true
}

func applyPolicy(table *Table, policy query.TablePolicy) {
	if table == nil || policy.Table == "" {
		return
	}
	if policy.TenantColumn != "" {
		table.Policies = append(table.Policies, Policy{Type: "tenant_scope", Column: policy.TenantColumn, Mode: policy.TenantMode})
		markColumn(table, policy.TenantColumn, func(c *Column) { c.TenantScope = true; c.RequiredFilter = true })
	}
	if policy.SoftDeleteColumn != "" {
		table.Policies = append(table.Policies, Policy{Type: "soft_delete", Column: policy.SoftDeleteColumn, Mode: policy.SoftDeleteMode})
		markColumn(table, policy.SoftDeleteColumn, func(c *Column) { c.SoftDelete = true })
	}
	for _, col := range policy.PIIColumns {
		table.Policies = append(table.Policies, Policy{Type: "pii", Column: col, Mode: policy.PIIMode})
		markColumn(table, col, func(c *Column) { c.PII = true })
	}
	for _, col := range policy.RequiredFilterColumns {
		table.Policies = append(table.Policies, Policy{Type: "required_filter", Column: col, Mode: policy.RequiredFilterMode})
		markColumn(table, col, func(c *Column) { c.RequiredFilter = true })
	}
	table.QueryExamples = queryExamplesForPolicy(policy)
	sortTable(table)
}

func markColumn(table *Table, name string, fn func(*Column)) {
	for i := range table.Columns {
		if normalizeName(table.Columns[i].Name) == normalizeName(name) {
			fn(&table.Columns[i])
			return
		}
	}
	column := Column{Name: name, Nullable: true}
	fn(&column)
	table.Columns = append(table.Columns, column)
}

func queryExamplesForPolicy(policy query.TablePolicy) []QueryExample {
	var examples []QueryExample
	if policy.TenantColumn != "" {
		examples = append(examples, QueryExample{
			Name:       "tenant_scoped_select",
			Operation:  "select",
			Select:     []string{"id"},
			RequiredBy: "tenant_scope",
		})
	}
	if policy.SoftDeleteColumn != "" {
		examples = append(examples, QueryExample{
			Name:       "active_rows_select",
			Operation:  "select",
			Select:     []string{"id"},
			RequiredBy: "soft_delete",
		})
	}
	return examples
}

func mergeTables(a, b Table) Table {
	if a.Name == "" {
		return b
	}
	if b.Model != "" {
		a.Model = b.Model
	}
	a.Columns = mergeColumns(a.Columns, b.Columns)
	if len(b.Indexes) > 0 {
		a.Indexes = b.Indexes
	}
	sortTable(&a)
	return a
}

func mergeColumns(a, b []Column) []Column {
	byName := map[string]Column{}
	for _, column := range a {
		byName[normalizeName(column.Name)] = column
	}
	for _, column := range b {
		key := normalizeName(column.Name)
		existing := byName[key]
		if existing.Name == "" {
			byName[key] = column
			continue
		}
		if column.Type != "" {
			existing.Type = column.Type
		}
		existing.Primary = existing.Primary || column.Primary
		existing.Nullable = existing.Nullable || column.Nullable
		if column.Default != "" {
			existing.Default = column.Default
		}
		byName[key] = existing
	}
	out := make([]Column, 0, len(byName))
	for _, column := range byName {
		out = append(out, column)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedTables(tables map[string]Table) []Table {
	out := make([]Table, 0, len(tables))
	for _, table := range tables {
		sortTable(&table)
		out = append(out, table)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortTable(table *Table) {
	sort.Slice(table.Columns, func(i, j int) bool { return table.Columns[i].Name < table.Columns[j].Name })
	sort.Slice(table.Indexes, func(i, j int) bool { return table.Indexes[i].Name < table.Indexes[j].Name })
	sort.Slice(table.Policies, func(i, j int) bool {
		if table.Policies[i].Type == table.Policies[j].Type {
			return table.Policies[i].Column < table.Policies[j].Column
		}
		return table.Policies[i].Type < table.Policies[j].Type
	})
	sort.Slice(table.QueryExamples, func(i, j int) bool { return table.QueryExamples[i].Name < table.QueryExamples[j].Name })
}

func fingerprintSchema(tables []Table) string {
	schemaTables := make([]Table, len(tables))
	for i, table := range tables {
		schemaTables[i] = Table{Name: table.Name, Model: table.Model, Columns: table.Columns, Indexes: table.Indexes, Relations: table.Relations}
	}
	return fingerprintValue(schemaTables)
}

func fingerprintPolicies(tables []Table) string {
	type tablePolicies struct {
		Name     string        `json:"name"`
		Policies []Policy      `json:"policies,omitempty"`
		Columns  []ColumnFlags `json:"columns,omitempty"`
	}
	type payload struct {
		Tables []tablePolicies `json:"tables"`
	}
	var p payload
	for _, table := range tables {
		tp := tablePolicies{Name: table.Name, Policies: table.Policies}
		for _, column := range table.Columns {
			if column.PII || column.TenantScope || column.SoftDelete || column.RequiredFilter {
				tp.Columns = append(tp.Columns, ColumnFlags{
					Name: column.Name, PII: column.PII, TenantScope: column.TenantScope,
					SoftDelete: column.SoftDelete, RequiredFilter: column.RequiredFilter,
				})
			}
		}
		p.Tables = append(p.Tables, tp)
	}
	return fingerprintValue(p)
}

// ColumnFlags is used only to keep policy fingerprints stable and compact.
type ColumnFlags struct {
	Name           string `json:"name"`
	PII            bool   `json:"pii,omitempty"`
	TenantScope    bool   `json:"tenant_scope,omitempty"`
	SoftDelete     bool   `json:"soft_delete,omitempty"`
	RequiredFilter bool   `json:"required_filter,omitempty"`
}

func fingerprintMigrationSchema(schema migration.Schema) string {
	return fingerprintValue(schema)
}

func fingerprintPaths(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", ".codex", ".gocache", "vendor", "node_modules", "dist", "build", "coverage":
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(p, ".go") {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	h := sha256.New()
	for _, file := range files {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		_, _ = h.Write([]byte(file))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write(b)
		_, _ = h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func fingerprintValue(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func splitTag(tag string) (string, []string) {
	if tag == "" {
		return "", nil
	}
	parts := strings.Split(tag, ",")
	return strings.TrimSpace(parts[0]), parts[1:]
}

func parseORMColumn(tag string) string {
	for _, part := range strings.Split(tag, ",") {
		key, value, ok := strings.Cut(part, "=")
		if ok && strings.TrimSpace(key) == "column" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func goTypeString(t reflect.Type) string {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.PkgPath() == "" {
		return t.String()
	}
	return t.PkgPath() + "." + t.Name()
}

func isNullableType(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		return true
	}
	if t.PkgPath() == "database/sql" && strings.HasPrefix(t.Name(), "Null") {
		return true
	}
	return false
}

func normalizeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "`\"")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	return strings.ToLower(name)
}
