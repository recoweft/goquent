# Static precision and planner samples

GQ-AI-09 PR2 (2/3), [Issue #82](https://github.com/recoweft/goquent/issues/82).
This extends the typed review introduced in PR83. It does not implement SARIF,
changed-line review or expiring baselines (PR3), and does not change database
execution, Strict, tenant authorization or validated execution handles.

## Four independent axes

`review.ReviewReport.Operations` retains tracked operations even when there is no
risk warning. Each operation has a version-1 `analysis.Detail` with `sql`,
`columns`, `bindings` and `policy`. Each axis is `known`, `partial`, `unknown` or
`unsupported`. The policy axis describes declaration inspection only. Runtime
policy, physical schema, live database state, other tenants and atomicity are
unverified. Supplied manifest freshness never fills missing axes.

The initial exact static domain is generated SELECT with a literal table,
selection, simple Where/OrWhere or null predicates, limit/offset and ordering.
Native string, boolean and signed integer constants are retained by type. Unknown
values, unsupported types, branch/loop uncertainty and opaque calls remain
unknown/partial. Existing write, generic, validated API, loader, escape and helper
findings are preserved; their unavailable axes are not guessed. Unsupported
operations remain in the report. This is deliberately narrower than the planner.

Missing/legacy details normalize to all unknown. Unknown enum/reason values
normalize to fixed unknown classifications; unsupported detail versions fail
source decoding. Reasons and missing evidence use a closed vocabulary. They
never carry loader errors, arbitrary exception text or application value methods.

Overall precision is a conservative adapter: every axis must be known to be
`precise`; an unsupported axis is `unsupported`; all other cases are `partial`.
All unknown and partially known operations remain distinguishable in the axes.
Existing unsupported findings cannot be upgraded. This changes precision **gate
semantics**, not wire defaults: previously shape-precise database findings can
now be partial because policy or bindings are unknown. A test sample never fills
those static unknowns. Existing codes, locations and configured suppression
expiry are retained. Operations whose associated findings are all explicitly
suppressed follow those suppressions; otherwise operation coverage participates
in the precision gate, including operations without risk warnings. Suppression
is not authorization. No threshold or suppression is introduced to make CI green.

## Ephemeral application-test evidence

`review.Options.Evidence` is an explicit, synchronous application-test callback.
It receives a `review.Session`; `References()` returns sensitive source references
and `Token()` returns its opaque token. For each selected reference, the test
builds its real query using the common planner, calls
`q.ReviewReceipt(ctx, session.Token())`, and calls `session.Record(ref, receipt)`.
The test may attach an optional ID to the reference; IDs alone never match.
`EvidenceDialect` and `EvidenceSettings` declare the current comparison context.
No supplied JSON or manifest can construct a receipt.

The reference binds module-relative file, byte offset, the actual input-set
version and session identity. The session captures selected inputs and module
files and the Go sources actually parsed by bounded type loading. Selected
source is supplied through the loader overlay. Content is checked again after
loading and after the producer returns. Changing source, dependencies, module
inputs, reference fields, IDs or session makes correspondence unavailable or
mismatched. A previous session's receipt cannot be reused. Concurrent mutation of
queries/files during review is unsupported; producers must return synchronously
and cooperate with cancellation. The library does not launch application tests.

Receipts are issued from the final private common SELECT planner snapshot, not
from a public `QueryPlan`. They own canonical bytes, not caller maps/slices or
callbacks. Current owner state, including its executor/context references, is checked and
replanned before correspondence is
accepted. Generating and comparing receipts execute no SQL. Serialization,
decoding and public formatting cannot reveal or restore their private material.
Neither receipts nor review tokens are accepted by execution APIs.

The initial receipt family is generated SELECT. Opaque/raw expressions, custom
dialects and values outside the existing canonical typed domain are unavailable.
This does not weaken the GQ-AI-05 identity domain: missing schema/tenant identity
keeps context unconfirmed, even when SQL and arguments match. The original
private owner/seal/binding/current lifecycle remains unchanged.

Correspondence has independent `operation`, `target`, `structure`, `arguments`,
`source` and `context` states: `matched`, `unconfirmed`, `mismatched`. Comparison
preserves dialect, exact generated SQL, condition-tree structure, argument order,
native type, NULL and presence. It does not implement arbitrary SQL equivalence.
Application-test policy or tenant settings keep static SQL partial and bindings
unknown, even when all six sample comparisons match. They are declaration
inputs, not values learned from the static source.
An unsupported static reconstruction retains unconfirmed components even if the
test supplies a real receipt. A known different operation is still a mismatch.
Context comparisons describe declared snapshots, never authenticated physical
DB identity or live policy checks. Matching samples are **not** evidence that the
static source actually executed or that the sample covers every runtime value.
The source association is the application test's declaration.

Duplicate registrations/IDs, absent/zero receipts, foreign sessions, expired
sessions, changed owner inputs and changed context cannot become positive proof.
Budget failure retains a fixed missing-evidence reason and coverage diagnostic;
it never deletes a discovered finding. Type loading keeps its 4,096-file,
2 MiB/file, 64 MiB parse-input, 256-directory and 60-second limits. Initial
session discovery is independently bounded to 4,096 inputs/64 MiB; the combined
initial and parsed-source snapshots are bounded by their two budgets. Sessions
retain at most 4,096 operation receipts, tokens permit at most 8,192 captures and
64 MiB of total canonical receipt material, and expire after 60 seconds. The
canonical encoder retains its 64-level/65,536-slot/8 MiB per-value limits.
Application producers are cooperative code, not a sandbox or forced timeout.

## Display and migration

Source JSON APIs remain sensitive. They can include file/version/position data;
source report JSON decoding restores no correspondence evidence. Private gate
operation snapshots are separate from mutable source/public operation views.
Do not use public views as authorization, evidence, or CI decision inputs.

### Serialized source reports and file discovery

`ReviewReport.UnmarshalJSON` preserves source diagnostic fields, including codes,
risk, locations and suppression flags. Those fields are input data, not trusted
analysis evidence. Successful decoding (including missing/0/1 envelope versions)
creates separate private finding and operation snapshots for precision decisions.
Missing detail and self-reported known axes become unknown with `unverified_input`;
unsupported findings and axes remain unsupported. Both finding lists project the
same conservative precision in public v1 and v2. Findings-only reports do not
invent operation records: v2 retains their partial/unsupported finding precision,
while per-operation axes are available only for supplied operation records.
Subsequent edits to decoded source views cannot replace these private snapshots.
In-process analysis retains its existing semantics. Source operations continue to
lose correspondence on decoding, and their private coverage is not suppressed by
supplied operation flags. Finding suppression flags retain their diagnostic meaning.
Neither form is authorization. Risk fields are not recomputed by this decode API.

This intentionally changes precision gates for legacy findings-only reports:
self-reported `precise` without verifiable analysis now meets the `partial`
threshold. Source JSON remains readable and retains its diagnostic data; display
and gate classification no longer repeat its precision claims. Decode failure is
atomic, including method-level version/type errors; the previous receiver remains
unchanged. Always decode external input into a fresh receiver and discard it on
error. No invalidation of copies/aliases is promised.

Standalone CLI and `review.Run` file discovery have a different contract: they
**do not import source reports**. A top-level `findings`, `suppressed_findings` or
`operations` list/null, or a truncated value after such a key, identifies a possible
source report and produces one fixed `STATIC_REVIEW_UNSUPPORTED` diagnostic at
line 1 of the input file. Keys are recognized case-insensitively, like the source
JSON decoder. This also covers legacy findings-only inputs, unknown/duplicate
versions, duplicate/mixed envelopes and damage after the identifying structure.
It deliberately changes previously ignored report files from zero findings to an
unsupported finding. Ordinary empty objects, nested keys and string contents do
not establish a report. Independently valid strict public v1/v2 report views are
separate display artifacts, not source reports. The detector examines at most
2 MiB, 65,536 tokens and 64 nested containers; it does not claim to classify every
arbitrary or damaged JSON file. Existing discovery/session budgets still apply.

The CLI marker imports no supplied diagnostics, locations, policy, provenance,
precision, correspondence or suppression. JSON strings are never inline
suppression directives. Only the existing application configuration rules and
expiring suppressions can configure it. Existing QueryPlan version errors remain
errors even in a mixed report/plan envelope. Independently recognized QueryPlan
or MigrationPlan risks in a mixed envelope are retained alongside the marker. `--fail-on-precision partial` and
`unsupported` fail on the unsuppressed marker. Its medium risk does not change a
risk-only `--fail-on high` exit status. Public output omits the input path and all
supplied arbitrary content. No producer, trusted artifact input, receipt recovery,
execution capability or PR3 import/baseline facility is introduced.

Existing `PublicView()`, v1 strict readers/writers and CLI default output remain
v1. Opt in with `report.PublicViewV2()`, `DecodeReportViewV2`, or:

```sh
goquent review --format json-v2 --fail-on-precision partial ./...
```

MCP `review_query` accepts `format: "json-v2"` for the same closed coverage
semantics; omitted format preserves its legacy display shape. V2 adds bounded
operation coverage, fixed reasons and comparisons, but no file names, IDs,
digests, SQL, columns, values, arbitrary errors or private provenance. Both
finding lists and operation lists are capped at 256, with full counts and
truncation retained. V1 rejects v2; v2 rejects v1 and ambiguous/unknown envelope
versions. Use the existing v1 reader explicitly for old display artifacts.
Unknown enum values are normalized, never treated as evidence.

The normal standalone CLI and MCP do **not** accept trusted positive evidence
files, run a producer or regenerate declarations. External QueryPlan JSON keeps
its independent unverified marker and cannot become precise through matching
hashes/IDs, `verified`, `precise`, or supplied freshness. API tests and the actual
shared CLI runner exercise the positive in-process path. The CLI test injection
is package-private; it does not create a second verification implementation or
an unredacted output path.

No new production dependencies are added. `query` does not import review, the
root facade or CLI; shared classifications live in `orm/analysis`. Internal
builder types and execution identity are not exposed by the new APIs.
