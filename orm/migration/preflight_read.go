package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"strings"
)

func detachPreflight(p *Preflight) (*Preflight, error) {
	b, e := json.Marshal(p)
	if e != nil {
		return nil, ErrPreflightInput
	}
	var out Preflight
	if json.Unmarshal(b, &out) != nil {
		return nil, ErrPreflightInput
	}
	return &out, nil
}

// ReadSchema consumes each result before requesting another. Cancel the previous
// per-query context on the next request and always cancel the last on return.
type boundedSchemaReader struct {
	c      *Collector
	cancel context.CancelFunc
}

func (r *boundedSchemaReader) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if r.cancel != nil {
		r.cancel()
	}
	ctx, r.cancel = context.WithTimeout(ctx, r.c.queryTimeout)
	return r.c.conn.QueryContext(ctx, q, args...)
}
func (c *Collector) observeSchemas(ctx context.Context, p PreflightPlan) (string, error) {
	reader := &boundedSchemaReader{c: c}
	defer func() {
		if reader.cancel != nil {
			reader.cancel()
		}
	}()
	observed := p
	all := append([]Schema{p.Current}, p.References...)
	for i, s := range all {
		opts := []SchemaReadOption{WithSchemaReadSchema(s.Context.Schema)}
		if s.Context.Scope == "tables" {
			opts = append(opts, WithSchemaReadTables(s.Context.Tables...))
		}
		v, e := ReadSchema(ctx, reader, c.dialect, opts...)
		if e != nil || canonicalSchema(&v) != nil {
			return "", ErrPreflightRead
		}
		if i == 0 {
			observed.Current = v
		} else {
			if i == 1 {
				observed.References = make([]Schema, len(p.References))
			}
			observed.References[i-1] = v
		}
	}
	return schemaFingerprint(observed), nil
}
func (c *Collector) count(ctx context.Context, q string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	rows, e := c.conn.QueryContext(ctx, q)
	if e != nil {
		return 0, ErrPreflightRead
	}
	defer rows.Close()
	var n int64
	if !rows.Next() {
		return 0, ErrPreflightRead
	}
	if rows.Scan(&n) != nil || n < 0 {
		return 0, ErrPreflightRead
	}
	if rows.Next() || rows.Err() != nil || ctx.Err() != nil {
		return 0, ErrPreflightRead
	}
	return n, nil
}
func quotePreflight(dialect, name string) string {
	if dialect == "mysql" {
		return "`" + name + "`"
	}
	return `"` + name + `"`
}
func preflightSQL(dialect string, r PreflightRequirement) (string, error) {
	if (dialect != "mysql" && dialect != "postgres") || !identityKnown(r.Target) || len(r.Columns) == 0 || len(r.Columns) > 16 {
		return "", ErrPreflightInput
	}
	names := func(cols []string, alias string) ([]string, bool) {
		out := []string{}
		seen := map[string]bool{}
		for _, col := range cols {
			if !simpleSchemaName.MatchString(col) || seen[col] {
				return nil, false
			}
			seen[col] = true
			out = append(out, alias+quotePreflight(dialect, col))
		}
		return out, true
	}
	cols, ok := names(r.Columns, "l.")
	if !ok {
		return "", ErrPreflightInput
	}
	table := quotePreflight(dialect, r.Target.Schema) + "." + quotePreflight(dialect, r.Target.Name)
	nn := []string{}
	for _, col := range cols {
		nn = append(nn, col+" IS NOT NULL")
	}
	switch r.Kind {
	case "null_count":
		if len(cols) != 1 || r.Semantics != "null_rows" {
			break
		}
		return "SELECT COUNT(*) FROM " + table + " AS l WHERE " + cols[0] + " IS NULL", nil
	case "duplicate_count":
		if r.Semantics != "duplicate_groups_nonnull" {
			break
		}
		return "SELECT COUNT(*) FROM (SELECT " + strings.Join(cols, ",") + " FROM " + table + " AS l WHERE " + strings.Join(nn, " AND ") + " GROUP BY " + strings.Join(cols, ",") + " HAVING COUNT(*) > 1) AS duplicate_groups", nil
	case "orphan_count":
		if r.Semantics != "match_simple_nonnull_rows" || !identityKnown(r.Reference) || r.Reference.Catalog != r.Target.Catalog || len(r.ReferencedColumns) != len(cols) || len(r.ReferencedColumns) > 16 {
			break
		}
		refs, ok := names(r.ReferencedColumns, "r.")
		if !ok {
			break
		}
		pairs := []string{}
		for i := range cols {
			pairs = append(pairs, refs[i]+" = "+cols[i])
		}
		rt := quotePreflight(dialect, r.Reference.Schema) + "." + quotePreflight(dialect, r.Reference.Name)
		return "SELECT COUNT(*) FROM " + table + " AS l WHERE " + strings.Join(nn, " AND ") + " AND NOT EXISTS (SELECT 1 FROM " + rt + " AS r WHERE " + strings.Join(pairs, " AND ") + ")", nil
	}
	return "", ErrPreflightInput
}
func (c *Collector) checkTarget(ctx context.Context, p PreflightPlan, r PreflightRequirement) ([]PreflightTargetObservation, string, string, error) {
	var observations []PreflightTargetObservation

	version := p.Current.Context.ServerVersion
	if (c.name == "postgres" && !pg16Version.MatchString(version)) || (c.name == "mysql" && !mysql84Version.MatchString(version)) {
		return observations, "unknown", "PREFLIGHT_DEFINITION", nil
	}
	local, ok := scopedTable(p, r.Target)
	if !ok || !columnsOK(r.Columns, local) {
		return observations, "unknown", "PREFLIGHT_SCOPE", nil
	}
	types, status, err := c.inspectTable(ctx, r.Target, r.Columns)
	observations = append(observations, PreflightTargetObservation{Target: r.Target, Columns: slices.Clone(r.Columns), Types: types, Coverage: status})
	if err != nil {
		return observations, "unknown", "PREFLIGHT_FAILED", err
	}
	if status != "satisfied" {
		return observations, status, "PREFLIGHT_VISIBILITY", nil
	}
	for _, typ := range types {
		if typ == "unsupported" {
			return observations, "unsupported", "PREFLIGHT_UNSUPPORTED", nil
		}
	}
	if r.Kind != "null_count" {
		for _, typ := range types {
			if !finiteInteger(c.name, typ) {
				return observations, "unsupported", "PREFLIGHT_UNSUPPORTED", nil
			}
		}
	}
	if r.Kind == "orphan_count" {
		ref, ok := scopedTable(p, r.Reference)
		if !ok || !columnsOK(r.ReferencedColumns, ref) {
			return observations, "unknown", "PREFLIGHT_SCOPE", nil
		}
		rt, status, err := c.inspectTable(ctx, r.Reference, r.ReferencedColumns)
		observations = append(observations, PreflightTargetObservation{Target: r.Reference, Columns: slices.Clone(r.ReferencedColumns), Types: rt, Coverage: status})
		if err != nil {
			return observations, "unknown", "PREFLIGHT_FAILED", err
		}
		if status != "satisfied" {
			return observations, status, "PREFLIGHT_VISIBILITY", nil
		}
		if !slices.Equal(types, rt) {
			return observations, "unsupported", "PREFLIGHT_UNSUPPORTED", nil
		}
	}
	return observations, "satisfied", "PREFLIGHT_OK", nil
}
func scopedTable(p PreflightPlan, id ObjectIdentity) (TableSchema, bool) {
	for _, s := range append([]Schema{p.Current}, p.References...) {
		for _, t := range s.Tables {
			if t.Identity == id && inScope(t, s.Context) {
				return t, true
			}
		}
	}
	return TableSchema{}, false
}
func finiteInteger(d, t string) bool {
	if d == "postgres" {
		return t == "int2" || t == "int4" || t == "int8"
	}
	switch t {
	case "tinyint", "smallint", "mediumint", "int", "bigint", "tinyint unsigned", "smallint unsigned", "mediumint unsigned", "int unsigned", "bigint unsigned":
		return true
	}
	return false
}

