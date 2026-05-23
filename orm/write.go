package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/model"
	"github.com/faciam-dev/goquent/orm/query"
)

// WriteOpt configures write behavior.
type WriteOpt func(*writeOptions)

type writeOptions struct {
	cols               map[string]struct{}
	omit               map[string]struct{}
	wherePK            bool
	returning          []string
	table              string
	tablePath          []string
	schema             string
	pkCols             map[string]struct{}
	conflictCols       []string
	conflictWhere      string
	conflictConstraint string
	conflictTargetRaw  string
	upsertUpdateCols   []string
	hasUpsertUpdates   bool
	conflictDoNothing  bool
	assignments        []writeAssignment
	expectAffected     *int64
	zeroRowsErr        error
}

type writeAssignmentKind int

const (
	writeAssignmentRaw writeAssignmentKind = iota
	writeAssignmentExpr
	writeAssignmentColumn
	writeAssignmentIncrement
)

type writeAssignment struct {
	column       string
	expression   string
	args         []any
	sourceColumn string
	kind         writeAssignmentKind
}

// Columns limits write to specified columns.
func Columns(cols ...string) WriteOpt {
	return func(o *writeOptions) {
		if o.cols == nil {
			o.cols = make(map[string]struct{}, len(cols))
		}
		for _, c := range cols {
			o.cols[c] = struct{}{}
		}
	}
}

// Omit excludes specified columns.
func Omit(cols ...string) WriteOpt {
	return func(o *writeOptions) {
		if o.omit == nil {
			o.omit = make(map[string]struct{}, len(cols))
		}
		for _, c := range cols {
			o.omit[c] = struct{}{}
		}
	}
}

// WherePK uses primary key columns in WHERE clause.
func WherePK() WriteOpt { return func(o *writeOptions) { o.wherePK = true } }

// Returning specifies columns to return (Postgres only).
func Returning(cols ...string) WriteOpt { return func(o *writeOptions) { o.returning = cols } }

// ConflictColumns sets the conflict target columns for Upsert.
func ConflictColumns(cols ...string) WriteOpt {
	return func(o *writeOptions) { o.conflictCols = append([]string(nil), cols...) }
}

// ConflictWhere adds a Postgres partial-index predicate to the conflict target.
func ConflictWhere(predicate string) WriteOpt {
	return func(o *writeOptions) { o.conflictWhere = predicate }
}

// ConflictConstraint sets a Postgres named constraint as the conflict target.
func ConflictConstraint(name string) WriteOpt {
	return func(o *writeOptions) { o.conflictConstraint = name }
}

// ConflictTargetRaw sets a raw Postgres ON CONFLICT target.
//
// Use this for expression indexes such as:
//
//	ConflictTargetRaw(`("tenant_id", COALESCE("target_node_id", '')) WHERE "active"`)
//
// Prefer ConflictColumns, ConflictWhere, or ConflictConstraint when they can
// express the target.
func ConflictTargetRaw(target string) WriteOpt {
	return func(o *writeOptions) { o.conflictTargetRaw = target }
}

// UpdateColumns limits the conflict UPDATE side of Upsert/UpsertReturning.
// The insert side still uses Columns/Omit plus required conflict or primary-key columns.
func UpdateColumns(cols ...string) WriteOpt {
	return func(o *writeOptions) {
		o.upsertUpdateCols = append([]string(nil), cols...)
		o.hasUpsertUpdates = true
	}
}

// ConflictDoNothing makes Upsert/UpsertReturning use a no-op conflict action.
func ConflictDoNothing() WriteOpt {
	return func(o *writeOptions) {
		o.upsertUpdateCols = nil
		o.hasUpsertUpdates = true
		o.conflictDoNothing = true
	}
}

// Table sets table name (required for map writes).
func Table(name string) WriteOpt {
	return func(o *writeOptions) {
		o.table = name
		o.tablePath = nil
	}
}

// TablePath sets a schema-qualified or otherwise path-qualified table name.
//
// For example, TablePath("app", "users") renders "app"."users" on
// PostgreSQL and `app`.`users` on MySQL.
func TablePath(parts ...string) WriteOpt {
	return func(o *writeOptions) {
		o.tablePath = append([]string(nil), parts...)
		o.table = strings.Join(parts, ".")
		o.schema = ""
	}
}

// SchemaName sets the schema for the write table. It can be combined with
// Table("users") or an inferred struct table name.
func SchemaName(name string) WriteOpt {
	return func(o *writeOptions) {
		o.schema = name
	}
}

// PK specifies primary key columns for map writes.
func PK(cols ...string) WriteOpt {
	return func(o *writeOptions) {
		if o.pkCols == nil {
			o.pkCols = make(map[string]struct{}, len(cols))
		}
		for _, c := range cols {
			o.pkCols[c] = struct{}{}
		}
	}
}

// SetRaw adds a database-side assignment to Update or the conflict-update side
// of Upsert. The expression is a trusted SQL fragment and must not contain
// placeholders.
func SetRaw(column, expression string) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			column:     column,
			expression: expression,
			kind:       writeAssignmentRaw,
		})
	}
}

// SetExpr adds a database-side assignment with positional ? placeholders.
// Placeholders are rewritten for the active dialect.
func SetExpr(column, expression string, args ...any) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			column:     column,
			expression: expression,
			args:       append([]any(nil), args...),
			kind:       writeAssignmentExpr,
		})
	}
}

// SetColumn assigns one column from another column.
func SetColumn(column, sourceColumn string) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			column:       column,
			sourceColumn: sourceColumn,
			kind:         writeAssignmentColumn,
		})
	}
}

