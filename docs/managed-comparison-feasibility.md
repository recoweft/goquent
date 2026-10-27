# Managed comparison: bounded feasibility result

GQ-MAINT-15 PR7; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
**No-go on the inspected worker profile, before managed admission.** No managed
comparison was run and no private eligible pair was established. This is the
bounded stopping result of the separately authorized test-only experiment in the
[managed comparison design](managed-comparison-contract-design.md), not the
previously anticipated production stage three. Production retains whole-tree
comparison and the ordinary note-addition unknown outcome; it is not remeasured.

## First unmet premise

The worker has no demonstrated, authorized mechanism for imposing the candidate
**2 GiB memory and 64-process limits on the owned process domain**. The sandbox
exposes cgroup v2 read-only. A separately approved, non-mutating diagnostic
outside that sandbox found the host cgroup mount writable, but the worker's
current cgroup directory and control files not writable by the worker.
`memory.max` and `pids.max` both read `max`; `cgroup.subtree_control` was empty.
`cgroup.procs` and `cgroup.kill` were also not writable. The diagnostic exited 0:
these are observed access results, not a failed host launch.

This is a missing provisioning premise for this attempt, not proof that Linux,
WSL or every possible supervisor cannot enforce the limits. No delegated
alternative domain was established. Per-process address-space limits or
monitoring after allocation are not demonstrated substitutes for the required
process-domain envelope. User/cgroup namespaces alone do not establish resource
delegation. Observed usage after an unbounded run would not establish a cap.

The authorized stop-on-missing-boundary rule applies. No launcher was selected
or adopted, no isolation runtime was launched, and no supervisor, private owner
or loader adapter was implemented. New delegation, a privileged launcher,
service or different provisioned host would change permissions or operating
assumptions and require a concrete specification consultation before use. None
was requested or installed as a workaround. No tool approval remains pending;
no approval was denied.

## Environment and candidate mechanisms

Actual environment: Ubuntu 24.04.5, Linux
`6.18.54.1-microsoft-standard-WSL2`, x86_64. This WSL environment is **not a
qualified profile**. Read-only inventory found bubblewrap 0.9.0 and util-linux
unshare 2.39.3. The bounded capability probe
`timeout 5 unshare --user --map-root-user true` exited 0 in the sandbox. It created
only a transient namespace for `true`, not a fixture or managed loader. The
optional `unprivileged_userns_clone` sysctl was absent; this was not interpreted
as disabled user namespaces.

Bubblewrap help exposes readonly mounts, PID/IPC/network namespaces, capability
dropping, explicit environment, seccomp input, sized tmpfs and parent-death
handling. These are candidate building blocks, not an installed complete policy.
No bubblewrap policy was executed. Docker and systemd-run command names were
present, but no daemon, image, socket, service or Windows program was accessed.
No image pull/build/start, package installation, dependency update, host/kernel
setting or daemon configuration change occurred.

| Required boundary | Evidence in this attempt | Qualification |
| --- | --- | --- |
| Memory/process envelope and descendant accounting | Sandbox cgroup read-only; approved host check found no current-domain write permission and unlimited memory/pids values | First unmet premise; no managed run |
| Immutable full trees, analysis views, readonly runtime/modules and no writable aliases | Candidate readonly-mount interface found; no sealed inventory or projection created | Not verified |
| Descriptor closure, PID/IPC ownership and descendant cleanup | Candidate interfaces found; only minimal user namespace probe executed | Not verified; no cleanup adversary run |
| All socket families/network, restricted exec, tracing/mount denial | Candidate seccomp interface found; no filter or executable allowlist installed | Not verified; network namespace alone is insufficient |
| Explicit environment, PATH/module/workspace and negative entries | Design read; no managed environment or complete selection inventory acquired | Not verified |
| Fresh export/cache provenance and freeze-before-read | Current loader and pinned x/tools source inspected; no export preparation/consumer handoff attempted | Not verified; readonly GOCACHE compatibility not assumed |

The trusted foundation remains the design's provisioner, kernel, isolation
runtime, reviewed pinned Go/Goquent and projection code. Their names/version
strings are neither byte identity nor enforcement evidence. This no-go produces
no admitted combination, profile revision, executable hash inventory or reusable
attestation. Host compromise remains outside the proposed claim.

The selected Go tool reports `go1.26.10 linux/amd64`; the ambient installation
with `GOTOOLCHAIN=local` reports Go 1.25.0. Ordinary local validation used the
already cached Go 1.26.10 executable explicitly, local toolchain selection,
offline modules, workspace disabled and readonly module flags. This does not
qualify its bytes or the inherited validation environment for managed execution.
Current `packages.Config` has no explicit Env; pinned x/tools v0.43.0 defaults to
the process environment and opens export files outside ParseFile. These known
lifecycle gaps were not relabeled as closure evidence.

## Independent acceptance matrix

Every row below is **not run through a real managed boundary and real loader**.
There are zero managed comparisons and zero established managed positive
witnesses. No false-acceptance rate was measured: zero runs is not successful
adversarial coverage. Historical fixture/model results are not substituted.

