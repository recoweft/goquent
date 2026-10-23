package orm

import "github.com/recoweft/goquent/orm/query"

type AuthorizationProvider = query.AuthorizationProvider
type Authorization = query.Authorization
type AuthorizationRequest = query.AuthorizationRequest
type AuthorizationDecision = query.AuthorizationDecision

var (
	ErrAuthorizationMissing  = query.ErrAuthorizationMissing
	ErrAuthorizationDenied   = query.ErrAuthorizationDenied
	ErrAuthorizationProvider = query.ErrAuthorizationProvider
	ErrAuthorizationInvalid  = query.ErrAuthorizationInvalid
	ErrAuthorizationMismatch = query.ErrAuthorizationMismatch
	ErrAuthorizationExpired  = query.ErrAuthorizationExpired
	ErrAuthorizationCanceled = query.ErrAuthorizationCanceled
)

func NewAuthorization(provider AuthorizationProvider, subject string) (Authorization, error) {
	return query.NewAuthorization(provider, subject)
}

// WithAuthorization installs an explicit registration on this DB's settings.
// High-risk execution is supported only through the six validated Query families.
// Ordinary high-risk paths refuse; Plan and Validate never call the provider.
func WithAuthorization(a Authorization) Option {
	return func(db *DB) { db.settings = db.settings.WithAuthorization(a) }
}
