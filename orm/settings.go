package orm

import (
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

// PolicySet is an immutable collection of table policies.
type PolicySet = query.PolicySet

// Settings owns policy/risk/context snapshots; the zero value imports no globals.
type Settings = query.Settings

// ExecutionContext retains application data without asserting authentication.
type ExecutionContext = query.ExecutionContext

// ExecutionContextInput records caller-supplied provenance and tenant presence.
type ExecutionContextInput = query.ExecutionContextInput

var ErrUnsupportedExecutionContext = query.ErrUnsupportedExecutionContext
var ErrUnsupportedRiskEngine = query.ErrUnsupportedRiskEngine

func NewPolicySet(policies ...TablePolicy) (PolicySet, error) { return query.NewPolicySet(policies...) }
func NewExecutionContext(input ExecutionContextInput) (ExecutionContext, error) {
	return query.NewExecutionContext(input)
}
func NewSettings(p PolicySet, r RiskConfig, c ExecutionContext) Settings {
	return query.NewSettings(p, r, c)
}
func SnapshotDefaultSettings() Settings { return query.SnapshotDefaultSettings() }

// WithSettings explicitly replaces all settings, including any legacy defaults.
func WithSettings(s Settings) Option { return func(db *DB) { db.settings = s } }
func WithPolicySet(p PolicySet) Option {
	return func(db *DB) { db.settings = db.settings.WithPolicySet(p) }
}

// WithRiskConfig captures config when the option is created, not when applied.
func WithRiskConfig(r RiskConfig) Option {
	captured := query.NewSettings(PolicySet{}, r, ExecutionContext{})
	return func(db *DB) { db.settings = db.settings.WithRiskConfig(captured.RiskConfig()) }
}
func WithExecutionContext(c ExecutionContext) Option {
	return func(db *DB) { db.settings = db.settings.WithExecutionContext(c) }
}

// Settings returns the immutable settings snapshot, with detached data getters.
func (db *DB) Settings() Settings { return db.settings }

// Clone copies this DB wrapper. Connection ownership and executor are unchanged;
// Close has the same semantics as on the source. Settings are immutable.
func (db *DB) Clone() *DB {
	next := *db
	next.rawTables = append([]string(nil), db.rawTables...)
	return &next
}

// WithOptions derives a DB without changing existing DBs, queries or transactions.
// Options run in order; the last explicit value for each setting wins.
func (db *DB) WithOptions(opts ...Option) *DB {
	next := db.Clone()
	for _, o := range opts {
		o(next)
	}
	return next
}

// WrapExecutor inherits this DB's settings, dialect and scan/raw options while
// using an external executor. It neither owns/closes that executor nor acquires
// the ability to begin transactions. Use WrapTx for its legacy ownership rules.
func (db *DB) WrapExecutor(exec Executor, opts ...Option) *DB {
	next := db.WithOptions(opts...)
	next.drv = &driver.Driver{Dialect: db.Dialect()}
	next.exec = exec
	return next
}

// ApplicationSchema is an immutable application assertion, not live schema evidence.
type ApplicationSchema = query.ApplicationSchema
type ApplicationSchemaInput = query.ApplicationSchemaInput
type ApplicationTable = query.ApplicationTable

func NewApplicationSchema(input ApplicationSchemaInput) (ApplicationSchema, error) {
	return query.NewApplicationSchema(input)
}
func NewApplicationTenantContext(input ExecutionContextInput) (ExecutionContext, error) {
	return query.NewApplicationTenantContext(input)
}

// WithTenantPolicy opts documented Query paths into conditional strict inspection.
func WithTenantPolicy(database string, schema ApplicationSchema, automatic bool) Option {
	return func(db *DB) { db.settings = db.settings.WithTenantPolicy(database, schema, automatic) }
}
