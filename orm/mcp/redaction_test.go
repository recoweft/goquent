package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
)

const publicCanary = "person-redaction@example.test_TOKEN_password_identifier"

type publicBomb struct{}

func (publicBomb) MarshalJSON() ([]byte, error) { panic("MarshalJSON evaluated") }
func (publicBomb) String() string               { panic("String evaluated") }
func (publicBomb) Error() string                { panic("Error evaluated") }
func assertPublic(t *testing.T, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(publicCanary)) {
		t.Fatal("public canary escaped")
	}
}
func TestPublicAllMCPRoutes(t *testing.T) {
	m := &manifest.Manifest{Version: manifest.Version, GeneratorVersion: publicCanary, Dialect: publicCanary, SchemaFingerprint: publicCanary, Tables: []manifest.Table{{Name: publicCanary, Model: publicCanary, Columns: []manifest.Column{{Name: publicCanary, Default: publicCanary, EnumValues: []string{publicCanary}}}, Relations: []manifest.Relation{{Name: publicCanary}}, Policies: []manifest.Policy{{Type: publicCanary}}, QueryExamples: []manifest.QueryExample{{Name: publicCanary, Description: publicCanary}}}}, Verification: &manifest.Verification{Checks: []manifest.FreshnessCheck{{Name: publicCanary, Status: publicCanary, Message: publicCanary, Expected: publicCanary, Actual: publicCanary}}}}
	s := NewServer(Options{Manifest: m})
	for _, r := range s.Resources() {
		text, _, err := s.ReadResource(r.URI)
		if err != nil {
			t.Fatal(err)
		}
		assertPublic(t, text)
	}
	for _, tool := range s.Tools() {
		result, err := s.CallTool(context.Background(), tool.Name, map[string]any{"sql": "SELECT '" + publicCanary + "' FROM secret", "name": publicCanary, "spec": map[string]any{"model": publicCanary, "operation": "select", "select": []any{publicCanary}}})
		assertPublic(t, result)
		if err != nil {
			assertPublic(t, fmt.Sprintf("%+v %#v", err, err))
		}
	}
	for _, p := range s.Prompts() {
		out, err := s.GetPrompt(p.Name, map[string]any{"model": publicCanary})
		if err != nil {
			t.Fatal(err)
		}
		assertPublic(t, out)
	}
	for _, method := range []string{"resources/read", "tools/call", "prompts/get", publicCanary} {
		req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": publicCanary, "method": method, "params": map[string]any{"uri": publicCanary, "name": publicCanary, "arguments": map[string]any{"name": publicCanary}}})
		b, ok := s.HandleJSONRPC(context.Background(), req)
		if !ok {
			t.Fatal("missing response")
		}
		var resp map[string]any
		if json.Unmarshal(b, &resp) != nil || resp["id"] != publicCanary {
			t.Fatal("correlation ID changed")
		}
		delete(resp, "id")
		assertPublic(t, resp)
	}
	// Successful tool and prompt RPC results also exclude the caller text.
	for _, method := range []string{"tools/call", "prompts/get"} {
		name := "propose_repository_method"
		if method == "prompts/get" {
			name = "add_repository_method"
		}
		req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": map[string]any{"name": name, "arguments": map[string]any{"name": publicCanary, "model": publicCanary}}})
		b, _ := s.HandleJSONRPC(context.Background(), req)
		assertPublic(t, string(b))
	}
}
func TestPublicInputRejectsCallbacksAndCycles(t *testing.T) {
	s := NewServer(Options{})
	cycle := map[string]any{}
	cycle["x"] = cycle
	var deep any = "x"
	for range 40 {
		deep = []any{deep}
	}
	for _, x := range []any{publicBomb{}, cycle, deep, make([]any, 20000), strings.Repeat("x", maxMessageBytes)} {
		if _, err := s.CallTool(context.Background(), "compile_operation_spec", map[string]any{"spec": x}); err == nil {
			t.Fatal("opaque input accepted")
		}
		if _, err := s.GetPrompt("add_repository_method", map[string]any{"model": x}); err == nil {
			t.Fatal("opaque prompt accepted")
		}
	}
}
func TestPublicRPCIDsAndEnvelope(t *testing.T) {
	s := NewServer(Options{})
	for _, id := range []string{`"correlation"`, `1`, `1.0`, `1e3`, `-9007199254740991`, `9007199254740991`, `0e999999999999999999999`} {
		b, ok := s.HandleJSONRPC(context.Background(), []byte(`{"jsonrpc":"2.0","id":`+id+`,"method":"ping"}`))
		if !ok || bytes.Contains(b, []byte(`"error"`)) {
			t.Fatalf("valid ID rejected: %s", id)
		}
	}
	for _, id := range []string{`null`, `true`, `[]`, `{"secret":"` + publicCanary + `"}`, `9007199254740992`, `1.1`, `1e999999999`, `1e-99999999`, `"` + strings.Repeat("x", 1025) + `"`} {
		b, ok := s.HandleJSONRPC(context.Background(), []byte(`{"jsonrpc":"2.0","id":`+id+`,"method":"tools/call","params":{"name":"generate_test_fixture"}}`))
		if !ok || !bytes.Contains(b, []byte(`"id":null`)) || !bytes.Contains(b, []byte(`"error"`)) || bytes.Contains(b, []byte(`"result"`)) {
			t.Fatal("invalid ID dispatched")
		}
		assertPublic(t, string(b))
	}
	for _, input := range []string{`{"jsonrpc":"2.0","id":1,"id":2,"method":"ping"}`, `{"jsonrpc":"2.0","ID":1,"method":"ping"}`, `{"jsonrpc":"2.0","method":"ping","params":null}`, `{"jsonrpc":"2.0","method":"ping","params":{"x":1,"X":2}}`, strings.Repeat(" ", maxMessageBytes+1), `{"secret":"` + publicCanary + `"}`} {
		b, ok := s.HandleJSONRPC(context.Background(), []byte(input))
		if !ok || !bytes.Contains(b, []byte(`"error"`)) {
			t.Fatal("invalid envelope accepted")
		}
		assertPublic(t, string(b))
	}
	if b, ok := s.HandleJSONRPC(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); ok || len(b) != 0 {
		t.Fatal("notification responded")
	}
}

type publicErrorSink struct{ short bool }

func (s publicErrorSink) Write(b []byte) (int, error) {
	if s.short {
		return 0, nil
	}
	return 0, publicBomb{}
}
func TestPublicServeBoundsAndSinkErrors(t *testing.T) {
	s := NewServer(Options{})
	for _, in := range []string{strings.Repeat("x", maxMessageBytes+1), "Content-Length: 4\r\n\r\n", "Content-Length: 1\r\n" + strings.Repeat("x", maxMessageBytes+1)} {
		err := s.Serve(context.Background(), strings.NewReader(in), &bytes.Buffer{})
		if err == nil {
			t.Fatal("unbounded/truncated input accepted")
		}
		assertPublic(t, fmt.Sprintf("%+v %#v", err, err))
	}
	for _, sink := range []publicErrorSink{{}, {short: true}} {
		err := s.Serve(context.Background(), strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}\n"), sink)
		if err == nil {
			t.Fatal("sink failure ignored")
		}
		assertPublic(t, err.Error())
	}
}
