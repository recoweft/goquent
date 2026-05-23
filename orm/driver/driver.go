package driver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	// TODO: evaluate using a lightweight pq or pgx driver
	_ "github.com/lib/pq"
)

// Dialect defines the SQL dialect abstraction.
type Dialect interface {
	Placeholder(n int) string
	QuoteIdent(ident string) string
}

// MySQLDialect implements Dialect for MySQL.
type MySQLDialect struct{}

// PostgresDialect implements Dialect for PostgreSQL.
type PostgresDialect struct{}

func (d PostgresDialect) Placeholder(n int) string { return fmt.Sprintf("$%d", n) }

func (d PostgresDialect) QuoteIdent(ident string) string {
	escaped := strings.ReplaceAll(ident, `"`, `""`)
	return `"` + escaped + `"`
}

func (d MySQLDialect) Placeholder(_ int) string { return "?" }

func (d MySQLDialect) QuoteIdent(ident string) string {
	escaped := strings.ReplaceAll(ident, "`", "``")
	return "`" + escaped + "`"
}

// Driver wraps sql.DB with a dialect.
type Driver struct {
	DB      *sql.DB
	Dialect Dialect
}

// Open initializes the DB connection with pooling configuration.
func Open(driverName, dsn string, maxOpen, maxIdle int, lifetime time.Duration) (*Driver, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(lifetime)
	if err = db.Ping(); err != nil {
		return nil, err
	}

	var dialect Dialect
	switch driverName {
	case "postgres":
		dialect = PostgresDialect{}
	case "mysql":
		dialect = MySQLDialect{}
	default:
		return nil, fmt.Errorf("unsupported driver %s", driverName)
	}

	return &Driver{DB: db, Dialect: dialect}, nil
}

// Close closes the underlying DB.
func (d *Driver) Close() error {
	if d == nil || d.DB == nil {
		return nil
	}
	return d.DB.Close()
}

// Tx wraps sql.Tx for transaction handling.
type Tx struct{ *sql.Tx }

// Transaction executes fn within a transaction.
func (d *Driver) Transaction(fn func(Tx) error) error {
	tx, err := d.DB.Begin()
	if err != nil {
		return err
	}
	if err = fn(Tx{tx}); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

// Begin starts a transaction and returns the Tx.
func (d *Driver) Begin() (Tx, error) {
	tx, err := d.DB.Begin()
	if err != nil {
		return Tx{}, err
	}
	return Tx{tx}, nil
}

// TransactionContext executes fn within a transaction using ctx.
func (d *Driver) TransactionContext(ctx context.Context, fn func(Tx) error) error {
	tx, err := d.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err = fn(Tx{tx}); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

// BeginTx starts a transaction with ctx and returns the Tx.
func (d *Driver) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := d.DB.BeginTx(ctx, opts)
	if err != nil {
		return Tx{}, err
	}
	return Tx{tx}, nil
}
