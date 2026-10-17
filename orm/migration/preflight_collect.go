package migration

import (
	"context"
	"database/sql"
	"sync"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/recoweft/goquent/orm/driver"
)

// Collector owns a dedicated connection. Closing it invalidates every receipt.
// Concurrent calls are serialized; neither exported labels nor arbitrary
// StatusExecutor implementations can create a collector receipt.
type Collector struct {
	generation   uint64
	mu           sync.Mutex
	conn         *sql.Conn
	dialect      driver.Dialect
	name         string
	closed       bool
	clock        func() time.Time
	queryTimeout time.Duration
	totalTimeout time.Duration
}
type preflightReceipt struct {
	generation   uint64
	owner        *Collector
	digest, plan string
}

// CollectorOptions may only reduce the built-in time bounds. Zero means default.
type CollectorOptions struct {
	Clock                      func() time.Time
	QueryTimeout, TotalTimeout time.Duration
}

func NewPreflightCollector(ctx context.Context, db *sql.DB, dialect driver.Dialect, opts CollectorOptions) (*Collector, error) {
	if ctx == nil || db == nil || opts.Clock == nil {
		return nil, ErrPreflightInput
	}
	name := ""
	switch dialect.(type) {
	case driver.MySQLDialect:
		if _, ok := db.Driver().(*mysql.MySQLDriver); !ok {
			return nil, ErrPreflightInput
		}
		name = "mysql"
	case driver.PostgresDialect:
		if _, ok := db.Driver().(*pq.Driver); !ok {
			return nil, ErrPreflightInput
		}
		name = "postgres"
	default:
		return nil, ErrPreflightInput
	}
	if opts.QueryTimeout == 0 {
		opts.QueryTimeout = 5 * time.Second
	}
	if opts.TotalTimeout == 0 {
		opts.TotalTimeout = 30 * time.Second
	}
	if opts.QueryTimeout < 0 || opts.QueryTimeout > 5*time.Second || opts.TotalTimeout < 0 || opts.TotalTimeout > 30*time.Second {
		return nil, ErrPreflightInput
	}
	acquire, cancel := context.WithTimeout(ctx, opts.TotalTimeout)
	defer cancel()
	conn, err := db.Conn(acquire)
	if err != nil {
		return nil, ErrPreflightRead
	}
	return &Collector{conn: conn, dialect: dialect, name: name, clock: opts.Clock, queryTimeout: opts.QueryTimeout, totalTimeout: opts.TotalTimeout}, nil
}
func (c *Collector) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
func (c *Collector) Evaluate(p *Preflight, opts PreflightOptions) (PreflightView, error) {
	if c == nil {
		return EvaluatePreflight(p, opts)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return evaluatePreflight(p, opts, c)
}

// Collect returns detached partial evidence on failure. Call Evaluate before
// Close to produce an observation-bound summary. Returned source is sensitive.
func (c *Collector) Collect(ctx context.Context, input *Preflight) (*Preflight, error) {
	if c == nil || ctx == nil || ValidatePreflightSource(input) != nil {
		return nil, ErrPreflightInput
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || input.Plan.Current.Context.Dialect != c.name {
		return nil, ErrPreflightInput
	}
	// A new attempt supersedes all prior receipts, including if it fails. This
	// prevents a known schema/read failure from leaving older evidence reusable.
	c.generation++
	if c.generation == 0 {
		c.closed = true
		_ = c.conn.Close()
		return nil, ErrPreflightInput
	}
	out, err := detachPreflight(input)
	if err != nil {
		return nil, err
	}
	out.Collection = nil
	ctx, cancel := context.WithTimeout(ctx, c.totalTimeout)
	defer cancel()
	col := &PreflightCollection{PlanID: out.Plan.ID, SchemaFingerprint: schemaFingerprint(out.Plan), FingerprintDomain: "goquent.preflight.schema.v1", StartedAt: c.clock().UTC(), Method: "bounded_select", IdentityBasis: "dedicated_connection_session", Scope: "explicit_target_rows", Code: "PREFLIGHT_FAILED", Results: []PreflightResult{}}
	out.Collection = col
	finish := func(code string, err error) (*Preflight, error) {
		col.Code = code
		col.Completed = err == nil
		col.FinishedAt = c.clock().UTC()
		col.Digest = collectionDigest(*col)
		out.receipt = &preflightReceipt{generation: c.generation, owner: c, digest: col.Digest, plan: out.Plan.ID}
		return out, err
	}
	before, err := c.observeSchemas(ctx, out.Plan)
	if err != nil {
		return finish("PREFLIGHT_FAILED", ErrPreflightRead)
	}
	if before != schemaFingerprint(out.Plan) {
		return finish("PREFLIGHT_SCHEMA_CHANGED", ErrPreflightRead)
	}
	for _, r := range out.Plan.Requirements {
		result := PreflightResult{RequirementID: r.ID, Status: preflightStatus(r.Status), Code: preflightCode(r.Code)}
		if r.Status == "pending" {
			targets, status, code, err := c.checkTarget(ctx, out.Plan, r)
			result.Targets = targets
			if err != nil {
				col.Results = append(col.Results, result)
				return finish("PREFLIGHT_FAILED", ErrPreflightRead)
			}
			if status != "satisfied" {
				result.Status = status
				result.Code = code
			} else {
				q, err := preflightSQL(c.name, r)
				if err != nil {
					result.Status = "unsupported"
					result.Code = "PREFLIGHT_UNSUPPORTED"
				} else {
					count, err := c.count(ctx, q)
					if err != nil {
						col.Results = append(col.Results, result)
						return finish("PREFLIGHT_FAILED", ErrPreflightRead)
					}
					result.Count = &count
					result.Completed = true
					result.Status = "satisfied"
					result.Code = "PREFLIGHT_OK"
				}
			}
		}
		col.Results = append(col.Results, result)
	}
	// Re-observe the finite row-scope attributes as well as schema catalogs.
	for i, r := range out.Plan.Requirements {
		if r.Status != "pending" {
			continue
		}
		targets, _, _, err := c.checkTarget(ctx, out.Plan, r)
		if err != nil {
			return finish("PREFLIGHT_FAILED", ErrPreflightRead)
		}
		if digestPreflight("targets", targets) != digestPreflight("targets", col.Results[i].Targets) {
			return finish("PREFLIGHT_SCHEMA_CHANGED", ErrPreflightRead)
		}
	}
	after, err := c.observeSchemas(ctx, out.Plan)
	if err != nil {
		return finish("PREFLIGHT_FAILED", ErrPreflightRead)
	}
	if after != before {
		return finish("PREFLIGHT_SCHEMA_CHANGED", ErrPreflightRead)
	}
	if ctx.Err() != nil {
		return finish("PREFLIGHT_FAILED", ErrPreflightRead)
	}
	return finish("PREFLIGHT_OK", nil)
}