| Independent class | Result and remaining obligation |
| --- | --- |
| Identical control and one listed note.txt addition to literal Table/Select/Where/Get | Not run; no eligible private pair, independent compile, selector/full-view preservation or full-head parity proof |
| Related source/helper/build/file selection/dependency/export change | Not run; no real-boundary rejection result |
| Environment, workspace/module/PATH/cache-program and negative-file transitions | Not run; negative selection coverage not measured |
| Embed/config/generated/overlay consumption of text | Not run; no projection admitted |
| External file, socket/network, inherited descriptor, unauthorized exec and descendant escape | Not run; no installed boundary to attack |
| Mutation/restoration, hardlink/alias, tool replacement, stale cache, replay and swapped pair | Not run; no private owner lifetime established |
| Duplicate physical operation/prior key, conflicting variant, moved or second equal operation | Not run; no new variant/unique-operation adapter |
| Missing/partial input, parse/file/directory budget, deadline/cancel and cleanup failure | Not run; no managed supervisor or successful cleanup claim |
| Existing new/worsened/matched precedence, whole-context baseline and full head risk/precision/manifest/coverage | No production change; managed parity not run. Warning-free, unsupported and global operations are not excluded |

No synthetic closed flag, matching SQL/counts, absent access logs or pre/post
hashes are offered as non-consumption or mutation-prevention evidence. The
[MAINT03 exception](static-review-ci.md),
[MAINT11 diagnosis](review-triage.md#gq-maint-11-private-comparison-branch-census),
[MAINT12 finite model](document-context-proof.md),
[SELECT identity](select-comparison-identity.md) and
[closure limits](select-context-closure.md) remain distinct and unchanged.

## Budgets and cost

Candidate limits remain acceptance conditions: runtime/modules 2 GiB and 100,000
entries; scratch/cache 512 MiB per side; memory 2 GiB; 64 processes; preparation
60 seconds. Existing source limits remain 4,096 files, 2 MiB/file, 64 MiB
tree/parsed input, 256 loader directories and 60 seconds/load; diff limits remain
1 MiB, 4,096 records and 10,000 hunks, with existing receipt limits. No limit was
increased and no warm cache was substituted for a cold managed run.

Managed peak memory, preparation/load time, runtime/module/tool inventory bytes
and entries, scratch/cache size, export bytes and negative lookup coverage are
**unmeasured**. Capability probes and ordinary validation costs do not estimate
the two cold analyses. The no-go avoids that cost without the required boundary.
Repetition with unchanged permissions cannot establish the missing delegation
or other untested boundaries.

## Source and validation record

Source base is independently resolved main
`36553f1c38df3cc92fa795c14439cde9942ab0c4`, containing merged
[PR149](https://github.com/recoweft/goquent/pull/149) head
`54a6ac002857dbad8ecccd2dcd9317dd6f4dd0af`. Local ancestry checks passed after
fetch. Exact-main [CI](https://github.com/recoweft/goquent/actions/runs/40507206111),
[Security](https://github.com/recoweft/goquent/actions/runs/40507206100) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40507205794) succeeded
before repository editing; CI was initially pending and editing waited.
Cwd/origin/OS/clean state, repository instructions/contracts/template, worker API
identity/push permission and local/remote/all-state PR duplication were checked.
The experiment branch and matching PR were absent before creation.

Sandbox GitHub access failed; approved targeted API/remote reads succeeded.
Sandbox fetch failed on readonly Git metadata; approved fetch succeeded. The
initial local ancestry check lacked the merge object, then passed after fetch;
it was not treated as an ancestry mismatch. Approved cgroup inspection succeeded
with the missing permissions described above. No approval was denied.

Changes are English documentation only. Public-output wrapper checks and the
strict-input regression checker passed (exit 0); the latter retains eight
ordinary CLI cases with expected gate failures, not managed acceptance tests.
Local link/format validation and exact final-head CI/Security belong in the PR
record. Local full/focused/race Go suites, DB/native suites, whole-tree review or
census, manifest regeneration and generated API docs are not rerun for this
documentation-only stopping result. Unchanged repository CI is separate evidence.
Protected command outcomes/logs stay outside the checkout and analysis roots,
with owner-only directory/file permissions; raw artifacts are not uploaded.

## Disposition

Retain design alternative B in production. This attempt stops with the first
missing premise and finite unverified list above; it does not start an open-ended
redesign or provisioning task. A future attempt would first need an explicitly
reviewed concrete launcher/host combination with enforceable process-domain
budgets and cleanup, followed by every remaining boundary, cache-handoff and
acceptance test. Existing binaries alone do not satisfy that entry condition;
provisioning alone would not satisfy the acceptance matrix.

No production integration, unknown promotion, public format, threshold,
suppression, baseline, test exclusion, runtime authorization, DB identity,
privacy guarantee or release-blocker disposition changes. Seven blockers and
separate human release judgement remain. Draft PR/check success does not mean
managed feasibility, worker readiness, merge approval, whole-Issue acceptance,
user completion or permission to start another work item.
