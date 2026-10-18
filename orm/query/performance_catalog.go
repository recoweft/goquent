package query

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"

	qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"
)

var performancePGVersion = regexp.MustCompile(`^16\.[0-9]+(?: \([^\r\n]*\))?$`)
var performanceMySQLVersion = regexp.MustCompile(`^8\.4\.[0-9]+$`)

// Catalogs are deliberately narrower than the structural SELECT grammar.
// Integer columns avoid collation, temporal and floating coercion semantics.
// Unsupported combinations remain refused rather than inferred from type names.
const performancePGTable = `SELECT c.oid::pg_catalog.text, c.relkind::pg_catalog.text,
 (c.relam = 2 AND NOT c.relispartition AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
 AND NOT c.relhasrules AND c.relpersistence = 'p' AND c.reloptions IS NULL
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_statistic_ext st WHERE st.stxrelid=c.oid)
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid)
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_constraint x WHERE x.conrelid=c.oid AND x.contype NOT IN ('p','u'))
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class ic ON ic.oid=i.indexrelid
 WHERE i.indrelid=c.oid AND (i.indexprs IS NOT NULL OR i.indpred IS NOT NULL OR NOT i.indisvalid OR NOT i.indisready OR ic.relam<>403))
 )::pg_catalog.text
 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname=$1 AND c.relname=$2`
const performancePGColumns = `SELECT a.attname, a.atttypid::pg_catalog.text,
 (a.attgenerated='' AND a.attcollation=0 AND a.atttypmod=-1 AND t.typtype='b'
 AND t.typnamespace=11 AND t.oid IN (20,21,23)
 AND p.oid<16384 AND p.pronamespace=11 AND p.prolang=12 AND p.prosupport=0
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_cast x WHERE (x.castsource=t.oid OR x.casttarget=t.oid) AND x.oid>=16384)
 )::pg_catalog.text
 FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_type t ON t.oid=a.atttypid
 JOIN pg_catalog.pg_proc p ON p.oid=t.typinput
 WHERE a.attrelid=$1::pg_catalog.oid AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`

// Built-in integer operator identities and all referenced implementation/support
// functions must belong to the bootstrap catalog. Public-schema overloads cannot
// participate after search_path is fixed; operands infer the column's integer type.
const performancePGOperators = `SELECT o.oprname, o.oprleft::pg_catalog.text, o.oprright::pg_catalog.text,
 (o.oid<16384 AND o.oprnamespace=11 AND o.oprresult=16 AND p.oid<16384 AND p.pronamespace=11 AND p.prolang=12 AND p.prosupport=0
 AND (o.oprrest=0 OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc r WHERE r.oid=o.oprrest AND r.oid<16384 AND r.pronamespace=11 AND r.prolang=12 AND r.prosupport=0))
 AND (o.oprjoin=0 OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc j WHERE j.oid=o.oprjoin AND j.oid<16384 AND j.pronamespace=11 AND j.prolang=12 AND j.prosupport=0)))::pg_catalog.text
 FROM pg_catalog.pg_operator o JOIN pg_catalog.pg_proc p ON p.oid=o.oprcode
 WHERE o.oprleft IN (20,21,23) AND o.oprright IN (20,21,23) AND o.oprnamespace=11 AND o.oprname IN ('=','<>','<','<=','>','>=')`
