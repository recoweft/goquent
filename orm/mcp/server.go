package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
)

const (
	ProtocolVersion = "2024-11-05"
	ServerName      = "goquent"
	ServerVersion   = "0.1.0"
)

// Options configures the read-only MCP server.
type Options struct {
	Manifest  *manifest.Manifest
	Resources []string
	Tools     []string
	Prompts   []string
}

// Server exposes Goquent schema, review, and planning helpers through MCP.
type Server struct {
	manifest         *manifest.Manifest
	allowedResources map[string]struct{}
	allowedTools     map[string]struct{}
	allowedPrompts   map[string]struct{}
}

// NewServer creates a read-only MCP server.
func NewServer(opts Options) *Server {
	return &Server{
		manifest:         opts.Manifest,
		allowedResources: allowSet(opts.Resources),
		allowedTools:     allowSet(opts.Tools),
		allowedPrompts:   allowSet(opts.Prompts),
	}
}

// Resource describes an MCP resource.
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Tool describes an MCP tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ToolResult is an MCP tool result.
type ToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content is text content returned by tools/prompts/resources.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Prompt describes an MCP prompt template.
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes a prompt parameter.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage is returned by prompts/get.
type PromptMessage struct {
	Role    string  `json:"role"`
	Content Content `json:"content"`
}

// Resources lists read-only Goquent resources.
func (s *Server) Resources() []Resource {
	resources := []Resource{
		{URI: "goquent://schema", Name: "schema", Description: "Manifest table and column metadata", MimeType: "application/json"},
		{URI: "goquent://manifest", Name: "manifest", Description: "Full Goquent manifest including freshness status", MimeType: "application/json"},
		{URI: "goquent://models", Name: "models", Description: "Model-to-table metadata", MimeType: "application/json"},
		{URI: "goquent://relations", Name: "relations", Description: "Relation metadata from the manifest", MimeType: "application/json"},
		{URI: "goquent://policies", Name: "policies", Description: "Policy metadata from the manifest", MimeType: "application/json"},
		{URI: "goquent://migrations", Name: "migrations", Description: "Migration review capabilities; apply is not exposed", MimeType: "text/plain"},
		{URI: "goquent://query-examples", Name: "query-examples", Description: "Safe query-shape examples from the manifest", MimeType: "application/json"},
		{URI: "goquent://review-rules", Name: "review-rules", Description: "Built-in review warning codes", MimeType: "application/json"},
		{URI: "goquent://manifest-status", Name: "manifest-status", Description: "Manifest freshness status", MimeType: "application/json"},
	}
	out := resources[:0]
	for _, resource := range resources {
		if s.resourceAllowed(resource.URI) || s.resourceAllowed(resource.Name) {
			out = append(out, resource)
		}
	}
	return out
}

// ReadResource returns a resource body.
func (s *Server) ReadResource(uri string) (string, string, error) {
	if !s.resourceAllowed(uri) && !s.resourceAllowed(resourceName(uri)) {
		return "", "", fmt.Errorf("resource %q is not exposed", uri)
	}
	switch uri {
	case "goquent://manifest":
		return s.jsonText(s.manifestPayload()), "application/json", nil
	case "goquent://manifest-status":
		return s.jsonText(s.manifestStatus()), "application/json", nil
	case "goquent://schema":
		return s.jsonText(schemaPayload{Tables: s.tables()}), "application/json", nil
	case "goquent://models":
		return s.jsonText(modelsPayload{Models: s.models()}), "application/json", nil
	case "goquent://relations":
		return s.jsonText(relationsPayload{Relations: s.relations()}), "application/json", nil
	case "goquent://policies":
		return s.jsonText(policiesPayload{Policies: s.policies()}), "application/json", nil
	case "goquent://migrations":
		return "Migration planning and review are available through review_migration. Migration apply is intentionally not exposed by MCP.", "text/plain", nil
	case "goquent://query-examples":
		return s.jsonText(queryExamplesPayload{Examples: s.queryExamples()}), "application/json", nil
	case "goquent://review-rules":
		return s.jsonText(reviewRules()), "application/json", nil
	default:
		return "", "", fmt.Errorf("unknown resource %q", uri)
	}
}

