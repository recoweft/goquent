# Bounded JSON validation sharing

GQ-REF-01 / PR3, Refs #135. The pre-change comparison below selects only the
identical raw validator in `internal/inputjson.CheckJSON` and
`orm/mcp.validateJSON`. It does not replace consumer decoders.

## Rule comparison before implementation

| Rule | CheckJSON | MCP validateJSON | Consumer rules retained |
| --- | --- | --- | --- |
| Raw bytes | At most 1,048,576 including whitespace | Same | MCP line/header includes newline; framed body has its own cap. Operation combines spec/values/wrapper and decoded Size. Legacy five file loaders retain 8 MiB. |
| Depth | Root depth 0; reject only depth >32 | Same | Preflight Document depth64; publicview depth32; neither acquires a new node cap. |
| Nodes | Root and each value/container count once; keys/closing tokens do not; reject >16384 | Same | Direct-value budgets remain independent. Empty root container counts one. |
| Limit -1/equal/+1 | Bytes 1048575/1048576 accepted, 1048577 refused; depths31/32 accepted,33 refused; nodes16383/16384 accepted,16385 refused | Same | No thresholds change. |
| UTF-8 / escape | Raw invalid UTF-8 refused; encoding/json decodes escapes, including replacement for unpaired surrogate escapes | Same | No new normalization or surrogate restriction. Document/publicview have no explicit raw UTF-8 check. |
| Duplicate / key case | Per-object decoded key strings lowered by strings.ToLower; exact, escaped and lowercase-colliding duplicates refused; a lone uppercase key allowed | Same | No Unicode normalization or strings.EqualFold substitution. Closed consumer fields can reject the lone uppercase key. |
| Null / root | Null, scalar, array, object allowed by raw checker | Same | MCP envelope must be object, params object and ID non-null; Operation limit:null rejected, Filter null presence retained; Document/publicview recursively refuse null. |
| Unknown / version | No schema or version decision | Same | Operation uses planversion.Read before CheckJSON and closed fields afterward; MCP exact envelope keys/jsonrpc 2.0; publicview exact kind/version and closed fields. Legacy unknowns remain legacy. |
| Numeric | UseNumber; accepts JSON number lexemes without float conversion, including large exponent/integer | Same | MCP ID additionally exact mathematical safe integer or bounded string; Operation decoding retains json.Number and direct native scalar types. |
| Syntax / trailing | One complete JSON value; trailing whitespace only | Same | MCP json.Valid precheck preserves distinct invalid-JSON versus invalid-request errors. String-contained spec/values decode once. |
| Return / error | nil or the existing ErrInvalid sentinel, no parser cause | bool | MCP keeps bool adapter and fixed public error mapping; Operation error ordering/identity unchanged. |

## Selected sharing and change entry points

MCP raw validation delegates to the existing CheckJSON; its transport byte cap
stays local. A test pins both byte caps together and independently checks MCP
acceptance at raw boundaries, preventing a future shared-limit change from
silently changing only one entry. The existing CheckJSON implementation
and inputjson API remain unchanged; no performance improvement is claimed.

Previously a raw grammar/budget correction required editing two token walks.
Now `internal/inputjson/budget.go` is the single raw-check implementation;
`orm/mcp/input.go` retains its boolean adaptation and consumer-only checks.
Dependency direction is MCP -> root internal/inputjson -> standard library;
inputjson never imports ORM or CLI. Other MCP paths already indirectly reach
inputjson through operation. No framework, configurable policy or new dependency.

Paths remain HandleJSONRPC -> syntax -> validateJSON -> envelope/ID -> dispatch;
decodeParams -> validateJSON -> typed decode; decodeAny(string) -> validateJSON
-> UseNumber decode. Operation -> version -> CheckJSON -> closed/typed decode
remains unchanged. Repeated checks at separate entry boundaries remain deliberate.

Excluded: inputjson.Size and MCP primitiveJSON have different supported native
slices/float32, exact encoded size versus conservative sixfold-string accounting,
and duplicate-key behavior. Sharing them would change accepted inputs or evaluate
opaque methods. Object, Document, publicview, legacy loaders, report-envelope
recognition, preflight and baseline readers have different null/key/schema/budget
contracts. A configurable universal parser would obscure those differences for
no needed benefit. Framing, safe IDs, direct typed-value restoration and one-time
string decoding stay in their adapters.

Private authority, binding/tenant/current-input checks, context and typed values,
Plan DB0 and refusal dispatch0 are untouched. No API/wire/version/error identity,
SQL budget, filesystem/process guarantee or existing format is changed. No
manifest, baseline, threshold, snapshot or generated artifact is regenerated.

## Validation scope and residual limits

Reuse SEC03 byte/depth/node and framing boundaries, RPC/frame fuzzing, typed
Operation regressions and SEC02 opaque/canary/authority tests. The new adapter
cases have explicit expected outcomes, not just equality of two calls that now
share implementation. Existing fuzz agreement becomes an adapter consistency
check; it is no longer independent-parser equivalence evidence. Short fuzzing
and finite regressions do not prove all-input safety.

