package orm

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/faciam-dev/goquent/orm/driver"
	"github.com/faciam-dev/goquent/orm/model"
	"github.com/faciam-dev/goquent/orm/query"
)

// Executor abstracts sql.DB, sql.Tx, and compatible transaction wrappers.
type Executor interface {
	// Query runs a SQL statement returning multiple rows.
	Query(query string, args ...any) (*sql.Rows, error)
	// QueryContext is the context-aware version of Query.
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	// QueryRow executes a query expected to return at most one row.
	QueryRow(query string, args ...any) *sql.Row
	// QueryRowContext executes a single-row query with context.
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	// Exec runs a SQL statement that doesn't return rows.
	Exec(query string, args ...any) (sql.Result, error)
	// ExecContext runs Exec with a context.
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// DB provides main ORM interface.
type DB struct {
	drv         *driver.Driver
	exec        Executor
	scanOpts    ScanOptions
	rawApproval *query.Approval
	rawTables   []string
	rawErr      error
}

// Option configures DB at creation.
type Option func(*DB)

// WithBoolScanPolicy sets the bool scanning policy.
func WithBoolScanPolicy(p BoolScanPolicy) Option {
	return func(db *DB) { db.scanOpts.BoolPolicy = p }
}

// SQLDB returns the underlying *sql.DB.
func (db *DB) SQLDB() *sql.DB {
	if db.drv == nil {
		return nil
	}
	return db.drv.DB
}

// Database driver names.
const (
	MySQL    = "mysql"
	Postgres = "postgres"
)

func defaultDialect(name string) driver.Dialect {
	if strings.Contains(strings.ToLower(name), "postgres") {
		return driver.PostgresDialect{}
	}
	return driver.MySQLDialect{}
}

func newDB(d *driver.Driver, exec Executor, opts ...Option) *DB {
	db := &DB{drv: d, exec: exec, scanOpts: ScanOptions{BoolPolicy: BoolCompat}}
	for _, o := range opts {
		o(db)
	}
	return db
}

// RequireRawApproval returns a shallow DB copy that can execute risky raw SQL
// with an explicit approval reason.
func (db *DB) RequireRawApproval(reason string) *DB {
	next := *db
	reason = strings.TrimSpace(reason)
	if reason == "" {
		next.rawErr = query.ErrApprovalReasonRequired
		next.rawApproval = nil
		return &next
	}
	next.rawErr = nil
	next.rawApproval = &query.Approval{Reason: reason, CreatedAt: time.Now().UTC()}
	return &next
}

// TouchedTables returns a shallow DB copy that annotates raw SQL QueryPlans
// with the tables the caller reviewed.
func (db *DB) TouchedTables(tables ...string) *DB {
	next := *db
	next.rawTables = append([]string(nil), db.rawTables...)
	seen := make(map[string]struct{}, len(next.rawTables)+len(tables))
	for _, table := range next.rawTables {
		seen[strings.ToLower(strings.TrimSpace(table))] = struct{}{}
	}
	for _, table := range tables {
		table = strings.TrimSpace(table)
		if table == "" {
			continue
		}
		key := strings.ToLower(table)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		next.rawTables = append(next.rawTables, table)
	}
	return &next
}

// NewDB wraps an existing sql.DB with a dialect into DB.
func NewDB(sqlDB *sql.DB, dialect driver.Dialect, opts ...Option) *DB {
	d := &driver.Driver{DB: sqlDB, Dialect: dialect}
	return newDB(d, sqlDB, opts...)
}

// NewDBWithExecutor wraps an existing sql.DB/sql.Tx-compatible executor with a
// dialect into DB. The returned DB does not own the executor; Close is a no-op
// unless the executor was created by Open/OpenWithDriver/NewDB with *sql.DB.
func NewDBWithExecutor(exec Executor, dialect driver.Dialect, opts ...Option) *DB {
	d := &driver.Driver{Dialect: dialect}
	return newDB(d, exec, opts...)
}

// NewTxDB wraps an existing sql.Tx with a dialect into DB.
func NewTxDB(tx *sql.Tx, dialect driver.Dialect, opts ...Option) *DB {
	return NewDBWithExecutor(tx, dialect, opts...)
}

// Open opens a MySQL database with default pooling. Deprecated: use
// OpenWithDriver to specify a driver explicitly.
func Open(dsn string) (*DB, error) {
	return OpenWithDriverOptions(MySQL, dsn)
}

// OpenWithDriver opens a database with default pooling for the given driver.
func OpenWithDriver(driverName, dsn string) (*DB, error) {
	return OpenWithDriverOptions(driverName, dsn)
}

// OpenWithDriverOptions opens a database with options for the given driver.
func OpenWithDriverOptions(driverName, dsn string, opts ...Option) (*DB, error) {
	if drv, ok := GetDriver(driverName); ok {
		dc, ok := drv.(sqldriver.DriverContext)
		if !ok {
			return nil, fmt.Errorf("driver %q does not implement DriverContext", driverName)
		}
		connector, err := dc.OpenConnector(dsn)
		if err != nil {
			return nil, fmt.Errorf("failed to create connector: %w", err)
		}
		sqlDB := sql.OpenDB(connector)
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Hour)
		if err = sqlDB.Ping(); err != nil {
			return nil, err
		}
		dialect, ok := getDialect(driverName)
		if !ok {
			dialect = defaultDialect(driverName)
		}
		d := &driver.Driver{DB: sqlDB, Dialect: dialect}
		return newDB(d, sqlDB, opts...), nil
	}

	drv, err := driver.Open(driverName, dsn, 10, 10, time.Hour)
	if err != nil {
		return nil, err
	}
	return newDB(drv, drv.DB, opts...), nil
}

