package query

import (
	"bytes"
	"context"
	"crypto/hmac"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/planidentity"
	"github.com/recoweft/goquent/orm/internal/planversion"
	"github.com/recoweft/goquent/orm/predicate"
	"github.com/recoweft/goquent/orm/scanner"
)

var (
	ErrBindingContext     = errors.New("goquent: invalid binding context")
	ErrBindingUnavailable = errors.New("goquent: binding unavailable")
	ErrBindingMismatch    = errors.New("goquent: binding mismatch")
	ErrBindingExpired     = errors.New("goquent: binding expired")
	ErrBindingOwner       = errors.New("goquent: binding owner mismatch")
	ErrBindingConsumed    = errors.New("goquent: binding already attempted")
)

// BindingContextInput supplies application-managed key and target assertions.
// Never log Key. These assertions do not attest the physical database.
type BindingContextInput struct {
	Key                                []byte
	Scope, Generation, Target, Dialect string
}

// BindingContext is an immutable, process-local context with no secret getters.
type BindingContext struct{ input *BindingContextInput }

// NewBindingContext validates and detaches explicit application key/target input.
func NewBindingContext(input BindingContextInput) (BindingContext, error) {
	if len(input.Key) < 32 || input.Scope == "" || input.Generation == "" || input.Target == "" || (input.Dialect != "mysql" && input.Dialect != "postgres") {
		return BindingContext{}, ErrBindingContext
	}
	if _, err := planidentity.Canonical([]any{input.Key, input.Scope, input.Generation, input.Target, input.Dialect}); err != nil {
		return BindingContext{}, ErrBindingContext
	}
	input.Key = append([]byte(nil), input.Key...)
	return BindingContext{input: &input}, nil
}
func (BindingContext) MarshalJSON() ([]byte, error) { return nil, ErrBindingContext }
func (c *BindingContext) UnmarshalJSON([]byte) error {
	if c != nil {
		*c = BindingContext{}
	}
	return ErrBindingContext
}
func (BindingContext) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte("<binding context>")) }

// BindingCurrent must be supplied from current trusted application state on
// every call. Reusing stale assertions cannot detect external changes.
type BindingCurrent struct {
	Settings       Settings
	BindingContext BindingContext
}

// ValidatedPlan is an opaque single-attempt handle. Copies share consumption.
// It is neither a serialized permit nor a replacement for application authority.
type ValidatedPlan struct{ state *bindingState }
type bindingState struct {
	used                  atomic.Bool
	owner                 *Query
	executor              executor
	context, queryContext context.Context
	binding               BindingContext
	snapshot              *planidentity.Snapshot
	ids                   planidentity.IDs
	settings, operation   []byte
	inspection            []byte
	expires               time.Time
}

func (ValidatedPlan) MarshalJSON() ([]byte, error) { return nil, ErrBindingUnavailable }
func (h *ValidatedPlan) UnmarshalJSON([]byte) error {
	if h != nil {
		*h = ValidatedPlan{}
	}
	return ErrBindingUnavailable
}
func (ValidatedPlan) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte("<validated plan>")) }

// bindingError hides source text while retaining errors.Is without exposing an
// underlying error whose display might contain operation values.
type bindingError struct{ cause error }

func (bindingError) Error() string { return "goquent: binding inspection rejected" }
func (bindingError) Format(s fmt.State, _ rune) {
	_, _ = s.Write([]byte("goquent: binding inspection rejected"))
}
func (e bindingError) Is(target error) bool { return errors.Is(e.cause, target) }
func bindingFailure(err error) error {
	if err == nil {
		return nil
	}
	return bindingError{err}
}

