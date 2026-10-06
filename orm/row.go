package orm

import (
	"database/sql"
	"errors"
)

var errUninitializedRow = errors.New("goquent: uninitialized Row")

// Row retains a pre-dispatch refusal or delegates to a database/sql Row.
// QueryRow and QueryRowContext now return this type in every mode. Callers that
// require *sql.Row must use QueryRowE and check its error first.
type Row struct {
	row *sql.Row
	err error
}

// Err returns the original refusal or the underlying Row's error.
func (r *Row) Err() error {
	if r == nil {
		return errUninitializedRow
	}
	if r.err != nil {
		return r.err
	}
	if r.row == nil {
		return errUninitializedRow
	}
	return r.row.Err()
}

// Scan delegates without additional dispatch. A rejected Row never writes dest.
func (r *Row) Scan(dest ...any) error {
	if err := r.Err(); err != nil {
		return err
	}
	return r.row.Scan(dest...)
}
