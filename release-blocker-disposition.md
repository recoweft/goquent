# Remaining release blockers: evidence and decisions

GQ-MAINT-15, revision 1; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
This is a diagnosis with an **adopted finite review scope**, not release
acceptance. The delivered answer to consultation
`ae2c4b96-b04b-4ab0-9bd8-c719e251dec8` selected limited option A for this work
and revision. It does not accept current gate failures or certify the original
universal acceptance criteria. All seven [fixed runner blockers](../scripts/release_validation/run.py)
remain unchanged. Neither integration nor passing tests clears them.

## Measurement identity and limits

For the later GQ-MAINT-15 PR2 measurement at PR144 merge, see
[review warning provenance](review-warning-provenance.md): 1,510 complete
findings, 187 high-or-above, and unknown attribution 1,014 identity / 496 context.
It reviews all eleven non-test high-or-above findings. The measurements below
remain historical main23059408 evidence; PR2 does not replace or clear the seven
fixed blockers or extend saved-model/native guarantees.

The measured current source is main
`23059408f16d463bbe9594b5d29040a3b7be5fc3`, the merge of
[PR127](https://github.com/recoweft/goquent/pull/127). Its exact
[test run](https://github.com/recoweft/goquent/actions/runs/40187408773) and
[Docs run](https://github.com/recoweft/goquent/actions/runs/40187408774) were both
successful before this work began changing files. The independently resolved
comparison base is `03a23376baba678f54ff3da2323825bd16687a2f`.
The checkout was clean throughout measurement; the diff was nonempty.
The measurements below precede this document and are not final PR-head results.

Local Go was explicitly 1.26.4, with cached dependencies, GOPROXY=off and
GOTOOLCHAIN=local. No baseline, configuration, manifest or runtime-evidence
producer was supplied to review. Manifest verification was a separate stage.
The existing runner collected ordinary/strict/manifest once; the existing
type-loading and comparison census tests each ran once. Complete reports, exact
archives, patch, loader captures, command exits and private triage stay outside
the checkout and both analysis roots in owner-only storage, without upload.

| Check on measured main | Actual result |
| --- | --- |
| Ordinary built CLI, high threshold | Exit 1; complete report; input valid, risk failed; precision threshold not selected; no comparison |
| Exact clean base-to-main strict CLI, high/partial thresholds | Exit 1; complete report; input valid, risk and precision failed; unknown 1,504, all other comparison states 0 |
| Existing private comparison census | Exit 0, one test pass, no skip; actual ordered branch parity and complete populations checked; its inner CI gate exit remains 1 |
| Existing type-loading census | Exit 0, one test pass, no skip; independent loader state and tracker replay agree |
| Original saved MySQL manifest with all four explicit inputs | Exit 0; schema/policy/generated_code/database each `ok` |
| Focused existing historical/diagnostic controls | Exit 0; 48 leaf + 6 parent passes, zero failures/skips, two packages |
| Collector | Complete, exit 1, release blocked; unselected stages remain not-run |

The focused selection includes native-test-executable roles/driver boundaries,
capture mutation/external mismatch, comparison context/identity/precedence,
dynamic raw positive/negative controls and final binding dispatch checks. These
are source/mock/loader tests, not production native DB trials. No full/race/native
exploration batch was run. Final documentation checks and exact PR-head CI belong
in the PR ledger, with their own immutable head.

## Complete private review populations

Sources: [review and gates](../orm/review/review.go),
[precision reconstruction](../orm/review/precision.go),
[actual comparison census](../orm/review/ci_diagnostic_test.go),
[type census](../orm/review/type_diagnostic_test.go), and
[current precision contract](static-review-precision.md).
The ordinary source report and strict head report each contain 1,504 findings,
zero suppressed findings and 1,189 operations. Counts are observations, never
expected values copied from PR127 or previous censuses. Finding, operation,
reason-membership and comparison populations must not be added together.

| Finding code | Count | Risk |
| --- | ---: | --- |
| STATIC_REVIEW_PARTIAL | 834 | medium |
| STATIC_REVIEW_UNSUPPORTED | 215 | medium |
| RAW_SQL_USED | 149 | high |
| DESTRUCTIVE_SQL_DETECTED | 28 | destructive |
| MIGRATION_DROP_COLUMN | 1 | destructive |
| DELETE_WITHOUT_WHERE | 5 | blocked |
| WEAK_PREDICATE | 4 | high |
| BULK_UPDATE_DETECTED | 53 | medium |
| BULK_DELETE_DETECTED | 26 | medium |
| LIMIT_MISSING | 112 | medium |
| SELECT_STAR_USED | 76 | medium |
| MIGRATION_ADD_INDEX_NON_CONCURRENT | 1 | medium |

There are **187 high-or-above warnings**, not 187 demonstrated production bugs:
153 high, 29 destructive and 5 blocked; 1,317 are medium. Provenance is 176 in
Go test sources, nine literal catalog-read warnings in
[schema catalog](../orm/migration/schema_catalog.go) and
[schema read](../orm/migration/schema_read.go), one generic delete warning in
[prepareGeneric](../orm/query/generic.go), and one example destructive migration.
Test provenance alone is not an intent or safety verdict. Literal catalog reads
still meet the existing raw-risk rule. The generic adapter receives predicates
through request/base state and writes the builder before PlanDelete; its warning
does not establish an unfiltered runtime dispatch. Interprocedural request-state
reconstruction is not a currently promised exact static domain. No warning is
suppressed or relabeled safe here.

Finding precision is partial 1,289, unsupported 215, precise 0.

| Operation axis | known | partial | unknown | unsupported |
| --- | ---: | ---: | ---: | ---: |
| SQL | 18 | 695 | 261 | 215 |
| Columns | 24 | 50 | 1,115 | 0 |
| Bindings | 18 | 0 | 1,171 | 0 |
| Policy | 0 | 0 | 1,189 | 0 |

Private operation-kind metadata records select 210, update 53, delete 31 and
unavailable kind 895. Unavailable kind is not an inferred SQL operation.
Every operation has missing runtime_policy and evidence_missing. Other missing
memberships are database_specific 496, static_unknown 50 and dynamic_value 39.
Operation reason memberships are static_unknown 476, opaque_operation 225,
dynamic_value 207 and unverified_input 2. These overlap.

The 215 unsupported findings partition into dynamic_value 207 and
opaque_operation 8. All 207 dynamic items were replayed as typed raw calls with
complete selected type information and unsupported nonliteral SQL reconstruction;
one is an unqualified recognized raw constructor. Type-unavailable findings and
operations are both zero. This census reports zero selected incomplete states,
package errors, missing type information, budget hits, cancellations, state
mismatches and unmatched replays for this attribution; 22 locations have multiple
loaded variants and 1,880 parse calls were observed. This does not certify every
possible loader input or environment. The eight opaque items are dynamic raw
query fragments/projection or the existing native-test raw expression, outside
the exact static reconstruction domain. MAINT08's diagnostic repair is already
present; repeating the old type-unavailable investigation is not a next repair.

| Exclusive actual-unknown attribution | Count | Disposition |
| --- | ---: | --- |
| Head comparison identity unavailable | 1,013 | Current producer/identity contract limit; not proof of a loader defect |
| Whole-tree context changed, known matching diagnostic | 491 | Expected under current exact-context contract; narrower context requires a design decision |
| Other ordered primary causes | 0 | No new attribution is inferred |

Overlaps: whole-tree changed 1,504; same diagnostic 1,497; missing head operation
identity 1,013; unavailable prior identity 1,006; unsupported precision 215;
no prior finding 7. In particular, seven absent prior findings remain unknown
without identity; absence is not permission to call them new. The existing
document-context collector finds 210 SELECT operations, 18 static candidates,
14 candidate diagnostics and **0 eligible**, with no closure producer.
See [comparison contract](static-review-ci.md) and
[unadopted closure proof](document-context-proof.md). Identity alone cannot repair
changed context; narrower context alone cannot supply identity.

### Repair triage and next work

**Zero contract-preserving production defects have been demonstrated by this
bounded investigation.** This is not a zero-defect assertion about the product.
The 187 warnings, 215 unsupported findings and 1,504 unknowns are not counts of
repairable bugs or safe false positives. Test files remain in discovery; their
provenance does not grant an exclusion. No production repair candidate is
manufactured from these counts.
Private triage retains the gate-bearing and opaque findings and their source
locations; minimal mechanism/control references are retained with expected and
actual classifications. The following are decision/investigation units, not
authorized fixes or promises that their implementation will make review green:

| Unit | Minimal existing reproducer / expectation and actual result | Impact and next scope |
| --- | --- | --- |
| Dynamic raw | `TestDynamicRawReasonAndFallback` and `TestDiagnosticDynamicRawCounterexample`: typed dynamic operand remains unsupported/dynamic_value; literal control retains raw warning; unrelated receiver remains excluded. Focused run passes | No repair indicated. Do not add a new dynamic SQL interpreter merely to reduce 207 findings |
| Missing identity | `TestComparisonCensusUnknownIdentityAndAmbiguity`: unsupported/missing operation identity and ambiguous keys remain unknown. Focused run passes | A separate specification may choose one supported diagnostic producer and define independently bound operation/location/build/dependency identity, invalidation and collision negatives; no inferred identity or public wire addition in this PR |
| Changed context | `TestComparisonCensusContextControls`: same context permits unchanged; listed context change stays unknown with head gates retained; unlisted change fails input. Focused run passes | A separate design-only PR may seek one finite SELECT dependency-closure witness. Stop if closure cannot be supplied; current eligible=0 remains. No promotion from matching text or docs-only filenames |

Recommendation: finish this disposition first. If comparison utility is the next
priority, commission one **design-only producer/closure feasibility PR**, bounded
to the existing static SELECT candidate family and the above refusal controls.
Its deliverable would be a concrete witness/invalidation design or a documented
infeasibility result, not a lower unknown count. This is an optional future
work candidate, not authorization to begin another PR or implement a producer. Production implementation needs
its own adopted contract and work item. Actual reproducible defects found later
receive a separate minimal repair PR; no broad repair is bundled here.

## Seven-blocker disposition register

Every row retains the original obligation, current finite evidence, missing
evidence, responsible decision maker and an executable ending/reopen condition.
The adopted historical exploration rule does not resolve the original causes.
Release decisions and future management designs remain pending; none of these
conditions is authority to remove a fixed ID. Multiple states coexist within an
ID: supplied manifest evidence and unguaranteed broader DB properties are not
collapsed into a single pass.

All fresh rows refer to the measured main and inputs above; older native and
fixture sensitivity results name their separate sources below. Those trials
were not rerun here. Changes to a relied-on implementation, fixture, dependency,
input owner or declared usage invalidate reuse for that changed scope and require
targeted revalidation. A fresh classification, cleanup or watchdog failure is a
current blocker and must not be transferred automatically into historical debt.
The runner still unconditionally aggregates all seven as release-blocking.

| Fixed ID | Original obligation and current classification | Evidence / unmet reason | Next action, owner and finite ending condition |
| --- | --- | --- | --- |
| review-risk-precision | Resolve high/partial gate failures; current gate failure plus contract limits, not a demonstrated 187-bug backlog | Measured main ordinary/strict both exit 1; complete populations above. Missing runtime policy prevents precise standalone review. Full/race cannot discharge this | Maintainer repairs only a reproduced violation in the supported domain. Specification owner must decide a finite release use/safety boundary for remaining expected findings; release reviewer accepts or rejects that exact inventory. The finite review scope is adopted, but current failed gates are not accepted; no threshold/baseline/suppression change |
| review-unknown | Explain unknown and preserve incomplete comparison; current identity/context contract limitations | Census: 1,013 identity + 491 context; input valid, eligible=0. No new production defect established | Specification owner decides whether to retain conservative comparison or commission the bounded design above. Finish diagnosis when all actual items have parity-checked attribution; finish any future feature only with real witness and refusal controls. Human acceptance is separate; unknown never becomes green by this ledger |
| manifest-coverage | Address missing original saved-model code/DB coverage; finite evidence now supplied, broader guarantees absent | Fresh original four-axis pass; PR127 original supplied and owned-native matrices remain separate historical evidence. Handwritten model, live identity/freshness and partial coverage are not certified | Maintainer preserves owner/input and stale/skip regressions; release reviewer explicitly accepts saved-model-only scope and named non-guarantees. No further attempt to fill already supplied axes. New model/live-identity scope needs separate design; blocker stays fixed |
| native-performance-limits | Establish finite native classification/resource evidence and address residual release decision | PR124/125 observation-conditioned contract, cases and counterexamples; original 17/18 and MAINT12 causal attribution unknown; all-order and invisible-return guarantees unavailable | Apply items (a)–(d) below. Maintainer handles any fresh fixed-expectation/cleanup/watchdog failure as a current defect. Human release reviewer alone may accept finite observations and risks; universal ACK/close/reuse is not an invented test AC |
| historical-testmain-cause | Attribute historical intermediate unknown; historical information insufficient, current finite repair independently supported | MAINT03 source/negative coverage below; current input correspondence succeeds. Original trial raw/loader/environment correspondence not recovered | Adopted exploration boundary: retain original attribution unavailable and finite repair evidence permanently; stop repeated exploration without a new distinguishing artifact from existing raw evidence or a captured new failure. Maintainer reopens on either trigger and cannot claim the original cause solved |
| historical-expiry-cause | Attribute original expiry failure; historical information insufficient, deterministic fixture repair independently supported | MAINT05 repaired two fixture assumptions; original error/counters absent; current final-dispatch controls pass | Adopted exploration boundary: preserve original cause undetermined and separate finite fixture repair evidence; stop repeated exploration until a new distinguishing artifact or captured new failure. Maintainer investigates that trigger; any fresh final-gate failure remains a current blocker, not an automatic historical residual |
| human-release-review | Separate authorized acceptance of release candidate scope and deployment assumptions | No release candidate has been accepted by this work. Matrix below gives finite functions and explicit exclusions | Named human release authority records candidate head, accepted use domain, outstanding risks, evidence and decision. Maintainer may prepare a separate blocker-management change only after adoption. No automatic acceptance, ready, merge, close, tag or publication |

## Manifest evidence now supplied

[PR127's immutable head](https://github.com/recoweft/goquent/pull/127)
`eb8c5427cabc5ec7398aa670e4db08980c99dc6b` deliberately added code and declared-DB
expectations to the **original** saved MySQL manifest; it did not merely pass a
companion. Its schema/policy content and fingerprints were preserved. Actual
generated bytes/path, fixed generator input/registry and independent current DB
declaration now participate. The fresh main check above confirms four unique
`ok` results with real exit 0.

The [owner table](../examples/ai-safe-orm/storedmodel/README.md) distinguishes:

- Original MySQL saved string-tenant/timestamp model: supplied four-axis evidence.
- Separate PostgreSQL manifest: separate dialect result.
- Native owned derivatives: expectations instantiated before connection, current
  and drift reads, assertion-failure cleanup/absence/control observations.
- Handwritten main.go: integer tenant/datetime/explicit nullable knowledge,
  offline planning/refusal; not generated or DB-verified by the saved artifact.
- MAINT10 companion: separate generated repository and expectations; its counts
  cannot be credited to the original artifact.

PR127 reports 48 expected regressions each for original supplied and owned-native
matrices. Per dialect, verification itself is 1 pass / 21 expected fail / 2 skip;
regression success does not turn those negatives/skips into positive verification.
The companion has 24 cases per matrix, separately. No native matrix was rerun
for this diagnosis. Main includes that immutable source; source/input equality
supports reference to the old finite evidence, not a new host/DB execution claim.
Live physical identity/freshness, complete catalog visibility, all schemas/types,
handwritten-model compatibility and authorization remain unguaranteed. Original
optional-axis omission semantics and the stale legacy fixture are unchanged.

## Historical attribution versus current contract

### Test executable correspondence

The historical [query-observation evidence](query-observation-validation.md)
records a generated testmain path/hash correspondence problem and unknowns.
Its PR95 source identity is `6cfa3f24702d5eba05e87bc0737159b0c94dcf99`, merged
as `c316ba5654fce8f3656e93fc22dd0022901b7ebb`; the document's earlier-trial
summary is not a retained raw failing loader capture. Historical counts from
different trials must not be combined or assigned this diagnosis's head.

MAINT03 implementation `6edaf0e4ac58fa044e2850ec30323f2e41052633`,
[Issue97](https://github.com/recoweft/goquent/issues/97), provides the finite
[unused native test-executable exception](static-review-ci.md).
[Role tests](../orm/review/type_testmain_test.go) reject ordinary/shared/consumed,
incoming dependency, explicit source/overlay, incomplete metadata/type/syntax,
custom-driver and invalid-role cases; captured bytes still obey same-session
mutation checks. [External/capture controls](../orm/review/ci_changed_source_test.go)
retain external mismatch and exact-byte boundaries. Focused controls passed on
measured main; the fresh strict input gate also passed.
These facts do not identify the original trial's causal branch. The original
raw/host state has not been recovered, so same-trial attribution is unavailable.
No currently reproduced correspondence defect was found. No count30/full search
is justified without a new distinguishing observation.

### Final binding expiry

[MAINT05](binding-expiry-determinism.md), implementation
`7e4e22330a24600373fa6b79867dba21b517f5a6`, records the PR95 race result:
expiry-last leaf and parent failed, without a race detector warning. The returned
error and six executor counters were not retained, so even the failed assertion
operand cannot be recovered. The recorded candidate identity above is not proof
of the exact original failing process/environment. Later non-reproduction is
preserved as non-reproduction, not repair attribution.

Two fixture weaknesses were established: the third context callback did not
select the final binding boundary, and monotonic sleeping did not prove the
wall-clock expiry predicate. MAINT05 replaced the fixture with direct private
prepare/final-bind/dispatch controls and fixed past/future deadlines, without
production clock/expiry changes. [Current tests](../orm/query/binding_test.go)
cover both dialects, zero dispatch on expiry/cancel, positive dispatch and
handle/record consumption. Historical mutation controls separately detected
removal of each final check. Fresh focused controls pass; equality remains
expired, with no grace period. This supports the current finite contract, not
the original host cause. Original attribution remains unavailable; new failures
must be minimized and captured rather than dismissed as historical.

## Native separate-track acceptance (a)–(d)

The authoritative [Issue123 residual](https://github.com/recoweft/goquent/issues/123#issuecomment-6386876158)
lists four obligations, mapped here as (a)–(d). PR124 and
[PR125](https://github.com/recoweft/goquent/pull/125) adopted finite integration
scope, not release approval. PR125 immutable head is
`ca9e9c1a128a081165d5b5ef2cd4869c0cfe2743`; its dedicated native normal/race each
report 18 leaf + 7 parent passes, zero fail/skip. Those historical observations
are not fresh executions in this docs work.

| Item | Established finite evidence | Still unproved / decision |
| --- | --- | --- |
| (a) Map contexts, observations and cleanup | [Contract map](native-cancellation-contract.md) and pinned production/Go/driver sources map caller/frozen/budget/Tx contexts; query, rows, explicit/automatic rollback, owned close, publication, proxy/pool/server/object observations | Physical completion and automatic rollback/close returns can be invisible; mapping does not expose them |
| (b) Distinguish cancel and independent faults with fixed results and sensitivity/cleanup controls | 13 rollback and 17 owned-close unit cases; synthetic visible/hidden rows-close pair with mutation and forced-failure controls; separate production-native cancel=CANCELED, deadline=TIMEOUT, disconnect=CLEANUP trials and cleanup observations | Not all native orders or hidden independent faults. Known CLEANUP keeps precedence. Synthetic/raw-driver controls never become production-native evidence |
| (c) Preserve concrete native check and identify unavailable order point | Concrete-driver check retained; two explicit Tx.done schedules show rollback ErrBadConn visible versus automatic result hidden behind ErrTxDone. Full cancellation-frame forwarding does not select the CAS winner | No public synchronization point for that winner. All-order classification, physical Close, rollback/server ACK and same-connection reuse remain unguaranteed; a new lifecycle design would be separate work |
| (d) Retain historical unknown and obtain human release decision | Original PG 17 pass / 1 fail and later 30/30 non-reproduction remain separate; failing whole-tree and causal branch unavailable; original MAINT12 attribution remains unconfirmed. Finite risks/observations are reviewable | **Human release acceptance pending.** No further reconstruction batch without distinguishing evidence. Only a separate explicit release review can accept this residual |

The [finite native matrix](native-performance-boundaries.md) also covers the
specified restricted roles, PostgreSQL custom operator/cast refusals and held
post-Begin loopback responses. It does not cover every permission arrangement,
TLS/remote network, protocol corruption, query phase or server implementation.
MySQL custom operator/cast N/A is not a passing trial. No single native expectation,
watchdog, cleanup precedence or concrete-driver restriction is weakened.

## One-page human decision matrix

The [safe workflow](safe-workflows-migration.md) and
[contracts v3](../specs/goquent_ai_contracts_v3.md) define the functions below.
The consultation adopts the explicit boundaries below as a finite **scope for
future human review**; it does not certify a release. The controller had not read
the uncommitted full table or rerun the private census when answering. The final
commit and PR still require review for consistency with those boundaries; this
is not blanket adoption of additional guarantees, exceptions or assumptions.

| Candidate function | Required use assumptions | Explicit non-guarantees / decision to record |
| --- | --- | --- |
| Offline plans, read-only CLI/MCP, typed read OperationSpec | Trusted application schema/tenant declarations; supported types/structure; source artifacts protected | No DB execution permission, live freshness or physical identity; migration apply stays separately human-controlled |
| Conditional Strict and six Validate/ExecuteValidated families | Trusted immutable settings/current assertions, application-managed binding key/scope/generation, authenticated subject/tenant and real provider where required | Provider sees finite scope/family/target, not SQL/value-specific approval; no revocation service, commit/exactly-once or arbitrary external executor guarantees |
| Ordinary/generic/RETURNING/compound paths | Respect existing Strict/provider high-risk refusals, per-statement preflight and ownership rules; no fallback after refusal | No composite/RETURNING authorization; Compatibility may have earlier effects; direct SQLDB/sql.DB/sql.Tx/Executor lies outside interception |
| Public views and generated repositories | Public bounded views only for display; complete private reports for gates; generated source matches actual inputs | Views are not executable/authoritative; code equality and typed columns are not business authorization |
| Saved-model manifests/schema snapshots | Separate original/PG/native/companion owners; explicit current inputs, fixed expectations and partial/skip outcomes | Not handwritten main.go equivalence, full visibility, physical DB identity, live freshness or deployment approval |
| Static review/comparison | Retain full risk/precision gates and operation coverage; expose unsupported/unknown with their census | 187 gate-bearing warnings and 1,504 unknowns are unresolved under current release criteria; no whole-tree safety, narrower-context or closure witness guarantee |
| Explicit native performance/observation | Supported native versions/structure and bounded observation inputs; fixed fault classification and cleanup requirements | No automatic EXPLAIN, all-order cancellation, full hidden-fault detection, physical Close/ACK/reuse or universal performance bound |

### Adopted limited decision and future proposals

**Limited A adopted:** finish the finite scope, usage assumptions, non-guarantees
and seven-ID disposition for a later explicit human release review. Retain
unknown/skips and original historical attribution unavailable. Finite original
manifest coverage is supplied, with broader non-guarantees preserved. Native
(a)–(c) are referenced only at their documented evidence layers; (d) remains
unaccepted by a human release authority. Historical exploration resumes only
when existing raw evidence yields a new distinguishing artifact or a captured
new failure occurs. The original unknown record is retained permanently, even
when a later failure can be investigated. This is neither endless retry nor
permanent refusal to investigate new evidence. No production bug fix is invented;
the producer/closure study remains only a possible separately authorized design.

For blocker management, propose a **separate future specification/PR** with an
explicit per-ID decision record (candidate head, source evidence, finite scope,
non-guarantees, authorized human decision and invalidation/reopen conditions).
Missing, stale or conflicting decisions must remain blocked. Command success alone must never
clear an ID. This document is not a runner-trusted clearance artifact or reusable
certificate. This PR does not implement a waiver input, change aggregate behavior
or decide whether failed static gates may ever be accepted at release. That
release-policy decision must explicitly address the warning/unknown inventory;
until then current criteria still forbid release.

**Alternative B — not selected:** retain the original release obligations without accepting the
finite scope. Record release as unavailable; do not schedule repetitions that
cannot establish missing historical or invisible information. Cost: useful
finite features remain unreleasable under those criteria.

**Alternative C — not selected:** commission new identity/closure and native lifecycle guarantees
before considering release. This needs separate designs and acceptance criteria;
it may be source/API/semantics-affecting and cannot restore lost historical
observations. Cost and compatibility risk are substantially larger than docs.

Risk of A: users may mistake bounded checks for production-wide safety, and
observation-conditioned native results can conceal independent physical faults.
Mitigation is explicit usage restrictions/non-guarantees and an attributable
human decision, not green labels. This adoption explicitly changes the finite
scope of future acceptance; it does not satisfy unchanged universal requirements.
General integration or next-work approval is not candidate-specific release
approval. The delivered answer authorizes finishing this docs/diagnosis PR only;
it does not authorize ready, merge, Issue closure, whole-project acceptance,
new services/dependencies, tags, release or package publication.
