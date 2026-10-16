package migration

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/driver"
)

// SchemaReadOptions configures live database schema export.
type SchemaReadOptions struct {
	Schema string
	Tables []string
}

// SchemaReadOption configures ReadSchema.
type SchemaReadOption func(*SchemaReadOptions)

// WithSchemaReadSchema sets the database schema/catalog to inspect. PostgreSQL
// defaults to public. MySQL defaults to DATABASE().
func WithSchemaReadSchema(schema string) SchemaReadOption {
	return func(o *SchemaReadOptions) { o.Schema = schema }
}

// WithSchemaReadTables limits export to the named tables.
func WithSchemaReadTables(tables ...string) SchemaReadOption {
	return func(o *SchemaReadOptions) { o.Tables = append(o.Tables, tables...) }
}

// ReadSchema exports a minimal migration.Schema from database metadata.
func ReadSchema(ctx context.Context, exec StatusExecutor, dialect driver.Dialect, opts ...SchemaReadOption) (Schema, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if exec == nil {
		return Schema{}, fmt.Errorf("goquent: schema export executor is required")
	}
	if dialect == nil {
		return Schema{}, fmt.Errorf("goquent: schema export dialect is required")
	}
	cfg := SchemaReadOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.normalize()
	switch dialect.(type) {
	case driver.PostgresDialect:
		return readPostgresSchema(ctx, exec, cfg)
	case driver.MySQLDialect:
		return readMySQLSchema(ctx, exec, cfg)
	default:
		return Schema{}, fmt.Errorf("goquent: schema export is not supported on dialect: %T", dialect)
	}
}

func (o *SchemaReadOptions) normalize() {
	o.Schema = strings.TrimSpace(o.Schema)
	out := make([]string, 0, len(o.Tables))
	seen := map[string]struct{}{}
	for _, table := range o.Tables {
		table = strings.TrimSpace(table)
		if table == "" {
			continue
		}
		key := tableKey(table)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, table)
	}
	o.Tables = out
}

func readPostgresSchema(ctx context.Context, exec StatusExecutor, cfg SchemaReadOptions) (Schema, error) {
	schemaName := cfg.Schema
	if schemaName == "" {
		schemaName = "public"
	}
	rows, err := exec.QueryContext(ctx, `
SELECT table_name, column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = $1
ORDER BY table_name, ordinal_position`, schemaName)
	if err != nil {
		return Schema{}, err
	}
	tables, err := scanSchemaColumns(rows, cfg.tableSet(), "postgres")
	if err != nil {
		return Schema{}, err
	}

	indexRows, err := exec.QueryContext(ctx, `
SELECT t.relname AS table_name, i.relname AS index_name, ix.indisunique, pg_get_indexdef(ix.indexrelid)
FROM pg_class t
JOIN pg_index ix ON t.oid = ix.indrelid
JOIN pg_class i ON i.oid = ix.indexrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = $1
ORDER BY t.relname, i.relname`, schemaName)
	if err != nil {
		return Schema{}, err
	}
	if err := scanPostgresIndexes(indexRows, tables, cfg.tableSet()); err != nil {
		return Schema{}, err
	}
	return schemaFromTableMap(tables), nil
}

func readMySQLSchema(ctx context.Context, exec StatusExecutor, cfg SchemaReadOptions) (Schema, error) {
	rows, err := exec.QueryContext(ctx, `
SELECT table_name, column_name, column_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = IF(? = '', DATABASE(), ?)
ORDER BY table_name, ordinal_position`, cfg.Schema, cfg.Schema)
	if err != nil {
		return Schema{}, err
	}
	tables, err := scanSchemaColumns(rows, cfg.tableSet(), "mysql")
	if err != nil {
		return Schema{}, err
	}

	indexRows, err := exec.QueryContext(ctx, `
SELECT table_name, index_name, non_unique, seq_in_index, column_name, sub_part, collation, index_type, is_visible
FROM information_schema.statistics
WHERE table_schema = IF(? = '', DATABASE(), ?)
ORDER BY table_name, index_name, seq_in_index`, cfg.Schema, cfg.Schema)
	if err != nil {
		return Schema{}, err
	}
	if err := scanMySQLIndexes(indexRows, tables, cfg.tableSet()); err != nil {
		return Schema{}, err
	}
	return schemaFromTableMap(tables), nil
}

func (o SchemaReadOptions) tableSet() map[string]struct{} {
	if len(o.Tables) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(o.Tables))
	for _, table := range o.Tables {
		out[tableKey(table)] = struct{}{}
	}
	return out
}

func tableIncluded(table string, set map[string]struct{}) bool {
	if len(set) == 0 {
		return true
	}
	_, ok := set[tableKey(table)]
	return ok
}

func scanSchemaColumns(rows *sql.Rows, allowed map[string]struct{}, dialect ...string) (map[string]*TableSchema, error) {
	defer rows.Close()
	tables := map[string]*TableSchema{}
	for rows.Next() {
		var tableName, columnName, columnType, nullableText string
		var defaultRaw any
		if err := rows.Scan(&tableName, &columnName, &columnType, &nullableText, &defaultRaw); err != nil {
			return nil, err
		}
		if !tableIncluded(tableName, allowed) {
			continue
		}
		table := ensureSchemaTable(tables, tableName)
		defaultExpr := schemaString(defaultRaw)
		state, source := DefaultUnknown, ""
		if len(dialect) > 0 {
			source = dialect[0] + ".information_schema.columns"
		}
		// PostgreSQL column_default is an SQL expression. MySQL catalog values
		// are not SQL literals; preserve them without reconstructing quotation.
		if len(dialect) > 0 && dialect[0] == "postgres" && defaultRaw != nil && defaultExpr != "" {
			state = DefaultExpression
		}
		if len(dialect) > 0 && dialect[0] == "mysql" && mysqlIntegerType.MatchString(columnType) && mysqlIntegerDefault.MatchString(defaultExpr) {
			state = DefaultExpression
		}

		table.Columns = append(table.Columns, ColumnSchema{
			Name:              columnName,
			Type:              columnType,
			Nullable:          strings.EqualFold(nullableText, "yes"),
			NullableKnown:     strings.EqualFold(nullableText, "yes") || strings.EqualFold(nullableText, "no"),
			HasDefault:        defaultRaw != nil,
			DefaultExpression: defaultExpr,
			DefaultState:      state, DefaultSource: source,
		})
	}
	return tables, rows.Err()
}

