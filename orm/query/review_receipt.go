package query

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/planidentity"
	"github.com/recoweft/goquent/orm/predicate"
)

var ErrReviewBudget = errors.New("goquent: review receipt budget exceeded")

var ErrReviewReceipt = errors.New("goquent: review receipt unavailable")

// ReviewToken scopes ephemeral planner samples. It is not an execution handle.
// Review callers must use the token supplied by their active review session.
type ReviewToken struct{ state *reviewTokenState }
type reviewTokenState struct {
	ctx      context.Context
	deadline time.Time
	closed   atomic.Bool
	receipts atomic.Int64
	bytes    atomic.Int64
}

func NewReviewToken(ctx context.Context) ReviewToken {
	if ctx == nil {
		ctx = context.Background()
	}
	return ReviewToken{&reviewTokenState{ctx: ctx, deadline: time.Now().Add(time.Minute)}}
}
func (t ReviewToken) Close() {
	if t.state != nil {
		t.state.closed.Store(true)
	}
}
func (t ReviewToken) valid() bool {
	return t.state != nil && !t.state.closed.Load() && t.state.ctx.Err() == nil && time.Now().Before(t.state.deadline)
}
func (ReviewToken) MarshalJSON() ([]byte, error)  { return nil, ErrReviewReceipt }
func (t *ReviewToken) UnmarshalJSON([]byte) error { *t = ReviewToken{}; return ErrReviewReceipt }
func (ReviewToken) Format(s fmt.State, _ rune)    { _, _ = s.Write([]byte("<review session>")) }

// ReviewReceipt contains only an immutable private planner sample and its owner.
// It cannot be serialized or used by any execution API. SELECT is the initial
// supported family. Source correspondence is declared by the application's test.
type ReviewReceipt struct {
	token         *reviewTokenState
	owner         *Query
	ownerContext  context.Context
	ownerExecutor executor
	sample        *reviewSnapshot
}
type reviewSnapshot struct {
	operation, target, structure, args, config []byte
	contextKnown                               bool
}

func (ReviewReceipt) MarshalJSON() ([]byte, error)  { return nil, ErrReviewReceipt }
func (r *ReviewReceipt) UnmarshalJSON([]byte) error { *r = ReviewReceipt{}; return ErrReviewReceipt }
func (ReviewReceipt) Format(s fmt.State, _ rune)    { _, _ = s.Write([]byte("<review receipt>")) }

// ReviewReceipt plans a SELECT without executing SQL. Public QueryPlan fields
// are never accepted as receipt inputs. Missing identity context stays unknown.
func (q *Query) ReviewReceipt(ctx context.Context, t ReviewToken) (ReviewReceipt, error) {
	if q == nil || !t.valid() || ctx == nil || ctx.Err() != nil {
		return ReviewReceipt{}, ErrReviewReceipt
	}
	switch q.dialect.(type) {
	case driver.MySQLDialect, driver.PostgresDialect:
	default:
		return ReviewReceipt{}, ErrReviewReceipt
	}
	if t.state.receipts.Add(1) > 8192 {
		return ReviewReceipt{}, ErrReviewBudget
	}
	p, err := q.Plan(ctx)
	if err != nil || p == nil || p.execution == nil || p.execution.owner != q {
		return ReviewReceipt{}, ErrReviewReceipt
	}
	s := q.reviewSnapshot(p.execution)
	if s == nil {
		return ReviewReceipt{}, ErrReviewReceipt
	}
	size := len(s.operation) + len(s.target) + len(s.structure) + len(s.args) + len(s.config)
	if t.state.bytes.Add(int64(size)) > 64<<20 {
		return ReviewReceipt{}, ErrReviewBudget
	}
	return ReviewReceipt{token: t.state, owner: q, ownerContext: q.ctx, ownerExecutor: q.exec, sample: s}, nil
}