// inspectTable verifies base-table shape and row visibility, and observes exact
// integer type semantics. RLS is refused even for bypass roles. No role or session
// settings are changed. MySQL's supported InnoDB base-table subset has no RLS;
// the eventual SELECT must still prove column read permission or fail.
func (c *Collector) inspectTable(ctx context.Context, id ObjectIdentity, cols []string) ([]string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.queryTimeout)
	defer cancel()
	var q string
	var args []any
	if c.name == "postgres" {
		q = `SELECT a.attname, ty.typname, c.relkind::text, c.relrowsecurity,
 EXISTS (SELECT 1 FROM pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid),
 has_table_privilege(c.oid, 'SELECT'), a.attcollation::bigint, a.attgenerated::text,
 tn.nspname
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 JOIN pg_attribute a ON a.attrelid=c.oid JOIN pg_type ty ON ty.oid=a.atttypid
 JOIN pg_namespace tn ON tn.oid=ty.typnamespace
 WHERE n.nspname=$1 AND c.relname=$2 AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`
		args = []any{id.Schema, id.Name}
	} else {
		q = `SELECT c.column_name,c.column_type,t.table_type,COALESCE(t.engine,''),c.extra,
 c.collation_name,EXISTS(SELECT 1 FROM information_schema.partitions p
 WHERE p.table_schema=t.table_schema AND p.table_name=t.table_name AND p.partition_name IS NOT NULL)
 FROM information_schema.tables t JOIN information_schema.columns c
 ON c.table_schema=t.table_schema AND c.table_name=t.table_name
 WHERE t.table_schema=? AND t.table_name=? ORDER BY c.ordinal_position`
		args = []any{id.Schema, id.Name}
	}
	rows, e := c.conn.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, "unknown", ErrPreflightRead
	}
	defer rows.Close()
	found := map[string]string{}
	status := "satisfied"
	for rows.Next() {
		var name, typ string
		if c.name == "postgres" {
			var kind, generated, ns string
			var rls, inherits, priv bool
			var collation int64
			if rows.Scan(&name, &typ, &kind, &rls, &inherits, &priv, &collation, &generated, &ns) != nil {
				return nil, "unknown", ErrPreflightRead
			}
			if kind != "r" || inherits {
				status = "unsupported"
			}
			if rls || !priv {
				status = "unknown"
			}
			if slices.Contains(cols, name) && (collation != 0 || generated != "" || ns != "pg_catalog") {
				typ = "unsupported"
			}
		} else {
			var kind, engine, extra string
			var collation sql.NullString
			var partition bool
			if rows.Scan(&name, &typ, &kind, &engine, &extra, &collation, &partition) != nil {
				return nil, "unknown", ErrPreflightRead
			}
			if kind != "BASE TABLE" || engine != "InnoDB" || partition {
				status = "unsupported"
			}
			if slices.Contains(cols, name) && (collation.Valid || extra != "") {
				typ = "unsupported"
			}
		}
		if _, exists := found[name]; exists {
			return nil, "unknown", ErrPreflightRead
		}
		found[name] = typ
	}
	if rows.Err() != nil || ctx.Err() != nil {
		return nil, "unknown", ErrPreflightRead
	}
	out := []string{}
	for _, col := range cols {
		typ, ok := found[col]
		if !ok {
			return nil, "unknown", nil
		}
		out = append(out, typ)
	}
	return out, status, nil
}