const performancePGIndexes = `SELECT i.indexrelid::pg_catalog.text,
 (NOT EXISTS (SELECT 1 FROM pg_catalog.unnest(i.indclass) AS k(oid)
 LEFT JOIN pg_catalog.pg_opclass oc ON oc.oid=k.oid
 WHERE oc.oid IS NULL OR oc.oid>=16384 OR oc.opcnamespace<>11 OR oc.opcmethod<>403 OR oc.opcintype NOT IN (20,21,23)
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_amproc ap LEFT JOIN pg_catalog.pg_proc p ON p.oid=ap.amproc
 WHERE ap.amprocfamily=oc.opcfamily AND (ap.oid>=16384 OR p.oid IS NULL OR p.oid>=16384 OR p.pronamespace<>11 OR p.prolang<>12 OR p.prosupport<>0))
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_amop ao LEFT JOIN pg_catalog.pg_operator o ON o.oid=ao.amopopr
 LEFT JOIN pg_catalog.pg_proc p ON p.oid=o.oprcode WHERE ao.amopfamily=oc.opcfamily
 AND (ao.oid>=16384 OR o.oid IS NULL OR o.oid>=16384 OR o.oprnamespace<>11 OR p.oid IS NULL OR p.oid>=16384 OR p.pronamespace<>11 OR p.prolang<>12 OR p.prosupport<>0)))
 AND NOT EXISTS (SELECT 1 FROM pg_catalog.unnest(i.indcollation) AS co(oid) WHERE co.oid<>0))::pg_catalog.text
 FROM pg_catalog.pg_index i WHERE i.indrelid=$1::pg_catalog.oid`

// Column-level grants can hide catalog columns. Require a direct SELECT grant
// covering the whole table; role-only grants conservatively remain unsupported.
const performanceMySQLVisibility = `SELECT (
 EXISTS (SELECT 1 FROM information_schema.USER_PRIVILEGES WHERE GRANTEE=CONCAT(QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',1)),'@',QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',-1))) AND PRIVILEGE_TYPE='SELECT')
 OR EXISTS (SELECT 1 FROM information_schema.SCHEMA_PRIVILEGES WHERE GRANTEE=CONCAT(QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',1)),'@',QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',-1))) AND PRIVILEGE_TYPE='SELECT' AND TABLE_SCHEMA=?)
 OR EXISTS (SELECT 1 FROM information_schema.TABLE_PRIVILEGES WHERE GRANTEE=CONCAT(QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',1)),'@',QUOTE(SUBSTRING_INDEX(CURRENT_USER(),'@',-1))) AND PRIVILEGE_TYPE='SELECT' AND TABLE_SCHEMA=? AND TABLE_NAME=?))`

const performanceMySQLTable = `SELECT TABLE_TYPE, ENGINE, COALESCE(CREATE_OPTIONS,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`
const performanceMySQLColumns = `SELECT COLUMN_NAME, DATA_TYPE, COLUMN_TYPE, EXTRA, COALESCE(GENERATION_EXPRESSION,''), COALESCE(COLLATION_NAME,'') FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? ORDER BY ORDINAL_POSITION`
const performanceMySQLIndexes = `SELECT INDEX_TYPE, COALESCE(COLUMN_NAME,''), COALESCE(EXPRESSION,'') FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`
const performanceMySQLPartitions = `SELECT COALESCE(PARTITION_NAME,'') FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`
const performanceMySQLConstraints = `SELECT CONSTRAINT_TYPE FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=? AND TABLE_NAME=?`

