package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBudgetConcurrentFinishIsolation(t *testing.T) {
	limit := int64(17)
	ctx, s, e := Start(context.Background(), Options{QueryLimit: &limit})
	if e != nil {
		t.Fatal(e)
	}
	limit = 999
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var sent atomic.Int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticket, err := Begin(child, "query", "exec", nil, "", false)
			if err == nil {
				sent.Add(1)
				ticket.Dispatch()
				ticket.End("success", "none", Metric{}, Metric{}, Metric{})
			} else if err != ErrBudget {
				t.Error("wrong refusal")
			}
		}()
	}
	wg.Wait()
	v := s.Finish()
	if sent.Load() != 17 || v.Attempted != 100 || v.Dispatched != 17 || v.Rejected != 83 || v.Completed != 17 || v.InFlight != 0 {
		t.Fatalf("counts: %v", v)
	}
	if _, e := Begin(ctx, "query", "exec", nil, "", false); e != ErrBudget {
		t.Fatal("Finish reset budget")
	}
	if s.Snapshot() != v {
		t.Fatal("frozen snapshot changed")
	}
	if _, _, e := Start(child, Options{}); e != ErrNested {
		t.Fatal("nested accepted")
	}
	_, other, _ := Start(context.Background(), Options{})
	if other.Snapshot().Dispatched != 0 {
		t.Fatal("scope leak")
	}
}
func TestFinishLateTrackingAndOverflow(t *testing.T) {
	ctx, s, _ := Start(context.Background(), Options{DetectRepeated: true})
	var first *Ticket
	for i := 0; i < MaxTracking+3; i++ {
		x, e := Begin(ctx, "query", "query", nil, "", true)
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			first = x
		}
	}
	v := s.Snapshot()
	if v.InFlight != MaxTracking || v.TrackingLimited != 3 || v.Incomplete != 3 {
		t.Fatal(v)
	}
	v = s.Finish()
	if v.InFlight != 0 || v.Incomplete != MaxTracking+3 || !v.Truncated || v.Dropped == 0 {
		t.Fatal(v)
	}
	first.Dispatch()
	first.Update(99, 99)
	first.End("success", "none", Metric{true, 99}, Metric{}, Metric{})
	if s.Snapshot() != v {
		t.Fatal("late finish altered scope")
	}
}
func TestDrainReentryPanicAndBatch(t *testing.T) {
	ctx, s, _ := Start(context.Background(), Options{})
	x, _ := Begin(ctx, "query", "exec", nil, "", false)
	x.End("success", "none", Metric{}, Metric{}, Metric{})
	n, p, e := s.Drain(func(Event) {
		if _, _, err := s.Drain(func(Event) {}); err != ErrBusy {
			t.Error("reentry")
		}
		q, _ := Begin(ctx, "query", "exec", nil, "", false)
		q.End("success", "none", Metric{}, Metric{}, Metric{})
		panic("private-canary")
	})
	if n != 1 || p != 1 || e != nil {
		t.Fatal(n, p, e)
	}
	v := s.Finish()
	n, p, e = s.Drain(func(Event) {})
	if n != 1 || p != 0 || e != nil || s.Snapshot() != v {
		t.Fatal("delivery changed frozen SQL interval")
	}
	n, _, _ = s.Drain(func(Event) {})
	if n != 0 {
		t.Fatal("redelivery")
	}
}
func TestShapeLimitsAndUnknown(t *testing.T) {
	ctx, s, _ := Start(context.Background(), Options{DetectRepeated: true})
	owner := new(int)
	for i := 0; i < MaxShapes+2; i++ {
		x, _ := Begin(ctx, "query", "query", owner, fmt.Sprint(i), true)
		x.End("partial", "none", Metric{true, 1}, Metric{true, 1}, Metric{})
	}
	for i := 0; i < 4; i++ {
		x, _ := Begin(ctx, "query", "query", owner, "0", true)
		x.End("partial", "none", Metric{true, 1}, Metric{true, 1}, Metric{})
	}
	v := s.Finish()
	if v.SuspectedGroups != 1 || v.MaxRepeat != 5 || v.ShapeUnknown != 2 || !v.Truncated {
		t.Fatal(v)
	}
}
func TestFinishRace(t *testing.T) {
	for i := 0; i < 100; i++ {
		ctx, s, _ := Start(context.Background(), Options{})
		x, _ := Begin(ctx, "query", "exec", nil, "", false)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); x.Dispatch(); x.End("success", "none", Metric{}, Metric{}, Metric{}) }()
		go func() { defer wg.Done(); s.Finish() }()
		wg.Wait()
		v := s.Snapshot()
		if v.Completed+v.Incomplete != 1 || v.InFlight != 0 {
			t.Fatal(v)
		}
	}
}

type maliciousError []int

