package planidentity

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"strconv"
	"testing"
)

func fixture(t *testing.T) (Material, Key) {
	t.Helper()
	return Material{Dialect: "postgres", Target: "fixture-db", Operation: "select", SQL: `SELECT "id" FROM "items" WHERE "id" = $1`, Shape: `SELECT "id" FROM "items" WHERE "id" = $1`, Args: []any{json.Number("9007199254740993")}, Tenant: json.Number("7"), TenantPresent: true, Policy: map[string]any{"mode": "fixture"}, Schema: map[string]any{"table": "items"}, Config: map[string]any{"strict": true}}, Key{Secret: bytes.Repeat([]byte{0xA5}, 32), Scope: "fixture-only", Generation: "generation-1"}
}

func TestLanguageNeutralVectors(t *testing.T) {
	b, err := os.ReadFile("../../../tests/contracts/testdata/plan_contract_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Canonical []struct{ Type, Value, Canonical string }
		Identity  struct {
			KeyHex            string `json:"key_hex"`
			Scope, Generation string
			ShapeHex          string `json:"shape_hex"`
			ExecutionHex      string `json:"execution_hex"`
			Material          struct {
				Dialect, Target, Operation, SQL, Shape string
				Args                                   []any
				Tenant, Policy, Schema, Config         any
			}
		}
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err = d.Decode(&f); err != nil {
		t.Fatal(err)
	}
	for _, v := range f.Canonical {
		t.Run(v.Type+v.Value, func(t *testing.T) {
			var x any
			switch v.Type {
			case "json":
				d := json.NewDecoder(bytes.NewBufferString(v.Value))
				d.UseNumber()
				err = d.Decode(&x)
			case "i64":
				x, err = strconv.ParseInt(v.Value, 10, 64)
			case "i32":
				var n int64
				n, err = strconv.ParseInt(v.Value, 10, 32)
				x = int32(n)
			case "u64":
				x, err = strconv.ParseUint(v.Value, 10, 64)
			case "jsonint":
				x = json.Number(v.Value)
			case "str":
				x = v.Value
			case "bytes":
				x = []byte(v.Value)
			case "nilbytes":
				x = []byte(nil)
			case "null":
				x = nil
			case "bool":
				x, err = strconv.ParseBool(v.Value)
			default:
				t.Fatal("unknown vector type")
			}
			if err != nil {
				t.Fatal(err)
			}
			got, e := Canonical(x)
			if e != nil || string(got) != v.Canonical {
				t.Fatalf("%q %v", got, e)
			}
		})
	}
	v := f.Identity
	m := v.Material
	k, e := hex.DecodeString(v.KeyHex)
	if e != nil {
		t.Fatal(e)
	}
	ids, e := Compute(Material{Dialect: m.Dialect, Target: m.Target, Operation: m.Operation, SQL: m.SQL, Shape: m.Shape, Args: m.Args, Tenant: m.Tenant, TenantPresent: true, Policy: m.Policy, Schema: m.Schema, Config: m.Config}, Key{Secret: k, Scope: v.Scope, Generation: v.Generation})
	if e != nil || hex.EncodeToString(ids.Shape[:]) != v.ShapeHex || hex.EncodeToString(ids.Execution[:]) != v.ExecutionHex {
		t.Fatalf("vector mismatch: %x %x %v", ids.Shape, ids.Execution, e)
	}
}

func TestIdentityDomainsAndDetachment(t *testing.T) {
	m, k := fixture(t)
	first, e := Compute(m, k)
	if e != nil {
		t.Fatal(e)
	}
	mutations := map[string]func(*Material){
		"sql": func(m *Material) { m.SQL += " LIMIT 1" }, "operation": func(m *Material) { m.Operation = "update" }, "dialect": func(m *Material) { m.Dialect = "mysql" }, "target": func(m *Material) { m.Target = "fixture-other" }, "tenant": func(m *Material) { m.Tenant = json.Number("8") }, "args": func(m *Material) { m.Args = []any{json.Number("9007199254740994")} }, "type": func(m *Material) { m.Args = []any{int64(9007199254740993)} }, "policy": func(m *Material) { m.Policy = map[string]any{"mode": "other"} }, "schema": func(m *Material) { m.Schema = map[string]any{"table": "other"} }, "config": func(m *Material) { m.Config = map[string]any{"strict": false} }, "shape": func(m *Material) { m.Shape = "other" },
	}
	for name, change := range mutations {
		t.Run(name, func(t *testing.T) {
			n := m
			change(&n)
			got, e := Compute(n, k)
			if e != nil || got.Execution == first.Execution {
				t.Fatal("domain collision", e)
			}
		})
	}
	for _, change := range []func(*Key){func(k *Key) { k.Secret = bytes.Repeat([]byte{0xB6}, 32) }, func(k *Key) { k.Scope = "other" }, func(k *Key) { k.Generation = "2" }} {
		n := k
		change(&n)
		got, e := Compute(m, n)
		if e != nil || got.Execution == first.Execution || got.Shape != first.Shape {
			t.Fatal("key domain", e)
		}
	}
	snap, e := Seal(m)
	if e != nil {
		t.Fatal(e)
	}
	m.Args[0] = "mutated"
	m.Policy.(map[string]any)["mode"] = "mutated"
	got, e := snap.Identify(k)
	if e != nil || got != first {
		t.Fatal("snapshot retained mutable input", e)
	}
	a, e := Canonical(map[string]any{"z": int64(1), "a": []any{true, "x"}})
	if e != nil {
		t.Fatal(e)
	}
	b, e := Canonical(map[string]any{"a": []any{true, "x"}, "z": int64(1)})
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("map order", e)
	}
	a, _ = Canonical([]any{int64(1), int64(2)})
	b, _ = Canonical([]any{int64(2), int64(1)})
	if bytes.Equal(a, b) {
		t.Fatal("ordered values lost")
	}
}

