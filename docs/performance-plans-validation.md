# GQ-AI-12 PR1 validation

Management ID: GQ-AI-12, PR1 (1/2),
[Issue #94](https://github.com/recoweft/goquent/issues/94).
The API, supported domain and trust assumptions are in
[performance plans](performance-plans.md). Actual PR/head and final-head CI are
recorded in the PR and Issue comment; no GitHub number is inferred here.

## Revision 2: unknown JSON structure correction

Work `965407a3-6c27-4a12-9442-666e24d59d75`, revision 2,
execution `0877712f-7c6e-4213-b9b4-261fac8a99fb` updates existing
[PR #95](https://github.com/recoweft/goquent/pull/95). The historical revision-1
results below are not revision-2 acceptance evidence.

Worker rechecked the expected cwd, Ubuntu 24.04.5/WSL2, origin, clean initial tree,
requested feature branch and head `c5e0cebfa9db802f3f83defd1e3518417f18c01f`.
Remote main remained `68987157de1e9cc7253f809214ab20efe4a5fae6`; PR95 remained
open on the expected head. Worker GitHub API permissions included push/admin;
SSH remote reads succeeded. PR67/68/69/74/75 were independently rechecked MERGED
and their merge commits were main ancestors. Issue94 remained open without a
cancellation, and no revert-subject entry was found on main (not proof against
all semantic reversals). Exact main CI39053595221 and Docs39053595325 were success.
AGENT.MD, applicable AGENTS locations, PR template and current contracts/API and
performance source/public paths were inspected.

The controller delivered the original consultation, adopted answer and review
261b6011-81d5-4a29-b98a-b95e7f30fe91 through this execution's question
61c1ac9e-f83a-4130-ab87-7e83991e3b23. The answer retained the already-adopted
native-only, integer-only, five-second, offline-Plan and source/public-v1 scope;
no new design adoption or scope expansion was requested.

The review counterexample first failed as an executable Go regression:
`TestPerformanceUnknownTableChildRegression` returned one node, nil error and
`unknown=false`. The old parser extracted metrics without classifying every
MySQL table/cost key or either dialect's root siblings. The correction validates
finite per-context key/type sets before extraction. Unsupported extensions set
aggregate unknown while preserving known partial estimates; malformed known
fields and numeric values return the fixed result error. Both source and public
projections preserve aggregate unknown. No arbitrary DB string enters an enum
or error. API/wire, dependencies, collector, catalog, policy and binding remain
unchanged; comments and documentation are English.

Native MySQL 8.4.6 and PostgreSQL 16.10 (Debian 16.10-1.pgdg13+1) JSON was freshly
captured through the bounded terminal using a dedicated two-row integer table,
then saved as parser fixtures (with a final newline). The native test now also
requires source/public `unknown=false` and its preservation after source
roundtrip. Synthetic mutations cover root, child, table and auxiliary-object
extensions, typed arrays, null/wrong types, numeric failures, unknown node values,
canaries and source/public roundtrips. A mock collector separately verifies
unknown propagation and fixed failure with rows closed and transaction rollback;
it never gains native provenance. Normal real-server metadata remains understood.

Initial sandbox GitHub DNS and localhost/Docker access checks failed. Narrow
approved read-only diagnostics succeeded; these did not establish test success.
Subsequent authorized DB test commands use explicit TEST_DB_DSN, TEST_MYSQL_DSN
and TEST_POSTGRES_DSN, sequential DB execution and `-p 1`. Full/race use empty
GOFLAGS and command-local `-json`; integration uses command-local GOFLAGS=-p=1.
No global environment, DNS, sandbox or WSL setting was changed. Private local
captures use mode 0600 and only aggregate results are published.

Toolchain: Go 1.26.4 linux/amd64. Final implementation checks:

| Check | Revision-2 result |
| --- | --- |
| Regression before parser correction | Expected FAIL: counterexample returned unknown=false |
| Parser/performance tests after final correction | 119 pass, 0 fail, 0 test skip; one tested package |
| Fresh final `go test -p 1 -json ./... -count=1` | 3436 pass, 0 fail, 0 test skip; 22 tested, 28 no-test packages |
| Fresh final `go test -p 1 -race -json ./... -count=1` | 3434 pass, 2 fail (expiry-last and parent), 0 test skip; 21 passed packages, 1 failed, 28 no-test |
| Final `make test-integration` | Exit 0; 22 passed packages, 0 failed, 28 no-test; individual test/skip counts not emitted |
| `go vet ./...`, gofmt, `git diff --check` | Pass |
| Repository regeneration comparison and public-output canary | Pass |
| Review risk/precision | Exit 1; 1386 findings, 0 suppressed; precise 0 / partial 1188 / unsupported 198 |
| Strict changed-source review against materialized exact main | Exit 2; input_failed, all 1386 head findings unknown; no invalid/unlisted/read-failed diff entries, one unknown correspondence |
| Example manifest verification | Exit 1; schema stale / policy OK / generated_code and database skipped |

The final race failure is the existing mock binding `expiry-last` assertion;
there was no race-detector warning. Binding/execution/identity code and the
binding test are unchanged from main. Its previous reproduction on unchanged
main remains inherited evidence, not a new reproduction in revision 2. No final
race retry was used to erase the failure. Earlier development full/race runs
(3434 pass each, before the last metadata cases) are not substituted for the
final checks above. Native restricted-role/custom-operator/cast/network-failure
fixtures remain unexecuted; existing corresponding negative/fault cases are mocks.
No threshold, suppression, mass baseline or fingerprint refresh was introduced.
Review correspondence/precision, example migration and expiry-last remain open.

Actual pushed head, exact-head CI and required checks are recorded in PR95 and
the Issue94 revision-2 comment after publication. Main protection was rechecked
as absent, its ruleset disabled, and no required checks were reported. This is
not a guarantee that a CI run passed. Feature Docs publishing is not configured.
PR2, merge, Issue closure, whole-Issue AC adoption and user completion are outside
this revision. The controller owns the next review.

## Historical revision 1 evidence

### Identity and preconditions

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

### Local verification

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

### Coverage and limits

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

### Existing expiry assertion reproduced on main

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