func bindingComparable(v any) bool { return v == nil || reflect.ValueOf(v).Comparable() }
func bindingSame(a, b any) bool    { return bindingComparable(a) && bindingComparable(b) && a == b }
func bindingContextEqual(a, b BindingContext) bool {
	if a.input == nil || b.input == nil {
		return false
	}
	x, y := a.input, b.input
	return x.Scope == y.Scope && x.Generation == y.Generation && x.Target == y.Target && x.Dialect == y.Dialect && hmac.Equal(x.Key, y.Key)
}
func (q *Query) bindingContext(ctx context.Context) (context.Context, error) {
	if q == nil || q.builder == nil || bindingNil(q.exec) {
		return nil, ErrBindingOwner
	}
	if ctx == nil {
		ctx = q.ctx
	}
	if !bindingComparable(ctx) || !bindingComparable(q.ctx) || (ctx != nil && bindingNil(ctx)) || (q.ctx != nil && bindingNil(q.ctx)) {
		return nil, ErrBindingUnavailable
	}
	return ctx, nil
}

// Settings are compared independently of Query's additional policy override.
// Otherwise that override could conceal a changed current PolicySet.
func bindingSettings(s Settings) ([]byte, error) {
	if s.err != nil {
		return nil, bindingFailure(s.err)
	}
	for _, p := range s.policies.Policies() {
		if err := planversion.Check(p.Version); err != nil {
			return nil, bindingFailure(err)
		}
	}
	fields, err := identityFields(struct {
		Strict, AutoTenant, Application bool
		Database                        string
		Schema                          ApplicationSchemaInput
		Policies                        []TablePolicy
		Risk                            RiskConfig
	}{s.strict, s.autoTenant, s.execution.application, s.database, s.schema.Input(), s.policies.Policies(), s.risk})
	if err != nil {
		return nil, bindingFailure(err)
	}
	out, err := planidentity.Canonical([]any{fields, s.execution.input.TenantPresent, s.execution.input.CurrentTenant})
	if err != nil {
		return nil, ErrBindingUnavailable
	}
	return out, nil
}
func (q *Query) checkBindingCurrent(c BindingCurrent) ([]byte, error) {
	b, s := c.BindingContext.input, c.Settings
	if b == nil || len(b.Key) < 32 || s.database == "" || s.schema.input == nil || b.Target != s.database || b.Target != s.schema.input.Database || b.Dialect != s.schema.input.Dialect {
		return nil, ErrBindingContext
	}
	dialect := ""
	switch q.dialect.(type) {
	case driver.MySQLDialect:
		dialect = "mysql"
	case driver.PostgresDialect:
		dialect = "postgres"
	}
	if dialect != b.Dialect {
		return nil, ErrBindingMismatch
	}
	current, err := bindingSettings(s)
	if err != nil {
		return nil, err
	}
	original, err := bindingSettings(q.settings)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(current, original) {
		return nil, ErrBindingMismatch
	}
	return current, nil
}

// Each family calls the existing planner once. Only clauses actually used by
// that planner and its gates participate; INSERT does not execute WHERE clauses.
func (q *Query) bindingPlan(ctx context.Context, family string, data any, batch []map[string]any, columns []string) (*QueryPlan, error) {
	if q.err != nil {
		return nil, bindingFailure(q.err)
	}
	if q.policy != nil {
		if err := planversion.Check(q.policy.Version); err != nil {
			return nil, bindingFailure(err)
		}
	}
	var p *QueryPlan
	var err error
	switch family {
	case "select":
		p, err = q.Plan(ctx)
	case "count":
		b := newSelectBuilder(q.dialect)
		b.Table(q.tableName())
		q.builder.CopyStateToSelect(b)
		b.Count(columns...)
		p, err = q.planSelectBuilder(ctx, b)
	case "insert":
		p, err = q.PlanInsert(ctx, data)
	case "batch":
		p, err = q.PlanInsertBatch(ctx, batch)
	case "update":
		p, err = q.PlanUpdate(ctx, data)
	case "delete":
		p, err = q.PlanDelete(ctx)
	default:
		return nil, ErrBindingUnavailable
	}
	if err != nil {
		return nil, bindingFailure(err)
	}
	if p == nil || p.execution == nil {
		return nil, ErrBindingUnavailable
	}
	e := p.execution
	e.ctx = ctx
	if e.gate != nil {
		return nil, bindingFailure(e.gate)
	}
	if e.identityErr != nil || e.identity == nil {
		return nil, ErrBindingUnavailable
	}
	return p, nil
}