// Increment adds delta to column on Update or the conflict-update side of
// Upsert.
func Increment(column string, delta any) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			column: column,
			args:   []any{delta},
			kind:   writeAssignmentIncrement,
		})
	}
}

// ExpectAffected requires the write to affect exactly n rows.
func ExpectAffected(n int64) WriteOpt {
	return func(o *writeOptions) {
		o.expectAffected = &n
	}
}

// NoRowsAs maps a zero-row write result to err. Use ErrConflict for explicit
// optimistic-concurrency guards such as content_hash or version predicates.
func NoRowsAs(err error) WriteOpt {
	return func(o *writeOptions) {
		o.zeroRowsErr = err
	}
}

func applyWriteOpts(opts []WriteOpt) *writeOptions {
	o := &writeOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *writeOptions) isPK(col string) bool {
	if o.pkCols == nil {
		return false
	}
	_, ok := o.pkCols[col]
	return ok
}

func (o *writeOptions) hasConflictTarget() bool {
	return len(o.conflictCols) > 0 ||
		strings.TrimSpace(o.conflictConstraint) != "" ||
		strings.TrimSpace(o.conflictTargetRaw) != ""
}

func (o *writeOptions) isConflictColumn(col string) bool {
	for _, c := range o.conflictCols {
		if c == col {
			return true
		}
	}
	return false
}

func quote(d driver.Dialect, ident string) string { return d.QuoteIdent(ident) }

func quoteIdentifierPath(d driver.Dialect, ident string) (string, error) {
	return quoteIdentifierPathParts(d, strings.Split(ident, "."))
}

func quoteIdentifierPathParts(d driver.Dialect, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("goquent: identifier path is required")
	}
	quoted := make([]string, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("goquent: identifier path contains an empty part")
		}
		quoted[i] = quote(d, part)
	}
	return strings.Join(quoted, "."), nil
}

func quoteWriteTable(d driver.Dialect, table string, o *writeOptions) (string, error) {
	if len(o.tablePath) > 0 {
		return quoteIdentifierPathParts(d, o.tablePath)
	}
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("goquent: table name is required")
	}
	if schema := strings.TrimSpace(o.schema); schema != "" {
		if strings.Contains(table, ".") {
			return "", fmt.Errorf("goquent: Schema cannot be combined with schema-qualified table %q", table)
		}
		return quoteIdentifierPathParts(d, []string{schema, table})
	}
	return quoteIdentifierPath(d, table)
}

func buildPlaceholders(d driver.Dialect, n int, start int) []string {
	ph := make([]string, n)
	for i := 0; i < n; i++ {
		ph[i] = d.Placeholder(start + i)
	}
	return ph
}

type returningResult struct {
	rowsAffected int64
}

func (r returningResult) LastInsertId() (int64, error) {
	return 0, fmt.Errorf("LastInsertId is not supported for RETURNING statements")
}

