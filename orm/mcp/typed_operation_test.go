package mcp

import (
	"context"
	"encoding/json"
	"github.com/recoweft/goquent/orm/manifest"
	"strings"
	"testing"
)

func TestTypedOperationDirectAndRPCNoCoercionOrDisclosure(t *testing.T) {
	const secret = "gq07_type_secret_canary"
	m := &manifest.Manifest{Version: manifest.Version, Dialect: "postgres", Tables: []manifest.Table{{Name: secret, Columns: []manifest.Column{{Name: "v", Type: "bigint", TypeSource: "sql", NullableKnown: true}}}}}
	s := NewServer(Options{Manifest: m})
	for _, v := range []any{float64(1), "1", json.Number("9223372036854775808"), nil} {
		args := map[string]any{"spec": map[string]any{"operation": "select", "model": secret, "select": []any{"v"}, "filters": []any{map[string]any{"field": "v", "op": "=", "value": v}}}}
		result, e := s.CallTool(context.Background(), "compile_operation_spec", args)
		if e == nil {
			t.Fatal("invalid direct numeric input accepted", result)
		}
		if strings.Contains(e.Error(), secret) {
			t.Fatal("error leaked name")
		}
	}
	args := map[string]any{"spec": map[string]any{"operation": "select", "model": secret, "select": []any{"v"}, "filters": []any{map[string]any{"field": "v", "op": "=", "value": json.Number("9007199254740993")}}}}
	result, e := s.CallTool(context.Background(), "compile_operation_spec", args)
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range result.Content {
		if strings.Contains(c.Text, secret) || strings.Contains(c.Text, "9007199254740993") {
			t.Fatal("output leaked")
		}
	}
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"compile_operation_spec","arguments":{"spec":{"operation":"select","model":"` + secret + `","select":["v"],"filters":[{"field":"v","op":"=","value":1.0}]}}}}`)
	out, _ := s.HandleJSONRPC(context.Background(), request)
	if strings.Contains(string(out), secret) || !strings.Contains(string(out), `"isError":true`) {
		t.Fatal("RPC refusal leaked or accepted")
	}
}