func reviewTree(n *predicate.Node, depth int) (any, bool) {
	if n == nil {
		return nil, true
	}
	if depth > planidentity.MaxDepth || !identityTree(n, depth) {
		return nil, false
	}
	children := []any{}
	for _, c := range n.Children {
		v, ok := reviewTree(c, depth+1)
		if !ok {
			return nil, false
		}
		children = append(children, v)
	}
	positions := []any{}
	for _, i := range n.Parameters {
		positions = append(positions, i)
	}
	// Values are compared only through ordered native execution arguments.
	return []any{n.Kind, n.Column, n.Operator, n.ValueColumn, n.Correspondence, positions, children}, true
}
func (q *Query) reviewSnapshot(e *plannedExecution) *reviewSnapshot {
	if e == nil || e.owner != q || e.inspection == nil {
		return nil
	}
	p := e.inspection
	if p.Operation != OperationSelect || len(p.Unverified) > 0 || len(e.sql) > planidentity.MaxBytes || len(e.args) > planidentity.MaxSlots || len(p.Tables) > 1024 || len(p.Columns) > 4096 || len(p.Joins) > 1024 {
		return nil
	}
	dialect := ""
	switch q.dialect.(type) {
	case driver.MySQLDialect:
		dialect = "mysql"
	case driver.PostgresDialect:
		dialect = "postgres"
	default:
		return nil
	}
	if _, ok := identitySQLShape(e.sql, dialect); !ok {
		return nil
	}
	where, ok := reviewTree(p.WhereTree, 0)
	if !ok {
		return nil
	}
	having, ok := reviewTree(p.HavingTree, 0)
	if !ok {
		return nil
	}
	tables := []any{}
	for _, t := range p.Tables {
		tables = append(tables, []any{t.Name, t.Alias})
	}
	columns := []any{}
	for _, c := range p.Columns {
		if c.Raw {
			return nil
		}
		columns = append(columns, []any{c.Table, c.Name, c.Distinct, c.Function, c.Count, c.Expression})
	}
	joins := []any{}
	for _, j := range p.Joins {
		tree, ok := reviewTree(j.OnTree, 0)
		if !ok || j.Subquery {
			return nil
		}
		joins = append(joins, []any{j.Type, j.Table, j.Alias, tree})
	}
	s := &reviewSnapshot{}
	var err error
	encode := func(dst *[]byte, v any) bool { *dst, err = planidentity.Canonical(v); return err == nil }
	if !encode(&s.operation, []any{string(p.Operation)}) || !encode(&s.target, []any{dialect, tables}) || !encode(&s.structure, []any{dialect, e.sql, columns, joins, where, having}) || !encode(&s.args, e.args) {
		return nil
	}
	// bindingSettings serializes only closed library-owned settings, not values.
	config, err := bindingSettings(q.settings)
	if err != nil {
		return nil
	}
	policy, err := identityFields(q.policy)
	if err != nil {
		return nil
	}
	required, err := identityFields(struct {
		Required                 []RequiredPredicate
		Approval                 *Approval
		Suppressions             []Suppression
		AccessReason, PrimaryKey string
		WriteKeys                *WriteKeyContext
	}{q.requiredPredicates, q.approval, q.suppressions, q.accessReason, q.primaryKey, q.writeKeys})
	if err != nil {
		return nil
	}
	if !encode(&s.config, []any{config, policy, required, q.withDeleted, q.onlyDeleted, q.settings.execution.input.TenantPresent, q.settings.execution.input.CurrentTenant}) {
		return nil
	}
	s.contextKnown = e.identity != nil && e.identityErr == nil
	return s
}
func (r ReviewReceipt) current(t ReviewToken) bool {
	if !t.valid() || r.token != t.state || r.owner == nil || r.sample == nil || !bindingSame(r.ownerContext, r.owner.ctx) || r.owner.builder == nil || !bindingSame(r.ownerExecutor, r.owner.exec) {
		return false
	}
	if r.owner.ctx != nil && r.owner.ctx.Err() != nil {
		return false
	}
	p, err := r.owner.Plan(t.state.ctx)
	if err != nil || p == nil {
		return false
	}
	s := r.owner.reviewSnapshot(p.execution)
	if s == nil {
		return false
	}
	a, b := r.sample, s
	return bytes.Equal(a.operation, b.operation) && bytes.Equal(a.target, b.target) && bytes.Equal(a.structure, b.structure) && bytes.Equal(a.args, b.args) && bytes.Equal(a.config, b.config)
}

// CompareReviewReceipts compares private samples. Context is matched only within
// the existing supported identity domain; neither result grants authorization.
func CompareReviewReceipts(t ReviewToken, a, b ReviewReceipt) analysis.Correspondence {
	c := analysis.Correspondence{}.Normalize()
	if !b.current(t) {
		c.Source = analysis.Mismatched
		return c
	}
	if a.sample == nil {
		return c
	}
	if !a.current(t) {
		c.Source = analysis.Mismatched
		return c
	}
	cmp := func(x, y []byte) analysis.Match {
		if bytes.Equal(x, y) {
			return analysis.Matched
		}
		return analysis.Mismatched
	}
	c.Operation = cmp(a.sample.operation, b.sample.operation)
	c.Target = cmp(a.sample.target, b.sample.target)
	c.Structure = cmp(a.sample.structure, b.sample.structure)
	c.Arguments = cmp(a.sample.args, b.sample.args)
	if !bytes.Equal(a.sample.config, b.sample.config) {
		c.Context = analysis.Mismatched
	} else if a.sample.contextKnown && b.sample.contextKnown {
		c.Context = analysis.Matched
	}
	return c
}

// CompareReviewOperation compares a known static operation with a planner sample.
// It does not disclose executable material or certify source execution.
func CompareReviewOperation(t ReviewToken, r ReviewReceipt, operation OperationType) analysis.Match {
	if !r.current(t) {
		return analysis.Unconfirmed
	}
	switch operation {
	case OperationSelect, OperationInsert, OperationUpdate, OperationDelete:
	default:
		return analysis.Unconfirmed
	}
	b, err := planidentity.Canonical([]any{string(operation)})
	if err != nil {
		return analysis.Unconfirmed
	}
	if bytes.Equal(b, r.sample.operation) {
		return analysis.Matched
	}
	return analysis.Mismatched
}
