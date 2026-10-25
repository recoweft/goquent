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
also verifies the original saved manifest with explicit schema/policy/generated
repository/declared DB inputs and nonwriting regeneration. This is supplied
snapshot evidence, not a native DB run or release acceptance. The saved model is
separate from the executable; see the [owner and evidence table](storedmodel/README.md).

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


The original saved manifest now declares four explicit axes for the saved
string-tenant repository model. Its schema/policy expectations are unchanged;
generated-code/database expectations are a new, intentional scope extension.
The earlier schema/policy-only state had two honest optional skips. The legacy
schema fixture in `orm/manifest/testdata/legacy-example.json` remains stale.

The handwritten executable uses integer tenant, `datetime` and explicit nullable
knowledge; saved `schema.json` uses string tenant, `timestamp` and omitted
nullable-known. They are separate models. Saved artifact verification does not
verify the executable's internal manifest or prove its live DB compatibility.
See [saved model instructions](storedmodel/README.md) for repository generation,
explicit CLI inputs, independent declarations, nonwriting verification, negative
cases and separately opted-in native checks. `tests/manifestfixture` remains a
separate companion whose success is not original example evidence.

The executable supplies an explicit signed application tenant through DB Settings.
`operation.json` and `values.json` still contain an untrusted tenant value claim
that the CLI Operation command rejects. Manifest freshness does not authorize
that claim. See the [typed validation guide](../../docs/typed-operation-validation.md);
CLI/MCP cannot deserialize application trust.
