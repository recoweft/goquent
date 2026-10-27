# Bounded documentation comparison proposal

GQ-MAINT-12 PR1, [Issue #121](https://github.com/recoweft/goquent/issues/121).
Status: **design/falsification record accepted; narrower-context guarantee not
adopted**. The delivered specification decision selects the design-only record
with zero real-run eligibility. This PR contains documentation and test-only
falsification. Production comparison still requires whole-tree context equality.
No unknown finding is promoted to unchanged or matched. A later implementation
needs a separate PR/work item and a new specification consultation.

## Candidate and present obstruction

The single candidate family is the existing typed static generated SELECT:
`track.go` assigns the private SELECT operation after typed terminal tracking;
`sampleSelect` reconstructs the bounded Get/GetMaps/Plan domain. The census further
requires SQL, columns and bindings to be known. It uses private operation fields,
physical location, exact containing-source bytes and a unique prior diagnostic.
Policy and runtime correspondence can remain unknown; this is a comparison
proposal, not a new claim about runtime safety. Literal raw SQL and missing
operation identities are outside this proposal. Neither the historical 998 nor
491 is an expected eligible population.

The existing data does **not** close the input dependency universe. `Run` discovers
Go/SQL/JSON files; `newSession` captures these plus explicit configuration,
manifest and discovered module files. `loadReviewTypesCaptured` additionally
records parsed source and selects build/test variants through `go/packages`.
This does not record every toolchain, export-data, workspace, environment or
driver input consumed by the external Go process. Absence from capture is not
proof of non-consumption. Native driver detection alone supplies no such proof.
Consequently no real-run closure witness can currently be constructed by this
PR: **eligible is zero under the proposed rule**. Fresh candidate counts are
separate measurement evidence, not permission to ignore context.

## Proposed finite comparison obligation

One synchronous RunCI invocation would own both snapshots and a private,
non-serializable witness. The witness must be produced by a supported bounded
input owner, not accepted from configuration, public output or a caller's claim.
The present implementation has no such owner. A future implementation must
demonstrate the following obligations before enabling any positive real case.

| Obligation | Required evidence and refusal |
| --- | --- |
| Identity | Existing private typed operation kind, physical terminal location/offset, full containing-source bytes, selected package/test variant and known static reconstruction. Require one prior diagnostic and one corresponding operation at that location. No reconstruction from reason text, SQL spelling or public projection. Reject dynamic/unsupported producers and ambiguous keys. |
| Discovery | Enumerate both complete bounded trees and both discovery results, including exclusions and errors. Use the current exact diff reconstruction for every changed byte. Reject unlisted changes, unsafe paths, links, special files, rename/add/delete and unsupported diff forms in the initial subset. |
| Source and helpers | Bind all selected syntax, imports/export inputs, helper declarations, constants, overlays, embedded inputs and generated inputs. Unchanged terminal text alone is insufficient. A helper or consumed document change refuses the narrower proof. |
| Build universe | Bind selected ordinary/internal/external test variants, dependency graph, go.mod/go.sum, active go.work/go.work.sum, replacements, build flags/tags, GOOS/GOARCH/cgo settings, environment affecting loading, toolchain/compiler and driver executable bytes/selection. Either enumerate every consumed external input with independently complete provenance, or refuse. Disable unsupported/dynamic/custom-driver cases for this proposed subset. |
| Review context | Bind actual options and configuration, rules, suppressions, policy, manifest and all SQL/JSON inputs. Account for in-memory settings and manifest freshness inputs as well as files. Initially refuse Evidence callbacks, caller-supplied CurrentManifest/ManifestInputs and other contexts without an immutable complete owner. File extensions do not identify consumption: a config stored as .md is still consumed. |
| Negative dependencies | For each changed existing document, establish that no discovery, configuration, helper, build, driver or other consumer can read it in either run. Require a closed inventory and a justified role for every input, not merely a list of observed reads. Reject absent/unknown roles. A document label is necessary for the proposed narrow purpose but never sufficient evidence. |
| External and mutation | Preserve current external equality, captureValid and Session.unchanged checks. Any consumed external mismatch or same-session mutation refuses. The MAINT03 unused-native-test-executable exception keeps its existing independent graph/role and same-session byte obligations; do not generalize it to this proposal. |

Path correspondence is an exact normalized root-relative bijection for internal
files; external paths retain exact identity and bytes. No basename matching or
automatic rename mapping is allowed. Observe inventory and options before
loading, actual consumption during loading/review, and revalidate source,
external inputs, environment and discovery after both runs and immediately
before classification. An earlier successful read is not a freshness guarantee.
An unobservable race, changed option, stale witness, cancellation, incomplete
load or missing consumer invalidates the witness for the entire invocation.
No witness survives return, can be replayed, or is shared between runs.

Failure to establish narrower context retains the existing unknown result;
existing correspondence/diff/capture failures retain input failure. Never turn
an input failure into ordinary unknown. Existing new/worsened precedence remains
separate. Even if a future witness supports unchanged display, full head risk,
precision and manifest gates and **all operations** remain intact. This proposal
does not broaden baseline matching: the current exact whole-tree baseline rule,
expiry/owner/annotation behavior and diagnostic retention remain in force.

Retain existing bounds: 4,096 files, 2 MiB/file, 64 MiB input/tree, 256 loader
directories, 60-second loading deadline, 4,096 receipts; diff 1 MiB/4,096 records/
10,000 hunks. Closure work must fit those bounds, with cancellation; exhaustion
refuses rather than truncates or raises budgets. No threshold, suppression,
exclusion, baseline or loader-budget relaxation enables this proposal.

Private source, paths, settings, captures and witness material stay local outside
both analyzed roots and the checkout, in owner-only directories (0700), files
(0600) and executables (0700), without upload. Only fixed aggregate counts and
classification results belong in public review material. This proposal adds no
public API, wire format, version, hash domain or reusable certificate.

## Falsification and measurement

`TestDocumentContextFiniteModel` models an explicitly closed finite input
universe. It permits a listed unconsumed document edit, including a non-Markdown
path, and rejects consumed-document, source, helper, .md configuration/manifest,
build/dependency, ambiguous, unsupported, dynamic, incomplete-closure,
custom-driver, environment, capture-mutation, external-consumed, unclassified and
unlisted counterexamples. Its `closed` premise is supplied by the synthetic
model, **not measured from Go**. These tests check the proposed logical rule;
they neither build a real closure certificate nor establish Go loader coverage.
The synthetic positive must not be described as a production eligible finding.

The existing opt-in `TestLocalComparisonCensus` collects complete private RunCI
reports, typed operation fields, source trees/captures, exact diff and actual
branch outcomes, and checks ordinary/full-head parity. Its additional
`document-candidates.json` records SELECT operation count, static candidate
operation count, candidate diagnostic count, closure availability and eligible
count. Candidate diagnostics require known identity and unique same-diagnostic
prior source correspondence. Candidate operation counts and finding counts are
different units. Closure availability remains false; eligible remains zero.
Do not publish raw collector artifacts or import them as future CI inputs.

Fresh complete-private census on 2026-10-24 compared base
`672c3eb7d501b486a08c83fc6e6dbd2b916136ac` with design/prototype commit
`9e08aac` using Go 1.26.4, a clean exact nonempty diff and ordinary-report parity.
It measured 210 SELECT operations, 18 static candidate operations and 14
candidate diagnostic items. Closure availability was false and eligible was
**0**. Actual production outcomes were 1,489 unknown, zero unchanged/matched;
exclusive attribution was 998 missing head identities and 491 whole-tree
context changes. These freshly observed counts happen to agree with MAINT11;
they are not assertions or default eligibility. Input correspondence passed;
risk and precision failed, with actual RunCI exit 1. The synthetic model's 18
cases passed. Later final-head validation belongs in the PR evidence.

The first sandbox census completed but type loading lacked writable build-cache
access and produced no SELECT identities. A bounded `go list` diagnostic showed
a read-only build-cache error; approved execution of that diagnostic succeeded.
The fresh census above then ran through the approved execution path. The first
attempt is environment-limited evidence, not a measured absence of SELECTs and
not a reason to weaken loading or eligibility. Raw captures from both attempts
remain protected locally; they are not uploaded.

## Delivered decision and future work

The delivered decision selects retaining the present whole-tree contract in
production and preserving this finite design/falsification record with zero
current eligibility. It does not adopt an operational narrower-context guarantee.
The alternative of deferring the model as well was not selected. Both alternatives
leave current classifications and compatibility unchanged. Future work must
identify a bounded immutable input owner, its captured and excluded inputs and
negative dependencies, implementation feasibility, resource budget, invalidation
and refusal conditions, and positive/negative evidence before another decision.
That owner/loader implementation is not an authorized part of this PR.

## Historical native failure retained (revision 1, 2026-10-24)

The initial fresh full suite failed one leaf,
`TestPerformancePlanNativeBoundaries/mysql/network_cancel` (three failure events
including its two parents). The subsequent race suite passed that leaf. Both
suites skipped the two unselected local census collectors. This does not repair
or waive the full-suite failure.

A protected, uncommitted test-overlay diagnostic repeated each dialect's cancel
case 20 times without race and 10 times with race. Normal MySQL had 16 passes and
4 failures; normal PostgreSQL had 20 passes. Race had 10 passes per dialect.
The diagnostic preserves the original expected error, two-second context and
transport teardown. It records only fixed error classes, context-state booleans
and phase ordering; no payloads or arbitrary driver errors are published.

In the four reproduced failures, the proxy was reached, explicit cancel changed
the context to canceled, then transport teardown began, and result publication
reported `PERFORMANCE_CLEANUP` instead of expected `PERFORMANCE_CANCELED` before
teardown returned. The context was canceled, not deadline-exceeded, with result
publication less than one millisecond after fault setup. Those observations do
not support a two-second deadline explanation for those four trials. They do
not establish the origin of the earlier uninstrumented failure or isolate the
specific cleanup path. Failed assertions precede explicit server-session cleanup
observation, so that observation is unverified for failed trials; deferred test
cleanup remains in place.

At the revision-1 review, root cause and fixture stability remained unresolved,
and PR122 was held draft pending a separate repair. That historical decision and
the first full-suite failure are retained; passing repetitions did not establish
a repair. PR122 itself changed no runtime semantics, driver, timeout, expected
error or skip rule. The current synchronization below supersedes the old
repair-pending status, not the historical evidence or its causal uncertainty.

The original example still has generated_code/database verification skips;
the manifest companion's four-axis success is a separate finite evidence path.
Native coverage has finite residuals. Historical testmain/expiry causes remain
unproven by later passing tests. The staged runner's fixed BLOCKERS and the
separate human release boundary remain. No unknown reduction, whole-Issue
acceptance, merge or release readiness is claimed.


## Revision 2: synchronization after PR124

PR122 now includes main `cf1c8651dbdb2ea57b251d02115245f122273614` by a normal
merge. That main includes [PR124](https://github.com/recoweft/goquent/pull/124)
at `a92abbcf49be4e691e5de24ae21a77c7c0dc7cbb`. Its exact-main
[CI](https://github.com/recoweft/goquent/actions/runs/40114301787) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40114301785) both succeeded
before synchronization. The merge had no conflicts. The PR's diff against main
still consists only of two design documents and two test files.

PR124 was integrated under explicitly limited acceptance of finite improvements,
not historical causal repair. Its strict rollback sentinel and owned logical-close
classification, complete cancellation-frame forwarding, real-pq mechanism tests
and failed-assertion cleanup remain intact. These do not guarantee physical
cleanup, server abort acknowledgement or every cancellation ordering. The original
MAINT12 failure's cause remains unproven; the historical PG failure's exact tree
and cause also remain unknown. The independent
[unresolved release-blocking track in Issue123](https://github.com/recoweft/goquent/issues/123#issuecomment-6386876158)
remains open. PR124 integration is not unconditional permission to integrate PR122.

The 18 synthetic cases and candidate design are unchanged. Complete real Go
consumed-universe and negative-closure evidence remain unavailable; no producer
or production unknown promotion is added. Fresh final-head census, ordinary
and strict gates, local full/race/native results and CI are recorded with exact
source identities in PR122 and Issue121. Historical counts above, and PR124's
separate 1,495-unknown measurement, are not substituted for this PR's census.
A fresh classification, cleanup or watchdog failure is a current review blocker;
it must not be absorbed into historical residuals or erased by rerunning to pass.
The worker keeps PR122 draft; the controller decides its finite integration
readiness from current evidence and retained residuals.

### Revision-2 measured census (before this evidence paragraph)

A fresh complete-private census compared clean archives of base
`cf1c8651dbdb2ea57b251d02115245f122273614` and synchronized source
`e8ae0342734cabf24bf83203fa3e6d23dc82cc62` with Go 1.26.4, a writable cache
and a nonempty exact diff. Collector exit was 0; actual RunCI exit was 1, with
valid input and failed risk/precision gates. Actual branch parity, complete
ordinary/head parity and population/operation checks passed. Head, base and
ordinary reports each retained 1,182 operations and 1,495 primary findings
(no suppressed findings). All 1,495 comparison items were unknown: 1,004 lacked
head identity and 491 had whole-tree context changes. Other classes and primary
causes were zero. The 210 unsupported items are an overlapping subset, not an
additional population.

The fresh candidate measurement was 210 SELECT operations, 18 static candidate
operations and 14 candidate diagnostic items, with closure unavailable and
eligible **0**. Equality with any earlier number is not reused evidence or proof
of improvement. This paragraph changes the measured source; the final exact-head
census and strict comparison are therefore rerun after committing it and are
reported against their own SHA in PR122/Issue121. Full/race/native validation
also uses that final clean head, without edits during collection. The historical
revision-1 native failure and independent Issue123 release blocker remain.

## Bounded SELECT identity follow-up

[GQ-MAINT-15 PR4](select-comparison-identity.md) separates existing typed SELECT
identity from dependency closure and records buildable source counterexamples.
It changes no production comparison rule and does not establish a narrower
context witness or promote unknown findings.

## SELECT closure investigation follow-up

[GQ-MAINT-15 PR5](select-context-closure.md) inventories current input owners,
negative dependencies and external-process gaps, with finite source controls.
It establishes no narrower-context witness or production comparison change.
