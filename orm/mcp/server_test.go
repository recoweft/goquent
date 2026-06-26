package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
)

func TestResourcesExposeManifestAndStaleStatus(t *testing.T) {
	server := NewServer(Options{Manifest: mcpTestManifest(true)})

	resources := server.Resources()
	if !hasResource(resources, "goquent://manifest") || !hasResource(resources, "goquent://manifest-status") {
		t.Fatalf("expected manifest resources, got %#v", resources)
	}
	statusText, mimeType, err := server.ReadResource("goquent://manifest-status")
	if err != nil {
		t.Fatal(err)
	}
	if mimeType != "application/json" {
		t.Fatalf("expected json mime type, got %s", mimeType)
	}
	if !strings.Contains(statusText, `"fresh": false`) {
		t.Fatalf("expected stale manifest status, got %s", statusText)
	}
}

func TestToolsAreReadOnlyAndCompileOperationSpec(t *testing.T) {
	server := NewServer(Options{Manifest: mcpTestManifest(false)})
	tools := server.Tools()
	if !hasTool(tools, "compile_operation_spec") || !hasTool(tools, "review_migration") {
		t.Fatalf("expected review/compile tools, got %#v", tools)
	}
	if hasTool(tools, "apply_migration") || hasTool(tools, "exec_sql") {
		t.Fatalf("MCP server must not expose write tools: %#v", tools)
	}

	limit := float64(10)
	result, err := server.CallTool(context.Background(), "compile_operation_spec", map[string]any{
		"spec": map[string]any{
			"operation": "select",
			"model":     "User",
			"select":    []any{"id", "name"},
			"filters": []any{map[string]any{
				"field":     "tenant_id",
				"op":        "=",
				"value_ref": "current_tenant",
			}},
			"limit": limit,
		},
		"values": map[string]any{"current_tenant": "tenant-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) != 1 || !strings.Contains(result.Content[0].Text, "tenant-1") {
		t.Fatalf("expected compiled QueryPlan with resolved value, got %#v", result)
	}
}

func TestReviewToolsDoNotExecuteSQL(t *testing.T) {
	server := NewServer(Options{Manifest: mcpTestManifest(false)})

	queryResult, err := server.CallTool(context.Background(), "review_query", map[string]any{"sql": "DROP TABLE users"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(queryResult.Content[0].Text, query.WarningDestructiveSQL) {
		t.Fatalf("expected destructive SQL warning, got %s", queryResult.Content[0].Text)
	}

	migrationResult, err := server.CallTool(context.Background(), "review_migration", map[string]any{"sql": "ALTER TABLE users DROP COLUMN legacy_id;"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(migrationResult.Content[0].Text, "MIGRATION_DROP_COLUMN") {
		t.Fatalf("expected migration warning, got %s", migrationResult.Content[0].Text)
	}
}

func TestJSONRPCHandleAndServe(t *testing.T) {
	server := NewServer(Options{Manifest: mcpTestManifest(false)})
	request := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	response, ok := server.HandleJSONRPC(context.Background(), request)
	if !ok {
		t.Fatal("expected response")
	}
	if !bytes.Contains(response, []byte("compile_operation_spec")) {
		t.Fatalf("expected tools/list response, got %s", string(response))
	}

	var framed bytes.Buffer
	payload := []byte(`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"goquent://policies"}}`)
	framed.WriteString("Content-Length: ")
	framed.WriteString(stringInt(len(payload)))
	framed.WriteString("\r\n\r\n")
	framed.Write(payload)
	var out bytes.Buffer
	if err := server.Serve(context.Background(), &framed, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Content-Length:")) || !bytes.Contains(out.Bytes(), []byte("tenant_scope")) {
		t.Fatalf("expected framed resource response, got %s", out.String())
	}
}

func TestReadMessageRejectsInvalidContentLength(t *testing.T) {
	for name, input := range map[string]string{
		"negative": "Content-Length: -1\r\n\r\n",
		"tooLarge": "Content-Length: 1048577\r\n\r\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := readMessage(bufio.NewReader(strings.NewReader(input)))
			if err == nil {
				t.Fatal("expected invalid Content-Length error")
			}
		})
	}
}

func TestPrompts(t *testing.T) {
	server := NewServer(Options{Manifest: mcpTestManifest(false)})
	if !hasPrompt(server.Prompts(), "write_safe_migration") {
		t.Fatalf("expected write_safe_migration prompt")
	}
	messages, err := server.GetPrompt("review_database_change", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || !strings.Contains(messages[0].Content.Text, "RiskLow") {
		t.Fatalf("expected review prompt guidance, got %#v", messages)
	}
}

func TestExposureAllowlist(t *testing.T) {
	server := NewServer(Options{
		Manifest:  mcpTestManifest(false),
		Resources: []string{"manifest"},
		Tools:     []string{"get_manifest"},
		Prompts:   []string{"explain_query_plan"},
	})

	if !hasResource(server.Resources(), "goquent://manifest") || hasResource(server.Resources(), "goquent://schema") {
		t.Fatalf("expected only manifest resource, got %#v", server.Resources())
	}
	if _, _, err := server.ReadResource("goquent://manifest"); err != nil {
		t.Fatalf("expected manifest resource by URI to be allowed: %v", err)
	}
	if _, _, err := server.ReadResource("goquent://schema"); err == nil {
		t.Fatalf("expected schema resource to be blocked")
	}
	if !hasTool(server.Tools(), "get_manifest") || hasTool(server.Tools(), "review_query") {
		t.Fatalf("expected only get_manifest tool, got %#v", server.Tools())
	}
	if _, err := server.CallTool(context.Background(), "review_query", map[string]any{"sql": "select 1"}); err == nil {
		t.Fatalf("expected review_query to be blocked")
	}
	if !hasPrompt(server.Prompts(), "explain_query_plan") || hasPrompt(server.Prompts(), "write_safe_migration") {
		t.Fatalf("expected only explain_query_plan prompt, got %#v", server.Prompts())
	}
	if _, err := server.GetPrompt("write_safe_migration", nil); err == nil {
		t.Fatalf("expected write_safe_migration prompt to be blocked")
	}
}

func mcpTestManifest(stale bool) *manifest.Manifest {
	m := &manifest.Manifest{
		Version:          manifest.Version,
		GeneratedAt:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: manifest.Generator,
		Tables: []manifest.Table{{
			Name:  "users",
			Model: "User",
			Columns: []manifest.Column{
				{Name: "deleted_at", SoftDelete: true, Nullable: true},
				{Name: "email", PII: true},
				{Name: "id", Primary: true},
				{Name: "name"},
				{Name: "tenant_id", TenantScope: true, RequiredFilter: true},
			},
			Policies: []manifest.Policy{
				{Type: "tenant_scope", Column: "tenant_id", Mode: query.PolicyModeEnforce},
				{Type: "soft_delete", Column: "deleted_at", Mode: query.PolicyModeEnforce},
				{Type: "pii", Column: "email", Mode: query.PolicyModeWarn},
			},
			QueryExamples: []manifest.QueryExample{{Name: "tenant_scoped_select", Operation: operation.OperationSelect, Select: []string{"id"}}},
		}},
	}
	if stale {
		m.Verification = &manifest.Verification{Fresh: false, CheckedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	}
	return m
}

func hasResource(resources []Resource, uri string) bool {
	for _, resource := range resources {
		if resource.URI == uri {
			return true
		}
	}
	return false
}

func hasTool(tools []Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func hasPrompt(prompts []Prompt, name string) bool {
	for _, prompt := range prompts {
		if prompt.Name == name {
			return true
		}
	}
	return false
}

func stringInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
