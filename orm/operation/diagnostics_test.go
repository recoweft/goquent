package operation

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/internal/publicview"
	"github.com/recoweft/goquent/orm/query"
)

const diagnosticCanary = "fictional_gq07_diagnostic_secret"

type diagnosticOpaque struct{}

func (diagnosticOpaque) String() string               { panic("String called") }
func (diagnosticOpaque) Error() string                { panic("Error called") }
func (diagnosticOpaque) MarshalJSON() ([]byte, error) { panic("Marshal called") }
func (diagnosticOpaque) Value() (driver.Value, error) { panic("Value called") }
func (diagnosticOpaque) Is(error) bool                { panic("Is called") }
func (diagnosticOpaque) As(any) bool                  { panic("As called") }
func (diagnosticOpaque) Unwrap() error                { panic("Unwrap called") }

func TestDiagnosticsInternalHistoryAndPositions(t *testing.T) {
	spec, opts := typedFixture("smallint", "postgres")
	spec.Filters[0].Op = "in"
	spec.Filters[0].Value = []any{1, 40000}
	_, v, e := CompileWithDiagnostics(t.Context(), spec, opts)
	if !errors.Is(e, ErrTypeMismatch) {
		t.Fatal("sentinel changed")
	}
	d := v.Diagnostics[0]
	if d.Code != "OPERATION_TYPE_MISMATCH" || d.Location.Section != "filters" || d.Location.Index != 0 || !d.Location.IndexKnown || d.Location.Element != 1 || !d.Location.ElementKnown || d.Location.Origin != "go" {
		t.Fatal("wrong logical refusal")
	}
	failure, ok := e.(*validationFailure)
	if !ok || len(failure.diagnostics) == 0 {
		t.Fatal("missing private history")
	}
	last := failure.diagnostics[len(failure.diagnostics)-1]
	if last.Table != "items" || last.Field != "v" || last.Declaration != "smallint" || last.Bits != 16 || last.Evidence != "supplied_declaration_not_live" || last.Missing == "" || last.Action != "supply_value_matching_declaration" {
		t.Fatal("missing private evidence")
	}
	for _, record := range failure.diagnostics {
		if record.Code == "OPERATION_CHECK_BINDING" {
			t.Fatal("unreached binding claimed checked")
		}
	}
	raw, _ := json.Marshal(spec)
	var decoded OperationSpec
	if json.Unmarshal(raw, &decoded) != nil {
		t.Fatal("decode")
	}
	_, j, _ := CompileWithDiagnostics(t.Context(), decoded, opts)
	if j.Diagnostics[0].Location.Origin != "json" {
		t.Fatal("origin lost")
	}
	// Select and order locations are list positions, not filter positions.
	for _, section := range []string{"select", "order_by"} {
		s, o := typedFixture("bigint", "postgres")
		if section == "select" {
			s.Select = append(s.Select, "missing")
		} else {
			s.OrderBy = []OrderSpec{{Field: "v"}, {Field: "missing"}}
		}
		_, view, _ := CompileWithDiagnostics(t.Context(), s, o)
		loc := view.Diagnostics[0].Location
		if loc.Section != section || loc.Index != 1 || !loc.IndexKnown {
			t.Fatal("wrong target location")
		}
	}
}

func TestDiagnosticsUnknownStrictAndRefusalAfterTruncation(t *testing.T) {
	for _, strict := range []bool{false, true} {
		s, o := typedFixture("custom_type", "postgres")
		if strict {
			o = strictFixture(t, o, false, false)
		}
		p, v, e := CompileWithDiagnostics(t.Context(), s, o)
		if strict {
			if !errors.Is(e, ErrTypeUnverified) || p != nil || v.Outcome != "rejected" {
				t.Fatal("strict refusal weakened")
			}
		} else if e != nil || p == nil || !p.Blocked || v.Coverage != "partial" {
			t.Fatal("unknown marked verified")
		}
	}
	s, o := typedFixture("bigint", "postgres")
	s.Select = make([]string, 200)
	for i := range s.Select {
		s.Select[i] = "v"
	}
	s.Filters[0].Value = "wrong"
	_, v, e := CompileWithDiagnostics(t.Context(), s, o)
	if e == nil || !v.Truncated || v.DiagnosticCount <= MaxDiagnostics || len(v.Diagnostics) != MaxDiagnostics || v.Diagnostics[0].Code != "OPERATION_TYPE_MISMATCH" {
		t.Fatal("tail refusal lost")
	}
}

