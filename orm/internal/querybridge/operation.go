package querybridge

import "context"

// OperationCheck is internal handoff data, never a caller authorization verdict.
// The operation compiler owns its strings and does not retain arbitrary values.
type OperationCheck struct {
	Position                                              int
	Field, TypeSource, Type, Checked, Missing, Provenance string
	DriverStatus, BindingStatus                           string
	ValuePresent, RefPresent                              bool
}
type OperationValidation struct {
	Diagnostics  []OperationDiagnostic
	Checks       []OperationCheck
	Unknown      []string
	WarningCodes []string
}

var ConfigureOperation func(any, OperationValidation) error
var AutomaticTenant func(any) bool

// OperationDiagnostic is private source detail. It contains declarations and
// fixed inspection facts, never argument values, executable material or errors.
type OperationDiagnostic struct {
	Code, Status                                                string
	Source, Section, Member, Origin                             string
	Index, Element                                              int
	IndexKnown, ElementKnown                                    bool
	Table, Qualifier, Field, Declaration, TypeSource            string
	Checked, Missing, Evidence, Action                          string
	Bits, Precision, Scale, Length, Fraction                    int
	Unsigned, NullableKnown, Nullable, ValuePresent, RefPresent bool
}

var OperationInputOrigin func(any, string)

// PrepareOperation is installed by operation. Only the facade supplies the executor.
var PrepareOperation func(context.Context, any, any, Executor) (Planned, error)

// SealedSelect captures the exact already finalized Query plan without rebuilding.
// It is internal and refuses any plan lacking that Query's private seal.
var SealedSelect func(any, any) (Planned, error)