// Close closes the owned underlying DB. DB values created around an external
// executor or transaction do not own a sql.DB, so Close is a no-op for them.
func (db *DB) Close() error {
	if db == nil || db.drv == nil || db.drv.DB == nil {
		return nil
	}
	return db.drv.Close()
}

// newTransactionDB wraps a sql.Tx in a DB instance bound to the same driver.
func (db *DB) newTransactionDB(tx *sql.Tx) *DB {
	return &DB{drv: db.drv, exec: tx, scanOpts: db.scanOpts, rawApproval: db.rawApproval, rawTables: append([]string(nil), db.rawTables...), rawErr: db.rawErr}
}

// WrapTx returns a DB copy that executes through tx while preserving this DB's
// dialect, scan options, and raw-SQL approval state.
func (db *DB) WrapTx(tx *sql.Tx, opts ...Option) *DB {
	next := db.newTransactionDB(tx)
	for _, o := range opts {
		o(next)
	}
	return next
}

// Tx represents a transaction-scoped DB wrapper.
type Tx struct {
	*DB
	driver.Tx
}

// Transaction executes fn in a transaction.
func (db *DB) Transaction(fn func(tx Tx) error) error {
	return db.drv.Transaction(func(t driver.Tx) error {
		txDB := db.newTransactionDB(t.Tx)
		return fn(Tx{DB: txDB, Tx: t})
	})
}

// TransactionContext executes fn in a transaction using ctx.
func (db *DB) TransactionContext(ctx context.Context, fn func(tx Tx) error) error {
	return db.drv.TransactionContext(ctx, func(t driver.Tx) error {
		txDB := db.newTransactionDB(t.Tx)
		return fn(Tx{DB: txDB, Tx: t})
	})
}

// Begin starts a transaction for manual control.
func (db *DB) Begin() (Tx, error) {
	t, err := db.drv.Begin()
	if err != nil {
		return Tx{}, err
	}
	txDB := db.newTransactionDB(t.Tx)
	return Tx{DB: txDB, Tx: t}, nil
}

// BeginTx starts a transaction using ctx and returns the Tx.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	t, err := db.drv.BeginTx(ctx, opts)
	if err != nil {
		return Tx{}, err
	}
	txDB := db.newTransactionDB(t.Tx)
	return Tx{DB: txDB, Tx: t}, nil
}

// Model creates a query for the struct table.
func (db *DB) Model(v any) *query.Query {
	return query.New(db.exec, model.TableName(v), db.drv.Dialect)
}

// Table creates a query for table name.
func (db *DB) Table(name string) *query.Query {
	return query.New(db.exec, name, db.drv.Dialect)
}

// TablePath creates a query for a schema-qualified or otherwise
// path-qualified table name.
func (db *DB) TablePath(parts ...string) *query.Query {
	return db.Table(strings.Join(parts, "."))
}

