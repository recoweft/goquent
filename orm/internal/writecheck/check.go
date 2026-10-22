// Package writecheck owns finite result expectations shared by write dispatchers.
package writecheck

import (
	"database/sql"
	"errors"
)

var ErrOptions = errors.New("goquent: invalid or unsupported write options")
var ErrOwnership = errors.New("goquent: atomic write requires an owned database")
var ErrOutcomeUnknown = errors.New("goquent: write outcome unknown; reconcile before retry")

type Version struct {
	Column   string
	Expected int64
}
type Options struct {
	Expected *int64
	Zero     error
	Atomic   bool
	Version  *Version
	Invalid  bool
}

func (o Options) Clone() Options {
	if o.Expected != nil {
		n := *o.Expected
		o.Expected = &n
	}
	if o.Version != nil {
		v := *o.Version
		o.Version = &v
	}
	return o
}
func (o Options) Active() bool {
	return o.Expected != nil || o.Zero != nil || o.Atomic || o.Version != nil || o.Invalid
}
func (o Options) Validate() error {
	if o.Invalid || (o.Expected != nil && *o.Expected < 0) {
		return ErrOptions
	}
	if o.Version != nil && (o.Version.Expected < 0 || o.Version.Column == "" || (o.Expected != nil && *o.Expected != 1) || o.Zero != nil) {
		return ErrOptions
	}
	return nil
}
func (o Options) Check(n int64) error {
	if n < 0 {
		return ErrOptions
	}
	expected, zero := o.Expected, o.Zero
	if o.Version != nil {
		one := int64(1)
		expected = &one
		zero = ErrConflict
	}
	if expected != nil {
		if n == *expected {
			return nil
		}
		cause := ErrRowsAffected
		if n == 0 && zero != nil {
			cause = zero
		}
		return RowsAffectedError{Expected: *expected, Actual: n, Cause: cause}
	}
	if n == 0 && zero != nil {
		return RowsAffectedError{Expected: 1, Actual: n, Cause: zero}
	}
	return nil
}
func (o Options) Result(r sql.Result) error { _, _, e := o.ResultCount(r); return e }

// ResultCount reuses the expectation's existing RowsAffected call, never adds one.
func (o Options) ResultCount(r sql.Result) (int64, bool, error) {
	if !o.Active() {
		return 0, false, nil
	}
	if r == nil {
		return 0, false, ErrOptions
	}
	n, e := r.RowsAffected()
	if e != nil {
		return 0, false, e
	}
	return n, n >= 0, o.Check(n)
}
