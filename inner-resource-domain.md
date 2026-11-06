# Prototype inner resource domain and launch contract

GQ-MAINT-15 PR8; Refs [Issue 128](https://github.com/recoweft/goquent/issues/128).
**Design-only handoff: no inner domain has been established in this execution.**
This specifies one missing resource mechanism for the test-only prototype. It
preserves the [managed design](managed-comparison-contract-design.md),
[feasibility acceptance matrix](managed-comparison-feasibility.md),
[SELECT closure gaps](select-context-closure.md) and
[identity limits](select-comparison-identity.md). It does not adopt a production
profile, reduce unknowns or establish a managed comparison witness.

## Current execution evidence

The independently resolved main is `59d9ddece93f7c6552b4d5538865da422de30350`,
also the merged PR150 commit; ancestry passed after fetch. Exact-main
[CI](https://github.com/recoweft/goquent/actions/runs/40511832806),
[Security](https://github.com/recoweft/goquent/actions/runs/41388705951) and
[Docs](https://github.com/recoweft/goquent/actions/runs/40511832936) succeeded.
PR150 is historical work, not recreated here. Cwd/origin, clean initial checkout,
Ubuntu 24.04.5/WSL2, repository instructions and PR template were checked.
Approved GitHub reads confirmed worker identity and repository push permission,
all-state PRs and remote branches; the requested branch/PR were absent.

The current cgroup is a populated domain owned by a transient systemd user
service described as the RelayWeft resource gate. Read-only observations:

| Item | Current observation | Meaning |
| --- | --- | --- |
| `memory.max` | `2147483648` | Outer RAM cap is 2 GiB |
| `pids.max` | `512` | Outer task cap stays 512 |
| `memory.current` | `359927808` at one sample | Usage only; not reserved headroom |
| `pids.current` / `pids.peak` | `87` / `102` at one sample | Current execution observation, not the inherited 79/102 sample |
| `memory.swap.max` | `max` | No no-swap guarantee; ancestor/system constraints not qualified |
| `cgroup.controllers` | `memory pids` | Available here; not delegated to children |
| `cgroup.subtree_control` / child directories | empty / none observed | No prepared child resource domain |
| User unit properties | `Delegate=no`, `MemoryMax=2147483648`, `TasksMax=512`, `KillMode=control-group` | Existing manager enforces the outer envelope |
| Sandbox cgroup mount/access | read-only / not writable | Worker cannot provision through this mount |
| Approved external access diagnostic | directory and selected controls reported writable | Access check only; no write, mkdir, migration or launch permission inferred |

Without explicit runtime-directory/bus selection, the approved user-manager
query failed with `No medium found`. A subsequent bounded read with the existing
local bus selected succeeded. Socket presence alone was not treated as manager
availability. Neither successful query nor writable mode checks authorize
StartTransientUnit, a child delegation or arbitrary cgroup writes. No dedicated
helper or delegated job token was supplied through this execution's interfaces.
No general helper search, unrelated unit inspection or host modification occurred.

The supplied prior-execution profile revision 5, required/linuxdescendants,
start/stop/cleanup verified, application exit 0 and final counters false are
historical handoff claims. This execution has no controller start-record API;
its own profile revision, mode, start-verification and final counters remain
unverified. Current kernel values independently agree with the handed-off outer
2 GiB/512 values; that does not attest the other record fields.

Sandbox GitHub DNS/API access failed; approved targeted API/remote reads and
fetch succeeded. The sandbox auth-status failure was not evidence of invalid
credentials: approved identity/permission reads succeeded. Sandbox branch
creation failed on read-only Git metadata; approved branch creation succeeded.
No approval was denied or remains pending. Windows, SDKs, dependency builds and
GUI capability were not needed or checked. Raw diagnostic material, if retained,
belongs outside checkout/analysis roots with owner-only directory/file modes;
no raw reports, credentials or detailed host identities are uploaded.

## Existing alternative and selection

[Draft PR151](https://github.com/recoweft/goquent/pull/151), head
`a5af58966a1f3025ff6fea21fc53198188ef2528`, already proposes a test resource runner
on another branch. Its source uses systemd-run user services, pre-exec cap
readback and bounded stop. Its user-reported host results are historical, not
this execution's tests. It accepts an arbitrary absolute command and does not
select a job aggregate slice or verify ancestry under the current worker domain.
The default app.slice service is a sibling of this worker service, so its own
2 GiB limit would not preserve the requested shared outer envelope. It is not
imported, rerun, replaced or merged here. This PR adds only the narrower nested
launch contract; PR151's CI/source changes remain on that independent draft.

Choose **manager-provisioned per-job aggregate plus fixed atomic launcher**.
The Linux [cgroup v2 specification](https://docs.kernel.org/admin-guide/cgroup-v2.html)
defines hierarchical controllers, inheritance, the no-internal-process rule,
TID-based PID accounting and populated notifications. The
[clone3 specification](https://man7.org/linux/man-pages/man2/clone.2.html)
provides `CLONE_INTO_CGROUP`: a directory FD selects the child's birth domain,
subject to ordinary placement permission and topology checks. API availability
on this particular host is untested. Fail closed if unavailable; do not silently
fork outside then move the payload.

The cheap fallback is docs-only handoff and ordinary comparison with existing
unknown results. It needs no provisioning, extra process, runner or host rights;
it establishes no inner-domain feasibility. A new user service outside the
aggregate, bubblewrap/unshare alone, RLIMIT_AS, monitoring-only limits and moving
the current worker do not satisfy this resource contract. No image, dependency,
daemon, service configuration or kernel-global change is proposed for this PR.

## Required topology and responsibility

For a separately authorized future execution, the existing resource manager must
create this topology **before starting the worker**, not restructure this live
service:

```text
job aggregate (empty; memory.max=2147483648, pids.max=512)
  worker leaf (worker remains here throughout the execution)
  inner leaf (one fixed prototype job; memory.max=2147483648, pids.max=64)
```

The aggregate has memory/pids enabled for its leaves and contains no processes.
A populated non-root domain cannot distribute the memory controller. Consequently
mkdir under the present populated worker service cannot alone supply this
layout. Existing manager setup must support the aggregate, or implementation
stops before launch. This is a new provisioning prerequisite, not permission to
move this worker, edit app.slice, enable controllers or change its limits now.

One trusted manager-side launch owner is accountable for the whole lifecycle:

| Duty | Owner and narrow operation |
| --- | --- |
| Create and identify | Existing resource manager creates the empty aggregate and two job-owned leaves; binds fresh job/launch nonce to verified cgroup directory identities. Never accept a caller cgroup path. |
| Set limits | Same owner sets aggregate 2 GiB/512 and inner 2 GiB/64 before spawn; reads back values, type, emptiness and ancestry. Worker receives no cgroup write delegation. |
| Atomic launch | Same owner's fixed reviewed launcher uses clone3 with CLONE_INTO_CGROUP and a manager-held inner FD; records child pidfd and PID/start identity. The newborn executes only the fixed probe. No caller executable, argv, shell or environment overrides. |
| Contain descendants | Ordinary children inherit inner membership. Probe receives neither writable cgroup controls nor manager bus/token/FDs; descriptors are closed except bounded private result channels. Escape resistance for hostile code remains outside this benign qualification. |
| Observe | Owner samples both leaves/aggregate, membership of child and grandchild, limits, events and peaks; verifies the worker's membership and outer caps against the pre-run snapshot. |
| Deadline/cancel | Owner uses monotonic deadlines independent of the controller worker deadline; accepts cancellation only for this execution's single opaque launch handle. |
| Kill and reap | Owner stops only inner via its held identity, then uses cgroup.kill if needed, waits on pidfd and reaps adopted descendants as a subreaper. Never kill aggregate, worker, app.slice or another job. |
| Cleanup | Owner observes populated=0 and empty process membership, accounts for every probe descendant, collects final counters, removes only the empty inner leaf and closes handles. A vanished path alone is not a reap proof. |
| Record and consume | Same owner returns bounded fixed status/counters to this execution's test consumer. Consumer rejects missing/mismatched nonce, membership, limits, exit or cleanup evidence; no public receipt confers authority. |

The manager must already have, or separately obtain review for, authority limited
to creating/removing this job topology, setting its four cap values, enabling its
controllers before population and spawning/stopping/reaping its fixed probe.
No general privileged RPC, arbitrary host command, arbitrary PID migration,
arbitrary cgroup path, sudo/root credential, host administration entitlement or
RelayWeft repository edit is requested. A local bus client with general user-unit
control is broader than this contract and is not exposed to the payload.

Development-chat handoff: implement an internal `prepare -> launch -> cancel/finish`
state machine in the manager's existing resource-launch boundary. `prepare`
binds this execution and fixed profile to one manager-created inner identity;
`launch` consumes that capability exactly once, using the fixed probe identity;
`cancel` is idempotent for that handle only. Reject parallel jobs, stale/replayed
handles, changed ancestors, nonempty targets, unverified caps and caller-selected
commands. Never return raw control FDs to the untrusted probe. No network API,
persistent daemon or public Goquent API is needed. This is a reviewable proposed
contract, not implemented functionality or authorization to modify RelayWeft.

If new helper installation, service/delegation changes or elevated host operations
are necessary, first consult with concrete operations, authority, impact and
rollback. The rollback plan is to stop/reap the exact inner job, verify empty,
remove its leaf, and retire the empty aggregate only after its worker exits under
normal manager ownership. Cleanup uncertainty retains identities for owner
inspection and prohibits another run; it must not trigger broad cleanup.

## Cap meanings and bounded test protocol

`pids.max=64` is the first conservative **task/thread** candidate. It includes
probe and Go tool threads; it is not proof of the design's original “64
processes” feasibility. No threshold is raised and no process-count guarantee
is adopted. If meeting a future real-loader goal requires a different meaning
or limit, consult on scope/risk before changing it. No loader is run here.

Inner memory.max=2 GiB is constrained by the shared 2 GiB aggregate, including
worker use. It neither reserves 2 GiB nor equals measured free headroom. OOM at
the ancestor can affect the worker; this benign test must not approach that cap.
Swap remains unchanged; memory.max alone is not a total RAM-plus-swap limit.
The controller's supplied 600000-second worker deadline is not a preparation or
load budget and has not been independently verified in the start record.
The same launch owner must enforce 60 seconds for preparation and, in a future
loader scope, each 60-second load phase; a missing authenticated phase transition
fails closed. None is implemented by a long outer deadline.

Future opt-in qualification uses one fixed probe parent, one child and one
grandchild waiting on a private bounded channel, no project execution. Before
run, verify dedicated empty inner identity, job linkage, topology, both caps,
outer snapshot and unique handle. Use a short 5-second probe deadline and at
most 3 seconds graceful stop plus 3 seconds forced-kill/reap observation. On
expiry or cancel the owner follows the bounded cleanup above. Record normal
exit and cancel cases sequentially only after each cleanup is confirmed.
Do not increase to 64 tasks, attempt fork exhaustion, allocate toward OOM or
perform stress tests. Pre-exec-only guard verification is insufficient to prove
atomic birth; retain launcher source evidence plus actual newborn membership.

Success requires observed child/grandchild membership, exact cap readback,
unchanged worker membership and outer caps, finite cancellation, empty inner,
reaped descendants and final counters. Readback is configuration evidence;
small descendants demonstrate inheritance only. Neither proves enforcement at
the threshold, hostile escape isolation, Go thread feasibility or managed
comparison acceptance. On unconfirmed stop/cleanup, record the exact target
privately and bounded state publicly, declare failure and do not run again.

## Disposition and validation boundaries

**Go for the concrete design handoff; no-go for a live inner probe on the current
contract.** There is no prepared dedicated leaf, Delegate is no, and the current
populated service cannot distribute the memory controller. Writable access in
an approved diagnostic does not fill those ownership/topology premises. No
cgroup/control write, service launch, child/grandchild probe, timeout/cancel or
cleanup test was attempted. Clone3 support/permissions, new aggregate creation,
manager integration, actual inner readback and final counters remain unverified.
The missing operation is explicit job-aggregate provisioning and fixed atomic
spawn by its lifecycle owner, not a generic request to “prepare an environment.”

This commit changes English documentation only. Local link and diff checks
apply; exact final-head CI/Security results belong in the PR/controller report.
Go/focused/race, full/DB/native suites, review census, manifest regeneration and
generated API docs are not run locally for this specification. No tests are
added that merely mirror an unimplemented launcher. No ordinary CI, RunCI,
loader, analysis view, export cache, API, wire or production path changes.

All remaining managed-design acceptance rows, network/all socket family and exec
isolation, full adversarial matrix, real-loader positive, private comparison owner
and operational qualification remain open. Seven blockers, production unknowns,
finite manifest evidence and separate human release judgement remain unchanged.
Even a future resource success could establish only a resource domain. This draft
is not worker readiness, merge/Issue closure, whole-AC acceptance, user completion,
release/tag/package approval or permission to start the next work.
