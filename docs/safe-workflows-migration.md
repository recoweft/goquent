# Current safe workflows and migration

GQ-AI-14 PR1(1/2), [Issue #106](https://github.com/recoweft/goquent/issues/106).
This guide describes current bounded APIs. Staged validation is described in [the PR2 checklist](staged-release-validation.md); neither this guide nor successful tests adopts whole-project acceptance or
approves production release.

## Start without a connection

Run `go run ./examples/ai-safe-orm` and
`go test ./examples/ai-safe-orm -count=1` from the repository root. The example
creates a DB with a connection-free refusing executor, supplies fictional application schema and tenant
settings, generates a manifest, compiles a typed read and validates an explicitly
bound read. The default path calls no execution terminal. It prints public views
and a manifest summary, never reusable source JSON or binding keys.

`Plan` and `Validate*` do not connect, read live schema, run EXPLAIN or contact an
authorization provider. Inspect private source artifacts in an appropriately
restricted local review environment. Publish only `PublicView` projections and
fixed error classifications. Source `QueryPlan` JSON, SQL/params, manifest inputs,
Build/Dump/RawSQL and source error messages can contain sensitive material.
Public views cannot be converted into execution permissions or used to replace
complete private review/CI inputs. See [output boundaries](redacted-output.md).

## Move from Compatibility to conditional Strict

1. Inventory ordinary/generic/RETURNING, Raw, callback, nested and direct SQLDB
   paths. Compatibility retains historical heuristic checks and reason paths
   without an authorization provider. A reason or Approval is recorded intent,
   not business authorization. Do not infer stronger guarantees from a low risk.
2. Authenticate subject and tenant in the application. Construct an immutable
   `NewPolicySet`, `NewApplicationTenantContext` and `NewApplicationSchema` from
   trusted application configuration. Do not promote request JSON, a manifest
   Fresh result or a matching fingerprint into trusted context. Signed SQL
   integer tenant/conflict types and the declared plain-table/constraint inventory
   must fit the supported structural domain.
3. Use `NewSettings(...).WithTenantPolicy(database, schema, automatic)` and
   `orm.WithSettings(settings)` to opt into conditional Strict. This is an
   application assertion, not physical database attestation or live freshness.
   Existing DB/query/transaction copies retain their snapshots. Rebuild queries
   after settings changes; do not mutate globals to change an existing query.
4. Plan and inspect the supported structure. Missing/wrong tenant predicates,
   unsupported Raw/opaque expressions, unknown schema and protected projections
   refuse. Approval, suppression and provider allow cannot override a structural
   refusal. Automatic tenant predicates cover the base WHERE; JOIN ON and other
   documented limits still require explicit review.
5. For one of the six supported Query families, construct `NewBindingContext`
   using application-managed key (at least 32 bytes), scope, generation, target
   and dialect. Supply current trusted `BindingCurrent` and finite expiry to
   Validate and obtain current assertions again before ExecuteValidated. A stale
   application assertion cannot detect external schema/policy changes.
6. For high risk, register `NewAuthorization(provider, authenticatedSubject)`
   before constructing the query. The adapter must implement the application's
   external authority contract. The example's deny-only adapter and ephemeral
   fixture key demonstrate refusals; they are not authentication/key management.

There is no fallback from failed binding or Strict to ordinary CRUD. Compile-only
OperationSpec is read-only and does not create a ValidatedPlan. Tenant reads use
DB-scoped `CompileOperation` with the reserved `current_tenant` reference supplied
by the application, never a Values JSON tenant claim. Preserve the old JSON as a
negative fixture. See [typed operations](typed-operation-validation.md) and
[validated binding](validated-plan-binding.md).

## Authorization and execution limits

| Path | Current guarantee and refusal |
| --- | --- |
| ValidateSelect/Count/Insert/InsertBatch/Update/Delete | Provider0/DB0; structural refusal yields no usable candidate; high-risk pending candidate has a blocked diagnostic |
| Corresponding ExecuteValidated methods | CAS consumes first; current input checks precede high-risk provider call; private reconstruction and deadline checks follow it and precede dispatch; low risk calls provider zero times |
| Denial, mismatch, expiry, cancellation or DB failure | Candidate stays consumed; obtain a new candidate for any new attempt; no exactly-once commit guarantee |
| Ordinary/generic/scoped CRUD and RETURNING | High risk refuses in Strict and provider-registered Compatibility; no authorized RETURNING adapter |
| Compatibility with provider opt-in | High-risk ordinary calls refuse rather than use the legacy reason path; validated calls still need trusted schema/tenant/binding inputs |
| Raw, opaque callbacks, compound operations | No composite external authorization; existing refusals/preflights remain; Compatibility can have earlier effects before a later refusal |
| Direct SQLDB/sql.DB/sql.Tx/Executor | Outside interception; not Goquent-authorized |
| CLI/MCP and migrations | Read-only context/planning tools; migration deployment is a separate human-controlled boundary |

The trusted provider sees scope, subject, tenant, family, target, dialect and
expiry, not SQL or each assigned value. Private correspondence does not prove
external statement/value-specific review. No external authentication server,
network protocol, secret store or revocation service is implemented. Final checks
do not span an uncooperative executor or transaction commit. High-risk owned-Tx
AtomicWrite/OptimisticVersion paths refuse at their extra boundary. Consult the
[full authorization matrix](external-authorization.md) before migration.

## Finite evidence and release checkpoints

| Area | Evidence to collect | What remains separate |
| --- | --- | --- |
| Source compatibility | Fresh full/race/vet; v1 compatibility fixtures; actual repository regeneration check | Compile success is not execution permission; Row wrapper/type/fingerprint migrations require their documented source updates |
| Generated columns/patches | `manifest check-repositories --config tests/typedfixture/repositories.json`; typed update tests | Nominal column states and application UpdateSpec are distinct from read-only OperationSpec; no arbitrary expression/type coverage |
| Public output | Example subprocess refusal checks; CLI JSON/SARIF/MCP and CI canaries | Sensitive source artifacts stay local; do not publish arbitrary tool/driver errors |
| Schema/migration | Snapshot comparison, capability/coverage and preflight evidence tests | Unknown capabilities, live freshness, deployment approval and data-loss/locking decisions are not proved by parsing or Fresh |
| MySQL/PostgreSQL | Explicit dedicated fixture opt-in and three DSNs; sequential database suites; record versions, pass/fail/skip | sqlmock/spy results are not native DB results; fixture authority is not a production service |
| Binding/authorization | Six families, refusal, expiry, current-input changes and single-attempt tests | No global interception, external exactly-once or business correctness |
| Review | Ordinary review, exact clean nonempty base/head strict diff review, manifest verification separately | Preserve risk/precision failures and unknown/new counts; do not lower thresholds or bulk-adopt baselines to make a gate green |
| Performance/observation | Explicit performance plan and bounded event tests, separately identified native/mock measurements | No automatic EXPLAIN; restricted role/custom operator/cast/network faults remain unverified unless tested |

The original saved example manifest now explicitly declares schema, policy,
generated code and a database expectation for its string-tenant model. Use the
[example-owned inputs and verification commands](../examples/ai-safe-orm/storedmodel/README.md)
for four-axis supplied comparison. This deliberately extends the prior two-axis
scope; old measurements with optional skips remain historical evidence. The
handwritten executable's integer-tenant/datetime model stays separate. The legacy
manifest in `orm/manifest/testdata/legacy-example.json` remains a stale negative.
Do not regenerate it or adopt a review baseline to silence findings.
Neither supplied equality nor separately collected native owned derivatives prove
live freshness, physical DB identity, authorization or release acceptance.

For native validation use only the repository's dedicated disposable Compose
fixtures and the existing integration suite. Explicitly set `TEST_MYSQL_DSN`,
`TEST_DB_DSN` (the same MySQL fixture) and `TEST_POSTGRES_DSN`, then run
`go test -p 1 ./... -count=1`; run race separately. These tests create/drop fixture
tables and must never target production. The example itself has no DB execution
opt-in and does not read DSNs. Native execution demonstrations remain in
`tests/plan_binding_test.go` and `tests/external_authorization_test.go`.

Use the [staged validation runner and blocker ledger](staged-release-validation.md)
for fresh evidence collection after PR1.
The [release disposition and human decision matrix](release-blocker-disposition.md)
separate current measured findings, finite guarantees, unavailable historical
attribution and the adopted finite scope for future review; no release decision is adopted there.
Retain inherited unresolved evidence: intermittent testmain review unknown and
historical expiry failure causes are not established; previous successful reruns
are not explanations. Record tests that were skipped or not run, and separate
local/CI, native/mock and automatic success from human release decisions.

Related details: [typed patches](typed-update-patches.md),
[schema capabilities](schema-capabilities.md), [preflight](migration-preflight.md),
[performance](performance-plans.md), [observation](query-observation.md).
