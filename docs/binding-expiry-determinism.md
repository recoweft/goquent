# Deterministic final binding checks

GQ-MAINT-05 PR1 (1/1), [Issue #101](https://github.com/recoweft/goquent/issues/101).

## Investigation and limits

The historical PR95 validation record reports two race-suite failures: the
`expiry-last` leaf and its parent, without a race-detector warning. It does not
retain the returned error or executor counters. The failing assertion required
both `errors.Is(err, ErrBindingExpired)` and all six executor counters to be zero;
the historical record does not identify which operand failed. This change does
not claim to have recovered those missing observations or proven host clock drift.

On Go 1.26.4, the unchanged PR100 head (whose tree equals merged main
`88bb17c51afd024b32dddbea66b1846bab1f4947`) ran the old owner/last-dispatch test
with `-count=20 -shuffle=101`, once normally and once with `-race`. Each run had
220 test/subtest passes, zero failures and zero skips. The original intermittent
failure was not reproduced. These passes are not evidence of a repair.

Source tracing establishes two independent weaknesses in the old fixture:

1. `prepareBinding` reserves the handle before checking ownership, expiry and
   context. It re-plans and compares private material, copies the minimum expiry
   into the fresh execution record, and checks context and expiry again. Its
   first two context checks precede dispatch. The third `Err` call for INSERT is
   `runWrite`'s precheck; `bindExecution` checks again afterward. Therefore a
   callback on call three does not specifically select the final bind boundary.
2. The fixture builds a monotonic deadline using `time.Now().Add` and sleeps
   using `time.Until`. Preparation compares monotonic readings when available.
   Final `checkExecution` compares against `time.Now().UTC()`, which removes the
   monotonic reading and therefore compares wall times. Go's local `time`
   package documentation explicitly distinguishes these clocks. A monotonic
   wait plus one millisecond does not establish the wall-clock predicate if
   wall time adjusts backward. For example, 1001 milliseconds of monotonic
   progress and only 999 milliseconds of wall progress satisfy the wait but
   leave a one-second wall deadline unexpired. This is a finite causal
   counterexample to the fixture assumption, not an observed clock adjustment.

The repair targets these fixture assumptions. It does not diagnose an execution
contract defect or change clock semantics. Historical failure attribution beyond
these findings remains unverified.

## Regression boundary

`TestBindingFinalDispatchChecks` uses the existing private `prepareBinding`,
`checkExecution`, `executeResult` and `bindExecution` boundaries. No production
seam is added. Each case first proves successful preparation and an executable
record, the original context and deadline, a reserved handle, an unused execution
record and zero executor calls. It then changes only that record's deadline to
a fixed past instant, or cancels its real context. There is no sleep, context-call
counter, concurrent mutation or global hook. A fixed far-future deadline supplies
the positive case without a short scheduling budget.

Both dialects cover dispatch and direct final bind. Positive dispatch calls the
context executor exactly once; expiry/cancellation call none and preserve their
error identities. Direct bind independently proves its last check and execution
CAS behavior. A rejected execution record remains unused; the already reserved
handle remains consumed on all paths. Existing six-family, context/owner,
concurrency, DB-error consumption and approval-ceiling tests remain in place.
The owner cases now use a real context instead of the synthetic counter context.

The strict `!expires.After(now)` predicates are unchanged: equality is expired,
with no grace period. Exact equality at a live clock tick is source-inspected,
not claimed as a controlled-clock test. Owner/context checks and the preparation
expiry check retain their existing precedence; cancellation precedes expiry in
the final gate. No production source, public API, wire/hash, authorization,
tenant/Strict, observation, schema/preflight or review/manifest contract changes.
MAINT03's finite test-executable exception and MAINT04 Verify checks are untouched.

## Validation evidence

All counts include parent tests and subtests. Local targeted tests use mocks;
they are not native DB results.

| Check | Result |
| --- | --- |
| New owner/final checks, `-count=100 -shuffle=101` | 2200 pass, 0 fail, 0 skip |
| All `TestBinding` tests, `-race -count=100 -shuffle=101` | 8900 pass, 0 fail, 0 skip |
| Temporary-copy mutation removing final expiry gate | Expected failure: 8 pass, 5 fail (four expiry leaves and parent), 0 skip |
| Temporary-copy mutation removing final context gate | Expected failure: 8 pass, 5 fail (four cancellation leaves and parent), 0 skip |

Mutation checks change only a disposable copy, never the delivered production
code. Each removes one final gate while retaining preparation. All corresponding
negative cases detect the defect, including zero-executor and consumption checks.
They establish regression sensitivity, not reproduction of the historical failure.

Fresh full/race/vet/regeneration/canary, explicit-three-DSN sequential native DB
checks, exact-commit ordinary/strict-diff review, example manifest verification,
and final-head CI results are recorded in the PR/report. Review risk/precision
failures or unknowns must remain visible; passing binding tests do not resolve
them. Native performance restricted-role/custom-operator/cast/network-fault
coverage is outside this change. No retry, skip, suppression, threshold relaxation
or mass baseline update is part of the repair. No merge or Issue close is implied.
