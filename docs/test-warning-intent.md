# Test-origin high-risk warning intent and execution paths

GQ-MAINT-15 PR3; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
This finite audit accounts for every test-origin finding at high or above in the
complete source report below. It does not reduce warnings, exclude tests, change
analysis or execution contracts, or accept a release. **176 findings were reviewed;
no implementation repair candidate was established.** Source review and executed
controls are distinguished throughout; this is not a universal safety claim.

## Fixed source and measurement

The measured source is main `ae1a98821d01361d969367ef3d45c76add7639d0`, the
[PR145](https://github.com/recoweft/goquent/pull/145) merge. Its merged head is
`d5efb385db47132f4214d3a37b1859a6feef2eeb`. Remote main was independently resolved,
its ancestry checked, and exact-main
[CI](https://github.com/recoweft/goquent/actions/runs/40359714485),
[Security](https://github.com/recoweft/goquent/actions/runs/40359715151) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40359714471) succeeded before
repository edits. The worker checked cwd/origin/Ubuntu WSL2/clean state,
repository guidance and PR template, its own authenticated push permission, and
local/remote target-branch and all-state duplicate-PR absence. Sandboxed DNS and
Git metadata writes failed; approved GitHub reads, fetch and branch creation
succeeded. These observations do not verify a database environment.

The previous private archive matched all 641 committed files at
`c9fbf749b1cbc40ac4293e62160e9d3708dac36b`, with no extra files. However, its saved
CLI build information and saved `go version` output say **Go 1.25.0**, contrary to
the earlier provenance document's Go 1.26.10 attribution. The old census process's
exact toolchain/environment was not independently established from that evidence.
Consequently the old report was not adopted as the new population. A single fresh
complete `review.Run(Options{Paths: []string{"./..."}})` collection used an exact
archive of the new measured source and the existing review implementation. The
small external invocation only saves the complete report and runtime version;
it adds no source fixture, analysis rule, comparison or loader instrumentation.

The new collection completed successfully with **Go 1.26.10 linux/amd64**,
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOWORK=off`, `GOFLAGS=-mod=readonly`, the cached
pinned modules and a separate build cache. `go mod verify` succeeded. Test DSN and
native opt-in variables were removed for collection and local focused tests.
The retained archive, full report, per-finding correspondence, source evidence,
commands and raw logs are outside the repository and comparison roots in
owner-only storage (0700 directories, 0600 evidence), without upload.

Between the old and new measured commits only three Markdown documents changed.
The new complete finding array equals the old one after normalizing archive-root
spelling; source locations, codes, levels and evidence otherwise agree. This
explains the unchanged aggregate; 176 was not an expected count or a public
projection. No strict comparison or unknown-cause census was rerun. The historical
1,014 identity / 496 context partition remains evidence for its original pair,
not a newly measured comparison result.

| Origin | Medium | High | Destructive | Blocked | High or above |
| --- | ---: | ---: | ---: | ---: | ---: |
| Library/CLI | 83 | 9 | 0 | 1 | 10 |
| Example excluding tests | 13 | 0 | 1 | 0 | 1 |
| Test, including example tests | 1,227 | 144 | 28 | 4 | 176 |
| Total | 1,323 | 153 | 29 | 5 | 187 |

The complete report contains **1,510 findings, 1,194 operations, zero suppressed
findings**. Test high-or-above codes are RAW_SQL_USED 140,
DESTRUCTIVE_SQL_DETECTED 28, WEAK_PREDICATE four and DELETE_WITHOUT_WHERE four;
all 176 are partial. The other high-or-above findings are the previously reviewed
nine library raw calls, one library DELETE and one example MIGRATION_DROP_COLUMN.
They were counted but not re-audited. The complete remaining code totals are
STATIC_REVIEW_PARTIAL 839, STATIC_REVIEW_UNSUPPORTED 215, BULK_UPDATE_DETECTED 54,
BULK_DELETE_DETECTED 26, LIMIT_MISSING 112, SELECT_STAR_USED 76 and
MIGRATION_ADD_INDEX_NON_CONCURRENT one, all medium.

## Individual coverage and execution boundaries

The private correspondence covers **176/176 finding ordinals**, **148 distinct
file/line/column call sites**, **44 files** and **73 enclosing functions**, including
helpers and benchmarks. These are not 176 tests. Each entry retains its code,
call position, enclosing test/helper and variant source, individual purpose,
expected assertion and execution boundary. The 28 extra warnings are distinct
raw/destructive codes at the same call sites. The two tenant readbacks on one
line have distinct columns and check different tenants; they are not duplicates.
Shared helper uses and dialect/subtest variants are not multiplied into findings.

| Harness reached from the reviewed source | Findings | Evidence boundary |
| --- | ---: | --- |
| Live database fixture or native benchmark | 113 | Source reviewed; local live execution not selected. Includes calls intended to be refused after live setup |
| Mock, capture executor or synthetic database/sql driver | 54 | Includes refused callbacks, mixed plan/execution tests and one benchmark helper; focused controls below |
| Plan/review-only harness | 9 | No database dispatch; diagnostic, serialization, settings or risk assertions |

These categories describe the harness, not the number of successful SQL
statements. A mock can accept strings a real SQL server would reject. A static
warning does not establish that its call is reached, approved or dispatched.
Conversely, a test filename does not make real fixture destruction harmless.

The [typed tracker](static-review-types.md) recognizes declared database/sql,
Goquent raw and Query methods but does not reconstruct arbitrary control flow,
helper callbacks or dynamic SQL. Literal raw detection retains structural risk;
missing runtime policy and partial reconstruction are not promoted by this audit.
The [raw bridge](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/raw_execution.go) seals plans and enters the private
execution gate; [DB raw entry points](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/orm.go) use that bridge. Compatibility
approval reasons, Strict structural refusal and external authorization are
separate boundaries. Direct `SQLDB`, `sql.DB`, `sql.Tx` and `sql.Conn` fixture calls
bypass ORM approval/Strict/tenant gates entirely. Planning never grants permission
to execute its exported SQL through such a direct connection.

## Blocked DELETE and weak predicates

| Committed test contract | Finding count | What the source checks |
| --- | ---: | --- |
| [Observation policy/budget](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/observation_test.go) | 1 blocked | A predicate-free DELETE errors distinctly from budget refusal; one earlier approved mock write is the only dispatched/completed statement. SQL expectations and counters are checked |
| [Private execution ignores public verdicts](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/execution_test.go) | 1 blocked | Forged public plan fields and decoded JSON both fail with ErrBlockedOperation; capture calls remain zero |
| [Public view secret dispatch/refusal](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/public_view_test.go) | 1 blocked | After a positive mocked INSERT, mutating the blocked DELETE's public view cannot enable execution; reset spy counters stay zero, in both dialect variants |
| [Write-scope database semantics](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/write_scope_test.go) | 1 blocked | Branches install equality or singleton-IN predicates before PlanDelete. The tracker misses that state mutation. Source asserts at-most-one, explicitly executes exported SQL in a transaction, checks one affected row and rolls back for integer-width/value/dialect variants |
| [Approval required/provided](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/risk_test.go) | 3 weak | No reason: approval error and zero calls. Reason supplied: exactly one fake-executor call and its expected error. Planning control: reason retained while approval-required remains true |
| [Final dispatch/structural refusal](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/authorization_execution_test.go) | 1 weak | Raw-predicate validation variant returns no handle and an error, with provider and all executor counters zero |

The write-scope item is a current reconstruction limit, not evidence of a
predicate-free execution. Its live affected-row assertion was read, not rerun
locally. The mock refusal controls were run. None proves arbitrary predicates,
external executors or production DSNs safe.

## Destructive operations

Three destructive findings belong to intentional non-live controls:
[deprecated raw Row refusal](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/orm_raw_test.go),
[Row error identity](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/row_execution_test.go), and
[raw risk classification](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/risk_test.go). The first two inspect
refusal and empty capture logs; the third constructs a plan and asserts both raw
and destructive warning codes. No destructive server command is sent by them.

The other **25 destructive findings** are real fixture operations if their
surrounding tests run: table/trigger cleanup, shared-table truncation, fixture
reset, sequence alteration and temporary hook-table renaming. The source inventory
below covers them as well as the remaining raw warnings. Setup generally requires
a successful return; several deferred drops and the session-setting reset discard
the returned error. For example, [compound fixtures](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/compound_plan_test.go)
and [atomic integration](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/atomic_integration_test.go) do not establish
successful cleanup merely by registering a deferred call. This is a limit of
those assertions, not a reproduced cleanup failure. The hook-catalog test checks
both rename operations, the intended command error and unchanged business counts.

## Raw-call intent and fixture ownership

The following contracts summarize the individual private records. All references
are committed files whose bytes agree with the measured source; links remain
reviewable with this document's fixed commit above. Counts include co-located
higher-risk codes, so this table sums to 176, not just the raw code count.

| Source contract | Findings | Intent and boundary |
| --- | ---: | --- |
| [examples/ai-safe-orm/storedmodel/native_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/examples/ai-safe-orm/storedmodel/native_test.go) | 1 | Owned native cleanup positive control; exact disposable-fixture opt-in. |
| [orm/atomic_integration_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/atomic_integration_test.go) | 5 | Nontransactional/trigger refusal readback and table/trigger cleanup, under command fixture. |
| [orm/authorization_execution_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/authorization_execution_test.go) | 7 | Ordinary raw facade, clone/wrapper and risk-downgrade refusal; provider/executor zero effects. |
| [orm/command_hooks_integration_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/command_hooks_integration_test.go) | 5 | Rejected business-command readback and missing-hook rename/restore controls. |
| [orm/command_integration_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/command_integration_test.go) | 4 | Explicit-DSN command fixture version, creation and cleanup. |
| [orm/compound_execution_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/compound_execution_test.go) | 1 | Strict refuses effectful opaque scope before callback invocation. |
| [orm/generic_plan_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/generic_plan_test.go) | 2 | Strict generic raw-read wrappers refuse even with reason/table annotation. |
| [orm/migration/column_presence_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/migration/column_presence_test.go) | 2 | Mock nullable-token and row/scan-error propagation. |
| [orm/observation_baseline_bench_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/observation_baseline_bench_test.go) | 1 | Synthetic in-memory benchmark write helper shared by enabled/disabled modes. |
| [orm/observation_bench_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/observation_bench_test.go) | 1 | Explicit-DSN native benchmark scalar control; not executed here. |
| [orm/observation_integration_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/observation_integration_test.go) | 1 | Live version read preceding observation fixture. |
| [orm/observation_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/observation_test.go) | 12 | Mock row/handoff/transaction/error/budget/owner/result boundaries and policy refusal. |
| [orm/orm_raw_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/orm_raw_test.go) | 5 | Deprecated Row refusal/delegation and touched-table planning. |
| [orm/query/authorization_execution_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/authorization_execution_test.go) | 1 | Weak raw predicate fails structural validation before provider/dispatch. |
| [orm/query/execution_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/execution_test.go) | 1 | Private execution rejects public verdict and JSON tampering. |
| [orm/query/performance_rows_close_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/performance_rows_close_test.go) | 1 | Synthetic driver explicit/automatic row-close observation, distinct from native proof. |
| [orm/query/plan_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/plan_test.go) | 1 | Raw plan snapshot, parameter and diagnostic serialization. |
| [orm/query/public_view_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/public_view_test.go) | 1 | Public view cannot change sealed blocked execution. |
| [orm/query/risk_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/risk_test.go) | 6 | Weak predicate approval boundaries and raw/destructive plan classification. |
| [orm/query/settings_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/query/settings_test.go) | 1 | Opaque engine refusal with explicit built-in settings control. |
| [orm/review/review_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/review/review_test.go) | 1 | Serialized raw-plan review evidence and high-risk gate. |
| [orm/row_execution_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/row_execution_test.go) | 5 | Row refusal identity/cancellation and approved delegation outcome matrix. |
| [orm/settings_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/orm/settings_test.go) | 4 | Local settings snapshots, unsupported engine and high/blocked raw delegation. |
| [tests/compound_plan_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/compound_plan_test.go) | 12 | Live recipe fixture creation/cleanup, non-unit ID allocation and reference correspondence. |
| [tests/contracts/compatibility_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/contracts/compatibility_test.go) | 1 | Mock caller-owned transaction remains usable after wrapper Close. |
| [tests/custom_driver_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/custom_driver_test.go) | 4 | Registered-driver setup/create/truncate/seed with explicit custom DSN. |
| [tests/generic_plan_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/generic_plan_test.go) | 1 | Live generic semantics version metadata after rollback checks. |
| [tests/orm_postgres_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/orm_postgres_test.go) | 11 | Shared PostgreSQL setup/reset/seed and custom-primary-key fixture. |
| [tests/orm_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/orm_test.go) | 8 | Shared MySQL setup/reset/seed for API test consumers. |
| [tests/performance_boundaries_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/performance_boundaries_test.go) | 2 | Restricted-principal effective privilege reads in opted-in disposable fixture. |
| [tests/performance_pq_mechanism_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/performance_pq_mechanism_test.go) | 2 | Opted-in cancellation mechanism PID and cleanup controls. |
| [tests/predicate_tree_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/predicate_tree_test.go) | 5 | Live predicate result-set fixture creation/reset/seed/cleanup. |
| [tests/preflight_live_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/preflight_live_test.go) | 1 | Fixture lock release around bounded collector timeout negative control. |
| [tests/raw_sql_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/raw_sql_test.go) | 11 | Approved live raw read/write, unapproved and canceled entry points. |
| [tests/scanner_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/scanner_test.go) | 10 | Ten sqlmock result-mapping cases; synthetic connection-field values never connect. |
| [tests/schema_capabilities_live_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/schema_capabilities_live_test.go) | 1 | Catalog absence after PostgreSQL DDL rollback. |
| [tests/schema_diff_live_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/schema_diff_live_test.go) | 1 | Version read for explicit-DSN schema export fixture. |
| [tests/settings_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/settings_test.go) | 1 | Version read for live settings isolation/transaction fixture. |
| [tests/tenant_policy_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/tenant_policy_test.go) | 1 | Version read for live tenant-policy fixture. |
| [tests/typed_operation_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/typed_operation_test.go) | 10 | Numeric storage/zero-limit and driver integer-extreme reset/cleanup experiments. |
| [tests/typed_patch_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/typed_patch_test.go) | 6 | Patch reset/seed, per-state readback and caller-owned rollback verification. |
| [tests/typed_repository_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/typed_repository_test.go) | 3 | Generated repository typed read fixture seed/cleanup. |
| [tests/typed_tenant_patch_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/typed_tenant_patch_test.go) | 6 | Tenant patch setup and separate own/other tenant readbacks. |
| [tests/write_scope_test.go](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/write_scope_test.go) | 10 | Branch-built DELETE, numeric coercion and deferred-unique fixture boundaries. |

### Connection selection is a precondition, not a safety theorem

[lookupTestDSN/openTestDB](https://github.com/recoweft/goquent/blob/ae1a98821d01361d969367ef3d45c76add7639d0/tests/testenv.go) select a nonempty per-dialect
environment value before the local test fallback. Only unavailable connections
with an implicit fallback can skip there; explicit failures fail. The helper does
not validate that an explicitly supplied destination is disposable. Shared
`setupDB`/`setupPgDB` then create/reset/seed fixed tables, so their many consumers
inherit that real mutation before their own assertions. An assertion checking a
refusal later in the test does not undo setup. No production connection was
observed in this audit, and no claim is made that one was used historically.
The source does show that an arbitrary environment value is accepted; a variable
named TEST is not a physical database-identity check.

`setupCustomDB` separately uses TEST_DB_DSN, skips when absent, and can skip on its
direct network-error check. Command/observation, schema and preflight fixtures
require explicit dialect DSNs but do not equate nonempty input with verified
isolation. Their direct SQL setup has no ORM approval boundary. In contrast,
stored-model native and performance boundary/mechanism fixtures require the
explicit disposable opt-in and all three exact fixture DSN values. Even that
string equality is a fixture applicability condition, not proof of server
ownership or freshness. These distinctions come from the committed guards;
no skip result is being promoted to successful execution.

All actual local DB-dependent trials were **not run**. There was no demonstrated
expected/actual mismatch requiring a live reproduction, and no explicit isolated
fixture destination was established for this audit. Existing hosted CI success
is separate integration evidence, not a new per-item local live measurement.
The in-memory benchmark helper was source-reviewed but not benchmarked; the
native benchmark was neither run nor timed.

### What the assertions do and do not establish

Approved raw integration tests check inserts/readbacks, updates/readbacks and
query entry success. Their unapproved/canceled cases assert error identities,
without a dispatch counter at those live call sites. The separately run capture
controls assert zero dispatch; do not silently substitute that stronger assertion
into the live tests. Some read tests check returned content, others merely check
query success and close rows. For instance the legacy raw Query test checks a
nonzero iteration count without a final Rows.Err assertion. These are bounded
contracts, not proof of all possible result or cleanup failures.

The typed-operation and write-scope experiments deliberately execute exported
plans through database/sql to compare storage/row counts. That is explicit test
harness execution, not execution by CompileOperation or PlanDelete, and not
proof that public plan fields are reusable authority. Typed patch tests separately
check plan dispatch zero, permitted update results, rejected updates retaining
call counts, other-tenant preservation and external rollback. Synthetic scanner
records carrying connection-looking fields remain data in sqlmock rows.

## Validation and disposition

The private ledger records exact commands, exits and selected test names. Focused
existing controls cover the audited non-live test functions, including the blocked
DELETE and weak-predicate refusal/approval cases. No new regression test mirrors
the implementation. Full/race/both-DB/native suites, loader diagnostics, strict
comparison and unknown census were not rerun locally. No manifest inputs or
migration execution paths changed; the previous supplied-manifest result is not
recast as fresh live evidence. Handwritten documentation needs link/whitespace
checks, not unrelated API regeneration.

| Local check | Actual result |
| --- | --- |
| Complete source collection through existing review API | Exit 0, complete; runtime Go 1.26.10 |
| Pinned dependency verification | Exit 0, all modules verified |
| Focused existing controls, six packages | Each exit 0; 80 leaf and five parent pass events, zero fail/skip |
| Initial CLI archive build | Exit 1 obtaining VCS status; archive build then explicitly disabled VCS stamping |
| CLI archive build with `-buildvcs=false` | Exit 0; source identity retained separately, no fabricated VCS stamp |
| Built ordinary review at high threshold | Exit 1; full report, risk failed, input/manifest did not fail, precision gate not selected; comparison not selected |
| Public-output boundary checker | Exit 0 |
| Existing strict-input regression checker | Exit 0; this is not a new strict source census |
| Documentation links and diff whitespace | Checked on the documentation change; final command evidence in the PR/controller ledger |
| Exact final PR-head CI/Security | Required separately after commit/push; remote result retained in the PR/controller ledger |

| Conclusion | Finite result |
| --- | --- |
| Repair necessary | **0 confirmed candidates**; no supported-contract expected/actual mismatch established in this population |
| Intentional verification | Refusal/approval, private-seal, risk-classification, mock/scanner and fixture setup/readback experiments have individual source explanations |
| Guarantee limits | Direct fixture SQL bypasses ORM gates; explicit DSN does not verify isolation; unchecked cleanup and limited assertions do not prove success; dynamic predicate reconstruction remains partial |
| Information insufficient | Live execution/cleanup and server identity for this audit, benchmark results, arbitrary executor behavior, old census process environment and historical/native causal unknowns |
| Coverage | 176/176 findings have source evidence and execution status; zero source-unreviewed findings. This does not mean 176 live executions passed |

All seven [release blockers](release-blocker-disposition.md), native residuals,
historical unknowns and finite saved-manifest limits remain. This finishes the
commissioned finite source review together with the eleven non-test paths in the
[earlier audit](review-warning-provenance.md). Warning counts alone do not call for
another cleanup task. Comparison identity/context design and release acceptance
remain separate controller decisions. Delivery stops at a draft PR and report.
