# Bounded external input audit

GQ-SEC-03 / planned PR1, Refs #134. This finite audit covers CLI JSON file
loaders, OperationSpec input and MCP input/framing. It does not claim that all
CLI inputs are bounded or that all denial-of-service attacks are absent.

## Baseline and scope decision

Baseline main: `9f884258e6d4dc196e41aa816e6d70e318b139c3`. PR139 head
`2c5bc78207c3a9d454590544ab25a6908fe9dccf` is an ancestor, checked by GitHub
compare and local merge-base after fetching main. Before implementation, exact
main CI **40212711393**, Security **40212713379**, and Docs **40212711243**
succeeded. CI was initially in progress and was not treated as success then.
Docs build/publish both succeeded; gh-pages
`3d35be5b47397661b4c46316194cfa3db491b6a7` records deployment of that exact main.
The Pages settings API returned 404: serving configuration remains unverified,
separate from the observed branch publication. No schedule wait or substitute
dispatch was performed.

The worker checked the requested cwd, Ubuntu 24.04.5 / WSL2, clean starting tree,
HEAD and origin, root AGENT.MD, the versioned contracts, PR template and prior
[authorization](security-authorization-audit.md), [output](security-output-audit.md),
[dependency](security-dependency-audit.md) and [workflow](security-workflows.md)
audits. Sandboxed Git DNS and gh authentication checks failed; approved SSH/API
checks succeeded as worker `recoweft`, with repository push permission and a
successful push dry-run. No planned remote branch or matching PR in any state
existed at that check. This is worker authentication, not controller authentication.
Sandbox .git writes failed; approved branch creation succeeded. Actual push, draft
PR identity and exact-head checks belong in the final PR/controller report.

The delivered specification answer for question
`02315411-85b2-4d83-8f2c-fe6503f43344` adopted the following compatibility change:
**8 MiB (8,388,608 bytes) per raw file** for exactly these five loaders:

- `cmd/goquent/manifest.go`: `loadSchema`, `loadPolicies`.
- `orm/manifest/manifest.go`: `Load` (also facade `orm.LoadManifest`).
- `orm/review/config.go`: `LoadConfig`.
- `orm/review/review.go`: `reviewPlanJSONFile`.

Previously these called unbounded `os.ReadFile`. This is a new rejection of
previously accepted files, not restoration of an existing universal 1 MiB limit.
The selected 8 MiB is neither an optimal bound nor a proof of downstream safety.
All downstream compatibility is unverified. No unbounded override is added.

`internal/inputjson/source.go` now opens/closes the file and reads through
`io.LimitReader` with limit+1; actual bytes, including whitespace/BOM, count.
Exactly 8 MiB reaches the unchanged decoder. Oversize returns `inputjson.ErrInvalid`
without source bytes; read errors return no partial bytes. It does not use stat
size as the enforcement mechanism. Existing CLI adapters keep fixed
`PUBLIC_OUTPUT` errors and exit 2. No public error type, version or schema changes.
A normal short read continues; data plus EOF succeeds; non-EOF read errors fail.
EOF alone is not a file-length promise: incomplete JSON still reaches the existing
parser and follows its existing semantics. A successful byte read is not a valid
JSON or semantic-validation claim. File-close errors retain the old ReadFile-style
behavior; special files, races and filesystem trust are not certified here.

## Finite entry inventory

Paths are relative to this baseline plus this change. Owners denote component
maintainers, not a new personal assignment. Read → parse → typed decode → validate
→ publication are distinct steps; a byte cap does not authorize the result.

