package contracts_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/internal/operationtest"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/mcp"
	"github.com/recoweft/goquent/orm/operation"
)

func TestOperationDiagnosticSurfaceFixtures(t *testing.T) {
	cases, err := operationtest.Load("testdata/operation_diagnostics_v1.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := operation.JSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if operationtest.SchemaAccepts(schema, c.Spec) != c.SchemaValid {
				t.Fatal("schema classification differs")
			}
			var spec operation.OperationSpec
			if json.Unmarshal(c.Spec, &spec) != nil {
				t.Fatal("invalid fixture decode")
			}
			values, _ := operationtest.Decode(c.Values)
			valueMap, _ := values.(map[string]any)
			opts := operation.Options{Manifest: c.Manifest(), Values: valueMap}
			plan, view, err := operation.CompileWithDiagnostics(t.Context(), spec, opts)
			if (err == nil) != c.Compiled {
				t.Fatal("API acceptance differs")
			}
			checkDiagnosticFixture(t, view, c)
			legacy, legacyErr := operation.Compile(t.Context(), spec, opts)
			if (legacyErr == nil) != (err == nil) {
				t.Fatal("legacy acceptance changed")
			}
			if err == nil && (legacy.SQL != plan.SQL || !reflect.DeepEqual(legacy.Params, plan.Params)) {
				t.Fatal("legacy SQL or typed args differ")
			}
			_, rootView, rootErr := orm.CompileOperationSpecWithDiagnostics(t.Context(), spec, opts)
			if (rootErr == nil) != c.Compiled || rootView.String() != view.String() {
				t.Fatal("root API differs")
			}
			_, validateView, validateErr := operation.ValidateWithDiagnostics(spec, opts)
			if (validateErr == nil) != c.Compiled || validateView.String() != view.String() {
				t.Fatal("Validate differs")
			}
			rawSpec, _ := operationtest.Decode(c.Spec)
			args := map[string]any{"spec": rawSpec}
			if valueMap != nil {
				args["values"] = valueMap
			}
			server := mcp.NewServer(mcp.Options{Manifest: c.Manifest()})
			for _, name := range []string{"compile_operation_spec", "generate_query_plan"} {
				result, e := server.CallTool(context.Background(), name, args)
				if (e == nil) != c.Compiled || result.IsError == c.Compiled || len(result.Content) != 1 {
					t.Fatal("direct tool acceptance differs")
				}
				got, e := operation.DecodeDiagnosticView([]byte(result.Content[0].Text))
				if e != nil {
					t.Fatal(e)
				}
				checkDiagnosticFixture(t, got, c)
				// Origin is acquisition metadata, not a permission or category.
				if diagnosticCategories(got) != diagnosticCategories(view) {
					t.Fatal("direct categories differ")
				}
				request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
				response, ok := server.HandleJSONRPC(t.Context(), request)
				if !ok {
					t.Fatal("missing RPC response")
				}
				checkDiagnosticRPC(t, response, c)
				for _, framed := range []bool{false, true} {
					input := append(append([]byte(nil), request...), '\n')
					if framed {
						input = []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(request), request))
					}
					var out bytes.Buffer
					if e := server.Serve(t.Context(), bytes.NewReader(input), &out); e != nil {
						t.Fatal(e)
					}
					wire := out.Bytes()
					_, wire, _ = bytes.Cut(wire, []byte("\r\n\r\n"))

					checkDiagnosticRPC(t, wire, c)
				}
			}
		})
	}
}
func diagnosticCategories(v operation.DiagnosticView) string {
	var out strings.Builder
	out.WriteString(v.Outcome + "/" + v.Coverage)
	for _, d := range v.Diagnostics {
		fmt.Fprintf(&out, "/%s:%s:%s:%d:%t:%d", d.Code, d.Status, d.Location.Section, d.Location.Index, d.Location.ElementKnown, d.Location.Element)
	}
	return out.String()
}
func checkDiagnosticFixture(t *testing.T, v operation.DiagnosticView, c operationtest.Case) {
	t.Helper()
	if (v.Outcome == "compiled") != c.Compiled {
		t.Fatal("view outcome differs")
	}
	if c.Compiled {
		want := "checked_subset"
		if c.Partial {
			want = "partial"
		}
		if v.Coverage != want || v.Plan == nil || c.Partial && !v.Plan.Blocked {
			t.Fatal("coverage/plan differs")
		}
	} else {
		if v.Plan != nil || len(v.Diagnostics) == 0 || v.Diagnostics[0].Code != c.Code || v.Diagnostics[0].Status != "refused" {
			t.Fatalf("refusal category differs: %s", v.String())
		}
	}
	b, e := v.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(b, []byte("diagnostic_items")) || bytes.Contains(b, []byte("9007199254740993")) {
		t.Fatal("source data disclosed")
	}
}
func checkDiagnosticRPC(t *testing.T, b []byte, c operationtest.Case) {
	t.Helper()
	var response struct {
		Result mcp.ToolResult
		Error  json.RawMessage
	}
	if json.Unmarshal(b, &response) != nil || len(response.Error) > 0 || len(response.Result.Content) != 1 || response.Result.IsError == c.Compiled {
		t.Fatal("RPC result differs")
	}
	v, e := operation.DecodeDiagnosticView([]byte(response.Result.Content[0].Text))
	if e != nil {
		t.Fatal(e)
	}
	checkDiagnosticFixture(t, v, c)
}

func TestOperationDiagnosticWireDifferences(t *testing.T) {
	b, e := os.ReadFile("testdata/operation_diagnostic_wire_differences.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Name, Raw   string
		SchemaValid bool `json:"schema_valid"`
		Decoded     bool
	}
	if json.Unmarshal(b, &cases) != nil {
		t.Fatal("fixture")
	}
	schema, _ := operation.JSONSchema()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if operationtest.SchemaAccepts(schema, []byte(c.Raw)) != c.SchemaValid {
				t.Fatal("schema difference changed")
			}
			var s operation.OperationSpec
			e := json.Unmarshal([]byte(c.Raw), &s)
			if (e == nil) != c.Decoded {
				t.Fatal("wire difference changed")
			}
			if e == nil {
				opts := operation.Options{Manifest: (operationtest.Case{Type: "bigint", TypeSource: "sql"}).Manifest()}
				_, v, e := operation.CompileWithDiagnostics(t.Context(), s, opts)
				if e == nil || v.Diagnostics[0].Code != "OPERATION_TYPE_MISMATCH" {
					t.Fatal("integer lexeme coerced")
				}
			}
		})
	}
}
