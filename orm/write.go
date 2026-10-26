package orm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/observe"
	"github.com/recoweft/goquent/orm/internal/writecheck"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

// WriteOpt configures write behavior.
type WriteOpt func(*writeOptions)

type writeOptions struct {
	results            writecheck.Options
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
		if o.expectAffected != nil {
			o.results.Invalid = true
		}
		o.expectAffected = &n
		o.results.Expected = &n
	}
}

// NoRowsAs maps a zero-row write result to err. Use ErrConflict for explicit
// optimistic-concurrency guards such as content_hash or version predicates.
func NoRowsAs(err error) WriteOpt {
	return func(o *writeOptions) {
		if o.zeroRowsErr != nil {
			o.results.Invalid = true
		}
		o.zeroRowsErr = err
		o.results.Zero = err
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

// AtomicWrite completes the write and result inspection in an owned transaction.
func AtomicWrite() WriteOpt { return func(o *writeOptions) { o.results.Atomic = true } }

// OptimisticVersion compares and advances a supported signed version in one UPDATE.
// It requires an owned transaction and exactly one affected row.
func OptimisticVersion(column string, expected int64) WriteOpt {
	return func(o *writeOptions) {
		if o.results.Version != nil {
			o.results.Invalid = true
		}
		o.results.Version = &writecheck.Version{Column: column, Expected: expected}
	}
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
	return queryWriteAll[R](ctx, db, input, o)
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
	return queryWriteAll[R](ctx, db, input, o)
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
	if o.results.Active() {
		return zero, false, ErrWriteOptions
	}
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
	var parent compoundPlan
	if err = parent.add(ctx, db, "insert", 0, 1, input, nil); err != nil {
		return zero, false, err
	}
	lookupCols := dedupeColumns(input.Options.Conflict.Columns)
	if len(lookupCols) == 0 {
		return zero, false, fmt.Errorf("InsertOnceReturning existing-row lookup requires ConflictColumns or WherePK primary key columns")
	}
	q := db.Table(input.Table).Select(o.returning...)
	for _, col := range lookupCols {
		value, ok := parent.steps[0].plan.InsertRows[0][col]
		if !ok {
			return zero, false, fmt.Errorf("InsertOnceReturning lookup requires column %s", col)
		}
		q.Where(col, value)
	}
	if predicate := strings.TrimSpace(o.conflictWhere); predicate != "" {
		q.WhereRawNoArgs(predicate)
	}
	if err = parent.add(ctx, db, "lookup", 0, 1, querybridge.Request{Base: q, Operation: "select"}, nil); err != nil {
		return zero, false, err
	}
	var out T
	scan := func(rows observe.Reader) error { var e error; out, e = scanRowsOne[T](db, rows); return e }
	err = parent.steps[0].plan.Scan(scan)
	if err == nil {
		return out, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return zero, false, err
	}
	err = parent.steps[1].plan.Scan(scan)
	if err != nil {
		return zero, false, err
	}
	return out, false, nil
}