func (q *Query) bindingOperation(family string, columns []string) ([]byte, error) {
	cols := make([]any, len(columns))
	for i, col := range columns {
		cols[i] = col
	}
	if family == "count" && len(cols) == 0 {
		cols = []any{"*"}
	}
	// These library-owned records contain no arbitrary value callbacks. Approval
	// creation clocks are not semantic; expiry is handled separately.
	config, err := identityFields(struct {
		Suppressions []Suppression
		AccessReason string
	}{q.suppressions, q.accessReason})
	if err != nil {
		return nil, ErrBindingUnavailable
	}
	out, err := planidentity.Canonical([]any{family, cols, config})
	if err != nil {
		return nil, ErrBindingUnavailable
	}
	return out, nil
}

func (q *Query) validateBinding(ctx context.Context, c BindingCurrent, expiry time.Time, family string, data any, batch []map[string]any, columns []string) (*ValidatedPlan, *QueryPlan, error) {
	effective, err := q.bindingContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	if expiry.IsZero() || !expiry.After(time.Now()) {
		return nil, nil, ErrBindingExpired
	}
	if effective != nil {
		if err := effective.Err(); err != nil {
			return nil, nil, bindingFailure(err)
		}
	}
	settings, err := q.checkBindingCurrent(c)
	if err != nil {
		return nil, nil, err
	}
	operation, err := q.bindingOperation(family, columns)
	if err != nil {
		return nil, nil, err
	}
	p, err := q.bindingPlan(effective, family, data, batch, columns)
	if err != nil {
		return nil, nil, err
	}
	e := p.execution
	ids, err := e.identity.Identify(planidentity.Key{Secret: c.BindingContext.input.Key, Scope: c.BindingContext.input.Scope, Generation: c.BindingContext.input.Generation})
	if err != nil {
		return nil, nil, ErrBindingUnavailable
	}
	expiry = bindingDeadline(expiry, e.expires)
	if !expiry.After(time.Now()) {
		return nil, nil, ErrBindingExpired
	}
	if effective != nil {
		if err := effective.Err(); err != nil {
			return nil, nil, bindingFailure(err)
		}
	}
	inspection, err := bindingInspection(e.inspection)
	if err != nil {
		return nil, nil, ErrBindingUnavailable
	}
	if !expiry.After(time.Now()) {
		return nil, nil, ErrBindingExpired
	}
	h := &ValidatedPlan{state: &bindingState{owner: q, executor: q.exec, context: effective, queryContext: q.ctx, binding: c.BindingContext, snapshot: e.identity, ids: ids, inspection: inspection, settings: settings, operation: operation, expires: expiry}}
	// Detach the optional approval timestamp from the Query as well.
	if p.Approval != nil && p.Approval.ExpiresAt != nil {
		exp := *p.Approval.ExpiresAt
		p.Approval.ExpiresAt = &exp
	}
	// No private execution or inspection evidence leaves the validated entry.
	p.execution, p.tenantEvidence, p.writeEvidence, p.conditionSource = nil, nil, nil, nil
	p.generatedConditions = false
	return h, p, nil
}

