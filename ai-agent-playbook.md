# AI Agent Playbook

This is the operating guide for AI coding agents that write or review Goquent-powered database code. Use it before adding repository methods, changing query logic, using raw SQL, touching migrations, relying on manifests, or using Goquent through MCP.

Goquent is not an autonomous database agent. Treat it as a deterministic review boundary for database code that still needs human and business approval.

## Trust Boundary

These rules are mandatory:

- `RiskLow` means low structural database risk. It does not mean business approval, authorization, product correctness, or permission to execute.
- A passing `goquent review` means no configured finding at or above the selected threshold was detected. It does not prove semantic correctness.
- Static review can be `precise`, `partial`, or `unsupported`. Never describe `partial` or `unsupported` review as safe.
- A stale manifest must not be trusted for schema, policy, PII, relation, tenant, or migration decisions.
- Goquent warnings, suppressions, and approvals are review artifacts. They are not the only approval source.
- The current MCP scope is read-only context and review tooling. Do not use MCP for DB writes, migration apply, or raw SQL execution.
- Goquent does not give AI agents authority to operate a production database autonomously.

## Standard Workflow For DB Reads

From the repository root, use the project-specific schema and policy inputs:

```bash
go run ./cmd/goquent manifest verify \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json
```

Then choose the narrowest supported representation:

1. Use `OperationSpec` for single-model read-only `select` operations with explicit fields, filters, order, and limit.
2. Use the Goquent DSL when the read does not fit the OperationSpec MVP.
3. Use raw SQL only when the raw SQL conditions below are satisfied.

Compile OperationSpec when it fits:

```bash
go run ./cmd/goquent operation compile \
  --manifest goquent.manifest.json \
  --spec operation.json \
  --values values.json \
  --require-fresh-manifest \
  --format json
```

For Go code, compile and test first:

```bash
go test ./...
```

Review the database surface:

```bash
go run ./cmd/goquent review \
  --fail-on high \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --require-fresh-manifest \
  ./...
```

Before claiming a read is ready, inspect the final `QueryPlan` or review output for:

- SQL shape and parameter binding
- touched tables and selected columns
- predicates, required filters, ordering, and limit
- tenant scope and soft-delete behavior
- PII selection and access reason
- risk level and warnings
- suppressions and approval reasons
- manifest freshness
- `analysis_precision`

## Adding A Go Repository Method

Use this order:

1. Find the local repository style for the model or package.
2. Verify the manifest against current schema and policy inputs.
3. Identify relevant policies: tenant scope, soft delete, PII, and required filters.
4. Write the method using Goquent DSL or supported generic helpers.
5. Prefer explicit fields over `SELECT *`.
6. Keep tenant and authorization inputs explicit in the method signature when policy requires them.
7. Generate or inspect the `QueryPlan` with `Plan`, `PlanInsert`, `PlanUpdate`, or `PlanDelete` where possible.
8. Add focused tests for policy-relevant behavior, not only the happy path.
9. Run `gofmt` on changed Go files.
10. Run `go test ./...`.
11. Run `go run ./cmd/goquent review --fail-on high ./...`.
12. Attach the relevant output to the PR.

Do not hide a policy bypass inside a helper. If a repository method intentionally includes deleted rows, selects PII, omits a tenant filter, or performs a broad write, make the reason visible in the method name, call site, `AccessReason`, `RequireApproval`, suppression, or PR notes.

## Tenant Scope, Soft Delete, PII, And Required Filters

Tenant-scoped models must include the tenant boundary unless an explicit administrative path is approved. Do not assume the caller already filtered it.

Soft-deleted rows should be excluded from normal reads. Use deleted-row behavior only when the method intent is explicit.

PII columns should not be selected unless required. When policy requires an access reason, use a narrow reason that explains the specific purpose, such as a support ticket or export job.

Required filters are part of the access contract. Missing required filters usually mean the query is too broad or policy context is absent.

## Raw SQL

Prefer OperationSpec or the Goquent DSL. Raw SQL is acceptable only when all of these are true:

