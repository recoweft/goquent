// Package querybridge connects the ORM facade to private Query operations without
// exposing builder types, accepting public executable artifacts or importing orm.
package querybridge

import (
	"context"
	"database/sql"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

type Executor interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	QueryRowContext(context.Context, string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Request struct {
	Base, Settings   any // Only the Query package can resolve these concrete identities.
	Executor         Executor
	Dialect          driver.Dialect
	Context          context.Context
	Operation, Table string
	Batch            bool
	Rows             []map[string]any
	WhereColumns     []string
	WhereValues      []any
	Options          writeinput.Options
	Returning        []string
}

type Planned struct {
	Diagnostic any
	InsertRows []map[string]any // Detached final INSERT candidates; never public verdicts.
	Check      func() error
	Rows       func() (*sql.Rows, error)
	Row        func() (*sql.Row, error)
	Exec       func() (sql.Result, error)
	Scan       func(func(*sql.Rows) error) error
}

// Prepare is installed once by query initialization. The facade imports query,
// so initialization finishes before it can call this internal-only connection.
// It accepts structural input, never SQL, public diagnostics or authorization.
var Prepare func(Request) (Planned, error)

// RawRequest carries raw input, not a public diagnostic or a semantic assertion.
type RawRequest struct {
	Request
	SQL      string
	Args     []any
	Approval any
	Tables   []string
}

var PrepareRaw func(RawRequest) (Planned, error)
var Strict func(any) bool