func (q *Query) prepareBinding(ctx context.Context, c BindingCurrent, h *ValidatedPlan, family string, data any, batch []map[string]any, columns []string) (*QueryPlan, error) {
	if h == nil || h.state == nil {
		return nil, ErrBindingUnavailable
	}
	s := h.state
	if !s.used.CompareAndSwap(false, true) {
		return nil, ErrBindingConsumed
	}
	effective, err := q.bindingContext(ctx)
	if err != nil {
		return nil, err
	}
	if s.owner != q || !bindingSame(s.context, effective) || !bindingSame(s.queryContext, q.ctx) {
		return nil, ErrBindingOwner
	}
	if !bindingSame(s.executor, q.exec) {
		return nil, ErrBindingOwner
	}
	if !s.expires.After(time.Now()) {
		return nil, ErrBindingExpired
	}
	if effective != nil {
		if err := effective.Err(); err != nil {
			return nil, bindingFailure(err)
		}
	}
	settings, err := q.checkBindingCurrent(c)
	if err != nil {
		return nil, err
	}
	if !bindingContextEqual(s.binding, c.BindingContext) || !bytes.Equal(s.settings, settings) {
		return nil, ErrBindingMismatch
	}
	operation, err := q.bindingOperation(family, columns)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(s.operation, operation) {
		return nil, ErrBindingMismatch
	}
	p, err := q.bindingPlan(effective, family, data, batch, columns)
	if err != nil {
		return nil, err
	}
	inspection, err := bindingInspection(p.execution.inspection)
	if err != nil {
		return nil, ErrBindingUnavailable
	}
	ids, err := p.execution.identity.Identify(planidentity.Key{Secret: c.BindingContext.input.Key, Scope: c.BindingContext.input.Scope, Generation: c.BindingContext.input.Generation})
	if err != nil {
		return nil, ErrBindingUnavailable
	}
	if ids != s.ids || !bytes.Equal(s.inspection, inspection) || !s.snapshot.Equal(p.execution.identity) {
		return nil, ErrBindingMismatch
	}
	expiry := bindingDeadline(s.expires, p.execution.expires)
	p.execution.bindingExpires = &expiry
	if effective != nil {
		if err := effective.Err(); err != nil {
			return nil, bindingFailure(err)
		}
	}
	if !expiry.After(time.Now()) {
		return nil, ErrBindingExpired
	}
	return p, nil
}

// ValidateSelect inspects the current SELECT without accessing the database.
func (q *Query) ValidateSelect(ctx context.Context, c BindingCurrent, expiresAt time.Time) (*ValidatedPlan, *QueryPlan, error) {
	return q.validateBinding(ctx, c, expiresAt, "select", nil, nil, nil)
}

// ValidateCount inspects COUNT with explicit current columns without DB access.
func (q *Query) ValidateCount(ctx context.Context, c BindingCurrent, expiresAt time.Time, columns ...string) (*ValidatedPlan, *QueryPlan, error) {
	return q.validateBinding(ctx, c, expiresAt, "count", nil, nil, columns)
}

// ValidateInsert inspects a single INSERT from current struct/map input.
func (q *Query) ValidateInsert(ctx context.Context, c BindingCurrent, expiresAt time.Time, data any) (*ValidatedPlan, *QueryPlan, error) {
	return q.validateBinding(ctx, c, expiresAt, "insert", data, nil, nil)
}

// ValidateInsertBatch inspects all candidates in one INSERT statement.
func (q *Query) ValidateInsertBatch(ctx context.Context, c BindingCurrent, expiresAt time.Time, data []map[string]any) (*ValidatedPlan, *QueryPlan, error) {
	return q.validateBinding(ctx, c, expiresAt, "batch", nil, data, nil)
}

// ValidateUpdate inspects current UPDATE data and effective conditions.
func (q *Query) ValidateUpdate(ctx context.Context, c BindingCurrent, expiresAt time.Time, data any) (*ValidatedPlan, *QueryPlan, error) {
	return q.validateBinding(ctx, c, expiresAt, "update", data, nil, nil)
}

// ValidateDelete inspects current DELETE conditions without DB access.
func (q *Query) ValidateDelete(ctx context.Context, c BindingCurrent, expiresAt time.Time) (*ValidatedPlan, *QueryPlan, error) {
	return q.validateBinding(ctx, c, expiresAt, "delete", nil, nil, nil)
}

// ExecuteValidatedSelect attempts a handle once and scans using Get semantics.
func (q *Query) ExecuteValidatedSelect(ctx context.Context, c BindingCurrent, h *ValidatedPlan, dest any) error {
	p, err := q.prepareBinding(ctx, c, h, "select", nil, nil, nil)
	if err != nil {
		return err
	}
	return q.executeRows(p, func(rows *sql.Rows) error { return scanner.Structs(dest, rows) })
}

