package orm

import (
	"context"

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
