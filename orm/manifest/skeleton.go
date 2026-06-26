package manifest

import (
	"fmt"
	"go/format"
	"sort"
	"strings"
	"unicode"
)

// RepositorySkeletonOptions controls manifest-backed repository skeleton output.
type RepositorySkeletonOptions struct {
	PackageName        string
	TableName          string
	RowTypeName        string
	RepositoryTypeName string
	ORMImportPath      string
}

// GenerateRepositorySkeleton emits a Go repository skeleton for one manifest table.
func GenerateRepositorySkeleton(m *Manifest, opts RepositorySkeletonOptions) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("goquent: manifest is nil")
	}
	table, err := selectSkeletonTable(m.Tables, opts.TableName)
	if err != nil {
		return nil, err
	}
	return GenerateRepositorySkeletonForTable(table, opts)
}

// GenerateRepositorySkeletonForTable emits a Go repository skeleton for table.
func GenerateRepositorySkeletonForTable(table Table, opts RepositorySkeletonOptions) ([]byte, error) {
	if strings.TrimSpace(table.Name) == "" {
		return nil, fmt.Errorf("goquent: skeleton table name is required")
	}
	packageName := sanitizeGoIdentifier(opts.PackageName)
	if packageName == "" {
		packageName = "repository"
	}
	rowType := sanitizeGoIdentifier(opts.RowTypeName)
	if rowType == "" {
		rowType = pascalName(singularTableName(table.Name)) + "Row"
	}
	repoType := sanitizeGoIdentifier(opts.RepositoryTypeName)
	if repoType == "" {
		repoType = strings.TrimSuffix(rowType, "Row") + "Repository"
	}
	ormImport := strings.TrimSpace(opts.ORMImportPath)
	if ormImport == "" {
		ormImport = "github.com/recoweft/goquent/orm"
	}

	imports := map[string]struct{}{
		"context":            {},
		ormImport:            {},
		ormImport + "/query": {},
	}
	fields, needsTime := skeletonFields(table.Columns)
	if needsTime {
		imports["time"] = struct{}{}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", packageName)
	writeSkeletonImports(&b, imports)
	fmt.Fprintf(&b, "// %s maps the %s table.\n", rowType, table.Name)
	fmt.Fprintf(&b, "type %s struct {\n", rowType)
	for _, field := range fields {
		fmt.Fprintf(&b, "\t%s %s `db:%q`\n", field.Name, field.Type, field.DBTag)
	}
	fmt.Fprintf(&b, "}\n\n")
	fmt.Fprintf(&b, "func (%s) TableName() string { return %q }\n\n", rowType, table.Name)

	fmt.Fprintf(&b, "// %s is a manifest-backed repository skeleton for %s.\n", repoType, table.Name)
	fmt.Fprintf(&b, "type %s struct {\n\tdb *orm.DB\n}\n\n", repoType)
	fmt.Fprintf(&b, "func New%s(db *orm.DB) *%s {\n\treturn &%s{db: db}\n}\n\n", repoType, repoType, repoType)

	required := requiredPredicateColumns(table)
	softDelete := softDeleteColumn(table)
	fmt.Fprintf(&b, "func (r *%s) BaseQuery(ctx context.Context, scopes ...orm.Scope) *query.Query {\n", repoType)
	fmt.Fprintf(&b, "\tq := r.db.Model(&%s{}).WithContext(ctx)\n", rowType)
	if len(required) > 0 {
		fmt.Fprintf(&b, "\tq = q.RequirePredicates(\n")
		for _, col := range required {
			fmt.Fprintf(&b, "\t\torm.RequirePredicate(%q, %q),\n", table.Name, col)
		}
		fmt.Fprintf(&b, "\t)\n")
	}
	if softDelete != "" {
		fmt.Fprintf(&b, "\tq = q.WhereNull(%q)\n", softDelete)
	}
	fmt.Fprintf(&b, "\treturn orm.ApplyScopes(q, scopes...)\n")
	fmt.Fprintf(&b, "}\n\n")

	fmt.Fprintf(&b, "func (r *%s) PlanBaseQuery(ctx context.Context, scopes ...orm.Scope) (*orm.QueryPlan, error) {\n", repoType)
	fmt.Fprintf(&b, "\treturn r.BaseQuery(ctx, scopes...).Plan(ctx)\n")
	fmt.Fprintf(&b, "}\n\n")

	for _, col := range required {
		fn := strings.TrimSuffix(rowType, "Row") + pascalName(col) + "Scope"
		if isTenantColumn(table, col) {
			fmt.Fprintf(&b, "func %s(value any) orm.Scope {\n\treturn orm.TenantScope(value, %q)\n}\n\n", fn, col)
			continue
		}
		fmt.Fprintf(&b, "func %s(value any) orm.Scope {\n", fn)
		fmt.Fprintf(&b, "\treturn func(q *query.Query) *query.Query {\n\t\treturn q.Where(%q, value)\n\t}\n", col)
		fmt.Fprintf(&b, "}\n\n")
	}

	fmt.Fprintf(&b, "func (r *%s) SelectAll(ctx context.Context, scopes ...orm.Scope) ([]%s, error) {\n", repoType, rowType)
	fmt.Fprintf(&b, "\treturn orm.SelectAllBy[%s](ctx, r.db, r.BaseQuery(ctx), scopes...)\n", rowType)
	fmt.Fprintf(&b, "}\n\n")

	fmt.Fprintf(&b, "func (r *%s) Insert(ctx context.Context, row %s) error {\n", repoType, rowType)
	fmt.Fprintf(&b, "\t_, err := orm.Insert(ctx, r.db, row)\n\treturn err\n")
	fmt.Fprintf(&b, "}\n\n")

	if pk := singlePrimaryColumn(table); pk != "" {
		fmt.Fprintf(&b, "func (r *%s) FindByID(ctx context.Context, id any, scopes ...orm.Scope) (%s, error) {\n", repoType, rowType)
		fmt.Fprintf(&b, "\tscopes = append(scopes, func(q *query.Query) *query.Query { return q.Where(%q, id) })\n", pk)
		fmt.Fprintf(&b, "\treturn orm.SelectOneBy[%s](ctx, r.db, r.BaseQuery(ctx), scopes...)\n", rowType)
		fmt.Fprintf(&b, "}\n\n")
		fmt.Fprintf(&b, "func (r *%s) UpdateByID(ctx context.Context, id any, patch map[string]any, scopes ...orm.Scope) error {\n", repoType)
		fmt.Fprintf(&b, "\tscopes = append(scopes, func(q *query.Query) *query.Query { return q.Where(%q, id) })\n", pk)
		fmt.Fprintf(&b, "\t_, err := orm.UpdateBy(ctx, r.BaseQuery(ctx), patch, scopes...)\n\treturn err\n")
		fmt.Fprintf(&b, "}\n\n")
		fmt.Fprintf(&b, "func (r *%s) DeleteByID(ctx context.Context, id any, scopes ...orm.Scope) error {\n", repoType)
		fmt.Fprintf(&b, "\tscopes = append(scopes, func(q *query.Query) *query.Query { return q.Where(%q, id) })\n", pk)
		fmt.Fprintf(&b, "\t_, err := orm.DeleteBy(ctx, r.BaseQuery(ctx), scopes...)\n\treturn err\n")
		fmt.Fprintf(&b, "}\n")
	}

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, err
	}
	return src, nil
}

