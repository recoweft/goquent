package orm

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/recoweft/goquent/orm/driver"
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
// the whole sequence should run in a new transaction.
func ReplaceNestedCollection[P any, C any, G any](ctx context.Context, db *DB, spec NestedCollectionReplace[P, C, G]) (NestedCollectionWriteResult, error) {
	var result NestedCollectionWriteResult
	if err := validateNestedDB(db); err != nil {
		return result, err
	}

	parentMode, err := normalizeNestedWriteMode(spec.ParentMode, NestedWriteUpsert)
	if err != nil {
		return result, fmt.Errorf("parent mode: %w", err)
	}
	childMode, err := normalizeNestedWriteMode(spec.ChildMode, NestedWriteInsert)
	if err != nil {
		return result, fmt.Errorf("child mode: %w", err)
	}
	grandchildMode, err := normalizeNestedWriteMode(spec.GrandchildMode, NestedWriteInsert)
	if err != nil {
		return result, fmt.Errorf("grandchild mode: %w", err)
	}

	if !spec.SkipParent {
		if err := runNestedOneWrite(ctx, db, spec.Parent, parentMode, spec.ParentOpts); err != nil {
			return result, err
		}
	}

	for _, del := range spec.DeleteBefore {
		if err := runNestedDelete(ctx, db, del); err != nil {
			return result, err
		}
	}

	if len(spec.Children) == 0 {
		return result, nil
	}

	needsChildIDs := spec.AssignChildID != nil || spec.Grandchildren != nil
	if needsChildIDs {
		if childMode != NestedWriteInsert {
			return result, fmt.Errorf("goquent: nested child IDs can only be collected for insert child writes")
		}
		childIDColumn := strings.TrimSpace(spec.ChildIDColumn)
		if childIDColumn == "" {
			childIDColumn = "id"
		}
		childIDs, err := insertManyWithNestedIDs(ctx, db, spec.Children, childIDColumn, spec.ChildOpts)
		if err != nil {
			return result, err
		}
		result.ChildIDs = childIDs
		for i, id := range childIDs {
			if spec.AssignChildID != nil {
				spec.AssignChildID(i, id)
			}
		}
	} else if err := runNestedManyWrite(ctx, db, spec.Children, childMode, spec.ChildOpts); err != nil {
		return result, err
	}

	if spec.Grandchildren == nil {
		return result, nil
	}
	grandchildren := make([]G, 0)
	for i, child := range spec.Children {
		var childID int64
		if i < len(result.ChildIDs) {
			childID = result.ChildIDs[i]
		}
		rows, err := spec.Grandchildren(i, child, childID)
		if err != nil {
			return result, err
		}
		grandchildren = append(grandchildren, rows...)
	}
	result.GrandchildCount = len(grandchildren)
	if len(grandchildren) == 0 {
		return result, nil
	}
	if err := runNestedManyWrite(ctx, db, grandchildren, grandchildMode, spec.GrandchildOpts); err != nil {
		return result, err
	}
	return result, nil
}

// ReplaceNestedCollectionTx runs ReplaceNestedCollection inside a transaction.
func ReplaceNestedCollectionTx[P any, C any, G any](ctx context.Context, db *DB, spec NestedCollectionReplace[P, C, G]) (NestedCollectionWriteResult, error) {
	var result NestedCollectionWriteResult
	if err := validateNestedDB(db); err != nil {
		return result, err
	}
	err := db.TransactionContext(ctx, func(tx Tx) error {
		var err error
		result, err = ReplaceNestedCollection(ctx, tx.DB, spec)
		return err
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

func runNestedOneWrite[T any](ctx context.Context, db *DB, value T, mode NestedWriteMode, opts []WriteOpt) error {
	switch mode {
	case NestedWriteInsert:
		_, err := Insert(ctx, db, value, opts...)
		return err
	case NestedWriteUpsert:
		_, err := Upsert(ctx, db, value, opts...)
		return err
	default:
		return fmt.Errorf("unsupported nested write mode %d", mode)
	}
}

func runNestedManyWrite[T any](ctx context.Context, db *DB, values []T, mode NestedWriteMode, opts []WriteOpt) error {
	if len(values) == 0 {
		return nil
	}
	switch mode {
	case NestedWriteInsert:
		_, err := InsertMany(ctx, db, values, opts...)
		return err
	case NestedWriteUpsert:
		_, err := UpsertMany(ctx, db, values, opts...)
		return err
	default:
		return fmt.Errorf("unsupported nested write mode %d", mode)
	}
}

func runNestedDelete(ctx context.Context, db *DB, del NestedDelete) error {
	table := strings.TrimSpace(del.Table)
	if table == "" {
		return fmt.Errorf("goquent: nested delete table is required")
	}
	_, err := DeleteBy(ctx, db.Table(table), del.Scopes...)
	return err
}

func insertManyWithNestedIDs[T any](ctx context.Context, db *DB, values []T, idColumn string, opts []WriteOpt) ([]int64, error) {
	if len(values) == 0 {
		return nil, nil
	}
	idColumn = strings.TrimSpace(idColumn)
	if idColumn == "" {
		return nil, fmt.Errorf("goquent: child id column is required")
	}
	if _, ok := db.drv.Dialect.(driver.PostgresDialect); ok {
		rows, err := InsertManyReturning[map[string]any](ctx, db, values, appendNestedWriteOpts(opts, Returning(idColumn))...)
		if err != nil {
			return nil, err
		}
		return nestedIDsFromReturningRows(rows, idColumn, len(values))
	}

	res, err := InsertMany(ctx, db, values, opts...)
	if err != nil {
		return nil, err
	}
	firstID, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	if firstID == 0 {
		return nil, fmt.Errorf("goquent: LastInsertId returned 0 for nested child insert")
	}
	ids := make([]int64, len(values))
	for i := range values {
		ids[i] = firstID + int64(i)
	}
	return ids, nil
}

func appendNestedWriteOpts(opts []WriteOpt, extra ...WriteOpt) []WriteOpt {
	out := make([]WriteOpt, 0, len(opts)+len(extra))
	out = append(out, opts...)
	out = append(out, extra...)
	return out
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
