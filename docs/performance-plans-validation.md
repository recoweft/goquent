# GQ-AI-12 PR1 validation

Management ID: GQ-AI-12, PR1 (1/2),
[Issue #94](https://github.com/recoweft/goquent/issues/94).
The API, supported domain and trust assumptions are in
[performance plans](performance-plans.md). Actual PR/head and final-head CI are
recorded in the PR and Issue comment; no GitHub number is inferred here.

## Identity and preconditions

This execution is work `965407a3-6c27-4a12-9442-666e24d59d75`, revision 1,
execution `a385ec11-5e8c-4e7c-b179-dd08ab062b7f`. Worker checked actual cwd
`/home/murai/github/goquent`, Ubuntu 24.04.5/WSL2, origin fetch/push
`git@github.com:recoweft/goquent.git`, and an initially clean tree. The previous
feature checkout was preserved; its tree equaled main. The requested branch was
created from main `68987157de1e9cc7253f809214ab20efe4a5fae6`.

Remote main matched that SHA on initial and subsequent direct checks. Worker
independently confirmed PR67/68/69/74/75 MERGED and all five merge commits as main
ancestors. No revert-subject entry or Issue cancellation was found; this is not
proof against every possible semantic reversal. Issue #94 was open with the
supplied specification and no comments. Paginated all-state Issue inspection found
only #94 for GQ-AI-12; all-state requested-head PR lookup and local/remote branch
lookup found no existing PR/branch before creation. No alternate work was selected.

Exact main [CI 39053595221](https://github.com/recoweft/goquent/actions/runs/39053595221)
and [Docs 39053595325](https://github.com/recoweft/goquent/actions/runs/39053595325)
were directly verified completed/success on that exact SHA. These are start gates,
not the new branch's test evidence. Worker gh authentication, repository push/admin
permissions and SSH authentication as recoweft were checked independently of any
Issue connector. Main reported unprotected and its ruleset disabled. Actual push
and PR-creation results are recorded separately after publication.

AGENT.MD, applicable AGENTS.md locations, PR template, contracts/API, complete
internal builder, policy/typed values, private execution, public output, driver,
CLI and MCP paths were inspected. Design question
`5db071a4-1362-4992-b338-e66a4479abe9` was submitted before implementation/tests;
the delivered controller answer was retrieved and used for this revision.

## Local verification

Toolchain: Go 1.26.4 linux/amd64. Native containers: MySQL 8.4.6 and PostgreSQL
16.10 (Debian 16.10-1.pgdg13+1). TEST_MYSQL_DSN, TEST_POSTGRES_DSN and TEST_DB_DSN
were explicit. GOFLAGS was empty for the full/race captures; the separate required make target
used command-local `GOFLAGS=-p=1` to serialize packages. JSON capture used the go test command flag,
never global GOFLAGS. Private 0600 captures stayed outside the repository and
only counts were published.

| Check | Observed result |
| --- | --- |
| `go test -p 1 -json ./... -count=1` | 3364 pass, 0 fail, 0 test skip; 22 tested and 28 no-test packages |
| First final-tree `go test -p 1 -race -json ./... -count=1` | 3362 pass, 2 fail (one leaf plus parent), 0 test skip; existing expiry-last assertion |
| Fresh race retry on unchanged source | 3364 pass, 0 fail, 0 test skip; 22 tested and 28 no-test packages |
| `make test-integration` with command-local `GOFLAGS=-p=1` | Exit 0; 22 passed packages, 0 failed, 28 no-test; non-verbose target does not emit individual test/skip counts |
| `go vet ./...` | Exit 0 |
| `gofmt` and `git diff --check` | Clean |
| `manifest check-repositories --config tests/typedfixture/repositories.json` | Exit 0, actual supplied-source regeneration comparison |
| `sh scripts/test-public-output-check.sh` | Exit 0, canary/exit-preservation checks |
| New pure/mock and native performance tests | Included in full/race; native MySQL/PostgreSQL subtests run sequentially |
| Review risk/precision gate | Exit 1; 1386 findings, 0 suppressed; precise 0, partial 1188, unsupported 198 |
| Strict changed-source review against materialized main | input_failed; all 1386 findings unknown; not a successful changed-code review |
| Example manifest verification with checked-in schema/policy | Exit 1: schema stale, policy OK, generated_code/database skipped |

Earlier development runs used default package concurrency. The final acceptance
runs explicitly use `-p 1`; the CI capture wrapper now does too, preserving its
redaction and exit-status behavior. No two final DB test commands run concurrently.
Initial sandbox-only DB connection/cache failures were environmental and were not
counted as passing tests. Test development also caught a malformed test API call
and a HavingRaw fixture lacking GroupBy; the fixture was corrected without an
unrelated change to that pre-existing API. Earlier successful counts are not
substituted for the final runs above.

## Coverage and limits

The new native fixture checks bounded acquisition, native source roundtrip losing
private provenance, mutation isolation, view/generated-column rejection, external
Tx refusal, connection-wait timeout and the captured Query context deadline.
PostgreSQL also checks RLS and partial-index refusal; MySQL checks temporary-table
shadowing. Fixture DDL/seed/cleanup is test setup, not the acquisition API. No
unsafe EXPLAIN is executed to induce a stored-function side effect.

Pure/mock tests cover static planning with no dispatch; raw/function/group/order/
lock/join/distinct/column-comparison/empty-IN/oversized-input refusals; accepted
finite grouped predicates; tenant/Strict gates; frozen private dispatch after
Query mutation; unknown version with no target query; catalog visibility/unsafe
metadata refusal; Begin/rollback, cancellation, Scan/rows.Err/Close/partial-result/
size failures; duplicate/trailing/deep/large JSON; invalid numbers and unknown
nodes; source versus view canaries, fmt tampering and strict public decoding.
Mock collection never acquires native provenance.

The live schema domain is deliberately integer-only, with schema-qualified
PostgreSQL and same-database MySQL targets. Other scalar combinations are
unsupported, not silently coerced into supported evidence. The catalog checks
are conditional on standard server/driver behavior and no concurrent DDL; no
claim of comprehensive arbitrary-server or race detection is made. Rollback/cleanup failures and oversized server cells are injected in mocks;
native network-failure recovery is unverified. Restricted-grant visibility and
custom operator/cast negative cases use mocks rather than native role/operator
creation fixtures. A client post-Scan bound cannot bound
driver cell allocation or server planner resources.

The inherited strict source-comparison problem remains unresolved. This run does
not uniquely identify all private capture mismatch conditions; the previous
three generated testmain path/hash hypothesis remains unverified. The aggregate
review count differs from inherited main evidence, but unknown correspondence is
not relabeled as confirmed new/unchanged findings. No threshold relaxation,
suppression, mass baseline or formal fingerprint refresh was used. Example
manifest migration and review precision remain separate outstanding work.

No new dependencies or existing wire/API migrations are required. New source/view
v1 contracts are additive. No PR2 hooks, budgets or benchmarks, merge, Issue close,
overall acceptance or user completion is performed. CI patch-level DB versions,
CI test counts/skips and a feature-head Docs run are not inferred from local tests;
Docs publishing runs only on main. Final-head CI/required-check evidence belongs
to the actual PR record.

## Existing expiry assertion reproduced on main

The first final-tree race run failed only
`TestBindingContextOwnerAndLastDispatchChecks/expiry-last` (and its parent).
No race-detector report was emitted. The same isolated test on the materialized
unchanged main failed once in ten iterations (18 pass / 2 fail including parents),
while the feature tree's ten isolated iterations passed (20 pass). The binding,
execution and existing test files have no changes in this PR.

The test waits using monotonic time; prepareBinding compares time.Now while the
last dispatch check calls time.Now().UTC(), stripping monotonic readings. This is
a possible clock-domain explanation, not a uniquely captured cause of the failing
iteration. Existing ValidatedPlan/approval semantics and the test threshold are
preserved. A fresh unchanged-source full race retry is recorded separately; a
passing retry does not resolve or erase the main-reproducible failure.
