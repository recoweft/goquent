package orm

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/observe"
)

var errUninitializedRow = errors.New("goquent: uninitialized Row")

// Row retains a pre-dispatch refusal or delegates to a database/sql Row.
// QueryRow and QueryRowContext now return this type in every mode. Callers that
// require *sql.Row must use QueryRowE and check its error first.
type Row struct {
	observed *observe.Row
	row      *sql.Row
	err      error
}

// Err returns the original refusal or the underlying Row's error.
func (r *Row) Err() error {
	if r == nil {
		return errUninitializedRow
	}
	if r.err != nil {
		return r.err
	}
	if r.observed != nil {
		return r.observed.Err()
	}
	if r.row == nil {
		return errUninitializedRow
	}
	return r.row.Err()
}

// Scan delegates without additional dispatch. A rejected Row never writes dest.
func (r *Row) Scan(dest ...any) error {
	if r != nil && r.err == nil && r.observed != nil {
		return r.observed.Scan(dest...)
	}
	if err := r.Err(); err != nil {
		return err
	}
	if r.observed != nil {
		return r.observed.Scan(dest...)
	}
	return r.row.Scan(dest...)
}

func (r *Row) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte("goquent.Row")) }
