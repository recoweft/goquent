# Bounded file and analysis process audit

GQ-SEC-03 / planned PR2, Refs #134. This finite Linux audit separates caller-owned
paths, rooted readers, sensitive export and host process execution. It does not
certify absence of all attacks, close Issue 134 or authorize a release.

## Source and prerequisites

Baseline main is `af00cda3a82690d4a4d2a3ead6cfd9b1bc6dc37d`. PR140 head
`f459c0a76558bc835013a8acc3757e96adc7110e` is its ancestor (local merge-base and
GitHub compare); PR140 was read back as merged. Before branch creation or source
changes, exact-main CI **40214936514**, Security **40214936520**, and Docs
**40214936535** succeeded. CI was initially running and was not counted as green.
Docs build and publish both succeeded; gh-pages
`9adc9efca82945584fb5855f980748344fa20855` records deployment of this main SHA.
Pages settings API returned 404 again: branch publication is observed, serving
configuration is unverified. Weekly actual schedule is separate; no dispatch or
schedule wait was performed.

The worker checked requested work/revision, cwd, Ubuntu 24.04.5 / WSL2 kernel,
clean starting tree, HEAD, origin, applicable root AGENT.MD, contracts, PR template,
and prior dependency/workflow/authorization/output/input audits. Initial HEAD was
PR140 head. Sandboxed DNS and gh authentication diagnostics failed; approved
SSH/API diagnostics succeeded as the worker, with repository push permission and
planned-branch push dry-run. No planned remote branch or matching PR in any state
existed at that check. This is not controller authentication evidence. Fetch and
branch creation used approved execution after the observed sandbox restrictions;
actual push/PR and exact-head validation are recorded in the final report.

Contracts: [input budgets](security-input-audit.md), [local exports](redacted-output.md),
[repository checking](typed-update-patches.md), [type loading](static-review-types.md),
[source comparison](static-review-ci.md), [workflow capture](security-workflows.md),
and the [versioned inventory](../specs/goquent_ai_contracts_v3.md).
No production implementation, dependency, workflow, gate, threshold or baseline
changes are introduced. The type-loading document's broad time-bound wording is
corrected to distinguish cancellation from a hard process-tree deadline.

## Finite path and ownership matrix

Owners below are component maintainers; deployment owners control the host and
filesystem. A check at one point is not an immutable snapshot or inode provenance.

