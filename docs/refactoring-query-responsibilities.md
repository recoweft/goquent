# Query responsibility extraction

GQ-REF-01 / PR1, Refs #135. This is a same-package source reorganization,
without new API, guarantees, dependencies or execution behavior.

## Boundary and change entry points

Baseline: main `8f84f64c3150dedbd5c82d061e429764950a560a`, containing PR141.
Before editing, its CI 40321533476, Security 40321533436 and Docs 40321533415
completed successfully. Docs build/publish succeeded and gh-pages
`861688cf416db955b8b16e3320ac972e8a7ed746` names that source SHA.
The Pages settings API returned 404; branch publication does not verify serving.

`query.go` combines construction/private state, DSL mutation, input validation,
plan finalization and terminal adapters. Two concrete tracing problems motivate
this bounded extraction: cursor constructors, predicate generation and validation
were separated by unrelated execution/DSL methods; write plan entry points were
interleaved with dispatch adapters. Readers changing cursor ordering or tracing a
write refusal had to cross those unrelated responsibilities.

| Entry point after extraction | Moved from query.go | Role and dependencies |
| --- | --- | --- |
| `cursor.go` | `CursorColumn`; `CursorAsc`, `CursorDesc`, `CursorAscExpr`, `CursorDescExpr`, `CursorAscAlias`, `CursorDescAlias`; `WhereCursorAfter`, `WhereCursorBefore`, `whereCursor`, `cursorPredicate`, `cursorColumnSQL`, `validateCursorColumns`, `cursorComparisonOperator` | Keyset DSL construction and its validation/lexicographic SQL. Uses existing shared fragment/column/order validators and identifier quoting; feeds the private select builder. |
| `write_plan.go` | `dataToMap`; `PlanInsert`, `PlanInsertBatch`, `planInsertOrIgnore`, `planUpsert`, `planUpdateOrInsert`, `planInsertUsing`, `PlanUpdate`, `PlanDelete` | Nonexecuting Query write-plan entry points and struct/map adaptation. Delegates to existing generic planners or builds the existing snapshot/finalizer path. |

The extraction moves 22 functions and one type. All remain in `orm/query`.
Shared validators, builder construction/copy helpers, Query state, finalization,
other DSL methods and terminal adapters stay in `query.go`. This is not a target
line count or a requirement to dismantle that file.

Major paths remain:

- `New` → `NewWithSettings` → dialect builder and immutable settings;
  cursor DSL → validation → predicate construction → builder `WhereRaw`.
- `First`/`Get` → `Plan` → `planSelectBuilder` in `plan.go` → policy builder,
  snapshot and `finalizePlan` → existing execution adapter.
- `Insert`/`Update` → moved Plan entry → `planInsertRows`/`planUpdateValues`
  in `generic.go` → tenant/write inspection, snapshot and finalization.
  `Delete` → moved `PlanDelete` → policy/copy/delete snapshot/finalization.
- Terminal adapters → `execution.go` and write-result handling. Validated
  binding and external authorization retain their existing independent rechecks
  and consumption paths; they are not redesigned by this extraction.

The facade depends on query through its existing bridge; query depends on internal
builders, value/tenant/write helpers and driver abstractions. No lower-level
package acquires a facade dependency. Future cursor construction changes start in
`cursor.go`; write-plan entry adaptation starts in `write_plan.go`; shared generic
planning stays in `generic.go`, dispatch in `execution.go`, and authority in the
existing binding/authorization files. See [planned execution](planned-query-execution.md)
and [validated binding](validated-plan-binding.md) for the contracts.

## Equivalence and source review

A temporary Go AST inventory compared every non-test declaration in `orm/query`
by name/receiver (init declarations by token content), independently of path and
position. Before/after declaration sets, token streams and complete declaration
text including comments matched for all **610 declarations**. Exactly 23 declarations moved
from query.go into the two files above. No function body, signature, generic
constraint, type field, documentation comment or private seal changed. The only
Go edits outside movement are required imports and gofmt whitespace. Package
import dependencies are unchanged.

This check is runtime-source equivalence evidence, not static-review identity
correspondence. Moving source changes physical locations, whole-tree context and
potential code fingerprints. Unknown comparison results are not reclassified as
unchanged. Review and manifest results are recorded below; no baseline, snapshot,
gate or manifest is regenerated to make them pass.