// Tools lists read-only MCP tools.
func (s *Server) Tools() []Tool {
	stringProp := map[string]any{"type": "string"}
	objectProp := map[string]any{"type": "object"}
	tools := []Tool{
		{Name: "get_schema", Description: "Return manifest schema tables and columns", InputSchema: objectSchema(nil)},
		{Name: "get_manifest", Description: "Return the full manifest", InputSchema: objectSchema(nil)},
		{Name: "get_manifest_status", Description: "Return manifest freshness status", InputSchema: objectSchema(nil)},
		{Name: "explain_query", Description: "Create a raw SQL QueryPlan without executing SQL", InputSchema: objectSchema(map[string]any{"sql": stringProp})},
		{Name: "review_query", Description: "Review raw SQL without executing it", InputSchema: objectSchema(map[string]any{"sql": stringProp})},
		{Name: "review_migration", Description: "Review migration SQL without applying it", InputSchema: objectSchema(map[string]any{"sql": stringProp})},
		{Name: "generate_query_plan", Description: "Generate a QueryPlan from raw SQL or read-only OperationSpec", InputSchema: objectSchema(map[string]any{"sql": stringProp, "operation_spec": objectProp, "values": objectProp})},
		{Name: "compile_operation_spec", Description: "Compile read-only select OperationSpec to QueryPlan", InputSchema: objectSchema(map[string]any{"spec": objectProp, "values": objectProp, "require_fresh_manifest": map[string]any{"type": "boolean"}})},
		{Name: "propose_repository_method", Description: "Return a safe repository method skeleton for an OperationSpec", InputSchema: objectSchema(map[string]any{"name": stringProp, "spec": objectProp})},
		{Name: "generate_test_fixture", Description: "Generate a small JSON fixture for manifest and OperationSpec tests", InputSchema: objectSchema(nil)},
	}
	out := tools[:0]
	for _, tool := range tools {
		if s.toolAllowed(tool.Name) {
			out = append(out, tool)
		}
	}
	return out
}

// CallTool executes a read-only MCP tool.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (ToolResult, error) {
	_ = ctx
	if !s.toolAllowed(name) {
		return ToolResult{}, fmt.Errorf("tool %q is not exposed", name)
	}
	switch name {
	case "get_schema":
		return s.textTool(s.mustRead("goquent://schema")), nil
	case "get_manifest":
		return s.textTool(s.mustRead("goquent://manifest")), nil
	case "get_manifest_status":
		return s.textTool(s.mustRead("goquent://manifest-status")), nil
	case "explain_query", "review_query":
		sqlText, err := requiredString(args, "sql")
		if err != nil {
			return ToolResult{}, err
		}
		plan := query.NewRawPlan(sqlText)
		b, err := plan.ToJSON()
		if err != nil {
			return ToolResult{}, err
		}
		return s.textTool(string(b)), nil
	case "review_migration":
		sqlText, err := requiredString(args, "sql")
		if err != nil {
			return ToolResult{}, err
		}
		plan, err := migration.PlanSQL(sqlText)
		if err != nil {
			return ToolResult{}, err
		}
		b, err := plan.ToJSON()
		if err != nil {
			return ToolResult{}, err
		}
		return s.textTool(string(b)), nil
	case "generate_query_plan":
		if sqlText, ok := optionalString(args, "sql"); ok {
			plan := query.NewRawPlan(sqlText)
			b, err := plan.ToJSON()
			if err != nil {
				return ToolResult{}, err
			}
			return s.textTool(string(b)), nil
		}
		return s.compileOperationSpec(args)
	case "compile_operation_spec":
		return s.compileOperationSpec(args)
	case "propose_repository_method":
		return s.proposeRepositoryMethod(args)
	case "generate_test_fixture":
		return s.textTool(testFixture()), nil
	default:
		return ToolResult{}, fmt.Errorf("unknown tool %q", name)
	}
}

// Prompts lists prompt templates.
func (s *Server) Prompts() []Prompt {
	prompts := []Prompt{
		{Name: "add_repository_method", Description: "Guide an AI to add a safe repository method", Arguments: []PromptArgument{{Name: "model", Required: true}, {Name: "operation", Required: false}}},
		{Name: "review_database_change", Description: "Review a DB code or migration change with Goquent safety rules"},
		{Name: "write_safe_migration", Description: "Write a staged migration and review it before apply"},
		{Name: "debug_slow_query", Description: "Use QueryPlan and manifest context to debug a slow query"},
		{Name: "explain_query_plan", Description: "Explain a QueryPlan for human review"},
	}
	out := prompts[:0]
	for _, prompt := range prompts {
		if s.promptAllowed(prompt.Name) {
			out = append(out, prompt)
		}
	}
	return out
}