| Source / owner | Root, acquisition and use | Existing protection and limit |
| --- | --- | --- |
| CLI JSON/SQL/config/manifest paths; CLI/input owners; `cmd/goquent/{manifest,operation,preflight,migrate}.go`, `internal/inputjson/source.go`, `review/config.go` | Caller explicitly chooses filesystem paths; no checkout confinement. Read bytes then decode/validate at the consuming layer. | PR1 caps only five legacy JSON readers at 8 MiB each. Other existing per-entry budgets remain distinct. Ordinary opens follow symlinks/hardlinks; byte caps do not stop FIFO open/read blocking. SQL/stdin/desired-version/aggregate changes remain outside this PR. |
| Explicit sensitive export; CLI owners; `cmd/goquent/local_output.go` | Caller supplies a new path; parent is resolved before opening. Not confined to repository or a new trusted root. | CI markers refuse; `/dev`, `/proc`, `/sys` resolved parents refuse. O_EXCL prevents replacing existing leaf files, symlinks, hardlinks, FIFO and directories. Opened descriptor is checked regular and owner-only. Write count/error and close error refuse with fixed output. Partial file remains; no atomic publication, overwrite fallback or durability claim. Parent resolution is not protection against all later parent changes or caller publication. |
| Manifest `code_paths`; manifest owners; `manifest.go: fingerprintPaths` | Caller-selected paths, walked and read for fingerprinting; no registry root. | Digest compares supplied bytes, not ownership/authenticity. Ordinary Go-file reads may follow leaf symlinks and aliases; paths and concurrent source ownership remain caller responsibilities. Do not import checker confinement into this API. |
| Registry/config, managed roots, inputs, targets, `code_paths`; manifest owners; `repository_check.go` | Caller chooses registry path. Ancestors checked before OpenRoot(registry directory); subsequent lookup relative to that opened root. Root acquisition itself is a separate trust point. | Canonical local paths/backslash refusal, component Lstat symlink checks, regular-file/size checks, Root.Open, descriptor Stat/SameFile, actual limit+1 read and aggregate budget. Root protects lookup escape after acquisition, including replaced parents. SameFile detects changed inode identity after open, not same-inode content changes, hardlink provenance or blocking before Stat. Directory walk bounds entries/depth. Generator runs in-process and compares bytes, never executes registry commands or writes targets. |
| Repository generation/export; generator + CLI owners; `manifest/skeleton*.go`, `cmd/goquent/manifest.go`, `tests/manifestfixture/generate`, `examples/ai-safe-orm/storedmodel/generate` | Generation builds/formats Go source in memory. CLI output goes through explicit local export. Development fixture helper has its own trusted write workflow. | Generator options are source declarations, not shell commands. `go/format` is not a subprocess. Checked-in generation/test helpers are executable development code; compiler tests explicitly invoke Go. A generated file's header identifies comparison scope, not author authenticity. |
| Ordinary review discovery; review owners; `review.go`, `types.go` | Explicit files or walked directories; module root found by walking ancestors. SQL/Go/source JSON later read separately. | Walk skips designated directories but is not os.Root confinement or global file-budget enforcement. Leaf aliases may be read. Type budgets do not bound all preceding discovery/reads. Explicit files still load surrounding packages/tests and dependencies. |
| CI trees/snapshots/overlay; review owners; `ci.go`, `ci_diff.go`, `types.go` | Caller materializes head/base roots. OpenRoot snapshots are compared before/after analysis, and selected/loader bytes retained in overlays/captures. | Static symlinks/special files refuse; actual read limits and tree budgets apply. Root confines snapshot lookup; overlay does not sandbox Go's filesystem/process access. Concurrent mutation is unsupported. Walk entry inspection and later open are separate: no SameFile recheck in readTree. No deterministic test of every within-call replacement window is claimed. |
| Source report / baseline / diff; review owners; `source_report_json.go`, `ci_baseline.go`, `ReadCIInput` | External reports carry no private execution/correspondence authority. Baseline/diff caller paths can be outside analysis roots. | ReadCIInput Lstat rejects static special files/symlinks and bounds actual bytes, but has no root or opened-file identity recheck. Its check/open window and read blocking are not a race-proof guarantee. Strict baseline paths/version/expiry constrain annotations; full head gates remain authoritative. |
| CI/security capture and artifact; CI owners; `scripts/test-public-output.sh`, `scripts/security/check.py`, `.github/workflows/*` | Raw capture before subprocess start; normal owner-only temporary storage outside analysis roots. Public Docs artifact has a separate trusted main-only build/publish handoff. | Shell umask077/mktemp and security unique directory/exclusive files; fixed classifications, no raw artifact upload. Security process-group timeout is separate from Go analysis cancellation. Capture does not sandbox executed code or prevent all disk consumption; configured temporary storage must be private/outside trees. Docs publishes docs only, not raw diagnostic files. |

Unix mode0600 does not prove Windows ACLs, physical confidentiality, owner account
isolation, absence of privileged readers or safety of a later publication. Hardlinks
can refer to an inode with another name outside a root; root lookup confinement
is not inode-origin validation. Stable special-file refusal does not prove that a
regular-file-to-FIFO replacement cannot block between inspection and open.

## Process and environment trace

`types.go` calls x/tools v0.43.0 `packages.Load` with inherited environment,
module-root cwd, Tests=true, `-mod=readonly`, overlay and a 60-second context.
`type_testmain.go` mirrors driver selection for its narrow correspondence exception;
it does not disable an external driver. x/tools `go/packages/{external,packages,golist}.go`
and `internal/gocommand/invoke.go` were read from the pinned local module source.

* GOPACKAGESDRIVER selects a program; absent/empty searches PATH for
  gopackagesdriver; `off` selects native Go. Driver receives environment, patterns,
  build flags, tests and overlay, runs in the module root, and can have host side
  effects. Its JSON is loader input, not trusted execution evidence. Empty results
  retain unsupported/incomplete review evidence.
* PATH also selects `go` and downstream tools. GOFLAGS and GOENV affect build
  settings; GOWORK and local replace can select inputs beyond the requested tree.
  `-mod=readonly` restricts module-file changes, not network, cache writes, tool
  execution or access to other files. GOTOOLCHAIN, GOPROXY, GOSUMDB and private
  module settings govern acquisition. Go may acquire modules/toolchains and invoke
  build tools; ordinary loading does not invoke `go generate` or execute test bodies.
