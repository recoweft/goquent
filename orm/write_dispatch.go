package orm

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/observe"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"reflect"
)

type returningResult struct {
	rowsAffected int64
}

func (r returningResult) LastInsertId() (int64, error) {
	return 0, fmt.Errorf("LastInsertId is not supported for RETURNING statements")
}

func (r returningResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func ensureReturningColumns[T any](o *writeOptions) error {
	returning := o != nil && len(o.returning) > 0
	if returning {
		return nil
	}
	cols, err := returningColumnsForQuery[T]()
	if err != nil {
		return err
	}
	o.returning = cols
	return nil
}

func returningColumnsForQuery[T any]() ([]string, error) {
	var zero T
	typ := reflect.TypeOf(zero)
	if typ == nil {
		return nil, fmt.Errorf("Returning columns are required for untyped return values")
	}
	if isMapStringInterface(typ) {
		return nil, fmt.Errorf("Returning columns are required for map return values")
	}
	cols, err := structColumnNames(typ)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns to return")
	}
	return cols, nil
}

func checkRowsAffected(res sql.Result, o *writeOptions) error {
	if o == nil {
		return nil
	}
	return o.results.Result(res)
}

func execWriteInput(ctx context.Context, db *DB, r querybridge.Request, o *writeOptions) (sql.Result, error) {
	r.Results = o.results.Clone()
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return nil, err
	}
	return execPreparedWrite(p, len(r.Returning) > 0, o)
}

func execPreparedWrite(p querybridge.Planned, returning bool, o *writeOptions) (sql.Result, error) {
	var err error
	var res sql.Result
	if !returning {
		res, err = p.Exec()
	} else {
		var count int64
		err = p.ScanCount(func(rows observe.Reader) (int64, error) {
			cols, err := rows.Columns()
			if err != nil {
				return count, err
			}
			dest := make([]any, len(cols))
			values := make([]any, len(cols))
			for i := range dest {
				dest[i] = &values[i]
			}
			for rows.Next() {
				if len(dest) > 0 {
					if err := rows.Scan(dest...); err != nil {
						return count, err
					}
				}
				count++
			}
			return count, rows.Err()
		})
		res = returningResult{rowsAffected: count}
	}
	if err != nil {
		return nil, err
	}

	return res, nil
}

func queryWriteOne[T any](ctx context.Context, db *DB, r querybridge.Request) (T, error) {
	return queryWriteOneWithOptions[T](ctx, db, r, &writeOptions{})
}

func queryWriteAll[T any](ctx context.Context, db *DB, r querybridge.Request, options ...*writeOptions) (out []T, err error) {
	if len(options) > 0 {
		r.Results = options[0].results.Clone()
	}
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return nil, err
	}
	err = p.ScanCount(func(rows observe.Reader) (int64, error) {
		out, err = scanRowsAll[T](db, rows)
		return int64(len(out)), err
	})
	return out, err
}

func queryWriteOneWithOptions[T any](ctx context.Context, db *DB, r querybridge.Request, o *writeOptions) (out T, err error) {
	r.Results = o.results.Clone()
	p, err := prepareWrite(ctx, db, r)
	if err != nil {
		return out, err
	}
	var missing bool
	err = p.ScanCount(func(rows observe.Reader) (int64, error) {
		var count int64
		for {
			row, e := scanRowsOne[T](db, rows)
			if e == sql.ErrNoRows {
				break
			}
			if e != nil {
				return count, e
			}
			if count == 0 {
				out = row
			}
			count++
		}
		missing = count == 0
		if missing {
			return count, sql.ErrNoRows
		}
		return count, nil
	})
	if err == nil && missing {
		err = sql.ErrNoRows
	}
	return out, err
}
