# Context query observation

GQ-AI-12, PR2 (2/2), [Issue #94](https://github.com/recoweft/goquent/issues/94).
Observation is opt-in, independent of structural QueryPlan and estimated
PerformancePlan. It neither authorizes execution nor proves performance, safety,
tenant isolation, native database execution, or an N+1 cause. It runs no diagnostic
SQL and never calls EXPLAIN/ANALYZE. No monitoring dependency is required.

## Starting and ending a request

Use `observation.Start(parent, observation.Options{})` to obtain a derived context
and a Scope. Pass that context explicitly to ORM operations, or use
`Query.WithContext(ctx)`. Call `Snapshot` for current counters and `Finish` at the
end of the request. Both return a detached Summary. Starting on a context that
already contains a scope returns `ErrNestedScope`, even after Finish.

`Options.QueryLimit` is a copied `*int64`: nil means unlimited, a pointer to zero
means no dispatch is allowed, and 1 through 2^53-1 specifies a finite limit.
Negative or larger values are rejected. `DetectRepeated` separately opts into
bounded repeated-shape diagnostics. Changing an options pointer after Start does
not change the scope. Parent cancellation/deadlines are preserved.

Ordinary child contexts share the scope and its budget. Independent requests and
tenants must start from a parent without an observation scope. Deliberately
sharing a context shares its counters; no tenant values or labels are collected,
and no automatic tenant separation is claimed. There is no global registry.
Revert explicitly to the original context to stop applying the scope; Finish is
not a budget reset or an authorization mechanism.

Finish is idempotent and linearizes against completion. It freezes SQL counters,
marks tracked unfinished operations incomplete, and releases retained shape keys.
It does not close rows, cancel contexts, or commit/rollback transactions. Late row
completion and later queries do not rewrite that frozen interval. The budget
continues to be consumed and enforced on the same context after Finish. Its
post-Finish usage is intentionally not part of the frozen Summary.

Scopes, Snapshot, Finish, Drain, and independent query dispatches can run
concurrently. Query builders remain mutable: sequential reuse is supported;
concurrent mutation/use of the same Query is not made safe by this API. Rows/Row
iteration and scanning require caller serialization, as before. Scope completion
can safely race with a caller consuming its rows.

## What one query means

One query is one Goquent call to an Executor's Exec, Query or QueryRow method,
including its context variant. This is not the number of server round trips,
rows, or database/sql internal retries. A failed connection attempt still counts
as a dispatch. An internal database/sql retry is invisible. A single bulk SQL
statement counts once; compound operations and Goquent-level retries/replays
count each actual dispatch. A business callback itself counts zero; its covered
ORM operations count normally. Begin, Commit and Rollback are separate transaction
boundaries and do not consume the query budget.

Existing policy, Strict, tenant, private plan binding, expiry and one-use gates
remain authoritative. Budget reservation occurs atomically immediately before
each covered dispatch, after applicable gates. Exhaustion returns
`ErrQueryBudgetExceeded` / `QUERY_BUDGET_EXCEEDED` without calling that Executor
method. Reservations are not refunded on failure. Already-dispatched SQL is not
undone by the budget. Existing owned transaction error paths perform their usual
rollback; external transactions acquire no new commit/rollback side effects.
There is no per-query time limit, result-row limit or server-wide SQL limit here.

Before saturation, Summary obeys these relationships:

- `attempted = dispatched + rejected`: attempts reaching a dispatch reservation;
  rejected counts budget refusals at this boundary.
- `dispatched = completed + incomplete + in_flight`.
- `completed = successful + failed + partial`. A partial completion has finished
  cleanup but does not establish full result consumption.
- `handoffs` and `tracking_limited` are subsets of incomplete.
- `preflight_rejected` is separate from reservation attempts. It covers recorded
  terminal preparation/common-gate refusals, not every application validation,
  callback error, or diagnostic Plan failure.
- `tx_boundaries` and `tx_errors` are independent of query counts.

Snapshot shows in-flight counts; it does not fabricate successful completion.
Duration/count sums saturate at 2^53-1 with `saturated` visible. Missing metric
values have `known=false` and zero storage, which is not an observed zero.

## Coverage and lifecycle

Every public event and summary states `goquent_executor_boundaries`. Custom
executors may do more, less, or no database work; no native provenance bit is
inferred from their type, label, result or externally supplied JSON.

| Path | Observation boundary |
| --- | --- |
| Query reads/writes, Count, InsertGetId, scoped and generic reads/writes, typed operation adapters | Common private dispatch and owned result handling |
| Bulk INSERT/UPSERT and RETURNING helpers | One dispatch per generated statement; consumed rows/scans separate from existing write expectations |
| Raw ExecContext | Dispatch, existing result inspection, return; returned rows unknown |
| Existing Raw QueryContext and QueryRowE | Dispatch/budget and one handoff/incomplete event; subsequent concrete sql.Rows/sql.Row use is unknown |
| New DB.QueryObservedContext | Same raw gate/private dispatch, with tracked Rows and no underlying-rows escape |
| Existing DB.QueryRowContext / orm.Row | Deferred Scan completion; Err alone does not complete it |
| Generic Raw SelectOne/All | Private owned scanner path, not an intermediate handoff event |
| AtomicWrite storage checks | Each internal metadata SQL consumes budget and records an internal event |
| CommandStore creation/run/replay | Internal catalog/ledger SQL plus common business/hook SQL; no double counting |
| Owned context transactions, manual orm.Tx from BeginTx | Begin and finalization outcomes, separately from statements |
| External sql.Tx/custom Executor via Goquent helpers | Goquent boundary only; external commit and physical execution unknown |
| Plan/DryRun/typed compilation | No query observation or dispatch |
| Explicit ExplainPerformance, migration, direct SQLDB/Executor/sql.Tx calls | Outside this initial coverage and budget |

A context supplied to BeginTx is not implicitly injected into non-context query
terminals. Use the context on each operation. Direct access through embedded
`driver.Tx`/`sql.Tx` bypasses the manual orm.Tx finalization observation.

`dispatch_ns` measures the Executor method call, including client connection wait.
`lifecycle_ns` runs from dispatch start through observed scanning/checks/cleanup.
It is unknown for handoff or an unfinished operation. Summed durations of parallel
queries are not scope wall time. `wall_ns` measures the observation interval.

Rows.Next true increments **consumed**. Successful Rows.Scan calls increment
**scans**, including repeated successful scans of one row. Mapping/decoding after
a successful Scan can still fail the operation. Neither counter is RowsAffected.
The full **returned** count becomes known only when all result sets were consumed
and cleanup/Err checks succeeded. Next false alone is insufficient: there may be
another result set. The wrapper does not advance NextResultSet for observation.
Call NextResultSet according to normal database/sql iteration semantics; its final
false plus Err/cleanup permits a complete count. Advancing past a partially read
set keeps the total unknown. Repeated Close/Err/Scan do not issue duplicate final
events. First/One and early Close report consumed rows but an unknown total.
Internal scanners that only iterate the first result set conservatively retain
that unknown total, even if all its rows were consumed.

Scan, rows.Err, automatic-close errors visible via Err, explicit Close errors,
cancel and timeout are recorded with fixed classifications. Original API errors
remain original; no new public hook retains their causes. Cleanup failure is not
a successful observation, even where a compatibility helper previously ignored
its deferred Close error. Dispatch failures never become zero-row successes.

QueryRow.Scan success observes one consumed row; ErrNoRows observes zero. Other
Scan failures do not infer consumed rows. QueryRow discards remaining rows, so it
never proves the full returned count. An unscanned Row stays in-flight until
Finish marks it incomplete. Go 1.26.4 database/sql's Row.Scan/Rows.Next/Err/Close/
NextResultSet implementation was inspected for these cleanup boundaries.

Exec returned rows stay unknown. `affected` is known only when an existing write
expectation already called RowsAffected; observation adds no custom Result method
call. A successful SQL event is not a commit. A later rollback does not erase a
SQL dispatch; owned atomic/command result success still requires the existing
checks and transaction finalization.

## Repeated shapes are only a suspicion

With DetectRepeated, the private already-built single-table SELECT structure is
examined without rerendering SQL. Initial coverage requires bounded ordinary
explicit columns, ordinary ordering and predicates, LIMIT 1..10000 and OFFSET
0..10000. Raw, joins, unions, subqueries, expressions/functions, aggregates,
locking/grouping and unknown/opaque inputs remain unknown. These restrictions
apply only to the diagnosis, never to ordinary CRUD execution.

Canonical keys separate built-in dialect, actual sql.DB/sql.Tx identity, target,
projection, operators, placeholder order/IN arity, ordering and limits. Identity uses
standard-library weak pointers without retaining or dereferencing a pool/Tx or its
context; identity equality survives collection without address-reuse collisions. Custom
executor identity cannot be established by a label, so it remains unknown. Keys
contain no parameter values; custom Valuer/Stringer/Marshaler methods are not
invoked. Private structural identifiers are never published. Complete canonical
keys, not a hash alone, determine equality. The fifth successfully cleaned-up
matching SELECT records `N_PLUS_ONE_SUSPECTED`. Early single-row consumption may
qualify after successful cleanup, without claiming a full row count. Failed,
handoff, unfinished and untracked observations do not qualify.

Only suspected-group count, maximum repetition and unknown count are published;
there is no public shape/group identifier or SQL/value fingerprint. Repeating the
same values can trigger the suspicion. It proves neither distinct values nor
parent/child dependencies, a cause, committed data, or a performance improvement.

## Bounds and hook delivery

The scope retains at most 1024 in-flight entries, 128 canonical keys (at most
2 KiB each, 256 KiB key bytes total), and 256 queued events (plus one active Drain batch of at most 256). Shape traversal stops
at depth 32 / 256 nodes. Management objects have additional bounded overhead;
caller wrappers and driver/server resources are not constrained by these limits.
Tracking overflow does not refuse otherwise allowed SQL: it records
tracking_limited/incomplete and keeps enforcing the budget. New shapes beyond the
cap stay unknown/truncated; retained shapes can keep counting. Finish releases
keys and pending entries and never retains SQL, arguments, rows, contexts or raw
errors in the scope. Pending entries store only safe metrics and private bounded
shape bookkeeping.

Call `Scope.Drain(func(Event))` explicitly to deliver a bounded batch outside SQL
execution and scope locks. Drain returns delivery-attempt and recovered-panic
counts for that call; these are not frozen SQL counters. A slow hook delays Drain,
not query dispatch. Concurrent/reentrant Drain returns ErrDrainBusy. Panics are
recovered without formatting or retaining their payload, with no redelivery.
A hook closure may issue ORM queries: they consume the ordinary budget, and new
events wait for a later batch. No context, SQL, arguments, errors or arbitrary
metadata are passed as hook arguments.

A full event buffer drops new events and increments dropped/truncated; aggregate
query counts and budget enforcement remain. Every retained event gets at most
one delivery attempt. There is no durable exactly-once delivery or guarantee of
all events. Finish allows draining already retained events, including later Drain
failures, without changing frozen SQL statistics. Collector-only use is supported.

## Publication and compatibility

Event and Summary have independent strict v1 JSON formats. Their hook arguments
are already public-safe, not sensitive objects hidden behind a display method.
Snapshot, Drain, String, Format and JSON reproject closed classifications and
bounded numbers after exported-field mutation. Decoders reject unknown fields,
versions, duplicate keys, trailing data, excessive size/depth and invalid values.
No source API or view-to-execution capability is added. Direct extraction of
caller-mutated fields or methodless application casts is outside provided display
methods; supplied JSON never restores private acquisition evidence.

SQL, identifiers, DSNs, tenant/parameter values, raw error causes, callback values
and keys are absent from hook/view data. Error classification does not call
Error, Is, As or Unwrap on application errors; unfamiliar causes remain unknown or
phase-specific. Existing error identity and execution APIs remain intact.
Executor's six concrete signatures, public scanner signatures, QueryPlan and
PerformancePlan wire formats, CLI/MCP and ordinary transaction semantics remain.
The internal scanner implementation moved behind a private reader interface.
No production dependency or automatic DB operation was added.

See [validation and benchmark evidence](query-observation-validation.md). This
feature and its evidence do not close Issue #94 or adopt whole-Issue acceptance.
