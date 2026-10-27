# Bounded SELECT comparison identity

GQ-MAINT-15 PR4; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
**Outcome: an identity example exists; narrower-context comparison still needs
additional premises.** This is a design and source-counterexample record. It adds
no production identity, comparison rule, wire field or authorization. PR5 may
investigate a complete dependency boundary for this family only after controller
review; PR6 implementation requires that boundary to succeed. Neither work is
started here.

## Input and evidence boundary

Implementation inspected: main `1386acc670f20e47c6b0b4759232c585eedeea2d`, the
[PR146](https://github.com/recoweft/goquent/pull/146) merge. Independently resolved
remote main equals that commit; ancestry therefore holds. Exact-main
[CI](https://github.com/recoweft/goquent/actions/runs/40462309645),
[Security](https://github.com/recoweft/goquent/actions/runs/40462309690) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40462309802) succeeded before
editing. Cwd/origin, clean checkout, Ubuntu 24.04.5/WSL2, applicable repository
instructions/contracts/template, worker API identity and push permission, and
local/remote branch and all-state PR duplication were checked. Sandboxed GitHub
DNS/API access and Git metadata writes failed; approved reads/fetch/branch
creation succeeded. These are separate from test results and DB capabilities.

Local validation uses the actual cached Go **1.26.10 linux/amd64 executable**,
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOWORK=off`, `GOFLAGS=-mod=readonly`, a separate
writable cache and private temporary directory. The selected environment is
Linux/amd64 with cgo enabled, no explicit build tags and the native package
driver (no custom driver requested). The default sandbox cache was
read-only; the private-cache dependency probe succeeded without network access.
An earlier launcher probe with checksum verification disabled failed before
loading; it is not a dependency or source failure. Modules remain the existing
`go.mod`/`go.sum` pins (including x/tools v0.43.0); no dependency update occurs.
The fixture module copies those files and replaces Goquent with this checkout.
No config, manifest, baseline, Evidence callback, explicit dialect/settings or
live DB input is supplied. Each generated fixture package is independently
compiled with `go test -run '^$' .`; its SELECT function is not executed.

The new [source counterexamples](../orm/review/select_identity_test.go) reuse
`ciFixture`, `fixtureModule`, exact-patch generation and the existing `observedCI`
collector hook. They assert physical positions from fixture source, static axes,
operation retention and explicit case outcomes, not a second implementation of
RunCI's switch. Full source reports/captures and raw logs are not published.
Final commit/check identities belong in the PR and controller report; this
paragraph does not claim that checks on an earlier tree validate a later commit.

Historical 1,014 identity / 496 context unknowns belong to their old comparison
pair. The [PR3 correction](test-warning-intent.md) leaves that census environment
unverified; its fresh Go 1.26.10 source census (1,510 findings / 1,194 operations)
is not a fresh comparison. None of these numbers is an expected fixture or
current-head population. No whole-repository census is repeated here.

## What already identifies an operation

There are different mechanisms; none may be substituted for another:

| Mechanism | Current producer and limit |
| --- | --- |
| Typed operation | `goTracker.terminal` in [track.go](../orm/review/track.go) resolves typed methods, calls `sampleSelect`, adds the operation and assigns private `operationForTerminal` kind. A nonempty kind can exist even when reconstruction is partial. Fallback raw/opaque/loader operations have no equivalent kind. |
| Source reference | `Session.add` in [precision.go](../orm/review/precision.go) records module-relative file, physical chain-start byte offset and `inputVersion`. The diagnostic location instead points at the terminal method. Version hashes sorted captured absolute paths and file digests, not a portable commit or complete dependency closure. Separate roots can have different versions for identical content. |
| Session authority | References carry private session/index while Evidence runs synchronously. Caller ID is only a bounded declaration; duplicate IDs/receipts are ambiguous. `finish` closes the token, invalidates the session and clears expected receipts/reference session pointers before retaining detached private operations. Serialized references cannot recreate authority. |
| Comparison candidate key | `ciFinding.key` in [ci.go](../orm/review/ci.go) is root-relative file + physical line + byte column + diagnostic code. It is a finding key, not an operation ID; operation kind is checked separately. `collectCI` uses private operations at the same location, not public fields, diagnostic prose, risk or SQL spelling. |
| Private report and context | `Run` retains canonical findings and `privateOperations`. `RunCI` owns both full reports, tree snapshots and captures; it does not accept a decoded report as comparison authority. `sameDiagnostic` additionally checks kind, severity, precision, suppression, normalized detail and complete containing-file bytes. `sameContext` still requires whole-tree equality and valid input correspondence. |

`collectCI.known` requires a valid canonical root-relative source position,
containing bytes, nonempty private operation kind and non-unsupported finding.
It does **not** require SQL/columns/bindings all known, uniqueness of matching
operations or explicit build-variant metadata. It takes the first private
operation at a matching location. This existing behavior is not silently
upgraded into the stronger design obligation below.

`types.go` loads tests and prefers the variant with more syntax (normally the
internal test variant), retaining one `typedFile` per physical path. Package/type
objects are local to the load. The retained operation does not carry an explicit
selected-variant descriptor or complete consumed-input witness. This is a gap
for future reuse across changed contexts, not a missing SELECT kind to fix now.

## One bounded family and its proposed use contract

The initial family is a straight-line, typed `db.Table` chain ending in `Get`,
`GetMaps` or `Plan`, within `sampleSelect`'s current reconstruction domain. Require
SQL, columns and bindings known on both sides; partial overall precision due to
unknown policy/runtime correspondence is allowed. The measured member uses
literal Table/Select/Where/Get. Other supported modifiers include finite literal
null predicates, Limit/Offset and OrderBy, but this PR does not independently
prove every combination. Model/scopes, alias/control-flow uncertainty, dynamic
arguments, opaque modifiers, raw SQL, arbitrary callbacks and incomplete loads
are outside the initial family. A typed SELECT kind alone is insufficient.

An exploratory named-constant predicate fixture compiled but failed the proposed
known-axis premise: `simpleModifier`'s syntactic filter makes it partial before
`sampleSelect` can use its constant evaluator. It was narrowed to a literal
predicate, rather than loosening production analysis or claiming all compile-time
constants are supported. The build-selection control also retains an additional
unsupported operation for discovered but excluded source; SELECT-only accounting
must not discard that operation from the full gate.

For later comparison reuse, the proposed identity is a **private relation between
two captured operations**, not a new digest/API. Its obligations are:

1. Map canonical root-relative files bijectively; require identical containing
   bytes, physical terminal line/byte column, chain-start offset and private
   operation kind. Verify offset/location consistency against that same capture.
   Use exact paths, never basenames, `//line` directives or public locations.
2. Require exactly one operation per location in each selected variant and one
   prior finding per diagnostic key across primary and suppressed lists. Different
   codes at one operation are separate diagnostics, not extra operations. Equal
   SQL at two locations remains two operations. Reject duplicates and conflicting
   kinds; never choose a first match for a proposed reuse witness.
3. Bind selected package/test variant, selected file set, build environment and
   consumed dependency context to the captures which produced the operations.
   These are separate context obligations, not inferred from the location key or
   SourceReference.Version. PR5 must define their complete owner/inventory,
   including negative dependencies, before any narrower-context positive exists.
4. Limit validity to one synchronous comparison over those immutable captures.
   Revalidate inputs before classification; missing source/capture, mutation,
   cancellation, exhaustion or incomplete loading cannot produce a reuse witness.
   No durable identifier or source-edit tracking is promised. Inserted lines,
   offset/column shifts, code moves/renames, edits elsewhere in the containing
   file and replaced operations require fresh analysis; no fuzzy mapping applies.

This relation may hold for identical input under the current whole-tree contract.
It is not implemented as a new predicate. For changed trees it remains conditional
on PR5's missing input owner. Identity and context stay independent: a terminal
can retain its location/kind yet lose comparison context after a dependency edit.

Collision handling does not authorize new suppression, deduplication or baseline
semantics. With `ShowSuppressed`, the existing overlapping lists can contain the
same suppressed item twice; prior cardinality then prevents unchanged/matched.
A future witness must reject both ambiguous prior and ambiguous head operation
mapping. Missing identity/unsupported findings stay unknown unless an existing
higher-precedence branch applies. Input/diff/capture/external failures remain
input failures (exit 2), not ordinary unknown success. Current new/worsened
precedence is preserved. No warning-free or globally unknown operation may be
removed from full risk, precision, manifest or coverage gates.

Runtime receipt correspondence remains independent and unknown without evidence.
Even an application test receipt is bounded planner correspondence, not proof
that source executed, runtime permission, physical database identity or policy
knowledge. No public wire, private planner seal, SQL meaning or guarantee changes.

## Executable positive and negative cases

The committed fixture contains a typed function accepting `*orm.DB`, declares a
map-slice destination, and uses the literal Table/Select/Where/Get chain above.
It extends the existing buildable `ciFixture`; there is no DSN or execution.
A same-package `value.go` is parsed during loading. Changing its constant is a
captured source dependency counterexample even though the literal SELECT does
not read that constant; this is **not** a proof that the edit is irrelevant.

| Actual source case | Explicit expected observation in current RunCI |
| --- | --- |
| Identical trees, distinct roots | One SELECT; known SQL/columns/bindings; `SELECT_STAR_USED` unchanged. Policy/runtime remain unproven; strict medium/partial gate exit 1. |
| Add a listed text document | Same SELECT identity and known axes, but diagnostic unknown because whole-tree context differs; valid input, exit 1. No document exemption. |
| Change the parsed same-package constant | Unchanged terminal bytes/coordinates; diagnostic unknown, valid input, exit 1. No dependency exemption. |
| Exclude value.go with a build tag and add its equivalent alternate | Same terminal and constant value, different selected source set; diagnostic unknown, full unsupported coverage retained, exit 1. Does not test every GOOS/tag/toolchain change. |
| Insert a line before the function | Physical line and source offset change; diagnostic new, not unchanged. No persistent correspondence. |
| Add another equal SELECT at another location | Two SELECT operations and distinct finding keys; both star diagnostics new under containing-source change; no equal-SQL conflation. |

All six source cases passed in the local literal-fixture run (test command exit
0; each inner RunCI exit 1 as expected). These are observed outcomes, not results
of an unimplemented narrower-context rule. The initial constant-fixture run failed
its known-axis assertion and is retained as exploratory falsification evidence.

Existing controls additionally cover duplicate suppressed/prior keys, missing
operation identity, unsupported loading, Unicode/physical locations, changed
source/new keys, worsening precedence, capture mutation and consumed external
mismatch. The [MAINT12 finite model](../orm/review/document_context_proof_test.go)
remains a synthetic closure model only. Its build/dependency refusals do not prove
a real Go closure. No new production-equivalent classifier model is added.

## Handoff and retained limits

Stage-one identity has a concrete positive. The obstruction to a changed-context
positive is **additional premises needed**, not absence of every SELECT identity.
PR5 must enumerate and bind source/helpers, imports/export data, selected test
variants, configuration/policy/manifest, toolchain/workspace/build/driver inputs
and external consumption; demonstrate invalidation for relevant edits; and prove
one actual edit unconsumed by every relevant consumer. Observed capture absence
is not negative-dependency evidence. Current snapshots do not enumerate every
external Go-process input. The MAINT03 unused-native-test-executable exception
retains its separate graph/role and same-session byte checks and is not extended.

MAINT12's old eligible-zero result is not remeasured or repaired here. If PR5
cannot close that universe within existing resource bounds, it must record the
missing evidence and stop; PR6 must not implement a shortcut. This document does
not adopt a broader operational guarantee requiring a specification decision.

The [CI contract](static-review-ci.md), [triage](review-triage.md),
[document proposal](document-context-proof.md),
[warning provenance](review-warning-provenance.md),
[test audit](test-warning-intent.md) and
[release disposition](release-blocker-disposition.md) remain authoritative at
their stated boundaries. Seven blockers, finite manifest evidence, historical
native/unknown residuals and human release judgement remain unchanged. Local
full/race/DB/native suites, full census and manifest regeneration are not repeated
for this design task. Exact final-head CI/Security results are recorded separately;
passing them is not release acceptance or permission to start the next work.

Local focused validation selected the new source cases, existing comparison
census context/identity/precedence controls, physical Unicode positions,
structure/worsening, capture/external-input controls and the MAINT12 finite
model. `go test ./orm/review -run
'^(TestBoundedSelectIdentitySourceCounterexamples|TestComparisonCensus.*|TestCIUnicodePhysicalLocation|TestCIWorsenedAndStructureComparison|TestCICaptureRetainsExactBytes|TestCIChangedSourceNewExternalContextRemainsUnknown|TestDocumentContextFiniteModel)$'
-count=1 -json` exited 0 with 48 test pass events, zero failures/skips.
`go mod verify`, local documentation-link checks and `git diff --check` also
passed. The broader suites are delegated to the unchanged repository workflows,
not represented as local execution.

## SELECT closure investigation follow-up

[GQ-MAINT-15 PR5](select-context-closure.md) inventories current input owners,
negative dependencies and external-process gaps, with finite source controls.
It establishes no narrower-context witness or production comparison change.
