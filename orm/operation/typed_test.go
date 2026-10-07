package operation

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/query"
)

func typedFixture(sqlType, dialect string) (OperationSpec, Options) {
	n := int64(5)
	return OperationSpec{Operation: "select", Model: "items", Select: []string{"v"}, Filters: []FilterSpec{{Field: "v", Op: "=", Value: 1}}, Limit: &n}, Options{Manifest: &manifest.Manifest{Version: "1", Dialect: dialect, Tables: []manifest.Table{{Name: "items", Columns: []manifest.Column{{Name: "v", Type: sqlType, TypeSource: "sql", NullableKnown: true}}}}}}
}
func strictFixture(t *testing.T, opts Options, tenant, automatic bool) Options {
	t.Helper()
	table := opts.Manifest.Tables[0]
	columns := []query.WriteKeyColumn{}
	for _, c := range table.Columns {
		typ := c.Type
		if opts.Manifest.Dialect == "mysql" {
			typ = strings.ToUpper(typ)
		}
		columns = append(columns, query.WriteKeyColumn{Name: c.Name, DBType: typ, Bits: 64})
	}
	schema, err := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: opts.Manifest.Dialect, Tables: []query.ApplicationTable{{Table: table.Name, PlainTable: true, Columns: columns, CompleteUniqueConstraints: true}}})
	if err != nil {
		t.Fatal(err)
	}
	policies := query.PolicySet{}
	execution := query.ExecutionContext{}
	if tenant {
		policies, err = query.NewPolicySet(query.TablePolicy{Table: table.Name, TenantColumn: "v"})
		if err != nil {
			t.Fatal(err)
		}
		execution, err = query.NewApplicationTenantContext(query.ExecutionContextInput{CurrentTenant: int64(7), TenantPresent: true})
		if err != nil {
			t.Fatal(err)
		}
		opts.Manifest.Tables[0].Columns[0].TenantScope = true
	}
	settings := query.NewSettings(policies, query.RiskConfig{}, execution).WithTenantPolicy("fixture", schema, automatic)
	opts.Settings = &settings
	return opts
}
func TestTypedValuesExactDomain(t *testing.T) {
	cases := []struct {
		typ, dialect string
		value        any
		valid        bool
	}{
		{"bigint", "postgres", json.Number("9007199254740993"), true}, {"bigint", "mysql", json.Number("9223372036854775807"), true},
		{"bigint", "mysql", json.Number("9223372036854775808"), false}, {"bigint", "postgres", json.Number("-9223372036854775808"), true},
		{"bigint unsigned", "mysql", uint64(math.MaxUint64), true}, {"bigint unsigned", "mysql", -1, false},
		{"int8", "postgres", int64(9223372036854775807), true}, {"tinyint", "mysql", 128, false}, {"tinyint", "mysql", -128, true},
		{"integer", "postgres", json.Number("1.0"), false}, {"integer", "mysql", json.Number("1e3"), false},
		{"integer", "mysql", float64(1), false}, {"integer", "mysql", "1", false},
		{"numeric(5,2)", "postgres", json.Number("999.99"), true}, {"decimal(5,2)", "mysql", json.Number("999.9900"), true},
		{"decimal(5,2)", "mysql", json.Number("1000"), false}, {"decimal(5,2)", "mysql", json.Number("1.001"), false},
		{"decimal(5,2)", "mysql", json.Number("12e-2"), true}, {"decimal(5,2)", "mysql", json.Number("1e999999999999999999999"), false},
		{"decimal(5,2)", "mysql", json.Number("0e999999999999999999999"), true}, {"decimal(5,2)", "mysql", 1.25, false},
		{"varchar(2)", "postgres", "猫😀", true}, {"varchar(2)", "mysql", "猫😀x", false},
		{"boolean", "postgres", false, true}, {"boolean", "mysql", 0, false}, {"tinyint(1)", "mysql", true, true},
		{"uuid", "postgres", "aBcDefAB-1234-5678-90ab-123456789abc", true}, {"uuid", "postgres", "abcdef", false},
		{"date", "mysql", "2024-02-29", true}, {"date", "postgres", "2025-02-29", false}, {"date", "mysql", "0000-00-00", false},
		{"time(3)", "mysql", "23:59:59.123", true}, {"time(3)", "mysql", "24:00:00", false}, {"time(3)", "postgres", "12:00:00.1234", false},
		{"timestamp(3) with time zone", "postgres", "2026-01-01T12:00:00.123Z", true},
		{"timestamp(3) with time zone", "postgres", "2026-01-01T12:00:00+24:00", false},
		{"timestamp(3) without time zone", "postgres", "2026-01-01T12:00:00.123Z", false},
		{"datetime(6)", "mysql", "2026-01-01T12:00:00.123456", true},
	}
	for i, tc := range cases {
		t.Run(strings.Join([]string{tc.dialect, tc.typ}, "/")+"/"+string(rune('A'+i)), func(t *testing.T) {
			spec, opts := typedFixture(tc.typ, tc.dialect)
			spec.Filters[0].Value = tc.value
			p, e := Compile(t.Context(), spec, opts)
			if (e == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, e)
			}
			if e == nil && (len(p.Params) != 1 || !reflect.DeepEqual(p.Params[0], tc.value)) {
				t.Fatal("argument type or lexeme changed")
			}
		})
	}
}
func TestTypedUnknownAndStrict(t *testing.T) {
	for _, source := range []string{"", "go", "sql"} {
		spec, opts := typedFixture("custom_type", "postgres")
		opts.Manifest.Tables[0].Columns[0].TypeSource = source
		p, e := Compile(t.Context(), spec, opts)
		if e != nil || !p.Blocked || p.AnalysisPrecision != query.AnalysisPartial {
			t.Fatal("unknown became executable", e)
		}
		opts = strictFixture(t, opts, false, false)
		if _, e = Compile(t.Context(), spec, opts); !errors.Is(e, ErrTypeUnverified) {
			t.Fatal(e)
		}
	}
	spec, opts := typedFixture("bigint", "postgres")
	opts = strictFixture(t, opts, false, false)
	if _, e := Compile(t.Context(), spec, opts); e != nil {
		t.Fatal(e)
	}
	spec, opts = typedFixture("decimal(5,2)", "postgres")
	spec.Filters[0].Value = json.Number("1.23")
	opts = strictFixture(t, opts, false, false)
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrTypeUnverified) {
		t.Fatal("decimal escaped canonical domain", e)
	}
	spec, opts = typedFixture("integer[]", "postgres")
	spec.Filters[0].Value = []int{1}
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrArrayBinding) {
		t.Fatal(e)
	}
}
func TestTypedPresenceArityAndBounds(t *testing.T) {
	for _, payload := range []string{
		`{"field":"v","op":"=","value":null,"value_ref":""}`,
		`{"field":"v","op":"=","value":null}`, `{"field":"v","op":"="}`, `{"field":"v","op":"is_null","value":null}`,
		`{"field":"v","op":"=","value_ref":""}`, `{"field":"v","op":"in","value":[]}`, `{"field":"v","op":"in","value":[1,null]}`,
	} {
		spec, opts := typedFixture("bigint", "mysql")
		if json.Unmarshal([]byte(payload), &spec.Filters[0]) != nil {
			continue
		}
		if _, e := Compile(t.Context(), spec, opts); e == nil {
			t.Fatal(payload)
		}
	}
	for _, v := range []any{nil, 0, false, ""} {
		f := FilterSpec{Field: "v", Op: "=", Value: v, ValuePresent: true}
		b, e := json.Marshal(f)
		if e != nil {
			t.Fatal(e)
		}
		var out FilterSpec
		if json.Unmarshal(b, &out) != nil || !out.hasValue() {
			t.Fatal("presence lost")
		}
	}
	f := FilterSpec{Field: "v", Op: "is_null"}
	b, _ := json.Marshal(f)
	if strings.Contains(string(b), "value") {
		t.Fatal("missing became null")
	}
	for _, n := range []int64{-1, 0, 1, 10000, 10001} {
		spec, opts := typedFixture("bigint", "mysql")
		spec.Limit = &n
		p, e := Compile(t.Context(), spec, opts)
		if (e == nil) != (n >= 0 && n <= 10000) {
			t.Fatal(n, e)
		}
		if e == nil && (p.Limit == nil || *p.Limit != n) {
			t.Fatal("limit lost")
		}
	}
	for _, n := range []int{0, 1, 999, 1000, 1001} {
		spec, opts := typedFixture("bigint", "postgres")
		spec.Filters[0].Op = "in"
		spec.Filters[0].Value = make([]int, n)
		_, e := Compile(t.Context(), spec, opts)
		if (e == nil) != (n >= 1 && n <= 1000) {
			t.Fatal(n, e)
		}
	}
	for _, s := range []string{`{"operation":"select","Operation":"select"}`, `{"filters":[{"field":"v","Field":"v"}]}`, `{"filters":[{"field":"v","value_present":true}]}`, `{"order_by":[{"field":"v","raw":"x"}]}`, `{"limit":null}`, `{} {}`} {
		var spec OperationSpec
		if json.Unmarshal([]byte(s), &spec) == nil {
			t.Fatal(s)
		}
	}
}
func TestTypedTenantTrust(t *testing.T) {
	for _, auto := range []bool{false, true} {
		spec, opts := typedFixture("bigint", "postgres")
		opts = strictFixture(t, opts, true, auto)
		spec.Filters[0] = FilterSpec{Field: "v", Op: "eq", ValueRef: "current_tenant"}
		p, e := Compile(t.Context(), spec, opts)
		if e != nil || p.Blocked {
			t.Fatal("trusted binding failed", e)
		}
		opts.Values = map[string]any{"current_tenant": int64(7)}
		if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrReservedBinding) {
			t.Fatal(e)
		}
		opts.Values = nil
		spec.Filters[0] = FilterSpec{Field: "v", Op: "=", Value: int64(7)}
		if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrReservedBinding) {
			t.Fatal("literal gained trust", e)
		}
		spec.Filters = nil
		_, e = Compile(t.Context(), spec, opts)
		if (e == nil) != auto {
			t.Fatal("automatic binding", auto, e)
		}
		s := opts.Settings.WithExecutionContext(query.ExecutionContext{})
		opts.Settings = &s
		if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrReservedBinding) {
			t.Fatal(e)
		}
	}
}
func TestTypedNullEnumAndOperator(t *testing.T) {
	spec, opts := typedFixture("varchar(5)", "mysql")
	opts.Manifest.Tables[0].Columns[0].EnumValues = []string{"a"}
	spec.Filters[0].Value = "A"
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrTypeMismatch) {
		t.Fatal(e)
	}
	spec.Filters[0].Value = "a"
	if _, e := Compile(t.Context(), spec, opts); e != nil {
		t.Fatal(e)
	}
	spec, opts = typedFixture("bigint", "postgres")
	spec.Filters[0].Op = "like"
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrTypeMismatch) {
		t.Fatal(e)
	}
	spec.Filters[0] = FilterSpec{Field: "v", Op: "is_null"}
	opts.Manifest.Tables[0].Columns[0].NullableKnown = false
	opts = strictFixture(t, opts, false, false)
	if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrTypeUnverified) {
		t.Fatal(e)
	}
}