- The operation cannot reasonably be represented with supported Goquent APIs.
- The reason is specific, such as database-specific syntax, a measured performance need, or migration-only SQL.
- User input is parameterized and not concatenated into SQL text.
- Tenant scope, soft delete, PII, and required filters are applied manually and visibly.
- The raw SQL is covered by `goquent review`, a `QueryPlan` JSON artifact, or manual review evidence.
- The PR explains why raw SQL is necessary and how policy requirements are handled.

Review raw SQL paths with:

```bash
go run ./cmd/goquent review --fail-on high ./path/to/sql-or-go-files
```

If review reports `partial` or `unsupported`, include that limitation in the PR and add manual evidence. Do not say the SQL is safe.

## Migrations

Migrations are especially risky for AI-generated changes. Always plan and dry-run before any apply path.

```bash
go run ./cmd/goquent migrate plan ./migrations/001_change.sql
go run ./cmd/goquent migrate dry-run ./migrations/001_change.sql
```

If the plan is high or destructive, add a human-readable approval reason and preflight evidence:

```bash
go run ./cmd/goquent migrate dry-run \
  --approve "legacy column retired after code rollout" \
  ./migrations/001_change.sql
```

For migration PRs, attach:

- `MigrationPlan` output
- dry-run output
- destructive or high-risk approval reason
- preflight evidence
- rollback or compatibility notes when relevant
- `goquent review` output

Suggested preflight evidence for destructive changes:

- code search confirms no active references remain
- read/write paths have stopped using the object before it is dropped
- production usage or query logs show no recent access when applicable
- backup or restore strategy is documented
- deployment order is documented

Do not apply migrations through MCP or from an AI agent workflow. Do not present a migration as
safe because `migrate plan` parsed it. The plan is review input.

## Stale Manifest Behavior

When `goquent manifest verify` fails:

1. Stop using the manifest as authoritative context.
2. Do not generate confident database code from stale manifest data.
3. Regenerate the manifest or request a fresh manifest from the project workflow.
4. Re-run verification.
5. If the manifest remains stale, include the stale result in the PR and avoid claims of safety.

Do not suppress or ignore stale-manifest output to keep an AI workflow moving.

## Static Review Precision

Use these meanings consistently:

| Precision | Meaning | Required behavior |
| --- | --- | --- |
| `precise` | Goquent reconstructed the operation or plan shape enough for configured checks. | Review findings normally and still check business intent. |
| `partial` | Goquent extracted only part of the operation. | Do not claim safety. Add a QueryPlan test, manual SQL review, or simplify the query. |
| `unsupported` | Goquent could not analyze the pattern beyond fallback findings. | Do not claim safety. Provide manual evidence or rewrite into a supported pattern. |

`partial` or `unsupported` output is PR-relevant even when the command exits successfully under the selected threshold.

## MCP Usage

Use MCP only for deterministic context and read-only review tooling.

Allowed:

- read schema, manifest, model, relation, policy, migration, example, and review-rule resources
- inspect manifest status
- review query text without executing it
- review migration SQL without applying it
- compile read-only OperationSpec to a `QueryPlan`
- generate repository-method suggestions that still go through local tests and review
- use prompts as review guidance only; prompts do not write files, apply migrations, execute SQL, or approve destructive operations

Forbidden:

- DB writes through MCP
- migration apply through MCP
- raw SQL execution through MCP
- bypassing tenant, soft delete, PII, or required-filter policy through MCP
- treating MCP output as business approval

Use a narrow MCP surface when possible:

```bash
go run ./cmd/goquent mcp \
  --manifest goquent.manifest.json \
  --resource manifest \
  --resource manifest-status \
  --resource policies \
  --tool get_manifest \
  --tool get_manifest_status \
  --tool review_query \
  --tool review_migration \
  --tool compile_operation_spec
```

## Decision Tree

