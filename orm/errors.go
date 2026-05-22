package orm

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by one-row read helpers when no row is available.
//
// It aliases sql.ErrNoRows for compatibility with existing callers.
var ErrNotFound = sql.ErrNoRows

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

// IsNotFound reports whether err represents a no-row result.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}

// IsConflict reports whether err represents an explicit write conflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}
