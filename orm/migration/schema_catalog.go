package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// All statements in this file are catalog SELECTs. Unknown visibility is not
// replaced by an empty complete inventory, even for an administrative reader.
func readExtendedCatalog(ctx context.Context, exec StatusExecutor, cfg SchemaReadOptions, tables map[string]*TableSchema, dialect string) (Schema, error) {
	c := &SchemaContext{Dialect: dialect, Scope: "namespace", Visibility: "catalog_visible", VersionSource: "catalog_select", Coverage: SchemaCoverage{Tables: CoveragePartial, Identity: CoverageComplete, Version: CoverageComplete}}
	if len(cfg.Tables) > 0 {
		c.Scope = "tables"
		c.Tables = slices.Clone(cfg.Tables)
	}
	var rows *sql.Rows
	var err error
	if dialect == "postgres" {
		name := cfg.Schema
		if name == "" {
			name = "public"
		}
		rows, err = exec.QueryContext(ctx, `SELECT current_database(), nspname, current_setting('server_version') FROM pg_namespace WHERE nspname = $1`, name)
	} else {
		rows, err = exec.QueryContext(ctx, `SELECT catalog_name, schema_name, VERSION() FROM information_schema.schemata WHERE schema_name = IF(? = '', DATABASE(), ?)`, cfg.Schema, cfg.Schema)
	}
	if err != nil {
		return Schema{}, err
	}
	n := 0
	for rows.Next() {
		n++
		if err = rows.Scan(&c.Catalog, &c.Schema, &c.ServerVersion); err != nil {
			rows.Close()
			return Schema{}, err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Schema{}, err
	}
	if n != 1 {
		return Schema{}, fmt.Errorf("goquent: schema identity unavailable")
	}
	c.observedVersion = c.ServerVersion
	// Column catalogs can omit inaccessible objects. Successful queries establish
	// observed definitions, not absence or complete namespace visibility.
	for _, t := range tables {
		t.Identity = ObjectIdentity{c.Catalog, c.Schema, t.Name, CoverageComplete}
		t.Coverage = SchemaCoverage{Columns: CoveragePartial, Indexes: CoveragePartial, ForeignKeys: CoveragePartial, Checks: CoveragePartial, Identity: CoverageComplete}
	}
	if dialect == "postgres" {
		err = readPostgresDetails(ctx, exec, c, tables, cfg.tableSet())
	} else {
		err = readMySQLDetails(ctx, exec, c, tables, cfg.tableSet())
	}
	if err != nil {
		return Schema{}, err
	}
	c.Coverage.Columns = CoveragePartial
	c.Coverage.Indexes = CoveragePartial
	c.Coverage.ForeignKeys = CoveragePartial
	c.Coverage.Checks = CoveragePartial
	if dialect == "postgres" {
		c.Coverage.Indexes = CoverageComplete
		c.Coverage.ForeignKeys = CoverageComplete
		c.Coverage.Checks = CoverageComplete
	}
	out := schemaFromTableMap(tables)
	out.Context = c
	return out, nil
}

func readPostgresDetails(ctx context.Context, exec StatusExecutor, c *SchemaContext, tables map[string]*TableSchema, allowed map[string]struct{}) error {
	rows, err := exec.QueryContext(ctx, `SELECT t.relname, con.conname, con.contype::text,
 COALESCE((SELECT json_agg(a.attname ORDER BY k.ord)::text FROM unnest(con.conkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.num),'[]'),
 COALESCE(rn.nspname,''), COALESCE(rt.relname,''),
 COALESCE((SELECT json_agg(a.attname ORDER BY k.ord)::text FROM unnest(con.confkey) WITH ORDINALITY k(num,ord) JOIN pg_attribute a ON a.attrelid=rt.oid AND a.attnum=k.num),'[]'),
 con.confupdtype::text, con.confdeltype::text, con.confmatchtype::text, con.convalidated, con.condeferrable, con.condeferred,
 COALESCE(pg_get_expr(con.conbin,con.conrelid),''), pg_get_constraintdef(con.oid), con.conislocal, con.coninhcount, con.connoinherit
 FROM pg_constraint con JOIN pg_class t ON t.oid=con.conrelid JOIN pg_namespace n ON n.oid=t.relnamespace
 LEFT JOIN pg_class rt ON rt.oid=con.confrelid LEFT JOIN pg_namespace rn ON rn.oid=rt.relnamespace
 WHERE n.nspname=$1 AND con.contype IN ('f','c') ORDER BY t.relname,con.conname`, c.Schema)
	if err != nil {
		return err
	}
	for rows.Next() {
		var table, name, kind, local, rs, rt, ref, up, del, match, expr, raw string
		var validated, deferrable, deferred, islocal, noinherit bool
		var inherited int
		if err = rows.Scan(&table, &name, &kind, &local, &rs, &rt, &ref, &up, &del, &match, &validated, &deferrable, &deferred, &expr, &raw, &islocal, &inherited, &noinherit); err != nil {
			rows.Close()
			return err
		}
		if !tableIncluded(table, allowed) {
			continue
		}
		t := tables[table]
		if t == nil {
			continue
		}
		state := CoverageComplete
		if !islocal || inherited != 0 {
			state = CoverageUnsupported
		}
		validation := "not_validated"
		if validated {
			validation = "validated"
		}
		if kind == "c" {
			inheritance := "inheritable"
			if noinherit {
				inheritance = "no_inherit"
			}
			t.Checks = append(t.Checks, CheckSchema{Inheritance: inheritance, Name: name, Expression: expr, Validation: validation, Enforcement: "enforced", Definition: state, RawDefinition: raw})
			continue
		}
		f := ForeignKeySchema{Name: name, Reference: ObjectIdentity{c.Catalog, rs, rt, CoverageComplete}, OnUpdate: pgAction(up), OnDelete: pgAction(del), Match: map[string]string{"s": "simple", "f": "full", "p": "partial"}[match], Validation: validation, Enforcement: "enforced", Deferrability: "not_deferrable", Definition: state, RawDefinition: raw}
		if json.Unmarshal([]byte(local), &f.Columns) != nil || json.Unmarshal([]byte(ref), &f.ReferencedColumns) != nil {
			rows.Close()
			return fmt.Errorf("goquent: invalid catalog array")
		}
		if deferrable {
			f.Deferrability = "immediate"
			if deferred {
				f.Deferrability = "deferred"
			}
		}
		t.ForeignKeys = append(t.ForeignKeys, f)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = exec.QueryContext(ctx, `SELECT t.relname,i.relname,ix.indisunique,pg_get_indexdef(i.oid),am.amname,
 COALESCE(pg_get_expr(ix.indpred,t.oid),''),k.ord,COALESCE(a.attname,''),pg_get_indexdef(i.oid,k.ord::int,false),
 ix.indnkeyatts,COALESCE(ix.indoption[k.ord::int-1],0),ix.indisvalid,ix.indisready,
 COALESCE(op.opcdefault,false),COALESCE(ix.indcollation[k.ord::int-1],0)=COALESCE(a.attcollation,0),
 ix.indnullsnotdistinct,(i.reloptions IS NULL AND i.reltablespace=0 AND NOT ix.indisexclusion),t.relkind::text
 FROM pg_class t JOIN pg_namespace n ON n.oid=t.relnamespace JOIN pg_index ix ON ix.indrelid=t.oid
 JOIN pg_class i ON i.oid=ix.indexrelid JOIN pg_am am ON am.oid=i.relam
 CROSS JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY k(num,ord)
 LEFT JOIN pg_attribute a ON a.attrelid=t.oid AND a.attnum=k.num
 LEFT JOIN pg_opclass op ON op.oid=ix.indclass[k.ord::int-1]
 WHERE n.nspname=$1 ORDER BY t.relname,i.relname,k.ord`, c.Schema)
	if err != nil {
		return err
	}
	pending := map[[2]string]*IndexSchema{}
	for rows.Next() {
		var table, name, raw, method, predicate, col, expr, relkind string
		var unique, valid, ready, defaultop, samecoll, nullsdist, noopts bool
		var ord, nkeys, options int
		if err = rows.Scan(&table, &name, &unique, &raw, &method, &predicate, &ord, &col, &expr, &nkeys, &options, &valid, &ready, &defaultop, &samecoll, &nullsdist, &noopts, &relkind); err != nil {
			rows.Close()
			return err
		}
		if !tableIncluded(table, allowed) || tables[table] == nil {
			continue
		}
		key := [2]string{table, name}
		idx := pending[key]
		if idx == nil {
			idx = &IndexSchema{Name: name, Unique: unique, RawDefinition: raw, Method: method, Visibility: "visible", Attributes: CoverageComplete, Predicate: ExpressionDefinition{State: "none"}}
			if predicate != "" {
				idx.Predicate = ExpressionDefinition{"expression", predicate}
			}
			pending[key] = idx
		}
		if !valid || !ready || method != "btree" || nullsdist || !noopts || relkind != "r" {
			idx.DefinitionUnknown = true
		}
		if ord > nkeys {
			idx.Include = append(idx.Include, col)
			if col == "" {
				idx.DefinitionUnknown = true
			}
			continue
		}
		k := IndexKey{Column: col, Order: "asc", Nulls: "last"}
		if col == "" {
			k.Expression = expr
		}
		if options&1 != 0 {
			k.Order = "desc"
		}
		if options&2 != 0 {
			k.Nulls = "first"
		}
		if options&^3 != 0 || !defaultop || !samecoll {
			idx.DefinitionUnknown = true
		}
		// Expression collations are not inferred from SQL text.
		if ord != len(idx.Keys)+1 {
			idx.DefinitionUnknown = true
		}
		idx.Keys = append(idx.Keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for key, idx := range pending {
		all := true
		for _, k := range idx.Keys {
			if k.Column == "" {
				all = false
			}
		}
		if all {
			for _, k := range idx.Keys {
				idx.Columns = append(idx.Columns, k.Column)
			}
		}
		tables[key[0]].Indexes = append(tables[key[0]].Indexes, *idx)
	}
	for _, t := range tables {
		sortIndexes(t)
		t.Coverage.Indexes = CoverageComplete
		t.Coverage.ForeignKeys = CoverageComplete
		t.Coverage.Checks = CoverageComplete
	}
	return nil
}
func pgAction(s string) string {
	return map[string]string{"a": "no_action", "r": "restrict", "c": "cascade", "n": "set_null", "d": "set_default"}[s]
}
func sortIndexes(t *TableSchema) {
	slices.SortFunc(t.Indexes, func(a, b IndexSchema) int { return strings.Compare(a.Name, b.Name) })
}

func readMySQLDetails(ctx context.Context, exec StatusExecutor, c *SchemaContext, tables map[string]*TableSchema, allowed map[string]struct{}) error {
	rows, err := exec.QueryContext(ctx, `SELECT k.table_name,k.constraint_name,k.column_name,k.ordinal_position,k.referenced_table_schema,k.referenced_table_name,k.referenced_column_name,r.update_rule,r.delete_rule,r.match_option
 FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r
 ON r.constraint_schema=k.constraint_schema AND r.table_name=k.table_name AND r.constraint_name=k.constraint_name
 WHERE k.table_schema=? AND k.referenced_table_name IS NOT NULL ORDER BY k.table_name,k.constraint_name,k.ordinal_position`, c.Schema)
	if err != nil {
		return err
	}
	pending := map[[2]string]*ForeignKeySchema{}
	for rows.Next() {
		var table, name, col, rs, rt, rc, up, del, match string
		var ord int
		if err = rows.Scan(&table, &name, &col, &ord, &rs, &rt, &rc, &up, &del, &match); err != nil {
			rows.Close()
			return err
		}
		if !tableIncluded(table, allowed) || tables[table] == nil {
			continue
		}
		key := [2]string{table, name}
		f := pending[key]
		if f == nil {
			f = &ForeignKeySchema{Name: name, Reference: ObjectIdentity{c.Catalog, rs, rt, CoverageComplete}, OnUpdate: mysqlAction(up), OnDelete: mysqlAction(del), Match: "simple", Validation: "unknown", Enforcement: "enforced", Deferrability: "not_deferrable", Definition: CoveragePartial}
			pending[key] = f
		}
		// MySQL metadata does not prove rows were validated (foreign_key_checks may
		// have been disabled). Preserve this limitation rather than inventing validity.
		if match != "NONE" || f.Reference.Schema != rs || f.Reference.Name != rt || ord != len(f.Columns)+1 || f.OnUpdate != mysqlAction(up) || f.OnDelete != mysqlAction(del) {
			f.Definition = CoverageUnknown
		}
		f.Columns = append(f.Columns, col)
		f.ReferencedColumns = append(f.ReferencedColumns, rc)
		raw, _ := json.Marshal([]any{col, ord, rs, rt, rc, up, del, match})
		f.RawDefinition += string(raw) + "\n"
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for key, f := range pending {
		tables[key[0]].ForeignKeys = append(tables[key[0]].ForeignKeys, *f)
	}
	rows, err = exec.QueryContext(ctx, `SELECT tc.table_name,tc.constraint_name,cc.check_clause,tc.enforced FROM information_schema.table_constraints tc JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name WHERE tc.table_schema=? AND tc.constraint_type='CHECK' ORDER BY tc.table_name,tc.constraint_name`, c.Schema)
	if err != nil {
		return err
	}
	for rows.Next() {
		var table, name, expr, enforced string
		if err = rows.Scan(&table, &name, &expr, &enforced); err != nil {
			rows.Close()
			return err
		}
		if !tableIncluded(table, allowed) || tables[table] == nil {
			continue
		}
		v := CheckSchema{Inheritance: "not_applicable", Name: name, Expression: expr, RawDefinition: expr, Validation: "unknown", Enforcement: "unknown", Definition: CoveragePartial}
		if enforced == "YES" {
			v.Enforcement = "enforced"
		}
		if enforced == "NO" {
			v.Enforcement = "not_enforced"
		}
		tables[table].Checks = append(tables[table].Checks, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = exec.QueryContext(ctx, `SELECT table_name,index_name,non_unique,seq_in_index,column_name,sub_part,collation,index_type,is_visible,expression FROM information_schema.statistics WHERE table_schema=? ORDER BY table_name,index_name,seq_in_index`, c.Schema)
	if err != nil {
		return err
	}
	indexes := map[[2]string]*IndexSchema{}
	for rows.Next() {
		var table, name, method, visible string
		var col, expr, order sql.NullString
		var prefix sql.NullInt64
		var nonunique, ord int
		if err = rows.Scan(&table, &name, &nonunique, &ord, &col, &prefix, &order, &method, &visible, &expr); err != nil {
			rows.Close()
			return err
		}
		if !tableIncluded(table, allowed) || tables[table] == nil {
			continue
		}
		key := [2]string{table, name}
		idx := indexes[key]
		if idx == nil {
			idx = &IndexSchema{Name: name, Unique: nonunique == 0, Method: strings.ToLower(method), Visibility: map[string]string{"YES": "visible", "NO": "invisible"}[visible], Attributes: CoverageComplete, Predicate: ExpressionDefinition{State: "none"}}
			indexes[key] = idx
		}
		k := IndexKey{Column: col.String, Expression: expr.String, Order: map[string]string{"A": "asc", "D": "desc"}[order.String], Nulls: "engine_default", Prefix: int(prefix.Int64)}
		if method != "BTREE" || !order.Valid || idx.Visibility == "" || ord != len(idx.Keys)+1 || idx.Unique != (nonunique == 0) || idx.Method != strings.ToLower(method) {
			idx.DefinitionUnknown = true
		}
		raw, _ := json.Marshal([]any{col, expr, prefix, order, method, visible, ord, nonunique})
		idx.RawDefinition += string(raw) + "\n"
		idx.Keys = append(idx.Keys, k)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for key, idx := range indexes {
		all := true
		for _, k := range idx.Keys {
			if k.Column == "" {
				all = false
			}
		}
		if all {
			for _, k := range idx.Keys {
				idx.Columns = append(idx.Columns, k.Column)
			}
		}
		tables[key[0]].Indexes = append(tables[key[0]].Indexes, *idx)
	}
	for _, t := range tables {
		sortIndexes(t)
		slices.SortFunc(t.ForeignKeys, func(a, b ForeignKeySchema) int { return strings.Compare(a.Name, b.Name) })
	}
	return nil
}
func mysqlAction(s string) string {
	return map[string]string{"NO ACTION": "no_action", "RESTRICT": "restrict", "CASCADE": "cascade", "SET NULL": "set_null", "SET DEFAULT": "set_default"}[s]
}