func TestDiagnosticsNoOpaqueEvaluationOrDisclosure(t *testing.T) {
	cycle := map[string]any{}
	cycle["self"] = cycle
	for _, value := range []any{diagnosticOpaque{}, cycle, strings.Repeat(diagnosticCanary, 100000)} {
		s, o := typedFixture("text", "postgres")
		s.Filters[0].Value = value
		_, v, e := CompileWithDiagnostics(t.Context(), s, o)
		if !errors.Is(e, ErrInputLimit) {
			t.Fatal("opaque or excessive accepted")
		}
		for _, b := range []string{v.String(), fmt.Sprintf("%#v", v), fmt.Sprintf("%+v", &v), fmt.Sprintf("%#v", e)} {
			if strings.Contains(b, diagnosticCanary) {
				t.Fatal("canary disclosure")
			}
		}
	}
	s, o := typedFixture("text", "postgres")
	s.Filters[0].Value = diagnosticCanary
	s.AccessReason = diagnosticCanary
	p, v, e := CompileWithDiagnostics(t.Context(), s, o)
	if e != nil {
		t.Fatal(e)
	}
	if p.Params[0] != diagnosticCanary {
		t.Fatal("argument was redacted")
	}
	originalSQL := p.SQL
	originalArgs := append([]any(nil), p.Params...)
	v.Outcome = diagnosticCanary
	v.Coverage = diagnosticCanary
	v.Diagnostics[0].Code = diagnosticCanary
	v.Diagnostics[0].Message = diagnosticCanary
	v.Diagnostics[0].Status = diagnosticCanary
	v.Diagnostics[0].Location.Source = diagnosticCanary
	v.Diagnostics[0].Location.Member = diagnosticCanary
	v.Diagnostics[0].Location.IndexKnown = false
	v.Diagnostics[0].Location.Index = 999
	v.Plan.Warnings = append(v.Plan.Warnings, query.WarningView{Code: diagnosticCanary, Message: diagnosticCanary})
	v.DetailsOmitted = false
	for _, value := range []any{v, &v, v.Diagnostics[0], &v.Diagnostics[0], v.Diagnostics[0].Location, []DiagnosticView{v}} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal("serialization")
		}
		if bytes.Contains(b, []byte(diagnosticCanary)) || strings.Contains(fmt.Sprintf("%#v", value), diagnosticCanary) {
			t.Fatal("modified view disclosed")
		}
	}
	b, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	var forged query.QueryPlan
	_ = json.Unmarshal(b, &forged)
	if forged.SQL != "" || len(forged.Params) > 0 {
		t.Fatal("execution material reconstructed")
	}
	if p.SQL != originalSQL || !reflect.DeepEqual(p.Params, originalArgs) {
		t.Fatal("projection mutated source")
	}
	var out bytes.Buffer
	if WriteDiagnosticPretty(&out, v) != nil || WriteDiagnosticJSON(&out, v) != nil || strings.Contains(out.String(), diagnosticCanary) {
		t.Fatal("writer disclosure")
	}
}

type diagnosticSink struct{ short bool }

func (s diagnosticSink) Write(b []byte) (int, error) {
	if s.short {
		return len(b) - 1, nil
	}
	return 0, diagnosticOpaque{}
}
func TestDiagnosticWireErrorsBoundsAndSink(t *testing.T) {
	s, o := typedFixture("bigint", "postgres")
	_, v, e := CompileWithDiagnostics(t.Context(), s, o)
	if e != nil {
		t.Fatal(e)
	}
	good, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeDiagnosticView(good); e != nil {
		t.Fatal(e)
	}
	bad := [][]byte{
		nil, []byte("null"), append(append([]byte(nil), good...), good...),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1.0`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1e0`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":"1"`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":0`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":2`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":null`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		bytes.Replace(good, []byte(`"version":1`), []byte(`"Version":1`), 1),
		bytes.Replace(good, []byte(`"kind":"goquent.operation_diagnostics"`), []byte(`"kind":"goquent.plan_view"`), 1),
		bytes.Replace(good, []byte(`"details_omitted":true`), []byte(`"details_omitted":null`), 1),
		append([]byte(`{"arbitrary":"secret",`), good[1:]...),
		bytes.Repeat([]byte(" "), 1<<20+1),
	}
	for i, b := range bad {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			out, e := DecodeDiagnosticView(b)
			if e != publicview.ErrInvalid || out.Kind != "" {
				t.Fatal("wire accepted")
			}
			receiver := v
			if e := receiver.UnmarshalJSON(b); e != publicview.ErrInvalid || receiver.Kind != "" {
				t.Fatal("receiver not cleared")
			}
		})
	}
	for _, sink := range []io.Writer{nil, diagnosticSink{}, diagnosticSink{short: true}} {
		if WriteDiagnosticJSON(sink, v) != publicview.ErrWrite || WriteDiagnosticPretty(sink, v) != publicview.ErrWrite {
			t.Fatal("sink error escaped")
		}
	}
	for _, mutate := range []func(*DiagnosticView){func(v *DiagnosticView) { v.Kind = "bad" }, func(v *DiagnosticView) { v.Version = 2 }} {
		bad := v
		mutate(&bad)
		if _, e := bad.ToJSON(); e != publicview.ErrInvalid || bad.String() != publicview.ErrInvalid.Error() {
			t.Fatal("bad envelope laundered")
		}
	}
	v.Diagnostics = make([]DiagnosticEntry, 10000)
	v.Plan.Warnings = make([]query.WarningView, 10000)
	v.Plan.SuppressedWarnings = make([]query.WarningView, 10000)
	b, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := DecodeDiagnosticView(b)
	if e != nil || !decoded.Truncated || len(decoded.Diagnostics) != 128 || len(decoded.Plan.Warnings) != 128 {
		t.Fatal("bounded roundtrip")
	}
	// Raw structural element budget is checked before nested list truncation.
	type raw DiagnosticView
	v.Diagnostics = make([]DiagnosticEntry, 513)
	v.Plan = nil
	b, _ = json.Marshal(raw(v))
	if _, e := DecodeDiagnosticView(b); e != publicview.ErrInvalid {
		t.Fatal("structural budget ignored")
	}
}

