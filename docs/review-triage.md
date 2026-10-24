# Unresolved review evidence and repair priorities

GQ-MAINT-06 PR1, [Issue #109](https://github.com/recoweft/goquent/issues/109).
This is a bounded diagnosis of the current review contract, not a repair of its
gates or a release decision. See [staged validation](staged-release-validation.md),
[typed loading](static-review-types.md), [precision](static-review-precision.md)
and [source comparison](static-review-ci.md).

## Fresh evidence and measurement boundary

Fresh local collection used Go 1.26.4 after both integration workflows for PR108
succeeded. Exact commit identities and command exits are recorded in the PR and
the protected local collection. Counts below come from this collection, not the
historical PR108 totals. The initial comparison used the preceding main and the
PR108 merge, with a clean checkout and a nonempty exact-commit patch.

| Independent check | Fresh result |
| --- | --- |
| Ordinary CLI, high risk threshold | Exit 1; full report; risk failed; input did not fail; precision threshold not selected; no comparison requested |
| Strict comparison, high risk and partial precision thresholds | Exit 1; full report; input did not fail; risk and precision failed |
| Strict finding classifications | unknown 1,474; new 0; worsened 0; unchanged 0; matched 0 |
| Complete source report, separate existing in-process analysis | 1,474 unsuppressed findings; 0 suppressed; 1,165 operations |
| Manifest, supplied example schema and policy | CLI exit 0; generated_code and database skipped |

The built CLI computes gates from complete private reports. Its public CI view
only supplies supplementary classification totals; it is never fed back as an
analysis input. Ordinary's zero comparison counters mean **not compared**, not
zero unresolved findings. Its unselected precision gate is not precision success.

The source report was collected separately with the existing `review.Run` API,
using the same ordinary discovery input and no evidence producer, configuration,
manifest or baseline. Its full finding and operation arrays, rather than the
256-item public projection, supplied the following histograms. This is an
independent measurement, not access to the opaque CLI result or a serialized
substitute for its private gate. Preserve complete input trees, patch, source
report, command mapping and real CLI exits locally. Do not upload them.

## Three principal groups

These groups overlap: risk is a finding severity, precision is knowledge about
findings and operations, and unknown is a comparison outcome. Do not sum their
populations or label every unknown a defect or harmless result.

### 1. Raw and destructive structural risk

There are 181 findings at or above high: high 147, destructive 29, blocked 5.
The remaining 1,293 findings are medium. The dominant high code is
`RAW_SQL_USED` (143). Other gate-bearing codes are `DESTRUCTIVE_SQL_DETECTED`
(28), `MIGRATION_DROP_COLUMN` (1), `DELETE_WITHOUT_WHERE` (5) and
`WEAK_PREDICATE` (4). These are finding counts, not distinct executions or bugs.

The existing risk engine emits high for a recognized raw operation, with
independent destructive and weak-predicate checks; static review also discovers
SQL and supported plan inputs. Test sources are included in ordinary discovery.
No count of intentional negative fixtures versus unintended application behavior
was obtained. The raw code alone cannot establish that distinction, business
intent, actual dispatch or runtime authorization.

Minimal synthetic evidence already exists in
`TestRunReviewsRawSQLAndQueryPlanJSON` and `TestTypedIdentityAndTracking`.
The latter also excludes unrelated methods with similar spellings. Use those
fixtures without publishing their source payloads or substituting them for the
whole-tree run.

**Next unit, P1:** a bounded raw-call provenance investigation, starting with the
143 dominant findings, to separate recognized direct calls, serialized inputs
and intentional negative test cases locally. Repair a demonstrated recognition
error only in a separate task. Positive regression: supported raw calls retain
their high/destructive warnings. Negative regression: unrelated receiver methods
and non-plan JSON do not acquire raw warnings. Do not exclude tests, lower the
threshold or suppress the population to obtain green.

### 2. Missing static knowledge and unsupported loading

Finding precision is partial 1,277 and unsupported 197; precise 0. Complete
operation axes are:

| Axis | known | partial | unknown | unsupported |
| --- | ---: | ---: | ---: | ---: |
| SQL | 18 | 689 | 261 | 197 |
| Columns | 24 | 50 | 1,091 | 0 |
| Bindings | 18 | 0 | 1,147 | 0 |
| Policy | 0 | 0 | 1,165 | 0 |

All 1,165 operations record missing `runtime_policy` and `evidence_missing`.
Other missing-reason memberships are `database_specific` 496, `static_unknown`
50 and `dynamic_value` 39. Reason memberships are `static_unknown` 470,
`opaque_operation` 224, `type_unavailable` 190 and `unverified_input` 2.
Memberships can overlap and are not an exclusive partition. Unsupported findings
carry `type_unavailable` 190 or `opaque_operation` 7.

The conservative precision adapter requires every axis to be known. Supported
literal chains therefore cannot become precise merely because their SQL shape
is known; standalone CLI review does not supply runtime evidence. Fallback
operations retain raw/opaque/loader findings. Dynamic values, generic and
validated operations, opaque helpers and unavailable type information have
documented limits. These classifications do not establish that all 190 type
failures share a cause. The loader discards compiler diagnostics, so a breakdown
into build selection, missing types, malformed fixtures, dependency failure,
budget or timeout was **not obtained** from the existing report. Do not invent it.

Minimal synthetic evidence: `TestReviewPrecisionUnknownAndWire` retains unknown
dynamic bindings; `TestPlannerCorrespondenceSession` retains unknown policy
even when a planner sample matches; `TestLegacyTypeFailureAndCanceledLoad`
preserves unavailable-type coverage. These reproduce contract conditions, not
the root cause of every repository finding.

**Next unit, P1:** isolate the 190 `type_unavailable` findings with local,
bounded package-selection diagnostics before proposing a loader repair.
Positive regression: buildable typed calls remain recognized across test
variants. Negative regressions: cancellation, unavailable imports/types,
excluded inputs and budget exhaustion remain non-success; custom package drivers
must not receive a native-loader exception. Missing policy/runtime evidence
remains a separate contract limit, not a target for automatic promotion.

### 3. Conservative whole-tree comparison unknown

The exact comparison produced 1,474 unknown findings with input_failed=false.
In `RunCI`, input correspondence and per-finding classification are distinct.
`sameContext` additionally requires equal complete source trees. A valid
nonempty edit changes that context; a finding with the same diagnostic and
unchanged containing source can remain unknown instead of unchanged. Other
unknown conditions include unavailable operation identity, unsupported precision
and ambiguous previous keys. No baseline was supplied.

The existing result has no per-reason unknown histogram. The 1,474 total cannot
be partitioned among those branches from the public view or source report;
that partition is **not obtained**. Source inspection establishes the condition,
and the synthetic comparison regressions demonstrate it, but neither supplies
an invented repository-wide causal count. A successful input gate is not proof
of full comparison precision.

`TestCIChangedSourceTestVariants` is an existing nonempty-diff reproducer: input
correspondence succeeds while findings remain unknown after a context change.
`TestCIChangedSourceNewExternalContextRemainsUnknown` checks the separate
fail-closed external-input case. MAINT03 exempts only independently proven unused
native test executables from cross-session external equality. Metadata agreement,
absence of incoming/ordinary/shared/consumed roles and same-session byte checks
remain mandatory. It does not normalize generated files or exempt test variants,
ordinary dependencies, custom drivers or every testmain-shaped file.

**Next unit, P2:** obtain a private branch-reason partition on a bounded synthetic
comparison before considering any correspondence change. Positive regression:
listed context-only changes preserve full head gates and unknown findings.
Negative regressions: unlisted changes, consumed external inputs, mutations and
ambiguous matches remain fail-closed. Any proposal to certify narrower context
or change comparison meaning requires specification consultation in its own
authorized task; this PR adopts no such repair.

## Reproduction and validation

Use the existing staged runner with fresh protected output directories. Select
ordinary and manifest separately; strict additionally requires a clean exact
head, independently resolved base and nonempty diff. Keep generated artifacts
outside both analyzed roots. Record actual built-CLI exits, not `go run` wrapper
exits. For complete classification, collect the existing source API locally and
aggregate only fixed codes, normalized axes/reasons and counts. Never import a
public view as a source report or publish source JSON, SQL, identifiers, values,
absolute paths or arbitrary errors.

Existing synthetic regressions can be selected without connecting to a DB:

```sh
go test -p 1 -count=1 ./orm/review -run 'Test(RunReviewsRawSQLAndQueryPlanJSON|TypedIdentityAndTracking|ReviewPrecisionUnknownAndWire|PlannerCorrespondenceSession|LegacyTypeFailureAndCanceledLoad|CIChangedSourceTestVariants|CIChangedSourceNewExternalContextRemainsUnknown|NativeTestExecutableRoles|CICaptureRetainsExactBytes)$'
```

Capture test stdout/stderr privately too. Fresh full/race, vet, regeneration,
canary, runner regressions and final ordinary/strict/manifest results are
reported separately in the PR, including Go/DB versions, testskip/no-test and
local/native/mock/CI scope. Full/race require the existing disposable fixtures,
all three explicit DSNs, sequential stages and `-p 1`; no global GOFLAGS `-json`,
new service provisioning or production connection is appropriate.

No historical testmain or expiry failure cause is established by a later passing
test. Native restricted-role, custom operator/cast and network-fault performance
cases remain unverified. Manifest generated_code/database coverage stays skipped.
All fixed staged-runner blockers and the separate human release boundary remain.
This diagnosis adds no public API, JSON/SARIF/wire/hash format, runtime semantics,
loader repair, suppression or baseline update, and does not claim release
readiness or whole-Issue acceptance.
