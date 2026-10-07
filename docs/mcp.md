# MCP Server Guide

Goquent can run as a read-only MCP server over stdio. It is intended to provide AI editors and
agents with current schema, policy, manifest freshness, review, and planning context without
granting database write capability.

```bash
go run ./cmd/goquent mcp --manifest goquent.manifest.json
```

You can attach current inputs to mark stale manifests:

```bash
go run ./cmd/goquent mcp \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --code ./orm
```

Resources:

- `goquent://manifest`
- `goquent://manifest-status`
- `goquent://schema`
- `goquent://models`
- `goquent://relations`
- `goquent://policies`
- `goquent://migrations`
- `goquent://query-examples`
- `goquent://review-rules`

Tools:

- `get_schema`
- `get_manifest`
- `get_manifest_status`
- `explain_query`
- `review_query`
- `review_migration`
- `generate_query_plan`
- `compile_operation_spec`
- `propose_repository_method`
- `generate_test_fixture`

Prompts:

- `add_repository_method`
- `review_database_change`
- `write_safe_migration`
- `debug_slow_query`
- `explain_query_plan`

MCP prompts return guidance text only. They do not write files, apply migrations, execute SQL, or
approve destructive operations. In particular, `write_safe_migration` is a migration-planning and
review prompt; it does not make a migration safe and it does not apply one.

Restrict the exposed surface with allowlists:

```bash
go run ./cmd/goquent mcp \
  --manifest goquent.manifest.json \
  --resource manifest \
  --resource manifest-status \
  --tool get_manifest \
  --tool review_query \
  --prompt review_database_change
```

The current MCP implementation does not expose DB writes, raw SQL execution, or migration apply.
Migration SQL and query text can be reviewed, but not executed, through MCP. MCP output is review
context, not business approval.

## Public output compatibility

Resources now return bounded redacted summaries, so real schema names/defaults/
examples cannot be discovered through MCP. Plans use PlanView/MigrationPlanView.
Prompts and generated teaching text ignore caller names. Valid string/integer IDs
are echoed only in top-level id; clients must not put secrets there. Unknown or
ambiguous input and errors never echo caller text. See [the inventory and exact
bounds](redacted-output.md). This remains read-only and does not grant execution.