```text
Start
  |
  v
Does the change touch DB code, schema, policy, manifest, or DB-facing tests?
  |
  +-- no --> Use normal project workflow.
  |
  v
Run manifest verify with current schema/policy inputs.
  |
  +-- stale or failed --> Stop trusting the manifest. Regenerate or ask for a fresh one.
  |
  v
Is this a read query?
  |
  +-- yes --> Can it fit OperationSpec MVP?
  |             |
  |             +-- yes --> Use OperationSpec and compile to QueryPlan.
  |             +-- no  --> Use Goquent DSL. Use raw SQL only with documented reason.
  |
  v
Is this a write or migration?
  |
  +-- write --> Generate/review QueryPlan; require approval for high risk.
  +-- migration --> Run migrate plan and dry-run; require approval/preflight for high or destructive steps.
  |
  v
Run go test ./... and goquent review.
  |
  +-- partial/unsupported --> Include limitation and manual review evidence.
  |
  v
Attach review output to the PR.
```

## PR Output To Attach

Every PR that changes database access should include the relevant subset:

- `go test ./...` result
- `goquent manifest verify` result
- `goquent review` result
- `QueryPlan` output for changed repository methods or OperationSpecs
- `MigrationPlan` and dry-run output for migrations
- suppression reasons, owners, and expirations
- approval reasons for high or destructive operations
- `partial` or `unsupported` static review locations
- raw SQL reason and review result when raw SQL is used

Do not include secrets, production credentials, or sensitive parameter values in PR comments.

## Example Commands

These commands are verified against the checked-in AI-safe example and do not require a live database:

```bash
go test ./examples/ai-safe-orm
go run ./examples/ai-safe-orm
go run ./cmd/goquent manifest verify \
  --manifest ./examples/ai-safe-orm/goquent.manifest.json \
  --schema ./examples/ai-safe-orm/schema.json \
  --policy ./examples/ai-safe-orm/policies.json
go run ./cmd/goquent operation compile \
  --manifest ./examples/ai-safe-orm/goquent.manifest.json \
  --spec ./examples/ai-safe-orm/operation.json \
  --values ./examples/ai-safe-orm/values.json \
  --format json
go run ./cmd/goquent review --format pretty --fail-on blocked ./examples/ai-safe-orm
go run ./cmd/goquent migrate plan ./examples/ai-safe-orm/migrations/001_add_users.sql
go run ./cmd/goquent migrate plan ./examples/ai-safe-orm/migrations/002_drop_legacy_email.sql
go run ./cmd/goquent migrate dry-run \
  --approve "legacy column retired" \
  ./examples/ai-safe-orm/migrations/002_drop_legacy_email.sql
```

The example intentionally contains findings. Use `--fail-on blocked` for the walkthrough so output remains visible without failing on the intentional high or destructive findings.

## Pre-PR Checklist

- [ ] Manifest verification passed, or stale status is explicitly reported as untrusted.
- [ ] Tenant scope, soft delete, PII, and required filters were checked.
- [ ] `RiskLow` was not treated as business approval.
- [ ] `partial` and `unsupported` review output was not described as safe.
- [ ] Raw SQL was avoided or justified and reviewed.
- [ ] Migrations include plan, dry-run, approval reason, and preflight evidence when needed.
- [ ] Suppressions include reasons and expiration when appropriate.
- [ ] `go test ./...` was run.
- [ ] `goquent review` output is ready for the PR.
- [ ] MCP was used only for read-only context or review.

## Copyable Instruction Block

Use this in a system or developer prompt for AI coding agents working on Goquent projects:

```text
When writing database code with Goquent, treat Goquent as a deterministic database review boundary, not as business approval. Verify the manifest against current schema and policy inputs before using it; if it is stale, do not trust it or claim safety. Prefer read-only OperationSpec for supported select/filter/order/limit reads, otherwise use Goquent DSL. Compile or inspect QueryPlan output and review SQL, params, tables, predicates, risk, warnings, tenant scope, soft delete, PII, required filters, suppressions, approvals, and analysis precision. Never treat RiskLow as business approval. Never describe partial or unsupported static review as safe. Avoid raw SQL unless it is necessary, parameterized, policy-reviewed, and documented. For migrations, run migrate plan and dry-run, and require explicit approval plus preflight evidence for high or destructive changes. Use MCP only as read-only context and review tooling; do not use it for DB writes, migration apply, or raw SQL execution. Attach manifest verification, go test, goquent review, QueryPlan or MigrationPlan output, dry-run output, suppressions, approvals, raw SQL rationale, and precision limitations to PRs.
```
