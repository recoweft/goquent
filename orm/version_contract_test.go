package orm_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/orm/review"
)

func TestEnvelopeVersionFixtures(t *testing.T) {
	b, e := os.ReadFile("../tests/contracts/testdata/plan_contract_v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Versions []struct {
			Name, JSON string
			Accepted   bool
			Version    int
		}
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	factories := map[string]func() any{"query": func() any { return new(query.QueryPlan) }, "review": func() any { return new(review.ReviewReport) }, "policy": func() any { return new(query.TablePolicy) }, "operation": func() any { return new(operation.OperationSpec) }}
	for name, newValue := range factories {
		for _, c := range f.Versions {
			t.Run(name+"/"+c.Name, func(t *testing.T) {
				v := newValue()
				e := json.Unmarshal([]byte(c.JSON), v)
				if !c.Accepted {
					if !errors.Is(e, query.ErrJSONVersion) {
						t.Fatalf("expected version error: %v", e)
					}
					return
				}
				if e != nil {
					t.Fatal(e)
				}
				if got := reflect.ValueOf(v).Elem().FieldByName("Version").Int(); got != int64(c.Version) {
					t.Fatalf("version %d", got)
				}
				out, e := json.Marshal(v)
				if e != nil {
					t.Fatal(e)
				}
				var raw map[string]json.RawMessage
				json.Unmarshal(out, &raw)
				if string(raw["version"]) != "1" {
					t.Fatalf("migration: %s", out)
				}
				if e = json.Unmarshal(out, v); e != nil {
					t.Fatal(e)
				}
			})
		}
		for _, version := range []int{-1, 2} {
			v := newValue()
			reflect.ValueOf(v).Elem().FieldByName("Version").SetInt(int64(version))
			if _, e := json.Marshal(v); !errors.Is(e, query.ErrJSONVersion) {
				t.Fatalf("%s unknown writer: %v", name, e)
			}
		}
	}
	if b, e := json.Marshal((*query.QueryPlan)(nil)); e != nil || string(b) != "null" {
		t.Fatal(string(b), e)
	}
}

func TestUnknownGoVersionsAtInputBoundaries(t *testing.T) {
	p := query.TablePolicy{Version: 2, Table: "items"}
	if _, e := query.NewPolicySet(p); !errors.Is(e, query.ErrJSONVersion) {
		t.Fatal(e)
	}
	if e := query.RegisterTablePolicy(p); !errors.Is(e, query.ErrJSONVersion) {
		t.Fatal(e)
	}
	s := operation.OperationSpec{Version: 2}
	if _, e := operation.Validate(s, operation.Options{}); !errors.Is(e, operation.ErrJSONVersion) {
		t.Fatal(e)
	}
	if _, e := operation.Compile(t.Context(), s, operation.Options{}); !errors.Is(e, operation.ErrJSONVersion) {
		t.Fatal(e)
	}
}

func TestDiagnosticNumbersRetainLexemes(t *testing.T) {
	var p query.QueryPlan
	if e := json.Unmarshal([]byte(`{"params":[9007199254740993,1.25,1e10],"metadata":{"n":9223372036854775807}}`), &p); e != nil {
		t.Fatal(e)
	}
	for i, want := range []string{"9007199254740993", "1.25", "1e10"} {
		if p.Params[i] != json.Number(want) {
			t.Fatalf("lost number: %#v", p.Params)
		}
	}
	if p.Metadata["n"] != json.Number("9223372036854775807") {
		t.Fatal(p.Metadata)
	}
}