type performanceSession interface {
	performanceQuerier
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func collectPerformance(ctx context.Context, tx performanceSession, dialect string, p *QueryPlan, shape qbapi.PerformanceShape) (*PerformancePlan, error) {
	versionSQL := `SELECT VERSION(), DATABASE()`
	if dialect == "postgres" {
		versionSQL = `SELECT pg_catalog.current_setting('server_version'), pg_catalog.current_database()`
	}
	r, e := performanceRows(ctx, tx, versionSQL, nil, 1, 4096)
	if e != nil {
		return nil, e
	}
	if len(r) != 1 || len(r[0]) != 2 {
		return nil, ErrPerformanceResult
	}
	out := &PerformancePlan{Kind: PerformancePlanKind, Version: 1, Dialect: dialect, ServerVersion: r[0][0], Database: r[0][1], Target: p, Unknown: true, CapturedAt: performanceNow()}
	known := dialect == "postgres" && performancePGVersion.MatchString(out.ServerVersion) || dialect == "mysql" && performanceMySQLVersion.MatchString(out.ServerVersion)
	if !known {
		return out, ErrPerformanceUnsupported
	}
	var types map[string]int
	if dialect == "postgres" {
		types, e = performancePGCatalog(ctx, tx, shape)
	} else {
		types, e = performanceMySQLCatalog(ctx, tx, shape, out.Database)
	}
	if e != nil {
		return nil, e
	}
	for _, col := range shape.Columns {
		if _, ok := types[col]; !ok {
			return nil, ErrPerformanceUnsupported
		}
	}
	for col, vs := range shape.Values {
		bits, ok := types[col]
		if !ok {
			return nil, ErrPerformanceUnsupported
		}
		for _, v := range vs {
			if v == nil {
				continue
			}
			if _, ok := scopeInteger(v, bits); !ok {
				return nil, ErrPerformanceUnsupported
			}
		}
	}
	statement := "EXPLAIN FORMAT=JSON " + p.execution.sql
	if dialect == "postgres" {
		statement = "EXPLAIN (ANALYZE FALSE, FORMAT JSON) " + p.execution.sql
	}
	r, e = performanceRows(ctx, tx, statement, p.execution.args, 1, 1<<20)
	if e != nil {
		return nil, e
	}
	if len(r) != 1 || len(r[0]) != 1 {
		return nil, ErrPerformanceResult
	}
	nodes, unknown, e := parsePerformance(dialect, []byte(r[0][0]))
	if e != nil {
		return nil, e
	}
	out.Raw = []byte(r[0][0])
	out.Nodes = nodes
	out.Unknown = unknown
	out.CapturedAt = performanceNow()
	return out, nil
}

func performancePGCatalog(ctx context.Context, tx performanceSession, shape qbapi.PerformanceShape) (map[string]int, error) {
	parts := strings.Split(shape.Table, ".")
	if len(parts) != 2 {
		return nil, ErrPerformanceUnsupported
	}
	// Only a transaction-local setting: rollback restores the caller's session.
	if _, e := tx.ExecContext(ctx, `SET LOCAL search_path TO pg_catalog`); e != nil {
		return nil, performanceFailure(ctx, ErrPerformanceQuery)
	}
	r, e := performanceRows(ctx, tx, performancePGTable, []any{parts[0], parts[1]}, 1, 4096)
	if e != nil {
		return nil, e
	}
	if len(r) != 1 || len(r[0]) != 3 || r[0][1] != "r" || r[0][2] != "true" {
		return nil, ErrPerformanceUnsupported
	}
	oid := r[0][0]
	if _, e := strconv.ParseUint(oid, 10, 32); e != nil {
		return nil, ErrPerformanceResult
	}
	r, e = performanceRows(ctx, tx, performancePGColumns, []any{oid}, 128, 32<<10)
	if e != nil {
		return nil, e
	}
	if len(r) == 0 {
		return nil, ErrPerformanceUnsupported
	}
	types := map[string]int{}
	for _, c := range r {
		if len(c) != 3 || c[2] != "true" {
			return nil, ErrPerformanceUnsupported
		}
		bits := 0
		switch c[1] {
		case "20":
			bits = 64
		case "21":
			bits = 16
		case "23":
			bits = 32
		default:
			return nil, ErrPerformanceUnsupported
		}
		if _, ok := types[c[0]]; ok {
			return nil, ErrPerformanceResult
		}
		types[c[0]] = bits
	}
	r, e = performanceRows(ctx, tx, performancePGOperators, nil, 54, 16<<10)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, v := range r {
		if len(v) != 4 || v[3] != "true" {
			return nil, ErrPerformanceUnsupported
		}
		k := strings.Join(v[:3], "/")
		if seen[k] {
			return nil, ErrPerformanceResult
		}
		seen[k] = true
	}
	if len(seen) != 54 {
		return nil, ErrPerformanceUnsupported
	}
	r, e = performanceRows(ctx, tx, performancePGIndexes, []any{oid}, 128, 16<<10)
	if e != nil {
		return nil, e
	}
	for _, v := range r {
		if len(v) != 2 || v[1] != "true" {
			return nil, ErrPerformanceUnsupported
		}
	}
	return types, nil
}

func performanceMySQLCatalog(ctx context.Context, tx performanceSession, shape qbapi.PerformanceShape, database string) (map[string]int, error) {
	parts := strings.Split(shape.Table, ".")
	schema, table := database, parts[0]
	if len(parts) == 2 {
		schema, table = parts[0], parts[1]
	}
	if schema != database {
		return nil, ErrPerformanceUnsupported
	}
	args := []any{schema, table}
	visibility, visibilityErr := performanceRows(ctx, tx, performanceMySQLVisibility, []any{schema, schema, table}, 1, 64)
	if visibilityErr != nil {
		return nil, visibilityErr
	}
	if len(visibility) != 1 || len(visibility[0]) != 1 || visibility[0][0] != "1" {
		return nil, ErrPerformanceUnsupported
	}

	// MySQL temporary tables shadow catalog base tables even with qualification.
	// SHOW resolves the actual target; identifiers came from the ASCII allowlist.
	if !scopeIdent(schema) || !scopeIdent(table) {
		return nil, ErrPerformanceUnsupported
	}
	definition, e := performanceRows(ctx, tx, "SHOW CREATE TABLE `"+schema+"`.`"+table+"`", nil, 1, 64<<10)
	if e != nil {
		return nil, e
	}
	if len(definition) != 1 || len(definition[0]) != 2 || !strings.HasPrefix(definition[0][1], "CREATE TABLE ") {
		return nil, ErrPerformanceUnsupported
	}

	r, e := performanceRows(ctx, tx, performanceMySQLTable, args, 1, 4096)
	if e != nil {
		return nil, e
	}
	if len(r) != 1 || len(r[0]) != 3 || r[0][0] != "BASE TABLE" || r[0][1] != "InnoDB" || r[0][2] != "" {
		return nil, ErrPerformanceUnsupported
	}
	r, e = performanceRows(ctx, tx, performanceMySQLColumns, args, 128, 32<<10)
	if e != nil {
		return nil, e
	}
	if len(r) == 0 {
		return nil, ErrPerformanceUnsupported
	}
	types := map[string]int{}
	for _, c := range r {
		if len(c) != 6 || c[4] != "" || c[5] != "" || c[3] != "" && c[3] != "auto_increment" || strings.Contains(c[2], "unsigned") || strings.Contains(c[2], "zerofill") {
			return nil, ErrPerformanceUnsupported
		}
		bits := 0
		switch c[1] {
		case "tinyint":
			bits = 8
		case "smallint":
			bits = 16
		case "mediumint":
			bits = 24
		case "int":
			bits = 32
		case "bigint":
			bits = 64
		default:
			return nil, ErrPerformanceUnsupported
		}
		if _, ok := types[c[0]]; ok {
			return nil, ErrPerformanceResult
		}
		types[c[0]] = bits
	}
	r, e = performanceRows(ctx, tx, performanceMySQLIndexes, args, 256, 32<<10)
	if e != nil {
		return nil, e
	}
	for _, v := range r {
		if len(v) != 3 || v[0] != "BTREE" || v[1] == "" || v[2] != "" {
			return nil, ErrPerformanceUnsupported
		}
		if _, ok := types[v[1]]; !ok {
			return nil, ErrPerformanceUnsupported
		}
	}
	r, e = performanceRows(ctx, tx, performanceMySQLPartitions, args, 1, 4096)
	if e != nil {
		return nil, e
	}
	if len(r) != 1 || len(r[0]) != 1 || r[0][0] != "" {
		return nil, ErrPerformanceUnsupported
	}
	r, e = performanceRows(ctx, tx, performanceMySQLConstraints, args, 128, 16<<10)
	if e != nil {
		return nil, e
	}
	for _, v := range r {
		if len(v) != 1 || v[0] != "PRIMARY KEY" && v[0] != "UNIQUE" {
			return nil, ErrPerformanceUnsupported
		}
	}
	return types, nil
}
