# Unresolved review evidence and repair priorities

GQ-MAINT-06 PR1, [Issue #109](https://github.com/recoweft/goquent/issues/109).
This is a bounded diagnosis of the current review contract, not a repair of its
gates or a release decision. See [staged validation](staged-release-validation.md),
[typed loading](static-review-types.md), [precision](static-review-precision.md)
and [source comparison](static-review-ci.md).

Current-state update: [GQ-MAINT-15 disposition](release-blocker-disposition.md)
records a fresh complete-private census and remaining decision boundaries.
The measurements below belong to their stated historical heads. In particular,
their original-example generated-code/database skips predate PR127: both inputs
are now explicitly supplied for the original saved model. They are no longer
current missing axes; handwritten-model/live-identity/coverage limits remain.

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

### GQ-MAINT-07: fresh type-loading attribution

GQ-MAINT-07 PR1, [Issue #111](https://github.com/recoweft/goquent/issues/111).
The fresh complete `review.Run` source report again contains 1,474 findings and
1,165 operations. This independently measured population contains 190
`type_unavailable` findings and 190 reason-bearing operations, with zero duplicate
finding locations. These are separate report arrays, not 380 failures. The
fallback creates at most one operation per location; the reason does not by itself
identify a failed package load. The preceding 190 was a reference, not the input
to this census.

| Exclusive attribution of the 190 findings | Count |
| --- | ---: |
| Complete selected loader state; type-confirmed raw call with nonliteral SQL operand | 189 |
| Complete selected loader state; local cause not established by the bounded matcher | 1 |
| Selected incomplete package / missing selected syntax / unmatched input | 0 |
| Total | 190 |

All 190 findings were reproduced by the existing tracker using diagnostic-loaded
type information at the same physical locations. Independent existing-loader and
diagnostic-loader file states agreed. For this population, selected packages have
no IllTyped flag, package Errors or missing TypesInfo. Twenty-two finding locations
have multiple loaded syntax variants; these overlap the attribution rows and are
not 22 additional failures. The bounded parser made 1,857 calls, with zero observed
parse-budget hits or cancellation. No finding was attributed to a compiler error
string. The residual one is **unknown**, not repaired or silently assigned to the
dominant cause. This does not establish that arbitrary broken inputs load correctly.

The dominant mechanism is in the existing raw-call tracker: a recognized raw API
whose SQL operand cannot be reconstructed as a supported string literal uses the
same `type_unavailable` reason as unavailable types. The finding is retained in
fallback operations. Typed AST identity, operand classification and tracker replay
support the 189 count; message text alone does not. The synthetic
`TestDiagnosticDynamicRawCounterexample` proves that a buildable typed call with a
dynamic operand produces that reason, while replacing the operand with a literal
retains the raw-risk warning without that reason. Neither case acquires runtime
policy evidence or precise report status.

**Next repair unit, P1:** separate raw-operand reconstruction diagnostics from
actual loading failures in the tracker/fallback diagnostic classification. Scope
is the reason and associated diagnostic explanation, preserving unsupported SQL,
unknown runtime policy, risk/precision gates and complete private decision inputs.
Positive regression: supported literal raw calls retain their warnings and typed
recognition. Negative regression: dynamic operands, invalid argument forms and
unavailable types remain non-success and cannot become precise. Any change to the
closed public reason vocabulary or its interpretation needs its own specification
consultation before implementation. This PR makes no production diagnostic or
loader change. Investigate the residual one privately before assigning it to this
repair unit; it supplies no evidence for a loader repair.

The test-only `diagnosticLoad` mirrors the existing loader, including
`LoadSyntax | NeedForTest`, `Tests=true`, `-mod=readonly`, module grouping, sorted
package selection, test variants, overlays and unchanged parse/load budgets.
`TestDiagnosticLoaderParity` removes only trace statements and checks normalized
syntax against the production function to detect drift. Collection uses ordinary
`./...` discovery, the same root and session input snapshot, Go 1.26.4, cached
dependencies with GOPROXY=off/GOTOOLCHAIN=local, and no configuration, baseline,
manifest or runtime evidence producer. Separate loads are not the same type-object
universe: the harness checks selected-file state correspondence, replays existing
tracking and rejects changed captured inputs. It does not claim that standalone
`go list` proves loader success, or that this finite run covers every environment.

`TestDiagnosticSelectionMechanisms` covers ordinary/internal/external typed test
variants, an intentionally invalid internal variant, missing import/type and an
excluded input. The invalid internal variant retains known typed raw recognition
and unsupported coverage. `TestDiagnosticRefusalBoundaries` covers cancellation,
file budget exhaustion and an empty custom driver without granting it a native
exception. Existing typed, partial-load and driver regressions remain applicable.
These negative fixtures explain mechanisms; none is asserted to account for the
190 repository findings.

For a fresh local census, create a mode-0700 output directory outside the input
root and set `GOQUENT_TYPE_DIAGNOSTIC_OUTPUT` only for
`go test -p 1 -count=1 ./orm/review -run
'^Test(Diagnostic.*|LocalTypeLoadingCensus)$'`. Capture stdout/stderr locally too.
The opt-in test writes new mode-0600 source report, input snapshot, selected paths,
compiler/package diagnostics, context and aggregate files; it refuses existing
output files. Without this opt-in the census is skipped. Do not upload these
artifacts or publish package/file mappings. The ordinary CLI, CI wrapper, MCP and
public formats do not use this diagnostic path. Full/race, vet, regeneration,
canary, runner and exact ordinary/strict/manifest evidence are reported separately
in the PR, including failures and skips.

This attribution does not explain all 1,474 comparison unknowns or all 181 risky
findings. Missing runtime policy remains independent. Historical testmain/expiry
causes, generated_code/database manifest skips, native performance gaps, fixed
BLOCKERS and the human release boundary below remain unchanged.

### GQ-MAINT-08: bounded dynamic raw attribution repair

GQ-MAINT-08 PR1, [Issue #113](https://github.com/recoweft/goquent/issues/113).
The production raw tracker now uses the existing `dynamic_value` reason when a
recognized raw call has complete selected type information and only the SQL
operand cannot be recovered by the existing `stringLiteralValue` implementation.
Its message and hint are fixed strings describing unsupported SQL reconstruction.
The unsupported code, medium severity, physical location and unsupported SQL /
precision remain unchanged. Incomplete or missing types and invalid argument
forms retain `type_unavailable`; literal raw warnings and the recognized receiver
set are unchanged. This does not extend SQL analysis, change the loader, or grant
runtime policy evidence or authorization.

Fresh opt-in censuses and complete source reports were collected locally on Go
1.26.4, before and after the repair. No previous count was an expected test value.
The initial selector-only matcher independently reproduced the old 189 identified
calls and one unknown. The residual was then checked by typed AST identity: it is
a same-package, unqualified `NewRawPlan` call with a nonliteral operand. The
extended test-only matcher covers that existing recognition shape and has a
synthetic regression. Production recognition was not expanded. This establishes
the residual's local cause; it does not establish general loader completeness.

| Complete report population | Before | After |
| --- | ---: | ---: |
| Findings / operations | 1,474 / 1,165 | 1,474 / 1,165 |
| `type_unavailable` findings / operations | 190 / 190 | 0 / 0 |
| `dynamic_value` findings / operations | 0 / 0 | 190 / 190 |
| Unsupported findings / operations | 197 / 197 | 197 / 197 |
| Partial findings | 1,277 | 1,277 |
| High / destructive / blocked findings | 147 / 29 / 5 | 147 / 29 / 5 |

All finding codes, risk counts and SQL/policy axis counts are unchanged. Policy
remains unknown for every finding and operation. Both censuses have zero selected
incomplete packages, missing type information, parse-budget hits, cancellations,
loader-state mismatches and unmatched tracker replay locations. Multiple syntax
variants still overlap 22 finding locations. The revised matcher confirms 190
typed dynamic calls, including one unqualified call, with no remaining unknown
in this bounded population. The two report arrays are not additional failures.

The fallback regression checks both findings and operations, preserving
`unsupported`, unknown policy, `runtime_policy` and `evidence_missing`. Synthetic
coverage includes context calls, method expressions, bound methods, unrelated
receivers, literal SQL, actual load/parse failures and dynamic SQL in an incomplete
package. Existing cancellation, budget and loader parity guards remain in force.
Actual CLI subprocess tests retain medium-risk and precision exits of 1; a
medium-risk dynamic finding still yields exit 0 at the high-only risk threshold
while keeping the unsupported finding visible. Literal raw SQL still fails the
high-risk gate. Public v1, v2 and SARIF retain their existing fixed projection;
source locations remain sensitive source API data. MCP only accepts SQL text in
its existing review route, not source files; its existing redaction/unknown-evidence
regressions remain applicable without adding a source-review route.

Detailed full/race, vet, regeneration, canary, runner, ordinary, clean exact strict
and manifest outcomes are reported separately in the PR. Protected source reports
and diagnostic logs remain local, outside the analyzed roots, and are not uploaded.
Ordinary risk, strict risk/precision, comparison unknowns and manifest
generated_code/database skips are not discharged by this classification repair.
Native restricted-role/operator/cast/network-fault cases and historical
testmain/expiry causes remain unverified. Fixed runner BLOCKERS and the human
release boundary remain unchanged. No threshold, suppression, baseline, exclusion,
public API, wire/hash/version or runtime authorization change is included.

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

### GQ-MAINT-11: private comparison branch census

GQ-MAINT-11 PR1, [Issue #119](https://github.com/recoweft/goquent/issues/119).
The initial clean exact comparison used main `1a9702e` as base and diagnostic
commit `ffde82f` as head, after that main's CI and Docs both succeeded. Go 1.26.4
and the same built CLI produced ordinary exit 1 (risk failed; precision not
selected) and strict exit 1 (input valid; risk and precision failed). The original
example manifest returned exit 0 with generated_code/database still skipped.
Collector completion remained release-blocked, with exit 1.

An internal observation point records RunCI's actual final state for every head
list item. The opt-in test-only census obtains complete private reports, source
trees, loader captures and exact diff from that same implementation. It never
uses public projection as input. A separate diagnostic mirror checks its predicted
state against **every actual state**, including the existing ordered new,
worsened, unchanged, changed-source new and final matched override. Any difference
fails collection. Default RunCI passes no observer and retains its original gates,
comparison branches, operation coverage and public output vocabulary.

The unit is a head finding-list item, including suppressed-list items separately
when present. Private ordinal, canonical key, source bytes and operation identity
retain correspondence; they are not published. This collection had 1,489 primary
items, no suppressed items and 1,178 operations. All 1,489 were unknown; other
comparison states were zero. Ordinary and strict complete head findings and
operation counts agreed. The historical 1,489 was not an expected test value.

| Exclusive attribution of actual unknown items | Count |
| --- | ---: |
| Head comparison identity unavailable | 998 |
| Whole-tree context changed, with known matching diagnostic | 491 |
| Other attribution categories | 0 |

These are ordered blocking-condition attributions, not new production reasons or
proof that each item has only one cause. After preserving all existing positive
classification precedence, the diagnostic attributes unknown to invalid input,
missing file, ambiguous prior key, unavailable head identity, unavailable prior
identity, whole-tree context mismatch, then remaining diagnostic mismatch.
The primary counts sum to the actual unknown count.

Overlapping memberships were whole-tree change 1,489, same diagnostic 1,489,
missing head operation identity 998, unavailable prior identity 998 and
unsupported precision 208. Unsupported is contained within the 998 here;
it is not an additional population. No ambiguous prior key or input-correspondence
failure occurred in this pair. The 998 include supported-but-partial diagnostics,
so unsupported alone cannot explain that group. Risk counts are a separate axis.

Same-context and listed nonempty-context fixtures exercise unchanged versus
unknown with full risk/precision gates retained. Unlisted input remains an input
failure. Unsupported, missing-operation and duplicate suppressed-list fixtures
exercise independent unknown conditions and ambiguous keys. Existing external
input, same-session mutation and finite native-test-executable role regressions
remain controls; no external-consumed input exception is added. Test source and
all raw artifacts remain protected local evidence, outside both analyzed roots,
with no group/other access and no upload.

No contract-preserving production defect has been established by these counts.
Certifying narrower context alone would not supply the missing operation identity
for 998 items, and supplying identity alone would not establish unchanged context.
Promoting these findings would require a concrete comparison-contract decision;
this investigation does not adopt one. The delivered specification decision selects a diagnosis-only PR under the
current contract. This is not a permanent decision to require whole-tree equality,
and does not authorize narrower context, inferred operation identities or
unknown promotion. The initial counts above remain tied to their measured commit;
final exact-head measurements and fresh validation belong in the linked PR.
Population agreement and successful parity are not identity or context proofs.
This is not acceptance or release readiness; all existing fixed blockers and the
separate human boundary remain.

Two separate future design candidates need explicit decisions before implementation:

| Candidate | Finite scope and required proof | Invalidation, refusal and compatibility |
| --- | --- | --- |
| Missing comparison identity | First choose one supported diagnostic producer; bind an operation kind and canonical location to independently analyzed source, its selected build variant and consumed dependencies. Do not infer identity from diagnostic text, risk, SQL spelling or public fields. | Changed or incomplete source/dependency evidence invalidates identity. Ambiguous keys, unsupported producers, mutation and external-consumed mismatch refuse. Positive same-input and negative collision/change/unsupported regressions are prerequisites. No public wire identity or loader reinterpretation is authorized. |
| Narrower context | First choose one already identified supported operation family; specify the complete source/configuration/policy/manifest/build/helper/external-input dependency closure before deciding whether any unrelated edit can be ignored. | Missing closure, changed consumed dependencies, stale captures, ambiguity and unsupported coverage remain unknown or input failure. Require positive proven-independent edits and negative relevant/unlisted/external/mutated edits. Retain full head gates, operation coverage, baseline rules and existing compatibility; any semantic change requires separate approval. |

Supplying identity alone cannot discharge changed-context evidence. Narrower
context alone cannot supply the missing identities. These are design inputs, not
new guarantees or a request to weaken thresholds, baselines, exclusions or budgets.

The opt-in collector is `TestLocalComparisonCensus`. Set
`GOQUENT_COMPARISON_HEAD`, `GOQUENT_COMPARISON_BASE` and
`GOQUENT_COMPARISON_DIFF` to the runner's exact private archives and nonempty
patch; set `GOQUENT_COMPARISON_OUTPUT` to a fresh existing mode-0700 directory
outside both roots and the checkout. Use the same toolchain/environment as the
built CLI and capture test output privately. With no output variable it skips;
with explicit inputs it checks actual branch parity and full populations before
writing mode-0600 private census evidence. Files are not reusable CI inputs.
Do not upload the directory. Final full/race opt-in skips remain explicit skips.

## Reproduction and validation

GQ-MAINT-12 PR1 investigates a [bounded documentation comparison
proposal](document-context-proof.md). The delivered decision preserves the finite
design/falsification record with eligible zero; it does not adopt a narrower-context
guarantee. The test-only model does not supply a real loader closure witness. Current production
comparison classification is unchanged; the historical MAINT11 measurements above
remain tied to their original source. Revision 2 synchronizes PR124 through main
`cf1c8651dbdb2ea57b251d02115245f122273614` after exact-main CI/Docs success.
Fresh PR122 measurements are separate from PR124 counts. Its limited cancellation
improvements do not prove physical cleanup or historical causality. The
[Issue123 release blocker](https://github.com/recoweft/goquent/issues/123#issuecomment-6386876158)
remains open, and any fresh native classification/cleanup/watchdog failure is a
current blocker. See the proposal for the preserved revision-1 failure record and
revision-2 evidence boundary. The fresh pre-evidence-paragraph census at
`e8ae0342734cabf24bf83203fa3e6d23dc82cc62` against that main measured
1,495 unknown (1,004 identity / 491 context), 1,182 operations and candidate
210 SELECT / 18 static operations / 14 diagnostics / 0 eligible. These are
source-specific observations, not fixed expectations; final exact-head results
are recorded separately in PR122/Issue121.

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
test. [GQ-MAINT-09](native-performance-boundaries.md) adds finite restricted-role,
PostgreSQL custom operator/cast and controlled loopback fault cases. Broader
privilege, extension and network-fault coverage remains unverified; per-case
fresh execution evidence is required, and MySQL operator/cast is not applicable.
The original manifest generated_code/database skips above are historical;
PR127 supplied both axes for its saved model, as detailed in the current
[disposition](release-blocker-disposition.md).
All fixed staged-runner blockers and the separate human release boundary remain.
This diagnosis adds no public API, JSON/SARIF/wire/hash format, runtime semantics,
loader repair, suppression or baseline update, and does not claim release
readiness or whole-Issue acceptance.
