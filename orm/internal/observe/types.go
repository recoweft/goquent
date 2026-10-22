// Package observe owns private recording capabilities, never SQL or arguments.
package observe

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/publicview"
)

const MaxNumber int64 = 1<<53 - 1
const Coverage = "goquent_executor_boundaries"

var ErrBudget = errors.New("QUERY_BUDGET_EXCEEDED")
var ErrOptions = errors.New("OBSERVATION_OPTIONS")
var ErrNested = errors.New("OBSERVATION_NESTED_SCOPE")
var ErrBusy = errors.New("OBSERVATION_DRAIN_BUSY")

// Metric distinguishes an observed zero from an unavailable measurement.
type Metric struct {
	Known bool  `json:"known"`
	Value int64 `json:"value"`
}

func (m Metric) Safe() Metric {
	if !m.Known {
		m.Value = 0
	}
	m.Value = bounded(m.Value)
	return m
}
func bounded(n int64) int64 {
	if n < 0 {
		return 0
	}
	if n > MaxNumber {
		return MaxNumber
	}
	return n
}
func fixed(s string, values ...string) string {
	for _, v := range values {
		if s == v {
			return s
		}
	}
	return "unknown"
}

// Event is a detached statement or transaction boundary, not execution evidence.
type Event struct {
	Saturated   bool   `json:"saturated"`
	Kind        string `json:"kind"`
	Version     int    `json:"version"`
	Coverage    string `json:"coverage"`
	Class       string `json:"class"`
	Method      string `json:"method"`
	Outcome     string `json:"outcome"`
	Code        string `json:"code"`
	DispatchNS  Metric `json:"dispatch_ns"`
	LifecycleNS Metric `json:"lifecycle_ns"`
	Consumed    Metric `json:"consumed"`
	Scans       Metric `json:"scans"`
	Returned    Metric `json:"returned"`
	Affected    Metric `json:"affected"`
}

func (e Event) Safe() Event {
	e.Kind = "goquent.observation_event"
	e.Version = 1
	e.Coverage = Coverage
	e.Class = fixed(e.Class, "query", "internal", "transaction")
	e.Method = fixed(e.Method, "exec", "query", "row", "begin", "commit", "rollback", "terminal")
	e.Outcome = fixed(e.Outcome, "success", "error", "rejected", "partial", "incomplete", "handoff", "tracking_limited")
	e.Code = fixed(e.Code, "none", "cancelled", "deadline", "no_rows", "dispatch", "scan", "rows", "close", "result", "policy", "QUERY_BUDGET_EXCEEDED", "N_PLUS_ONE_SUSPECTED", "tracking_limited")
	for _, m := range []Metric{e.DispatchNS, e.LifecycleNS, e.Consumed, e.Scans, e.Returned, e.Affected} {
		if m.Known && (m.Value < 0 || m.Value > MaxNumber) {
			e.Saturated = true
		}
	}
	e.DispatchNS = e.DispatchNS.Safe()
	e.LifecycleNS = e.LifecycleNS.Safe()
	e.Consumed = e.Consumed.Safe()
	e.Scans = e.Scans.Safe()
	e.Returned = e.Returned.Safe()
	e.Affected = e.Affected.Safe()
	return e
}
func (e Event) MarshalJSON() ([]byte, error) { type wire Event; return json.Marshal(wire(e.Safe())) }
func (e Event) String() string               { b, _ := e.MarshalJSON(); return string(b) }
func (e Event) Format(s fmt.State, _ rune)   { _, _ = s.Write([]byte(e.String())) }
func (e *Event) UnmarshalJSON(b []byte) error {
	type wire Event
	var w wire
	if publicview.Decode(b, "goquent.observation_event", &w) != nil {
		return publicview.ErrInvalid
	}
	v := Event(w)
	if v != v.Safe() {
		return publicview.ErrInvalid
	}
	*e = v
	return nil
}
func (m Metric) MarshalJSON() ([]byte, error) { type wire Metric; return json.Marshal(wire(m.Safe())) }
func (m Metric) String() string               { b, _ := m.MarshalJSON(); return string(b) }
func (m Metric) Format(s fmt.State, _ rune)   { _, _ = s.Write([]byte(m.String())) }