func (r returningResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func appendReturningClause(d driver.Dialect, sqlStr string, cols []string) (string, error) {
	if len(cols) == 0 {
		return sqlStr, nil
	}
	if _, ok := d.(driver.PostgresDialect); !ok {
		return "", fmt.Errorf("Returning is not supported on dialect: %T", d)
	}
	rc := make([]string, len(cols))
	for i, c := range cols {
		quoted, err := quoteIdentifierPath(d, c)
		if err != nil {
			return "", err
		}
		rc[i] = quoted
	}
	return sqlStr + " RETURNING " + strings.Join(rc, ", "), nil
}

func execReturningRows(ctx context.Context, db *DB, sqlStr string, args ...any) (sql.Result, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if ctx != nil {
		rows, err = db.exec.QueryContext(ctx, sqlStr, args...)
	} else {
		rows, err = db.exec.Query(sqlStr, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	scanDst := make([]any, len(cols))
	values := make([]any, len(cols))
	for i := range scanDst {
		scanDst[i] = &values[i]
	}

	var count int64
	for rows.Next() {
		if len(scanDst) > 0 {
			if err := rows.Scan(scanDst...); err != nil {
				return nil, err
			}
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return returningResult{rowsAffected: count}, nil
}

func queryReturningOne[T any](ctx context.Context, db *DB, sqlStr string, args ...any) (T, error) {
	var zero T
	rows, err := db.queryContextTrusted(ctx, sqlStr, args...)
	if err != nil {
		return zero, err
	}
	defer rows.Close()
	return scanRowsOne[T](db, rows)
}

func queryReturningAll[T any](ctx context.Context, db *DB, sqlStr string, args ...any) ([]T, error) {
	rows, err := db.queryContextTrusted(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRowsAll[T](db, rows)
}

func queryReturningOneWithOptions[T any](ctx context.Context, db *DB, sqlStr string, o *writeOptions, args ...any) (T, error) {
	row, err := queryReturningOne[T](ctx, db, sqlStr, args...)
	if err == nil || !IsNotFound(err) || o == nil || o.zeroRowsErr == nil {
		return row, err
	}
	var zero T
	return zero, RowsAffectedError{Expected: 1, Actual: 0, Cause: o.zeroRowsErr}
}

func ensureReturningColumns[T any](o *writeOptions) error {
	returning := o != nil && len(o.returning) > 0
	if returning {
		return nil
	}
	cols, err := returningColumnsForQuery[T]()
	if err != nil {
		return err
	}
	o.returning = cols
	return nil
}

func returningColumnsForQuery[T any]() ([]string, error) {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil {
		return nil, fmt.Errorf("Returning columns are required for untyped return values")
	}
	if isMapStringInterface(typ) {
		return nil, fmt.Errorf("Returning columns are required for map return values")
	}
	cols, err := structColumnNames(typ)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns to return")
	}
	return cols, nil
}

func execWriteStatement(ctx context.Context, db *DB, sqlStr string, args []any, o *writeOptions) (sql.Result, error) {
	var (
		res sql.Result
		err error
	)
	if o != nil && len(o.returning) > 0 {
		res, err = execReturningRows(ctx, db, sqlStr, args...)
	} else {
		res, err = db.execContextTrusted(ctx, sqlStr, args...)
	}
	if err != nil {
		return nil, err
	}
	if err := checkRowsAffected(res, o); err != nil {
		return nil, err
	}
	return res, nil
}

func checkRowsAffected(res sql.Result, o *writeOptions) error {
	if res == nil || o == nil || (o.expectAffected == nil && o.zeroRowsErr == nil) {
		return nil
	}
	actual, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if o.expectAffected != nil {
		expected := *o.expectAffected
		if actual == expected {
			return nil
		}
		cause := ErrRowsAffected
		if actual == 0 && o.zeroRowsErr != nil {
			cause = o.zeroRowsErr
		}
		return RowsAffectedError{Expected: expected, Actual: actual, Cause: cause}
	}
	if actual == 0 && o.zeroRowsErr != nil {
		return RowsAffectedError{Expected: 1, Actual: actual, Cause: o.zeroRowsErr}
	}
	return nil
}

func validateWriteRawSQLFragment(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("goquent: raw SQL fragment is required")
	}
	if strings.ContainsAny(trimmed, ";\x00") ||
		strings.Contains(trimmed, "--") ||
		strings.Contains(trimmed, "/*") ||
		strings.Contains(trimmed, "*/") {
		return fmt.Errorf("goquent: raw SQL fragment contains a statement separator or comment")
	}
	upper := strings.ToUpper(trimmed)
	for _, token := range []string{"ALTER", "CREATE", "DELETE", "DROP", "GRANT", "INSERT", "REVOKE", "TRUNCATE", "UPDATE"} {
		if containsSQLWord(upper, token) {
			return fmt.Errorf("goquent: raw SQL fragment contains disallowed SQL token %q", token)
		}
	}
	return nil
}

func writeAssignmentTargets(assignments []writeAssignment) map[string]struct{} {
	targets := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		column := strings.TrimSpace(assignment.column)
		if column == "" {
			continue
		}
		targets[column] = struct{}{}
	}
	return targets
}

func validateWriteAssignments(assignments []writeAssignment) error {
	seen := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		column := strings.TrimSpace(assignment.column)
		if column == "" {
			return fmt.Errorf("goquent: assignment column is required")
		}
		if _, ok := seen[column]; ok {
			return fmt.Errorf("goquent: duplicate assignment for column %s", column)
		}
		seen[column] = struct{}{}
	}
	return nil
}

func buildWriteSetParts(d driver.Dialect, setCols []string, setArgs []any, assignments []writeAssignment, start int) ([]string, []any, error) {
	if err := validateWriteAssignments(assignments); err != nil {
		return nil, nil, err
	}
	setParts := make([]string, 0, len(setCols)+len(assignments))
	args := make([]any, 0, len(setArgs)+len(assignments))
	argPos := start
	for i, col := range setCols {
		setParts = append(setParts, fmt.Sprintf("%s=%s", quote(d, col), d.Placeholder(argPos)))
		args = append(args, setArgs[i])
		argPos++
	}
	for _, assignment := range assignments {
		target, err := quoteIdentifierPath(d, assignment.column)
		if err != nil {
			return nil, nil, err
		}
		expr, exprArgs, err := renderWriteAssignment(d, assignment, argPos)
		if err != nil {
			return nil, nil, err
		}
		setParts = append(setParts, fmt.Sprintf("%s=%s", target, expr))
		args = append(args, exprArgs...)
		argPos += len(exprArgs)
	}
	return setParts, args, nil
}

func renderWriteAssignment(d driver.Dialect, assignment writeAssignment, start int) (string, []any, error) {
	switch assignment.kind {
	case writeAssignmentRaw:
		if len(assignment.args) > 0 {
			return "", nil, fmt.Errorf("goquent: SetRaw does not accept args")
		}
		expr := strings.TrimSpace(assignment.expression)
		if err := validateWriteRawSQLFragment(expr); err != nil {
			return "", nil, err
		}
		if strings.Contains(expr, "?") {
			return "", nil, fmt.Errorf("goquent: SetRaw expression contains placeholders; use SetExpr")
		}
		return expr, nil, nil
	case writeAssignmentExpr:
		expr := strings.TrimSpace(assignment.expression)
		if err := validateWriteRawSQLFragment(expr); err != nil {
			return "", nil, err
		}
		rendered, err := renderWriteExpressionPlaceholders(d, expr, len(assignment.args), start)
		if err != nil {
			return "", nil, err
		}
		return rendered, append([]any(nil), assignment.args...), nil
	case writeAssignmentColumn:
		if len(assignment.args) > 0 {
			return "", nil, fmt.Errorf("goquent: SetColumn does not accept args")
		}
		expr, err := quoteIdentifierPath(d, assignment.sourceColumn)
		return expr, nil, err
	case writeAssignmentIncrement:
		if len(assignment.args) != 1 {
			return "", nil, fmt.Errorf("goquent: Increment requires one delta arg")
		}
		target, err := quoteIdentifierPath(d, assignment.column)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s + %s", target, d.Placeholder(start)), append([]any(nil), assignment.args...), nil
	default:
		return "", nil, fmt.Errorf("goquent: unknown assignment kind")
	}
}

func renderWriteExpressionPlaceholders(d driver.Dialect, expr string, argCount int, start int) (string, error) {
	if strings.Count(expr, "?") != argCount {
		return "", fmt.Errorf("goquent: SetExpr placeholder count does not match args")
	}
	if argCount == 0 {
		return expr, nil
	}
	var b strings.Builder
	b.Grow(len(expr) + argCount*2)
	argIndex := 0
	for i := 0; i < len(expr); i++ {
		if expr[i] != '?' {
			b.WriteByte(expr[i])
			continue
		}
		b.WriteString(d.Placeholder(start + argIndex))
		argIndex++
	}
	return b.String(), nil
}

// Insert inserts v into its table.
func Insert[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	sqlStr, args, err := buildInsertStatement(db, v, o)
	if err != nil {
		return nil, err
	}
	return execWriteStatement(ctx, db, sqlStr, args, o)
}

// InsertReturning inserts v and scans the Postgres RETURNING row into T.
func InsertReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, err
	}
	sqlStr, args, err := buildInsertStatement(db, v, o)
	if err != nil {
		return zero, err
	}
	return queryReturningOneWithOptions[T](ctx, db, sqlStr, o, args...)
}

