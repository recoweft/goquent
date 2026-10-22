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


The manifest is regenerated from this directory's schema and policy files:

```bash
go run ./cmd/goquent manifest --dialect mysql \
  --schema ./examples/ai-safe-orm/schema.json \
  --policy ./examples/ai-safe-orm/policies.json \
  --unsafe-local-output /tmp/new-goquent-manifest.json
```

Use a new local artifact path, verify it with the inputs above, then replace only
this example's manifest. Generation time varies; the remaining generated content
is deterministic. GQ-MAINT-04 updates the five SQL type-source declarations from
the GQ-AI-07 migration. The old manifest remains an independent negative fixture
in `orm/manifest/testdata/legacy-example.json`; its schema comparison stays stale.
Schema and policy checks now pass; generated-code and database checks are skipped
because no such evidence is supplied. This is not full schema/capability coverage.

The executable supplies an explicit signed application tenant through DB Settings.
`operation.json` and `values.json` still contain an untrusted tenant value claim
that the CLI Operation command rejects. Manifest freshness does not authorize
that claim. See the [typed validation guide](../../docs/typed-operation-validation.md);
CLI/MCP cannot deserialize application trust.