type skeletonField struct {
	Name  string
	Type  string
	DBTag string
}

func selectSkeletonTable(tables []Table, tableName string) (Table, error) {
	tableName = strings.TrimSpace(tableName)
	if tableName == "" {
		if len(tables) == 1 {
			return tables[0], nil
		}
		return Table{}, fmt.Errorf("goquent: --table is required when manifest contains %d tables", len(tables))
	}
	for _, table := range tables {
		if normalizeName(table.Name) == normalizeName(tableName) {
			return table, nil
		}
	}
	return Table{}, fmt.Errorf("goquent: table %q not found in manifest", tableName)
}

func writeSkeletonImports(b *strings.Builder, imports map[string]struct{}) {
	var standard, external []string
	for path := range imports {
		if strings.Contains(path, ".") {
			external = append(external, path)
		} else {
			standard = append(standard, path)
		}
	}
	sort.Strings(standard)
	sort.Strings(external)
	b.WriteString("import (\n")
	for _, path := range standard {
		fmt.Fprintf(b, "\t%q\n", path)
	}
	if len(standard) > 0 && len(external) > 0 {
		b.WriteString("\n")
	}
	for _, path := range external {
		fmt.Fprintf(b, "\t%q\n", path)
	}
	b.WriteString(")\n\n")
}