// GetPrompt returns a prompt body.
func (s *Server) GetPrompt(name string, args map[string]any) ([]PromptMessage, error) {
	if !s.promptAllowed(name) {
		return nil, fmt.Errorf("prompt %q is not exposed", name)
	}
	modelName, _ := optionalString(args, "model")
	switch name {
	case "add_repository_method":
		return userPrompt("Add a Goquent repository method for " + modelName + ". Use OperationSpec or QueryPlan first, enforce manifest policies, avoid raw SQL unless explicitly justified, and include tests for required filters and PII handling."), nil
	case "review_database_change":
		return userPrompt("Review this database change with Goquent. Check QueryPlan or MigrationPlan output, risk warnings, approval requirements, suppressions, and manifest freshness. Do not treat RiskLow as business approval."), nil
	case "write_safe_migration":
		return userPrompt("Write a safe staged migration. Run goquent migrate plan, avoid destructive changes without preflight checks and explicit approval, and do not use MCP to apply migrations."), nil
	case "debug_slow_query":
		return userPrompt("Debug the slow query using manifest schema/index context and QueryPlan output. Suggest builder changes without executing database writes."), nil
	case "explain_query_plan":
		return userPrompt("Explain the QueryPlan in review-friendly terms: SQL, params, tables, predicates, risk, warnings, approval, and policy implications."), nil
	default:
		return nil, fmt.Errorf("unknown prompt %q", name)
	}
}

func allowSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}
	return out
}

func (s *Server) resourceAllowed(uri string) bool {
	return allowed(s.allowedResources, uri)
}

func resourceName(uri string) string {
	return strings.TrimPrefix(uri, "goquent://")
}

func (s *Server) toolAllowed(name string) bool {
	return allowed(s.allowedTools, name)
}

func (s *Server) promptAllowed(name string) bool {
	return allowed(s.allowedPrompts, name)
}

func allowed(allow map[string]struct{}, name string) bool {
	if len(allow) == 0 {
		return true
	}
	_, ok := allow[name]
	return ok
}

func (s *Server) compileOperationSpec(args map[string]any) (ToolResult, error) {
	if s.manifest == nil {
		return ToolResult{}, fmt.Errorf("manifest is required to compile operation specs")
	}
	rawSpec, ok := args["spec"]
	if !ok {
		rawSpec, ok = args["operation_spec"]
	}
	if !ok {
		return ToolResult{}, fmt.Errorf("spec is required")
	}
	var spec operation.OperationSpec
	if err := decodeAny(rawSpec, &spec); err != nil {
		return ToolResult{}, err
	}
	values := map[string]any(nil)
	if rawValues, ok := args["values"]; ok {
		if err := decodeAny(rawValues, &values); err != nil {
			return ToolResult{}, err
		}
	}
	requireFresh, _ := optionalBool(args, "require_fresh_manifest")
	plan, err := operation.Compile(context.Background(), spec, operation.Options{
		Manifest:             s.manifest,
		Values:               values,
		RequireFreshManifest: requireFresh,
	})
	if err != nil {
		return ToolResult{}, err
	}
	b, err := plan.ToJSON()
	if err != nil {
		return ToolResult{}, err
	}
	return s.textTool(string(b)), nil
}

func (s *Server) proposeRepositoryMethod(args map[string]any) (ToolResult, error) {
	name, _ := optionalString(args, "name")
	if name == "" {
		name = "FindRows"
	}
	text := fmt.Sprintf(`func (r *Repository) %s(ctx context.Context, spec operation.OperationSpec, values map[string]any) (*query.QueryPlan, error) {
	plan, err := operation.Compile(ctx, spec, operation.Options{
		Manifest: r.manifest,
		Values: values,
		RequireFreshManifest: true,
	})
	if err != nil {
		return nil, err
	}
	return plan, nil
}
`, name)
	return s.textTool(text), nil
}

func (s *Server) manifestPayload() any {
	if s.manifest == nil {
		return map[string]any{"version": manifest.Version, "tables": []any{}, "verification": s.manifestStatus()}
	}
	return s.manifest
}

func (s *Server) manifestStatus() manifest.Verification {
	if s.manifest != nil && s.manifest.Verification != nil {
		return *s.manifest.Verification
	}
	return manifest.Verification{Fresh: true}
}