* Overlay supplies selected file bytes, not an isolated mount or an authorization
  decision. Dependency export data, additional source and workspace/module files
  retain their loader/host boundary. Rooted snapshots do not constrain that loader.
* External driver uses exec.CommandContext, with direct-process kill and no
  configured WaitDelay/group kill. Descendants can outlive cancellation; inherited
  pipes may also delay return. Native gocommand uses interrupt, then a five-second
  grace before direct-process kill, and a 30-second WaitDelay for output plumbing.
  These are source-derived mechanisms, not a measured universal deadline.
* Security capture starts a new session, waits with a 300-second configured
  timeout, then SIGKILLs the process group and waits. Tests exercise its timeout
  with a shorter value; they are not a fresh vulnerability scan. A process that
  deliberately leaves its group is outside the demonstrated coverage.

Application/host owners must authorize opening repositories with their build
environment and executable tools, control credential exposure/network/cache/temp
directories, and use an independently managed execution boundary when inputs are
not trusted at that level. No new runtime approval prompt, environment stripping,
timeout or OS isolation service is introduced. Explicitly configured drivers and
trusted application callbacks are not reclassified as demonstrated vulnerabilities.
DB read-only protects the DB execution contract, not the worker host.

## Measured fixtures and sensitivity

All added OS tests are Linux-only. They use disposable owner-only directories,
fictional contents and no production DB/credentials/external attack target.

| Regression source | Positive, refusal and boundary observations |
| --- | --- |
| `orm/manifest/security_file_linux_test.go` | Normal rooted read; traversal/absolute/noncanonical/backslash drive/UNC refusal; static symlink parent/FIFO/directory refusal; regular hardlink accepted. After acquiring Root, rename original directory and replace its path with an outside symlink: reader still sees original content while ordinary-path negative control sees replacement. This is a deterministic root-acquisition boundary, not probabilistic racing of Lstat/Open. POSIX `C:/name` can be a local filename; Windows drive behavior is not inferred. |
| `orm/review/security_file_linux_test.go` | Caller-selected external absolute CI input succeeds; static symlink/FIFO refused by both input and snapshot readers; hardlink accepted. Diff path traversal/absolute/drive/UNC refused. |
| `cmd/goquent/security_file_linux_test.go` | Export cannot overwrite a hardlink and original bytes remain; explicit symlink parent resolves and creates a new0600 file at the selected destination. Isolated subprocess RLIMIT_FSIZE forces a short regular-file write: fixed refusal and partial0600 file verified. No host-wide limit or disk exhaustion. |
| `orm/review/security_process_linux_test.go` | Explicit and PATH-discovered harmless driver receives expected cwd/environment/overlay/readonly flag. Cancellation returns incomplete; descendant remains able to write a fictional marker afterward, then exits and is reaped. Separate credential-free process, independent supervisor deadline/group cleanup and Linux subreaper prevent orphaned fixtures. Pipes intentionally detached: inherited-pipe delay is source-traced, not exercised. |
| Same process test, native modes | Actual Go1.26.10, GOPACKAGESDRIVER=off, GOPROXY=off: controlled GOENV enables build tag; local replace or GOWORK selects sibling dependency outside analysis root; complete type information obtained. Custom-driver marker remains absent. This is real isolated type loading, distinct from stub results and network acquisition. |
| `scripts/security/test_process_boundary.py` | Credential-free Linux supervisor verifies actual timeout kills a child with SIGKILL and reaps it. Bounded fallback lifetime/cleanup; no real scanner or network. Existing wrapper failure/capture/permission tests reused. |

The root regression also fails against a temporary Go overlay replacing Root.Open
with ordinary path opening (test assertion, not build failure). The actual source
passes. No mutant source is committed. Initial process-fixture failure was a
missing JSON `build_flags` tag in the fixture decoder; correcting that fixture
made the probe pass. It was not a product defect or an unexplained retry.

Close-error injection for real regular export files, same-inode concurrent writes,
every Lstat/Open race, hardlink provenance enforcement, ACLs, Windows/macOS,
network/toolchain download and credentialed deployments remain unverified. Existing
code checks close errors; this audit does not manufacture a successful OS close-error
experiment. No new arbitrary-path loader sandbox or special-file rejection policy.

## Validation and finite disposition

