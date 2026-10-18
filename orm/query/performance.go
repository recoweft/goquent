package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/publicview"
	qbapi "github.com/recoweft/goquent/orm/internal/querybuilder/api"
)

// PerformanceOptions may shorten, but never extend, the five second total budget.
type PerformanceOptions struct{ Timeout time.Duration }

// PerformanceError is a closed public error without a retained driver cause.
type PerformanceError string

func (e PerformanceError) Error() string              { return performanceCode(string(e)) }
func (e PerformanceError) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(e.Error())) }

const (
	ErrPerformanceUnsupported PerformanceError = "PERFORMANCE_UNSUPPORTED"
	ErrPerformanceBlocked     PerformanceError = "PERFORMANCE_BLOCKED"
	ErrPerformanceConnection  PerformanceError = "PERFORMANCE_CONNECTION"
	ErrPerformanceQuery       PerformanceError = "PERFORMANCE_QUERY"
	ErrPerformanceResult      PerformanceError = "PERFORMANCE_RESULT"
	ErrPerformanceLimit       PerformanceError = "PERFORMANCE_LIMIT"
	ErrPerformanceCanceled    PerformanceError = "PERFORMANCE_CANCELED"
	ErrPerformanceTimeout     PerformanceError = "PERFORMANCE_TIMEOUT"
	ErrPerformanceCleanup     PerformanceError = "PERFORMANCE_CLEANUP"
)

func performanceCode(s string) string {
	switch PerformanceError(s) {
	case ErrPerformanceUnsupported, ErrPerformanceBlocked, ErrPerformanceConnection, ErrPerformanceQuery, ErrPerformanceResult, ErrPerformanceLimit, ErrPerformanceCanceled, ErrPerformanceTimeout, ErrPerformanceCleanup:
		return s
	}
	return "PERFORMANCE_UNKNOWN"
}
func performanceFailure(ctx context.Context, fallback PerformanceError) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrPerformanceTimeout
	}
	if ctx.Err() != nil {
		return ErrPerformanceCanceled
	}
	return fallback
}

// PerformanceNode is sensitive source data. Rows and costs are database estimates,
// not execution results, portable units, authorization, or resource bounds.
type PerformanceNode struct {
	Scan    string   `json:"scan"`
	Index   string   `json:"index,omitempty"`
	Rows    *float64 `json:"rows,omitempty"`
	Cost    *float64 `json:"cost,omitempty"`
	Unknown bool     `json:"unknown"`
}

// PerformancePlan is a separate diagnostic source contract. Explicit JSON contains
// SQL, values and database identifiers. Use PublicView for publication.
type PerformancePlan struct {
	Kind          string            `json:"kind"`
	Version       int               `json:"version"`
	Dialect       string            `json:"dialect"`
	ServerVersion string            `json:"server_version"`
	CapturedAt    time.Time         `json:"captured_at"`
	Database      string            `json:"database"`
	Target        *QueryPlan        `json:"target"`
	Raw           json.RawMessage   `json:"raw"`
	Nodes         []PerformanceNode `json:"nodes"`
	Unknown       bool              `json:"unknown"`
	native        bool
}

const PerformancePlanKind = "goquent.performance_plan"
const PerformancePlanViewKind = "goquent.performance_plan_view"

// Native reports only this process's acquisition provenance, never a JSON claim.
func (p *PerformancePlan) Native() bool { return p != nil && p.native }
func (p *PerformancePlan) ToJSON() ([]byte, error) {
	if p == nil {
		return nil, ErrPerformanceResult
	}
	return p.MarshalJSON()
}
func (p PerformancePlan) MarshalJSON() ([]byte, error) {
	if p.Kind != PerformancePlanKind || p.Version != 1 {
		return nil, ErrPerformanceResult
	}
	type wire PerformancePlan
	b, e := json.Marshal(wire(p))
	if e != nil {
		return nil, ErrPerformanceResult
	}
	if _, e := performanceJSON(b); e != nil {
		return nil, e
	}
	return b, nil
}
func (p *PerformancePlan) UnmarshalJSON(b []byte) error {
	if p == nil {
		return ErrPerformanceResult
	}
	*p = PerformancePlan{}
	root, err := performanceJSON(b)
	if err != nil {
		return err
	}
	fields, ok := root.(map[string]any)
	if !ok {
		return ErrPerformanceResult
	}
	for k := range fields {
		switch k {
		case "kind", "version", "dialect", "server_version", "captured_at", "database", "target", "raw", "nodes", "unknown":
		default:
			return ErrPerformanceResult
		}
	}
	type wire PerformancePlan
	var v wire
	if json.Unmarshal(b, &v) != nil || v.Kind != PerformancePlanKind || v.Version != 1 {
		return ErrPerformanceResult
	}
	if len(v.Raw) > 0 {
		if _, e := performanceJSON(v.Raw); e != nil {
			return e
		}
	}
	*p = PerformancePlan(v)
	return nil
}
func (p PerformancePlan) String() string             { return p.PublicView().String() }
func (p PerformancePlan) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(p.String())) }