func (maliciousError) Error() string { panic("must not evaluate") }
func (maliciousError) Is(error) bool { panic("must not evaluate") }
func (maliciousError) Unwrap() error { panic("must not evaluate") }
func TestPublicProjectionAndUntrustedWire(t *testing.T) {
	if Classify(maliciousError{1}, "dispatch") != "dispatch" {
		t.Fatal("classification")
	}
	e := Event{Kind: "secret-canary", Coverage: "secret-canary", Class: "secret-canary", Method: "secret-canary", Outcome: "secret-canary", Code: "secret-canary", Consumed: Metric{false, 987}}
	for _, text := range []string{fmt.Sprintf("%#v", e), fmt.Sprintf("%+v", e), e.String()} {
		if strings.Contains(text, "secret-canary") || strings.Contains(text, "987") {
			t.Fatal("public leak")
		}
	}
	b, _ := e.MarshalJSON()
	var decoded Event
	if json.Unmarshal(b, &decoded) != nil {
		t.Fatal("roundtrip")
	}
	for _, bad := range [][]byte{append(b, []byte(" {}")...), []byte(strings.Replace(string(b), `"version":1`, `"version":2`, 1)), []byte(strings.Replace(string(b), `"code":"unknown"`, `"code":"secret-canary"`, 1)), []byte(strings.Replace(string(b), `"version":1`, `"version":1,"version":1`, 1)), []byte(strings.Replace(string(b), `"value":0`, `"value":-1`, 1))} {
		if json.Unmarshal(bad, &decoded) == nil {
			t.Fatal("accepted hostile wire")
		}
	}
	for _, n := range []int64{-1, MaxNumber + 1} {
		if _, _, e := Start(context.Background(), Options{QueryLimit: &n}); e != ErrOptions {
			t.Fatal("invalid option")
		}
	}
	zero := int64(0)
	ctx, _, _ := Start(context.Background(), Options{QueryLimit: &zero})
	if _, e := Begin(ctx, "query", "exec", nil, "", false); e != ErrBudget {
		t.Fatal("zero limit")
	}
}
func TestFinishRetainsBudgetOutsideInterval(t *testing.T) {
	n := int64(2)
	ctx, s, _ := Start(context.Background(), Options{QueryLimit: &n})
	frozen := s.Finish()
	for i := 0; i < 2; i++ {
		x, e := Begin(ctx, "query", "exec", nil, "", false)
		if e != nil || x != nil {
			t.Fatal("outside interval")
		}
	}
	if _, e := Begin(ctx, "query", "exec", nil, "", false); e != ErrBudget {
		t.Fatal("budget reset")
	}
	if s.Snapshot() != frozen {
		t.Fatal("outside interval statistics")
	}
}

func TestSlowDrainDoesNotLockSQLAndConcurrentDrainBusy(t *testing.T) {
	ctx, s, _ := Start(context.Background(), Options{})
	x, _ := Begin(ctx, "query", "exec", nil, "", false)
	x.End("success", "none", Metric{}, Metric{}, Metric{})
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	go func() { defer close(done); _, _, _ = s.Drain(func(Event) { close(entered); <-release }) }()
	<-entered
	if _, _, e := s.Drain(func(Event) {}); e != ErrBusy {
		t.Fatal("concurrent Drain accepted")
	}
	x, e := Begin(ctx, "query", "exec", nil, "", false)
	if e != nil {
		t.Fatal(e)
	}
	x.End("success", "none", Metric{}, Metric{}, Metric{})
	if s.Snapshot().Completed != 2 {
		t.Fatal("slow hook blocked recording")
	}
	close(release)
	<-done
	n, _, e := s.Drain(func(Event) {})
	if n != 1 || e != nil {
		t.Fatal("batch recursion")
	}
}
func TestCounterSaturationDoesNotReopenBudget(t *testing.T) {
	limit := MaxNumber
	ctx, s, _ := Start(context.Background(), Options{QueryLimit: &limit})
	s.used = MaxNumber
	s.sum.Attempted = MaxNumber
	s.sum.Rejected = MaxNumber
	if _, e := Begin(ctx, "query", "exec", nil, "", false); e != ErrBudget {
		t.Fatal("overflow budget")
	}
	v := s.Finish()
	if !v.Saturated || v.Attempted != MaxNumber || v.Rejected != MaxNumber {
		t.Fatal(v)
	}
}

func TestSummaryWireBounds(t *testing.T) {
	v := Summary{Kind: "private-canary", Coverage: "private-canary", Attempted: -1, WallNS: MaxNumber + 1}
	b, e := v.MarshalJSON()
	if e != nil || strings.Contains(string(b), "private-canary") {
		t.Fatal("projection")
	}
	var out Summary
	if json.Unmarshal(b, &out) != nil {
		t.Fatal("summary roundtrip")
	}
	for _, bad := range []string{strings.Replace(string(b), `"version":1`, `"version":1,"extra":0`, 1), strings.Replace(string(b), `"attempted":0`, `"attempted":null`, 1), strings.Replace(string(b), `"wall_ns":9007199254740991`, `"wall_ns":9007199254740992`, 1)} {
		if json.Unmarshal([]byte(bad), &out) == nil {
			t.Fatal("invalid summary accepted")
		}
	}
	if json.Unmarshal([]byte(strings.Repeat("[", 34)+strings.Repeat("]", 34)), &out) == nil {
		t.Fatal("deep JSON")
	}
	if json.Unmarshal([]byte(`{"kind":"`+strings.Repeat("x", (1<<20))+`"}`), &out) == nil {
		t.Fatal("oversize JSON")
	}
}

func TestDurationAndMetricSaturationVisible(t *testing.T) {
	ctx, s, _ := Start(context.Background(), Options{})
	s.start = time.Now().Add(-time.Duration(MaxNumber + 1000000000))
	x, _ := Begin(ctx, "query", "exec", nil, "", false)
	x.start = s.start
	x.Dispatch()
	x.Affected(MaxNumber+1, true)
	x.End("success", "none", Metric{}, Metric{}, Metric{})
	v := s.Finish()
	if !v.Saturated || v.WallNS != MaxNumber || v.DispatchNS != MaxNumber {
		t.Fatal(v)
	}
	_, _, _ = s.Drain(func(e Event) {
		if !e.Saturated || e.DispatchNS.Value != MaxNumber || e.Affected.Value != MaxNumber {
			t.Fatal(e)
		}
	})
}
