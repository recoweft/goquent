package query

import "errors"

// ErrInvalidLimit identifies a negative LimitExact input without retaining it.
var ErrInvalidLimit = errors.New("goquent: limit must be non-negative")

// LimitExact sets an explicit SELECT limit, including zero rows. Its range is
// 0..max int for the current architecture. Legacy Limit and Take retain their
// zero-as-unlimited behavior and clear this state. Errors remain sticky.
func (q *Query) LimitExact(n int) *Query {
	if q.err != nil {
		return q
	}
	if n < 0 {
		q.err = ErrInvalidLimit
		return q
	}
	q.builder.LimitExact(int64(n))
	return q
}

func (q *Query) checkExactWrite() error {
	if q.err != nil {
		return q.err
	}
	if q.builder.HasExactLimit() {
		return ErrBlockedOperation
	}
	return nil
}
