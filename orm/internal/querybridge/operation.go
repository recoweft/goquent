package querybridge

// OperationCheck is internal handoff data, never a caller authorization verdict.
// The operation compiler owns its strings and does not retain arbitrary values.
type OperationCheck struct {
	Position                                              int
	Field, TypeSource, Type, Checked, Missing, Provenance string
	DriverStatus, BindingStatus                           string
	ValuePresent, RefPresent                              bool
}
type OperationValidation struct {
	Checks       []OperationCheck
	Unknown      []string
	WarningCodes []string
}

var ConfigureOperation func(any, OperationValidation) error
var AutomaticTenant func(any) bool
