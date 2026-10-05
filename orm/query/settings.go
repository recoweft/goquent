package query

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/internal/valuecopy"
)

// PolicySet is an immutable collection of table policies. Its zero value is empty.
// Build a new set to change policies; input and returned slices are detached.
type PolicySet struct{ byTable map[string]TablePolicy }

func NewPolicySet(policies ...TablePolicy) (PolicySet, error) {
	set := PolicySet{byTable: make(map[string]TablePolicy, len(policies))}
	for _, p := range policies {
		if strings.TrimSpace(p.Table) == "" {
			return PolicySet{}, fmt.Errorf("goquent: policy table is required")
		}
		p.Table = normalizeTableName(p.Table)
		set.byTable[p.Table] = cloneTablePolicy(normalizeTablePolicy(p))
	}
	return set, nil
}

// PolicyForTable returns a detached policy using the compatibility table normalization.
func (s PolicySet) PolicyForTable(table string) (TablePolicy, bool) {
	p, ok := s.byTable[normalizeTableName(table)]
	return cloneTablePolicy(p), ok
}

// Policies returns detached policies in stable table order.
func (s PolicySet) Policies() []TablePolicy {
	out := make([]TablePolicy, 0, len(s.byTable))
	for _, p := range s.byTable {
		out = append(out, cloneTablePolicy(p))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Table < out[j].Table })
	return out
}

// SnapshotPolicyRegistry copies the legacy registry at this explicit boundary.
func SnapshotPolicyRegistry() PolicySet {
	set, _ := NewPolicySet(RegisteredTablePolicies()...)
	return set
}

var (
	ErrUnsupportedExecutionContext = errors.New("goquent: execution context contains an unsupported, recursive, deep or oversized value")
	ErrUnsupportedRiskEngine       = errors.New("goquent: custom global risk engine cannot be snapshotted; supply an explicit RiskConfig")
)

// ExecutionContextInput is application-supplied provenance and current tenant data.
// Source is an application label, not verified identity. TenantPresent preserves
// absence separately from an explicitly supplied nil/empty tenant. None of these
// fields assert authentication, authorization or a verified tenant binding.
type ExecutionContextInput struct {
	Source        string
	CurrentTenant any
	TenantPresent bool
}

// ExecutionContext holds an immutable application-supplied snapshot. The zero
// value is missing context. ORM verification remains unconfirmed in PR1.
type ExecutionContext struct {
	input       ExecutionContextInput
	application bool
}

// NewExecutionContext detaches supported values using the predicate ownership
// copier. Unsupported values are rejected without calling user methods or
// retaining references. See docs/db-settings.md for supported types and budgets.
func NewExecutionContext(input ExecutionContextInput) (ExecutionContext, error) {
	value, ok := valuecopy.Copy(input.CurrentTenant)
	if !ok {
		return ExecutionContext{}, ErrUnsupportedExecutionContext
	}
	if !input.TenantPresent && input.CurrentTenant != nil {
		return ExecutionContext{}, fmt.Errorf("goquent: tenant value supplied without TenantPresent")
	}
	input.CurrentTenant = value
	return ExecutionContext{input: input}, nil
}

// Input returns a detached copy; it is data, never an authorization verdict.
func (c ExecutionContext) Input() ExecutionContextInput {
	out := c.input
	out.CurrentTenant, _ = valuecopy.Copy(out.CurrentTenant)
	return out
}

// Settings is an immutable policy, risk and execution-context snapshot. The zero
// value uses empty policies, built-in risk defaults and missing context. Copies
// safely share only privately owned immutable data.
type Settings struct {
	strict     bool
	autoTenant bool
	database   string
	schema     ApplicationSchema
	policies   PolicySet
	risk       RiskConfig
	execution  ExecutionContext
	err        error
}

// NewSettings builds a detached snapshot without consulting globals.
func NewSettings(policies PolicySet, risk RiskConfig, execution ExecutionContext) Settings {
	return Settings{policies: policies, risk: cloneRiskConfig(risk), execution: execution}
}

// SnapshotDefaultSettings imports legacy defaults once. Global engine assignment
// must finish before concurrent use; the historical exported variable cannot
// synchronize caller writes. Arbitrary custom engines cannot be copied safely.
// Such a snapshot records ErrUnsupportedRiskEngine and blocks inspected paths
// until explicitly replaced with WithRiskConfig. No custom method is invoked.
func SnapshotDefaultSettings() Settings {
	s := Settings{policies: SnapshotPolicyRegistry()}
	if engine, ok := DefaultRiskEngine.(defaultRiskEngine); ok {
		s.risk = cloneRiskConfig(engine.config)
	} else {
		s.err = ErrUnsupportedRiskEngine
	}
	return s
}

func (s Settings) PolicySet() PolicySet               { return s.policies }
func (s Settings) RiskConfig() RiskConfig             { return cloneRiskConfig(s.risk) }
func (s Settings) ExecutionContext() ExecutionContext { return s.execution }
func (s Settings) Err() error                         { return s.err }
func (s Settings) WithPolicySet(p PolicySet) Settings { s.policies = p; return s }
func (s Settings) WithRiskConfig(r RiskConfig) Settings {
	s.risk = cloneRiskConfig(r)
	s.err = nil
	return s
}
func (s Settings) WithExecutionContext(c ExecutionContext) Settings { s.execution = c; return s }

func cloneRiskConfig(c RiskConfig) RiskConfig {
	out := RiskConfig{Environment: c.Environment}
	if c.Rules != nil {
		out.Rules = make(map[string]RiskRuleConfig, len(c.Rules))
		for k, r := range c.Rules {
			if r.Enabled != nil {
				v := *r.Enabled
				r.Enabled = &v
			}
			if r.Severity != nil {
				v := *r.Severity
				r.Severity = &v
			}
			if r.Suppressible != nil {
				v := *r.Suppressible
				r.Suppressible = &v
			}
			if r.RequiresReason != nil {
				v := *r.RequiresReason
				r.RequiresReason = &v
			}
			out.Rules[k] = r
		}
	}
	return out
}
