# Bounded native cancellation evidence contract

GQ-MAINT-13 PR2; Refs [Issue 123](https://github.com/recoweft/goquent/issues/123).
The [release blocker](https://github.com/recoweft/goquent/issues/123#issuecomment-6386876158)
remains unresolved. This document separates current behavior from an **adopted limited** finite acceptance contract. It does not change production code
or existing native test expectations. Separate human release review is required.

## Evidence sufficiency before new work

| Evidence | Sufficient for | Insufficient for |
| --- | --- | --- |
| Production source and pinned dependency source | Conditional public-result classification and locations where information is lost | A native execution trace or historical cause |
| Existing 13 rollback and 17 owned-close unit cases | Strict sentinel and prior CLEANUP precedence, negative wrapped/joined/independent cases | Native acquisition or driver cleanup success |
| Existing synthetic Conn/Tx ordering and rows-cleanup tests | Logical close before physical completion; invisible physical-close errors; visible rows failure precedence | Native scheduling or absence of hidden failures |
| Existing three real-pq mechanism controls | Ordinary raw rollback; stored-cancel raw ErrBadConn; automatic-first public ErrTxDone with hidden driver ErrBadConn | Passing the production concrete-connection check or forcing the native Tx.done winner |
| Existing production native network fixtures | Fixed expectations for the particular held-version-response cancel, deadline and disconnect trials; separate resource observations | All scheduling orders, arbitrary query phases, server abort/rollback ACK, physical-close success or safe reuse |
| Historical PG race 17 pass / 1 fail | An unresolved failure existed | Exact failing whole-tree or causal attribution; both remain unknown |

Repeating those controls is validation, not a new discovery. Later 30/30
non-reproduction does not repair the historical failure. Historical counts are source-attributed evidence only; fresh PR2 evidence is
reported separately below and in the PR validation ledger.

## Source and version scope

Baseline: main `7a39d9fdb09364f92e16660040161b6822ffb6a9`, containing merged
PR124 (`cf1c8651dbdb2ea57b251d02115245f122273614`) and PR122. During this execution,
the worker independently checked main CI 40116721981 and Docs 40116722026 success.
Old references to PR122/124 as draft describe historical states.

`go.mod` declares Go 1.25.0, preferred toolchain 1.26.4,
mysql v1.9.3 and pq v1.10.9. Locally available Go 1.25.0 and Go 1.26.4
`src/database/sql/sql.go` and `ctxutil.go` are byte-identical. This source comparison
is not a runtime test on either toolchain. No driver update is proposed.

Primary local sources:

- [`ExplainPerformance`, acquisition, rows and classification](../orm/query/performance.go).
- [Collectors](../orm/query/performance_catalog.go),
  [synthetic classification/order cases](../orm/query/performance_cancel_test.go).
- [Native network fixture and cleanup](../tests/performance_proxy_test.go),
  [owned-object fixtures](../tests/performance_boundaries_test.go),
  [forwarding controls](../tests/performance_proxy_order_test.go),
  [real-pq mechanisms](../tests/performance_pq_mechanism_test.go).
- Go versions above: `database/sql` functions `beginDC`, `Tx.awaitDone`,
  `Tx.rollback`, `Conn.close`, `Rows.awaitDone`, `Rows.close`, `Rows.Err`.
- pq v1.10.9: `conn_go18.go` `BeginTx`, `watchCancel`, `QueryContext`;
  `conn.go` `syncErr.get`, `Rollback`, `closeTxn`.
- mysql v1.9.3: `connection.go` `BeginTx`, `watchCancel`, `startWatcher`,
  `cancel`, `cleanup`; `transaction.go` `Rollback`.

## Context and observation boundaries

| Boundary | Actual context/order and visible result | Invisible or unpromised |
| --- | --- | --- |
| Entry caller context | `WithTimeout(caller, budget)`, default/max five seconds; positive option shortens it; caller may end sooner | Budget is not a hard bound on driver cleanup completion |
| Frozen Query context | Already-ended context refuses before acquisition. Otherwise `AfterFunc(frozen.ctx, cancel)` cancels the budget child; outer override uses frozen context's own reason unless prior is CLEANUP | A frozen deadline initially cancels the budget child as Canceled; outer classification recovers frozen DeadlineExceeded. No shared-reason identity is implied |
| Connection / transaction start | `db.Conn(budgetCtx)`; concrete raw connection checked; `BeginTx(budgetCtx, ReadOnly)` | No driver/connection wrapper may stand in for native evidence |
| Driver Tx lifetime | `beginDC` forwards its caller (budgetCtx) to driver BeginTx, then derives an internal child for `Tx.awaitDone` | Driver does not receive that later internal child; caller cannot obtain Tx.done or automatic rollback's result |
| Query and rows | Collector uses budgetCtx in `QueryContext`; database/sql rows also watch internal Tx context | Query cancellation and transaction rollback race; a return does not join all cancellation goroutines |
| Query/iteration failure | Query/rows error maps through current context to CANCELED/TIMEOUT or fixed QUERY/RESULT fallback | Raw error and causal fault identity are not public; cancellation can coexist with transport failure |
| Explicit rows close | Non-nil `Rows.Close` overrides to CLEANUP | If auto-close already ran, repeated Close returns nil. `Rows.Err` can prefer context error over driver Close error; nil Close is not proof of no cleanup failure |
| Transaction override | Ended budgetCtx overrides collection result except known CLEANUP | No reconstruction of already-hidden errors |
| Explicit rollback | Tx.done CAS winner calls driver and returns its error; loser returns ErrTxDone | Public API has no gate immediately before this CAS |
| Automatic rollback | `awaitDone` calls rollback; winner sets done before driver call and connection release | Automatic driver return is discarded by awaitDone, not returned to the explicit caller |
| Owned Conn close | `Conn.done` CAS precedes release; explicit close result enters final classifier | ErrConnDone can precede physical completion. Pool release can hide driver Close error even when public Close returns nil |
| Outer result | Known CLEANUP survives frozen-query context override; acquisition cleanup errors clear the plan | Result publication is not a join of all driver/server work; no successful native authority follows from a failure result |

MySQL's watcher stores the context error and closes the connection; rollback of
a closed connection can return that stored exact sentinel. pq stores cancellation
before dialing its separate cancellation connection; `syncErr.get` converts a
stored error to `driver.ErrBadConn`. pq rollback defers transaction finishing even
on that error. These are different mechanisms, not driver-name exemptions.

## Fixed classification by observable inputs

Each row is a conditional statement about a particular observation boundary,
not a two-result assertion for one native event. Downstream known cleanup failure
can override earlier cancellation. CANCELED/TIMEOUT below abbreviate the existing
`PERFORMANCE_*` values. No new public code is proposed.

| Observed condition | Fixed result at that boundary | Coverage and limitation |
| --- | --- | --- |
| Query failure, live context | QUERY | Source/unit; does not identify transport cause |
| QueryContext failure or Rows.Err failure, standard canceled / deadline context | CANCELED / TIMEOUT respectively | Two distinct inputs; later cleanup still runs |
| Live Rows.Err iteration failure | RESULT | Source; distinct from QueryContext failure |
| Columns error or more than 16 columns; Scan error | RESULT | Direct branch before later transaction/context/cleanup overrides |
| Row count or byte limit exceeded | LIMIT | Direct branch before later overrides |
| Explicit rows Close returns non-nil | CLEANUP | Existing synthetic visible-rows regression; independent fault remains known |
| Rollback nil | Preserve prior | Does not imply universal rollback ACK |
| Public rollback ErrTxDone with ended context | Preserve prior | Source uses `errors.Is` for this existing TxDone branch; does not expose automatic result |
| Rollback exactly equals standard context termination sentinel | CANCELED or TIMEOUT matching that sentinel, unless prior CLEANUP | Existing 13-case unit matrix; exact equality, not wrapped/joined matching |
| Rollback independent error, ErrBadConn, EOF, mismatched or wrapped/joined context sentinel | CLEANUP | Existing negative unit cases; raw pq BadConn mechanism is a separate evidence layer |
| Owned close nil | Preserve prior | Public nil does not certify physical driver close |
| Owned close exact ErrConnDone, exact matching prior CANCELED/TIMEOUT and standard context | Preserve matching prior | Existing 17-case unit matrix; logical exception only |
| Other non-nil owned close | CLEANUP | Synthetic classifier inputs; not a claim all such values are returned by sql.Conn.Close |
| Known prior CLEANUP at transaction/final/outer overrides | CLEANUP | No cancellation exemption for known independent cleanup failures |
| Independent proxy disconnect in existing native fixture | CLEANUP, unchanged fixed expectation | Separate event from cancel-only, with real transport teardown |
| Cancel-only held response in existing native fixture | CANCELED, unchanged fixed expectation | Existing empirical regression; all Tx.done orders remain unproved |
| Deadline held response in existing native fixture | TIMEOUT, unchanged fixed expectation | Semantic two-second deadline differs from three-second watchdog; all orders unproved |

“Known” means received by production processing, not merely recorded by a test
observer. The observer does not feed its driver return back into production.

An independent fault hidden before a public boundary cannot be assigned CLEANUP
by a classifier that never receives it. For example, automatic rows close can
lose its independent driver error while the caller observes Canceled and nil
Close. The fixed public classification is conditional on public observations;
it is not a causal promise that cancellation was the only physical event.

## Proxy, publication and resource synchronization

| Event / condition | Establishes | Does not establish |
| --- | --- | --- |
| `proxy.entered` | Real version request forwarded and real response held after read-only Begin | Which rollback will win |
| Cancellation frame receipt | Frame read by proxy | Full forwarding or server receipt |
| `proxy.cancellation` | All frame bytes successfully written upstream; partial/error writes do not notify | Server processing, abort ACK, or rollback ACK |
| `proxy.release` | Response allowed after context ends; PG also waits for full cancel forwarding | Ordering of Tx.done CAS, rows auto-close, driver return or connection release |
| Result channel / `published` | Acquisition returned / worker published its result | All driver goroutines or physical closes joined |
| Proxy close/join | Fixture listener/connections/workers stopped, under watchdog | Application cancellation alone caused server cleanup |
| Pool close and InUse=0 | No checked-out pool connection at observation | Driver Close succeeded, no earlier hidden error, or safe reuse |
| Server session absence | Tracked session and its transaction absent at observation after fixture cleanup | Successful rollback command ACK or prompt abort before teardown |
| Owned table/role/schema absence | Separate fixture cleanup succeeded | Absence of arbitrary external side effects |
| Subsequent native control | Another bounded acquisition works | Reuse of the same physical connection or global health |

Native cleanup is registered before acquisition and remains active after fatal
assertions: cancel, proxy close/join, publication join, pool close/count, server
absence, and normal control; outer cleanup removes owned objects. Watchdog or
resource failures are test failures, never successful cancellation evidence.
Older forced-assertion evidence is historical and must be attributed to its source
and subsequent changes; it is not a new execution on this revision.

## Concrete ordering counterexamples

Both schedules below permit context termination, complete PG cancellation
forwarding and held-response release. Neither requires an independent injected
transport cut. They are source-level possible schedules supported by separate
mechanism controls, **not two newly observed production native traces**.

1. pq stores cancellation; forwarding completes. Collection returns; explicit
   deferred rollback wins Tx.done before awaitDone runs. pq Rollback sees stored
   error and returns ErrBadConn. Production classifies CLEANUP.
2. awaitDone wins Tx.done and enters driver rollback first. Explicit rollback
   returns ErrTxDone without waiting for the automatic driver's return. That
   driver's ErrBadConn is invisible to this caller. Given prior CANCELED and
   nil or matching logical owned close, production preserves CANCELED.

The response gate is upstream of the competing goroutines; moving its release
cannot force their CAS order. A driver rollback entry gate can prove automatic
ownership, but wrapping the connection/transaction is mechanism instrumentation
and fails the concrete native connection check. Raw driver calls bypass the
production acquisition path entirely. No exported Tx.done hook exists. Waiting
for ErrTxDone, ErrConnDone or InUse=0 does not recover the lost return value.

Likewise, existing synthetic close controls give the same public nil close /
ErrConnDone observations with successful and failed physical Close. They refute
inference of physical success; they do not inject that fault into a native driver.

## Adopted finite acceptance and decision

The delivered answer to consultation
`5b2b245b-e704-4b33-8182-66e75f24eacc` adopts option A for this work,
revision 1 and execution: accept an observation-conditioned contract
and explicit coverage ledger for this PR's review, without certifying every native
schedule. Preserve all current production priorities and native fixed tests.
Keep native all-order cancellation, hidden cleanup errors and physical guarantees
as unproved release-review inputs, not as waived passing tests.

Additional coverage is one minimal synthetic paired rows-close control
in [the observation tests](../orm/query/performance_rows_close_test.go):
live explicit close exposes an independent error; canceled automatic close hides
that error behind context plus idempotent Close. Use driver-entry/release channels,
join the close and observer, and register fallback cleanup before assertions.
Record the actual driver error separately from public Close/Err; assert each
observation exactly. A mutation swallowing the independent driver error must
fail the mechanism assertion. Forced failure while gated must still release/join
and observe pool cleanup. This adds an unrepresented information-loss boundary;
it is not a fourth pq trial or new native cancellation coverage. The implementation uses real database/sql with a synthetic driver, no transaction
context, and no Next/EOF. Explicit close cancels the internal close context;
query cancellation stores the query reason. Driver return notification is joined
separately from public Close completion through database/sql's close mutex.
The live control uses a fresh live context for pool Ping; it is synthetic, not
a native acquisition or physical reuse guarantee.

This adoption explicitly separates all-order native cancellation, complete hidden-fault
detection, physical Close success, rollback/server-abort ACK and safe reuse from
completion criteria for the finite documents and controls. They remain unproved
human release-review items; this is an acceptance-scope reduction, not attainment
of unchanged universal acceptance or permission to clear the release blocker.

The reduction relative to a universal expectation is explicit: this adopted scope
does not prove all native cancel-only schedules return CANCELED, or all hidden
independent cleanup failures return CLEANUP. It neither changes an existing test
expectation nor permits two candidates, skipping, timeout relaxation or retries
until success. Any new ordinary validation failure remains a current blocker.

Alternatives: keep universal native acceptance unmet and publish only the boundary
analysis; or commission a separate runtime/lifecycle design to expose or serialize
the missing boundary. The latter changes scope, requires its own specification
and must still address hidden physical-close results. Exempting pq ErrBadConn
would erase independent-fault distinctions and is not recommended or authorized.

## Validation and release ledger

The inherited untracked draft was retained with controller approval in answer
to question `da921aea-2727-48a8-b223-2580bc6622a3`. Its source claims were
checked against the local baseline and cached dependencies; prior draft prose
is not execution evidence. GitHub repository permissions report push access,
SSH main lookup and planned-branch push dry-run succeeded, and the all-state
branch PR lookup returned none. API and SSH initially failed in the sandbox
(network/DNS); approved diagnostics succeeded. No remote ref was created by
the dry-run. Actual PR creation remains untested until the authorized creation.

Before freezing, the new pair passed an initial normal execution (two leaves
and one parent). The initial launcher did not separately retain the Go exit;
its counts are preliminary only. The protected nil-error sensitivity overlay
and gated forced-failure overlay each exited 1 with two failed leaves and one
failed parent, zero skips. Both cleanup callbacks completed their public-close,
driver-observer, pool and live-control checks with no extra assertion output.
These deliberate failures are separate from ordinary validation. Overlays do not
modify repository source. A fresh cache outside the repository avoids a read-only
cache-trim diagnostic from the initial launcher.

Approved read-only diagnostics confirmed existing goquent Compose services and
MySQL 8.4.6 / PostgreSQL 16.10; sandbox loopback diagnostics had been denied.
Connectivity and version checks alone are not successful native tests.
Final clean-head focused/full/race/native/vet/canary/review/manifest results and
exact-head CI belong in the PR and Issue validation ledger, keyed to the immutable
commit. This document records scope and pre-freeze controls, not anticipated
final successes. Historical PR124 evidence uses Go 1.25.0,
MySQL 8.4.6 / PostgreSQL 16.10; historical CI used Go 1.26.4. PR122's reported
full/race 3841 pass events, zero failures, two skips; native 25 pass events each,
zero failures/skips; CI 40115120576 success are separate-source evidence, not PR2
results. Detailed leaf/parent counts and actual exits must accompany fresh tests.

For the adopted scope, freeze the final candidate before validation; test-source changes
require fixed-head full and race once each, plus focused and relevant native,
vet/format/diff/canary and repository-required review/manifest checks. Preserve
ordinary and exact-clean nonempty strict diff findings; reuse only with explicit
source head, input invariance and limits. No exploratory count batches are planned.

Raw evidence stays outside repository/analysis roots in owner-only 0700/0600
storage, owner-only executable helpers, without upload. Three explicit disposable
DSNs, opt-in, `-p 1` and sequential DB suites remain required. No target query or
EXPLAIN ANALYZE, new service, driver update or global network change is proposed.

Historical unknown1495 is its own measured result, not a new PR2 count.
Risk/precision findings, original manifest generated-code/database skips,
separate companion limitations, closureeligible=0, unimplemented loader producer,
historical test-main/expiry concerns, fixed BLOCKERS and human release review
remain. Neither this adoption nor eventual PR integration closes Issue123,
establishes whole-Issue acceptance, or authorizes release/package publication.