// InsertMany inserts all values in one INSERT statement.
//
// Empty slices return an error instead of a no-op result. Map writes require
// Table, and every row must provide the selected column set.
func InsertMany[T any](ctx context.Context, db *DB, values []T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	sqlStr, args, err := buildInsertManyStatement(db, values, o)
	if err != nil {
		return nil, err
	}
	return execWriteStatement(ctx, db, sqlStr, args, o)
}

// InsertManyReturning inserts all values and scans PostgreSQL RETURNING rows.
func InsertManyReturning[R any, T any](ctx context.Context, db *DB, values []T, opts ...WriteOpt) ([]R, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("goquent: no rows to insert")
	}
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[R](o); err != nil {
		return nil, err
	}
	sqlStr, args, err := buildInsertManyStatement(db, values, o)
	if err != nil {
		return nil, err
	}
	return queryReturningAll[R](ctx, db, sqlStr, args...)
}

func buildInsertStatement(db *DB, v any, o *writeOptions) (string, []any, error) {
	if len(o.assignments) > 0 {
		return "", nil, fmt.Errorf("assignment options are not supported for Insert")
	}
	val := reflect.ValueOf(v)
	typ := val.Type()
	var table string
	var cols []string
	var args []any

	if isMapStringInterface(typ) {
		if o.table == "" {
			return "", nil, fmt.Errorf("Table option required for map writes")
		}
		table = o.table
		iter := val.MapRange()
		for iter.Next() {
			col := iter.Key().String()
			if len(o.cols) > 0 {
				if _, ok := o.cols[col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[col]; ok {
				continue
			}
			cols = append(cols, col)
			args = append(args, iter.Value().Interface())
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		meta, err := getTypeMeta(typ)
		if err != nil {
			return "", nil, err
		}
		for _, fm := range meta.FieldsByName {
			if fm.Readonly {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[fm.Col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[fm.Col]; ok {
				continue
			}
			fv := val.FieldByIndex(fm.IndexPath)
			if fm.OmitEmpty && fv.IsZero() {
				continue
			}
			cols = append(cols, fm.Col)
			args = append(args, fv.Interface())
		}
	} else {
		return "", nil, fmt.Errorf("unsupported type %s", typ)
	}
	if len(cols) == 0 {
		return "", nil, fmt.Errorf("no columns to insert")
	}
	ph := buildPlaceholders(db.drv.Dialect, len(cols), 1)
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quote(db.drv.Dialect, c)
	}
	tableSQL, err := quoteWriteTable(db.drv.Dialect, table, o)
	if err != nil {
		return "", nil, err
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableSQL, strings.Join(quotedCols, ", "), strings.Join(ph, ", "))
	sqlStr, err = appendReturningClause(db.drv.Dialect, sqlStr, o.returning)
	if err != nil {
		return "", nil, err
	}
	return sqlStr, args, nil
}

func buildInsertManyStatement[T any](db *DB, values []T, o *writeOptions) (string, []any, error) {
	if len(values) == 0 {
		return "", nil, fmt.Errorf("goquent: no rows to insert")
	}
	if err := validateInsertManyOptions(o); err != nil {
		return "", nil, err
	}

	first := reflect.ValueOf(values[0])
	if !first.IsValid() {
		return "", nil, fmt.Errorf("unsupported type <nil>")
	}
	typ := first.Type()

	var (
		table string
		cols  []string
		args  []any
		err   error
	)
	switch {
	case isMapStringInterface(typ):
		if o.table == "" {
			return "", nil, fmt.Errorf("Table option required for map writes")
		}
		table = o.table
		cols, args, err = insertManyMapColumnsAndArgs(values, o)
	case typ.Kind() == reflect.Struct:
		table = o.table
		if table == "" {
			table = model.TableName(values[0])
		}
		cols, args, err = insertManyStructColumnsAndArgs(values, typ, o)
	default:
		return "", nil, fmt.Errorf("unsupported type %s", typ)
	}
	if err != nil {
		return "", nil, err
	}
	return buildInsertManySQL(db, table, cols, len(values), args, o)
}

func validateInsertManyOptions(o *writeOptions) error {
	if len(o.assignments) > 0 {
		return fmt.Errorf("assignment options are not supported for InsertMany")
	}
	if len(o.conflictCols) > 0 ||
		strings.TrimSpace(o.conflictWhere) != "" ||
		strings.TrimSpace(o.conflictConstraint) != "" ||
		strings.TrimSpace(o.conflictTargetRaw) != "" ||
		o.hasUpsertUpdates ||
		o.conflictDoNothing {
		return fmt.Errorf("conflict/upsert options are not supported for InsertMany")
	}
	return nil
}

func insertManyStructColumnsAndArgs[T any](values []T, typ reflect.Type, o *writeOptions) ([]string, []any, error) {
	var cols []string
	args := make([]any, 0, len(values))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, fmt.Errorf("goquent: InsertMany row %d is nil", i)
		}
		if val.Type() != typ {
			return nil, nil, fmt.Errorf("goquent: InsertMany row %d has type %s, expected %s", i, val.Type(), typ)
		}
		rowCols, rowArgs, err := insertStructColumnsAndArgs(val, o)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			cols = rowCols
		} else if !sameColumns(cols, rowCols) {
			return nil, nil, fmt.Errorf("goquent: InsertMany requires identical columns in every row")
		}
		args = append(args, rowArgs...)
	}
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("no columns to insert")
	}
	return cols, args, nil
}

