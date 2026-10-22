# GQ-AI-12 PR2 evidence

Management ID GQ-AI-12, PR2 (2/2),
[Issue #94](https://github.com/recoweft/goquent/issues/94).
Work cc0e2ff1-0378-432d-8322-0532a7fd86f9, revision 1;
execution 6867f507-b70a-4974-b74c-f55b85db2304.
The actual PR/head, fresh full/race/integration counts, review/manifest outcomes
and final-head CI are recorded in the PR and controller report, not inferred.

## Start gate and design

Worker checked the expected cwd, Ubuntu 24.04.5/WSL2, clean initial tree and
origin git@github.com:recoweft/goquent.git. Main was
c316ba5654fce8f3656e93fc22dd0022901b7ebb. PR95 was MERGED at that commit, with
head 6cfa3f24702d5eba05e87bc0737159b0c94dcf99 an ancestor and identical tree.
CI39768772780 and Docs39768772819 were completed/success on that exact main.
PR67/68/69/74/75 were also MERGED with main ancestry verified. No Revert-subject
entry or Issue cancellation was found; this does not prove absence of every
semantic reversal. Issue94 remained open, with no existing all-state PR or
local/remote branch for the requested PR2 head before creation.

Worker gh repository permissions and viewerPermission included push/admin.
SSH ls-remote and the specified branch's push dry-run succeeded. These are
separate from the Issue connector and do not themselves prove actual publication.
The PR records actual push/creation and exact head. Sandbox API, Git-index,
Docker-socket and loopback restrictions were encountered; narrowly approved
commands succeeded. The readonly default Go build cache was replaced with a
work-specific /tmp cache. No global sandbox/network/WSL settings changed.

Consultation 3d93292d-857a-4ace-8769-3c4045a3ddf3 was submitted before implementation;
the controller delivered its answer to this same execution. The adopted design
retains concrete signatures, adds tracked Rows, freezes interval statistics while
keeping budgets, bounds all retained tracking, and delivers hooks through Drain.
The operational contract is in [query-observation.md](query-observation.md).
No sensitive source API, monitoring dependency or PR1 acquisition expansion was
introduced. Standard Go 1.26.4 database/sql lifecycle source was inspected.

## Test interpretation

New mock/unit tests exercise concurrency and atomic budget reservation, children
and independent scopes, Finish/late-completion races, ring/tracking/key caps,
saturation, frozen intervals with continued budget enforcement, repeated shapes,
private owner separation, untrusted JSON/fmt canaries, opaque error/value methods,
Drain panic/reentry/concurrency/slow delivery, and distinct result metrics.
Rows fixtures include EOF, early/failed Close, Scan failures, rows.Err, multiple
result sets, repeated Scan, zero/never-scanned Rows/Row and handoff. Common
policy/static Plan, generic RETURNING, command internal dispatch and transaction
success/failure boundaries are covered without weakening existing regression tests.

Native fixtures use MySQL **8.4.6** and PostgreSQL **16.10**. Explicit
TEST_DB_DSN, TEST_MYSQL_DSN and TEST_POSTGRES_DSN are set from the checked-in
dedicated development configuration, without publishing their values. Database
suites run sequentially, with full/race using package parallelism 1. GOFLAGS never
contains global -json; JSON test events are command-local and raw diagnostics stay
in protected local captures. Test skips, no-test packages, failures, mocks and
native checks are reported separately.

Native observation fixtures cover writes, repeated structured reads, Raw tracked
rows, PostgreSQL RETURNING, manual transactions, external-transaction refusal
without finalization, and a rejected write leaving data unchanged. The existing
native command fixture now also checks catalog observations, ledger/business
counts and budget-exhaustion atomicity. Fault injection such as Close failure or
synthetic connection errors is mock evidence, not a native fault acquisition.

## Fresh source validation

After the final lifetime and saturation checks, full and race each reported
**3478 pass / 0 fail / 0 test skip**, with **23 tested / 30 no-test packages**:
`go test -p 1 ./... -count=1 -json` and
`go test -p 1 -race ./... -count=1 -json`.
`go vet ./...`, repository regeneration comparison and the public-output canary
also passed. Passing race here does not diagnose or repair the inherited
expiry-last failure. Additional integration/review/manifest/publication results
are recorded in the PR and controller report without substituting old evidence.

## Benchmark method

Environment: Go **1.26.4 linux/amd64**, Ubuntu **24.04.5** on WSL2 kernel
**6.18.54.1-microsoft-standard-WSL2**, Intel Core Ultra 9 275HX, 24 logical CPUs.
All reported benchmarks use **GOMAXPROCS=2**, `-benchmem -count=10 -benchtime=1s`,
without concurrent full/race tests or other benchmark commands. This is a local
workstation sample, not controlled production capacity evidence.

The exact main commit above was materialized in a separate /tmp directory and
given the identical baseline benchmark fixture. One mock benchmark operation is
one request containing **eight Raw writes and eight structured one-row reads**.
The in-memory database/sql driver measures client CPU/allocation work; it does not
measure database latency or provide native evidence. Disabled uses the original
context. Enabled variants Start/Finish a request scope; budget allows exactly 16
calls, repeat enables shape diagnosis, and drain also delivers the retained batch.
The baseline fixture adds no production driver or dependency.

Results below use median and observed min/max, not significance testing. Allocation
counts include the whole 16-query request and ORM planning/scanning; they are not
per-query collector-only costs. Compare main with PR2 disabled as well as comparing
PR2's modes. There is no claim of an unmeasured improvement.

| Mock request mode | Median ns/op | Min–max ns/op | Median B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: |
| main disabled | 76,721.5 | 70,682–88,032 | 120,918.5 | 1,736 |
| PR2 disabled | 77,125.5 | 70,576–83,207 | 121,879.0 | 1,768 |
| PR2 collector | 84,932.5 | 78,212–96,238 | 133,890.0 | 1,802 |
| PR2 budget | 86,146.5 | 76,543–104,346 | 133,890.0 | 1,802 |
| PR2 repeat | 93,068.0 | 85,781–101,260 | 144,668.0 | 1,893 |
| PR2 drain | 86,688.5 | 79,818–95,903 | 133,890.0 | 1,802 |

PR2 disabled has 32 additional allocations per 16-query request in this fixture.
Its median time is about 0.5% above main in this sample; the ranges overlap.
These results do not establish a production latency guarantee or zero overhead.

| Native single-query mode | Median ns/op | Min–max ns/op | Median B/op | Allocs/op |
| --- | ---: | ---: | ---: | ---: |
| mysql/disabled | 151,304.5 | 125,102–178,849 | 5,424.0 | 83 |
| mysql/collector | 150,883.0 | 140,403–185,450 | 6,552.0 | 91 |
| postgres/disabled | 149,950.5 | 132,306–164,326 | 5,456.0 | 82 |
| postgres/collector | 146,627.5 | 136,212–160,766 | 6,584.0 | 90 |

Native latency is separately measured using one `SELECT 1` through the Raw
QueryRowContext gate and Scan per operation. Collector mode includes Start/Finish.
It includes ORM, pool, driver, local networking and server; it is not server-only
execution time. Overlapping distributions and lower enabled medians in this sample
do not establish an improvement. The native result contains no table identifiers,
query values, credentials or raw server logs.

## Remaining evidence limits

Inherited PR1 evidence is not reused as PR2 validation: the previous race run had
3434 pass / 2 fail (expiry-last and parent), with unchanged-main reproduction
reported earlier but cause unresolved. A passing PR2 run cannot establish its
repair. Previous review had 1386 findings / 0 suppressed, precise 0 / partial 1188 /
unsupported 198, failing risk/precision; strict changed-source was input_failed
with all findings unknown. Generated testmain path/hash correspondence remains
unresolved. The example manifest was schema-stale, policy-OK, with generated-code
and database checks skipped. New results are reported separately without threshold
relaxation, suppression, baseline masking or arbitrary fingerprint refresh.

PR1 restricted-role, custom-operator/cast and native network-failure fixtures
remain unexecuted. This PR does not extend PR1's catalog/EXPLAIN scope, claim full
physical database coverage, merge, close the Issue, adopt whole-Issue acceptance,
or act as user completion.