// PerformancePlanView excludes SQL, parameters, identifiers and server text.
// Numeric estimates are bounded display data, not a complete decision input.
type PerformancePlanView struct {
	Kind       string                `json:"kind"`
	Version    int                   `json:"version"`
	Dialect    string                `json:"dialect"`
	Provenance string                `json:"provenance"`
	Unknown    bool                  `json:"unknown"`
	Truncated  bool                  `json:"truncated"`
	Nodes      []PerformanceNodeView `json:"nodes"`
}
type PerformanceNodeView struct {
	Scan      string  `json:"scan"`
	Indexed   bool    `json:"indexed"`
	RowsKnown bool    `json:"rows_known"`
	Rows      float64 `json:"rows"`
	CostKnown bool    `json:"cost_known"`
	Cost      float64 `json:"cost"`
	Unknown   bool    `json:"unknown"`
}

func (p PerformancePlan) PublicView() PerformancePlanView {
	v := PerformancePlanView{Kind: PerformancePlanViewKind, Version: 1, Dialect: p.Dialect, Provenance: "unverified", Unknown: p.Unknown}
	if p.native {
		v.Provenance = "native_acquisition"
	}
	if p.Kind != PerformancePlanKind || p.Version != 1 {
		v.Unknown = true
		return v.safe()
	}
	v.Truncated = len(p.Nodes) > 128
	for _, n := range p.Nodes[:min(len(p.Nodes), 128)] {
		x := PerformanceNodeView{Scan: n.Scan, Indexed: n.Index != "", Unknown: n.Unknown}
		if n.Rows != nil {
			x.RowsKnown = true
			x.Rows = *n.Rows
		}
		if n.Cost != nil {
			x.CostKnown = true
			x.Cost = *n.Cost
		}
		v.Nodes = append(v.Nodes, x)
	}
	return v.safe()
}
func performanceScan(s string) string {
	switch s {
	case "sequential", "index", "index_only", "bitmap", "range", "constant", "limit", "sort", "result":
		return s
	}
	return "unknown"
}
func (v PerformancePlanView) safe() PerformancePlanView {
	if v.Dialect != "mysql" && v.Dialect != "postgres" {
		v.Dialect = "unknown"
		v.Unknown = true
	}
	if v.Provenance != "native_acquisition" {
		v.Provenance = "unverified"
	}
	v.Truncated = v.Truncated || len(v.Nodes) > 128
	out := make([]PerformanceNodeView, min(len(v.Nodes), 128))
	copy(out, v.Nodes)
	v.Nodes = out
	for i := range v.Nodes {
		n := &v.Nodes[i]
		n.Scan = performanceScan(n.Scan)
		if n.Scan == "unknown" {
			n.Unknown = true
		}
		if !performanceNumber(n.Rows) || !n.RowsKnown {
			n.Rows = 0
			n.RowsKnown = false
			n.Unknown = true
		}
		if !performanceNumber(n.Cost) || !n.CostKnown {
			n.Cost = 0
			n.CostKnown = false
			n.Unknown = true
		}
		v.Unknown = v.Unknown || n.Unknown
	}
	return v
}
func (v PerformancePlanView) MarshalJSON() ([]byte, error) {
	if v.Kind != PerformancePlanViewKind || v.Version != 1 {
		return nil, publicview.ErrInvalid
	}
	type wire PerformancePlanView
	b, e := json.Marshal(wire(v.safe()))
	if e != nil {
		return nil, publicview.ErrInvalid
	}
	return b, nil
}
func (v PerformancePlanView) ToJSON() ([]byte, error) { return v.MarshalJSON() }
func (v PerformancePlanView) String() string {
	b, e := v.MarshalJSON()
	if e != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (v PerformancePlanView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(v.String())) }