func insertStructColumnsAndArgs(val reflect.Value, o *writeOptions) ([]string, []any, error) {
	meta, err := getTypeMeta(val.Type())
	if err != nil {
		return nil, nil, err
	}
	cols := make([]string, 0, len(meta.Fields))
	args := make([]any, 0, len(meta.Fields))
	for _, fm := range meta.Fields {
		if fm.Readonly {
			continue
		}
		if len(o.cols) > 0 {
			if _, ok := o.cols[fm.Col]; !ok {
				continue
			}
		}
		if _, ok := o.omit[fm.Col]; ok {
			continue
		}
		fv := val.FieldByIndex(fm.IndexPath)
		if fm.OmitEmpty && fv.IsZero() {
			continue
		}
		cols = append(cols, fm.Col)
		args = append(args, fv.Interface())
	}
	return cols, args, nil
}

func insertManyMapColumnsAndArgs[T any](values []T, o *writeOptions) ([]string, []any, error) {
	first := reflect.ValueOf(values[0])
	cols := mapInsertManyColumnSet(first, o)
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("no columns to insert")
	}
	requireExact := len(o.cols) == 0
	args := make([]any, 0, len(values)*len(cols))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, fmt.Errorf("goquent: InsertMany map row %d is nil", i)
		}
		if !isMapStringInterface(val.Type()) {
			return nil, nil, fmt.Errorf("goquent: InsertMany row %d has type %s, expected %s", i, val.Type(), first.Type())
		}
		if requireExact && !sameColumns(cols, mapInsertManyColumnSet(val, o)) {
			return nil, nil, fmt.Errorf("goquent: InsertMany map row %d has inconsistent columns", i)
		}
		rowArgs, err := mapInsertManyRowArgs(val, cols, i)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, rowArgs...)
	}
	return cols, args, nil
}

func mapInsertManyColumnSet(val reflect.Value, o *writeOptions) []string {
	if len(o.cols) > 0 {
		cols := make([]string, 0, len(o.cols))
		for col := range o.cols {
			if _, omitted := o.omit[col]; omitted {
				continue
			}
			cols = append(cols, col)
		}
		sort.Strings(cols)
		return cols
	}
	cols := make([]string, 0, val.Len())
	iter := val.MapRange()
	for iter.Next() {
		col := iter.Key().String()
		if _, omitted := o.omit[col]; omitted {
			continue
		}
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

func mapInsertManyRowArgs(val reflect.Value, cols []string, row int) ([]any, error) {
	args := make([]any, 0, len(cols))
	for _, col := range cols {
		mv := val.MapIndex(reflect.ValueOf(col))
		if !mv.IsValid() {
			return nil, fmt.Errorf("goquent: InsertMany map row %d missing column %s", row, col)
		}
		args = append(args, mv.Interface())
	}
	return args, nil
}

func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func buildInsertManySQL(db *DB, table string, cols []string, rowCount int, args []any, o *writeOptions) (string, []any, error) {
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quote(db.drv.Dialect, c)
	}
	values := make([]string, rowCount)
	argPos := 1
	for i := 0; i < rowCount; i++ {
		ph := buildPlaceholders(db.drv.Dialect, len(cols), argPos)
		values[i] = "(" + strings.Join(ph, ", ") + ")"
		argPos += len(cols)
	}
	tableSQL, err := quoteWriteTable(db.drv.Dialect, table, o)
	if err != nil {
		return "", nil, err
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", tableSQL, strings.Join(quotedCols, ", "), strings.Join(values, ", "))
	sqlStr, err = appendReturningClause(db.drv.Dialect, sqlStr, o.returning)
	if err != nil {
		return "", nil, err
	}
	return sqlStr, args, nil
}

// Update updates record v.
func Update[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	sqlStr, args, err := buildUpdateStatement(db, v, o)
	if err != nil {
		return nil, err
	}
	return execWriteStatement(ctx, db, sqlStr, args, o)
}

// UpdateReturning updates v and scans the Postgres RETURNING row into T.
func UpdateReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, err
	}
	sqlStr, args, err := buildUpdateStatement(db, v, o)
	if err != nil {
		return zero, err
	}
	return queryReturningOneWithOptions[T](ctx, db, sqlStr, o, args...)
}

