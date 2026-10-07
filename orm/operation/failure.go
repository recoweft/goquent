package operation

import "github.com/recoweft/goquent/orm/internal/querybridge"

// Retained internally for the PR2 diagnostic projection. No arbitrary input
// value is retained or interpolated into Error, and no new public result exists.
type validationFailure struct {
	cause  error
	checks []querybridge.OperationCheck
}

func (e *validationFailure) Error() string { return e.cause.Error() }
func (e *validationFailure) Unwrap() error { return e.cause }
