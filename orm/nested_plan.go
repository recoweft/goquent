package orm

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"strings"
)

type nestedPlan struct {
	plan                          compoundPlan
	parent, child, grandchild     NestedWriteMode
	childOptions                  *writeOptions
	ids, postgres, dynamicDeletes bool
	idColumn                      string
}

func prepareNested[P any, C any, G any](ctx context.Context, db *DB, spec NestedCollectionReplace[P, C, G]) (*nestedPlan, error) {
	if err := validateNestedDB(db); err != nil {
		return nil, err
	}
	opaque := spec.Grandchildren != nil || spec.AssignChildID != nil
	dynamicDeletes := false
	for _, d := range spec.DeleteBefore {
		for _, scope := range d.Scopes {
			if scope != nil {
				opaque = true
				dynamicDeletes = true
			}
		}
	}
	if opaque {
		if err := rejectOpaqueCompound(db); err != nil {
			return nil, err
		}
	}
	p := &nestedPlan{ids: spec.Grandchildren != nil || spec.AssignChildID != nil, dynamicDeletes: dynamicDeletes}
	var err error
	if p.parent, err = normalizeNestedWriteMode(spec.ParentMode, NestedWriteUpsert); err != nil {
		return nil, err
	}
	if p.child, err = normalizeNestedWriteMode(spec.ChildMode, NestedWriteInsert); err != nil {
		return nil, err
	}
	if p.grandchild, err = normalizeNestedWriteMode(spec.GrandchildMode, NestedWriteInsert); err != nil {
		return nil, err
	}
	_, p.postgres = db.drv.Dialect.(driver.PostgresDialect)
	p.idColumn = strings.TrimSpace(spec.ChildIDColumn)
	if p.idColumn == "" {
		p.idColumn = "id"
	}
	if len(spec.Children) > 0 && p.ids && p.child != NestedWriteInsert {
		return nil, fmt.Errorf("goquent: nested child IDs can only be collected for insert child writes")
	}
	if !spec.SkipParent {
		o := applyWriteOpts(spec.ParentOpts)
		var r querybridge.Request
		if p.parent == NestedWriteInsert {
			r, err = buildInsertInput(db, spec.Parent, o)
		} else {
			r, err = buildUpsertInput(db, spec.Parent, o)
		}
		if err != nil {
			return nil, err
		}
		if err = p.plan.add(ctx, db, "parent", 0, 1, r, o); err != nil {
			return nil, err
		}
	}
	for i, d := range spec.DeleteBefore {
		table := strings.TrimSpace(d.Table)
		if table == "" {
			return nil, fmt.Errorf("goquent: nested delete table is required")
		}
		dynamic := false
		for _, scope := range d.Scopes {
			dynamic = dynamic || scope != nil
		}
		if dynamic {
			p.plan.steps = append(p.plan.steps, compoundStep{phase: "delete", start: i, end: i + 1})
			continue
		}
		if err = p.plan.add(ctx, db, "delete", i, i+1, querybridge.Request{Base: db.Table(table), Operation: "delete"}, nil); err != nil {
			return nil, err
		}
	}
	if len(spec.Children) > 0 {
		p.childOptions = applyWriteOpts(spec.ChildOpts)
		if err = prepareNestedChildren(p, ctx, db, spec.Children); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (p *nestedPlan) addChildren(ctx context.Context, db *DB, r querybridge.Request) error {
	if p.ids {
		if p.postgres {
			r.Returning = []string{p.idColumn}
		}
		// Each returned ID now belongs to exactly one input row on both dialects.
		// No assumptions about auto-increment spacing or multi-row RETURNING order.
		for i, row := range r.Rows {
			one := r
			one.Rows = []map[string]any{row}
			one.Batch = false
			if err := p.plan.add(ctx, db, "child", i, i+1, one, p.childOptions); err != nil {
				return err
			}
		}
		return nil
	}
	return p.plan.add(ctx, db, "child", 0, len(r.Rows), r, p.childOptions)
}

func (p *nestedPlan) childSteps() []compoundStep {
	var out []compoundStep
	for _, s := range p.plan.steps {
		if s.phase == "child" {
			out = append(out, s)
		}
	}
	return out
}

func prepareNestedChildren[C any](p *nestedPlan, ctx context.Context, db *DB, values []C) error {
	var r querybridge.Request
	var err error
	if p.child == NestedWriteInsert {
		r, err = buildInsertManyInput(db, values, p.childOptions)
	} else {
		r, err = buildUpsertManyInput(db, values, p.childOptions)
	}
	if err != nil {
		return err
	}
	return p.addChildren(ctx, db, r)
}

func executeNested[P any, C any, G any](ctx context.Context, db *DB, spec NestedCollectionReplace[P, C, G], p *nestedPlan) (NestedCollectionWriteResult, error) {
	var result NestedCollectionWriteResult
	for i := range p.plan.steps {
		s := &p.plan.steps[i]
		if s.phase == "parent" {
			if err := s.exec(); err != nil {
				return result, err
			}
		}
	}
	for i, d := range spec.DeleteBefore {
		for j := range p.plan.steps {
			s := &p.plan.steps[j]
			if s.phase != "delete" || s.start != i {
				continue
			}
			if s.plan.Check == nil {
				// An unresolved compatibility slot becomes a fresh destination-bound plan
				// only after its scope runs at the original position, exactly once.
				q := ApplyScopes(db.Table(strings.TrimSpace(d.Table)), d.Scopes...)
				var resolved compoundPlan
				if err := resolved.add(ctx, db, "delete", i, i+1, querybridge.Request{Base: q, Operation: "delete"}, nil); err != nil {
					return result, err
				}
				*s = resolved.steps[0]
			}
			if err := s.exec(); err != nil {
				return result, err
			}
		}
	}

	if len(spec.Children) == 0 {
		return result, nil
	}
	if p.dynamicDeletes {
		// Scopes may have changed captured child inputs. Such changes require new plans.
		fresh := *p
		fresh.plan = compoundPlan{}
		if err := prepareNestedChildren(&fresh, ctx, db, spec.Children); err != nil {
			return result, err
		}
		kept := p.plan.steps[:0]
		for _, step := range p.plan.steps {
			if step.phase != "child" {
				kept = append(kept, step)
			}
		}
		p.plan.steps = append(kept, fresh.plan.steps...)
	}
	var zeroID bool
	var idsCollected []int64
	var affected int64
	for _, s := range p.childSteps() {
		if !p.ids {
			if err := s.exec(); err != nil {
				return result, err
			}
			continue
		}
		var id int64
		if p.postgres {
			var rows []map[string]any
			err := s.plan.Scan(func(r *sql.Rows) error { var e error; rows, e = scanRowsAll[map[string]any](db, r); return e })
			if err != nil {
				return result, err
			}
			ids, err := nestedIDsFromReturningRows(rows, p.idColumn, 1)
			if err != nil {
				return result, err
			}
			id = ids[0]
		} else {
			res, err := s.plan.Exec()
			if err != nil {
				return result, err
			}
			id, err = res.LastInsertId()
			if err != nil {
				return result, err
			}
			zeroID = zeroID || id == 0
			if p.childOptions.expectAffected != nil || p.childOptions.zeroRowsErr != nil {
				n, err := res.RowsAffected()
				if err != nil {
					return result, err
				}
				affected += n
			}
		}
		idsCollected = append(idsCollected, id)
	}
	if p.ids && !p.postgres {
		if err := checkRowsAffected(returningResult{rowsAffected: affected}, p.childOptions); err != nil {
			return result, err
		}
	}
	if zeroID {
		return result, fmt.Errorf("goquent: LastInsertId returned 0 for nested child insert")
	}
	result.ChildIDs = idsCollected
	for i, id := range result.ChildIDs {
		if spec.AssignChildID != nil {
			spec.AssignChildID(i, id)
		}
	}
	if spec.Grandchildren == nil {
		return result, nil
	}
	var grandchildren []G
	for i, child := range spec.Children {
		rows, err := spec.Grandchildren(i, child, result.ChildIDs[i])
		if err != nil {
			return result, err
		}
		grandchildren = append(grandchildren, rows...)
	}
	result.GrandchildCount = len(grandchildren)
	if len(grandchildren) > 0 {
		o := applyWriteOpts(spec.GrandchildOpts)
		var r querybridge.Request
		var err error
		if p.grandchild == NestedWriteInsert {
			r, err = buildInsertManyInput(db, grandchildren, o)
		} else {
			r, err = buildUpsertManyInput(db, grandchildren, o)
		}
		if err != nil {
			return result, err
		}
		if err = p.plan.add(ctx, db, "grandchild", 0, len(grandchildren), r, o); err != nil {
			return result, err
		}
		if err = p.plan.steps[len(p.plan.steps)-1].exec(); err != nil {
			return result, err
		}
	}
	return result, nil
}
