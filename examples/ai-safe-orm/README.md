# AI-safe ORM Example

This example shows the Goquent safety workflow without requiring a live database.

The default executable has no database connection and never calls an execution
terminal. It demonstrates explicit application settings, a generated manifest
summary, typed read-only OperationSpec and a validated read diagnostic. All
printed artifacts are non-executable public views.

Run from the repository root:

```bash
go run ./examples/ai-safe-orm
go test ./examples/ai-safe-orm -count=1
sh examples/ai-safe-orm/check.sh
```

`check.sh` builds the example and CLI, checks normal startup, and asserts the
expected nonzero exit codes and fixed classifications of both refusal modes and
the legacy tenant JSON. It captures build/errors locally and deletes them. It
also verifies the supplied manifest schema/policy; generated-code/database checks
remain skipped. It does not run a native DB or adopt release acceptance.

The built example accepts `refuse-tenant` (exit 3, `EXAMPLE_TENANT_REFUSED`) and
`refuse-authorization` (exit 4, `EXAMPLE_AUTHORIZATION_DENIED_CONSUMED`). The latter
raises the missing-limit risk to high, validates an authorization-pending
candidate, then checks a deny-only adapter and the consumed handle. Neither mode
can connect: the executor refuses every dispatch. The subprocess tests check
public output and exact statuses. `go run` wraps a program's nonzero status;
use the checker to verify the built executable's exact code.

The tenant/subject are fictional application inputs, not deserialized authority.
The random ephemeral binding key is a fixture only, not production key management.
No allow-all adapter or DB execution mode is provided. For application migration,
current-input binding, external authorization limits, dedicated native fixtures
and separate review/release checkpoints, use the
[current workflow guide](../../docs/safe-workflows-migration.md).

Ordinary review is independent and can fail:

```bash
go run ./cmd/goquent review --format pretty --fail-on high ./examples/ai-safe-orm
```

Preserve its risk/precision/unknown findings; do not lower the threshold to make
this walkthrough green. Migration plans/dry-runs remain review inputs with
separate deployment approval, not execution permissions.

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


For an intentional schema/policy update, a new manifest can be generated from this directory's schema and policy files:

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
