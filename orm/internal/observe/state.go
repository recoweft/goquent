package observe

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const MaxEvents = 256
const MaxTracking = 1024
const MaxShapes = 128

// Options are copied at Start; a nil budget is unlimited, a pointer to zero rejects all dispatches.
type Options struct {
	QueryLimit     *int64
	DetectRepeated bool
}
type key struct{}
type State struct {
	mu          sync.Mutex
	start       time.Time
	finished    bool
	budget      bool
	limit, used int64
	repeat      bool
	sum         Summary
	pending     map[*Ticket]struct{}
	shapes      map[shape]*group
	events      []Event
	draining    atomic.Bool
}
type group struct{ n int64 }
type shape struct {
	Owner any
	Key   string
}
type Ticket struct {
	s        *State
	start    time.Time
	event    Event
	group    *group
	eligible bool
}

func Start(ctx context.Context, o Options) (context.Context, *State, error) {
	if ctx == nil {
		return nil, nil, ErrOptions
	}
	if From(ctx) != nil {
		return nil, nil, ErrNested
	}
	s := &State{start: time.Now(), repeat: o.DetectRepeated, pending: make(map[*Ticket]struct{})}
	if o.QueryLimit != nil {
		if *o.QueryLimit < 0 || *o.QueryLimit > MaxNumber {
			return nil, nil, ErrOptions
		}
		s.budget = true
		s.limit = *o.QueryLimit
	}
	return context.WithValue(ctx, key{}, s), s, nil
}
func From(ctx context.Context) *State {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(key{}).(*State)
	return s
}
func Enabled(ctx context.Context) bool  { return From(ctx) != nil }
func Repeated(ctx context.Context) bool { s := From(ctx); return s != nil && s.repeat }
func (s *State) add(p *int64, n int64) {
	if n < 0 || n > MaxNumber-*p {
		*p = MaxNumber
		s.sum.Saturated = true
	} else {
		*p += n
	}
}
func (s *State) enqueue(e Event) {
	if len(s.events) == MaxEvents {
		s.add(&s.sum.Dropped, 1)
		s.sum.Truncated = true
		return
	}
	s.events = append(s.events, e.Safe())
}
func Begin(ctx context.Context, class, method string, owner any, k string, eligible bool) (*Ticket, error) {
	s := From(ctx)
	if s == nil {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.finished {
		s.add(&s.sum.Attempted, 1)
	}
	if s.budget && s.used >= s.limit {
		if !s.finished {
			s.add(&s.sum.Rejected, 1)
			s.enqueue(Event{Class: class, Method: method, Outcome: "rejected", Code: "QUERY_BUDGET_EXCEEDED"})
		}
		return nil, ErrBudget
	}
	if s.used < MaxNumber {
		s.used++
	} // Saturating usage cannot reopen a bounded budget.
	if s.finished {
		return nil, nil
	}
	s.add(&s.sum.Dispatched, 1)
	if len(s.pending) >= MaxTracking {
		s.add(&s.sum.TrackingLimited, 1)
		s.add(&s.sum.Incomplete, 1)
		s.sum.Truncated = true
		if eligible && s.repeat {
			s.add(&s.sum.ShapeUnknown, 1)
		}
		s.enqueue(Event{Class: class, Method: method, Outcome: "tracking_limited", Code: "tracking_limited"})
		return nil, nil
	}
	t := &Ticket{s: s, start: time.Now(), event: Event{Class: class, Method: method}, eligible: eligible}
	if s.repeat && len(k) > 2048 {
		s.sum.Truncated = true
	}
	if s.repeat && eligible && owner != nil && k != "" && len(k) <= 2048 {
		if s.shapes == nil {
			s.shapes = make(map[shape]*group)
		}
		key := shape{Owner: owner, Key: k}
		t.group = s.shapes[key]
		if t.group == nil {
			if len(s.shapes) < MaxShapes {
				t.group = &group{}
				s.shapes[key] = t.group
			} else {
				s.sum.Truncated = true
			}
		}
	}
	s.pending[t] = struct{}{}
	s.add(&s.sum.InFlight, 1)
	return t, nil
}
func Reject(ctx context.Context, causes ...error) {
	s := From(ctx)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.add(&s.sum.PreflightRejected, 1)
	code := "unknown"
	if len(causes) > 0 {
		code = Classify(causes[0], "unknown")
	}
	s.enqueue(Event{Class: "query", Method: "terminal", Outcome: "rejected", Code: code})
}
func (t *Ticket) Dispatch() {
	if t == nil {
		return
	}
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[t]; !ok {
		return
	}
	t.event.DispatchNS = Metric{true, time.Since(t.start).Nanoseconds()}
}
func (t *Ticket) Update(consumed, scans int64) {
	if t == nil {
		return
	}
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[t]; ok {
		t.event.Consumed = Metric{true, consumed}
		t.event.Scans = Metric{true, scans}
	}
}
func (t *Ticket) End(outcome, code string, consumed, scans, returned Metric) {
	if t == nil {
		return
	}
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[t]; !ok {
		return
	}
	e := t.event
	e.Outcome = outcome
	e.Code = code
	e.Consumed = consumed
	e.Scans = scans
	e.Returned = returned
	if outcome != "handoff" {
		e.LifecycleNS = Metric{true, time.Since(t.start).Nanoseconds()}
	}
	s.end(t, e)
}
func (s *State) end(t *Ticket, e Event) {
	delete(s.pending, t)
	s.sum.InFlight--
	e = e.Safe()
	if e.Saturated {
		s.sum.Saturated = true
	}
	switch e.Outcome {
	case "incomplete", "handoff":
		s.add(&s.sum.Incomplete, 1)
		if e.Outcome == "handoff" {
			s.add(&s.sum.Handoffs, 1)
		}
	default:
		s.add(&s.sum.Completed, 1)
		switch e.Outcome {
		case "success":
			s.add(&s.sum.Successful, 1)
		case "partial":
			s.add(&s.sum.Partial, 1)
		default:
			s.add(&s.sum.Failed, 1)
		}
	}
	s.add(&s.sum.DispatchNS, e.DispatchNS.Value)
	s.add(&s.sum.LifecycleNS, e.LifecycleNS.Value)
	s.add(&s.sum.Consumed, e.Consumed.Value)
	s.add(&s.sum.Scans, e.Scans.Value)
	if e.Returned.Known {
		s.add(&s.sum.ReturnedKnown, 1)
		s.add(&s.sum.Returned, e.Returned.Value)
	}
	if s.repeat && t.eligible {
		if (e.Outcome == "success" || e.Outcome == "partial") && e.Code == "none" && t.group != nil {
			if t.group.n < MaxNumber {
				t.group.n++
			} else {
				s.sum.Saturated = true
			}
			n := t.group.n
			if n > s.sum.MaxRepeat {
				s.sum.MaxRepeat = n
			}
			if n == 5 {
				s.add(&s.sum.SuspectedGroups, 1)
				e.Code = "N_PLUS_ONE_SUSPECTED"
			}
		} else {
			s.add(&s.sum.ShapeUnknown, 1)
		}
	}
	s.enqueue(e)
}
func (s *State) Snapshot() Summary {
	if s == nil {
		return Summary{}.Safe()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.sum
	v.Finished = s.finished
	if !s.finished {
		elapsed := time.Since(s.start).Nanoseconds()
		v.WallNS = bounded(elapsed)
		if elapsed < 0 || elapsed > MaxNumber {
			v.Saturated = true
		}
	}
	return v.Safe()
}
func (s *State) Finish() Summary {
	if s == nil {
		return Summary{}.Safe()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.finished {
		for t := range s.pending {
			e := t.event
			e.Outcome = "incomplete"
			e.Code = "unknown"
			s.end(t, e)
		}
		elapsed := time.Since(s.start).Nanoseconds()
		s.sum.WallNS = bounded(elapsed)
		if elapsed < 0 || elapsed > MaxNumber {
			s.sum.Saturated = true
		}
		s.finished = true
		s.shapes = nil
	}
	v := s.sum
	v.Finished = true
	return v.Safe()
}
func (s *State) String() string             { return s.Snapshot().String() }
func (s *State) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(s.String())) }

// Drain returns per-call delivery attempts and recovered panics, not SQL counters.
func (s *State) Drain(hook func(Event)) (attempts, panics int64, err error) {
	if s == nil || hook == nil {
		return 0, 0, ErrOptions
	}
	if !s.draining.CompareAndSwap(false, true) {
		return 0, 0, ErrBusy
	}
	defer s.draining.Store(false)
	s.mu.Lock()
	batch := s.events
	s.events = nil
	s.mu.Unlock()
	for _, e := range batch {
		attempts++
		if call(hook, e.Safe()) {
			panics++
		}
	}
	return
}
func call(h func(Event), e Event) (failed bool) {
	failed = true
	defer func() { _ = recover() }()
	h(e)
	return false
}
func Tx(ctx context.Context, method string, err error, d time.Duration) {
	s := From(ctx)
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.add(&s.sum.TxBoundaries, 1)
	out := "success"
	if err != nil {
		s.add(&s.sum.TxErrors, 1)
		out = "error"
	}
	s.enqueue(Event{Class: "transaction", Method: method, Outcome: out, Code: Classify(err, "unknown"), LifecycleNS: Metric{true, d.Nanoseconds()}})
}

func Clock(ctx context.Context) time.Time {
	if !Enabled(ctx) {
		return time.Time{}
	}
	return time.Now()
}
func Elapsed(start time.Time) time.Duration {
	if start.IsZero() {
		return 0
	}
	return time.Since(start)
}

func (t *Ticket) Affected(n int64, known bool) {
	if t == nil {
		return
	}
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[t]; ok {
		t.event.Affected = Metric{known, n}
	}
}

func (t *Ticket) Saturate() {
	if t == nil {
		return
	}
	s := t.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[t]; ok {
		t.event.Saturated = true
		s.sum.Saturated = true
	}
}
