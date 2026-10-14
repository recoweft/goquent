# GQ-AI-09 PR1 revision 4 review and validation

Work: `e368a941-445f-4891-af3f-547b3268b1c7`, revision 4.
Execution: `ed9edd34-b784-439c-bd97-6b88c769f822`.
Issue: https://github.com/recoweft/goquent/issues/82.
PR1 (1/3): https://github.com/recoweft/goquent/pull/83.
Branch `feature/gq-ai-09-review-types`, base `main`; title unchanged.
The publication report and PR record the exact final commit and its CI run.

## Identity and review scope

The worker independently checked the requested cwd, Ubuntu 24.04.5 on WSL2,
origin, clean initial worktree, branch and local/remote/PR head
`8eae34437c2d875cf6f8f68c10ab4571b80c445e`. Fetch confirmed latest main remains
`ff01a7c954994998e662ecf5be54c02f83375739`, an ancestor of HEAD. Its CI run
38565946298 and docs run 38565946395 succeeded. Worker authentication,
repository push permission and SSH push dry-run succeeded with authorized
network access. Initial sandbox network failures were not credential failures.
Issue82 remains open, links PR83, and retains AC1–6, all three planned branches
and seven unchecked acceptance/plan items. No duplicate Issue/PR was created.

Reviewed AGENT.MD, the PR template, current shared contracts, the PR's type loader,
tracker, Go-to-plan adapter, JSON reader, rule/suppression integration, fixtures,
CLI gate and migration documentation. The review compared Query mutation,
scopes, generic/scoped operation declarations, raw Executor entry points and the
private binding operation families against the tracker inventory. Internal SQL
builder construction remains excluded; actual Query/DB/Executor operations are
not excluded by library path. No runtime DB, Strict, tenant, planner seal or
execution binding implementation changed.

## Reproductions and corrections

Buildable modules use actual aliased database/sql and Goquent types. Their two
new Go fixture bodies also compiled independently with `go test -run=^$ ./...`.
On the original head, the regression assertions failed at ten return/container
escape sites: Exec/Query returns, a Query terminal alias return, struct/slice/map
literals, a terminal slice and field/index assignments. Existing argument escape,
direct/local method value/method expression and unrelated same-name controls
were retained. Fixed partial markers now survive at the escape sites. Added
channel and SelectMap/SelectMaps escape checks pass. No container retrieval or
arbitrary interprocedural SQL reconstruction was added.

Additional actual failures on the original tracker were:

- Both branch arms replacing a method value with another receiver could retain
  the original receiver's precise evidence.
- Query alias rebinding could leave the other receiver precise after the join.
- All six Validate* and six ExecuteValidated* public Query operation families
  had no finding.
- A loop replacing a DB method with an unrelated method could erase the
  possible zero-iteration DB call (separate before-fix regression).

Branch snapshots now retain receiver identity; changed receiver alternatives
retain unknown state, without borrowing predicates/limits. For/range joins keep
the zero-iteration possibility. The twelve existing public operation families
are explicit partial targets. A shared declaration-based raw classifier keeps
direct and escaped callable inventories consistent. Independent Query, shadow,
helper, scope, loop mutation, closure capture, opaque call, expression, binding,
location, negative type and public canary controls pass.

## QueryPlan JSON boundary

The adopted QueryPlan-only boundary remains unchanged. Tests cover warning-free
input, partial/unsupported precision, write-scope precision override protection,
recomputed read/write risk, supplied warnings and suppressed warnings, forged
locations/reasons/approval/Fresh, unknown version, malformed envelopes and
unrelated JSON. An added regression checks the independent fixed marker's actual
file/line 1, reason and precision, explicit config suppression, expiry and public
canary omission. Existing SQL and shared serialization fixtures still run.
MigrationPlan provenance, multi-axis precision and runtime correspondence were
not added. Historical answer IDs are not treated as new answer deliveries.

