package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"github.com/recoweft/goquent/orm/model"
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

type writeAssignment = writeinput.Assignment

const (
	writeAssignmentRaw       = writeinput.Raw
	writeAssignmentExpr      = writeinput.Expr
	writeAssignmentColumn    = writeinput.Column
	writeAssignmentIncrement = writeinput.Increment
)

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
			Column:     column,
			Expression: expression,
			Kind:       writeAssignmentRaw,
		})
	}
}

// SetExpr adds a database-side assignment with positional ? placeholders.
// Placeholders are rewritten for the active dialect.
func SetExpr(column, expression string, args ...any) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			Column:     column,
			Expression: expression,
			Args:       append([]any(nil), args...),
			Kind:       writeAssignmentExpr,
		})
	}
}

// SetColumn assigns one column from another column.
func SetColumn(column, sourceColumn string) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			Column:       column,
			SourceColumn: sourceColumn,
			Kind:         writeAssignmentColumn,
		})
	}
}

// Increment adds delta to column on Update or the conflict-update side of
// Upsert.
func Increment(column string, delta any) WriteOpt {
	return func(o *writeOptions) {
		o.assignments = append(o.assignments, writeAssignment{
			Column: column,
			Args:   []any{delta},
			Kind:   writeAssignmentIncrement,
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

type returningResult struct {
	rowsAffected int64
}

func (r returningResult) LastInsertId() (int64, error) {
	return 0, fmt.Errorf("LastInsertId is not supported for RETURNING statements")
}

func (r returningResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
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

// Insert inserts v into its table.
func Insert[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	input, err := buildInsertInput(db, v, o)
	if err != nil {
		return nil, err
	}
	return execWriteInput(ctx, db, input, o)
}

// InsertReturning inserts v and scans the Postgres RETURNING row into T.
func InsertReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, err
	}
	input, err := buildInsertInput(db, v, o)
	if err != nil {
		return zero, err
	}
	return queryWriteOneWithOptions[T](ctx, db, input, o)
}

// InsertMany inserts all values in one INSERT statement.
//
// Empty slices return an error instead of a no-op result. Map writes require
// Table, and every row must provide the selected column set.
func InsertMany[T any](ctx context.Context, db *DB, values []T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	input, err := buildInsertManyInput(db, values, o)
	if err != nil {
		return nil, err
	}
	return execWriteInput(ctx, db, input, o)
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
	input, err := buildInsertManyInput(db, values, o)
	if err != nil {
		return nil, err
	}
	return queryWriteAll[R](ctx, db, input)
}

// UpsertMany inserts or updates all values in one bulk UPSERT statement.
//
// It supports the same conflict-target options as Upsert. Empty slices return
// an error instead of a no-op result. Map writes require Table, and every row
// must provide the selected column set.
func UpsertMany[T any](ctx context.Context, db *DB, values []T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	input, err := buildUpsertManyInput(db, values, o)
	if err != nil {
		return nil, err
	}
	return execWriteInput(ctx, db, input, o)
}

// UpsertManyReturning upserts all values and scans PostgreSQL RETURNING rows.
func UpsertManyReturning[R any, T any](ctx context.Context, db *DB, values []T, opts ...WriteOpt) ([]R, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("goquent: no rows to upsert")
	}
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[R](o); err != nil {
		return nil, err
	}
	input, err := buildUpsertManyInput(db, values, o)
	if err != nil {
		return nil, err
	}
	return queryWriteAll[R](ctx, db, input)
}

func buildInsertInput(db *DB, v any, o *writeOptions) (querybridge.Request, error) {
	if len(o.assignments) > 0 {
		return querybridge.Request{}, fmt.Errorf("assignment options are not supported for Insert")
	}
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := val.Type()
	var table string
	var cols []string
	var args []any

	if isMapStringInterface(typ) {
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
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
		}
		sort.Strings(cols)
		for _, col := range cols {
			args = append(args, val.MapIndex(reflect.ValueOf(col).Convert(typ.Key())).Interface())
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		var err error
		cols, args, err = insertStructColumnsAndArgs(val, o)
		if err != nil {
			return querybridge.Request{}, err
		}
	} else {
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if len(cols) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to insert")
	}
	return newWriteInput("insert", table, cols, args, 1, nil, o)
}

func buildInsertManyInput[T any](db *DB, values []T, o *writeOptions) (querybridge.Request, error) {
	if len(values) == 0 {
		return querybridge.Request{}, fmt.Errorf("goquent: no rows to insert")
	}
	if err := validateInsertManyOptions(o); err != nil {
		return querybridge.Request{}, err
	}

	first := reflect.ValueOf(values[0])
	if !first.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
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
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
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
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if err != nil {
		return querybridge.Request{}, err
	}
	r, err := newWriteInput("insert", table, cols, args, len(values), nil, o)
	r.Batch = true
	return r, err
}

func buildUpsertManyInput[T any](db *DB, values []T, o *writeOptions) (querybridge.Request, error) {
	if len(values) == 0 {
		return querybridge.Request{}, fmt.Errorf("goquent: no rows to upsert")
	}
	if !o.wherePK && !o.hasConflictTarget() {
		return querybridge.Request{}, fmt.Errorf("UpsertMany[T] requires WherePK, ConflictColumns, or ConflictConstraint")
	}
	if o.conflictDoNothing && (len(o.assignments) > 0 || len(o.upsertUpdateCols) > 0) {
		return querybridge.Request{}, fmt.Errorf("ConflictDoNothing cannot be combined with update or assignment options")
	}

	first := reflect.ValueOf(values[0])
	if !first.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := first.Type()

	var (
		table  string
		cols   []string
		args   []any
		pkCols []string
		err    error
	)
	switch {
	case isMapStringInterface(typ):
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		if o.wherePK && len(o.pkCols) == 0 {
			return querybridge.Request{}, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
		}
		table = o.table
		cols, args, pkCols, err = upsertManyMapColumnsArgsAndPK(values, o)
	case typ.Kind() == reflect.Struct:
		table = o.table
		if table == "" {
			table = model.TableName(values[0])
		}
		cols, args, pkCols, err = upsertManyStructColumnsArgsAndPK(values, typ, o)
	default:
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if err != nil {
		return querybridge.Request{}, err
	}
	if o.wherePK && len(pkCols) == 0 {
		return querybridge.Request{}, fmt.Errorf("WherePK requires pk values")
	}
	if len(cols) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to insert")
	}
	r, err := newWriteInput("upsert", table, cols, args, len(values), pkCols, o)
	r.Batch = true
	return r, err
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

func upsertManyStructColumnsArgsAndPK[T any](values []T, typ reflect.Type, o *writeOptions) ([]string, []any, []string, error) {
	meta, err := getTypeMeta(typ)
	if err != nil {
		return nil, nil, nil, err
	}
	var cols []string
	args := make([]any, 0, len(values))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany row %d is nil", i)
		}
		if val.Type() != typ {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany row %d has type %s, expected %s", i, val.Type(), typ)
		}
		rowCols, rowArgs := upsertStructColumnsAndArgs(val, meta, o)
		if i == 0 {
			cols = rowCols
		} else if !sameColumns(cols, rowCols) {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany requires identical columns in every row")
		}
		args = append(args, rowArgs...)
	}
	return cols, args, append([]string(nil), meta.PKCols...), nil
}

func upsertStructColumnsAndArgs(val reflect.Value, meta *typeMeta, o *writeOptions) ([]string, []any) {
	cols := make([]string, 0, len(meta.Fields))
	args := make([]any, 0, len(meta.Fields))
	for _, fm := range meta.Fields {
		fv := val.FieldByIndex(fm.IndexPath)
		if fm.PK {
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
	return cols, args
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

func upsertManyMapColumnsArgsAndPK[T any](values []T, o *writeOptions) ([]string, []any, []string, error) {
	first := reflect.ValueOf(values[0])
	cols := mapUpsertManyColumnSet(first, o)
	if len(cols) == 0 {
		return nil, nil, nil, fmt.Errorf("no columns to insert")
	}
	pkCols := sortedPKColumns(o)
	args := make([]any, 0, len(values)*len(cols))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany map row %d is nil", i)
		}
		if !isMapStringInterface(val.Type()) {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany row %d has type %s, expected %s", i, val.Type(), first.Type())
		}
		if !sameColumns(cols, mapUpsertManyColumnSet(val, o)) {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany map row %d has inconsistent columns", i)
		}
		rowArgs, err := mapInsertManyRowArgs(val, cols, i)
		if err != nil {
			return nil, nil, nil, err
		}
		args = append(args, rowArgs...)
	}
	return cols, args, pkCols, nil
}

func mapUpsertManyColumnSet(val reflect.Value, o *writeOptions) []string {
	cols := make([]string, 0, val.Len())
	iter := val.MapRange()
	for iter.Next() {
		col := iter.Key().String()
		if o.isPK(col) || o.isConflictColumn(col) {
			cols = append(cols, col)
			continue
		}
		if len(o.cols) > 0 {
			if _, ok := o.cols[col]; !ok {
				continue
			}
		}
		if _, omitted := o.omit[col]; omitted {
			continue
		}
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

func sortedPKColumns(o *writeOptions) []string {
	if len(o.pkCols) == 0 {
		return nil
	}
	cols := make([]string, 0, len(o.pkCols))
	for col := range o.pkCols {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
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

// Update updates record v.
func Update[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	input, err := buildUpdateInput(db, v, o)
	if err != nil {
		return nil, err
	}
	return execWriteInput(ctx, db, input, o)
}

// UpdateReturning updates v and scans the Postgres RETURNING row into T.
func UpdateReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, err
	}
	input, err := buildUpdateInput(db, v, o)
	if err != nil {
		return zero, err
	}
	return queryWriteOneWithOptions[T](ctx, db, input, o)
}

func buildUpdateInput(db *DB, v any, o *writeOptions) (querybridge.Request, error) {
	if !o.wherePK {
		return querybridge.Request{}, fmt.Errorf("Update[T] without WherePK is not allowed")
	}
	assignmentTargets := writeinput.Targets(o.assignments)
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := val.Type()
	var table string
	var setCols []string
	var setArgs []any
	var whereCols []string
	var whereArgs []any

	if isMapStringInterface(typ) {
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		if len(o.pkCols) == 0 {
			return querybridge.Request{}, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
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
				return querybridge.Request{}, fmt.Errorf("WherePK requires pk column %s", pk)
			}
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		meta, err := getTypeMeta(typ)
		if err != nil {
			return querybridge.Request{}, err
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
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if len(whereCols) == 0 {
		return querybridge.Request{}, fmt.Errorf("WherePK requires pk values")
	}
	if len(setCols) == 0 && len(o.assignments) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to update")
	}
	r, err := newWriteInput("update", table, setCols, setArgs, 1, nil, o)
	r.WhereColumns = whereCols
	r.WhereValues = whereArgs
	return r, err
}

// Upsert inserts or updates v using primary keys.
func Upsert[T any](ctx context.Context, db *DB, v T, opts ...WriteOpt) (sql.Result, error) {
	o := applyWriteOpts(opts)
	input, err := buildUpsertInput(db, v, o)
	if err != nil {
		return nil, err
	}
	return execWriteInput(ctx, db, input, o)
}

// UpsertReturning upserts v and scans the Postgres RETURNING row into T.
func UpsertReturning[T any, V any](ctx context.Context, db *DB, v V, opts ...WriteOpt) (T, error) {
	var zero T
	o := applyWriteOpts(opts)
	if err := ensureReturningColumns[T](o); err != nil {
		return zero, err
	}
	input, err := buildUpsertInput(db, v, o)
	if err != nil {
		return zero, err
	}
	return queryWriteOneWithOptions[T](ctx, db, input, o)
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

	input, err := buildUpsertInput(db, v, o)
	if err != nil {
		return zero, false, err
	}
	inserted, err := queryWriteOne[T](ctx, db, input)
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

func buildUpsertInput(db *DB, v any, o *writeOptions) (querybridge.Request, error) {
	if !o.wherePK && !o.hasConflictTarget() {
		return querybridge.Request{}, fmt.Errorf("Upsert[T] requires WherePK, ConflictColumns, or ConflictConstraint")
	}
	if o.conflictDoNothing && (len(o.assignments) > 0 || len(o.upsertUpdateCols) > 0) {
		return querybridge.Request{}, fmt.Errorf("ConflictDoNothing cannot be combined with update or assignment options")
	}
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := val.Type()
	var table string
	var cols []string
	var args []any
	var pkCols []string

	if isMapStringInterface(typ) {
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		if o.wherePK && len(o.pkCols) == 0 {
			return querybridge.Request{}, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
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
					return querybridge.Request{}, fmt.Errorf("WherePK requires pk column %s", pk)
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
			return querybridge.Request{}, err
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
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if o.wherePK && len(pkCols) == 0 {
		return querybridge.Request{}, fmt.Errorf("WherePK requires pk values")
	}
	if len(cols) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to insert")
	}
	return newWriteInput("upsert", table, cols, args, 1, pkCols, o)
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
	return SelectOneBy[T](ctx, db, q)
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
	assignmentTargets := writeinput.Targets(o.assignments)
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
