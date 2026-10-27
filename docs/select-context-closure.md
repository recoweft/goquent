# Bounded SELECT dependency closure investigation

GQ-MAINT-15 PR5; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
This is stage two of the [SELECT identity investigation](select-comparison-identity.md).
The specification decision retains design, diagnosis and test-only falsification;
it does not adopt a managed execution environment or change comparison semantics.

## Candidate and proof obligation

Use exactly the buildable literal Table/Select/Where/Get member in
[select_identity_test.go](../orm/review/select_identity_test.go). It accepts a DB
parameter without executing its function. SQL, columns and bindings must be
known; policy and runtime correspondence remain unknown. The candidate edit is
the listed addition of `note.txt` containing ordinary text. Both roots are
independently compiled; the physical terminal and containing source stay equal.
This edit already yields unknown under whole-tree comparison. Neither that
result nor equal diagnostics proves that every relevant consumer ignores it.

A complete owner would need to bind the actual inputs of discovery, type loading,
reconstruction and classification for both runs, including input selection and
absence. No such composite owner exists in the current implementation. Individual
owners below provide useful partial evidence. They must not be concatenated into
a claim of completeness. Potential inputs are distinguished from observed effects:
we do not claim that every external process input affects every SELECT, or that
this particular text addition changed the Go process's output.

## Input inventory and missing ownership

The candidate owner is one synchronous RunCI invocation over immutable paired
inputs. The table describes obligations, not implemented new capture fields.
All rows require validation before loading and immediately before classification;
intermediate reads alone cannot rule out mutation and restoration. Missing closure
means no narrower witness and retains unknown where current precedence permits.
An existing diff, capture or external-correspondence failure remains input failure
(exit 2), not a successful unknown comparison.

| Input group and consumer | Existing owner/capture | Missing evidence, validation and invalidation |
| --- | --- | --- |
| Containing source, physical terminal, helper bodies and constants; parser/tracker/sampleSelect | RunCI tree snapshots; Session source bytes; loader ParseFile capture; private operations | Bind exact bytes, offsets, typed operation and unique physical mapping to the selected variant. Changed containing bytes follow existing new/worsened rules. Changed helper context refuses narrower reuse even when literal reconstruction is equal. Missing or conflicting mappings refuse. |
| Discovered file/directory sets, ignored files, presence and absence; discovery and Go selection | Bounded source-tree enumeration; package metadata for the finite native test-main exception | No complete shared negative-dependency owner. An added/deleted member, nearer module/workspace, driver candidate or selected build file can change selection. Bind search directories, existence/nonexistence and selected sets on both sides; incomplete enumeration or changed selection refuses. A suffix is not an ownership proof. |
| Imports, export data, module dependencies, standard library and cache; go/packages and Go process | ParseFile records syntax actually parsed; metadata records graph/file roles for test-main classification | Export/cache reads are not all ParseFile calls. Bind graph, export bytes/provenance, cache selection and toolchain inputs or refuse. Same imported package name or successful load is insufficient. No export/cache perturbation experiment is claimed here. |
| go.mod/go.sum, replacements, vendor selection, go.work/go.work.sum and ancestor search; moduleRoot and Go process | Session adds discovered module files; root snapshot includes files within its boundary | Workspace and ancestor absence, external replacement contents and search selection have no complete immutable owner. Revalidate search results and selected files; missing sums and external changes cannot be inferred harmless. GOWORK=off in a test is a restriction, not general closure. |
| Toolchain, compiler and executable selection; subprocess launcher | Current code delegates loading; nativePackageDriver checks driver selection | Version strings do not bind executable bytes, libraries, tools or PATH absence. Need exact selected executables and their bounded input/access domain; replacement or unknown provenance refuses. No executable replacement was performed. |
| GOOS/GOARCH, cgo, tags, GOFLAGS, GOENV, workspace/toolchain settings, process environment; Go process | packages.Config supplies readonly module flag but no explicit Env; inherited environment remains active | Need effective settings, selected env/config files and child-process inputs fixed throughout the run. cgo may add compiler/header inputs. A recorded subset is not exhaustive. Changed or unowned settings refuse. |
| Ordinary/internal/external test package variants; loader and tracker | Tests=true; syntax-count sort prefers the larger syntax variant; one typedFile retained per path | No explicit selected-variant descriptor is retained on the operation. Need stable package/test identity and selected file set, including absent alternative members. Ambiguous or incomplete loads refuse; metadata agreement used for test-main roles is not a general variant witness. |
| Custom/automatically discovered driver; go/packages | nativePackageDriver checks GOPACKAGESDRIVER and PATH lookup | Detection does not own driver executable or arbitrary process reads. Initial candidate must refuse custom drivers; even native selection does not close all other rows. No custom-driver non-consumption guarantee is adopted. |
| Overlays and parsed inputs; parser and loader | Session inputs passed as overlay; ParseFile prefers overlay bytes and accumulates parsed source | Need immutable overlay map, origin, exact path mapping and agreement with discovery/metadata. Overlay source alone does not fix export data or process selection. Changed/missing capture refuses; existing captureValid checks retained. |
| Config, suppressions, settings, policy, manifest, SQL/JSON, evidence callbacks; review and planner | Explicit config/manifest files and CurrentManifest serialization in Session; RunCI remaps supported context and disables trust for certain supplied inputs | Bind in-memory options and every actual consumer, not extension-based labels. Evidence, CurrentManifest and ManifestInputs remain outside candidate reuse. Unknown policy stays unknown. Missing/unmappable config or mutated inputs refuse; caller claims never establish closure. |
| Other external files and process observations; loader subprocesses | Exact external captured-path/byte comparison with one finite native test-executable exception | There is no all-process input/access boundary, including negative lookups. Absence from capture cannot prove non-consumption. Unowned inputs refuse closure; captured mismatch retains input failure. Test-main exception keeps its graph/role and same-session checks unchanged. |

