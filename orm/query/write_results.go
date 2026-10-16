package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"strings"

	"github.com/recoweft/goquent/orm/internal/writecheck"
)

// AtomicWrite opts into an owned transaction through complete result inspection.
// Queries made from external executors or transactions reject before execution.
func (q *Query) AtomicWrite() *Query { q.writeResult.Atomic = true; return q }

// ExpectAffected checks the driver count (or complete RETURNING row count).
// Without AtomicWrite, an autocommitted write cannot be rolled back by this check.
func (q *Query) ExpectAffected(n int64) *Query {
	if q.writeResult.Expected != nil {
		q.writeResult.Invalid = true
	}
	q.writeResult.Expected = &n
	return q
}

// NoRowsAs maps zero affected rows to a source error, unless ExpectAffected(0) matches.
func (q *Query) NoRowsAs(err error) *Query {
	if q.writeResult.Zero != nil {
		q.writeResult.Invalid = true
	}
	q.writeResult.Zero = err
	return q
}

// OptimisticVersion compares and advances a signed, nonnegative version atomically.
func (q *Query) OptimisticVersion(column string, expected int64) *Query {
	if q.writeResult.Version != nil {
		q.writeResult.Invalid = true
	}
	q.writeResult.Version = &writecheck.Version{Column: column, Expected: expected}
	return q
}

func (q *Query) runWrite(p *QueryPlan, run func(*plannedExecution) (sql.Result, error)) (sql.Result, error) {
	if err := q.checkExecution(p); err != nil {
		return nil, err
	}
	e := p.execution
	o := e.results
	if err := o.Validate(); err != nil {
		return nil, err
	}
	if o.Active() && e.inspection.Operation != OperationInsert && e.inspection.Operation != OperationUpdate && e.inspection.Operation != OperationDelete {
		return nil, writecheck.ErrOptions
	}
	if o.Version != nil && e.inspection.Operation != OperationUpdate {
		return nil, writecheck.ErrOptions
	}
	atomic := o.Atomic || o.Version != nil
	if atomic {
		copier := valuecopy.New()
		for _, arg := range e.args {
			if _, ok := arg.(json.Number); ok {
				continue
			}
			if _, ok := copier.Copy(arg); !ok {
				return nil, writecheck.ErrOptions
			}
		}
	}

	if atomic && !e.managedTx && e.writeOwner == nil {
		return nil, writecheck.ErrOwnership
	}
	if !atomic || e.managedTx {
		e, err := q.bindExecution(p)
		if err != nil {
			return nil, err
		}
		return run(e)
	}
	// Consume the original seal before opening a transaction. Re-inspection uses
	// the retained private statement/typed values, never mutable public diagnostics.
	e, err := q.bindExecution(p)
	if err != nil {
		return nil, err
	}
	ctx := e.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := e.writeOwner.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rollback := func(cause error) (sql.Result, error) {
		if re := tx.Rollback(); re != nil {
			return nil, errors.Join(writecheck.ErrOutcomeUnknown, cause, re)
		}
		return nil, cause
	}
	if err = q.checkAtomicStorage(ctx, tx, e.inspection); err != nil {
		return rollback(err)
	}
	copy := *q
	copy.exec = &ownedExecutor{tx}
	copy.writeResult = o.Clone()
	copy.managedWriteTx = true
	private := *e.inspection
	private.execution = nil
	copy.finalizePlan(&private)
	private.execution.bindingExpires = e.bindingExpires
	if err = copy.checkExecution(&private); err != nil {
		return rollback(err)
	}
	bound, err := copy.bindExecution(&private)
	if err != nil {
		return rollback(err)
	}
	res, err := run(bound)
	if err != nil {
		return rollback(err)
	}
	if err = tx.Commit(); err != nil {
		return nil, errors.Join(writecheck.ErrOutcomeUnknown, err)
	}
	return res, nil
}

func (q *Query) executeCountedRows(p *QueryPlan, scan func(*sql.Rows) (int64, error)) error {
	_, err := q.runWrite(p, func(e *plannedExecution) (sql.Result, error) {
		var rows *sql.Rows
		var err error
		if e.ctx != nil {
			rows, err = e.executor.QueryContext(e.ctx, e.sql, e.args...)
		} else {
			rows, err = e.executor.Query(e.sql, e.args...)
		}
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		count, err := scan(rows)
		if err == sql.ErrNoRows {
			if checkErr := e.results.Check(count); checkErr != nil {
				return nil, checkErr
			}
		}
		if err != nil {
			return nil, err
		}
		if err = rows.Err(); err != nil {
			return nil, err
		}
		if err = rows.Close(); err != nil {
			return nil, err
		}
		return nil, e.results.Check(count)
	})
	return err
}

// MySQL transactions cannot roll back nontransactional table engines. Check the
// final private targets before business SQL; concurrent administrative DDL is
// outside the stable-schema trust assumption, as for CommandStore.
func (q *Query) checkAtomicStorage(ctx context.Context, tx *sql.Tx, p *QueryPlan) error {
	switch q.dialect.(type) {
	case driver.PostgresDialect:
		return nil
	case driver.MySQLDialect:
		if len(p.Tables) == 0 {
			return writecheck.ErrOptions
		}
		for _, table := range p.Tables {
			if table.Alias != "" || !scopePath(table.Name) {
				return writecheck.ErrOptions
			}
			parts := strings.Split(table.Name, ".")
			if len(parts) > 2 {
				return writecheck.ErrOptions
			}
			schema, name := "", parts[len(parts)-1]
			if len(parts) == 2 {
				schema = parts[0]
			}
			var engine string
			err := tx.QueryRowContext(ctx, "SELECT ENGINE FROM information_schema.tables WHERE table_schema = COALESCE(NULLIF(?, ''), DATABASE()) AND table_name = ? AND table_type = 'BASE TABLE'", schema, name).Scan(&engine)
			if err != nil {
				return err
			}
			if engine != "InnoDB" {
				return writecheck.ErrOptions
			}
		}
		return nil
	default:
		return writecheck.ErrOptions
	}
}
