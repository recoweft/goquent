# Bounded workflow security — GQ-SEC-01 / planned PR2

This change addresses [Issue 132](https://github.com/recoweft/goquent/issues/132)
workflow permissions, action provenance, bootstrap selection and continuous
source vulnerability checks. It does not approve a release or close the Issue.
Runtime APIs, SQL, language minimum, dependency versions, release runner,
BLOCKERS, thresholds, baselines and existing unknowns are unchanged.

## Events, source and trust boundaries

Every workflow defaults to `permissions: {}`. All checkouts disable credential
persistence. There is no `pull_request_target`, privileged workflow-run handoff,
PR title/body interpolation in shell commands, or dependency update bot.

| Job | Event and source | Permission and credentials | Cache / outputs |
| --- | --- | --- | --- |
| CI / test | PR exact head SHA; push to main exact event SHA | contents:read; no repository secrets; disposable local DB fixture credentials only | setup-go cache disabled; existing fixed test-output capture; no raw artifact |
| Security / vulnerability | PR exact head SHA; main push SHA; Monday 04:23 UTC schedule on default branch; manual dispatch selected ref | contents:read; no repository secrets; separate runner from CI and Docs | setup-go cache disabled; unique per-run scanner/module/build directories; fixed metadata/counts only; no artifact |
| Docs / build | main push SHA only | contents:read; no publish token exposed to build commands | no Go cache restore/save; one public-docs artifact containing docs/ only, retained one day |
| Docs / publish | same main-push run, requires successful build | contents:write only here; GITHUB_TOKEN supplied only to the fixed publisher action | fresh runner, no checkout or execution of generated files; downloads named artifact from this run only |

The publisher clones its destination branch itself; it does not need a source
checkout. The artifact handoff does not select another run or accept PR artifacts.
`actions:write` and `pages:write` are unnecessary for this existing gh-pages branch
publisher. Artifact upload/download within the same run use the runner artifact
service token, not extra repository token permissions. GitHub still controls
fork-PR approval policy and the runner image; those organization settings are
unchanged. Configuration review is not an executed fork-PR test.

No cache key or restore prefix crosses jobs or trust domains because workflow Go
caching is disabled. The hosted setup-go tool cache and upstream distribution
services remain acquisition dependencies. SHA pinning does not audit every
transitive dependency or guarantee uncompromised upstream infrastructure.

## Action and tool provenance

On 2026-10-25 the worker read official GitHub repository tag refs, matching release
tags and action.yml at these commits. The peaceiris tags are annotated: both v4
and v4.1.0 were dereferenced to the same commit. Full commit IDs, not tag-object
IDs, are used in `uses`.

| Action | Release | Full commit | Declared runtime |
| --- | --- | --- | --- |
| actions/checkout | v4.4.0 | 11d5960a326750d5838078e36cf38b85af677262 | node20 |
| actions/setup-go | v5.6.0 | 40f1582b2485089dde7abd97c1529aa768e1baff | node20 |
| actions/upload-artifact | v4.6.2 | ea165f8d65b6e75b540449e92b4886f43607fa02 | node20 |
| actions/download-artifact | v4.3.0 | d3f86a106a0bac45b974a628896c90dbdf5c8093 | node20 |
| peaceiris/actions-gh-pages | v4.1.0 | 84c30a85c19949d7eee79c4ff27748b70285e453 | node24 |

Verification can be repeated with `gh api repos/OWNER/REPO/git/ref/tags/VERSION`,
following `git/tags/OBJECT_SHA` for annotated tags, and reading
`repos/OWNER/REPO/contents/action.yml?ref=COMMIT_SHA`. These are the official
[checkout](https://github.com/actions/checkout),
[setup-go](https://github.com/actions/setup-go),
[upload-artifact](https://github.com/actions/upload-artifact),
[download-artifact](https://github.com/actions/download-artifact), and
[publisher](https://github.com/peaceiris/actions-gh-pages) repositories.

Existing checkout/setup-go majors are retained. The prior-main Docs run
40201521569 succeeded but warned that node20 actions were forced onto node24.
Do not suppress that warning or force an obsolete runtime. Exact-head CI tests
checkout/setup-go compatibility; the new Docs artifact handoff remains a
post-merge validation item. Runner behavior changes require a new evaluation,
not an automatic major upgrade. Newly added artifact actions use the v4 protocol.

All workflows explicitly install Go **1.26.10**, with `GOTOOLCHAIN=local`, before
running project commands. This matches go.mod's preferred toolchain. It avoids
using the older Go bootstrap for a subsequent automatic toolchain download.
The security runner checks actual Go, builds the scanner with that Go outside the
project module, and verifies scanner build metadata. The Go **1.25.0** declaration
remains a language/API minimum; it does not force downstream consumers to use a
safe toolchain. Older bootstraps and acquisition environments are not certified.

The official scanner is `golang.org/x/vuln/cmd/govulncheck@v1.9.0`.
The runner uses `https://proxy.golang.org`, `sum.golang.org`, no private-module
exceptions, and a new module cache. The checksum database validates retrieved
modules; scanner and application dependency graphs remain separate. Docs use
`github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0`, matching its official latest
release observed that day, instead of resolving `@latest` on each publish.

## Scan contract and publication boundary

Run from the repository root with the patched Go first in PATH:

```sh
GOTOOLCHAIN=local python3 scripts/security/check.py
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/security -p 'test_*.py'
```

The runner executes the following against `https://vuln.go.dev`:

```text
govulncheck -json -db https://vuln.go.dev ./...
govulncheck -json -db https://vuln.go.dev -test ./...
govulncheck -mode convert -format text   # each captured JSON stream on stdin
```

Normal scanning represents ordinary source entry points; test-inclusive scanning
also covers sqlmock and test-only standard-library calls. Neither executes tests
or connects to a database. Scope is Linux/amd64, GOAMD64=v1, CGO_ENABLED=1, default
tags, empty GOFLAGS, GOWORK=off and GOPACKAGESDRIVER=off. Other configurations and
downstream applications are unverified. Ordinary CI retains its database suite.

Govulncheck JSON output alone does **not** fail on detected vulnerabilities.
Therefore the official scanner's text conversion supplies the symbol-reachability
gate: exit 0 or exit 3. The wrapper validates complete JSON/configuration/SBOM,
requires a DB modified timestamp, counts distinct advisories at their highest
reported level, and checks the official gate against the symbol count. Raw
process exits are logged separately. Unknown/malformed data, missing DB evidence,
installation/scan/conversion errors, capture failure and timeouts fail the job;
none are mapped to clean. Each subprocess has a 300-second limit; the workflow
has a 25-minute limit. A timeout kills the subprocess group. No ignore list or
continue-on-error is used.

Public output includes only validated source SHA and dirty flag, fixed Go/tool/
platform settings, UTC start time, validated DB-modified timestamp, numeric exits
and fixed symbol/package-only/module-only counts. It never republishes source
paths, vulnerability traces, SQL, DSNs, host paths, arbitrary exception messages
or tokens. Raw streams are captured before spawning into a new owner-only
`/tmp/goquent-security-*` directory, with owner-only files. They are not uploaded.
On hosted CI they expire with the runner; locally the operator may inspect them
in an appropriately restricted environment and remove them after triage. A
rerun against the live DB is not necessarily the same DB snapshot.

A successful symbol gate does **not** mean no vulnerable module is present.
The prior audit's two x/mod module-only advisories remain, and its 15 graph-only
advisories in unloaded dependencies remain historical open inventory, not repaired
findings. Source scanning does not claim full selected-graph or toolchain-binary
coverage. See [dependency audit](security-dependency-audit.md) for their IDs,
versions, entry-point analysis and limitations. Reassess that inventory when
imports, modules, Go, platform/tags or DB evidence change. Static reachability
also has reflection/interface/runtime-loading limitations.

## Maintenance and finite failure response

1. A maintainer reviews a red security job's fixed classification. For execution,
   DB or timeout errors, restore the failed prerequisite and rerun on the same
   source; do not waive the job as clean. Raw evidence is inspected only through
   an authorized restricted channel or a local reproduction.
2. For a detection, evaluate reachable paths, affected versions and the smallest
   compatible repair. Retain module-only inventory even when the symbol gate
   passes. Breaking changes or unavoidable exceptions require a concrete review
   decision; never add a blanket ignore. Do not publish exploit details.
3. For Go updates, edit the preferred toolchain, all setup-go versions and the
   security runner's expected version together. Preserve the language minimum
   unless a separate compatibility change is approved. Rebuild the scanner with
   the patched Go and rerun both source modes and relevant regressions.
4. For action/scanner/DB changes, check official release-to-SHA correspondence,
   annotated-tag dereference, runtime compatibility and permissions. Recheck the
   matrix, boundary tests, actual scan and exact-head CI. Module or DB changes
   can change a previously green result; the weekly default-branch run exists to
   detect that without requiring a source change.
5. After a maintainer merges this PR, verify the exact merge SHA's CI, Security
   and Build & Publish Docs runs. Docs build and publish must both succeed and
   the destination gh-pages update must correspond to that source. Verify the
   first scheduled Security run (Monday 04:23 UTC), or explicitly dispatch main
   and then separately observe the scheduled event. Dispatch is not schedule
   execution evidence. Neither is performed by merging this branch automatically.

Implementation, verification, integration and release approval are independent.
The PR/controller report carries actual head identities and executed results.
Maintainers own post-merge Docs/schedule checks and integration decisions;
release owners retain existing SQL-review failures, stale generated-code and
skipped database evidence, historical unknowns and release blockers. Green
workflow checks do not clear any of those gates.
