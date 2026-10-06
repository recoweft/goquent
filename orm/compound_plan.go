package orm

import (
	"context"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

// ErrUnsupportedCompound identifies opaque recipes refused by conditional strict inspection.
var ErrUnsupportedCompound = errors.New("goquent: unsupported compound operation")

func rejectOpaqueCompound(db *DB) error {
	if querybridge.Strict(db.settings) {
		return fmt.Errorf("%w: %w: tenant_policy/opaque_compound", ErrBlockedOperation, ErrUnsupportedCompound)
	}
	return nil
}

// compoundStep owns the input range and operation represented by one statement.
// Diagnostic fields are never read back for inspection or execution.
type compoundStep struct {
	phase      string
	start, end int
	input      querybridge.Request
	plan       querybridge.Planned
	options    *writeOptions
}

type compoundPlan struct{ steps []compoundStep }

func (p *compoundPlan) add(ctx context.Context, db *DB, phase string, start, end int, r querybridge.Request, o *writeOptions) error {
	r.Options = r.Options.Clone()
	r.Returning = append([]string(nil), r.Returning...)
	r.WhereColumns = append([]string(nil), r.WhereColumns...)
	r.WhereValues = valuecopy.Slice(r.WhereValues)
	rows := make([]map[string]any, len(r.Rows))
	for i, row := range r.Rows {
		rows[i] = make(map[string]any, len(row))
		for k, v := range row {
			rows[i][k], _ = valuecopy.Copy(v)
		}
	}
	r.Rows = rows
	plan, err := prepareWrite(ctx, db, r)
	if err != nil {
		return err
	}
	if err = plan.Check(); err != nil {
		return err
	}
	p.steps = append(p.steps, compoundStep{phase: phase, start: start, end: end, input: r, plan: plan, options: o})
	return nil
}

// rebind re-inspects every known child on the actual transaction destination.
// It does not evaluate application input/options/scopes a second time.
func (p *compoundPlan) rebind(ctx context.Context, db *DB) error {
	next := make([]querybridge.Planned, len(p.steps))
	for i, s := range p.steps {
		if s.plan.Check == nil {
			continue
		} // Explicit unresolved compatibility scope slot.
		plan, err := prepareWrite(ctx, db, s.input)
		if err != nil {
			return err
		}
		if err = plan.Check(); err != nil {
			return err
		}
		next[i] = plan
	}
	for i := range next {
		p.steps[i].plan = next[i]
	}
	return nil
}

func (s *compoundStep) exec() error {
	_, err := execPreparedWrite(s.plan, len(s.input.Returning) > 0, s.options)
	return err
}