func (db *DB) rawPlan(ctx context.Context, q string, args ...any) (*query.QueryPlan, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if db.rawErr != nil {
		return nil, db.rawErr
	}
	plan := query.NewRawPlan(q, args...)
	if db.rawApproval != nil {
		copied := *db.rawApproval
		plan.Approval = &copied
	}
	for _, table := range db.rawTables {
		plan.Tables = append(plan.Tables, query.TableRef{Name: table})
	}
	return plan, nil
}

func (db *DB) ensureRawExecutable(ctx context.Context, q string, args ...any) (*query.QueryPlan, error) {
	plan, err := db.rawPlan(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if err := query.EnsurePlanExecutable(plan); err != nil {
		return plan, err
	}
	return plan, nil
}

func (db *DB) queryContextTrusted(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if ctx != nil {
		return db.exec.QueryContext(ctx, q, args...)
	}
	return db.exec.Query(q, args...)
}

func (db *DB) execContextTrusted(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if ctx != nil {
		return db.exec.ExecContext(ctx, q, args...)
	}
	return db.exec.Exec(q, args...)
}

const rawQueryRowRejectedSQL = "SELECT 1 WHERE 1 = 0"

func (db *DB) rejectedQueryRow(ctx context.Context) *sql.Row {
	if ctx == nil || ctx.Err() == nil {
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		ctx = canceled
	}
	return db.exec.QueryRowContext(ctx, rawQueryRowRejectedSQL)
}

// Query runs a raw SQL query returning multiple rows.
func (db *DB) Query(q string, args ...any) (*sql.Rows, error) {
	if _, err := db.ensureRawExecutable(nil, q, args...); err != nil {
		return nil, err
	}
	return db.queryContextTrusted(nil, q, args...)
}

// QueryContext runs Query with a context.
func (db *DB) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if _, err := db.ensureRawExecutable(ctx, q, args...); err != nil {
		return nil, err
	}
	return db.queryContextTrusted(ctx, q, args...)
}

// RawPlan creates a plan for caller-supplied SQL without executing it.
func (db *DB) RawPlan(ctx context.Context, q string, args ...any) (*QueryPlan, error) {
	return db.rawPlan(ctx, q, args...)
}

// Exec executes a raw SQL statement.
func (db *DB) Exec(q string, args ...any) (sql.Result, error) {
	if _, err := db.ensureRawExecutable(nil, q, args...); err != nil {
		return nil, err
	}
	return db.execContextTrusted(nil, q, args...)
}

// ExecContext executes a raw SQL statement with a context.
func (db *DB) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	if _, err := db.ensureRawExecutable(ctx, q, args...); err != nil {
		return nil, err
	}
	return db.execContextTrusted(ctx, q, args...)
}

// QueryRow executes a query that is expected to return at most one row.
//
// Deprecated: use QueryRowE so raw SQL safety errors can be returned before
// Scan. QueryRow cannot surface pre-execution approval errors because *sql.Row
// has no public error constructor. When raw SQL approval checks fail, QueryRow
// does not execute the caller-supplied SQL.
func (db *DB) QueryRow(q string, args ...any) *sql.Row {
	if _, err := db.ensureRawExecutable(nil, q, args...); err != nil {
		return db.rejectedQueryRow(nil)
	}
	return db.exec.QueryRow(q, args...)
}

// QueryRowContext executes a query with context returning at most one row.
//
// Deprecated: use QueryRowE so raw SQL safety errors can be returned before
// Scan. QueryRowContext cannot surface pre-execution approval errors because
// *sql.Row has no public error constructor. When raw SQL approval checks fail,
// QueryRowContext does not execute the caller-supplied SQL.
func (db *DB) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	if _, err := db.ensureRawExecutable(ctx, q, args...); err != nil {
		return db.rejectedQueryRow(ctx)
	}
	return db.exec.QueryRowContext(ctx, q, args...)
}

// QueryRowE validates raw SQL policy and executes a context-aware single-row query.
func (db *DB) QueryRowE(ctx context.Context, q string, args ...any) (*sql.Row, error) {
	if _, err := db.ensureRawExecutable(ctx, q, args...); err != nil {
		return nil, err
	}
	if ctx == nil {
		return db.exec.QueryRow(q, args...), nil
	}
	return db.exec.QueryRowContext(ctx, q, args...), nil
}