func DecodePerformancePlanView(b []byte) (PerformancePlanView, error) {
	type wire PerformancePlanView
	var v wire
	if publicview.Decode(b, PerformancePlanViewKind, &v) != nil {
		return PerformancePlanView{}, publicview.ErrInvalid
	}
	return PerformancePlanView(v).safe(), nil
}
func (v *PerformancePlanView) UnmarshalJSON(b []byte) error {
	if v == nil {
		return publicview.ErrInvalid
	}
	*v = PerformancePlanView{}
	x, e := DecodePerformancePlanView(b)
	if e == nil {
		*v = x
	}
	return e
}

// ExplainPerformance explicitly acquires a bounded non-ANALYZE plan. Ordinary
// Plan, DryRun and CRUD never call it. Unsupported shapes/executors are refused
// before connecting. See docs/performance-plans.md for the database trust boundary.
func (q *Query) ExplainPerformance(ctx context.Context, o PerformanceOptions) (*PerformancePlan, error) {
	if ctx == nil || o.Timeout < 0 || o.Timeout > 5*time.Second {
		return nil, ErrPerformanceUnsupported
	}
	budget := 5 * time.Second
	if o.Timeout > 0 {
		budget = o.Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	if ctx.Err() != nil {
		return nil, performanceFailure(ctx, ErrPerformanceCanceled)
	}
	if q == nil || q.builder == nil || q.exec == nil || q.err != nil {
		return nil, ErrPerformanceBlocked
	}
	// Copy owner state before planning; no field of the original Query is read
	// after acquisition starts. The policy builder supplies a detached structure.
	frozen := *q
	frozen.builder = newSelectBuilder(frozen.dialect)
	q.builder.CopyStateToSelect(frozen.builder)
	if frozen.ctx != nil && frozen.ctx.Err() != nil {
		return nil, performanceFailure(frozen.ctx, ErrPerformanceCanceled)
	}
	if frozen.ctx != nil {
		stop := context.AfterFunc(frozen.ctx, cancel)
		defer stop()
	}
	dialect := ""
	switch frozen.dialect.(type) {
	case driver.PostgresDialect:
		dialect = "postgres"
	case driver.MySQLDialect:
		dialect = "mysql"
	default:
		return nil, ErrPerformanceUnsupported
	}
	b, e := frozen.policyBuilder(frozen.builder)
	if e != nil {
		return nil, ErrPerformanceBlocked
	}
	statement, args, snap, shape, e := b.BuildPerformanceSnapshot()
	if e != nil {
		return nil, ErrPerformanceUnsupported
	}
	p := newQueryPlan(OperationSelect, statement, args)
	appendSelectSnapshotMetadata(p, snap)
	frozen.finalizePlan(p)
	if frozen.checkExecution(p) != nil {
		return nil, ErrPerformanceBlocked
	}
	owned, ok := frozen.exec.(*ownedExecutor)
	if !ok {
		return nil, ErrPerformanceUnsupported
	}
	db, ok := owned.executor.(*sql.DB)
	if !ok || db == nil {
		return nil, ErrPerformanceUnsupported
	}
	switch db.Driver().(type) {
	case *mysql.MySQLDriver:
		if dialect != "mysql" {
			return nil, ErrPerformanceUnsupported
		}
	case *pq.Driver:
		if dialect != "postgres" {
			return nil, ErrPerformanceUnsupported
		}
	default:
		return nil, ErrPerformanceUnsupported
	}
	// External executor labels and parsed source documents never reach this path.
	result, e := acquirePerformance(ctx, db, dialect, p, shape)
	if frozen.ctx != nil && frozen.ctx.Err() != nil {
		return nil, performanceFailure(frozen.ctx, ErrPerformanceCanceled)
	}
	if e != nil {
		return result, e
	}
	result.native = true
	return result, nil
}

func acquirePerformance(ctx context.Context, db *sql.DB, dialect string, p *QueryPlan, shape qbapi.PerformanceShape) (out *PerformancePlan, err error) {
	conn, e := db.Conn(ctx)
	if e != nil {
		return nil, performanceFailure(ctx, ErrPerformanceConnection)
	}
	defer func() {
		if conn.Close() != nil {
			err = ErrPerformanceCleanup
			out = nil
		}
	}()
	if e := conn.Raw(func(c any) error {
		t := reflect.TypeOf(c)
		if t == nil || t.Kind() != reflect.Pointer {
			return ErrPerformanceUnsupported
		}
		t = t.Elem()
		if dialect == "mysql" && t.PkgPath() == "github.com/go-sql-driver/mysql" && t.Name() == "mysqlConn" {
			return nil
		}
		if dialect == "postgres" && t.PkgPath() == "github.com/lib/pq" && t.Name() == "conn" {
			return nil
		}
		return ErrPerformanceUnsupported
	}); e != nil {
		return nil, ErrPerformanceUnsupported
	}
	return performanceTransaction(ctx, conn, dialect, p, shape)
}

func performanceTransaction(ctx context.Context, conn *sql.Conn, dialect string, p *QueryPlan, shape qbapi.PerformanceShape) (out *PerformancePlan, err error) {
	tx, e := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, performanceFailure(ctx, ErrPerformanceConnection)
	}
	defer func() {
		e := tx.Rollback()
		if e != nil && !(errors.Is(e, sql.ErrTxDone) && ctx.Err() != nil) {
			err = ErrPerformanceCleanup
			out = nil
		}
	}()
	out, err = collectPerformance(ctx, tx, dialect, p, shape)
	if ctx.Err() != nil {
		return nil, performanceFailure(ctx, ErrPerformanceCanceled)
	}
	return out, err
}

type performanceQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

// performanceRows always closes rows and refuses incomplete/oversized catalogs.
func performanceRows(ctx context.Context, exec performanceQuerier, statement string, args []any, maxRows, maxBytes int) (data [][]string, err error) {
	rows, e := exec.QueryContext(ctx, statement, args...)
	if e != nil {
		return nil, performanceFailure(ctx, ErrPerformanceQuery)
	}
	defer func() {
		if rows.Close() != nil {
			data = nil
			err = ErrPerformanceCleanup
		}
	}()
	cols, e := rows.Columns()
	if e != nil || len(cols) > 16 {
		return nil, ErrPerformanceResult
	}
	size := 0
	for rows.Next() {
		if len(data) >= maxRows {
			return nil, ErrPerformanceLimit
		}
		v := make([]string, len(cols))
		ptr := make([]any, len(cols))
		for i := range v {
			ptr[i] = &v[i]
		}
		if rows.Scan(ptr...) != nil {
			return nil, ErrPerformanceResult
		}
		for _, s := range v {
			size += len(s)
			if size > maxBytes {
				return nil, ErrPerformanceLimit
			}
		}
		data = append(data, v)
	}
	if rows.Err() != nil {
		return nil, performanceFailure(ctx, ErrPerformanceResult)
	}
	if ctx.Err() != nil {
		return nil, performanceFailure(ctx, ErrPerformanceCanceled)
	}
	return data, nil
}

func (n PerformanceNode) String() string {
	p := PerformancePlan{Kind: PerformancePlanKind, Version: 1, Nodes: []PerformanceNode{n}}
	return p.PublicView().String()
}
func (n PerformanceNode) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(n.String())) }
func (n PerformanceNodeView) safe() PerformanceNodeView {
	v := PerformancePlanView{Nodes: []PerformanceNodeView{n}}.safe()
	return v.Nodes[0]
}
func (n PerformanceNodeView) MarshalJSON() ([]byte, error) {
	type wire PerformanceNodeView
	return json.Marshal(wire(n.safe()))
}
func (n PerformanceNodeView) String() string {
	b, e := n.MarshalJSON()
	if e != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (n PerformanceNodeView) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(n.String())) }
