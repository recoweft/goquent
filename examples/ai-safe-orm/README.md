# AI-safe ORM Example

This example shows the Goquent safety workflow without requiring a live database.

It covers:

- simple CRUD planning with `QueryPlan`.
- tenant-scoped SaaS policies.
- soft-delete default filters.
- PII review with an access reason.
- suppression and approval examples.
- migration review inputs.
- manifest verification inputs.
- CI and MCP integration snippets.

Run:

```bash
go run ./examples/ai-safe-orm
```

Review:

```bash
go run ./cmd/goquent review --format pretty --fail-on blocked ./examples/ai-safe-orm
go run ./cmd/goquent migrate plan ./examples/ai-safe-orm/migrations/001_add_users.sql
go run ./cmd/goquent migrate plan ./examples/ai-safe-orm/migrations/002_drop_legacy_email.sql
go run ./cmd/goquent manifest verify \
  --manifest ./examples/ai-safe-orm/goquent.manifest.json \
  --schema ./examples/ai-safe-orm/schema.json \
  --policy ./examples/ai-safe-orm/policies.json
go run ./cmd/goquent operation compile \
  --manifest ./examples/ai-safe-orm/goquent.manifest.json \
  --spec ./examples/ai-safe-orm/operation.json \
  --values ./examples/ai-safe-orm/values.json \
  --format json
```

The example intentionally contains review findings so the output is visible; `--fail-on blocked`
keeps the walkthrough command runnable.

Run an MCP server with a narrow tool surface:

```bash
go run ./cmd/goquent mcp \
  --manifest ./examples/ai-safe-orm/goquent.manifest.json \
  --tool get_manifest \
  --tool review_query \
  --tool compile_operation_spec \
  --resource manifest \
  --resource manifest-status
```

The MCP command exposes context and review tools only. It does not perform DB writes, raw SQL
execution, or migration apply.

The executable prints redacted PlanView/MigrationPlanView output and fixed error
messages. Checked-in fictional source JSON is internal fixture data, not a public
display API. MCP returns structural summaries without real schema discovery.


GQ-AI-07 migration: the executable now supplies an explicit signed application
tenant through DB Settings. The preserved JSON fixtures intentionally remain
legacy: manifest verification reports a stale schema fingerprint after regeneration,
and the CLI Operation command rejects the tenant value claim. These are expected
refusals, not runnable trusted tenant input. See the [typed validation guide](../../docs/typed-operation-validation.md)
for regeneration and application migration; CLI/MCP cannot deserialize trust.
