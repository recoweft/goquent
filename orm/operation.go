package orm

import (
	"context"
	"database/sql"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/operation"
)

type OperationSpec = operation.OperationSpec
type FilterSpec = operation.FilterSpec
type OrderSpec = operation.OrderSpec
type OperationOptions = operation.Options

const (
	OperationSpecSelect                = operation.OperationSelect
	WarningOperationSpecPIISelected    = operation.WarningOperationPIISelected
	WarningOperationSpecRequiredFilter = operation.WarningOperationRequiredFilter
	WarningOperationSpecMissingLimit   = operation.WarningOperationMissingLimit
	WarningOperationSpecStaleManifest  = operation.WarningOperationStaleManifest
)

var (
	ErrOperationManifestRequired        = operation.ErrManifestRequired
	ErrOperationUnsupportedOperation    = operation.ErrUnsupportedOperation
	ErrOperationModelRequired           = operation.ErrModelRequired
	ErrOperationUnknownModel            = operation.ErrUnknownModel
	ErrOperationSelectRequired          = operation.ErrSelectRequired
	ErrOperationUnknownField            = operation.ErrUnknownField
	ErrOperationForbiddenField          = operation.ErrForbiddenField
	ErrOperationInvalidFilter           = operation.ErrInvalidFilter
	ErrOperationInvalidOrder            = operation.ErrInvalidOrder
	ErrOperationRequiredFilterMissing   = operation.ErrRequiredFilterMissing
	ErrOperationPIIAccessReasonRequired = operation.ErrPIIAccessReasonRequired
	ErrOperationStaleManifest           = operation.ErrStaleManifest
)

func CompileOperationSpec(ctx context.Context, spec OperationSpec, opts OperationOptions) (*QueryPlan, error) {
	return operation.Compile(ctx, spec, opts)
}

func ValidateOperationSpec(spec OperationSpec, opts OperationOptions) ([]Warning, error) {
	return operation.Validate(spec, opts)
}

func OperationSpecJSONSchema() ([]byte, error) {
	return operation.JSONSchema()
}

// CompileOperation plans a read-only operation using this DB's current immutable
// settings and dialect. Caller options cannot replace them. No SQL is executed.
func (db *DB) CompileOperation(ctx context.Context, spec operation.OperationSpec, opts operation.Options) (*QueryPlan, error) {
	s := db.Settings()
	opts.Settings = &s
	opts.Dialect = db.Dialect()
	return operation.Compile(ctx, spec, opts)
}

// ValidateOperation uses the same DB-scoped validation and DB-free planner.
func (db *DB) ValidateOperation(spec operation.OperationSpec, opts operation.Options) ([]Warning, error) {
	s := db.Settings()
	opts.Settings = &s
	opts.Dialect = db.Dialect()
	return operation.Validate(spec, opts)
}

// CompileOperationSpecWithDiagnostics adds detached public diagnostics.
func CompileOperationSpecWithDiagnostics(ctx context.Context, spec OperationSpec, opts OperationOptions) (*QueryPlan, operation.DiagnosticView, error) {
	return operation.CompileWithDiagnostics(ctx, spec, opts)
}

// ValidateOperationSpecWithDiagnostics uses the shared compiler once.
func ValidateOperationSpecWithDiagnostics(spec OperationSpec, opts OperationOptions) ([]Warning, operation.DiagnosticView, error) {
	return operation.ValidateWithDiagnostics(spec, opts)
}

// CompileOperationWithDiagnostics forces this DB's immutable settings and dialect.
func (db *DB) CompileOperationWithDiagnostics(ctx context.Context, spec OperationSpec, opts OperationOptions) (*QueryPlan, operation.DiagnosticView, error) {
	s := db.Settings()
	opts.Settings = &s
	opts.Dialect = db.Dialect()
	return operation.CompileWithDiagnostics(ctx, spec, opts)
}

// ValidateOperationWithDiagnostics forces this DB's immutable settings and dialect.
func (db *DB) ValidateOperationWithDiagnostics(spec OperationSpec, opts OperationOptions) ([]Warning, operation.DiagnosticView, error) {
	s := db.Settings()
	opts.Settings = &s
	opts.Dialect = db.Dialect()
	return operation.ValidateWithDiagnostics(spec, opts)
}

// SelectOperationBy validates and executes a single-model SELECT using the DB's
// immutable settings, dialect and executor. It scans through the generic read
// path, including DB bool policy. CompileOperation remains nonexecuting.
// This immediate read does not provide a reusable ValidatedPlan/current binding.
func SelectOperationBy[T any](ctx context.Context, db *DB, spec OperationSpec, opts OperationOptions) (out []T, err error) {
	if db == nil {
		return nil, operation.ErrInvalidFilter
	}
	s := db.Settings()
	opts.Settings = &s
	opts.Dialect = db.Dialect()
	prepared, err := querybridge.PrepareOperation(ctx, spec, opts, db.exec)
	if err != nil {
		return nil, err
	}
	err = prepared.Scan(func(rows *sql.Rows) error { out, err = scanRowsAll[T](db, rows); return err })
	return out, err
}
