package planidentity

import (
	"crypto/hmac"
	"crypto/sha256"
)

// Key is explicitly supplied per computation. No key is generated or persisted.
// Scope and Generation are mandatory, independently bound rotation inputs.
type Key struct {
	Secret            []byte
	Scope, Generation string
}

// Material must originate at a private planner boundary. Possession of this
// internal representation or a matching digest does not validate execution.
type Material struct {
	Dialect, Target, Operation, SQL string
	Shape                           any
	Args                            []any
	Tenant                          any
	TenantPresent                   bool
	Policy, Schema, Config          any
}

type IDs struct{ Shape, Execution [32]byte }

// Snapshot owns canonical bytes; it never retains caller slices, maps or callbacks.
// Fields are private and no serialization or public retrieval API is provided.
type Snapshot struct{ shape, execution, policy, schema, config []byte }

func Seal(m Material) (*Snapshot, error) {
	if m.Target == "" || (m.Dialect != "mysql" && m.Dialect != "postgres") || m.Operation == "" || m.SQL == "" || m.Shape == nil || m.Policy == nil || m.Schema == nil || m.Config == nil || !m.TenantPresent {
		return nil, ErrUnavailable
	}
	s := new(Snapshot)
	var err error
	s.shape, err = Canonical([]any{"goquent/shape", int64(Version), m.Dialect, m.Operation, m.Shape})
	if err != nil {
		return nil, err
	}
	s.execution, err = Canonical([]any{m.Dialect, m.Target, m.Operation, m.SQL, s.shape, m.Args, m.TenantPresent, m.Tenant})
	if err != nil {
		return nil, err
	}
	for _, c := range []struct {
		domain string
		value  any
		dst    *[]byte
	}{{"goquent/policy", m.Policy, &s.policy}, {"goquent/schema", m.Schema, &s.schema}, {"goquent/config", m.Config, &s.config}} {
		*c.dst, err = Canonical([]any{c.domain, int64(Version), c.value})
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Snapshot) Identify(k Key) (IDs, error) {
	var out IDs
	if s == nil || len(s.shape) == 0 || len(k.Secret) < 32 || k.Scope == "" || k.Generation == "" {
		return out, ErrUnavailable
	}
	key := append([]byte(nil), k.Secret...)
	digest := func(b []byte) []byte { h := hmac.New(sha256.New, key); h.Write(b); return h.Sum(nil) }
	// Context fingerprints are private and keyed, in separate domains. Never
	// publish a plain digest of target, tenant, policy literals or SQL.
	b, err := Canonical([]any{"goquent/execution", int64(Version), k.Scope, k.Generation, s.execution, digest(s.policy), digest(s.schema), digest(s.config)})
	if err != nil {
		return out, err
	}
	out.Shape = sha256.Sum256(s.shape)
	copy(out.Execution[:], digest(b))
	return out, nil
}

func Compute(m Material, k Key) (IDs, error) {
	s, e := Seal(m)
	if e != nil {
		return IDs{}, e
	}
	return s.Identify(k)
}
