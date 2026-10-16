package writecheck

import (
	"errors"
	"fmt"
)

// ErrConflict represents an explicit optimistic-concurrency or stale-write
// conflict chosen by the caller.
var ErrConflict = errors.New("goquent: conflict")

// ErrRowsAffected reports that a write did not affect the requested number of
// rows.
var ErrRowsAffected = errors.New("goquent: unexpected rows affected")

// RowsAffectedError describes a failed rows-affected expectation.
type RowsAffectedError struct {
	Expected int64
	Actual   int64
	Cause    error
}

func (e RowsAffectedError) Error() string {
	if e.Cause != nil && !errors.Is(e.Cause, ErrRowsAffected) {
		return fmt.Sprintf("%s: expected %d row(s), affected %d: %v", ErrRowsAffected, e.Expected, e.Actual, e.Cause)
	}
	return fmt.Sprintf("%s: expected %d row(s), affected %d", ErrRowsAffected, e.Expected, e.Actual)
}

func (e RowsAffectedError) Unwrap() error {
	if e.Cause != nil {
		return e.Cause
	}
	return ErrRowsAffected
}

func (e RowsAffectedError) Is(target error) bool {
	if target == ErrRowsAffected {
		return true
	}
	return e.Cause != nil && errors.Is(e.Cause, target)
}
