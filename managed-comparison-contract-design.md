# Managed execution for bounded comparison

GQ-MAINT-15 PR6; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
**Design proposal only. No real closure witness or narrower-context guarantee is
established.** This PR designs additional premise B after
[PR148](https://github.com/recoweft/goquent/pull/148); it is not the production
stage three anticipated by the earlier plan. A later implementation requires a
separate reviewed scope and approval.

## Decision and current boundary

Recommend one Linux/amd64 managed execution profile with immutable inputs,
a restricted process domain, and a private comparison owner. Start with the
literal Table/Select/Where/Get fixture in
[select_identity_test.go](../orm/review/select_identity_test.go), and exactly one
listed addition: root-relative `note.txt`, ordinary text, absent in base and
present in head. Do not generalize to arbitrary documents or projects.

Current production still uses whole-tree comparison. Stage one established
literal SELECT identity; [stage two](select-context-closure.md) found no complete
closure owner. [MAINT12](document-context-proof.md) remains a finite model and
eligible-zero record. [Limited A](release-blocker-disposition.md#adopted-limited-decision-and-future-proposals)
retains the finite review scope, seven blockers and separate human release
judgement. This proposal adopts none of B's operational guarantees. Historical
1,014/496 unknowns concern an old comparison pair; PR146's 1,510 source findings
are not a new comparison measurement. No counts are remeasured here.

The proposed statement is narrow: within the admitted profile, the listed text
addition cannot change the inputs used for this operation's typed reconstruction
or classification. Discovery and exact-diff validation **do** observe the added
file. They may reject the pair; they must not convert its bytes into SQL evidence.
A future unchanged display would not prove source execution, runtime policy,
DB identity/authorization, safe SQL, or precision on unknown axes. Full head risk,
precision, manifest and operation gates remain independent.

## Alternatives and recommendation

| Choice | Existing mechanisms reused | New mechanisms and maintenance | Applicability, user cost and limitation |
| --- | --- | --- | --- |
| A: managed isolated pair, recommended for a separately approved prototype | Exact tree/diff checks, real go/packages loader, parser/tracker, private reports, current full gates, physical identity controls | Small launcher and supervisor; immutable root provisioning; bounded input projection described below; explicit env; private variant/unique-operation evidence and comparison lifetime. Maintain one kernel/runtime profile, toolchain/image inventory and adversarial boundary tests on each upgrade. No service or framework is required by the design. | User supplies two complete snapshots, exact patch and supported profile. Dependencies must already be provisioned. cgo, workspace, external drivers and arbitrary config are initially excluded. Setup, two cold loads and sandbox maintenance cost more than current review. Only this path aims to establish the text-addition witness. |
| B: conservatively fix the entire source universe; retain current unknown | Current RunCI, whole-tree equality, input failure and gate rules; existing CI | No closure owner, runner, new dependency or comparison format. Maintain current tests only. | Works in today's supported review environments with their existing host trust. Identical trees can retain existing unchanged behavior. Any note addition changes the tree and remains unknown. Accepting edits only outside both comparison roots is cheap but does not solve the requested in-root example. Version pins or an ordinary shell do not make this alternative hermetic. |

Choose A as the bounded research implementation proposal, and B as the production
fallback until A passes its acceptance tests and a guarantee is separately
adopted. A fixed image alone is insufficient: the host kernel, exposed mounts,
process descriptors and dynamic inputs remain relevant. No image is pulled,
runner started, dependency added or environment policy altered by this design PR.

## Proposed profile and trusted foundation

Use one provisioned Linux/amd64 machine or VM with a reviewed kernel/runtime
configuration, not the user's ambient shell. Pin the actual Go 1.26.10 toolchain
and existing module versions for the first slice. This is a candidate profile,
not a claim that this worker's WSL environment implements it. Windows, WSL host
interop, Darwin, other architectures and other kernels require separate profile
qualification. A version/tag identifies a request, not immutable bytes.

Trust the provisioner, kernel and isolation runtime to enforce mounts, descriptor
closure, process/network restrictions and resource limits; trust the reviewed
Go toolchain, Goquent producer/consumer and projection code to implement their
stated semantics. Record their executable/root content identities and profile
revision. A malicious privileged host or a compromised compiler is outside this
claim. These trusted components are not proven correct by hashing them.

Enforce and verify, rather than merely record, the following initial choices:

- GOOS=linux, GOARCH=amd64, GOAMD64=v1, CGO_ENABLED=0, GOTOOLCHAIN=local;
  native gc tools only, no custom compiler, `-toolexec`, plugin or source execution.
- GOENV=off, GOWORK=off, GOPACKAGESDRIVER=off, GO111MODULE=on;
  fixed `-mod=readonly`, no caller flags/tags/overlay or alternate build driver.
  Pass a complete explicit environment to the producer and both package loads,
  not an append to the host environment. Clear duplicate keys and reject unknown
  caller overrides, including GOCACHEPROG, GODEBUG and GOEXPERIMENT.
- Fixed GOROOT, GOPATH, GOMODCACHE, GOCACHE, HOME, temporary roots and PATH.
  PATH contains only provisioned tools; empty/relative entries and current-directory
  search are forbidden. No shell startup files, host home, agent socket, VCS
  credentials, user GOENV or ancestor workspace/module files are exposed.
- Network denied by the process boundary, including descendants and local/Unix
  sockets; GOPROXY=off and GOVCS=*:off are additional settings, not enforcement.
  No module acquisition, generation, application/test execution or DB access.
- One explicitly identified fixture module and the frozen Goquent dependency
  snapshot. Its existing local replace must be relocated into the owned input
  domain with an exact manifest mapping; arbitrary external replaces are refused.
  No vendor mode, nested module/workspace, custom config, suppression, baseline,
  manifest, Evidence callback, CurrentManifest or ManifestInputs for the first
  positive. Their absence is checked, not inferred from omitted CLI flags.

These restrictions alter build selection. Projects requiring cgo, private modules
not preprovisioned, workspace replacements, build-time generators, external cache
programs, caller overlays, custom tags or dynamic review settings are not eligible.
They retain ordinary review and unknown behavior; they are not silently analyzed
under different settings and called equivalent. The general Goquent API and
current platform support do not change.

## Finite input domain and the text addition

A whole-tree snapshot reads the text and directory entries. The current
[readTree](../orm/review/ci_diff.go) does so for exact correspondence;
[discoverFiles](../orm/review/review.go) selects `.go`, `.sql` and `.json` for
review. Neither fact alone closes Go package selection, embedding or config
consumption. The proposed owner separates these consumers as follows.

1. Freeze the full base/head trees, including the added text, before discovery.
   Validate an exact patch with one addition and no unlisted changes. Reject
   links, special files, rename/move, binary diff and path aliases. Keep all
   containing-source bytes and physical coordinates equal. Full snapshots remain
   the authoritative diff and coverage inputs; never replace them with a filtered
   diff or rewrite the original comparison roots.
2. A bounded, trusted selection stage reads those full snapshots. Enumerate every
   member, selected and excluded Go file, package/test variant, config candidate,
   module search directory and negative selection. Require equal analysis lists
   and exact content except the single text entry. Admission audits the pinned
   selection code and fixture: no embed pattern can select the changed path,
   no config/manifest/SQL/JSON path points to it, no generation or executed source
   can read it, and no package/module/search placement changes. Reject ambiguous
   roles. An embed/config lookup, even of a `.txt` file, disqualifies the edit.
3. Construct a sealed analysis view containing **every other source member**,
   not just files observed in logs. In both views `note.txt` is absent; preserve
   all other paths, directory membership and bytes under the same virtual root.
   Only this view, the provisioned tool/dependency roots and private scratch are
   accessible to the loader, parser, tracker and classifier. The original full
   roots and supervisor descriptors are inaccessible to their descendants.
   A projection is a proposed new internal mechanism, not today's loader behavior.
4. Prove the projection's limited semantic preservation for the admitted fixture:
   the full-tree discovery lists and selected variants equal the view's lists;
   the trusted native selector ignores this ordinary text in the absence of
   embedding or another admitted consumer. Source audit plus real-loader tests
   must support that rule for the pinned versions. Equal outputs alone cannot
   establish it. If package placement, embed, generation, config lookup or an
   unexplained full/view discrepancy appears, refuse eligibility; do not hide it
   by projection. No arbitrary user assertion can mark a file unconsumed.
5. Bind the private candidate to full-tree provenance and the analysis view.
   Retain all ordinary head findings and operations, including warning-free,
   unsupported and global unknown operations. Full-head parity is a required
   acceptance check. The view is not a new test exclusion or a source exemption
   in general review. Any future integration must keep normal full-head gates
   authoritative and reject a projection that changes that population.

This gives two explicit obligations: the selection stage may read the added text
but must only use it for admission/diff; the analysis consumers cannot access it
and receive semantically equivalent selected inputs within the trusted profile.
The second obligation does not follow from missing access logs. If a future
implementation cannot demonstrate the first without broader machinery, stop at
alternative B; do not implement a general document whitelist.

### Filesystem, processes and dynamic areas

Copying and hashing are capture, not write prevention. The supervisor imports
regular files into private storage without reusing host hardlinks, checks content
and canonical paths, then seals the storage before exposing readonly mounts.
Untrusted writers cannot retain a writable alias or descriptor. Reject source
symlinks, multiply linked files, sockets/devices, mount crossings and noncanonical
paths. Runtime-internal symlinks, if needed, must be inventoried, resolve entirely
inside the sealed runtime root and have no writable target. No host bind mount,
external replacement or caller overlay may bypass this domain. Internally created
loader overlays must be exact copies of captured bytes with a bounded bijective
path map; metadata and syntax loads must agree on that map.

One supervisor owns the process tree. Drop capabilities, deny mount/namespace
changes and tracing of other processes, close inherited descriptors, isolate PID
and IPC domains, and expose no host proc/sys, runtime socket or arbitrary devices.
Provide only the explicitly needed virtual process files and null/random devices.
Network isolation must cover every socket family, not only DNS or TCP. Exec is
limited to the provisioned Go/compile/asm/link tools; writable roots are not
executable. The trusted launcher must verify the effective restrictions before
loading and kill/reap all descendants on cancellation or timeout. An isolated
namespace by itself is not the complete access policy. A readonly root alone
neither controls inherited descriptors nor enforces executable selection.

Use a fresh empty build/export cache and fresh scratch for each side; no shared
user cache or prior comparison cache. Exports are produced only by trusted tools
from sealed inputs in that invocation. Record their producer/input identities
and bytes before use; reject an unexplained export, conflicting path or changed
readback. Cache/temp are permitted **derived-input** regions, not arbitrary new
input roots. Only the owned tool process tree can write them. A cache object is
frozen before a consumer reads it; the future adapter must demonstrate that this
lifecycle is compatible with real go/packages, or stop. Do not assert that a
readonly GOCACHE works with unmodified Go. Retain existing captured external
byte/path checks and the narrowly proven MAINT03 test-main exception; do not
normalize new temp-path differences to make a pair pass.

Outputs are write-only toward the supervisor and never fed back as source.
Scratch paths, process IDs, scheduling and randomness may vary for runtime/tool
bookkeeping but cannot authorize comparison or affect admitted semantic inputs.
Do not execute project code that could turn them into SQL/config. The tool/runtime
semantics for their use are trusted and audited; nondeterministic cache/export
or selection discrepancies refuse the witness. Clock affects deadlines; an
expired deadline refuses. The first slice excludes time-sensitive suppression,
baseline and manifest policy. Future support would need an owned evaluation time
and explicit expiry rules, not a claim that the current `time.Now` calls are fixed.
Dynamic areas and their quotas remain part of the private evidence domain.

## Ownership and lifecycle (proposed)

All rows are acquired before analysis, held immutable or exclusively owned during
both loads, checked at each handoff, and validated again before classification.
Pre/post hashes alone cannot detect mutation followed by restoration. Enforcement
must prevent that mutation throughout the interval. A digest means byte equality,
not non-consumption. The input manifest is an internal inventory distinct from
Goquent's schema manifest; it does not regenerate or extend the latter.

| PR5 gap / input | Proposed owner and retained binding | Invalidating change or missing evidence |
| --- | --- | --- |
| Producer, consumer, host, OS/runtime | Provisioner binds kernel/profile and runtime root; supervisor binds actual producer/consumer executable bytes and launch policy before start | Tool replacement, unqualified host/profile or failed policy installation: no managed run; if discovered after admission, input failure |
| Toolchain, standard library, compiler/linker | Sealed runtime inventory covers binaries, library/source bytes, executable selection and any dynamic libraries; native tool exec allowlist | Tag/version alone, changed tool/std bytes, tool outside allowlist or missing tool: refuse; missing required input is input failure |
| Dependencies, export/cache | Provisioner owns exact module archive/extracted content plus sums and selection; supervisor owns fresh cache and exporter provenance through reads | Same module name/version without bytes, stale/shared cache, unknown writer or inconsistent export: no witness; admitted-input corruption is input failure |
| Full source/helpers and analysis snapshot | Supervisor owns complete base/head regular-file snapshots, exact diff, full/view mapping, content and offsets; parser capture binds actual reads | Helper/source change invalidates narrow eligibility; missing capture, TOCTOU or unequal source readback retains input failure |
| Config, env, settings and driver | Supervisor owns complete environment/options and absent configuration; producer uses the same explicit settings in both loads, native driver only | Different env/flags/options, config consumer or custom driver: outside profile/unknown; tampering after admission: input failure |
| Module/workspace/PATH and negative selection | Bounded selection owner records module graph, search directories, all entries and absent candidates inside sealed roots; ancestors terminate at the isolated root | Nearer module/workspace, alternate source or driver candidate, external replace or unenumerated search: refuse; unreadable required selection is input failure |
| Selected variants and unique physical operation | Loader adapter binds package ID/path, ForTest, selected and excluded sets, import graph and env to private operations; classifier counts exact mappings | Partial load, variant conflict, zero/multiple operations at the same physical location or duplicate diagnostic keys: no witness, retain unknown unless existing precedence applies |
| Dynamic region, overlay and external reads | Supervisor owns process lineage, cache/temp lifecycle and allowed reads; loader owns immutable overlay map and export handoffs | Unowned file/descriptor/process, changed overlay, forbidden access, missing provenance or boundary loss: input failure after admission; no log entry is never positive evidence |

Before admission, a supported ordinary comparison lacking narrower evidence stays
unknown where existing precedence permits. Unsupported profile selection may
return an unavailable managed mode; any fallback is a **fresh ordinary run** with
its full gates, never a successful managed witness. Existing malformed diff,
unlisted change, capture/external mismatch and missing required input remain
input failure (exit 2). A partial load/budget exhaustion must preserve unsupported
operations and refuse eligibility; it does not automatically recast all ordinary
loader unknowns as input failures. Failure to install or maintain the promised
managed boundary is a managed execution/input failure, not a valid unknown proof.

## Private evidence and consumer contract

Propose an internal owner/receipt type, not a public JSON certificate or new wire
version. The supervisor and classifier must share a trusted lifetime (or a private
inherited channel with authenticated process identity and a fresh invocation
challenge). Source-controlled JSON, caller hashes, public reports and arbitrary
external attestations are never authority. Cross-machine reusable attestation is
out of scope. Private material stays in protected local storage outside analyzed
roots; publication uses only fixed outcomes/counts.

Bind the evidence to the base/head commit identities **and actual source bytes**,
exact diff, full input inventories and negative entries, analysis views, build
environment, runtime/toolchain/module/export provenance, chosen variants,
producer/consumer revisions and the unique physical operation pair. Commit IDs
alone do not cover dirty files or external inputs. Use canonical length-delimited
records with explicit domains/roles; reject duplicate keys, path collisions and
conflicting digest/byte records. Keep actual byte comparison at trust handoffs;
never deduplicate ambiguous operations by SQL or digest. Hypothetical adversarial
hash collisions are not a new proof assumption replacing these identity checks.

For this pair require identical containing bytes, physical terminal line/byte
column, chain-start offset, operation kind and known SQL/columns/bindings. Verify
coordinates against the capture and one selected variant; reject multiple matching
private operations instead of `collectCI`'s current first match. Require unique
prior diagnostic keys across primary/suppressed lists. Package identity strings
alone and syntax-count variant preference are insufficient.

The receipt is valid only during one synchronous comparison and consumed once.
Check its owner/challenge, all handoff states and complete captures immediately
before classification. Close authority and erase handles on return, error,
cancellation or timeout. Replay, swapped base/head, stale cache, reused source
reference, changed binary, missing input, mutation/restoration or receipt collision
cannot produce unchanged. Preserve `captureValid`, `Session.unchanged` and external
correspondence failures independently; this is not a planner receipt replacement.

A future integration could consult this private relation only at the existing
unchanged-context decision, after existing new/worsened decisions. Keep matched
baseline behavior and its exact whole-tree requirement; the proposed relation
must not make a baseline eligible. Preserve finding retention, full head risk,
precision, manifest and complete operation coverage, regardless of display state.
Unknown policy/runtime axes remain unknown. No production integration is authorized
or present in this PR.

## Smallest implementation unit and acceptance plan

First commission a **test-only vertical slice**: one provisioned profile, one
supervisor, the real loader and one internal evidence adapter, paired literal
fixture and listed text addition. No production classifier branch, public API,
service, generic sandbox framework or dependency update is necessary for that
initial experiment. Reuse existing fixture/diff/observedCI controls and strict
input checks. New work is necessary for enforcement, full/view selection proof,
cache handoffs, selected-variant identity and uniqueness; existing captures cannot
supply these by relabeling them. Qualify any chosen OS launcher dependency in that
separate PR before adoption.

The following are **independent expected results for implementation acceptance,
not tests run by this design PR**. Compare full private reports and operation
populations with ordinary head analysis; do not derive expected results by calling
the proposed eligibility predicate twice. Compile the fixture independently
without executing its function. Controls must reach the real loader and actual
boundary, not a supplied synthetic `closed=true` flag.

| Test/control | Independent expectation |
| --- | --- |
| Identical inputs; then exactly one listed ordinary note addition | One typed literal SELECT, same physical position and known static axes. Real loader through enforced boundary produces one eligible private pair for the note case. Current production still reports note case unknown until a later adopted integration. Full star warning and failed risk/partial-precision gates remain; no policy/runtime proof. |
| Helper/source change despite identical literal terminal | Narrow witness refused; unchanged count for the changed-context candidate is zero; existing new behavior for changed containing bytes remains. |
| Build tag/file addition/deletion, alternate GOOS, dependency/export bytes or module selection | Refuse narrow witness; preserve extra unsupported operations. Corrupt/missing admitted exports are input failure, not unknown success. |
| Env/GOENV/GOWORK/PATH/cache-program change; absent nearer go.mod/go.work or alternative file becomes present | Refuse admission or invalidate witness. A file cannot escape the inventory merely because it was absent on the first read. |
| note embedded, used as config, generated input, relocated into a searched directory, or selected by an overlay | Projection prohibited; unknown retained, or existing input failure when correspondence fails. Never accept a text suffix alone. |
| Host file/socket/network attempt, inherited descriptor, descendant process escape or unauthorized executable | Boundary denies access; supervisor refuses completion as input failure. No candidate witness, even if the operation output happens to match. |
| Mutation and restoration during load, writable alias/hardlink, tool replacement, stale/preseeded cache, swapped pair/replayed receipt | Write/escape blocked or owner invalidated; zero accepted witnesses. Deliberately corrupt handoffs must yield input failure. |
| Two operations at one location, duplicate prior keys, conflicting test variants; moved terminal or second equal SQL | Ambiguous relation refused; moved/new locations keep existing new classification, never merge by SQL equality. |
| Missing source, partial load, parse/file/directory limit, cancellation/deadline, child surviving cancellation | No narrower witness. Existing unsupported coverage remains; input/cleanup failure is separately reported. No truncated successful receipt. |
| Worsened finding, valid matched baseline on identical trees, unknown/warning-free/global operation | Existing new/worsened/matched precedence and retention unchanged. Narrow relation cannot expand baseline matching. Full risk/precision/manifest gates and all operation counts equal ordinary head analysis. |

Go/no-go acceptance requires the one real positive **and every negative class above**, with zero
false unchanged results across adversarial controls and no lost operation/gate
coverage. The test-only predicate is insufficient unless enforcement and real
loader inputs are part of the same lifetime. No-go on any boundary bypass,
full/view parity loss, incomplete inventory or inability to meet budgets. Stop
that slice, record the concrete obstruction and retain B; do not increase limits,
exclude tests or weaken failure expectations to obtain a positive.

### Resource and cost envelope

Keep existing 4,096 files, 2 MiB/file, 64 MiB tree/parsed input, 256 loader
directories and 60-second loader deadline; keep diff limits 1 MiB, 4,096 records
and 10,000 hunks, and existing receipt limits. The two-file literal fixture plus
one text addition is small, but dependency/type loading is not free. Count all
actual parsed inputs and graph entries; fail before exceeding current caps.
The full/view materialization can store one immutable copy of shared bytes and
separate readonly views; admission traversal remains bounded. Do not count an
image as one source file to bypass the parser budget.

Proposed operational caps for the prototype are a preprovisioned runtime/modules
inventory of at most 2 GiB/100,000 entries, 512 MiB scratch/cache per side, 2 GiB
process memory and 64 processes, with a 60-second supervisor preparation deadline
and the existing 60-second deadline per load. These are conservative **planning
caps, not measured requirements or implemented thresholds**; exceeding them is
no-go and needs a separately reviewed scope, not an automatic retry. Provisioning
is offline and separate from comparison. Runtime inventory verification occurs
before use and its sealed identity is retained; source budgets stay unchanged.
A cold export cache may not fit the load deadline. A warm cache would require new
provenance/lifecycle work and is not assumed as a shortcut.

Expected cost is two cold analyses plus bounded snapshot/selection work and a
maintained platform fixture. No timing, memory, image-size or throughput result
has been measured here. The initial limits are feasibility questions for the
prototype. Operating cost also includes toolchain/security refresh, requalifying
profile changes, cache integrity and descendant cleanup tests. Users avoid this
cost by retaining ordinary comparison and unknown results.

## Evidence used and next decision

Source baseline is main `1bcb87dc7a0e8f556246a0c7baf5e7584a3b8e78`, containing PR148
head `c00fb5a321b1b173e440a308088018087e4b628b`. Exact-main
[CI](https://github.com/recoweft/goquent/actions/runs/40503559723),
[Security](https://github.com/recoweft/goquent/actions/runs/40503559668) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40503559669) succeeded before
editing. CI was initially in progress; editing waited for completion.

Source inspection includes [types.go](../orm/review/types.go),
[type_testmain.go](../orm/review/type_testmain.go),
[precision.go](../orm/review/precision.go), [ci.go](../orm/review/ci.go),
[review.go](../orm/review/review.go), pinned x/tools v0.43.0 packages.go/external.go
and installed Go 1.26.10 command/build source. Current packages.Config omits Env;
x/tools inherits the process environment and reads export data outside ParseFile.
Current typedFile does not retain the proposed variant receipt; collectCI takes
the first location match. These are explicit implementation gaps.

The [Go command documentation](https://pkg.go.dev/cmd/go) describes environment,
workspace, toolchain and cache selection; the pinned local source was checked
rather than assuming the currently served documentation describes Go 1.26.10.
[go/packages documentation](https://pkg.go.dev/golang.org/x/tools/go/packages)
distinguishes configuration, overlays and loading; its served version is likewise
not the repository pin. [Linux Landlock documentation](https://docs.kernel.org/userspace-api/landlock.html)
describes inherited restrictions, ABI-dependent rights and special-filesystem
limitations. Landlock is a possible component, not a chosen or verified complete
runner. Its name or kernel version alone cannot attest the boundary above.

This PR changes English design/reference docs only. Local output-boundary and
strict-input checks, documentation links and diff formatting are recorded with
final commit checks in the PR. No new tests, full/race/DB/native suite, census,
manifest regeneration, sandbox experiment or image build is claimed here.
Repository CI/Security are independent exact-head checks, not closure evidence.

The next concrete approval sought **after design review** is a separate test-only
implementation of A's single-profile/single-pair slice, including its launcher
choice, resource envelope and independent acceptance matrix. Before work starts,
reviewers must approve the trusted foundation and restricted compatibility domain.
The deliverable is one actual managed positive, all specified negative classes,
full-gate/coverage parity and zero false unchanged controls, or a bounded no-go
report naming the failed premise. Production integration, guarantee adoption,
unknown promotion, public formats, blocker clearance and release each remain
outside that approval. This worker does not start that next work.
