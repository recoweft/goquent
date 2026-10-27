# SARIF, source changes and bounded review baselines

GQ-AI-09 PR3 (3/3), [Issue #82](https://github.com/recoweft/goquent/issues/82).
PR83 and PR84 are merged prerequisites. This adds display comparison to the
common review report; it does not add execution, authorization, arbitrary
interprocedural analysis, or complete SQL reconstruction. The Issue's whole
acceptance criteria require separate review and are not accepted by this document.

## Formats and compatibility

`goquent review --format sarif ./...` explicitly selects SARIF 2.1.0.
`--format review-ci-json` selects `goquent.review_ci_view`, integer version 1,
containing a selected public v2 report, full finding counts, full gate flags, and
comparison classifications. Existing pretty, GitHub, JSON v1 default, explicit
JSON v2, source wire formats and strict v1/v2 readers remain separate. MCP does
not gain a comparison or execution endpoint. `review.RunCI` performs the same
analysis and selection as the CLI; its opaque result provides `PublicView`,
`WriteJSON`, `WriteSARIF`, `WriteDisplay` and `ExitCode`. `DecodeCIView` is an
independent strict display reader, never an analysis or gate input.

SARIF and JSON project the same generated diagnostics, not a second analysis.
Rule IDs are the existing closed diagnostic codes; messages and suppression
reasons are fixed public classifications. The tool identifies its adapter as
`review-sarif-v1`, not an invented library release. Results include risk,
precision and suppression properties; run properties retain operation coverage,
full counts, truncation and omitted-detail flags. Each finding list and the
operation list is independently bounded to 256 items, as in public v2. The
original complete private report determines the gate, even when the displayed
last item or entire selected list omits a blocking finding.

**AC1 location interpretation:** public SARIF has no physicalLocation, artifact
URI, source snippet, repository path, path hash or recoverable file identity.
Properties `line` and `column` correspond to the existing public JSON projection
of private canonical diagnostics. Go locations identify the terminal method;
physical lines ignore `//line`, and columns are 1-based UTF-8 **byte** offsets,
not SARIF Unicode character columns. A Unicode fixture checks this distinction.
Zero means unspecified. SQL/source JSON file diagnostics use the reviewed file's
first line, not externally supplied warning locations. File identities are
explicitly omitted. These outputs do not establish GitHub file annotations or
uniquely identify two files sharing a line number. No private SARIF artifact
route or relaxation of local-output/CI restrictions is introduced. The original
AC text remains unchanged; this documented limitation is not whole-AC acceptance.
`review.WriteSARIF` also refuses external/mutated source location claims: only
Run's private canonical snapshot retains locations. No opaque value methods are
called while projecting results.

## Bounded source comparison

Prepare old and new trees yourself; the command does not resolve Git commits,
read the Git index, start Git, evaluate diff text, invoke arbitrary commands, or
fetch remote data. Normal Go type loading keeps the existing PR1/2 contract.
For reproducible CI, materialize the two trees from independently resolved
immutable commits outside the review command and record those commits privately.
Keep patches, baseline files and generated outputs outside both reviewed roots.

```sh
goquent review --format review-ci-json \
  --diff /private/changes.patch --diff-base-root /private/base \
  --fail-on high --fail-on-precision partial /private/head
goquent review --format sarif \
  --diff /private/changes.patch --diff-base-root /private/base /private/head
```

Both trees are analyzed with full package/helper context. Root snapshots and
loader-consumed bytes are compared before/after analysis; external loader inputs
must agree by exact path and bytes across the two sessions, with one finite
exception for unused native Go test executables described below. Other generated
loader inputs at different temporary/cache paths can still leave comparison unknown
even when both analyses succeed. This checks source correspondence, not
package authenticity or source execution. Concurrent mutation is unsupported;
observed changes invalidate correspondence. Explicit runtime-evidence producers,
current-manifest inputs and unmappable configuration disable comparison trust,
without weakening the original head gate. Source JSON cannot supply comparison
evidence. Directory reads reject symlinks/special files and use `os.OpenRoot`
for confinement; existing module loading retains its own documented boundary.

GQ-MAINT-03 PR1 (1/1), [Issue #97](https://github.com/recoweft/goquent/issues/97),
separates an unused synthetic test executable from **cross-session external**
comparison only. Native `go/packages` loading still uses `Tests: true`. With
Go 1.26.4 and x/tools v0.43.0, a second metadata-only dependency load must agree
with the syntax-load roots, their file sets and import identities. The executable
must have a complete, error-free single-file main syntax/type result, a related
original package and test variant, native test-harness imports, and no incoming
dependency edge. File roles are combined across roots and transitive dependencies:
any ordinary/shared role vetoes the exception. Explicitly reviewed/overlaid files,
selected syntax/type information, helper ownership and transitive type imports
also veto it. Names, suffixes and generated comments alone never establish this
role. Custom or automatically discovered external package drivers do not receive
the exception; unknown or inconsistent metadata retains exact comparison.

All captured executable bytes remain subject to the existing same-session
mutation, parse, timeout and size checks. No source bytes are normalized and no
public exclusion marker is accepted. Independently proven unused executables may
have different counts, paths or registration bytes between base and head. This
does not certify their contents or execution. Tests, test variants, helpers,
ordinary generated files and consumed dependency/standard-library sources retain
their full source checks. For example, adding the first test can also add ordinary
external library inputs; if their consumed sets differ, comparison still fails
closed. This change does not make all valid source changes comparable, change
public/source wire versions, or improve precision. Whole-tree context changes
can still leave individual findings unknown even when input correspondence holds.

The finite diff subset is UTF-8 Git-style unified text with `a/` and `b/`
repository-relative exact paths, bounded to **1 MiB, 4,096 file records and
10,000 hunks**. Text changes require LF-terminated nonempty source files. Quoted
or whitespace-containing header paths, glob/escape paths, binary patches,
combined diffs, malformed/context/out-of-range hunks and no-newline markers are
not silently approximated. Context and hunk counts are checked against both
actual trees, including unchanged text outside hunks. Pure rename metadata
requires identical bytes; rename never moves a baseline automatically. Deletion
is recorded without inventing a head location. Binary changes are classified
and make correspondence unknown. Missing/unlisted changes, including new files,
remain visible and fail input correspondence; head discovery still includes
supported unlisted sources. Git tracking is always `not_assessed`, not a claim
of tracked/untracked completeness.

Whole source snapshots include context files, bounded to 4,096 files and 4,096 directories,
2 MiB/file and 64 MiB/tree. Standard generated/dependency/cache directories
(`.git`, `.codex`, `.agents`, `.aws`, `.gocache`, `vendor`, `node_modules`,
`dist`, `build`, `coverage`) are excluded from the comparison snapshot. Sources
outside its correspondence domain cannot establish a baseline match. Existing
Go loader limits remain 256 directories and 60 seconds per load, and existing
operation budgets/unknown diagnostics remain in force. Two analyses and repeated
bounded snapshots cost more than a single review; no performance guarantee or
repository-wide safety claim is made. Cancellation fails correspondence.

Selection retains changed operation spans, new/worsened diagnostics outside the
patch, and unknown/global findings. Operation coverage is retained in full in
the private report and bounded only for public display. Canonical finding
positions do not move to the start of a multiline expression: its private span
is separate. With conservative whole-tree context comparison, even an unrelated
context edit can leave unchanged findings classified unknown and displayed.

## Baseline source format

`--baseline /private/baseline.json` requires both diff options. Its dedicated
strict JSON source envelope is `kind: goquent.review_baseline`, `version: 1`.
Maximum size is **256 KiB / 64 entries**. Every entry field below is required,
including `column`; duplicate keys/targets, unknown fields/version/code, null,
globs, path escapes and absent scope/expiry are invalid.

```json
{
  "kind": "goquent.review_baseline",
  "version": 1,
  "entries": [{
    "file": "queries/read.go", "line": 12, "column": 24,
    "operation": "select", "code": "SELECT_STAR_USED",
    "risk": "medium", "precision": "partial",
    "sql": "known", "columns": "known", "bindings": "known", "policy": "unknown",
    "owner": "database-team", "reason": "Temporary projection cleanup tracked in review",
    "created_at": "2026-10-15T00:00:00Z", "expires_at": "2026-10-16T00:00:00Z"
  }]
}
```

```sh
goquent review --format review-ci-json \
  --diff /private/changes.patch --diff-base-root /private/base \
  --baseline /private/baseline.json --fail-on-precision partial /private/head
```

This illustrative entry is not a current valid baseline and is not generated or
installed by default. Use actual source diagnostics and a current UTC expiry.
Dates require UTC RFC3339 `Z`, `created_at <= now < expires_at`, and at most
30 days between creation and expiry. Expiry is exclusive, including the exact
boundary. API tests inject `CIOptions.Now`; the CLI always uses current UTC and
has no backdating flag. Owner is nonempty (at most 256 bytes); the reason reuses
the existing substantive suppression-reason validator (at most 1,024 bytes).
Only exact SELECT targets with `SELECT_STAR_USED`, `LIMIT_MISSING`, or
`WEAK_PREDICATE` are supported, each checked against existing suppressibility.
RAW SQL, writes, missing-WHERE/destructive codes, manifest, loader/global unknown
and missing-evidence codes are outside this closed allowlist. This list does
not authorize arbitrary new suppressible codes.

A match requires independently analyzed base and head findings, exact location,
operation/code/risk/precision/four-axis coverage, actual source bytes and matching
whole-tree and external-loader context. No caller hash, summary, `precise` claim,
masked output or decoded report suffices. Matching SELECT warnings can still
have partial coverage or missing runtime evidence: the annotation does not cover
that evidence deficit, remove the coverage data, or make it precise. Ambiguous,
unsupported, changed/stale or unavailable correspondence is unknown. Risk
increases and precision/axis degradation at the same canonical target are
worsened; known different positions/bytes are new. Unknown is not a synonym for
confirmed new. No fuzzy relocation, rename absorption or helper-change matching
is performed. The conservative comparison can reject a baseline after an
unrelated file edit. Review the current source rather than broadening scope.

Baselines annotate; **they never suppress a gate or authorize Strict execution**.
Matched findings remain displayed and retained. Original suppression semantics
are separate. Private baseline source retains owner/reason/expiry; public output
contains only fixed `review_annotation_only` and bounded classifications/counts.
It contains no owner, exact expiry, file/scope identifier or comparison digest.
No receipt/validated handle, DB write, runtime authorization or threshold change
is introduced. Existing full-repository failures are not made successful by
adding a baseline.

## Counts, failures and schema

The following version-1 fields have distinct units:

| Field | Unit and rule |
| --- | --- |
| `baseline_entries.entries` | Parsed entry count; zero on document-level rejection |
| `matched/expired/invalid/unused/unknown` under `baseline_entries` | Mutually exclusive per-entry outcomes; document rejection uses `invalid=1` with `entries=0` |
| `head_findings` | One classification per complete head finding-list item: matched, new, worsened, unknown or unchanged; zero when no comparison was requested |
| `full_finding_count`, `full_suppressed_finding_count` | Original complete lists, not selected counts |
| `selected` counts | Selected lists, including matched annotations; public truncation does not change counts |
| diff modified/added/deleted/renamed/binary | File record counts (binary is unknown, not modified) |
| diff unlisted | Actual changed paths omitted from the patch, not Git tracking status |
| diff invalid/unknown/read_failed | Input/correspondence problem counters; may overlap and are not file totals |

`--show-suppressed` preserves the old overlapping-list behavior: a suppressed
finding may occur in both primary and suppressed lists, and each list item is
counted. Such ambiguous duplicates cannot establish a baseline match.
Malformed documents reject the baseline as a whole. Semantic invalid/expired
entries are classified individually; duplicate targets are all invalid.
Complete-context missing targets are unused; incomplete context is unknown.
Any invalid, expired, unused or unknown baseline entry sets `input_failed`.
No entries are silently discarded to claim success.

Exit precedence is input/correspondence failure **2**, required-manifest failure
**3**, full risk/precision failure **1**, otherwise **0**. Gate flags remain
independent; an input error does not erase an existing risk failure. A head tree
that cannot be safely read yields `read_failed`, unknown, exit 2 and
`gate.full_report=false`; it is not a zero-finding successful analysis. Diff or
base failures after head analysis retain the complete head gate. Legacy display
formats do not carry comparison counters; use the new JSON/SARIF formats and
always check the exit status. `gate.full_report` means the original head report
was produced, not that all operations were supported or safe.

Tests exercise the real API/CLI entry points, Unicode physical positions,
mutable/untrusted locations, privacy, list truncation/full gates, diff content,
rename/delete/binary, multiline operations, unlisted/global unknown, expiry
boundaries, duplicate/unknown/missing/oversized baseline inputs and stale helper
context. The SARIF output is additionally validated against OASIS SARIF 2.1.0
errata01's official draft-04 JSON Schema using `scripts/check-review-sarif.py`.
That validation tool requires Python `jsonschema`; it is a development check,
not a new Go/runtime dependency. Pass a locally fetched official schema and an
actual CLI output file; validation prints only a fixed result, never raw errors.

## Bounded SELECT identity follow-up

[GQ-MAINT-15 PR4](select-comparison-identity.md) separates existing typed SELECT
identity from dependency closure and records buildable source counterexamples.
It changes no production comparison rule and does not establish a narrower
context witness or promote unknown findings.