func TestDiagnosticInternalDeclaredChecksAndActions(t *testing.T) {
	for _, tc := range []struct {
		typ, op string
		value   any
		code    string
	}{
		{"varchar(2)", "=", "abc", "OPERATION_TYPE_MISMATCH"},
		{"bigint[]", "=", 1, "OPERATION_ARRAY_UNSUPPORTED"},
		{"numeric(5,2)", "=", json.Number("1.25"), "OPERATION_UNVERIFIED_BINDING"},
		{"timestamp", "=", "2026-01-01T00:00:00", "OPERATION_TYPE_UNVERIFIED"},
	} {
		t.Run(tc.typ, func(t *testing.T) {
			s, o := typedFixture(tc.typ, "postgres")
			if tc.typ == "timestamp" {
				o.Manifest.Dialect = "mysql"
			}
			s.Filters[0].Op = tc.op
			s.Filters[0].Value = tc.value
			r := newRecorder(s)
			_, _ = compileOperation(t.Context(), s, o, r)
			found := false
			for _, d := range r.records {
				if d.Code == tc.code && d.Status != "warning" {
					found = true
					if d.Missing == "" || d.Action == "" {
						t.Fatal("missing repair evidence")
					}
				}
			}
			if !found {
				t.Fatal("missing expected record")
			}
		})
	}
}

func TestDiagnosticClosedCatalogFixture(t *testing.T) {
	b, e := os.ReadFile("../../tests/contracts/testdata/operation_diagnostic_codes.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct {
		Operation []string `json:"operation_codes"`
		Warnings  []string `json:"warning_codes"`
	}
	if json.Unmarshal(b, &catalog) != nil {
		t.Fatal("catalog")
	}
	for _, code := range append(catalog.Operation, catalog.Warnings...) {
		if diagnosticCode(code) != code {
			t.Fatal("catalog drift")
		}
		v := DiagnosticEntry{Code: code, Status: "refused", Message: diagnosticCanary}
		b, _ := v.MarshalJSON()
		if bytes.Contains(b, []byte(diagnosticCanary)) || !bytes.Contains(b, []byte(code+": diagnostic details omitted.")) {
			t.Fatal("message mapping changed")
		}
	}
	d := DiagnosticEntry{Code: "OPERATION_TYPE_MISMATCH", Status: "checked"}.safe()
	if d.Status != "unknown" {
		t.Fatal("mismatched status marked checked")
	}
}

func TestDiagnosticCombinedBudgetLocationUnknown(t *testing.T) {
	s, o := typedFixture("bigint", "postgres")
	o.Values = map[string]any{"unused": diagnosticOpaque{}}
	_, v, e := CompileWithDiagnostics(t.Context(), s, o)
	if !errors.Is(e, ErrInputLimit) || v.Diagnostics[0].Location.Source != "unknown" || v.Diagnostics[0].Location.IndexKnown {
		t.Fatal("combined wrapper attributed to a made-up spec location")
	}
	s, o = typedFixture("bigint", "postgres")
	s.Filters[0].Field = "wrong.v"
	_, _, e = CompileWithDiagnostics(t.Context(), s, o)
	f, ok := e.(*validationFailure)
	if !ok {
		t.Fatal("history")
	}
	d := f.diagnostics[len(f.diagnostics)-1]
	if d.Qualifier != "wrong" || d.Table != "items" || d.Code != "OPERATION_FIELD_UNKNOWN" {
		t.Fatal("input qualifier lost")
	}
}

func TestDiagnosticUnknownDriverNotChecked(t *testing.T) {
	s, o := typedFixture("bigint", "")
	p, v, e := CompileWithDiagnostics(t.Context(), s, o)
	if e != nil || !p.Blocked || v.Coverage != "partial" {
		t.Fatal("unknown dialect changed gate")
	}
	found := false
	for _, d := range v.Diagnostics {
		if d.Code == "OPERATION_CHECK_BINDING" {
			t.Fatal("unknown driver marked checked")
		}
		found = found || d.Code == "OPERATION_UNVERIFIED_DRIVER"
	}
	if !found {
		t.Fatal("missing driver uncertainty")
	}
}