Concrete implementation references: [types.go](../orm/review/types.go)
`loadReviewTypesCaptured`, `moduleRoot` and `typedFile`;
[precision.go](../orm/review/precision.go) `newSession` and `unchanged`;
[type_testmain.go](../orm/review/type_testmain.go) `nativePackageDriver` and metadata
roles; [ci.go](../orm/review/ci.go) `collectCI`, `captureValid` and `runCI`.
`collectCI` still stops at the first matching operation location. It does not
establish uniqueness. A future narrower witness cannot inherit that assumption.
The pinned x/tools v0.43.0 source was also inspected: `go/packages/packages.go`
uses `os.Environ` when Env is nil and opens ExportFile in `loadFromExportData`;
`external.go` launches the selected driver. These are concrete paths outside a
ParseFile-only inventory, not observations that the candidate edit changed them.

## Finite evidence and falsification

Production source measured is main `aa7897bcf84bb04de24b4a385643619f77149fb3`,
the PR147 merge, with dependency head `0899ab94a5df3913b2589d729cea98f7e25d75be`
verified as its ancestor. Independently resolved latest main and its exact
CI 40488930697, Security 40488930437 and Docs 40488930459 all succeeded before
editing. The earlier CI-in-progress observation was not treated as success.
The final document/test commit and its checks are recorded separately in the PR.

Local fixture environment: actual Go 1.26.10 linux/amd64 on Ubuntu 24.04.5/WSL2,
cgo enabled, GOTOOLCHAIN=local, GOPROXY=off, GOWORK=off,
GOFLAGS=-mod=readonly, GOPACKAGESDRIVER=off, no explicit tags, private writable
cache and temporary directory. Existing module pins are unchanged. These settings
make the experiment reproducible within its stated boundary; they do not inventory
all inherited environment or prove hermeticity. No live DB is used. Raw logs and
fixture material stay outside the repository/comparison roots with owner-only
permissions and are not uploaded.

| Control | Independent expectation and evidentiary limit |
| --- | --- |
| Identical literal source | One SELECT, known static axes, unchanged star diagnostic; risk/precision gate exit 1. Identity positive only. |
| Listed text addition | Same SELECT, unknown, valid input and exit 1. No non-consumption witness. |
| Parsed dependency edit; build selection change | Same literal terminal, unknown; selected operations retained, including unsupported coverage. Same constant value does not close selection. |
| Added/deleted same-package source member | New PR5 cases vary presence of value.go without changing the SELECT. Both sides compile; expected unknown and exit 1. These test real set transitions, not every negative lookup. |
| Moved terminal; second equal SELECT | New positions remain new. Equal SQL cannot merge distinct locations. |
| Existing census context/identity/precedence controls | Same/listed/unlisted contexts, incomplete load, missing operation, duplicate suppressed/prior keys, new/worsened/matched ordering and full gate/coverage parity. No whole-tree census run. |
| Capture and external controls | Same-session bytes mutation, missing source and external path/byte mismatch refused; added first test can introduce ordinary external inputs and yields input failure. |
| Session helper and duplicate-ID controls | Helper mutation and receipt-ID collisions cannot create planner correspondence. These are not a proof of duplicate-operation rejection in collectCI. |
| Config/policy/build/dependency finite model | Existing model falsifies its own finite candidate rule. Its supplied closed flag and synthetic positive are not real-loader evidence. Real config/policy transition coverage is limited; no exhaustive owner claimed. |

