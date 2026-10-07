# Goquent

[![Docs](https://img.shields.io/badge/docs-API-blue.svg)](https://recoweft.github.io/goquent/)

Goquent is an AI-safe ORM for Go. It helps humans and AI coding agents turn database operations into deterministic, reviewable artifacts: `QueryPlan`s, policy warnings, migration plans, manifests, and CI review output.

Goquent is not an ORM that asks AI to execute database work. It is an ORM and query-builder layer that makes AI-generated database code easier to inspect, constrain, and approve before it reaches production paths.

## Why Goquent Exists

AI coding agents are increasingly writing repository methods, query-builder chains, raw SQL, and migrations. The main risk is not that an agent cannot produce SQL; it is that generated database code can be hard to verify when SQL shape, parameters, policies, and migration effects are scattered across code.

Goquent keeps SQL visible. A database operation can be planned before execution, reviewed by humans, checked in CI, and handed back to an AI agent as structured feedback. The goal is a deterministic safety boundary around database code, not autonomous database access.

## What Goquent Provides

| Artifact | What it gives reviewers | Boundary |
| --- | --- | --- |
| `QueryPlan` | SQL, params, operation type, tables, columns, predicates, risk, warnings, approval state, and analysis precision | Describes database operation shape before execution |
| `RiskEngine` | Machine-readable warnings and risk levels such as `low`, `medium`, `high`, `destructive`, and `blocked` | Classifies structural database risk, not business correctness |
| `Policy` | Tenant scope, soft delete, PII, and required-filter checks | Makes application-specific data boundaries explicit |
| `goquent review` | CI-friendly review for Go source, raw SQL files, `QueryPlan` JSON, and `MigrationPlan` JSON | Reports precise, partial, or unsupported static analysis |
| `MigrationPlan` | Parsed migration steps, destructive-operation warnings, approval requirements, and preflight suggestions | Reviews schema change shape before apply |
| `Manifest` | AI-readable schema, policy, relation, example, and fingerprint context | Detects stale schema or policy context |
| `OperationSpec` | A narrow read-only JSON interface for single-model `select` operations with explicit fields, filters, ordering, and limit | Lets AI express supported reads without inventing free-form SQL |
| MCP server | Read-only redacted structural summaries, review and planning context for AI tools | Does not perform DB writes, raw SQL execution, or migration apply |

Compared with a conventional ORM or query builder, Goquent's differentiator is not hiding SQL. It is making SQL and database intent plan-first, reviewable, policy-aware, and suitable for AI-assisted code review.

## Trust Boundary

Use Goquent as a database safety boundary, not as an approval system.

- `RiskLow` means the database operation shape is low structural risk. It does not mean the operation is business-approved, authorized, or correct.
- A passing `goquent review` means no configured finding at or above the selected threshold was detected. It does not prove business correctness.
- Static review can be `precise`, `partial`, or `unsupported`. `partial` and `unsupported` output must not be described as safe.
- A stale manifest must not be trusted for schema, policy, PII, tenant, relation, or migration decisions.
- AI agents must not treat Goquent warnings as the only approval source. Human review and business context remain required.
- Goquent does not grant AI agents authority to operate a production database autonomously.

## Quick Start

The repository includes a runnable AI-safe ORM example that does not require a live database.

```bash
go test ./...
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
go run ./cmd/goquent migrate plan ./examples/ai-safe-orm/migrations/002_drop_legacy_email.sql
go run ./cmd/goquent migrate dry-run \
  --approve "legacy column retired" \
  ./examples/ai-safe-orm/migrations/002_drop_legacy_email.sql
```

The example intentionally contains medium, high, and destructive findings so the review output is visible. The walkthrough uses `--fail-on blocked` to keep the command runnable. In a project CI gate, use the threshold your team requires, commonly `--fail-on high`.

## Basic ORM Usage

Goquent supports MySQL and PostgreSQL through a small Go ORM and query-builder API.

```go
db, err := orm.OpenWithDriver(orm.MySQL, "root:password@tcp(localhost:3306)/testdb?parseTime=true")
if err != nil {
    return err
}

ctx := context.Background()

plan, err := db.Table("users").
    Select("id", "email").
    Where("tenant_id", tenantID).
    OrderBy("id", "asc").
    Limit(100).
    Plan(ctx)
if err != nil {
    return err
}

users, err := orm.SelectAll[User](ctx, db, plan.SQL, plan.Params...)
```

For CRUD helpers, scanning behavior, transactions, bool compatibility, and driver details, see the [ORM package API](docs/orm/README.md) and [generic CRUD guide](docs/orm/generic-crud.md).

## Human Workflow

When adding or changing database code:

1. Write the repository method using the local Goquent DSL or generic helper style.
2. Generate or inspect a `QueryPlan` for the final SQL shape.
3. Check warnings for tenant scope, soft delete, PII, required filters, broad writes, raw SQL, and missing limits.
4. Prefer fixing the query over suppressing warnings.
5. If a suppression is necessary, include a reason, owner, and expiration when possible.
6. If a high or destructive operation is intentional, add an explicit approval reason.
7. Run tests and `goquent review`; attach the relevant output to the PR.

Typical CI review command:

```bash
go run ./cmd/goquent review \
  --format github \
  --fail-on high \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --require-fresh-manifest \
  ./...
```

PRs that touch database code should include the `go test ./...` result, `goquent review` output, relevant `QueryPlan` or `MigrationPlan` output, manifest verification, suppressions or approval reasons, and any `partial` or `unsupported` static review entries.

## AI Agent Workflow

AI coding agents should use Goquent as a review boundary:

1. Verify the manifest with the current schema and policy inputs.
2. Use `OperationSpec` for supported read-only, single-model `select` operations when it fits; otherwise use Goquent DSL.
3. Compile and test the Go code.
4. Generate or inspect the `QueryPlan`.
5. Run `goquent review`.
6. Attach review output to the PR.
7. If review is `partial` or `unsupported`, report the limitation and add manual review evidence. Do not claim the query is safe.

Read the [AI agent playbook](docs/ai-agent-playbook.md) before asking an agent to write repository methods, raw SQL, or migrations.

## Migration Workflow

Migrations are a high-risk area for AI-generated code. Treat `MigrationPlan` as the review artifact before any apply path.

```bash
go run ./cmd/goquent migrate plan ./migrations/001_change.sql
go run ./cmd/goquent migrate dry-run ./migrations/001_change.sql
go run ./cmd/goquent migrate dry-run \
  --approve "documented reason for risky schema change" \
  ./migrations/001_change.sql
```

`migrate dry-run` validates the plan without executing SQL. Destructive or high-risk changes require explicit approval before dry-run or apply can pass. The public plan omits SQL, names and free-form preflight text; inspect internal migration data locally for deployment review.

Migration application is a human-controlled deployment step. AI agents and MCP tools must not run `goquent migrate apply`. Before a human applies a migration, include `goquent migrate plan`, dry-run output, approval reason, and preflight notes in the PR.

## Manifest and Stale Detection

The internal manifest gives compilation and review commands schema and policy context.
Default public output is a redacted summary. To create a sensitive local artifact
use a new private file explicitly (refused in CI); do not publish that file.

```bash
go run ./cmd/goquent manifest --format json \
  --schema schema.json \
  --policy policies.json \
  --unsafe-local-output goquent.manifest.json

go run ./cmd/goquent manifest verify \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json
```

Use `review --require-fresh-manifest` with current inputs such as `--schema`, `--policy`, `--code`,
or `--database-schema` when stale or unverified schema or policy context should fail CI. A stale
manifest is not a warning to ignore; regenerate it or stop using it as authoritative context.

## MCP Server

The current MCP server is read-only for AI editors and coding agents.

```bash
go run ./cmd/goquent mcp \
  --manifest ./examples/ai-safe-orm/goquent.manifest.json \
  --resource manifest \
  --resource manifest-status \
  --tool get_manifest \
  --tool review_query \
  --tool compile_operation_spec
```

MCP is for schema, policy, manifest, review-rule, and planning context. It can review query text or migration SQL without executing it. It does not perform DB writes, migration apply, or raw SQL execution.

## Documentation

- [Public API inspection inventory and current/future contracts](specs/goquent_ai_contracts_v3.md)
- [Documentation index](docs/index.md)
- [AI agent playbook](docs/ai-agent-playbook.md)
- [AI-safe ORM example](examples/ai-safe-orm)
- [QueryPlan guide](docs/query-plan.md)
- [Risk engine guide](docs/risk-engine.md)
- [Policy DSL guide](docs/policy-dsl.md)
- [Review CLI guide](docs/review-cli.md)
- [MigrationPlan guide](docs/migration-plan.md)
- [Manifest guide](docs/manifest.md)
- [Manifest stale detection](docs/manifest-stale-detection.md)
- [OperationSpec guide](docs/operation-spec.md)
- [MCP server guide](docs/mcp.md)
- [Suppression and approval](docs/suppression-and-approval.md)
- [Static review limits](docs/static-review-limits.md)

## Development

Run the unit test suite:

```bash
go test ./...
```

Run the integration suite with local MySQL and PostgreSQL containers:

```bash
make test-integration
```

The integration tests create the required tables. Override `TEST_MYSQL_DSN` and `TEST_POSTGRES_DSN` when needed.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

### Compound inspection and Row migration

See the [current execution inventory](docs/planned-query-execution.md) for private
compound preflight, Strict unsupported callbacks/scopes, and compatibility partial
execution. DB.QueryRow/QueryRowContext now return *orm.Row; Scan/Err chaining remains
available. Explicit *sql.Row consumers and interfaces must use QueryRowE and check
its error first. The Executor interface is unchanged, but *orm.DB no longer itself
implements it. SQLDB/direct Executor bypasses are not safe migration substitutes.

### Plan JSON migration

QueryPlan, ReviewReport, TablePolicy and OperationSpec now write integer
`version: 1`. Missing/0 reads remain legacy input; unsupported versions fail.
Decoded `any` numbers use `json.Number` instead of `float64`. Use fresh receivers
for external JSON and discard them on error. See [versions and private identity](docs/plan-version-identity.md)
for migration and limits. Public JSON is diagnostic, never an execution permit;
GQ-AI-05 PR2 adds opt-in [validated plan binding](docs/validated-plan-binding.md)
for Query Select/Count/Insert/InsertBatch/Update/Delete with explicit current
settings and key context. Generic/scoped/Raw/compound adapters are not included;
ordinary CRUD keeps its existing behavior.

## GQ-AI-06 PR1 public views

Additive `query.PlanView` and `review.ReportView` now provide detached, bounded,
non-executable output with fixed classification and strict independent v1 readers.
SQL, identifiers, values and opaque Evidence/Metadata are omitted without evaluating
custom value methods. See [public view APIs and output migration](docs/public-plan-views.md).
At the PR1 boundary, legacy outputs remained sensitive. PR2 now changes display
consumers; source JSON data APIs remain sensitive as documented below. Public views are not execution, approval or CI decision inputs; existing
private execution, typed binding, Strict and wire compatibility remain in force.

## GQ-AI-06 PR2 output migration

Default CLI/CI/MCP output and display-only String/Pretty/GitHub APIs now use
bounded public views and fixed error adapters. See [public output inventory and migration](docs/redacted-output.md).
Legacy source JSON, Query Build/Dump/RawSQL and internal artifact APIs retain
sensitive execution data and wire compatibility; they are not publication APIs.
Default manifest/schema JSON is a non-reusable summary; MCP schema-name discovery
is intentionally removed. Explicit new local artifact files and validated
JSON-RPC top-level correlation ID echoes are documented AC exceptions. No source
error or opaque value is retained in public errors. Public output is neither
execution material nor authorization. Original complete reports/plans decide CI,
freshness and apply gates; truncation cannot weaken those decisions. Whole-Issue
acceptance and merge remain separate from implementation and test evidence.