| Entry / owner / source | Read and parse bounds; typed validation and output |
| --- | --- |
| CLI operation spec/values; CLI + operation owners; `cmd/goquent/operation.go`, `orm/operation/{spec,input}.go`, `internal/inputjson/{budget,object}.go` | Each file read at most 1 MiB+1; combined raw spec+values+19-byte wrapper must fit 1 MiB. CheckJSON: UTF-8, depth32 root0, 16384 values including containers, case-fold duplicate keys, one document. Spec version/closed fields and Filter/Order validation precede compilation; UseNumber retains lexemes. Logical wrapper Size additionally counts actual encoded bytes and raw spec overhead. DiagnosticView is public; no DB execution. |
| Direct OperationSpec/UpdateSpec and Values; operation owners | No external IO in Compile. Closed primitive walk rejects opaque methods/cycles/depth/node/byte excess before inspecting values. Typed values retain native scalar types. Method UnmarshalJSON sees only the bytes supplied by encoding/json (outer whitespace may already be removed); caller must bound the full external stream. sourceBytes retains a count, not the source buffer. UpdateSpec is application-only, not a new CLI/MCP write endpoint. |
| CLI schema/database schema; CLI + migration owners; `loadSchema`, `schema_source_json.go`, `column_json.go` | New 8 MiB raw read → unchanged schema methods/Object → existing source/nullable/semantic checks at consumer → summary/plan view. Object alone has no byte/depth/node budget. Known field case/null and nullable presence rules differ from operation JSON. Legacy unknown fields remain permitted. Direct json.Unmarshal callers own their byte read. |
| CLI policy list or wrapped list; CLI + query owners; `loadPolicies`, `tablepolicy_json.go` | New 8 MiB read → array decode, then wrapper fallback on error → per-policy planversion → existing policy consumers → fixed summary/error. At most these two format attempts, not recursive string decoding. Unknown/duplicate non-version legacy behavior is preserved; not replaced by CheckJSON. |
| Manifest path; manifest owners; `manifest.Load`, facade `orm.LoadManifest` | New 8 MiB read → ordinary manifest decode and column methods → consumer-specific version/integrity/freshness checks → public summaries/diagnostics. Load alone does not validate freshness or confer authority. Used by CLI manifest/repository/doctor/operation/MCP and review context/manifest inspection, and fixture generator. In-memory manifests/private generated snapshots have no new byte cap. |
| Review config; review owners; `LoadConfig` | New 8 MiB read → existing json.Unmarshal into Config → Run/RunCI rule/suppression validation → fixed CLI errors/report views. Also applies to RunCI base config reads. No new key/null/UTF-8 policy is imposed on this legacy format. |
| Review plan/report file; review owners; `reviewPlanJSONFile`, `source_report_json.go`, `review.go` | New 8 MiB read → envelope recognition and existing QueryPlan/MigrationPlan decode/version checks → findings/unsupported markers → public ReportView. Report-envelope recognition separately scans at most maxTypeFileBytes, depth64, 65536 tokens; this is not a complete source JSON budget. Source reports are not execution/review evidence; mixed plan risks remain visible. Other parse attempts still operate on the bounded file; no universal node/CPU bound is claimed. |
| Preflight CLI source/plan/phases/reference; migration owners; `cmd/goquent/preflight.go`, `inputjson.Document`, `preflight_json.go` | Existing 8 MiB+1 read, Document depth64, lowercase/exact duplicate refusal, recursive null refusal, one document. No separate node counter or explicit raw UTF-8 rejection in Document. Source methods and preflight semantics follow; public PreflightView. Collection is an explicitly separate live-read mode; offline JSON cannot create its private receipt. No new policy here. |
| Review CI baseline/diff; review owners; `cmd/goquent/review_ci.go`, `review/{ci,ci_baseline,ci_diff}.go` | ReadCIInput checks regular file and limits actual reads to limit+1. Baseline 256 KiB /64 entries, strict publicview grammar depth32 and exact kind/version, required entry fields, then semantic/expiry evaluation. Invalid entries remain invalid; no threshold/baseline policy changed. Diff has its own existing size/file/hunk budgets. |
| Repository checker registry/input; manifest owners; `repository_check.go` | Existing bounded Root read: 1 MiB config/input, 4 MiB source file, 64 MiB aggregate. CheckJSON/closed registry decode precedes regeneration comparison. Fixed public classifications. Filesystem and generated-process trust beyond necessary read budgets are PR2, not re-audited here. |
| Public view readers; output owners; `orm/internal/publicview/wire.go` and consumer view decoders | Existing 1 MiB/depth32, exact kind/version, closed fields, lowercase/duplicate/null/trailing refusal, then consumer-specific counts/enums. These readers are display-only, not source loaders or execution authorization. No universal node cap or explicit UTF-8 check is added. |
| MCP direct map/prompt/tool; MCP owners; `input.go`, `server.go` | primitiveJSON walks built-in values first, conservative byte accounting (sixfold strings), depth32/16384 values; cycles stop at depth limit, opaque methods are not called. Supported maps/lists then marshal, with final 1 MiB check. Direct scalar types are restored for operation validation. String spec/values decode once through validateJSON then typed Decode; not an arbitrary recursive decode of every string. |
| MCP raw JSON-RPC; MCP owners; `jsonrpc.go`, `input.go` | 1 MiB raw → syntax → UTF-8/depth32/16384/duplicate scanner → exact envelope fields/version/object params and safe ID → dispatch. Nested params/spec get additional bounded scans before map decoding can lose keys; this is intentional defense at separate entry boundaries, not parser unification. ID max1024 decoded string bytes or exact safe integer ±9007199254740991. Fixed error responses; valid top-level ID echo remains the existing exception. |
| MCP Serve; MCP owners; `readMessage` | Shared 1 MiB accumulation for leading blank lines + line/headers; newline counts. Framed body has a separate 1 MiB Content-Length cap before allocation and ReadFull. Duplicate Content-Length rejected; length parse overflow/negative rejected. Default bufio may read ahead by its 4096-byte buffer. Line mode's newline means its largest returned payload is smaller than raw HandleJSONRPC's cap. Per-message budgets reset; no session-total limit or timeout is promised. |

