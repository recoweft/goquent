# Finite native performance boundary evidence

GQ-MAINT-09 PR1, [Issue #115](https://github.com/recoweft/goquent/issues/115).
This supplements [the acquisition contract](performance-plans.md); it does not
expand the accepted grammar, native versions, error meanings, or authority.

## Case matrix

All cases require the existing disposable MySQL/PostgreSQL Compose fixture,
three explicit fixture DSNs and the staged runner's explicit disposable opt-in.
The test additionally requires `GOQUENT_NATIVE_DISPOSABLE_FIXTURE=1`, which the
runner supplies only to native commands. Native/full/race use sequential dialects
and `-p 1`. No services are provisioned. Missing prerequisites remain skip or
unverified, never successful native plan acquisition.

| Group | MySQL 8.4 | PostgreSQL 16 | Evidence and finite boundary |
| --- | --- | --- | --- |
| Restricted role | Unique user with one column SELECT grant, zero direct broader SELECT grants; `PERFORMANCE_UNSUPPORTED` | Unique non-superuser, non-inheriting, non-bypass role with no memberships/table or column SELECT; `PERFORMANCE_QUERY` | Effective privilege queries precede collection. PostgreSQL can read catalogs but target EXPLAIN lacks SELECT permission. MySQL refuses incomplete visibility before target EXPLAIN. A direct SELECT grant on only the owned table supplies a same-principal positive control. This is not all privilege arrangements. |
| Custom operator | Not applicable: no PostgreSQL-style CREATE OPERATOR/opclass facility; no native pass claimed | Owned operator/function and B-tree opclass/index; `PERFORMANCE_UNSUPPORTED`; explicit Raw operator expression separately refused | Ordinary integer predicate reaches the custom-index catalog gate. A custom schema operator merely existing is not itself a refusal condition. The transaction fixes search_path and verifies bootstrap operators. This test does not modify pg_catalog or existing operator families. |
| Custom cast | Not applicable: no corresponding user-defined CREATE CAST; no native pass claimed | Owned composite destination type/function and bigint-to-owned-type assignment cast; `PERFORMANCE_UNSUPPORTED`; explicit Raw cast separately refused | The current column gate conservatively rejects a user cast touching a built-in integer, even if the target would not use that cast. No existing cast is replaced. Other sessions could observe this temporary catalog entry, so fixture tests must remain sequential. |
| Communication fault | Dedicated real connection and loopback proxy | Dedicated real connection and loopback proxy | After read-only Begin, the proxy forwards the version request and holds the real server response. Disconnect expects `PERFORMANCE_CLEANUP`; explicit cancel expects `PERFORMANCE_CANCELED`; deadline expects `PERFORMANCE_TIMEOUT`. These are failure-path successes, not acquired performance plans. |

## Nonexecution and cleanup evidence

PostgreSQL custom functions contain a nontransactional sequence marker. It is
unchanged before/after both refusals and after the normal control. A separate
request-framing proxy observes zero target dispatches during the refusals, then
one target EXPLAIN and zero target executions in the positive control after the
custom index/cast is removed. Thus a known SQL string match is not the sole
nonexecution evidence. Existing structural and catalog mock/spy boundaries remain
separate from these real-driver tests. No target query or EXPLAIN ANALYZE is run.

Each case uses cryptographically unique owned tables, schemas, roles and passwords.
Cleanup is registered immediately after creation; errors use fixed messages.
Only grants on owned objects to owned roles change. Missing role/schema creation
capability skips that case; later setup/cleanup or observation failures fail it.
The disposable fixture's administrative capabilities remain prerequisites for
custom opclass creation and full server session observation.

The proxy frames PostgreSQL and MySQL requests rather than assuming TCP packet
boundaries. A response-arrival channel establishes the injection point. Cancel
and deadline release the held real response after context termination; PostgreSQL
also requires full forwarding of its driver cancellation frame before release.
Explicit disconnect remains a separate fault. A watchdog still tears down the
fixture during cleanup. This does **not** prove prompt cancellation against an
indefinitely unresponsive peer.
The real server session/transaction must disappear within a bounded condition
poll, the pool must have zero connections in use, and all proxy workers/listeners
must close. The direct fixture must then collect a normal native control.
TLS, remote networks, arbitrary protocol corruption, faults at every catalog or
EXPLAIN phase, and all server-side cancellation implementations remain unverified.
Closed-pool and pool-acquisition-timeout tests are distinct from these real
communication faults.

Every expected failure requires the exact existing error identity, fixed public
formatting, no wrapped driver cause, and no returned performance evidence.
Metadata unknown is never promoted to a complete plan. Normal controls require
native provenance and known metadata; this is finite collection evidence, not
execution authorization. An empty MySQL target initially yielded unknown plan
metadata in development; the control fixture was populated instead of weakening
the assertion or changing parser/runtime behavior.

## Validation and residuals

Fresh local collection on this implementation used Go 1.25.0 linux/amd64
(`GOTOOLCHAIN=local`), MySQL 8.4.6 and PostgreSQL 16.10. The preliminary default
Go selector reported 1.26.4; that is not the local collection's toolchain.

| Check | Actual result | Collector interpretation |
| --- | --- | --- |
| Full | Exit 0; 3738 pass, 0 fail, 1 testskip; 24 passing packages, 30 no-test | skip |
| Race | Exit 0; 3738 pass, 0 fail, 1 testskip; 24 passing packages, 30 no-test | skip |
| Dedicated native stage | Exit 0; 25 pass, 0 fail, 0 testskip; 2 packages, 0 no-test | pass |
| Added boundary suite in each of full/race/native | 13 pass, 0 fail, 0 testskip, including all 10 added leaf cases | Actual native execution, not compilation alone |
| Vet / regeneration | Exit 0 each | pass |
| Canary | Shell positive/negative checker exit 0; 47 Go test passes, 0 fail/skip; 4 packages | pass |
| Runner | Exit 0; 12 orchestration tests | mock/orchestration evidence only |
| Formatting / diffcheck | Exit 0 | Local formatting evidence |

The full/race skip is the separate opt-in local type-loading census. The
collector completed with exit 1 and release blocked; passing command exits do
not discharge its fixed blockers. Ordinary/clean exact strict/manifest and CI
are recorded separately against the final commit, rather than inferred here.

Fresh final stage results, actual exits, collector status, Go/DB versions and
exact head/CI checks are recorded in the PR and RelayWeft report. Raw logs and
source evidence stay in protected local storage outside the repository; no
sensitive artifact upload is added. Runner evidence requires each added leaf
case to pass, not just a compiled suite or passing parent. MySQL operator/cast
N/A does not create artificial passing or skipped native test events.

All fixed staged-runner BLOCKERS remain. Review risk/precision failure and the
historical comparison with 1474 unknown findings, manifest generated_code/database
skips, historical testmain/expiry causes, and separate human release review are
not discharged by these tests. No threshold, suppression, baseline, API, wire,
hash, authorization or production runtime change is introduced. Whole-Issue
acceptance, merge, Issue closure and release publication remain separate.

## GQ-MAINT-13 cancellation classification revision

GQ-MAINT-13 PR1, [Issue #123](https://github.com/recoweft/goquent/issues/123),
starts from main `672c3eb7d501b486a08c83fc6e6dbd2b916136ac`. Specification
consultations explicitly approved two limited runtime error-priority changes.
This is not solely fixture stabilization or an unchanged error contract.

### Cause and adopted boundaries

A minimally instrumented MySQL failure came from deferred `tx.Rollback()` returning
exactly `context.Canceled`, with a canceled caller context and no deadline
expiration. It did not come from rows or connection close. MySQL driver v1.9.3
returns its stored cancellation when rolling back a closed connection.
`database/sql` also rolls back on context termination: its transaction-state CAS
determines whether explicit rollback reaches the driver or returns `ErrTxDone`.
The previous code exempted only the latter path from cleanup classification.
This does not establish the cause of every uninstrumented failure or MAINT12's
original failure.

The rollback exemption now requires strict identity between its return value and
the observed standard caller-context termination sentinel. Canceled maps to
`PERFORMANCE_CANCELED`; deadline exceeded maps to `PERFORMANCE_TIMEOUT`.
Wrapped/joined errors, a mismatched sentinel, live context, `ErrBadConn`, EOF and
independent errors remain `PERFORMANCE_CLEANUP`. Existing `ErrTxDone` handling is
not broadened. Known rows cleanup failures are preserved through transaction and
query-context classification. No failure returns a plan or native authority.

The second change applies only to final close of the exclusively owned acquisition
handle, which never escapes to the caller. It classifies logical handle termination,
not physical release. Exact identity is required throughout:

| Prior result | Context | Close result | Final result |
| --- | --- | --- | --- |
| CLEANUP | Any | Any | CLEANUP |
| Any | Any | nil | Preserve prior result |
| CANCELED | Canceled | ErrConnDone | CANCELED |
| TIMEOUT | DeadlineExceeded | ErrConnDone | TIMEOUT |
| CANCELED/TIMEOUT | Live or mismatched | ErrConnDone | CLEANUP |
| nil, QUERY, RESULT or another result | Any | ErrConnDone | CLEANUP |
| Any non-CLEANUP result | Any | Independent/wrapped/joined close error | CLEANUP |

Channel-controlled real `sql.Conn` tests show a second close can return
`ErrConnDone` while physical driver close is blocked. Explicit close can also
return nil when driver close returned an independent error. These identical public
observations cannot prove physical cleanup success or recover failures hidden by
`database/sql`. The adopted logical classification explicitly accepts that missing
information. It does not attest rollback acknowledgement, server abort, physical
close success, safe connection reuse, commit, or DB identity. Pool `InUse == 0`
is not physical-close proof. Known independent failures remain CLEANUP; errors not
exposed by the library cannot be recovered or ruled out here. No private fields,
unsafe access, lifecycle reflection, driver changes, or new public API are used.

### Regressions and finite evidence

Real `sql.Tx` fixtures fix explicit rollback entry before cancellation, and use a
driver-entry channel for automatic rollback. They cover 13 strict rollback cases,
prior rows cleanup, and both connection-close orders with blocked physical close.
The close classifier has 17 positive/negative cases; synthetic wrapper/join errors
are classifier tests, not claimed native `sql.Conn.Close` injection. Context phases
are synthetic unit evidence, separate from native driver/server evidence. No sleep
or context/error call-count dependence is used. Disabling each new exception in
turn makes its cancellation and deadline positive regressions fail.

Native cancellation/deadline release the real held response after context ends;
PostgreSQL first requires full forwarding of the actual driver cancellation frame. Explicit transport
disconnect is separate and still expects CLEANUP. Watchdog teardown is distinct
from the semantic timeout. Cleanup is registered before acquisition and checks
publication, proxy workers/listener, pool use, server session absence and a bounded
normal control even after assertion failure. Owned table/role/schema deletion has
explicit absence checks; cleanup statement failures do not stop later attempts.

| Measurement | Leaf results | Exit and limit |
| --- | --- | --- |
| Uninstrumented original main, MySQL cancel | 18 pass / 2 fail | 1; 20 trials |
| Separate minimally instrumented original main, MySQL cancel | 19 pass / 1 fail | 1; 20 trials; rollback sentinel origin observed |
| Instrumented original main, teardown after result | 20 pass / 0 fail | 0; 20 trials; not proof of repair |
| Initial response-release candidate, both DB network cases | 26 pass / 4 fail | 1; 30 leaves |
| Separate instrumented initial candidate, PG cancel | 0 pass / 5 fail | 1; 5 trials; rollback and connection cleanup observed |
| Packet-synchronized candidate, instrumented PG cancel | 5 pass / 0 fail | 0; 5 trials |
| Packet-synchronized candidate, normal both DB network cases | 30 pass / 0 fail | 0; 30 leaves |
| Same candidate, race both DB network cases | 17 pass / 1 fail | 1; 18 leaves; PG cancel origin unresolved |
| Separate instrumented PG race samples | 10 pass; then 30 pass | 0 each; neither waives the prior failure |
| Forced assertion cleanup control, both DB | 2 intentional failures | 1; both resource/control and owned-table absence observations reached |

All listed native leaf samples had zero skips. Forced failures and temporary
instrumentation were removed. Versions observed separately were Go 1.25.0,
MySQL 8.4.6 and PostgreSQL 16.10. Raw logs remain in protected local storage,
not uploaded. Final post-change results are recorded separately in the PR/report.
The earlier uninstrumented PG race failure remains causally unresolved: the new
logical-close exception is supported by deterministic unit evidence, but without
a matching native observation it is not claimed to repair that historical event.
Fresh passes do not waive that residual or establish whole-Issue acceptance.

[PR #122](https://github.com/recoweft/goquent/pull/122) remains a separate pending
design draft. No new comparison guarantee is adopted; closureeligible remains 0.
Review risk/precision/unknown findings, original example skips, historical causes,
fixed BLOCKERS and human release boundaries remain. No API/wire/hash/version or
authorization change is introduced. The repair PR must remain draft/blocked while
its residual is unresolved; merge, acceptance and release are separate decisions.

## GQ-MAINT-13 revision 2: finite residual investigation

The retained revision 1 `phase-fixed-race` log contains one failed PostgreSQL
`network_cancel` leaf (and its parent failures). Its exact failing source tree
was not recorded with that trial. Retained candidate source copies agree with
one another; compared with initial head `54f987c`, their production difference
is the later adopted owned-close classifier. Those copies do not establish the
whole tree that produced the historical failure. Later passing samples cannot
supply the missing attribution.

### Distinct paths and observations

The following analysis uses Go 1.25.0 `database/sql`, pq 1.10.9, and the initial
revision 2 acquisition code. It separates possible mechanisms from attribution
of the historical native failure.

| Boundary | Possible result and precedence | Observation limit |
| --- | --- | --- |
| Query/collect before cleanup | Context termination maps query/result failure to CANCELED/TIMEOUT | Does not imply rollback or connection release completed |
| Explicit rows close | A non-nil close result becomes CLEANUP | Rows may already have been closed by database/sql; unexposed failures cannot be reconstructed |
| Transaction context override | Preserves known prior CLEANUP | Cancellation cannot erase a known rows cleanup failure |
| Explicit rollback wins Tx.done CAS | The driver is called; independent errors including ErrBadConn become CLEANUP | pq's stored cancellation can be returned by `syncErr.get` as ErrBadConn; this is not the strict cancellation sentinel exception |
| Automatic rollback wins Tx.done CAS | Explicit rollback returns ErrTxDone; existing ended-context exception preserves prior result | Automatic rollback's driver result is not returned to the explicit caller |
| Exact rollback sentinel | Only matching standard caller-context sentinel receives the adopted exemption | Wrapped/joined errors and independent faults are not exempt |
| Owned final Conn.Close | Exact ErrConnDone with exact matching prior cancellation/timeout and context preserves that prior result | Conn.done precedes release completion; logical termination is not physical close proof |
| Prior CLEANUP at final close | Remains CLEANUP for every close/context result | Later logical-close exemption cannot hide independent rollback or rows cleanup |
| Outer query-context override | Preserves known CLEANUP | Does not create successful performance evidence |
| Proxy cancellation receipt | Complete startup cancellation frame has been read from client | Does not prove forwarding, server receipt, or server processing |
| Proxy cancellation forwarding | Every frame byte has been successfully written before notification | Does not add a server acknowledgement guarantee |
| Held response | The first nonempty server Read establishes response arrival | TCP Read is not a complete PostgreSQL message or query-response boundary |
| Response release | Allows response bytes to flow after context termination and forwarding notification | Does not choose which database/sql rollback wins |
| Result publication and teardown | Assertion-safe cleanup separately joins publication/workers, closes pool, observes session absence and normal control | Teardown observations cannot retroactively identify the original classification branch |

The proxy previously published `cancellation` before `server.Write`. A controlled
writer makes receipt-before-forwarding and short/error writes independently
observable. The fixture now publishes only after writing the full request frame;
a short successful write continues with its remainder, while zero progress or
an error refuses publication. This is a test-only transport ordering change.
It is not a claim that the historical PG failure came from this ordering, nor
that forwarding completion ensures cancellation processing or rollback success.
The server response remains a byte-stream gate, not a protocol completion parser.

Channel-controlled regression cases cover blocked first write, blocked remainder,
complete and short successful writes, zero progress, partial/full writes with an
error, and an ordinary non-cancellation frame. They must not publish cancellation
on a failed write. Watchdogs only bound stuck test workers; no scheduling sleeps,
error-call counts, alternate accepted classifications or server ACK are added.

The revision 2 investigation is capped at one predeclared PG cancel race batch
of 30 native trials on a protected snapshot of the initial head. Build/setup
failures are not native trials. Fixed-enum local instrumentation distinguishes
rows close, explicit rollback and final owned close; raw evidence is not uploaded.
The batch completed with 30 passing PG cancel leaves, zero failures and zero
skips (exit 0); no failure-origin trace was produced. This is non-reproduction,
not proof that the historical failure was repaired. Final immutable-head
validation is recorded separately in the PR/report.
If the historical branch remains unknown, the stopping condition is reached after
that batch: retain the draft and the residual. Further work requires a concrete
new distinguishing observation or an explicit acceptance decision, not another
success-count sample. Options remain (A) retain the blocked draft, (B) investigate
a specific contract-preserving deterministic alternative with new evidence, or
(C) separately consult on a concrete new acceptance/classification proposal.
Neither ErrBadConn exemption nor changing an independent CLEANUP expectation to
CANCELED is authorized here.

## Revision 3: finite real-pq mechanism controls

`TestPQCancellationMechanisms` uses pq 1.10.9 public driver interfaces and
Go 1.25.0 `database/sql` with the existing disposable PostgreSQL fixture. It
creates no persistent objects, roles or schemas and executes no performance
acquisition or target query. These are driver/library mechanism controls, not
production native-authority evidence or attribution of the historical failed
trial. The production concrete-driver check is not bypassed or changed.

| Fixed order | Native driver observation | Explicit public result | Existing contract mapping |
| --- | --- | --- | --- |
| Begin, then ordinary raw Rollback | nil | nil (raw driver.Tx) | No rollback cleanup failure |
| Begin, cancel lifetime, await complete cancel-packet forwarding, raw Rollback | exact driver.ErrBadConn | exact driver.ErrBadConn (raw driver.Tx) | An observed explicit error remains CLEANUP, not the cancellation-sentinel exemption |
| Begin through sql.DB, cancel lifetime, await automatic driver.Rollback entry and cancel forwarding, explicit sql.Tx.Rollback while driver blocked, release driver | exact driver.ErrBadConn | exact sql.ErrTxDone | The existing ended-context exception can preserve cancellation; the explicit caller cannot recover the hidden driver error |

Connect and the pre-transaction backend-PID query use Background contexts. BeginTx
receives a standard WithCancel lifetime context; database/sql forwards its derived
transaction context to pq in the automatic case. No query cancellation context is
used in these controls. There is no semantic timeout event; five-second channel
watchdogs and bounded cleanup conditions only detect a stalled fixture.

In pq 1.10.9, watchCancel stores the context error before opening its separate
cancellation connection. Complete proxy forwarding is therefore a public network
synchronization point after that store, not proof that the server processed the
cancel or acknowledged rollback. The raw control observes the subsequent actual
Rollback return. In the automatic control, driver entry proves database/sql already
won Tx.done; the gate blocks forwarding Rollback until cancellation was observed.
It does not infer the winner from scheduling or from Err-call counts. The
connector decorator forwards all optional connection interfaces pq 1.10.9
implements, including SessionResetter and Validator (which influence reuse).
No driver patch, private field, reflection, unsafe access or production hook is
used. The driver result is observed, not synthesized.

The normal and race controls each passed three leaves once locally with Go
1.25.0 and PostgreSQL 16.10. The initial launcher omitted opt-in and skipped the
suite; that was setup, not a native trial. Normal verification preceded a small
cleanup-only strengthening (checking the automatic result during fallback and
asserting the automatic lifetime context); race covered that strengthening.
A subsequent cleanup-only change registers lifetime cancellation after resource
cleanup and checks the expected raw fallback result. Its before-Rollback early
return branches were tested separately; no final immutable-head result is claimed.
These consultation measurements are not final-head validation. Final-head results
are recorded separately in PR124 and Issue123 after freezing the candidate.

Cleanup is registered before assertions. The automatic gate is released and the
rollback result joined on early failure; pool InUse is checked separately.
Native cancelled Rollback calls pq's transaction finish/close path; its return
is not a physical close-success certificate, and the fixture does not treat a
second raw Close on that closed connection as one. Proxy workers/listener are
joined, the backend PID is checked absent, and an independent SELECT control is
checked. There are no owned persistent objects to delete in this added suite.
Timeouts and independent cleanup failures remain test failures. These finite
resource checks do not establish general rollback acknowledgement or safe reuse.

The current production network_cancel fixture releases a held response after
cancellation forwarding. That fixes transport ordering but does not select the
winner of database/sql Tx.done. These controls consequently cannot establish a
universal CANCELED expectation for production native cancellation. A raw explicit
ErrBadConn is a separate fixed CLEANUP mapping, already covered by the strict
rollback regression; it is not interchangeable with the automatic-first public
ErrTxDone observation. The new decorator is not inserted into production acquire
and cannot establish its native authority. No two-result expectation, skip,
ErrBadConn exemption or driver-name exemption is introduced.

The missing historical whole-tree and failure-origin trace remain missing.
The adopted revision-3 answer to consultation
`370f63b7-8c2b-467c-94d3-d6715e0fc08d` explicitly changes the acceptance scope:
the finite improvements may undergo integration review without claiming that the
historical causal AC was achieved. That unmet AC moves from a prerequisite for
integration review to a separate, unresolved **release blocker** in
[Issue123](https://github.com/recoweft/goquent/issues/123#release-blocking-residual).
This is an acceptance change, not a statement that the original AC is unchanged.
PR124 remains draft for controller review, and PR122 remains unchanged. Even a
later controller integration of PR124 does not resolve or close Issue123.

The independent residual records the earlier PG cancel race result (17 passes,
one failure), the missing exact failing whole-tree and unknown cause. Later
30/30 non-reproduction and current mechanism results do not prove repair. Its
separate acceptance conditions are:

- Map production native acquisition's query/transaction lifetime contexts,
  public query/rows/rollback/close observations, invisible boundaries and
  proxy/server/pool cleanup.
- Separate cancel-only from independent transport/cleanup faults; require one
  fixed classification, sensitive regressions and assertion-failure resource
  observations for each observable event.
- Where public interfaces cannot guarantee Tx.done order while preserving the
  native concrete-type check, retain the unavailable synchronization point and
  concrete counterexample for specification review. Raw-driver/mock controls
  do not replace production native coverage.
- Preserve the historical attribution as unknown rather than demanding endless
  reconstruction. Keep the release blocker until a separate human release
  review explicitly accepts the finite guarantee, unobservable information,
  remaining risk and the preceding evidence.

An existing, readable unresolved tracking item with these boundaries is required
for this PR's integration review; resolving that entire separate work is not.
New final-validation classification/resource/watchdog failures remain current
integration blockers, not historical residuals. Existing native cancel/timeout/
disconnect expectations are unchanged; repeated success cannot erase failures.
Risk/precision/unknown1491 (the revision-2 measurement, not a new final count),
original generated_code/database example skips, companion evidence limits,
closureeligible=0, unadopted comparison guarantees, fixed BLOCKERS and human
release boundaries remain. Neither mechanism success nor CI success constitutes
historical causal repair or release eligibility.

A protected overlay mutation that swallowed the actual automatic driver rollback
error failed exactly that leaf (two other leaves passed, one parent failed,
exit 1, race). A separate forced assertion overlay failed all three leaves as
intended (one parent failed, exit 1, race) after raw Rollback or while automatic
Rollback was gated. All three reached their cleanup observations without extra
cleanup failures. The additional before-raw-Rollback forced-return control failed
two leaves as intended (one parent failed, exit 1, race); both exercised fallback
Rollback and completed their proxy/session/control checks. These are deliberate
failure controls, not failed native-cancellation investigation trials. No overlay
is committed. Existing strict rollback 13 cases, owned close 17 cases, actual
sql.Conn/Tx ordering and rows-failure controls are separate mock/unit evidence.