// ExecuteValidatedCount attempts a handle once with current COUNT columns.
func (q *Query) ExecuteValidatedCount(ctx context.Context, c BindingCurrent, h *ValidatedPlan, columns ...string) (int64, error) {
	p, err := q.prepareBinding(ctx, c, h, "count", nil, nil, columns)
	if err != nil {
		return 0, err
	}
	var n int64
	err = q.executeRow(p, &n)
	return n, err
}

// ExecuteValidatedInsert attempts a handle once with current struct/map data.
func (q *Query) ExecuteValidatedInsert(ctx context.Context, c BindingCurrent, h *ValidatedPlan, data any) (sql.Result, error) {
	p, err := q.prepareBinding(ctx, c, h, "insert", data, nil, nil)
	if err != nil {
		return nil, err
	}
	return q.executeResult(p)
}

// ExecuteValidatedInsertBatch attempts a handle once with all current candidates.
func (q *Query) ExecuteValidatedInsertBatch(ctx context.Context, c BindingCurrent, h *ValidatedPlan, data []map[string]any) (sql.Result, error) {
	p, err := q.prepareBinding(ctx, c, h, "batch", nil, data, nil)
	if err != nil {
		return nil, err
	}
	return q.executeResult(p)
}

// ExecuteValidatedUpdate attempts a handle once with current UPDATE data.
func (q *Query) ExecuteValidatedUpdate(ctx context.Context, c BindingCurrent, h *ValidatedPlan, data any) (sql.Result, error) {
	p, err := q.prepareBinding(ctx, c, h, "update", data, nil, nil)
	if err != nil {
		return nil, err
	}
	return q.executeResult(p)
}

// ExecuteValidatedDelete attempts a handle once with current DELETE conditions.
func (q *Query) ExecuteValidatedDelete(ctx context.Context, c BindingCurrent, h *ValidatedPlan) (sql.Result, error) {
	p, err := q.prepareBinding(ctx, c, h, "delete", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	return q.executeResult(p)
}

// bindingInspection retains typed tree material in addition to final SQL/args.
// It never marshals arbitrary predicate values or calls application methods.
func bindingInspection(p *QueryPlan) ([]byte, error) {
	var node func(*predicate.Node) any
	node = func(n *predicate.Node) any {
		if n == nil {
			return nil
		}
		children := make([]any, len(n.Children))
		for i, c := range n.Children {
			children[i] = node(c)
		}
		values := make([]any, len(n.Values))
		for i, v := range n.Values {
			values[i] = []any{v.Isolation, v.Reason, v.Data}
		}
		parameters := make([]any, len(n.Parameters))
		for i, v := range n.Parameters {
			parameters[i] = v
		}
		bound := make([]any, len(n.BoundColumns))
		for i, v := range n.BoundColumns {
			bound[i] = v
		}
		return []any{n.Kind, n.Column, n.Operator, n.ValueColumn, n.SQL, n.Correspondence, children, values, parameters, bound}
	}
	tables, err := identityFields(p.Tables)
	if err != nil {
		return nil, err
	}
	columns, err := identityFields(p.Columns)
	if err != nil {
		return nil, err
	}
	joins := make([]any, len(p.Joins))
	for i, j := range p.Joins {
		joins[i] = []any{j.Type, j.Table, j.Alias, j.LeftColumn, j.Operator, j.RightColumn, j.Subquery, node(j.OnTree)}
	}
	return planidentity.Canonical([]any{int64(p.Version), string(p.Operation), tables, columns, joins, node(p.WhereTree), node(p.HavingTree)})
}

func bindingNil(v any) bool {
	for {
		e, ok := v.(*ownedExecutor)
		if !ok {
			break
		}
		if e == nil {
			return true
		}
		v = e.executor
	}
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}

func bindingDeadline(explicit time.Time, approval *time.Time) time.Time {
	if approval != nil && approval.Before(explicit) {
		return *approval
	}
	return explicit
}
