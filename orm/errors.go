package orm

import (
	"database/sql"
	"errors"
	"github.com/recoweft/goquent/orm/internal/writecheck"
)

// ErrNotFound is returned by one-row read helpers when no row is available.
//
// It aliases sql.ErrNoRows for compatibility with existing callers.
var ErrNotFound = sql.ErrNoRows

// Write error identities are shared with the private execution layer.
var ErrConflict = writecheck.ErrConflict
var ErrRowsAffected = writecheck.ErrRowsAffected

type RowsAffectedError = writecheck.RowsAffectedError

var ErrWriteOptions = writecheck.ErrOptions
var ErrWriteOwnership = writecheck.ErrOwnership
var ErrWriteOutcomeUnknown = writecheck.ErrOutcomeUnknown

// IsNotFound reports whether err represents a no-row result.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows)
}

// IsConflict reports whether err represents an explicit write conflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}