func skeletonFields(columns []Column) ([]skeletonField, bool) {
	fields := make([]skeletonField, 0, len(columns))
	used := make(map[string]int, len(columns))
	needsTime := false
	for _, column := range columns {
		name := pascalName(column.Name)
		if name == "" {
			name = "Column"
		}
		if n := used[name]; n > 0 {
			used[name] = n + 1
			name = fmt.Sprintf("%s%d", name, n+1)
		} else {
			used[name] = 1
		}
		goType, usesTime := skeletonGoType(column)
		needsTime = needsTime || usesTime
		tag := column.Name
		if column.Primary {
			tag += ",pk"
		}
		fields = append(fields, skeletonField{Name: name, Type: goType, DBTag: tag})
	}
	return fields, needsTime
}

func skeletonGoType(column Column) (string, bool) {
	typ := strings.ToLower(strings.TrimSpace(column.Type))
	base := "any"
	usesTime := false
	switch {
	case strings.Contains(typ, "uuid"), strings.Contains(typ, "char"), strings.Contains(typ, "text"), strings.Contains(typ, "clob"):
		base = "string"
	case strings.Contains(typ, "json"):
		base = "orm.JSONField[map[string]any]"
	case strings.Contains(typ, "bool"):
		base = "bool"
	case strings.Contains(typ, "bigint"), strings.Contains(typ, "int8"):
		base = "int64"
	case strings.Contains(typ, "smallint"), strings.Contains(typ, "integer"), strings.Contains(typ, " int"), strings.HasPrefix(typ, "int"):
		base = "int"
	case strings.Contains(typ, "float"), strings.Contains(typ, "double"), strings.Contains(typ, "real"):
		base = "float64"
	case strings.Contains(typ, "numeric"), strings.Contains(typ, "decimal"):
		base = "string"
	case strings.Contains(typ, "time"), strings.Contains(typ, "date"):
		base = "time.Time"
		usesTime = true
	}
	if column.Nullable && base != "any" && !strings.HasPrefix(base, "orm.JSONField[") {
		base = "*" + base
	}
	return base, usesTime
}

func requiredPredicateColumns(table Table) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, column := range table.Columns {
		if column.RequiredFilter || column.TenantScope {
			key := normalizeName(column.Name)
			if _, ok := seen[key]; !ok {
				seen[key] = struct{}{}
				out = append(out, column.Name)
			}
		}
	}
	for _, policy := range table.Policies {
		if policy.Column == "" {
			continue
		}
		if policy.Type != "tenant_scope" && policy.Type != "required_filter" {
			continue
		}
		key := normalizeName(policy.Column)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			out = append(out, policy.Column)
		}
	}
	sort.Strings(out)
	return out
}

func softDeleteColumn(table Table) string {
	for _, column := range table.Columns {
		if column.SoftDelete {
			return column.Name
		}
	}
	for _, policy := range table.Policies {
		if policy.Type == "soft_delete" && policy.Column != "" {
			return policy.Column
		}
	}
	return ""
}

func singlePrimaryColumn(table Table) string {
	var out string
	for _, column := range table.Columns {
		if !column.Primary {
			continue
		}
		if out != "" {
			return ""
		}
		out = column.Name
	}
	return out
}

func isTenantColumn(table Table, column string) bool {
	for _, c := range table.Columns {
		if normalizeName(c.Name) == normalizeName(column) && c.TenantScope {
			return true
		}
	}
	for _, policy := range table.Policies {
		if policy.Type == "tenant_scope" && normalizeName(policy.Column) == normalizeName(column) {
			return true
		}
	}
	return false
}

func singularTableName(name string) string {
	name = strings.TrimSpace(name)
	switch {
	case strings.HasSuffix(name, "ies"):
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "ses"):
		return strings.TrimSuffix(name, "es")
	case strings.HasSuffix(name, "s") && len(name) > 1:
		return strings.TrimSuffix(name, "s")
	default:
		return name
	}
}

func pascalName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-' || r == '.' || r == ' '
	})
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)
		if isInitialism(upper) {
			b.WriteString(upper)
			continue
		}
		runes := []rune(strings.ToLower(part))
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return sanitizeGoIdentifier(b.String())
}

func sanitizeGoIdentifier(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range name {
		if r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r)) {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return ""
	}
	if unicode.IsDigit([]rune(out)[0]) {
		out = "_" + out
	}
	if goKeywords[out] {
		out += "_"
	}
	return out
}

func isInitialism(s string) bool {
	switch s {
	case "API", "DB", "HTML", "HTTP", "ID", "JSON", "SQL", "URL", "UUID", "XML":
		return true
	default:
		return false
	}
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}