Existing cursor/DSL, planning, typed-value/ownership, CRUD/RETURNING, authorization,
tenant, binding, compound, public-view and clone regressions are reused. No new
behavioral boundary is introduced, so no file-layout test is added. Pure function
movement does not justify a performance improvement claim or benchmark sweep.

## Limits

PR2 facade write.go extraction and PR3 JSON work are separate. This PR introduces
no framework, new runtime guarantee, API/wire change, gate relaxation or security
repair. The finite SEC audits remain finite: Go1.26.10/local, scanner1.9.0,
x/mod module-only2 and historical graph-only15 remain recorded inventory. Prior
SQL review1510/exit1 is a historical observation, not an expected fixed count.
Manifest generated_code stale/database skipped, native unknowns and release
blockers retain their existing owners and closure requirements. The controller's
historical weekly schedule success is not evidence for this source.

## Executed validation

Local commands used Go **1.26.10 linux/amd64**, explicitly placed first in PATH,
`GOTOOLCHAIN=local`, `GOPROXY=off` and a writable temporary GOCACHE. Default PATH
with local selection was Go1.25.0 and was not used for these checks. Raw logs and
comparison trees stayed in an owner-only temporary directory outside the checkout.

| Command/check | Observed result |
| --- | --- |
| `go test ./orm/query ./orm -count=1` | Exit0. |
| `go test -race ./orm/query ./orm -run 'Test(WhereCursor\|Plan\|PlannedExecution\|Generic\|Authorization\|Tenant\|Binding\|SecurityAudit\|PublicPlanView\|Compound\|Strict\|Clone)' -count=1` | Exit0. |
| `make test-integration` | Exit0: MySQL8/PostgreSQL16 explicit fixture DSNs; `go test ./... -count=1`, 26 passing packages, including public contracts, CLI/MCP and type analysis. Non-verbose suite does not enumerate individual skips; benchmark package has no tests to run. |
| Explicit-DSN `go test ./tests -run '^Test(ExternalAuthorizationDatabaseAndExternalTx\|TenantPolicyDatabaseSemantics\|ValidatedBindingDatabaseAndExternalTx)$' -count=1 -json` | Exit0; 9 test/subtest passes (3 parents and both dialects), zero skip/fail events. |
| `go vet ./...`; `sh scripts/test-public-output-check.sh`; `python3 scripts/release_validation/check_strict_inputs.py` | All exit0. |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit0, comparison only. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit1, actual 1510 findings, suppressed list0, truncated public display. Not a clean SQL review. |
| `review --format review-ci-json --diff PATCH --diff-base-root BASE --fail-on high HEAD` on materialized trees | Exit1; full1510, new8, unknown1502, worsened0, unchanged0, matched0; input_failed=false, full_report=true, risk_failed=true. Diff modified1/added3, unlisted/invalid/read_failed0; no baseline supplied. |
| Example manifest verification with checked-in schema/policy | Exit1: schema/policy ok, generated_code stale, database skipped. No regeneration. |
| gofmt; `git diff --check`; package production import-set comparison | Clean; all 37 imported package paths unchanged. |

Comparison used the full source and initial version of this document before this
validation appendix. Production code and tests did not change afterward. The
comparator uses physical source keys and conservative whole-tree context; the
8 new classifications do not by themselves establish new runtime defects, and
1502 unknowns remain unknown. No per-finding causal census or identity override
was performed. Declaration equality is a separate result, not a substitute for
review classification. Final commit and exact-head CI/Security are recorded in
the PR/controller report.

Sandbox DNS and Docker socket diagnostics failed; narrowly approved Git/API and
Docker diagnostics succeeded, followed by separately approved branch operations
and integration tests. Worker authentication, push permission and planned-branch
push dry-run succeeded independently of controller credentials. No matching
remote branch or PR in any state existed before creation. No approval was denied.
The first temporary declaration utility rejected duplicate init names; using
token content for init identity fixed the utility before any source movement.
Its initial default-cache build also failed; writable temporary GOCACHE resolved
that environment constraint. Neither was a product/test failure.

Implementation is this finite mechanical extraction. Verification is the
bounded evidence above plus final-head checks, with stated limits. Integration
is draft review only; release approval is absent. Five dispositions stay separate:
no implementation defect reproduced; unverified platforms/individual full-suite
skips are missing evidence; existing trust/Strict boundaries are guarantee limits;
historical native/manifest/release unknowns retain their prior owners; readiness,
merge and release remain maintainer decisions. No Issue closure, PR2/PR3 work,
release/tag/package publication or next-task start is included.