The independent parsers intentionally differ. Replacing them with one validator
would change legacy null/unknown/key/numeric behavior and is outside this PR.
The standard JSON parser also has its own syntax-depth limits; those are not a
new project-wide contract. Large numeric lexemes and escaped strings remain
byte-bounded on the operation/MCP paths, but decoding and re-encoding allocate
additional memory and repeat finite work. No whole-process 8 MiB memory ceiling,
constant CPU bound or universal allocation bound follows from these tests.

## Same-contract framing repair and regression evidence

After a Content-Length line, EOF before the terminating blank header line used
to return io.EOF from readMessage. Serve treated that as clean end-of-stream.
The new test reproduces this with a tiny harmless frame. The header loop now
returns the existing publicoutput.ErrOutput, like other incomplete frames. No
valid frame, message size, timeout or public error classification was changed.
This is malformed-input error handling, not evidence of a critical exploit.

Added regression sources:

- `cmd/goquent/security_input_test.go`: schema, both policy encodings, manifest,
  config at 8 MiB−1/equal/+1; compare unchanged typed values against small input;
  oversize CLI failures have fixed text, exit2 and no stdout.
- `orm/review/security_input_test.go`: plan-file boundaries and unchanged findings.
- `internal/inputjson/source_test.go`: generated counting reader proves at most
  limit+1 bytes; no huge backing fixture. Short/one-byte/final-data+EOF reads,
  non-EOF/UnexpectedEOF errors with no partial result. Existing raw JSON byte,
  depth and node limits at −1/equal/+1, escaped/case duplicate keys, malformed
  UTF-8/numbers, null/unknown responsibilities and trailing data.
- `orm/mcp/security_input_test.go`: line/header/body boundaries, duplicate headers,
  huge/negative length, split reads and truncated headers/body. A reader error
  whose Error method panics confirms public refusal neither formats nor retains
  the source error. Rejected frames produce no response/dispatch output.
  Two bounded fuzz targets check raw-validator agreement, fixed RPC errors and
  framing invariance under read fragmentation.

Before production changes, all five loader oversize branches (plus the wrapped
policy form), CLI refusal, plan-file oversize and incomplete-frame regressions
failed (exit1, no build failure). After changes they pass. This is sensitivity
evidence for the actual prior source, not only tests against a mock repair.

Existing tests reused include TestBudgetsAndOpaque, TestOperationSharedBudgetBoundaries,
TestJSONNumberCompileKeepsPrecisionAndINOrder, TestTypedNullEnumAndOperator,
TestOperationManifestVersionAndSchemaValueContract, TestPublicInputRejectsCallbacksAndCycles,
TestPublicRPCIDsAndEnvelope, TestPublicServeBoundsAndSinkErrors,
TestTypedOperationDirectAndRPCNoCoercionOrDisclosure, TestOperationValuesSingleDocumentPrecision,
preflight/nullable/version tests, source-report and baseline regressions, and
public output/contract suites. These supply typed numeric preservation, version
and unknown/null distinctions, callbacks0/cycles, no execution from plans and
existing DB refusal behavior. The five changed file loaders do not themselves
receive a DB/execution callback; the new size checks return before decoding or
calling their consumers. No connection or execution capability was added.

## Validation and finite exit

Actual local toolchain: Go **1.26.10 linux/amd64**, `GOTOOLCHAIN=local`, using the
installed toolchain explicitly (not the default older PATH Go). Local commands
use writable temporary GOCACHE and GOPROXY=off. Raw output is captured outside the
repository/analysis root under an owner-only temporary directory and never
uploaded. The final report separates exits, skips and unverified checks.