// Summary describes only the observation interval. Delivery is separate.
type Summary struct {
	Kind              string `json:"kind"`
	Version           int    `json:"version"`
	Coverage          string `json:"coverage"`
	Finished          bool   `json:"finished"`
	Saturated         bool   `json:"saturated"`
	Truncated         bool   `json:"truncated"`
	Attempted         int64  `json:"attempted"`
	Dispatched        int64  `json:"dispatched"`
	Rejected          int64  `json:"rejected"`
	PreflightRejected int64  `json:"preflight_rejected"`
	Completed         int64  `json:"completed"`
	Incomplete        int64  `json:"incomplete"`
	InFlight          int64  `json:"in_flight"`
	Handoffs          int64  `json:"handoffs"`
	TrackingLimited   int64  `json:"tracking_limited"`
	Successful        int64  `json:"successful"`
	Failed            int64  `json:"failed"`
	Partial           int64  `json:"partial"`
	DispatchNS        int64  `json:"dispatch_ns"`
	LifecycleNS       int64  `json:"lifecycle_ns"`
	WallNS            int64  `json:"wall_ns"`
	Consumed          int64  `json:"consumed"`
	Scans             int64  `json:"scans"`
	Returned          int64  `json:"returned"`
	ReturnedKnown     int64  `json:"returned_known"`
	TxBoundaries      int64  `json:"tx_boundaries"`
	TxErrors          int64  `json:"tx_errors"`
	SuspectedGroups   int64  `json:"suspected_groups"`
	MaxRepeat         int64  `json:"max_repeat"`
	ShapeUnknown      int64  `json:"shape_unknown"`
	Dropped           int64  `json:"dropped"`
}

func (v Summary) Safe() Summary {
	v.Kind = "goquent.observation_summary"
	v.Version = 1
	v.Coverage = Coverage
	for _, p := range []*int64{&v.Attempted, &v.Dispatched, &v.Rejected, &v.PreflightRejected, &v.Completed, &v.Incomplete, &v.InFlight, &v.Handoffs, &v.TrackingLimited, &v.Successful, &v.Failed, &v.Partial, &v.DispatchNS, &v.LifecycleNS, &v.WallNS, &v.Consumed, &v.Scans, &v.Returned, &v.ReturnedKnown, &v.TxBoundaries, &v.TxErrors, &v.SuspectedGroups, &v.MaxRepeat, &v.ShapeUnknown, &v.Dropped} {
		if *p < 0 || *p > MaxNumber {
			v.Saturated = true
		}
		*p = bounded(*p)
	}
	return v
}
func (v Summary) MarshalJSON() ([]byte, error) {
	type wire Summary
	return json.Marshal(wire(v.Safe()))
}
func (v Summary) String() string             { b, _ := v.MarshalJSON(); return string(b) }
func (v Summary) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func (v *Summary) UnmarshalJSON(b []byte) error {
	type wire Summary
	var w wire
	if publicview.Decode(b, "goquent.observation_summary", &w) != nil {
		return publicview.ErrInvalid
	}
	x := Summary(w)
	if x != x.Safe() {
		return publicview.ErrInvalid
	}
	*v = x
	return nil
}

// Classify never invokes methods on application errors, including Is/As/Unwrap.
func Classify(err error, fallback string) string {
	if err == nil {
		return "none"
	}
	if eq(err, context.Canceled) {
		return "cancelled"
	}
	if eq(err, context.DeadlineExceeded) {
		return "deadline"
	}
	if eq(err, sql.ErrNoRows) {
		return "no_rows"
	}
	if eq(err, ErrBudget) {
		return "QUERY_BUDGET_EXCEEDED"
	}
	return fallback
}
func eq(a, b error) bool { return a == b }