func (s *Server) tables() []manifest.Table {
	if s.manifest == nil {
		return nil
	}
	return s.manifest.Tables
}

func (s *Server) models() []map[string]any {
	var out []map[string]any
	for _, table := range s.tables() {
		out = append(out, map[string]any{"model": table.Model, "table": table.Name, "columns": table.Columns})
	}
	return out
}

func (s *Server) relations() []manifest.Relation {
	var out []manifest.Relation
	for _, table := range s.tables() {
		out = append(out, table.Relations...)
	}
	return out
}

func (s *Server) policies() []manifest.Policy {
	var out []manifest.Policy
	for _, table := range s.tables() {
		out = append(out, table.Policies...)
	}
	return out
}

func (s *Server) queryExamples() []manifest.QueryExample {
	var out []manifest.QueryExample
	for _, table := range s.tables() {
		out = append(out, table.QueryExamples...)
	}
	return out
}

func (s *Server) mustRead(uri string) string {
	text, _, err := s.ReadResource(uri)
	if err != nil {
		return err.Error()
	}
	return text
}

func (s *Server) textTool(text string) ToolResult {
	return ToolResult{Content: []Content{{Type: "text", Text: text}}}
}

func (s *Server) jsonText(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return `{"error":"` + err.Error() + `"}`
	}
	return string(b)
}

type schemaPayload struct {
	Tables []manifest.Table `json:"tables"`
}

type modelsPayload struct {
	Models []map[string]any `json:"models"`
}

type relationsPayload struct {
	Relations []manifest.Relation `json:"relations"`
}

type policiesPayload struct {
	Policies []manifest.Policy `json:"policies"`
}

type queryExamplesPayload struct {
	Examples []manifest.QueryExample `json:"examples"`
}

type reviewRule struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

func reviewRules() []reviewRule {
	return []reviewRule{
		{Code: query.WarningUpdateWithoutWhere, Description: "UPDATE without WHERE is blocked"},
		{Code: query.WarningDeleteWithoutWhere, Description: "DELETE without WHERE is blocked"},
		{Code: query.WarningLimitMissing, Description: "SELECT list query has no LIMIT"},
		{Code: query.WarningSelectStarUsed, Description: "SELECT * is harder to review"},
		{Code: query.WarningRawSQLUsed, Description: "Raw SQL cannot be fully inspected"},
		{Code: query.WarningDestructiveSQL, Description: "SQL contains destructive DDL"},
		{Code: migration.WarningMigrationDropTable, Description: "Migration drops a table"},
		{Code: migration.WarningMigrationDropColumn, Description: "Migration drops a column"},
		{Code: manifest.WarningStale, Description: "Manifest is stale"},
		{Code: operation.WarningOperationPIISelected, Description: "OperationSpec selects PII"},
		{Code: reviewStaticPartialCode(), Description: "Static review could only partially reconstruct a query"},
	}
}

func reviewStaticPartialCode() string {
	return query.WarningStaticReviewPartial
}

func objectSchema(properties map[string]any) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	return map[string]any{"type": "object", "additionalProperties": true, "properties": properties}
}

func requiredString(args map[string]any, key string) (string, error) {
	value, ok := optionalString(args, key)
	if !ok || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func optionalString(args map[string]any, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	value, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := value.(string)
	return s, ok
}

func optionalBool(args map[string]any, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	value, ok := args[key]
	if !ok {
		return false, false
	}
	b, ok := value.(bool)
	return b, ok
}

func decodeAny(v any, out any) error {
	if s, ok := v.(string); ok {
		return json.Unmarshal([]byte(s), out)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func userPrompt(text string) []PromptMessage {
	return []PromptMessage{{Role: "user", Content: Content{Type: "text", Text: text}}}
}

func testFixture() string {
	return `{
  "manifest": {
    "version": "1",
    "tables": [
      {
        "name": "users",
        "model": "User",
        "columns": [
          {"name": "id", "primary": true},
          {"name": "tenant_id", "tenant_scope": true, "required_filter": true},
          {"name": "email", "pii": true}
        ],
        "policies": [
          {"type": "tenant_scope", "column": "tenant_id", "mode": "enforce"}
        ]
      }
    ]
  },
  "operation_spec": {
    "operation": "select",
    "model": "User",
    "select": ["id"],
    "filters": [{"field": "tenant_id", "op": "=", "value_ref": "current_tenant"}],
    "limit": 100
  }
}`
}
