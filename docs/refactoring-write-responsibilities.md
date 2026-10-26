# Generic write responsibility extraction

GQ-REF-01 / PR2, Refs #135. This same-package extraction changes source locations,
not APIs, behavior, dependencies or execution guarantees.

## Boundary and change entry points

Baseline: main `e8f0dc9b96f5d4e43bc8ba2d11482191fbff5841`, containing PR142
head `a4fb15a0fb162d20e3bbabefee7b5f5ddfcb13a8` (ancestor check passed).
Before changes, exact-main CI 40328876052, Security 40328876323 and Docs
40328876293 completed successfully. Docs build and publish succeeded; gh-pages
`87e5d887377b282e35698be3734bd8a1d31f2250` names this source SHA.
Pages settings returned HTTP 404; branch publication does not verify serving.

Previously, tracing effective Columns/Omit or PK selection crossed public options,
terminal methods and scattered single/batch mapping helpers in `write.go`.
Tracing RETURNING results crossed helpers there and dispatch/scanners interleaved
with diagnostic planning in `write_plan.go`. The two extracted files give those
changes distinct entry points without dismantling public write configuration.

| File | Responsibility and movement |
| --- | --- |
| `orm/write_mapping.go` | 23 functions from `write.go`: `buildInsertInput`, `buildInsertManyInput`, `buildUpsertManyInput`, `validateInsertManyOptions`, `insertManyStructColumnsAndArgs`, `upsertManyStructColumnsArgsAndPK`, `upsertStructColumnsAndArgs`, `insertStructColumnsAndArgs`, `insertManyMapColumnsAndArgs`, `upsertManyMapColumnsArgsAndPK`, `mapUpsertManyColumnSet`, `sortedPKColumns`, `mapInsertManyColumnSet`, `mapInsertManyRowArgs`, `sameColumns`, `buildUpdateInput`, `sortWriteFields`, `buildUpsertInput`, `conflictTargetColumns`, `upsertUpdateColumns`, `filterColumns`, `dedupeColumns`, `ensureUpsertUpdateColumnsPresent`. Owns field/row extraction, effective columns, PK/conflict selection and ordered inputs. |
| `orm/write_dispatch.go` | From `write.go`: `returningResult` and its two methods, `ensureReturningColumns`, `returningColumnsForQuery`, `checkRowsAffected`. From `write_plan.go`: `execWriteInput`, `execPreparedWrite`, `queryWriteOne`, `queryWriteAll`, `queryWriteOneWithOptions`. Owns facade result/scan adaptation, including full RETURNING stream counts. |
| `orm/write.go` | Retains all public write options, private option fields and aliases, public CRUD/RETURNING terminals, and the conditional InsertOnceReturning recipe. |
| `orm/write_plan.go` | Retains `newWriteInput`, `prepareWrite`, diagnostic adaptation and five public Plan entry points. No bridge or inspection redesign. |

The unchanged call paths are:

- Public terminal → ordered WriteOpt evaluation → mapping → `newWriteInput`
  (detached row maps, literal columns, assignments/conflict shape) → facade dispatch.
- Public Plan → the same mapping → `writeDiagnostic` → `prepareWrite` →
  `querybridge.Prepare`; returns diagnostics without executing SQL.
- `prepareWrite` supplies actual destination settings, owner, executor, dialect
  and context → `query.prepareGeneric` → shared insert/update planner → tenant,
  soft-delete, policy/finalization and final RETURNING inspection → private sealed
  execution closures. Facade dispatch calls these closures; it does not replace
  final binding, authorization, consumption, atomic ownership or cleanup checks.
- Struct extraction still uses `meta.go` and `model.TableName`. Field order,
  readonly/omitempty handling, nil/typed values, map sorting and PK precedence
  retain their existing operation-specific differences. No extra reflection or
  caller-payload mutation is introduced.
- Nested/compound preparation reuses mapping and `prepareWrite`; scope timing,
  parent/delete/child order, generated-ID handling and external transactions stay
  in their existing files. InsertOnceReturning retains its prechecked conditional
  lookup. Hook/idempotent recipes and typed repository adapters are unchanged.

Dependency direction remains facade → query/internal bridge, model and existing
helpers; query → internal builders/inspection/dispatch. No lower-level package
imports the facade. Query's bridge installation `init` and metadata cache remain
in place. See [planned execution](planned-query-execution.md),
[atomic results](atomic-write-results.md), [validated binding](validated-plan-binding.md)
and [external authorization](external-authorization.md) for their contracts.

## Equivalence and limits

A temporary Go AST inventory compared all **462 non-test declarations** in `orm`
by name and receiver (including generic receivers), independently of file/position.
Declaration sets, token streams and full declaration text including comments are
identical. Exactly **34 declarations** moved. Bodies, signatures, generic
constraints, nominal types, fields and comments are unchanged. Only required
imports and gofmt whitespace changed otherwise. No var or init declaration moved;
there is no new initialization-order dependency.

