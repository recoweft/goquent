package orm

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// NestedWriteMode selects how a nested write step persists its rows.
type NestedWriteMode int

const (
	// NestedWriteDefault lets the helper choose the step default.
	NestedWriteDefault NestedWriteMode = iota
	// NestedWriteInsert inserts rows.
	NestedWriteInsert
	// NestedWriteUpsert upserts rows using the provided WriteOpt conflict target.
	NestedWriteUpsert
)

// NestedDelete describes one child-table cleanup step.
type NestedDelete struct {
	Table  string
	Scopes []Scope
}

// NestedCollectionReplace describes a parent + ordered child + grandchild write.
//
// ParentMode defaults to NestedWriteUpsert. ChildMode and GrandchildMode default
// to NestedWriteInsert. Use DeleteBefore to delete grandchildren before children
// when replacing a collection.
type NestedCollectionReplace[P any, C any, G any] struct {
	SkipParent bool
	Parent     P
	ParentMode NestedWriteMode
	ParentOpts []WriteOpt

	DeleteBefore []NestedDelete

	Children      []C
	ChildMode     NestedWriteMode
	ChildOpts     []WriteOpt
	ChildIDColumn string
	AssignChildID func(index int, id int64)

	Grandchildren  func(childIndex int, child C, childID int64) ([]G, error)
	GrandchildMode NestedWriteMode
	GrandchildOpts []WriteOpt
}

// NestedCollectionWriteResult reports generated IDs and row counts from a nested write.
type NestedCollectionWriteResult struct {
	ChildIDs        []int64
	GrandchildCount int
}

// ReplaceNestedCollection executes a parent + child collection replacement on db.
//
// The caller controls transaction boundaries. Use ReplaceNestedCollectionTx when
// the whole sequence should run in a new transaction. Conditional strict settings
// reject Grandchildren, AssignChildID, and every nonnil DeleteBefore Scope before
// execution. Compatibility callbacks can have effects before a later refusal.
func ReplaceNestedCollection[P any, C any, G any](ctx context.Context, db *DB, spec NestedCollectionReplace[P, C, G]) (NestedCollectionWriteResult, error) {
	p, err := prepareNested(ctx, db, spec)
	if err != nil {
		return NestedCollectionWriteResult{}, err
	}
	return executeNested(ctx, db, spec, p)
}

// ReplaceNestedCollectionTx validates known statements before starting a transaction.
func ReplaceNestedCollectionTx[P any, C any, G any](ctx context.Context, db *DB, spec NestedCollectionReplace[P, C, G]) (NestedCollectionWriteResult, error) {
	var result NestedCollectionWriteResult
	p, err := prepareNested(ctx, db, spec)
	if err != nil {
		return result, err
	}
	err = db.TransactionContext(ctx, func(tx Tx) error {
		if err := p.plan.rebind(ctx, tx.DB); err != nil {
			return err
		}
		var e error
		result, e = executeNested(ctx, tx.DB, spec, p)
		return e
	})
	return result, err
}

func validateNestedDB(db *DB) error {
	if db == nil {
		return fmt.Errorf("db is nil")
	}
	if db.drv == nil || db.exec == nil {
		return fmt.Errorf("goquent: db is not initialized")
	}
	return nil
}

func normalizeNestedWriteMode(mode NestedWriteMode, fallback NestedWriteMode) (NestedWriteMode, error) {
	if mode == NestedWriteDefault {
		mode = fallback
	}
	switch mode {
	case NestedWriteInsert, NestedWriteUpsert:
		return mode, nil
	default:
		return NestedWriteDefault, fmt.Errorf("unsupported nested write mode %d", mode)
	}
}

func nestedIDsFromReturningRows(rows []map[string]any, idColumn string, expected int) ([]int64, error) {
	if len(rows) != expected {
		return nil, fmt.Errorf("goquent: nested child insert returned %d ids for %d rows", len(rows), expected)
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		value, ok := nestedReturningValue(row, idColumn)
		if !ok {
			return nil, fmt.Errorf("goquent: nested child insert did not return column %s", idColumn)
		}
		id, err := nestedInt64(value)
		if err != nil {
			return nil, fmt.Errorf("goquent: nested child id %s row %d: %w", idColumn, i, err)
		}
		ids[i] = id
	}
	return ids, nil
}

func nestedReturningValue(row map[string]any, idColumn string) (any, bool) {
	if value, ok := row[idColumn]; ok {
		return value, true
	}
	if parts := strings.Split(idColumn, "."); len(parts) > 1 {
		value, ok := row[parts[len(parts)-1]]
		return value, ok
	}
	return nil, false
}

func nestedInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		if uint64(v) > math.MaxInt64 {
			return 0, fmt.Errorf("value overflows int64")
		}
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("value overflows int64")
		}
		return int64(v), nil
	case []byte:
		return strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64)
	case string:
		return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	default:
		return 0, fmt.Errorf("unsupported id type %T", value)
	}
}