type forbiddenMethods struct{}

func (forbiddenMethods) String() string               { panic("Stringer invoked") }
func (forbiddenMethods) MarshalJSON() ([]byte, error) { panic("marshaler invoked") }

func TestUnavailableAndTypedDomain(t *testing.T) {
	m, k := fixture(t)
	for _, bad := range []Key{{}, {Secret: k.Secret}, {Secret: k.Secret, Scope: k.Scope}, {Secret: k.Secret, Generation: k.Generation}, {Secret: make([]byte, 31), Scope: k.Scope, Generation: k.Generation}} {
		if _, e := Compute(m, bad); !errors.Is(e, ErrUnavailable) {
			t.Fatal(e)
		}
	}
	for _, change := range []func(*Material){func(m *Material) { m.Target = "" }, func(m *Material) { m.TenantPresent = false }, func(m *Material) { m.Policy = nil }, func(m *Material) { m.Schema = nil }, func(m *Material) { m.Config = nil }, func(m *Material) { m.Shape = nil }, func(m *Material) { m.Dialect = "unknown" }} {
		n := m
		change(&n)
		if _, e := Compute(n, k); !errors.Is(e, ErrUnavailable) {
			t.Fatal(e)
		}
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, v := range []any{forbiddenMethods{}, math.NaN(), math.Inf(1), json.Number("1.5"), json.Number("1e2"), json.Number("01"), json.Number("1 "), json.Number("-"), "\xff", cycle, make([]any, MaxSlots+1), bytes.Repeat([]byte{0}, MaxBytes)} {
		if _, e := Canonical(v); !errors.Is(e, ErrUnavailable) {
			t.Fatalf("unsupported %T: %v", v, e)
		}
	}
	distinct := []any{nil, []byte(nil), []byte{}, "", false, int16(1), int32(1), int64(1), uint64(1), json.Number("1"), float32(1), float64(1), float64(0), math.Copysign(0, -1), []any(nil), []any{}, map[string]any(nil), map[string]any{}}
	seen := map[string]bool{}
	for _, v := range distinct {
		b, e := Canonical(v)
		if e != nil || seen[string(b)] {
			t.Fatalf("typed collision %T: %q %v", v, b, e)
		}
		seen[string(b)] = true
	}
}

func TestLanguageNeutralIdentityRelationships(t *testing.T) {
	b, e := os.ReadFile("../../../tests/contracts/testdata/plan_contract_v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Cases []struct {
			Name, Field   string
			Value         any
			Available     bool
			SameShape     bool `json:"same_shape"`
			SameExecution bool `json:"same_execution"`
		} `json:"identity_cases"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if e = d.Decode(&f); e != nil {
		t.Fatal(e)
	}
	m, k := fixture(t)
	first, e := Compute(m, k)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			n, key := m, k
			switch c.Field {
			case "none", "diagnostic": // Explicitly excluded from semantic material; expiry is a separate gate.
			case "target":
				n.Target = c.Value.(string)
			case "tenant":
				n.Tenant = c.Value
			case "args":
				n.Args = c.Value.([]any)
			case "sql":
				n.SQL = c.Value.(string)
			case "shape":
				n.Shape = c.Value
			case "policy":
				n.Policy = c.Value
			case "schema":
				n.Schema = c.Value
			case "config":
				n.Config = c.Value
			case "scope":
				key.Scope = c.Value.(string)
			case "generation":
				key.Generation = c.Value.(string)
			default:
				t.Fatal("unknown fixture field")
			}
			got, e := Compute(n, key)
			if !c.Available {
				if !errors.Is(e, ErrUnavailable) {
					t.Fatal(e)
				}
				return
			}
			if e != nil || (got.Shape == first.Shape) != c.SameShape || (got.Execution == first.Execution) != c.SameExecution {
				t.Fatal("relationship mismatch", e)
			}
		})
	}
}