func TestOperationManifestVersionAndSchemaValueContract(t *testing.T) {
	for _, version := range []string{"", "0", "2"} {
		spec, opts := typedFixture("bigint", "postgres")
		opts.Manifest.Version = version
		if _, e := Compile(t.Context(), spec, opts); !errors.Is(e, ErrInvalidManifest) {
			t.Fatal("unknown manifest version accepted", e)
		}
		if _, e := Validate(spec, opts); !errors.Is(e, ErrInvalidManifest) {
			t.Fatal("validator version mismatch", e)
		}
	}
	b, e := JSONSchema()
	if e != nil {
		t.Fatal(e)
	}
	var schema map[string]any
	if json.Unmarshal(b, &schema) != nil {
		t.Fatal("invalid schema JSON")
	}
	props := schema["properties"].(map[string]any)
	filter := props["filters"].(map[string]any)["items"].(map[string]any)
	rules := filter["allOf"].([]any)
	scalar := rules[0].(map[string]any)["then"].(map[string]any)["properties"].(map[string]any)["value"].(map[string]any)["type"]
	if !reflect.DeepEqual(scalar, []any{"string", "number", "boolean"}) {
		t.Fatal("schema permits nonscalar comparison values")
	}
	in := rules[2].(map[string]any)["then"].(map[string]any)["properties"].(map[string]any)["value"].(map[string]any)
	if in["minItems"] != float64(1) || in["maxItems"] != float64(1000) || !reflect.DeepEqual(in["items"].(map[string]any)["type"], scalar) {
		t.Fatal("IN shape differs from common contract")
	}
}
