package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/publicoutput"
)

func TestOperationDiagnosticPrivateFailureAdapter(t *testing.T) {
	const secret = "fictional_gq07_adapter_secret"
	arbitrary := ToolResult{IsError: true, Content: []Content{{Type: "text", Text: secret}}}
	if _, ok := operationFailureResult(arbitrary); ok {
		t.Fatal("arbitrary failure content trusted")
	}
	arbitrary.operationDiagnostic = &operation.DiagnosticView{Kind: secret, Version: 1}
	if _, ok := operationFailureResult(arbitrary); ok {
		t.Fatal("invalid diagnostic accepted")
	}
	m := &manifest.Manifest{Version: "1", Dialect: "postgres", Tables: []manifest.Table{{Name: secret, Columns: []manifest.Column{{Name: "v", Type: "bigint", TypeSource: "sql", NullableKnown: true}}}}}
	server := NewServer(Options{Manifest: m})
	args := map[string]any{"spec": map[string]any{"model": secret, "select": []any{"v"}, "filters": []any{map[string]any{"field": "v", "op": "=", "value": "not_integer"}}}}
	out, e := server.CallTool(context.Background(), "compile_operation_spec", args)
	if e != publicoutput.ErrOutput || !out.IsError || out.operationDiagnostic == nil {
		t.Fatal("direct refusal contract")
	}
	v, err := operation.DecodeDiagnosticView([]byte(out.Content[0].Text))
	if err != nil || v.Diagnostics[0].Location.Origin != "go" {
		t.Fatal("direct origin")
	}
	out.Content = []Content{{Type: secret, Text: secret}}
	safe, ok := operationFailureResult(out)
	if !ok || strings.Contains(safe.Content[0].Text, secret) {
		t.Fatal("untrusted content passed")
	}
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": secret, "method": "tools/call", "params": map[string]any{"name": "compile_operation_spec", "arguments": args}})
	response, _ := server.HandleJSONRPC(context.Background(), request)
	var envelope struct {
		ID     string
		Result ToolResult
	}
	if json.Unmarshal(response, &envelope) != nil || envelope.ID != secret {
		t.Fatal("top level correlation lost")
	}
	b, _ := json.Marshal(envelope.Result)
	if strings.Contains(string(b), secret) {
		t.Fatal("id exception spread")
	}
	v, err = operation.DecodeDiagnosticView([]byte(envelope.Result.Content[0].Text))
	if err != nil || v.Diagnostics[0].Location.Origin != "json" {
		t.Fatal("RPC origin")
	}
	raw, e := server.CallTool(context.Background(), "generate_query_plan", map[string]any{"sql": "SELECT 1"})
	if e != nil || !strings.Contains(raw.Content[0].Text, "goquent.plan_view") || strings.Contains(raw.Content[0].Text, operation.DiagnosticViewKind) {
		t.Fatal("raw path changed")
	}
}