func buildUpdateStatement(db *DB, v any, o *writeOptions) (string, []any, error) {
	if !o.wherePK {
		return "", nil, fmt.Errorf("Update[T] without WherePK is not allowed")
	}
	assignmentTargets := writeAssignmentTargets(o.assignments)
	val := reflect.ValueOf(v)
	typ := val.Type()
	var table string
	var setCols []string
	var setArgs []any
	var whereCols []string
	var whereArgs []any

	if isMapStringInterface(typ) {
		if o.table == "" {
			return "", nil, fmt.Errorf("Table option required for map writes")
		}
		if len(o.pkCols) == 0 {
			return "", nil, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
		}
		table = o.table
		iter := val.MapRange()
		seen := make(map[string]bool)
		for iter.Next() {
			col := iter.Key().String()
			v := iter.Value()
			seen[col] = true
			if o.isPK(col) {
				whereCols = append(whereCols, col)
				whereArgs = append(whereArgs, v.Interface())
				continue
			}
			if _, ok := assignmentTargets[col]; ok {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[col]; ok {
				continue
			}
			setCols = append(setCols, col)
			setArgs = append(setArgs, v.Interface())
		}
		for pk := range o.pkCols {
			if !seen[pk] {
				return "", nil, fmt.Errorf("WherePK requires pk column %s", pk)
			}
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		meta, err := getTypeMeta(typ)
		if err != nil {
			return "", nil, err
		}
		for _, fm := range meta.FieldsByName {
			fv := val.FieldByIndex(fm.IndexPath)
			if fm.PK {
				whereCols = append(whereCols, fm.Col)
				whereArgs = append(whereArgs, fv.Interface())
				continue
			}
			if fm.Readonly {
				continue
			}
			if _, ok := assignmentTargets[fm.Col]; ok {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[fm.Col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[fm.Col]; ok {
				continue
			}
			if fm.OmitEmpty && fv.IsZero() {
				continue
			}
			setCols = append(setCols, fm.Col)
			setArgs = append(setArgs, fv.Interface())
		}
	} else {
		return "", nil, fmt.Errorf("unsupported type %s", typ)
	}
	if len(whereCols) == 0 {
		return "", nil, fmt.Errorf("WherePK requires pk values")
	}
	if len(setCols) == 0 && len(o.assignments) == 0 {
		return "", nil, fmt.Errorf("no columns to update")
	}
	setParts, setArgs, err := buildWriteSetParts(db.drv.Dialect, setCols, setArgs, o.assignments, 1)
	if err != nil {
		return "", nil, err
	}
	whereParts := make([]string, len(whereCols))
	for i, col := range whereCols {
		whereParts[i] = fmt.Sprintf("%s=%s", quote(db.drv.Dialect, col), db.drv.Dialect.Placeholder(len(setArgs)+i+1))
	}
	args := append(append([]any(nil), setArgs...), whereArgs...)
	tableSQL, err := quoteWriteTable(db.drv.Dialect, table, o)
	if err != nil {
		return "", nil, err
	}
	sqlStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s", tableSQL, strings.Join(setParts, ", "), strings.Join(whereParts, " AND "))
	sqlStr, err = appendReturningClause(db.drv.Dialect, sqlStr, o.returning)
	if err != nil {
		return "", nil, err
	}
	return sqlStr, args, nil
}

// Upsert inserts or updates v using primary keys.
func Upsert[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	sqlStr, args, err := buildUpsertStatement(db, v, o)
	if err != nil {
		return nil, err
	}
	return execWriteStatement(ctx, db, sqlStr, args, o)
}

// UpsertReturning upserts v and scans the Postgres RETURNING row into T.
func UpsertReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, err
	}
	sqlStr, args, err := buildUpsertStatement(db, v, o)
	if err != nil {
		return zero, err
	}
	return queryReturningOneWithOptions[T](ctx, db, sqlStr, o, args...)
}

// InsertOnceReturning inserts v once and scans the inserted or existing row.
//
// It uses ON CONFLICT DO NOTHING RETURNING for the insert attempt. If the
// conflict path returns no row, it looks up the existing row by ConflictColumns
// or WherePK primary-key columns. Expression-only raw conflict targets need
// ConflictColumns or WherePK as a lookup key.
func InsertOnceReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, bool, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, false, err
	}
	o.upsertUpdateCols = nil
	o.hasUpsertUpdates = true
	o.conflictDoNothing = true

	sqlStr, args, err := buildUpsertStatement(db, v, o)
	if err != nil {
		return zero, false, err
	}
	inserted, err := queryReturningOne[T](ctx, db, sqlStr, args...)
	if err == nil {
		return inserted, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return zero, false, err
	}

	existing, err := selectExistingInsertOnceRow[T](ctx, db, v, o)
	if err != nil {
		return zero, false, err
	}
	return existing, false, nil
}