No test asserts a new narrower-context positive. Unmeasured workspace search,
export/cache perturbations, arbitrary external consumption and duplicate physical
operation mapping remain explicit gaps. Additional repetitive census runs would
not distinguish these missing premises. Historical 1,014/496 unknown counts and
the PR146 source census of 1,510 are not current comparison measurements.

## Outcome and stopping contract

**Additional premises are needed; a real closure witness is not established under
the current contract.** The concrete obstruction is the unowned selection,
negative lookups and external process inputs above, together with missing selected
variant/unique-operation evidence. This is not a proof that such a witness is
impossible in every architecture. It does not repair MAINT12's eligible-zero
measurement or imply any decrease in unknowns.

| Stage | Disposition |
| --- | --- |
| 1: identity | Literal SELECT identity example established within existing full-context comparison. |
| 2: closure | Candidate edit tested, but complete non-consumption ownership unavailable; no changed-context witness. |
| 3: production PR6 | Not started; prerequisite unmet. Do not implement doc/source exemptions or unknown promotion. |

One future option is a separately approved managed execution contract: immutable
input roots, fixed executables and dependency/export provenance, explicit effective
environment, restricted driver selection, bounded process access and ownership of
absence/selection sets. Fixed versions or environment variables alone are not
sufficient. It would restrict applicability to that environment and require runner,
cache-provenance and platform maintenance plus proof that existing byte/file/time
budgets remain sufficient. The smallest next design unit would be an independently
validated owner for one native literal fixture and one edit, with adversarial
selection/access controls. No runner, new guarantee or follow-up work is adopted
here. The lower-cost alternative is retaining current unknown outcomes.

Existing new/worsened/matched precedence and complete head risk, precision,
manifest and operation gates remain authoritative. No baseline, suppression,
threshold, test exclusion, manifest regeneration, public API/wire, private identity,
loader or production comparison change is made. The MAINT11 diagnosis-only and
MAINT12 design-only boundaries, seven blockers, finite manifest evidence,
historical native residuals and human release judgement remain intact. This draft
is not whole-Issue acceptance, merge readiness or permission to start PR6.

## Validation record

The focused command selected `TestBoundedSelectIdentitySourceCounterexamples`,
`TestComparisonCensus.*`, `TestCIUnicodePhysicalLocation`,
`TestCIWorsenedAndStructureComparison`, `TestCICaptureRetainsExactBytes`,
`TestCIChangedSourceNewExternalContextRemainsUnknown`,
`TestCIChangedSourceTestVariants`, `TestCIBaselineChangedContextAndAdversarialInputs`,
`TestPlannerSessionHelperSnapshotAndDistinctIDs`, `TestPlannerCorrespondenceSession`
and `TestDocumentContextFiniteModel` from `go test ./orm/review -count=1 -json`.
It exited 0 with 78 test pass events and zero failures/skips. The eight literal
source cases each independently compiled both roots; their inner RunCI exit was
1 as expected, not a successful risk/precision gate. No initial focused failure
was hidden by a retry. `go mod verify` and the public-output wrapper check passed.
Local full/race/DB/native suites, whole-tree CLI review/census and manifest
regeneration are not run for this finite design task. Repository CI remains a
separate exact-head check, not local execution or proof of input closure.

Three additional real-review controls (`TestRunLintsConfigSuppressions`,
`TestMatchedPlannerContextDoesNotFillStaticAxes` and
`TestExternalQueryPlanMarkerLocationAndConfigExpiry`) passed with exit 0,
zero failures/skips. They check config/expiry and planner-policy separation,
not a complete paired config/policy dependency closure. The strict comparison
input checker, local documentation links and `git diff --check` also passed;
changed Go source was formatted with gofmt. No dependency or manifest changed.
