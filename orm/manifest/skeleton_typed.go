package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/recoweft/goquent/orm/internal/sqltype"
)

// RepositoryProjection declares an ordered, literal-column result shape.
// Names and declarations are sensitive source input, not display diagnostics.
type RepositoryProjection struct {
	Name    string
	Columns []string
}

// RepositoryGeneratorVersion identifies this implementation, not caller metadata.
const RepositoryGeneratorVersion = "typed-repository-v1"

var ErrRepositoryGeneration = errors.New("goquent: repository generation refused")

// Typed output deliberately excludes identifier expressions. The dynamic APIs
// retain their existing naming/SQL contracts; generation never infers raw SQL.
var repositoryIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type typedColumn struct {
	column                             Column
	field                              skeletonField
	decl                               sqltype.Type
	input, scalar, columnType, keyType string
	tenant                             bool
}
type typedProjection struct {
	name, row string
	columns   []typedColumn
}
type typedSkeleton struct {
	clockType         string
	table             Table
	opts              RepositorySkeletonOptions
	row, repo, prefix string
	fields            []skeletonField
	columns           []typedColumn
	projections       []typedProjection
	keys              []typedColumn
	names             map[string]bool
	snapshot          []byte
}

func (t *typedSkeleton) name(base string) string {
	if base == "" || base == "_" {
		base = "Column"
	}
	n := base
	for i := 2; t.names[n] || token.Lookup(n).IsKeyword(); i++ {
		n = base + strconv.Itoa(i)
	}
	t.names[n] = true
	return n
}
func prepareTypedSkeleton(table Table, opts RepositorySkeletonOptions, pkg, row, repo string) (*typedSkeleton, error) {
	if opts.Dialect != "mysql" && opts.Dialect != "postgres" {
		return nil, ErrRepositoryGeneration
	}
	if !repositoryIdentifier.MatchString(table.Name) || !token.IsIdentifier(pkg) || pkg == "_" || !token.IsIdentifier(row) || !token.IsIdentifier(repo) || row == "_" || repo == "_" || row == repo {
		return nil, ErrRepositoryGeneration
	}
	t := &typedSkeleton{table: table, opts: opts, row: row, repo: repo, prefix: strings.TrimSuffix(row, "Row"), names: map[string]bool{}}
	if t.prefix == "" {
		t.prefix = row
	}
	for _, n := range []string{"context", "sql", "json", "fmt", "time", "orm", "operation", "manifest", "string", "bool", "any", "int64", "uint64"} {
		t.names[n] = true
	}
	for _, n := range []string{row, repo, "New" + repo, t.prefix + "Predicate", t.prefix + "Order", t.prefix + "Read", t.prefix + "Key", t.prefix + "Columns", t.prefix + "ColumnSet", privatePrefix(t.prefix) + "Snapshot", privatePrefix(t.prefix) + "KeyInput"} {
		if t.names[n] {
			return nil, ErrRepositoryGeneration
		}
		t.names[n] = true
	}
	fieldNames := map[string]bool{"TableName": true, "String": true, "Format": true, "MarshalJSON": true}
	seen := map[string]bool{}
	cols := append([]Column(nil), table.Columns...)
	sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })
	keyOK := true
	primaryCount := 0
	for _, c := range cols {
		if c.Primary {
			primaryCount++
		}
	}
	for _, c := range cols {
		if !repositoryIdentifier.MatchString(c.Name) || seen[normalizeName(c.Name)] {
			return nil, ErrRepositoryGeneration
		}
		seen[normalizeName(c.Name)] = true
		d, err := sqltype.Parse(sqltype.Declaration{Type: c.Type, TypeSource: c.TypeSource}, opts.Dialect)
		if err != nil {
			return nil, ErrRepositoryGeneration
		}
		name := pascalName(c.Name)
		if name == "" || name == "_" {
			name = "Column"
		}
		base := name
		for i := 2; fieldNames[name]; i++ {
			name = base + strconv.Itoa(i)
		}
		fieldNames[name] = true
		col := typedColumn{column: c, decl: d, tenant: isTenantColumn(table, c.Name)}
		scalar := ""
		if d.Missing == "" && !d.Array {
			switch d.Kind {
			case "integer":
				width := d.Bits
				if width == 24 {
					width = 32
				}
				scalar = "int" + strconv.Itoa(width)
				if d.Unsigned {
					scalar = "u" + scalar
				}
			case "bool":
				scalar = "bool"
			case "string", "uuid", "date", "time", "timestamp", "datetime":
				scalar = "string"
			case "decimal":
				scalar = "json.Number"
			}
		}
		col.scalar = scalar
		col.input = scalar
		rowType := scalar
		if d.Kind == "time" && scalar != "" {
			if t.clockType == "" {
				t.clockType = t.name(t.prefix + "TimeText")
			}
			rowType = t.clockType
		}
		if rowType == "" {
			rowType = "any"
		}
		if d.Kind == "decimal" && scalar != "" {
			rowType = "string"
		}
		if scalar != "" && (d.Kind == "date" || d.Kind == "timestamp" || d.Kind == "datetime") {
			rowType = "time.Time"
		}
		if c.Nullable || !c.NullableKnown {
			if rowType == "bool" {
				rowType = "sql.NullBool"
			} else if rowType != "any" {
				rowType = "sql.Null[" + rowType + "]"
			}
		}
		if c.Primary {
			if scalar == "" || c.Forbidden {
				keyOK = false
			} else {
				if primaryCount == 1 && !col.tenant {
					col.keyType = t.prefix + "Key"
				} else {
					col.keyType = t.name(t.prefix + name + "Key")
				}
				col.input = col.keyType
			}
		} else if len(c.EnumValues) > 0 && scalar == "string" {
			col.input = t.name(t.prefix + name + "Value")
		}
		tag := c.Name
		if c.Primary {
			tag += ",pk"
		}
		if c.Readonly || c.Generated {
			tag += ",readonly"
		}
		col.field = skeletonField{Name: name, Type: rowType, DBTag: tag}
		col.columnType = t.name(t.prefix + name + "Column")
		t.columns = append(t.columns, col)
		t.fields = append(t.fields, col.field)
	}
	for _, c := range t.columns {
		if c.column.Primary && keyOK {
			t.keys = append(t.keys, c)
		}
	}
	methods := map[string]bool{"FindByKey": true, "PlanFindByKey": true, "String": true, "Format": true, "MarshalJSON": true}
	for _, p := range opts.Projections {
		name := pascalName(p.Name)
		if name == "" || name == "_" || len(p.Columns) == 0 || methods["Plan"+name] || methods["Select"+name] {
			return nil, ErrRepositoryGeneration
		}
		methods["Plan"+name] = true
		methods["Select"+name] = true
		pr := typedProjection{name: name, row: t.name(t.prefix + name + "Row")}
		used := map[string]bool{}
		for _, name := range p.Columns {
			if used[normalizeName(name)] {
				return nil, ErrRepositoryGeneration
			}
			used[normalizeName(name)] = true
			found := false
			for _, c := range t.columns {
				if c.column.Name == name && !c.column.Forbidden {
					pr.columns = append(pr.columns, c)
					found = true
					break
				}
			}
			if !found {
				return nil, ErrRepositoryGeneration
			}
		}
		t.projections = append(t.projections, pr)
	}
	m := opts.snapshot
	if m == nil {
		table.Columns = cols
		tables := []Table{table}
		m = &Manifest{Version: Version, Dialect: opts.Dialect, Tables: tables, SchemaFingerprint: fingerprintSchema(tables), PolicyFingerprint: fingerprintPolicies(tables)}
		t.opts.snapshotKind = "table"
	}
	var err error
	t.snapshot, err = json.Marshal(m)
	if err != nil {
		return nil, ErrRepositoryGeneration
	}
	return t, nil
}
func (t *typedSkeleton) imports(path string) map[string]struct{} {
	out := map[string]struct{}{"context": {}, "encoding/json": {}, "fmt": {}, path: {}, path + "/manifest": {}, path + "/operation": {}}
	if t.clockType != "" {
		out["time"] = struct{}{}
	}
	for _, f := range t.fields {
		if strings.Contains(f.Type, "sql.") {
			out["database/sql"] = struct{}{}
		}
		if strings.Contains(f.Type, "time.") {
			out["time"] = struct{}{}
		}
	}
	return out
}
func (t *typedSkeleton) header(b *strings.Builder) {
	fmt.Fprintf(b, "// Code generated by goquent %s; DO NOT EDIT.\n", RepositoryGeneratorVersion)
	var m Manifest
	_ = json.Unmarshal(t.snapshot, &m)
	fmt.Fprintf(b, "// Snapshot kind: %s; supplied declarations only, not live identity or authorization.\n// Schema fingerprint: %q\n// Policy fingerprint: %q\n", t.opts.snapshotKind, m.SchemaFingerprint, m.PolicyFingerprint)
}
func opaqueType(b *strings.Builder, name string) {
	fmt.Fprintf(b, `func (%s) String() string { return "goquent: generated details omitted" }
func (%s) Format(s fmt.State, verb rune) { _, _ = s.Write([]byte("goquent: generated details omitted")) }
func (%s) MarshalJSON() ([]byte,error) { return []byte("{\"details_omitted\":true}"),nil }
`, name, name, name)
}
func quotedColumns(cols []typedColumn) string {
	var names []string
	for _, c := range cols {
		names = append(names, strconv.Quote(c.column.Name))
	}
	return "[]string{" + strings.Join(names, ",") + "}"
}
func (t *typedSkeleton) emit(b *strings.Builder) {
	p := t.prefix
	if t.clockType != "" {
		fmt.Fprintf(b, `// %s scans the built-in drivers' clock results without a driver argument adapter.
type %s string
func (v *%s) Scan(source any) error {
 switch x:=source.(type) {
 case string: *v=%s(x)
 case []byte: *v=%s(string(x))
 case time.Time:
  if x.Year()!=0 || x.Month()!=time.January || x.Day()!=1 {return fmt.Errorf("goquent: clock result is unsupported")}
  *v=%s(x.Format("15:04:05.999999999"))
 default: return fmt.Errorf("goquent: clock result is unsupported")
 }
 return nil
}
`, t.clockType, t.clockType, t.clockType, t.clockType, t.clockType, t.clockType)
		opaqueType(b, t.clockType)
	}
	opaqueType(b, t.row)
	opaqueType(b, t.repo)
	fmt.Fprintf(b, `type %sPredicate struct { filter operation.FilterSpec }
type %sOrder struct { order operation.OrderSpec }
// %sRead carries typed filters and ordering. Runtime validation still applies.
type %sRead struct { Filters []%sPredicate; OrderBy []%sOrder; Limit *int64; AccessReason string }
`, p, p, p, p, p, p)
	for _, n := range []string{p + "Predicate", p + "Order", p + "Read"} {
		opaqueType(b, n)
	}
	fmt.Fprintf(b, `func %sSnapshot() *manifest.Manifest { var m manifest.Manifest; _ = json.Unmarshal([]byte(%q), &m); return &m }
`, privatePrefix(p), string(t.snapshot))
	// The private source function returns a fresh snapshot on every operation.
	snapshotName := privatePrefix(p) + "Snapshot"
	fmt.Fprintf(b, `func (in %sRead) spec(columns []string) operation.OperationSpec {
 s := operation.OperationSpec{Version:1, Operation:"select", Model:%q, Select:columns, AccessReason:in.AccessReason}
 if in.Limit != nil { n:=*in.Limit; s.Limit=&n }
 for _,f := range in.Filters { s.Filters=append(s.Filters,f.filter) }
 for _,o := range in.OrderBy { s.OrderBy=append(s.OrderBy,o.order) }
 return s
}
`, p, t.table.Name)
	fmt.Fprintf(b, "type %sColumnSet struct {\n", p)
	for _, c := range t.columns {
		if !c.column.Forbidden {
			fmt.Fprintf(b, "%s %s\n", c.field.Name, c.columnType)
		}
	}
	b.WriteString("}\n")
	opaqueType(b, p+"ColumnSet")
	fmt.Fprintf(b, "func %sColumns() %sColumnSet { return %sColumnSet{", p, p, p)
	for _, c := range t.columns {
		if !c.column.Forbidden {
			fmt.Fprintf(b, "%s:%s{valid:true},", c.field.Name, c.columnType)
		}
	}
	b.WriteString("} }\n")
	for _, c := range t.columns {
		if c.input != "" && c.input != c.scalar {
			if c.tenant && c.keyType != "" {
				fmt.Fprintf(b, "type %s struct { present bool }\nfunc %sCurrentTenantKey() %s { return %s{present:true} }\n", c.keyType, p, c.keyType, c.keyType)
			} else {
				fmt.Fprintf(b, "type %s %s\n", c.input, c.scalar)
			}
			opaqueType(b, c.input)
			if !c.tenant && c.scalar == "string" {
				for i, v := range c.column.EnumValues {
					fmt.Fprintf(b, "const %sChoice%d %s = %q\n", c.input, i+1, c.input, v)
				}
			}
		}
		if c.column.Forbidden {
			continue
		}
		fmt.Fprintf(b, "type %s struct { valid bool }\n", c.columnType)
		opaqueType(b, c.columnType)
		fmt.Fprintf(b, `func (c %s) predicate(op string, value any, ref string) %sPredicate {
 if !c.valid { return %sPredicate{} }
 return %sPredicate{filter:operation.FilterSpec{Field:%q,Op:op,Value:value,ValueRef:ref}}
}
func (c %s) Asc() %sOrder { if !c.valid{return %sOrder{}}; return %sOrder{order:operation.OrderSpec{Field:%q,Direction:"asc"}} }
func (c %s) Desc() %sOrder { if !c.valid{return %sOrder{}}; return %sOrder{order:operation.OrderSpec{Field:%q,Direction:"desc"}} }
`, c.columnType, p, p, p, c.column.Name, c.columnType, p, p, p, c.column.Name, c.columnType, p, p, p, c.column.Name)
		if c.tenant {
			fmt.Fprintf(b, "func (c %s) CurrentTenant() %sPredicate { return c.predicate(\"=\",nil,\"current_tenant\") }\n", c.columnType, p)
			continue
		}
		for _, null := range []struct{ name, op string }{{"IsNull", "is_null"}, {"IsNotNull", "is_not_null"}} {
			fmt.Fprintf(b, "func (c %s) %s() %sPredicate { return c.predicate(%q,nil,\"\") }\n", c.columnType, null.name, p, null.op)
		}
		if c.input == "" {
			continue
		}
		ops := []struct{ name, op string }{{"Eq", "="}, {"Ne", "!="}}
		if c.decl.Kind != "bool" {
			ops = append(ops, struct{ name, op string }{"Lt", "<"}, struct{ name, op string }{"Le", "<="}, struct{ name, op string }{"Gt", ">"}, struct{ name, op string }{"Ge", ">="})
		}
		if c.decl.Kind == "string" {
			ops = append(ops, struct{ name, op string }{"Like", "like"})
		}
		for _, op := range ops {
			fmt.Fprintf(b, "func (c %s) %s(value %s) %sPredicate { return c.predicate(%q,%s(value),\"\") }\n", c.columnType, op.name, c.input, p, op.op, c.scalar)
		}
		fmt.Fprintf(b, "func (c %s) In(values ...%s) %sPredicate { args:=make([]any,len(values));for i,v:=range values{args[i]=%s(v)};return c.predicate(\"in\",args,\"\") }\n", c.columnType, c.input, p, c.scalar)
	}
	for _, pr := range t.projections {
		fmt.Fprintf(b, "type %s struct {\n", pr.row)
		for _, c := range pr.columns {
			fmt.Fprintf(b, "%s %s `db:%q`\n", c.field.Name, c.field.Type, c.column.Name)
		}
		b.WriteString("}\n")
		opaqueType(b, pr.row)
		fmt.Fprintf(b, `func (r *%s) Plan%s(ctx context.Context, input %sRead) (*orm.QueryPlan, operation.DiagnosticView, error) {
 return r.db.CompileOperationWithDiagnostics(ctx,input.spec(%s),operation.Options{Manifest:%s()})
}
func (r *%s) Select%s(ctx context.Context, input %sRead) ([]%s,error) {
 return orm.SelectOperationBy[%s](ctx,r.db,input.spec(%s),operation.Options{Manifest:%s()})
}
`, t.repo, pr.name, p, quotedColumns(pr.columns), snapshotName, t.repo, pr.name, p, pr.row, pr.row, quotedColumns(pr.columns), snapshotName)
	}
	if len(t.keys) == 0 {
		return
	}
	if len(t.keys) != 1 || t.keys[0].tenant {
		fmt.Fprintf(b, "type %sKey struct {\n", p)
		for _, c := range t.keys {
			fmt.Fprintf(b, "%s %s\n", c.field.Name, c.keyType)
		}
		b.WriteString("}\n")
		opaqueType(b, p+"Key")
	}
	fmt.Fprintf(b, "func %sKeyInput(key %sKey, input %sRead) %sRead { input.Filters=append([]%sPredicate(nil),input.Filters...);cols:=%sColumns()\n", privatePrefix(p), p, p, p, p, p)
	for _, c := range t.keys {
		keyValue := "key." + c.field.Name
		if len(t.keys) == 1 && !c.tenant {
			keyValue = "key"
		}
		if c.tenant {
			fmt.Fprintf(b, "if %s.present {input.Filters=append(input.Filters,cols.%s.CurrentTenant())}else{input.Filters=append(input.Filters,%sPredicate{})}\n", keyValue, c.field.Name, p)
		} else {
			fmt.Fprintf(b, "input.Filters=append(input.Filters,cols.%s.Eq(%s))\n", c.field.Name, keyValue)
		}
	}
	b.WriteString("return input }\n")
	var selected []typedColumn
	for _, c := range t.columns {
		if !c.column.Forbidden {
			selected = append(selected, c)
		}
	}
	fmt.Fprintf(b, `func (r *%s) PlanFindByKey(ctx context.Context,key %sKey,input %sRead) (*orm.QueryPlan,operation.DiagnosticView,error) {
 input=%sKeyInput(key,input)
 return r.db.CompileOperationWithDiagnostics(ctx,input.spec(%s),operation.Options{Manifest:%s()})
}
func (r *%s) FindByKey(ctx context.Context,key %sKey,input %sRead) (%s,error) {
 input=%sKeyInput(key,input)
 rows,err:=orm.SelectOperationBy[%s](ctx,r.db,input.spec(%s),operation.Options{Manifest:%s()})
 if err!=nil{return %s{},err};if len(rows)==0{return %s{},orm.ErrNotFound};return rows[0],nil
}
`, t.repo, p, p, privatePrefix(p), quotedColumns(selected), snapshotName, t.repo, p, p, t.row, privatePrefix(p), t.row, quotedColumns(selected), snapshotName, t.row, t.row)
}

func privatePrefix(s string) string { r := []rune(s); r[0] = unicode.ToLower(r[0]); return string(r) }