func buildUpsertStatement(db *DB, v any, o *writeOptions) (string, []any, error) {
	if !o.wherePK && !o.hasConflictTarget() {
		return "", nil, fmt.Errorf("Upsert[T] requires WherePK, ConflictColumns, or ConflictConstraint")
	}
	if o.conflictDoNothing && len(o.assignments) > 0 {
		return "", nil, fmt.Errorf("ConflictDoNothing cannot be combined with assignment options")
	}
	val := reflect.ValueOf(v)
	typ := val.Type()
	var table string
	var cols []string
	var args []any
	var pkCols []string

	if isMapStringInterface(typ) {
		if o.table == "" {
			return "", nil, fmt.Errorf("Table option required for map writes")
		}
		if o.wherePK && len(o.pkCols) == 0 {
			return "", nil, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
		}
		table = o.table
		iter := val.MapRange()
		seen := make(map[string]bool)
		for iter.Next() {
			col := iter.Key().String()
			fv := iter.Value().Interface()
			seen[col] = true
			if o.isPK(col) {
				pkCols = append(pkCols, col)
				cols = append(cols, col)
				args = append(args, fv)
				continue
			}
			if o.isConflictColumn(col) {
				cols = append(cols, col)
				args = append(args, fv)
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[col]; ok {
				continue
			}
			cols = append(cols, col)
			args = append(args, fv)
		}
		if o.wherePK {
			for pk := range o.pkCols {
				if !seen[pk] {
					return "", nil, fmt.Errorf("WherePK requires pk column %s", pk)
				}
			}
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		meta, err := getTypeMeta(typ)
		if err != nil {
			return "", nil, err
		}
		for _, fm := range meta.FieldsByName {
			fv := val.FieldByIndex(fm.IndexPath)
			if fm.PK {
				pkCols = append(pkCols, fm.Col)
				cols = append(cols, fm.Col)
				args = append(args, fv.Interface())
				continue
			}
			if o.isConflictColumn(fm.Col) {
				cols = append(cols, fm.Col)
				args = append(args, fv.Interface())
				continue
			}
			if fm.Readonly {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[fm.Col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[fm.Col]; ok {
				continue
			}
			if fm.OmitEmpty && fv.IsZero() {
				continue
			}
			cols = append(cols, fm.Col)
			args = append(args, fv.Interface())
		}
	} else {
		return "", nil, fmt.Errorf("unsupported type %s", typ)
	}
	if o.wherePK && len(pkCols) == 0 {
		return "", nil, fmt.Errorf("WherePK requires pk values")
	}
	if len(cols) == 0 {
		return "", nil, fmt.Errorf("no columns to insert")
	}
	if err := ensureConflictColumnsPresent(o.conflictCols, cols); err != nil {
		return "", nil, err
	}
	ph := buildPlaceholders(db.drv.Dialect, len(cols), 1)
	quotedCols := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = quote(db.drv.Dialect, c)
	}
	tableSQL, err := quoteWriteTable(db.drv.Dialect, table, o)
	if err != nil {
		return "", nil, err
	}
	sqlStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableSQL, strings.Join(quotedCols, ", "), strings.Join(ph, ", "))
	targetCols := conflictTargetColumns(o, pkCols)
	updateCols, err := upsertUpdateColumns(cols, targetCols, o)
	if err != nil {
		return "", nil, err
	}
	assignmentParts, assignmentArgs, err := buildWriteSetParts(db.drv.Dialect, nil, nil, o.assignments, len(args)+1)
	if err != nil {
		return "", nil, err
	}
	switch db.drv.Dialect.(type) {
	case driver.MySQLDialect:
		if strings.TrimSpace(o.conflictWhere) != "" || strings.TrimSpace(o.conflictConstraint) != "" || strings.TrimSpace(o.conflictTargetRaw) != "" {
			return "", nil, fmt.Errorf("ConflictWhere, ConflictConstraint, and ConflictTargetRaw are not supported on dialect: %T", db.drv.Dialect)
		}
		if len(updateCols) > 0 || len(assignmentParts) > 0 {
			assigns := make([]string, 0, len(updateCols)+len(assignmentParts))
			for _, c := range updateCols {
				assigns = append(assigns, fmt.Sprintf("%s=VALUES(%s)", quote(db.drv.Dialect, c), quote(db.drv.Dialect, c)))
			}
			assigns = append(assigns, assignmentParts...)
			sqlStr += " ON DUPLICATE KEY UPDATE " + strings.Join(assigns, ", ")
		} else {
			sqlStr = strings.Replace(sqlStr, "INSERT", "INSERT IGNORE", 1)
		}
	case driver.PostgresDialect:
		target, err := postgresConflictTarget(db.drv.Dialect, targetCols, o)
		if err != nil {
			return "", nil, err
		}
		if len(updateCols) > 0 || len(assignmentParts) > 0 {
			assigns := make([]string, 0, len(updateCols)+len(assignmentParts))
			for _, c := range updateCols {
				assigns = append(assigns, fmt.Sprintf("%s=EXCLUDED.%s", quote(db.drv.Dialect, c), quote(db.drv.Dialect, c)))
			}
			assigns = append(assigns, assignmentParts...)
			sqlStr += fmt.Sprintf(" ON CONFLICT %s DO UPDATE SET %s", target, strings.Join(assigns, ", "))
		} else {
			sqlStr += fmt.Sprintf(" ON CONFLICT %s DO NOTHING", target)
		}
	default:
		return "", nil, fmt.Errorf("upsert not supported on dialect: %T", db.drv.Dialect)
	}
	sqlStr, err = appendReturningClause(db.drv.Dialect, sqlStr, o.returning)
	if err != nil {
		return "", nil, err
	}
	args = append(args, assignmentArgs...)
	return sqlStr, args, nil
}

func selectExistingInsertOnceRow[T any](ctx context.Context, db *DB, v any, o *writeOptions) (T, error) {
	var zero T
	table, values, pkCols, err := writeLookupValues(v, o)
	if err != nil {
		return zero, err
	}
	lookupCols := dedupeColumns(o.conflictCols)
	if len(lookupCols) == 0 {
		lookupCols = pkCols
	}
	if len(lookupCols) == 0 {
		return zero, fmt.Errorf("InsertOnceReturning existing-row lookup requires ConflictColumns or WherePK primary key columns")
	}

	q := db.Table(table).Select(o.returning...)
	for _, col := range lookupCols {
		value, ok := values[col]
		if !ok {
			return zero, fmt.Errorf("InsertOnceReturning lookup requires column %s", col)
		}
		q.Where(col, value)
	}
	if predicate := strings.TrimSpace(o.conflictWhere); predicate != "" {
		q.WhereRawNoArgs(predicate)
	}
	plan, err := q.Plan(ctx)
	if err != nil {
		return zero, err
	}
	if err := query.EnsurePlanExecutable(plan); err != nil {
		return zero, err
	}
	return SelectOne[T](ctx, db.RequireRawApproval("goquent generated insert-once lookup"), plan.SQL, plan.Params...)
}

func writeLookupValues(v any, o *writeOptions) (string, map[string]any, []string, error) {
	val := reflect.ValueOf(v)
	typ := val.Type()
	values := make(map[string]any)
	var table string
	var pkCols []string

	if isMapStringInterface(typ) {
		if o.table == "" {
			return "", nil, nil, fmt.Errorf("Table option required for map writes")
		}
		table = o.table
		iter := val.MapRange()
		for iter.Next() {
			values[iter.Key().String()] = iter.Value().Interface()
		}
		if o.wherePK {
			for col := range o.pkCols {
				pkCols = append(pkCols, col)
			}
			sort.Strings(pkCols)
		}
		return table, values, pkCols, nil
	}

	if typ.Kind() != reflect.Struct {
		return "", nil, nil, fmt.Errorf("unsupported type %s", typ)
	}
	table = o.table
	if table == "" {
		table = model.TableName(v)
	}
	meta, err := getTypeMeta(typ)
	if err != nil {
		return "", nil, nil, err
	}
	for _, fm := range meta.FieldsByName {
		fv := val.FieldByIndex(fm.IndexPath)
		values[fm.Col] = fv.Interface()
	}
	pkCols = append(pkCols, meta.PKCols...)
	return table, values, pkCols, nil
}

func conflictTargetColumns(o *writeOptions, pkCols []string) []string {
	if len(o.conflictCols) > 0 {
		return append([]string(nil), o.conflictCols...)
	}
	return append([]string(nil), pkCols...)
}

func upsertUpdateColumns(cols []string, targetCols []string, o *writeOptions) ([]string, error) {
	assignmentTargets := writeAssignmentTargets(o.assignments)
	if o.hasUpsertUpdates {
		updateCols := dedupeColumns(o.upsertUpdateCols)
		updateCols = filterColumns(updateCols, assignmentTargets)
		if err := ensureUpsertUpdateColumnsPresent(updateCols, cols); err != nil {
			return nil, err
		}
		return updateCols, nil
	}
	target := make(map[string]struct{}, len(targetCols))
	for _, col := range targetCols {
		target[col] = struct{}{}
	}
	updateCols := make([]string, 0, len(cols))
	for _, col := range cols {
		if _, ok := target[col]; ok {
			continue
		}
		if _, ok := assignmentTargets[col]; ok {
			continue
		}
		updateCols = append(updateCols, col)
	}
	return updateCols, nil
}

func filterColumns(cols []string, excluded map[string]struct{}) []string {
	if len(excluded) == 0 {
		return cols
	}
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		if _, ok := excluded[col]; ok {
			continue
		}
		out = append(out, col)
	}
	return out
}

func dedupeColumns(cols []string) []string {
	seen := make(map[string]struct{}, len(cols))
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		key := strings.TrimSpace(col)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func ensureConflictColumnsPresent(targetCols []string, cols []string) error {
	if len(targetCols) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		present[col] = struct{}{}
	}
	for _, col := range targetCols {
		if _, ok := present[col]; !ok {
			return fmt.Errorf("ConflictColumns requires column %s", col)
		}
	}
	return nil
}

func ensureUpsertUpdateColumnsPresent(updateCols []string, insertCols []string) error {
	if len(updateCols) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(insertCols))
	for _, col := range insertCols {
		present[col] = struct{}{}
	}
	for _, col := range updateCols {
		if _, ok := present[col]; !ok {
			return fmt.Errorf("UpdateColumns requires inserted column %s", col)
		}
	}
	return nil
}

