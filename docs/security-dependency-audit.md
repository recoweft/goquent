# Dependency vulnerability audit — GQ-SEC-01 / planned PR1

This is a bounded audit for [Issue 132](https://github.com/recoweft/goquent/issues/132),
not release approval, a universal safety guarantee, or completion of the Issue.
The repair changes the preferred Go toolchain from **1.26.4 to 1.26.10** within
the same release series. Dependency module versions and the Go 1.25.0 language
minimum remain unchanged. The toolchain directive is a preference for this main
module; consumers and environments overriding GOTOOLCHAIN must independently
select a patched toolchain. The older CI bootstrap is not certified by this audit;
workflow hardening and bootstrap policy remain planned PR2.

## Inputs and environment

- Source baseline: `8ac8145eb1e01847d0b61e4c935fa78bf4caef93` (remote main).
  Post-repair source is this PR: only go.mod, the toolchain instructions,
  the bounded ASN.1 regression and this document differ. Final commit identity
  and exact-head CI are recorded in the PR/controller report, avoiding a
  self-referential commit hash here.
- Ubuntu 24.04.5 under WSL2, Linux amd64, GOAMD64=v1, CGO_ENABLED=1,
  GOTOOLCHAIN=auto, empty GOFLAGS and GOWORK, no additional tags.
  Actual project Go before/after: `go1.26.4` / `go1.26.10`.
  No repository go:build directives, workspace, replace directives or vendor tree
  were found. Other platforms, architectures, custom tags, custom package drivers
  and downstream applications are unverified.
- Baseline go.mod SHA256: `de5957c0115f13d0e9ac1624d49cad8f21880d1b2f179c2921c1e541111e8ff8`.
  go.sum SHA256 (unchanged): `bdc51909709ac29db6f760a27a7bf5e1c01af057c78bb16818490b9cce66a171`.
- Scanner: official `golang.org/x/vuln/cmd/govulncheck@v1.9.0`, installed
  outside the module into `/tmp/gq-sec-01/bin`. Binary built by Go 1.26.10;
  this is distinct from the target Go version printed by the scanner.
  Module checksum: `h1:12VaqnI9kJfqwRoPptxoqsUhQtdfrSbdksT7X/KY/sc=`.
- DB: https://vuln.go.dev; `index/db.json` reported modified
  `2026-10-21T00:27:29Z`. Initial source scans ran on 2026-10-25 UTC,
  13:54:31–13:54:36 (normal), 13:54:53–13:54:58 (tests).
  Post-toolchain scans ran 13:55:48–13:55:56. Go binary scans ran
  13:56:57–13:57:02. Official advisory/toolchain index checks followed that day.
  The live DB was not frozen; future DB/tool updates can change results.

## Dependency inventory and entry points

`go mod graph`, `go list -m all`, and `go list -deps -test ./...` were checked.
The selected graph includes more modules than the packages actually loaded.

| Module | Selected version | Role in this build |
| --- | --- | --- |
| github.com/go-sql-driver/mysql | v1.9.3 | Direct runtime driver, connector and error handling |
| github.com/lib/pq | v1.10.9 | Direct runtime PostgreSQL registration and error handling |
| golang.org/x/tools | v0.43.0 | Direct runtime type analysis via go/packages |
| github.com/DATA-DOG/go-sqlmock | v1.5.0 | Direct declaration, test-only imports |
| filippo.io/edwards25519 | v1.1.1 | Indirect MySQL authentication dependency |
| golang.org/x/mod | v0.34.0 | Indirect type-loading dependency; sumdb packages not imported |
| golang.org/x/sync | v0.20.0 | Indirect type-loading dependency |
| github.com/google/go-cmp | v0.6.0 | Selected graph only, not loaded by this build |
| github.com/yuin/goldmark | v1.4.13 | Selected graph only |
| golang.org/x/net | v0.52.0 | Selected graph only |
| golang.org/x/sys | v0.42.0 | Selected graph only |
| golang.org/x/telemetry | v0.0.0-20260311193753-579e4da9a98c | Selected graph only |

Normal source analysis covered 57 root packages, including `orm`, `cmd/goquent`,
`orm/mcp`, `orm/review`, examples and generation helpers. It loaded six external
modules plus the main module and standard library. Test analysis also loaded
sqlmock. A module in the selected graph is not proof of runtime exposure; modules
not loaded above were not given a source reachability claim.

Driver use is in `orm/driver/driver.go`, command-store/migration and query paths.
`orm/review/types.go` and `type_testmain.go` use go/packages; CLI/MCP reach that
analysis through the review APIs. Static call paths through interfaces can be
conservative. In particular a scanner trace from MCP io.Reader to TLS does not
prove the stdio MCP server is a network TLS endpoint. MySQL/PostgreSQL TLS paths
are relevant depending on application connection configuration. No live hostile
server, certificate compromise, filesystem race or application exploit was
reproduced. P1 is the work priority, not a measured CVE severity.

## Findings and bounded disposition

Before repair, both normal and test scans completed with exit **3**, reporting
7 symbol-level standard-library findings, 2 package-only findings and 17
module-only findings. There was no additional test-only advisory. After repair,
both completed with exit **0** and no symbol/package findings; **two x/mod
module-only advisories remain**. Success does not mean the dependency inventory
contains no known vulnerability.

The complete initial source advisory set is retained below. Levels are scanner
classifications, not exploitability conclusions.

| Advisory | Initial level | Official summary |
| --- | --- | --- |
| [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) | Symbol | Distrust compromised intermediate certificates in crypto/x509 |
| [GO-2026-6607](https://pkg.go.dev/vuln/GO-2026-6607) | Symbol | Reject malformed ECH outer extension references in crypto/tls |
| [GO-2026-6604](https://pkg.go.dev/vuln/GO-2026-6604) | Symbol | Root.Mkdir(All) can follow junctions out of the root on Windows in os |
| [GO-2026-6090](https://pkg.go.dev/vuln/GO-2026-6090) | Symbol | Limit handshake messages we are willing to accept post-handshake in crypto/tls |
| [GO-2026-5972](https://pkg.go.dev/vuln/GO-2026-5972) | Symbol | Enforce maximum recursion depth in encoding/asn1 |
| [GO-2026-5856](https://pkg.go.dev/vuln/GO-2026-5856) | Symbol | Invoking Encrypted Client Hello privacy leak in crypto/tls |
| [GO-2026-4970](https://pkg.go.dev/vuln/GO-2026-4970) | Symbol | Root escape via symlink plus trailing slash in os |
| [GO-2026-6218](https://pkg.go.dev/vuln/GO-2026-6218) | Package | Avoid quadratic complexity in resolvePath in net/url |
| [GO-2026-5942](https://pkg.go.dev/vuln/GO-2026-5942) | Package | Parsing an invalid SVCB or HTTPS RR can panic in golang.org/x/net/dns/dnsmessage |
| [GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617) | Module | HTTP/2 server crash due to HPACK encoder race in net/http |
| [GO-2026-6613](https://pkg.go.dev/vuln/GO-2026-6613) | Module | HTTP/1 server connection desynchronization after 2xx CONNECT response in net/http |
| [GO-2026-6612](https://pkg.go.dev/vuln/GO-2026-6612) | Module | Double flow control refund on HTTP/2 server streams in net/http |
| [GO-2026-6611](https://pkg.go.dev/vuln/GO-2026-6611) | Module | Excessive CPU consumption from repeated initial window changes in net/http |
| [GO-2026-6610](https://pkg.go.dev/vuln/GO-2026-6610) | Module | HTTP/2 transport accepts malformed framing-related headers in net/http |
| [GO-2026-6609](https://pkg.go.dev/vuln/GO-2026-6609) | Module | Lack of limit on size of parsed Range headers in net/http |
| [GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608) | Module | Memory limit bypass when parsing MIME headers in net/textproto, mime/multipart |
| [GO-2026-6605](https://pkg.go.dev/vuln/GO-2026-6605) | Module | HTTP/1 client connection desynchronization after CONNECT rejection in net/http |
| [GO-2026-6603](https://pkg.go.dev/vuln/GO-2026-6603) | Module | HTTP/2 server memory exhaustion due to Trailer headers in net/http |
| [GO-2026-6600](https://pkg.go.dev/vuln/GO-2026-6600) | Module | Recognize yield as regexp preceder keyword in html/template |
| [GO-2026-6599](https://pkg.go.dev/vuln/GO-2026-6599) | Module | Reset context tracking on consecutive template expressions in html/template |
| [GO-2026-6180](https://pkg.go.dev/vuln/GO-2026-6180) | Module | Ignore unrelated, unauthenticated hashes in Lookup in golang.org/x/mod/sumdb |
| [GO-2026-6179](https://pkg.go.dev/vuln/GO-2026-6179) | Module | Fix transparency log tile verification bypass in golang.org/x/mod/sumdb/tlog |
| [GO-2026-6091](https://pkg.go.dev/vuln/GO-2026-6091) | Module | Fix Javascript regexp context tracking in html/template |
| [GO-2026-6089](https://pkg.go.dev/vuln/GO-2026-6089) | Module | Apply ReadHeaderTimeout when doing unencrypted HTTP/2 check in net/http |
| [GO-2026-6088](https://pkg.go.dev/vuln/GO-2026-6088) | Module | Add recursion depth guard during decode in encoding/xml |
| [GO-2026-5026](https://pkg.go.dev/vuln/GO-2026-5026) | Module | Invoking failure to reject ASCII-only Punycode-encoded labels in golang.org/x/net/idna |

GO-2026-6604 is explicitly Windows-only in the official advisory. Although the
scanner emitted a symbol trace to a shared os helper during the Linux scan, this
is not evidence of Linux exposure. Windows behavior remains untested. The other
symbol results identify candidate library paths, not demonstrated attacks.
The repository reader already rejects noncanonical paths and symlinks; its
additional checks do not justify ignoring an upstream os.Root repair.

Go 1.26.10 is the earliest 1.26 patch satisfying the newest detected x509 repair
(GO-2026-6809); it includes the preceding TLS, ASN.1, os and other standard-library
repairs. No major/minor upgrade or driver/API replacement is needed.
The remaining x/mod advisories GO-2026-6179 and GO-2026-6180 concern sumdb/tlog and
sumdb.Client.Lookup, not the imported module-parsing paths. Their fixed module
version is v0.40.0. Retaining v0.34.0 is a bounded non-reachability disposition,
not an ignore rule; reassess if imports or use change.

### Selected graph beyond the loaded packages

A supplemental check of `https://vuln.go.dev/index/modules.json` and each
candidate's `https://vuln.go.dev/ID/<ID>.json` at 2026-10-25 14:03–14:04 UTC
compared the `go list -m all` versions with official introduced/fixed ranges and
withdrawn status. Unlike source scanning, this checks module presence without
loading their packages. The graph is identical before/after the toolchain edit.
In addition to the two x/mod entries, these **15 active graph-only advisories**
remain; none of these modules appeared in the normal or test package dependency
list. They were not counted as source scanner detections or tested at runtime.

| Selected module | Advisory IDs | Fixed module version |
| --- | --- | --- |
| goldmark v1.4.13 | [GO-2026-5320](https://pkg.go.dev/vuln/GO-2026-5320) | v1.7.17 |
| x/sys v0.42.0 | [GO-2026-5024](https://pkg.go.dev/vuln/GO-2026-5024) | v0.44.0 |
| x/net v0.52.0 | [GO-2026-4918](https://pkg.go.dev/vuln/GO-2026-4918) | v0.53.0 |
| x/net v0.52.0 | [GO-2026-5025](https://pkg.go.dev/vuln/GO-2026-5025), [5026](https://pkg.go.dev/vuln/GO-2026-5026), [5027](https://pkg.go.dev/vuln/GO-2026-5027), [5028](https://pkg.go.dev/vuln/GO-2026-5028), [5029](https://pkg.go.dev/vuln/GO-2026-5029), [5030](https://pkg.go.dev/vuln/GO-2026-5030) | v0.55.0 |
| x/net v0.52.0 | [GO-2026-5942](https://pkg.go.dev/vuln/GO-2026-5942) | v0.56.0 |
| x/net v0.52.0 | [GO-2026-6603](https://pkg.go.dev/vuln/GO-2026-6603), [6610](https://pkg.go.dev/vuln/GO-2026-6610), [6611](https://pkg.go.dev/vuln/GO-2026-6611), [6612](https://pkg.go.dev/vuln/GO-2026-6612), [6617](https://pkg.go.dev/vuln/GO-2026-6617) | v0.60.0 |

The lib/pq index candidates GO-2026-6166, 6168, 6169, 6170, 6171, 6172 and
6173 were withdrawn on 2026-08-18 in their official ID records; they are not
active findings. No other active version-range candidates were identified for
the selected external modules. This comparison is bounded to that DB snapshot.

No blanket graph upgrade is performed: retain these findings and reassess before
loading new packages, changing build configuration or shipping downstream code
that imports them. Dependency maintainers own that reassessment; the finite
closure here is explicit inventory and absence from this build's package graph,
not a promise about other builds. An attempted `govulncheck -scan module` without
a package pattern exited 1 because the module root has no Go files; it was not
used as successful evidence or treated as clean.

### Toolchain, separately from the application

Binary scans of the actual old/new `GOROOT/bin/go` returned 3 / 0. The old binary
reported 16 symbol, 4 package-only and 3 module-only standard-library findings;
these are binary symbol-presence results, without source call reachability.
The new binary reported none. This does not audit every compiler/linker binary.

The official DB `index/modules.json` toolchain entry was also inspected because
source/binary stdlib scans do not cover all cmd/go advisories. Four entries apply
to Go 1.26.4: [GO-2026-6179](https://pkg.go.dev/vuln/GO-2026-6179) and
[GO-2026-6180](https://pkg.go.dev/vuln/GO-2026-6180), fixed in 1.26.6;
[GO-2026-6601](https://pkg.go.dev/vuln/GO-2026-6601) and
[GO-2026-6602](https://pkg.go.dev/vuln/GO-2026-6602), fixed in 1.26.9.
Go 1.26.10 is outside those affected ranges. The x/mod module copy and cmd/go's
bundled copy are separate: repairing the toolchain does not change go.mod's x/mod.
The older bootstrap and the acquisition environment are not retrospectively
certified. No evidence of compromise was established.

## Reproduction and validation

Commands were run from the repository root unless noted. `GOCACHE` was set to
`/tmp/gq-sec-01/cache` for builds. Network-dependent commands required approved
execution after sandbox DNS failures; no global network or Go configuration was
changed. Initial default-cache go list/Strict attempts encountered a read-only
cache; using the writable temporary cache resolved that execution constraint.
Scanner failure, unavailable environments and findings were never mapped to clean.

```sh
# Run outside the production module:
cd /tmp
GOBIN=/tmp/gq-sec-01/bin go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
# From the repository root, before and after the toolchain edit:
go version
go env GOTOOLCHAIN GOOS GOARCH GOAMD64 CGO_ENABLED GOFLAGS GOWORK
go mod graph
go list -m all
GOCACHE=/tmp/gq-sec-01/cache go list -deps -test ./...
GOCACHE=/tmp/gq-sec-01/cache GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
  /tmp/gq-sec-01/bin/govulncheck -db https://vuln.go.dev -show verbose ./...
GOCACHE=/tmp/gq-sec-01/cache GOOS=linux GOARCH=amd64 CGO_ENABLED=1 \
  /tmp/gq-sec-01/bin/govulncheck -db https://vuln.go.dev -test -show verbose ./...
/tmp/gq-sec-01/bin/govulncheck -db https://vuln.go.dev -mode binary -show verbose "$(go env GOROOT)/bin/go"
```

The post-toolchain scan preceded addition of the ASN.1 regression; final-head
scans are recorded in the PR report. The production inputs differ only in
selected toolchain; the new regression adds a test-only ASN.1 call.

| Validation | Observed result |
| --- | --- |
| `GOTOOLCHAIN=go1.26.4 go test ./tests -run '^TestDependencyASN1DepthRejection$' -count=1` | Exit 1, excessive nesting accepted; expected sensitivity failure |
| Same focused command under actual Go 1.26.10 | Exit 0, ordinary input accepted and excessive nesting rejected |
| `make test-integration` | Exit 0: MySQL 8/PostgreSQL 16 healthy, explicit DSNs, `go test ./... -count=1`; API, custom Executor, errors, SQL arguments, CLI/MCP/type analysis included |
| `go test -race ./tests -run '^TestDependencyASN1DepthRejection$' -count=1` | Exit 0 |
| `go test -race ./orm/manifest ./orm/driver ./orm/mcp -count=1` | Exit 0 |
| `go mod tidy -diff`; `go mod verify`; `git diff --check` | Exit 0; no module/sum changes beyond toolchain directive |
| `go vet ./...` | Exit 0 |
| `sh scripts/test-public-output-check.sh` | Exit 0 |
| `GOCACHE=/tmp/gq-sec-01/cache python3 scripts/release_validation/check_strict_inputs.py` | Exit 0, 8 cases; initial default-cache run failed before validation |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1, 1504 findings, public list truncated; not a clean review or new SQL audit |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit 1: schema/policy OK, generated_code stale, database skipped; no four-axis freshness claim |

The regression checks standard-library decoding behavior rather than comparing
a version string. It is not an exploit of a Goquent endpoint. The integration
run began before the new regression file was added; the focused run and exact
PR-head CI cover that addition. No runtime API or SQL implementation was changed.
Individual subtest skips were not enumerated by the non-verbose integration log;
explicit database connection failures were not skipped. No manifest was
regenerated, and no existing gate was weakened.

## Remaining boundaries and ownership

- Implementation: minimal patch selection and sensitive regression; reviewer
  acceptance remains separate. No demonstrated project-specific critical exploit
  or P0 escalation was established.
- Verification: limited to the recorded Linux/default-tag build and tests.
  Windows/macOS, other architectures/tags, deployment TLS configurations,
  downstream builds and complete compiler behavior remain unverified. The
  maintainer can close this finite verification step using exact-head CI and
  this evidence, without claiming all configurations are secure.
- Contract limits: static reachability is conservative for interfaces and
  incomplete for reflection/unsafe/runtime loading. The two x/mod findings are
  retained for reassessment by dependency maintainers if usage changes.
- Historical unknowns: preserved manifest/generated-code freshness, earlier
  native evidence and release blockers are not resolved by these green tests.
  Their existing owners and evidence requirements remain unchanged.
- Integration: draft PR review and merge are controller/maintainer decisions;
  Issue 132 remains open and planned PR2 is separate.
- Release approval: not requested or granted. No tag/package/release publication,
  readiness transition or Issue closure is part of this work.

Primary references: [govulncheck documentation](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck),
[official DB format](https://go.dev/security/vuln/database),
[official Go downloads](https://go.dev/dl/), and the advisory links above.
