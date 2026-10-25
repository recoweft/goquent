# Candidate-bound release policy and human decisions

GQ-MAINT-16 revision 1; Refs [Issue 130](https://github.com/recoweft/goquent/issues/130).
Status: **A adopted as a design-only endpoint**, through delivered consultation
`e1a69a0e-c694-4c9e-bda5-9a0126db377d`. Operational acceptance remains separate.
No candidate is accepted. This document is not a clearance input or certificate.

## Adopted design boundary

The selected design, option A, is a manually reviewed, candidate-bound policy that could
permit an authorized human to accept specifically enumerated residual risks
within a finite use domain. This would change future release acceptance criteria:
current static gate failures and historical unknowns could remain failures and
unknowns while a separate policy assessment becomes eligible for human release
review. It would not satisfy the original universal obligations. Adoption of A
is separate from accepting any candidate, and does not authorize publication.
Design adoption does not activate an operational exception; existing criteria
continue to forbid release.

[MAINT15's adopted finite review scope](release-blocker-disposition.md) is the
starting boundary, not adoption of A. The existing [runner](../scripts/release_validation/run.py)
retains its seven fixed blockers and always reports release blocked. No runtime,
runner, aggregate, threshold, baseline, unknown classification, API, wire format,
provider, binding or authorization behavior changes in this work.

Option B retains existing release criteria and records release as unavailable;
this design can end as a rejected proposal without another implementation PR.
Option C commissions additional identity/closure or native lifecycle guarantees
before release consideration. C requires separately scoped design and acceptance;
it cannot recover lost historical observations. No such producer or native
implementation is authorized by this proposal or by absence of a demonstrated bug.

A adds decision traceability, not execution coverage. Its main risk is human
misinterpretation of finite evidence as universal safety. B has no acceptance
change but leaves release unavailable. C may require API/semantic changes and
substantially more validation. All options preserve current compatibility and
raw evidence; none makes green CI or a general merge approval release authority.

## Five categories govern disposition

The seven IDs identify inventory groups, not issue types. Each individual item
within an ID requires its own category, basis/evidence, responsible owner and
ending condition. Preserve previous classifications and the reason for any
change. Several categories can coexist within one ID; no group-wide relabeling
can turn a defect or missing evidence into accepted residual risk.

| Category | Basis and owner | Finite ending condition |
| --- | --- | --- |
| (a) Implementation defect | Reproduced violation in the promised supported domain, new fixed-expectation violation or cleanup/watchdog failure; implementation maintainer | Capture a minimal reproduction, repair and demonstrate sensitive regression controls. Reclassification as a guarantee limit or risk acceptance is not repair |
| (b) Verification gap | Required evidence absent, stale/incomplete, or input correspondence unobservable; validation owner | Supply the necessary scoped evidence and verify input/result correspondence. PR127's original four axes are already supplied historically; actual candidate checks remain independently required |
| (c) Contract guarantee limit | Dynamic SQL reconstruction, all cancellation orders, invisible driver outcomes or physical DB identity outside the guarantee; design/review owner | State supported scope, assumptions and non-guarantees. This finishes design disposition, not candidate acceptance; that remains (e). A demonstrated supported violation becomes a separate (a) item |
| (d) Historical cause undetermined | Original testmain/expiry/cancel identifying artifacts unavailable; investigation owner | Preserve original observations, missing artifacts, bounded investigation and stopping condition. Pause exploration until a distinguishing artifact or captured recurrence; never overwrite the old cause with later success |
| (e) Release-policy decision | Candidate-specific warnings/unknowns/non-guarantees; independently verified human authority | Record explicit accept/reject/defer bound to candidate, evidence, use and expiry. Reject/defer concludes that decision but does not make the candidate eligible |

For review-risk-precision and review-unknown, current contract limits are (c),
candidate residual decisions are (e), missing candidate evidence is (b), and only
supported reproduced violations become (a). Manifest-coverage combines supplied
historical verification with candidate (b), broader (c) and human (e). Native
combines current failure triage (a)/(b), observation limits (c), original cause
(d) and decision (e). Historical-testmain/expiry combine (d) with current (b),
triggered (a) and candidate (e). Human-release-review is (e), with missing
verification of authority tracked as (b). These mappings do not assert any new
current defect. Unresolved does not mean everything needs implementation;
non-reproduction does not mean everything can be accepted.

## Candidate and evidence dossier

The following is a required logical record, **not a new JSON schema or parser**.
Every field is required; explicit unavailable values block assessment where the
field is necessary. A document author cannot fill missing proof with `approved=true`.

| Record | Mandatory contents and binding |
| --- | --- |
| Policy | Adopted policy version/full document commit, authenticated adoption reference, exact allowed residual categories, mandatory checks, authority designation and decision expiry rules |
| Candidate | Repository identity and complete commit SHA; clean exact tree; intended distribution artifacts, names/types and byte digests; artifact-to-source build provenance; no floating branch/tag as identity |
| Build/dependencies | Exact toolchain, build command/options/tags, platform/architecture/cgo, dependency versions/checksums/replacements/workspace, generated inputs, packaging recipe and external inputs; explicit absent components; artifact bytes checked against the recorded build |
| Evidence | Independent evidence ID, source repository/full source head, comparison base/head where relevant, immutable run/artifact reference and digest, command/tool/environment/input identity, capture interval, real exit, raw stage/gate outcomes and completeness; partial/unsupported/unknown/skip/not-run explicitly retained |
| Inventory | Exactly one row for each of the seven IDs below; complete candidate-specific finding/operation/comparison populations and source attribution, not totals alone; per-item category, evidence/basis, responsible decision owner and finite ending condition; category-change history, repair triage, acceptance/rejection reason, remaining risks and reopen conditions |
| Use domain | Target functions, dialects/deployment conditions, caller/owner assumptions, exclusions and explicit non-guarantees; link the MAINT15 function matrix and state the precise selected subset |
| Decision | Stable authenticated subject identity, independently designated role and authority reference, UTC decision time, immutable/versioned external reference, exact dossier digest and candidate/artifact identity, per-ID accept/reject/defer, rationale, validity deadline, scope and revocation/supersession links |
| Verification | Named verifier, verification time, actual authority/consent sources consulted and versions, evidence integrity checks and unresolved limitations; independent publication decision and intended channel/artifacts remain separate |

Evidence that claims current execution must match the exact candidate and actual
inputs. Earlier execution is historical evidence with its original source/head,
not a current run. For bounded historical mechanism evidence, explicitly compare
relied-on source, fixtures, dependencies and owner inputs against the candidate,
record equality or changes and limits, and require targeted revalidation for any
changed relied-on scope. Equality alone does not establish current host/DB results.
A current required check cannot be replaced by that historical reference.
Unobservable input correspondence, missing raw results or unexplained mismatch
blocks assessment. Raw artifacts remain outside the checkout and analysis roots
in 0700 directories, 0600 files and owner-only executables, without upload;
reviewers need authorized local access to complete evidence. Public projections
and digests alone do not prove the underlying evidence is valid.

MAINT15 measured main `23059408f16d463bbe9594b5d29040a3b7be5fc3`:
ordinary and strict exits were 1, input valid, risk/precision failed, with 187
high-or-above warnings and 1,504 comparison unknowns. These are historical
inventory, not expected candidate counts or proof of production defects.
Test-source provenance never makes a warning safe or excluded. A new candidate
requires complete re-binding of its actual review inventory and fresh required
ordinary/strict results; no docs-only hash exception, narrower context assumption
or automatic reuse of 187/1,504 is permitted. This docs work does not nominate a
release candidate and does not repeat that census without purpose.

### Freeze without self-reference

First finalize and commit policy/design changes, then choose an exact candidate
commit and build its artifacts. Collect required candidate evidence without
mutating that tree. Record candidate decisions **outside the candidate tree** in
an independently authenticated decision channel, referencing the frozen dossier.
Adding a decision file to Git, editing documentation, merging or rebuilding with
changed inputs creates a different candidate/artifact binding. It requires new
assessment and decisions; do not recursively edit the candidate to record its own
final hash. No file extension or documentation exclusion shrinks the hash domain.
An external record's edit creates a new version requiring re-verification; the
old version remains historical. Recheck tree, artifact and input identity at the
end of collection, assessment and any separately authorized publication step.
Mutation during a check invalidates that check; mutation after a decision expires
that binding. Failure to observe a stable boundary means blocked.

## Authority and consent trust boundary

No existing verified release authority registry or authenticated release-decision
service has been established by this work. GitHub push/admin access establishes
repository capabilities only, not delegation to accept release risk. Runtime
external-authorization providers, HMAC/binding keys and public Plan/Report views
are not release authority. No new authentication service or signing system is
proposed.

A can operate manually only after the project owner supplies an independently
verifiable designation through an existing authenticated project governance
channel. The designation must identify the actual subject(s), decision scope,
validity period and who can revoke it. Record its stable reference and version.
For example, an existing GitHub governance record can be used only if a reviewer
can independently authenticate the designator's governance authority and read
its author identity and version through the authenticated service. The worker's
own statement, copied text, controller public projection or PR approval alone is
insufficient. A controller consultation answer adopts policy only to its stated
scope; it is not automatically a candidate release decision.

For each assessment, a named human verifier must:

1. Read the original designation using the existing authenticated channel; check
   the designator's authority, subject identity, scope, expiry and revocations.
2. Read the original candidate decision there; verify author identity, timestamp,
   exact candidate/artifact/dossier binding and affirmative consent for every ID.
   A writer's self-asserted role or a locally created file cannot satisfy this.
3. Check all authorized decisions for conflict, supersession and revocation.
   Two authorities disagreeing means blocked, never latest-write-wins or majority
   inferred by a tool. Resolve through an explicit authorized supersession that
   names both records and reconsiders the whole dossier.
4. Record verification references/time and recheck immediately before any future
   publication decision. Inaccessible, edited without retained version, ambiguous
   or unverified authority/consent means blocked.

This manual process cannot guarantee instantaneous revocation visibility,
identity behind a compromised account, absence of undisclosed decisions, or an
atomic check-to-publication boundary. If the existing channel cannot establish
the necessary authority, complete decision set or stable versions, A is not
operationally usable and remains blocked. Adoption must acknowledge these manual
limits; no machine-readable boolean bridges them. This work verifies no actual
human designation or candidate consent and performs no publication operation.

## Per-ID repair versus explicit residual decision

Every row remains present even if finite evidence has been supplied. Each residual
acceptance below remains conditional on operational policy authorization, complete candidate evidence,
authenticated authority and an explicit accept/reject decision. Rejection or defer
blocks; none is an implemented waiver. All applicable current checks must meet
their fixed expectations except the specifically enumerated static residuals A
would permit the human to consider. Fresh failures are not historical residuals.

| Fixed ID | Repair/investigate condition | Finite residual the human may accept or reject under A |
| --- | --- | --- |
| review-risk-precision | Supported minimal reproduction violates the current analysis/runtime contract, or an actual unsafe supported use is demonstrated: isolate a separate minimal repair and its regression. New/unexplained findings require triage, not inheritance | Exact complete candidate warning/operation inventory, high/partial gate exits and missing runtime-policy limits; accept only the named use domain with no claim of whole-tree safety. Preserve all warnings, thresholds and unsupported items |
| review-unknown | Incorrect classification/correspondence in the supported domain needs a minimal repair; current input failure cannot be accepted here | Explicit complete candidate unknown inventory and ordered attribution under the unchanged exact-context contract; identity/context limitations remain unknown. No closure witness or unknown-to-unchanged promotion is implied |
| manifest-coverage | Current supplied inputs fail any required axis, generated bytes/path drift, owner mismatch or stale evidence: correct the real defect/input and revalidate without regenerating away unexplained drift | Original saved MySQL model's schema/policy/generated_code/database four-axis evidence only, with broader physical identity/freshness/visibility limits. Separate PG, native-owned derivatives and companion remain separate. Handwritten main.go is not certified |
| native-performance-limits | New fixed-expectation classification, resource cleanup or watchdog failure blocks and requires capture/minimization and repair where supported; known CLEANUP precedence remains | Observation-conditioned native evidence for the declared versions/roles/cases; retain (a)–(d) in MAINT15 and Issue123. No all-order Tx.done, hidden independent fault, physical Close/rollback/server ACK or same-connection reuse guarantee; original PG/MAINT12 cause remains unknown |
| historical-testmain-cause | New distinguishing artifact from existing raw evidence or a captured new failure reopens investigation; reproduced current contract violation needs a separate repair | Permanent original-trial attribution unavailable, alongside separately sourced MAINT03 finite correspondence repair and negative controls. Stop repeated reconstruction without those triggers; later green does not establish original cause |
| historical-expiry-cause | New distinguishing artifact or captured new final-gate/expiry failure reopens investigation; current expiry/cancel/dispatch violation needs repair | Permanent original error/counters/cause unavailable, alongside MAINT05 deterministic fixture evidence. No expiry grace or production-clock change; current successes do not explain the historical trial |
| human-release-review | Missing/invalid authority, false binding, conflicting decisions or unsupported safety claim requires correction/review; it is not solved by a production patch | Authenticated human assessment of all six technical rows plus the whole selected use domain, remaining risk and manual trust limits. This row records dossier review only; a separate explicit publication approval is still necessary |

The original four-axis gap was supplied by PR127; do not invent another missing
axis task. Preserve the [saved-model owner table](../examples/ai-safe-orm/storedmodel/README.md),
[finite native boundaries](native-performance-boundaries.md) and
[unresolved native track](https://github.com/recoweft/goquent/issues/123#issuecomment-6386876158).
Historical original-trial unknowns are retained permanently even if a later
failure is repaired. Adopted stop/reopen conditions remain in force.

Current `input_failed`, mismatched artifacts, unexplained new test failures,
cleanup/watchdog failures and new vulnerabilities are hard stops for this policy
assessment. Capture and triage them separately; no historical acceptance absorbs
them. A required skipped/not-run/partial check cannot become pass. Partial static
coverage is eligible for explicit residual consideration only as such, with its
full inventory; incomplete collection or missing required evidence is not.
New vulnerabilities require impact/remediation assessment before a new decision;
an old acceptance never authorizes them automatically.

## Four completion states, independent of the three decision layers

Every status record must name its scope, evidence, incomplete reason (or explicit
none), responsible owner and concrete ending condition. A completed collection
is not necessarily completed verification or satisfaction of candidate criteria.

| Completion state | Scope, owner and ending condition | Evidence and incomplete reason for this design work |
| --- | --- | --- |
| Implementation completion | Maintainer: finish the adopted implementation scope; here only the design document, no runtime implementation | Adopted document diff and paper-case review; final committed scope is recorded in the PR, not inferred from adoption |
| Verification completion | Validation owner: perform and review all required scoped checks and their actual results | Link/format/diff checks and exact-head CI recorded separately in the PR; required missing evidence or unexplained failures keep this incomplete |
| Integration completion | Integration authority: merge the exact reviewed change | No merge performed by this work; draft PR is not integration evidence |
| Release approval | Independently authorized human: explicitly accept a particular candidate, use domain and residual risks | Absent: no candidate or authority verified here; separately bound artifact/channel publication consent is also required |

Record collection execution finished, scoped verification complete, and candidate
acceptance criteria satisfied as separate facts. An expected, fully observed
unknown does not make its bounded verification perpetually unverified; its raw
unknown and acceptance consequence remain. Conversely, missing required evidence
or failure of a required verification cannot be relabeled verification complete.
For example, a complete census can validate attribution while its inner risk gate
fails; record both without calling the candidate accepted.

The following three layers are a different management axis, not substitutes for
these four completion states. Design completion or a later merge establishes no
release approval.

### Three decision layers

| Layer | States and rule | What a future management implementation could change |
| --- | --- | --- |
| Raw collection/gates | Existing stage pass/fail/skip/not-run/unverified and real exits; collector complete/error; runner release=blocked | Nothing: retain fixed BLOCKERS, aggregate, checks and raw results exactly |
| Candidate policy assessment | Proposed blocked / eligible-for-human-publication-review / invalidated, always qualified by policy version and candidate | A separately authorized management process could compute completeness and refusals and record verified human residual decisions; it cannot call raw failures pass or remove runner blockers |
| Human publication authority | Absent / explicitly approved for exact artifacts/channel / rejected / revoked/expired | A separate authenticated human action is required after eligible assessment. Eligibility and CI success never trigger release, tag or package publication |

All seven inventory rows, including human-release-review, are required for
eligible assessment. That row's dossier consent is distinct from permission to
publish. There is no loop in which raw runner success or a decision flag clears
itself. This design supplies no publication integration or executable transition.

Expire/reopen on candidate, artifact, build/dependency, evidence, owner, selected
function, usage assumptions or scope changes; expiry deadline; authority loss;
revocation; conflict; new failure/vulnerability; or evidence mutation/unavailability.
Retain old decisions and causes, invalidate readiness and any dependent publication
approval, and re-evaluate the new binding. Even narrowing scope requires explicit
reconsideration. Never silently extend time or reuse an approval for another build.

## Decision tables: falsification before implementation

These are paper cases, not new executed tests. C denotes a complete hypothetical
candidate identity, not this PR's commit or an accepted release.

| Dossier and decision condition | Raw result | Policy assessment / publication |
| --- | --- | --- |
| C has immutable matching artifacts, complete current evidence, all seven unique rows, verified unexpired authority/consent, no hard stop; operational A separately authorized; exactly enumerated static residuals explicitly accepted | Static gates still fail and unknowns remain; runner blocked | Eligible for human publication review only; publication absent until a distinct explicit decision |
| Same C with design-only adoption, no operational authorization, or option B | Unchanged | Blocked; proposal cannot override criteria |
| All command exits 0 but decision/authority absent | Runner still blocked | Blocked; run success never substitutes for authority |
| All seven rows present but one reject/defer, or two authorized subjects conflict | Unchanged | Blocked; no precedence guessed |

| Counterexample | Required refusal and recovery |
| --- | --- |
| Missing row, duplicate ID, unknown ID, empty inventory or only 187/1,504 totals | Blocked; reconstruct the exact complete seven-ID and candidate inventory, obtain new review |
| Stale evidence, another source/head presented as current, different comparison base or mismatched artifact digest | Blocked; retain historical identity, collect correct required evidence and rebind decisions |
| Required skip/not-run converted to pass; negative manifest regression described as positive verification | Blocked; restore actual outcomes and obtain the missing required evidence |
| Input failure, same-session mutation or build inputs changing after collection | Blocked/invalidate; freeze anew and recollect affected evidence; no doc/hash exception |
| Scope/function/owner or dependency/toolchain version changes after consent | Invalidate even if source commit unchanged; new dossier and explicit decisions required |
| Revocation, expired designation/decision, new test/cleanup/watchdog failure or vulnerability | Invalidate readiness and dependent approval immediately; investigate separately before new review |
| Local approved flag, public projection, green CI, generic merge approval or runtime binding offered as authority | Blocked; independently authenticate delegation and candidate-specific consent |
| Later green run claimed to resolve original historical cause | Refuse causal claim; preserve unknown permanently, assess only the new run's finite evidence |

## Design-only endpoint and one conditional follow-up

The selected endpoint is this design document. No follow-up work is started.
An eventual operational option is manual review with this procedure;
no parser, signature, runner code or new tests are needed in this revision.
Only if separately needed and authorized with a named purpose and verified
authority channel, the **one minimal follow-up PR candidate** is docs-only:
add a single operator checklist/template for external candidate dossiers, with the
adopted authority designation/verification procedure, explicit mandatory evidence
selection per supported use domain and the paper cases above. It must not embed
an actual candidate approval in its own tree or expand the inventory into another
open-ended diagnosis ledger. Before that work, an actual existing authority
channel must be identified and verified; otherwise report blocked, not an invented
service. This follow-up is a proposal, not authorization to start it now.

Necessary checks for that follow-up: link/format/diff checks; exact seven-ID
coverage against the unchanged tuple; manual walk-through of one hypothetical
complete dossier and every refusal row; confirmation that no code, dependencies,
thresholds or runtime contract changed; repository-required exact-head CI with
skips distinguished. A future actual candidate still needs fresh ordinary/strict
inventory, original four-axis checks and the use-domain's required finite tests;
historical/native source equality is reference evidence, not fresh execution.
Do not run full/race/native/census repeatedly for a docs-only template.

A was adopted only as a design reference: this PR is the finite endpoint and no
follow-up is necessary. Unverified operational authority does not prolong this
design work indefinitely. Any later automation would need separate
scope and acceptance, including authority verification and mutation/refusal tests;
it is not implied by the checklist proposal.

## Delivered decision and next candidate review

Consultation `e1a69a0e-c694-4c9e-bda5-9a0126db377d` selected A's design-only
endpoint with the five categories and four independent completion states above.
It adopts the per-ID design boundaries, hard stops, external candidate freeze,
authority/consent verification and invalidation design. It does not activate a
release exception, accept 187/1,504 or any current main/PR candidate, verify raw
artifacts or governance authority, or certify final-head CI. The delivered text's
proposal-origin labels do not expand the scope of this design adoption.

The next actual candidate review must decide once, against a complete frozen
dossier: operational policy authority and applicable acceptance criteria; the
selected functions/assumptions and all seven per-ID residual decisions; verified
subjects, designation and consent references; expiry/reopen conditions; and,
separately, any explicit artifact/channel publication approval. Missing authority
keeps operation blocked. No new ledger-cleanup work or unconditional automation
follows from this document.

This work proceeds only through document alignment, appropriate checks and one
draft PR referencing Issue130. Operator checklist work, parser/schema/signature
infrastructure, runner changes, new tests, producers and broader native guarantees
are not part of the adopted scope. Draft review, merge, whole-Issue acceptance,
candidate acceptance and publication remain separate decisions.