This is source-equivalence evidence, separate from runtime tests and static-review
identity. Source locations/tree fingerprints can change despite identical bodies;
unknown review comparisons must not be relabeled unchanged. No SQL snapshot,
baseline, threshold or manifest is regenerated. Existing tests exercise mapping,
plans, refusal, scoped/nested writes, RETURNING, atomic/version/idempotency,
tenant/binding/authorization, public views and typed repositories. No new behavior
boundary or missing regression was identified for this mechanical movement;
no file-layout tests or performance improvement claims are added.

Existing differences remain: generic literal columns versus Query path handling,
operation-specific Columns/Omit/PK rules, dialect conflict/affected-row behavior,
scoped option applicability, external transaction ownership, Strict opaque-recipe
refusal and Compatibility's possible earlier effects. This work adds no SQL/stdin/
aggregate budget, root/environment policy or stronger process guarantee. JSON
commonization and further Query restructuring are separate work.

Prior security audits retain Go1.26.10/local and scanner1.9.0, x/mod module-only2
and historical graph-only15; manifest stale/database skipped, native unknowns and
release blockers are not resolved here. Weekly schedule40270344441 success applies
only to its old source af00cda3, not this branch. No new security repair is mixed in.

## Executed validation

Actual local toolchain: Go **1.26.10 linux/amd64**, explicitly first in PATH,
`GOTOOLCHAIN=local`, `GOPROXY=off`, temporary writable GOCACHE. Default PATH/local
Go1.25.0 was observed but not used for validation. Raw logs and comparison trees
were captured in an owner-only temporary directory outside the checkout.

| Check | Result |
| --- | --- |
| `go test ./orm ./orm/query -count=1` before/after movement | Exit0 on both; baseline source tree matches main. |
| `go test -race ./orm ./orm/query -run 'Test(Insert\|Update\|Upsert\|Generic\|Plan\|Atomic\|Optimistic\|Compound\|Nested\|StaticNested\|Strict\|CompatibilityScope\|Returning\|SecurityAudit\|Authorization\|Tenant\|Binding\|PublicPlanView)' -count=1` | Exit0. |
| `make test-integration` | Exit0, MySQL8/PostgreSQL16 healthy with explicit fixture DSNs; `go test ./... -count=1`, 26 passing packages including public contracts, CLI/MCP, type analysis and typed repositories. Non-verbose full-suite output does not enumerate individual skips; no global zero-skip claim. |
| Explicit-DSN `go test ./tests -run '^Test(ExternalAuthorizationDatabaseAndExternalTx\|TenantPolicyDatabaseSemantics\|ValidatedBindingDatabaseAndExternalTx\|MySQLReplaceNestedCollectionTx\|PostgresInsertReturning\|PostgresUpdateReturning\|PostgresUpsertReturning)$' -count=1 -json` | Exit0, 13 test/subtest passes, zero skip/fail events. |
| `go vet ./...`; public-output wrapper regression; strict comparison-input regression | All exit0. |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit0; comparison only. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit1; actual1510 findings, suppressed list0, public display truncated. Not a clean SQL review. |
| `review --format review-ci-json --diff PATCH --diff-base-root BASE --fail-on high HEAD` | Exit1; full1510, new0, worsened0, unknown1510, unchanged0, matched0. Full report/risk_failed true, input_failed false. Diff modified2/added3; invalid/unlisted/read_failed0. No baseline supplied. |
| Example manifest verify with checked-in schema/policy | Exit1; schema/policy ok, generated_code stale, database skipped. Not regenerated. |
| Declaration/import comparison, gofmt, `git diff --check` | 462 declarations identical, 34 moved, all 35 package import paths unchanged; formatting/diff clean. |

Source comparison used materialized main and the changed tree with the initial
version of this document before this validation appendix; Go source/tests did not
change afterward. Physical source identity and whole-tree context are conservative:
1510 unknowns remain unknown. Declaration equality does not override review
classification, nor does a zero new count establish absence of defects. No
per-finding causal census or baseline migration was performed.

Two temporary tooling corrections were needed: the declaration utility initially
rejected generic receivers, before any movement; the first comparison patch lacked
Git headers (input_failed, invalid1). Generic receiver handling and a Git-generated
patch corrected those inputs. Neither failed attempt is counted as validation.

Sandbox DNS and Docker socket checks failed; narrowly approved diagnostics then
succeeded. Worker Git/API identity and repository push permission, plus the planned
branch push dry-run, succeeded independently of controller credentials. No matching
local/remote branch or PR in any state existed initially. Git branch creation
needed approved execution because `.git` is read-only in the sandbox. No approval
was denied. Main required-status-checks API returned “Branch not protected”/404;
this does not waive the requested exact-head CI/Security checks. Final commit,
actual draft PR and exact-head evidence are recorded in the PR/controller report.

Implementation is the finite extraction. Verification is the bounded evidence
above and final-head checks. Integration remains draft review, not main merge.
Release approval is absent. Five dispositions remain separate: no runtime defect
reproduced; unverified platforms/full-suite individual skips are missing evidence;
existing ownership/Strict/application trust boundaries are guarantee limits;
historical native/manifest/release unknowns retain their owners and closure
requirements; readiness/merge/release remain maintainer decisions. No Issue close,
ready transition, merge, release/tag/package publication or next work is included.
