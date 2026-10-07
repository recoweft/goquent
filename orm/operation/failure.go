package operation

import (
	"fmt"
	"github.com/recoweft/goquent/orm/internal/querybridge"
)

// Private validation history. Public diagnostics never retain this error.
type validationFailure struct {
	cause       error
	diagnostics []querybridge.OperationDiagnostic
	checks      []querybridge.OperationCheck
}

func (e *validationFailure) Error() string { return "goquent operation: validation failed" }
func (e *validationFailure) Unwrap() error { return e.cause }

func (e *validationFailure) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(e.Error())) }
