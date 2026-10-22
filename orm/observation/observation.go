// Package observation provides opt-in context query budgets and bounded, public-safe
// executor-boundary observations. It does not certify authorization or native SQL.
package observation

import (
	"context"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/observe"
)

type Options = observe.Options
type Event = observe.Event
type Summary = observe.Summary
type Metric = observe.Metric
type Rows = observe.Rows

var ErrQueryBudgetExceeded = observe.ErrBudget
var ErrOptions = observe.ErrOptions
var ErrNestedScope = observe.ErrNested
var ErrDrainBusy = observe.ErrBusy

// Scope owns an observation interval. Finish freezes SQL statistics, not its budget.
type Scope struct{ state *observe.State }
type Hook func(Event)

func Start(ctx context.Context, options Options) (context.Context, *Scope, error) {
	c, s, e := observe.Start(ctx, options)
	if e != nil {
		return nil, nil, e
	}
	return c, &Scope{state: s}, nil
}
func (s *Scope) Snapshot() Summary {
	if s == nil {
		return Summary{}.Safe()
	}
	return s.state.Snapshot()
}
func (s *Scope) Finish() Summary {
	if s == nil {
		return Summary{}.Safe()
	}
	return s.state.Finish()
}

// Drain attempts delivery once per retained event, outside SQL execution and locks.
// Its per-call counters remain separate from the frozen SQL observation interval.
func (s *Scope) Drain(h Hook) (attempts, panics int64, err error) {
	if s == nil {
		return 0, 0, ErrOptions
	}
	return s.state.Drain(h)
}
func (s *Scope) String() string               { return s.Snapshot().String() }
func (s *Scope) Format(f fmt.State, _ rune)   { _, _ = f.Write([]byte(s.String())) }
func (s *Scope) MarshalJSON() ([]byte, error) { return s.Snapshot().MarshalJSON() }