func postgresConflictTarget(d driver.Dialect, cols []string, o *writeOptions) (string, error) {
	rawTarget := strings.TrimSpace(o.conflictTargetRaw)
	constraint := strings.TrimSpace(o.conflictConstraint)
	predicate := strings.TrimSpace(o.conflictWhere)
	if rawTarget != "" {
		if len(o.conflictCols) > 0 {
			return "", fmt.Errorf("ConflictTargetRaw cannot be combined with ConflictColumns")
		}
		if constraint != "" {
			return "", fmt.Errorf("ConflictTargetRaw cannot be combined with ConflictConstraint")
		}
		if predicate != "" {
			return "", fmt.Errorf("ConflictTargetRaw cannot be combined with ConflictWhere")
		}
		if err := validateWriteRawSQLFragment(rawTarget); err != nil {
			return "", err
		}
		return rawTarget, nil
	}
	if constraint != "" {
		if len(o.conflictCols) > 0 {
			return "", fmt.Errorf("ConflictConstraint cannot be combined with ConflictColumns")
		}
		if predicate != "" {
			return "", fmt.Errorf("ConflictConstraint cannot be combined with ConflictWhere")
		}
		return "ON CONSTRAINT " + quote(d, constraint), nil
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("Postgres upsert requires ConflictColumns or WherePK primary key columns")
	}
	quoted := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = quote(d, col)
	}
	target := "(" + strings.Join(quoted, ", ") + ")"
	if predicate != "" {
		if err := validateWriteRawSQLFragment(predicate); err != nil {
			return "", err
		}
		target += " WHERE " + predicate
	}
	return target, nil
}

func containsSQLWord(upperSQL, token string) bool {
	for i := 0; i+len(token) <= len(upperSQL); i++ {
		if upperSQL[i:i+len(token)] != token {
			continue
		}
		beforeOK := i == 0 || !isSQLWordByte(upperSQL[i-1])
		after := i + len(token)
		afterOK := after >= len(upperSQL) || !isSQLWordByte(upperSQL[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isSQLWordByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