| Actual command (selected Go/environment above) | Result |
| --- | --- |
| `go test ./internal/inputjson ./cmd/goquent ./orm/review ./orm/mcp -run 'TestSecurityInput|FuzzSecurityInput' -count=1` | Exit0, deterministic boundaries and fuzz seeds. Later added `TestSecurityInputRawRPCBoundaries` separately passed; exact-head CI covers the final test set. |
| `go test -race ./internal/inputjson ./orm/mcp ./orm/review ./cmd/goquent -run 'Test(SecurityInput|Public|TypedOperation|OperationShared|OperationValues|RPCOperation)' -count=1` | Exit0, four packages. Final raw-RPC-only test added later is not claimed race-tested locally. |
| `go test ./orm/mcp -run '^$' -fuzz '^FuzzSecurityInputRPC$' -fuzztime=10s -parallel=1` | Exit0; 8 checked-in seeds; 33,413 executions, 42 new interesting inputs, about11s fuzz elapsed. |
| `go test ./orm/mcp -run '^$' -fuzz '^FuzzSecurityInputFrame$' -fuzztime=10s -parallel=1` | Exit0; 6 checked-in seeds; 107,801 executions, 32 new interesting inputs, about11s fuzz elapsed. |
| `make test-integration` | Exit0, explicit local MySQL8/PostgreSQL16 fixture DSNs, `go test ./... -count=1`, 26 passing packages. Non-verbose output does not enumerate individual skips. Final raw-RPC-only test added later passed separately. |
| `go vet ./...`; `sh scripts/test-public-output-check.sh` | Both exit0. |
| `python3 scripts/release_validation/check_strict_inputs.py` | Exit0. |
| `python3 -B -m unittest discover -s scripts/security -p 'test_*.py'`; corresponding `scripts/release_validation` discovery | Both exit0; wrapper/orchestration fixtures, not release acceptance or a fresh local vulnerability DB scan. |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit0. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit1, 1510 findings, suppressed list0, truncated display. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit1, schema/policy ok, generated_code stale, database skipped. |
| `git diff --check` | Exit0. |

Fuzz targets are in `orm/mcp/security_input_test.go`; source under test is the
current branch implementation of `input.go`, `jsonrpc.go`, `server.go` and
`internal/inputjson/budget.go`. They skip mutated inputs over64 KiB to keep this
short run inexpensive; deterministic tests separately exercise full configured
limits. Seed bytes are the checked-in f.Add cases, not an asserted reproducible
random mutation seed. Go does not expose a fixed mutation seed here. RPC seeds
include ping, duplicate keys, huge exponents, null, trailing input, invalid UTF-8,
surrogate escapes and string-contained spec. Frame seeds include line/framed JSON,
truncated header/body, blank input and zero Content-Length. No crash corpus was
produced. The final commit SHA is recorded in the PR/controller report.

Sandbox Docker socket access failed. A narrowly approved daemon/service diagnostic
succeeded, followed by a separately approved integration run; connectivity was
not counted as test success. No approval was denied. No Windows program, native
census or schedule dispatch was needed.
The finite exit condition is this trace, sensitive regression failures against
the prior implementation, passing relevant regressions/required checks, short
fuzzing and one draft PR with exact-head CI/Security evidence. Fuzz success is
not proof of all-input safety. No intentionally enormous/OOM test is performed.

## Residual limits and ownership

| Classification | Owner and remaining evidence/decision |
| --- | --- |
| Actual defect | MCP header EOF was silently accepted; fixed with sensitive regression. Five legacy JSON paths had unbounded reads; the adopted new cap addresses these paths only. No critical exploit/OOM demonstration or P0 conclusion. |
| Missing evidence | PR maintainer records actual final head, CI/Security, skipped/unavailable checks. Downstream compatibility of 8 MiB, other OS/platforms, Pages serving settings and first actual weekly schedule remain unverified. |
| Guarantee limit | CLI/migration owners retain unbounded `readMigrationSQL` stdin/file reads and multi-file Builder, `readDesiredVersions` list/file reads, review SQL/Go/source reads and aggregate file counts. New source cap is per file, not process memory/CPU/session total. Caller-owned direct decoding and trusted in-memory snapshots are distinct contracts. |
| Historical cause unknown | Go1.26.10/local and scanner1.9.0 unchanged; x/mod module-only2 and historical graph-only15 retained. Prior SQL review1510/exit1, manifest generated_code stale/database skipped, native unknowns and release blockers are not cleared by these checks. |
| Candidate/release judgement | The delivered answer authorizes only the five-loader 8 MiB change. Remaining SQL/stdin/desired-version/aggregate limits require CLI/migration maintainers to inspect compatibility, decide concrete limits and overflow classification, then implement read-before-decode regressions. Retain P1 planning priority; no independent severity assessment of an exploit. Do not silently move this design into PR2 or start it here. |

General path traversal/symlink/hardlink/ACL/process/env/packages.Load trust work
remains PR2; common-parser refactoring remains REF01. A context alone cannot stop
an arbitrary permanently blocked io.Reader. No transport timeout or host isolation
is introduced, and read-only analysis is not described as a sandbox.

Implementation, executed verification, integration/review and release approval
are four separate states. Only a draft PR is requested; no ready/merge/Issue close,
overall acceptance, release/tag/package publication or next work is performed.