Actual local Go: **1.26.10 linux/amd64**, explicitly selected, GOTOOLCHAIN=local,
GOPROXY=off, writable temporary GOCACHE. Raw command output is outside repository
and analysis roots under an owner-only directory, never uploaded. Relevant command
exits, skips, final commit, draft PR and exact-head CI/Security are in the final
PR/controller report. Native subprocess fixtures use explicit sanitized environments;
the normal repository test suite retains its established execution environment.

| Command under the selected toolchain/capture environment | Actual local result |
| --- | --- |
| `go test ./orm/manifest ./orm/review ./cmd/goquent -run '^TestSecurity(File\|Process)' -count=1 -timeout=90s` | Exit0 after correcting the fixture JSON tag. |
| `go test ./cmd/goquent -run '^TestSecurityFile' -count=1 -timeout=30s` | Exit0 including subsequently added short-write fixture. |
| `go test -race ./orm/manifest ./orm/review ./cmd/goquent -run '^Test(SecurityFile\|SecurityProcess\|ExplicitLocalArtifact\|RepositoryCheck\|CISourceBoundaries)' -count=1 -timeout=120s` | Exit0. |
| `go test ./orm/review -run '^TestSecurityProcessDriverBoundary/native_' -count=1 -timeout=45s`; same with `-race` | Both exit0; these native cases were added after the broad focused/race and integration run began. |
| `go test -race ./orm/review -run '^TestSecurityProcess' -count=1 -timeout=60s` | Exit0 after final fixture-only change to publish complete handshake markers by rename and clear reaped child identity. |
| Temporary `-overlay` mutant, manifest root regression only | Exit1 at opened-root identity assertion; successful build. Real implementation passes. |
| `make test-integration` | Exit0, explicit local MySQL8/PostgreSQL16 fixture DSNs, `go test ./... -count=1`, 26 passing packages. Non-verbose output does not enumerate individual skips. Later native additions tested separately above and by final-head CI. |
| `go vet ./...`; `sh scripts/test-public-output-check.sh`; `python3 scripts/release_validation/check_strict_inputs.py` | All exit0. |
| `python3 -B -m unittest discover -s scripts/security -p 'test_*.py'`; corresponding `scripts/release_validation` discovery | Both exit0; harmless wrapper/orchestration fixtures, not release acceptance or fresh vulnerability DB scans. |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit0; comparison, no regeneration/write. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit1; 1510 findings, suppressed list0, truncated public display. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit1; schema/policy ok, generated_code stale, database skipped. |
| `git diff --check` | Exit0. |

Docker socket access failed in the sandbox; approved diagnostic found healthy
fixture services, followed by a separately approved integration run. Connectivity
alone is not test success. No approval was denied. No Windows process, network
acquisition experiment, fresh native census or schedule dispatch was performed.

| Classification | Owner and concrete disposition / exit condition |
| --- | --- |
| Actual defect | No supported-contract runtime defect reproduced in the finite stable-input cases. Documentation overstated process timing; wording corrected and cancellation/descendant behavior measured. Future concrete defect requires its own supported-contract reproduction and sensitive regression. |
| Missing evidence | PR maintainer records final exact-head checks and actual integration/skip results. Platform owners need actual Windows ACL/process tests before making Windows claims. CI owners retain Pages settings and first actual weekly schedule checks. |
| Guarantee limit | Host/application owners authorize process environment/network and maintain private stable files/roots. No all-race, all-descendant, inode provenance, atomic export or physical confidentiality guarantee. Stronger controls require explicit entry-specific compatibility design and finite validation. |
| Historical cause unknown | Retain Go1.26.10/local, scanner1.9.0, x/mod module-only2, historical graph-only15, prior SQL review1510/exit1, manifest stale/database skipped, nativeunknown and existing release blockers. These tests do not establish their original causes or clear them. |
| Candidate judgement | No new root restriction, environment removal, execution permission API, timeout, dependency or platform guarantee proposed for adoption here. Any future change belongs to the affected CLI/review/host owner with old/new behavior, migration and test criteria. SQL/stdin/desired-version/aggregate limits and REF01 remain separate. |

Finite exit is this inventory, bounded harmless regressions, required checks and
one draft PR with exact-head CI/Security evidence. Implementation, verification,
integration/review and release approval are separate states. No ready/merge,
Issue close, overall acceptance, release/tag/package publication or next work.