The finite PR3 closes this implementation scope; it creates no further cleanup
ledger. Historical module-only2/graph-only15, SQL review1510 and diff unknown1510
remain historical evidence. Current measurements are recorded separately below.
Manifest stale/database skipped, native unknowns and release blockers retain
their existing owners and closure requirements.

## Executed checks

Baseline main `9485f298112219fc472bd16331c04cf29ffa5fd6` contains PR143 head
`69cf76bfc6934038b3b721b720bb8ce5ad492df3` (local merge-base and API comparison).
Before implementation, exact-main CI40340313987, Security40340314143 and
Docs40340313891 succeeded. Docs build/publish succeeded; gh-pages
`7220289716a254aec3c37e4ff7b6b290ccee30d2` names this source. The Pages settings
API returned404, leaving serving configuration unverified. Pending CI was awaited.

Actual environment: requested Ubuntu24.04.5/WSL2 cwd/origin, clean initial tree,
worker recoweft Git/API authentication, repository push permission and branch
push dry-run verified independently of controller credentials. No matching local/
remote branch or PR in any state existed initially. Sandbox DNS/Docker access
failed; narrowly approved diagnostics succeeded. Branch creation used approved
execution for read-only sandbox .git. No approval denial or host-setting change.
Root AGENT.MD, applicable-instruction search, contracts, PR template and prior
security/input/output/refactor documents were read. No Windows launch was needed.

Local checks use actual Go1.26.10 linux/amd64, explicitly first in PATH,
GOTOOLCHAIN=local, GOPROXY=off and a writable temporary GOCACHE. Raw captures are
owner-only outside the checkout and materialized comparison trees; no upload.

| Command/check | Observed result |
| --- | --- |
| `go test ./internal/inputjson ./orm/mcp ./orm/operation -count=1` | Exit0 before and after delegation. Initial head tree equals main. |
| `go test ./orm/mcp -run '^TestSharedRawJSON' -count=1` | Exit0 against original scanner; same tests pass after delegation. |
| `go test -race ./internal/inputjson ./orm/mcp ./orm/operation ./orm ./orm/query -run 'Test(SharedRawJSON\|SecurityInput\|SecurityOutput\|SecurityAudit\|Public\|TypedOperation\|OperationShared\|Budgets\|JSONNumber\|Authorization)' -count=1` | Exit0. |
| `go test ./orm/mcp -run '^$' -fuzz '^FuzzSecurityInputRPC$' -fuzztime=10s -parallel=1` | Exit0; 8 checked-in seeds,122972 executions. |
| Corresponding `FuzzSecurityInputFrame` command | Exit0; 6 checked-in seeds,77134 executions. |
| `make test-integration` | Exit0; explicit fixture DSNs, MySQL8/PostgreSQL16 healthy, `go test ./... -count=1`, 26 passing packages including CLI/MCP/contracts. Non-verbose output does not enumerate individual skips. |
| Explicit-DSN `go test ./tests -run '^Test(ExternalAuthorizationDatabaseAndExternalTx\|TenantPolicyDatabaseSemantics\|ValidatedBindingDatabaseAndExternalTx)$' -count=1 -json` | Exit0; 9 test/subtest passes, zero skip/fail events. |
| `go vet ./...`; public-output wrapper; strict comparison-input regression | All exit0. |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit0; comparison only. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit1; actual1510 findings, suppressed list0, truncated display. |
| `review --format review-ci-json --diff PATCH --diff-base-root BASE --fail-on high HEAD` | Exit1; actual full1510, new0, worsened0, unknown1510, unchanged0, matched0; input_failed=false, risk_failed=true, full_report=true. Diff modified2/added2, invalid/unlisted/read_failed0. No baseline supplied. |
| Example manifest verify with checked-in schema/policy | Exit1; schema/policy ok, generated_code stale, database skipped. |

The comparison materialized exact main and changed source with the first version
of this document; production code/tests did not change afterward. Unknown source
correspondence is not relabeled unchanged. Fuzz inputs over64KiB are skipped by
the existing harness; deterministic cases exercise the full configured limits.
Go's mutation seed is not fixed; no crash corpus was produced. No full native
census, release run or unrelated API-doc regeneration was performed. Exact final
head, draft identity and CI/Security evidence belong in the PR/controller report.

Implementation is the finite raw-validator delegation. Verification is only the
executed evidence above and final-head checks. Integration requires maintainer
review/merge; draft creation does not establish it. Release approval is absent.
Five dispositions remain separate: no new runtime defect reproduced; missing
platform/serving/full-suite skip evidence stays unverified; consumer/application
trust and finite input bounds are guarantee limits; historical native/manifest/
release unknowns retain existing owners; release decisions remain with release
owners. No ready transition, merge, Issue close, overall acceptance, release/tag/
package publication or next-task start is included.
