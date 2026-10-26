# Review warning provenance and comparison unknowns

GQ-MAINT-15 PR2; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
This finite diagnosis updates the complete-report breakdown and reviews every
non-test library/example high-or-above finding. It changes no execution,
comparison, threshold, suppression, baseline, manifest or release contract.

## Inputs and measurement boundary

The measured clean source is main `c9fbf749b1cbc40ac4293e62160e9d3708dac36b`
([PR144](https://github.com/recoweft/goquent/pull/144) merge). Remote main was
resolved independently; that merge is the measured head itself. Its exact
[CI](https://github.com/recoweft/goquent/actions/runs/40344676721),
[Security](https://github.com/recoweft/goquent/actions/runs/40344676385) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40344676102) succeeded.
The independently resolved first-parent base is
`9485f298112219fc472bd16331c04cf29ffa5fd6`. The exact nonempty patch changes
four files (212 insertions, 60 deletions); neither archive contains this document.
Measurement preceded all documentation edits. Final PR-head checks are separate
evidence in the PR/controller ledger, not a relabeling of this measurement.

The worker checked the requested cwd, origin, clean status, Ubuntu 24.04.5/WSL2,
repository instructions/contracts/template, Git connectivity, its own API push
permission, and absence of a local/remote target branch or any-state duplicate
PR before creating the branch. Sandboxed Git DNS and API connection failed;
approved read-only diagnostics succeeded. Branch creation required approval
because sandbox Git metadata was read-only. These results do not imply unrestricted
network access or DB availability.

Historical toolchain correction: the later [test-origin audit](test-warning-intent.md)
found that the retained CLI build and saved version log identify Go 1.25.0,
not the originally reported Go 1.26.10. The census process environment remains
unverified; the later audit obtains a fresh complete source report under explicit
Go 1.26.10. The originally intended environment used `GOTOOLCHAIN=local`,
`GOPROXY=off`, a private build cache and existing pinned dependencies. That
configuration does not independently establish the census process environment.
No configuration, baseline, manifest or
runtime-evidence producer was supplied to review. No live DB connection was
made; server versions, live schema/identity and native behavior are unverified.
The separate manifest check uses supplied saved-model inputs, not a live DB.

The existing [staged runner](../scripts/release_validation/run.py) ran inventory,
ordinary and strict once. The existing
[comparison collector](../orm/review/ci_diagnostic_test.go) ran once against its
exact private archives and patch, retaining complete ordinary/head/base source
reports and checking actual branch parity for every finding. It also checks
ordinary/head finding equality and full operation population/coverage. Public
256-item projections were supplementary output, never the census population.
Raw reports, locations, joins, SQL, captures and logs remain outside the repo and
both roots in owner-only storage (0700 directories/executables, 0600 evidence),
without upload. No fixture or generated diagnostic was added to an analysis root.

## Same-population breakdown

Both complete reports have **1,510 findings, zero suppressed findings and 1,194
operations**. The high-threshold gate has **187 contributing findings**: 153 high,
29 destructive and five blocked. The other 1,323 are medium. All 187 are partial.
These are warning counts, not demonstrated bug counts. Historical main23059408
had 1,504 findings, 1,189 operations and 187 high-or-above; its 1,013/491 unknown
partition is not this run. The earlier 1,510 observation was a total, not a
high-risk count, and was not used as an expected value.

The following table joins code/risk, provenance and exclusive unknown cause on
the same private finding ordinals. `I` means unavailable head comparison identity;
`C` means whole-tree context changed with a known matching diagnostic.
Test provenance takes precedence over directory: example `_test.go` files belong
to test. Library includes non-test `orm`/`cmd` sources; example excludes tests.
No findings fell outside these groups.

| Code | Risk | Library I / C | Test I / C | Example I / C | Total |
| --- | --- | ---: | ---: | ---: | ---: |
| STATIC_REVIEW_PARTIAL | medium | 23 / 19 | 582 / 203 | 11 / 1 | 839 |
| STATIC_REVIEW_UNSUPPORTED | medium | 29 / 0 | 186 / 0 | 0 / 0 | 215 |
| RAW_SQL_USED | high | 9 / 0 | 140 / 0 | 0 / 0 | 149 |
| DESTRUCTIVE_SQL_DETECTED | destructive | 0 / 0 | 28 / 0 | 0 / 0 | 28 |
| MIGRATION_DROP_COLUMN | destructive | 0 / 0 | 0 / 0 | 1 / 0 | 1 |
| DELETE_WITHOUT_WHERE | blocked | 0 / 1 | 0 / 4 | 0 / 0 | 5 |
| WEAK_PREDICATE | high | 0 / 0 | 4 / 0 | 0 / 0 | 4 |
| BULK_UPDATE_DETECTED | medium | 0 / 4 | 0 / 50 | 0 / 0 | 54 |
| BULK_DELETE_DETECTED | medium | 0 / 4 | 0 / 22 | 0 / 0 | 26 |
| LIMIT_MISSING | medium | 0 / 2 | 0 / 110 | 0 / 0 | 112 |
| SELECT_STAR_USED | medium | 0 / 2 | 0 / 74 | 0 / 0 | 76 |
| MIGRATION_ADD_INDEX_NON_CONCURRENT | medium | 0 / 0 | 0 / 0 | 1 / 0 | 1 |
| Total | | 61 / 32 | 940 / 463 | 13 / 1 | 1,510 |

Only STATIC_REVIEW_UNSUPPORTED items are unsupported (215); all other rows are
partial (1,295), with zero precise. Unsupported reason membership partitions
into dynamic_value 207 and opaque_operation eight; type_unavailable is zero.
This uses the current report's reasons, not a new independent type-loading census.
No whole-repository loader-cause investigation was repeated.

| Origin | medium | high | destructive | blocked | High-or-above |
| --- | ---: | ---: | ---: | ---: | ---: |
| Library/CLI | 83 | 9 | 0 | 1 | 10 |
| Test | 1,227 | 144 | 28 | 4 | 176 |
| Example | 13 | 0 | 1 | 0 | 1 |

The 176 test warnings occur in `tests` (106), `orm` (53), `orm/query` (12),
`orm/migration` (two), `orm/review` (one), `tests/contracts` (one), and
`examples/ai-safe-orm/storedmodel` (one). This is provenance accounting only.
Neither test placement nor a raw/unknown classification establishes safety or a
defect. The later [test-origin audit](test-warning-intent.md) individually reviews
a fresh complete test population. Its source, toolchain and execution limits are separate.

### Precision and overlapping evidence

| Axis | Finding known / partial / unknown / unsupported | Operation known / partial / unknown / unsupported |
| --- | ---: | ---: |
| SQL | 14 / 943 / 338 / 215 | 18 / 697 / 264 / 215 |
| Columns | 14 / 106 / 1,390 / 0 | 24 / 51 / 1,119 / 0 |
| Bindings | 14 / 0 / 1,496 / 0 | 18 / 0 / 1,176 / 0 |
| Policy | 0 / 0 / 1,510 / 0 | 0 / 0 / 1,194 / 0 |

Every finding and operation lacks runtime_policy and evidence_missing. Other
missing memberships are database_specific 698/500, static_unknown 106/51 and
dynamic_value 69/39 (finding/operation counts). Reason memberships are
static_unknown 595/477, opaque_operation 354/226, dynamic_value 207/207 and
unverified_input 2/2. These memberships overlap; they are not extra findings.
Operation kinds are select 213, update 54, delete 31 and unavailable 896.
Unavailable kinds are not inferred SQL operations. Finding, operation and
membership counts must never be added together.

### Why every finding is unknown here

Actual comparison states are unknown 1,510; matched/new/worsened/unchanged all
zero. Input correspondence and selection are valid, but sameContext is false.
The collector's ordered primary attribution is identity unavailable **1,014**
plus whole-tree context **496**; all other primary categories are zero.
For this pair every item has exactly one matching diagnostic, so the recorded
Known flag also provides the per-item join to those two parity-checked causes.

Overlapping memberships: whole-tree changed 1,510; same diagnostic 1,510;
head identity unavailable 1,014; operation missing 1,014; prior identity
unavailable 1,014; unsupported 215. No-prior, ambiguous-prior, missing-file and
input-correspondence memberships are zero. Unsupported is contained in the
identity group. High-or-above splits into 182 identity and five context items,
already included in the 187 risk-gate population.

[RunCI](../orm/review/ci.go) requires both usable identities and equal complete
context for unchanged classification. Matching text alone does not supply either.
The nonempty source edit changes whole-tree context even for untouched finding
files. Thus 496 identity-bearing diagnostics remain unknown; the other 1,014
already lack identity under the existing producer contract. The current collector
reports 213 SELECT operations, 18 static candidates, 14 candidate diagnostics and
zero eligible closure witnesses. Neither changing context semantics nor inventing
identities is part of this work. Unknown is independent of severity and does not
itself create 1,510 high-threshold failures.

## All non-test high-or-above findings

The measured set contains ten library findings and one example finding, all
reviewed. The count was obtained from the report, not fixed to the old ten-plus-one
inventory. References below are committed sources at the measured head; detailed
per-finding locations and review notes remain private.

### Nine literal catalog reads

Each entry below has one RAW_SQL_USED/high/partial finding, identity-primary
unknown, SQL partial and columns/bindings/policy unknown. Each has static_unknown
reason and missing runtime_policy/evidence_missing. Recognition is the typed
StatusExecutor.QueryContext raw-call path in [track.go](../orm/review/track.go),
with the unconditional raw structural warning from [risk.go](../orm/query/risk.go).

| Committed call site | Literal query purpose | Input and dispatch boundary |
| --- | --- | --- |
| [readPostgresSchema](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_read.go#L68) | Column catalog | One bound schema argument, default selected before call |
| [readMySQLSchema](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_read.go#L85) | Column catalog | Two bound copies of schema argument; database fallback in fixed SQL |
| [readExtendedCatalog, PostgreSQL](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L27) | Namespace identity/version | One bound schema argument |
| [readExtendedCatalog, MySQL](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L29) | Namespace identity/version | Two bound schema arguments |
| [readPostgresDetails, constraints](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L80) | Foreign keys/checks | One bound observed namespace |
| [readPostgresDetails, indexes](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L141) | Index metadata | One bound observed namespace |
| [readMySQLDetails, foreign keys](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L240) | Foreign keys | One bound observed namespace |
| [readMySQLDetails, checks](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L282) | Checks | One bound observed namespace |
| [readMySQLDetails, indexes](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/migration/schema_catalog.go#L309) | Index metadata | One bound observed namespace |

All nine SQL expressions are fixed literals; caller/returned schema values are
parameters, not concatenated SQL identifiers. Table selection filters scanned
results through tableSet/tableIncluded. ReadSchema selects the supported dialect
and forwards the supplied StatusExecutor to these calls. QueryContext is the
actual dispatch boundary: a supplied database/sql DB/Tx dispatches there, while
a custom executor retains its own behavior. No pure review path invokes it.
Scan/query/row errors propagate; namespace identity requires one returned row.
Catalog visibility remains partial as documented in [schema capabilities](schema-capabilities.md).

Disposition: intentional structural raw warnings, with no demonstrated injection
or contract violation in these nine paths. Fixed SQL and parameter binding do
not prove privileges, visibility, server behavior or arbitrary custom executor
safety. Existing catalog failure controls passed; no live catalog trial was run.

### Generic DELETE adapter

The sole blocked library finding is at
[prepareGeneric's PlanDelete call](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/orm/query/generic.go#L86).
It is partial/context-primary, with SQL/columns/bindings/policy unknown and
missing runtime_policy/database_specific/evidence_missing. The tracker recognizes
the typed DELETE terminal but does not reconstruct this interprocedural request,
base-builder copies or direct builder mutations. Its dynamic-table fallback
retains a blocked missing-predicate warning for a tracked root without a tracked
predicate; this is not an observation of executed unfiltered SQL.

The actual path is [nested planning](../orm/nested_plan.go) to
[compoundPlan.add](../orm/compound_plan.go), [prepareWrite](../orm/write_plan.go),
then prepareGeneric. Destination settings/executor/dialect/context are installed;
Base state is copied through CopyStateToSelect into a fresh builder. Request
WhereColumns/WhereValues are installed with WhereLiteral before the switch.
Current delete callers supply their predicates through Base; generic update
mapping supplies paired request predicates. Compatibility nested scopes are
applied before a fresh delete plan at the original execution position, including
later deletes changed by an earlier scope. Strict refuses non-nil opaque nested
scopes. No claim of whole-program predicate reconstruction is made.

[PlanDelete](../orm/query/write_plan.go) applies policyBuilder, copies the resulting
state with [CopyStateToDelete](../orm/internal/querybuilder/api/state.go), and
builds the SQL/arguments/inspection snapshot before finalization. An actually
predicate-free default-risk plan remains blocked. The returned private Exec
closure enters [executeResult](../orm/query/execution.go) and
[runWrite](../orm/query/write_results.go), checks the sealed gate, binds/consumes
the plan, then dispatches the retained SQL/arguments through ExecContext or Exec.
Public diagnostics are not dispatch inputs. Existing scope, later-delete mutation
and pre-dispatch rejection tests passed for their finite mock/capture cases.

Disposition: current static reconstruction limit; no reproduced predicate-loss
or gate-bypass defect. Predicates alone are not business authorization, and
compatibility callbacks/partial compound writes retain their documented limits.
This finding is neither suppressed nor labeled a safe false positive.

### Destructive example migration

The one example warning is MIGRATION_DROP_COLUMN/destructive/partial at the
committed [drop-column fixture](https://github.com/recoweft/goquent/blob/c9fbf749b1cbc40ac4293e62160e9d3708dac36b/examples/ai-safe-orm/migrations/002_drop_legacy_email.sql).
It is identity-primary unknown, SQL partial, other axes unknown, with
unverified_input and missing runtime_policy/evidence_missing. SQL discovery feeds
the migration parser/risk classifier; dropping a column is genuinely destructive
if executed, not a false positive because it is an example.

The [example checker](../examples/ai-safe-orm/check.sh) and
[example CI](../examples/ai-safe-orm/ci/goquent-review.yml) do not apply it.
[CLI migration](../cmd/goquent/migrate.go) plans first; dry-run checks approval
without connecting; apply additionally requires explicit driver/DSN and opens a
DB. [Migrator.Apply](../orm/migration/plan.go) replans, checks EnsureExecutable,
then sends each statement to Executor.ExecContext. With the CLI's ORM DB that
also enters its Raw gate and private execution dispatcher; other supplied
executors have their own behavior. Migration approval is not schema freshness,
backup, deployment authorization or an automatic transaction guarantee.

Disposition: intentional destructive verification fixture and a real destructive
path only under explicit execution. Built CLI plan at high threshold and
unapproved dry-run both returned 1; approval-before-DSN regressions passed.
No apply or live DB validation was performed.

## Validation and finite stopping conditions

| Check | Result and scope |
| --- | --- |
| Built ordinary CLI, high | Exit 1; full_report=true, input_failed=false, risk_failed=true, precision_failed=false, manifest_failed=false; no comparison selected |
| Built exact strict CLI, high/partial | Exit 1; full_report=true, input_failed=false, risk_failed=true, precision_failed=true, manifest_failed=false; unknown 1,510, other states zero |
| Staged collector | Exit 1, collection complete, release blocked; unselected stages not-run |
| TestLocalComparisonCensus | Exit 0; one pass, zero fail/skip; actual inner CI exit 1; full ordinary/head agreement and branch parity |
| Focused existing controls | Exit 0; five packages, 80 leaf + nine parent passes, zero fail/skip |
| Example migration plan/high and unapproved dry-run | Actual built-CLI exits 1 and 1; no DB execution |
| Original saved-model four-input manifest | Exit 0; schema/policy/generated_code/database each ok; supplied evidence only |
| Public-output checker; strict-input regression checker; staged runner unit suite | Each exit 0 |
| Documentation links and whitespace | Checked separately on the documentation change; final result belongs in PR/controller ledger |
| Final exact PR-head CI/Security | Separate remote evidence required after committing/pushing; never inferred from measured main |

Focused controls are ComparisonCensus context/identity/precedence, compound
known refusals, strict scope effects, compatibility scope timing/later-delete
mutation, static nested transaction ownership, scoped delete plan/dispatch,
generic literal-PK metadata, missing-predicate risk, catalog failure propagation,
and migration approval boundaries. No new test mirrors implementation. The local
full/race/both-DB/native suites, native census and type-loader census were not run;
the final hosted CI is separate. API documentation regeneration is unnecessary
for these handwritten docs; Docs workflow runs only on main.

| Conclusion category | Finite result / remaining decision |
| --- | --- |
| Implementation defect in this review | None demonstrated in the eleven non-test high-or-above paths; no whole-product zero-defect claim |
| Analyzer defect | None demonstrated within the current supported contract; counts alone are not repair evidence |
| Current contract limits | Missing identity, whole-tree context, dynamic/opaque reconstruction and runtime policy remain visible; no semantics change adopted |
| Intended verification | Nine literal catalog raw warnings and one destructive migration warning retain their existing risk; generic predicate/gate controls retain their finite behavior |
| Insufficient information | Live DB/executor behavior, complete test-warning intent, general runtime safety and historical/native causal unknowns remain unverified |
| Non-test high-or-above review | All 11 measured items reviewed; no unreviewed item in this population |
| Next repair candidates | **0**: no supported-contract violation with an expected/actual mismatch was established; do not invent a repair to reduce counts |
| Test-origin follow-up | The [PR3 audit](test-warning-intent.md) records the separately measured population and individual source coverage; it does not infer safety from test placement |

All seven [fixed blockers](release-blocker-disposition.md), native unknowns,
legacy stale-manifest behavior, and the limited applicability of existing
four-axis manifest evidence remain. No manifest was regenerated or released.
Human release judgment, whole-Issue acceptance and any new comparison contract
remain separate. This work ends at a draft PR and controller report, without
ready/merge/close or another investigation.