func scanPostgresIndexes(rows *sql.Rows, tables map[string]*TableSchema, allowed map[string]struct{}) error {
	defer rows.Close()
	for rows.Next() {
		var tableName, indexName, indexDef string
		var unique bool
		if err := rows.Scan(&tableName, &indexName, &unique, &indexDef); err != nil {
			return err
		}
		if !tableIncluded(tableName, allowed) {
			continue
		}
		table := ensureSchemaTable(tables, tableName)
		table.Indexes = append(table.Indexes, IndexSchema{
			Name:          indexName,
			Columns:       parsePostgresIndexColumns(indexDef),
			RawDefinition: indexDef, DefinitionUnknown: len(parsePostgresIndexColumns(indexDef)) == 0,
			Unique: unique,
		})
	}
	return rows.Err()
}

func scanMySQLIndexes(rows *sql.Rows, tables map[string]*TableSchema, allowed map[string]struct{}) error {
	defer rows.Close()
	type pendingIndex struct {
		table   string
		name    string
		unique  bool
		columns []string
		unknown bool
	}
	indexes := map[string]*pendingIndex{}
	for rows.Next() {
		var tableName, indexName string
		var columnName, collation sql.NullString
		var prefix sql.NullInt64
		var indexType, visible string
		var nonUnique int
		var seq int
		if err := rows.Scan(&tableName, &indexName, &nonUnique, &seq, &columnName, &prefix, &collation, &indexType, &visible); err != nil {
			return err
		}
		if !tableIncluded(tableName, allowed) {
			continue
		}
		key := tableName + "\x00" + indexName
		idx, ok := indexes[key]
		if !ok {
			idx = &pendingIndex{table: tableName, name: indexName, unique: nonUnique == 0}
			indexes[key] = idx
		}
		if !columnName.Valid || prefix.Valid || !collation.Valid || collation.String != "A" || indexType != "BTREE" || visible != "YES" || seq != len(idx.columns)+1 || idx.unique != (nonUnique == 0) {
			idx.unknown = true
		}
		idx.columns = append(idx.columns, columnName.String)
		ensureSchemaTable(tables, tableName)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	keys := make([]string, 0, len(indexes))
	for key := range indexes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		idx := indexes[key]
		table := ensureSchemaTable(tables, idx.table)
		table.Indexes = append(table.Indexes, IndexSchema{Name: idx.name, Columns: idx.columns, Unique: idx.unique, DefinitionUnknown: idx.unknown})
	}
	return nil
}

func ensureSchemaTable(tables map[string]*TableSchema, name string) *TableSchema {
	key := name
	if table, ok := tables[key]; ok {
		return table
	}
	table := &TableSchema{Name: name}
	tables[key] = table
	return table
}

func schemaFromTableMap(tables map[string]*TableSchema) Schema {
	keys := make([]string, 0, len(tables))
	for key := range tables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := Schema{Tables: make([]TableSchema, 0, len(keys))}
	for _, key := range keys {
		table := *tables[key]
		out.Tables = append(out.Tables, table)
	}
	return out
}

func schemaString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprint(x)
	}
}

// Deliberately bounded: expressions, predicates, INCLUDE, ordering, opclasses,
// quoted names and non-btree definitions remain unknown rather than truncated.
var postgresIndexColumnsRE = regexp.MustCompile(`^CREATE (?:UNIQUE )?INDEX [a-z_][a-z0-9_]* ON (?:[a-z_][a-z0-9_]*\.)?[a-z_][a-z0-9_]* USING btree \(([a-z_][a-z0-9_]*(?:, [a-z_][a-z0-9_]*)*)\)$`)

func parsePostgresIndexColumns(def string) []string {
	m := postgresIndexColumnsRE.FindStringSubmatch(def)
	if len(m) != 2 {
		return nil
	}
	return strings.Split(m[1], ", ")
}

func splitIndexColumnList(s string) []string {
	var out []string
	var b strings.Builder
	depth := 0
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
		case ')':
			if !inSingle && !inDouble && depth > 0 {
				depth--
			}
		case ',':
			if !inSingle && !inDouble && depth == 0 {
				out = append(out, b.String())
				b.Reset()
				continue
			}
		}
		b.WriteByte(ch)
	}
	if strings.TrimSpace(b.String()) != "" {
		out = append(out, b.String())
	}
	return out
}

// MySQL integer column defaults have an unquoted integer representation. Other
// catalog values remain opaque; no quoting or SQL normalization is inferred.
var mysqlIntegerType = regexp.MustCompile(`^(?:tinyint|smallint|mediumint|int|bigint)(?:\([0-9]+\))?(?: unsigned)?$`)
var mysqlIntegerDefault = regexp.MustCompile(`^-?[0-9]+$`)