## Measurements

Toolchain: Go 1.26.4 linux/amd64. Existing local test services: MySQL 8.4.6 and
PostgreSQL 16.10. These are worker measurements, not inferred CI versions.
Raw source reports and test logs stay local; public output does not contain
source, SQL, values, arbitrary errors or credentials.

| Command / check | Revision 4 result |
| --- | --- |
| New focused regressions against original tracker | Failed as described above; expected before-fix evidence |
| `go test ./orm/review -count=1` after fixes | Passed |
| Independent fixture `go test -run=^$ ./...` | Both fixture modules compiled; no runtime DB execution |
| Initial sandbox `go test ./... -count=1 -json` without explicit DSNs | Interrupted after DB-related skips; not a completed full-suite pass |
| `go test ./... -count=1 -json` with explicit TEST_DB_DSN, TEST_MYSQL_DSN, TEST_POSTGRES_DSN | 22 packages passed; 2,721 test/subtest pass events; zero test skips/failures; 26 no-test packages |
| `make test-integration` with the same explicit three DSNs, after full suite | Passed; 22 packages, 26 no-test packages |
| `go test -race ./orm/review ./cmd/goquent` | Both packages passed |
| `go vet ./...` | Passed |
| `sh scripts/test-public-output-check.sh` | Passed |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Passed; supplied-source comparison, not live evidence |
| Example `manifest verify` with manifest/schema/policies and `--format json` | Exit 1: schema stale, policy OK, generated_code/database skipped; unresolved |
| `gofmt`, `git diff --check` | Passed |

DB suites ran sequentially. No additional full suite is needed solely for final
English documentation and PR updates. No migration SQL changed, so migration
plan/apply is not applicable.

## Review count audit and compatibility

Fresh snapshots of main and the original head were measured again in this
execution. The final source tree was also measured. Main and final public CLI
`go run ./cmd/goquent review --fail-on high ./...` both exit 1. Untruncated private
reports, not the 256-item display cap, supply these counts.

| Measure | Main | Original head | Revision 4 |
| --- | ---: | ---: | ---: |
| Findings | 499 | 1,126 | 1,180 |
| Precise | 253 | 156 | 156 |
| Partial | 157 | 862 | 916 |
| Unsupported | 89 | 108 | 108 |
| Suppressed | 0 | 0 | 0 |

Matching code/file/physical-position tuples with unchanged-line mapping retains
all 1,126 original-head findings. The increase is 54 partial findings: 53 at the
previously omitted validated operation calls and one callable escape. No original
head finding was removed, and the risk-level counts other than medium are
unchanged. New fixture source strings are not counted as live program operations.

The main comparison independently reconfirms 425 retained and 74 removed tuples.
Of the 74, 57 are in internal builder files, ten are internal builder calls in
Query implementation and one is internal builder construction in generic Query
implementation: 68 false positives in total. The other six old variable partial
markers are replaced by reconstructed shapes: four need no warning, one retains
LIMIT_MISSING and one DELETE_WITHOUT_WHERE. No thresholds, bulk suppressions or
input exclusions changed. The original audit's removal explanation is supported,
while its API completeness claim needed the validated-operation correction above.

New unknown markers can change precision gate outcomes. The original invalid
snippets, typed counterparts, source-wire/public-view contracts and documented
JSON migration remain intact. Partial is neither authorization nor Strict
permission. No new dependency or Go version requirement was introduced.

PR2 runtime correspondence/multi-axis precision and PR3 SARIF/diff/baselines stay
separate work. Arbitrary application dispatch, complete SQL recovery, live
manifest freshness and exhaustive whole-program soundness are not established.
CI internal test counts and DB versions remain unverified behind the public-output
wrapper. Dependency PR merge observations supplied by the requester were not all
repeated. Passing tests do not merge PR83, close Issue82, check whole-Issue ACs or
declare user completion; integration judgment remains separate.
