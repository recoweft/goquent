# Bounded sensitive output audit

GQ-SEC-02 / planned PR2, Refs #133. This is a finite audit of the existing
publication contract, not a new masking API or a proof that all attacks are absent.
It complements [PR1's authorization audit](security-authorization-audit.md).

## Source and prerequisites

Baseline: main `47dca6180d3bcacd1d6b7105c24f96092a55db33`, with merged PR138 head
`de50cfc9fb493f68eb67af72e137b9afa2c8f4b9` as an ancestor (GitHub compare and local
merge-base). Before source changes, exact-main CI **40209617357**, Security
**40209617359**, and Docs **40209617364** succeeded. CI was initially running and
was not counted as success until completion. Docs build and publish succeeded;
gh-pages `a7c601552bc8b89437660f7fd3868d2fb739db4d` records deployment of this main
SHA. No weekly schedule event or substitute dispatch was requested.

The worker checked cwd `/home/murai/github/goquent`, Ubuntu 24.04.5 / WSL2,
origin `git@github.com:recoweft/goquent.git`, clean starting tree and starting HEAD
(PR138 head), root AGENT.MD, the versioned contracts and PR template. Sandboxed
Git DNS failed; approved SSH/API diagnostics succeeded, with worker identity
`recoweft`, repository push permission and planned-branch push dry-run. No remote
planned branch or PR in any state matched. These checks use worker authentication,
not controller authentication. Actual push/PR and exact-head checks are recorded
in the final PR/controller record. Sandbox .git write refusal was followed by
approved creation of the dedicated branch from origin/main.

Policy sources: [redacted output](redacted-output.md),
[public views](public-plan-views.md), [diagnostics](operation-diagnostics.md),
[observations](query-observation.md), and
[contracts](../specs/goquent_ai_contracts_v3.md), including later implementation
updates to historical inventory rows. #73 / GQ-AI-06 is reused, not reimplemented.

## Finite sink inventory

Owners below are repository component maintainers, not an asserted assignment to
an individual. Paths are relative to the repository at the baseline above. Listed
tests are evidence locations; executed checks are distinguished below.

| Sink / owner / exact implementation | Classification and publication boundary | Existing evidence reused |
| --- | --- | --- |
| CLI review / CLI and review owners: `cmd/goquent/main.go`, `review_ci.go`; `orm/review/{format,public_view,public_view_v2,sarif,ci}.go` | pretty/text, json v1/v2, github, sarif, review-ci-json project fixed vocabulary/counts. Source paths, SQL, Evidence, baseline owner/reason and arbitrary parser messages are omitted. Full internal report decides risk/precision/CI exit. | `TestPublicCLIOutputsAndErrors`, `TestPublicReviewTailDoesNotWeakenExit`, `TestReviewCICommandFormatsAndGate`, `TestReviewCIChangedGoTestsFormats`, `TestSARIFCanonicalJSONCorrespondence` |
| CLI migrate / migration owners: `cmd/goquent/migrate.go` | plan/dry-run/apply use MigrationPlanView; status/schema/drift use summaries. Application and DB errors become fixed text. Apply is an existing human-controlled execution command, not an MCP tool. | `TestPublicCLIOutputsAndErrors`, `TestMigrateApplyChecksApprovalBeforeDSN`, `TestSchemaComparisonUnknownIsNonExecuting`, `TestWholeTableUnknownCLI` |
| CLI preflight plan/collect/verify: `cmd/goquent/preflight.go` | Fixed PreflightView and fixed errors; sensitive source is separate. Offline verify cannot manufacture live collection evidence. | `TestPreflightCLISeparatedModes`, `TestPreflightCLIRejectsAmbiguousSource`, `TestPreflightPublicCanary` |
| CLI manifest generate/schema/verify/diff/repository (repo/skeleton aliases), doctor: `cmd/goquent/manifest.go` | Summary/default omission; schema is a fixed contract. Original manifest verification controls exit. Explicit local artifact is a separate sensitive output. | `TestPublicCLIOutputsAndErrors`, `TestManifestVerifyDetectsStaleAndDoctor`, `TestManifestIntegrityConsumers`, `TestExplicitLocalArtifactRestrictions`, `TestTypedRepositoryCLIArtifactBoundary` |
| CLI manifest check-repositories: `cmd/goquent/repository_check.go` | Fixed status/reason allowlist, no generated source/path publication. | `TestRepositoryCheckPublicBoundary` |
| CLI operation compile/schema: `cmd/goquent/operation.go` | Compile success/refusal DiagnosticView; precompile input errors fixed; schema is library-owned. No write operation endpoint. | `TestPublicOperationAndMCPCLI`, `TestOperationDiagnosticCLIFixtures`, `TestTypedCLIRefusalsAreFixed` |
| CLI mcp/help/unknown commands: `cmd/goquent/{main,mcp}.go` | Fixed help and error text; parser output discarded. MCP transport errors are adapted again at stderr. | `TestPublicCLIOutputsAndErrors`, `TestMCPCommandInitialize`, added CLI read-failure test |
| MCP tools/resources/prompts / MCP owners: `orm/mcp/server.go` | All registered tools enumerated by existing route test; read-only plans/reviews, safe schema/manifest/model/relation/policy/example/status summaries, fixed teaching snippets/prompts. Allowlisted names do not interpolate caller text. No execution handle or apply tool. | `TestPublicAllMCPRoutes`, `TestPublicInputRejectsCallbacksAndCycles`, operation diagnostic surface fixtures |
| MCP JSON-RPC/Serve/input: `orm/mcp/{jsonrpc,input}.go` | Fixed errors; primitive input validation precedes serialization; size/depth/node/ID validation; framed output and short-write detection. Validated top-level correlation ID is the explicit echo exception. | `TestPublicRPCIDsAndEnvelope`, `TestPublicServeBoundsAndSinkErrors`, added source-reader failure test |
| PlanView and QueryPlan.String / query owners: `orm/query/public_view.go`, `plan.go` | SQL, typed params, tenant, names, reason, Metadata, Evidence and opaque payloads omitted without traversing them. Closed classification and bounded lists; view mutation is detached. | `TestPublicPlanViewCanariesOpaqueAndMutation`, `TestPublicPlanViewBoundsAndLegacy`, `TestPublicPlanViewPrivateSealDispatch`, `TestPublicPlanViewSecretDispatchAndRefusal` |
| ReportView v1/v2, Pretty/GitHub/SARIF / review owners: `orm/review/{public_view,public_view_v2,format,sarif}.go` | No source Evidence/paths/messages; v2 retains normalized analysis classifications. SARIF drops untrusted source locations, uses canonical diagnostics when available. | `TestPublicReportViewOmissionAndTampering`, `TestPublicReportViewBoundsAndCodes`, review CI tests, added SARIF opaque/error test |
| Operation diagnostics / operation owners: `orm/operation/{diagnostics,diagnostic_view}.go` | Single-pass compile facts, fixed code/location vocabulary, no values, private targets or arbitrary causes. Internal compile errors have separate identity contracts. | `TestDiagnosticsNoOpaqueEvaluationOrDisclosure`, `TestDiagnosticWireErrorsBoundsAndSink`, `TestDiagnosticsUnknownStrictAndRefusalAfterTruncation`, shared contract fixtures |
| Migration/manifest/summary/preflight/performance views / respective owners: `orm/migration/{public_view,format,preflight,preflight_view}.go`, `orm/manifest/{public_view,format}.go`, `orm/publicoutput/output.go`, `orm/query/performance.go` | Closed display projections. Source SQL/catalog/name/default/DSN/receipt data remains internal. Performance acquisition is explicit and separate from ordinary planning. | `TestMigrationPublicBudgetAndTampering`, `TestSummaryViewsSanitizeAndBound`, `TestPreflightPublicCanary`, `TestPerformanceJSONAndPublicBoundary`, `TestPerformanceExternalExecutorAndPlanRemainOffline` |
| Built-in driver logs / driver owners: `orm/driver/driver.go`; JSON predicate logs: `orm/internal/querybuilder/database/{mysql,postgres}/where.go`; profiling: `orm/internal/querybuilder/internal/profiling/profiling.go` | MySQL connector installs a fixed logger, ignoring payloads. Predicate encoding failure log is fixed; actual binding may legitimately invoke JSON methods. Profiling publishes duration only. Returned driver errors remain sensitive library data. | `TestPublicDriverLoggerDoesNotFormatPayload`; source trace for predicate logs/profiling, not a new all-driver logging guarantee |
| Observation hook arguments, Scope/Event/Summary output / observation owners: `orm/observation/observation.go`, `orm/internal/observe/{state,types,rows}.go` | No SQL/args/causes/shape keys retained in public events. Drain explicitly invokes application hooks, outside dispatch; hook side effects are a trust boundary. Error classification does not call arbitrary error methods. | `TestPublicProjectionAndUntrustedWire`, `TestDrainReentryPanicAndBatch`, `TestSlowDrainDoesNotLockSQLAndConcurrentDrainBusy`, `TestObservationRowsBoundaries`, `TestObservationConnectionCancelAndResultMetrics` |
| CI wrapper / CI owners: `scripts/test-public-output.sh`, `scripts/test-public-output-check.sh` | Restricted ephemeral raw capture, fixed status and original exit, no raw DB log upload. Failure is not converted to success. | Wrapper scenarios: success, check/test failure and build failure |
| Security runner / security workflow owners: `scripts/security/{check,test_check}.py` | Fixed source/tool metadata, counts and exits; raw scanner stderr, progress and paths remain outside checkout, never uploaded. | `BoundaryTests`: complete/partial/malformed results, execution/install/gate failure, timeout, capture failure and exception publication |

## Added evidence and finite outcome

No new supported-contract runtime leak was reproduced. Only tests and this audit
are changed; production code, dependencies, workflow definitions, public versions
and compatibility remain unchanged.

* `TestSecurityOutputSARIFOpaqueAndErrors` adds the later SARIF consumer to opaque
  Evidence coverage: Stringer, Valuer, Marshaler, Formatter, Error and Unwrap
  counters/panic fixtures, cycles, nested SQL/typed params/Metadata callback,
  source/sink errors and short writes. It asserts an expected fixed diagnostic
  survives (not merely empty output), source fields remain intact, and no method
  or callback was evaluated. Counters also detect fmt's recovery of formatter
  panics. Existing all-format view tests are not duplicated.
* `TestSecurityOutputMCPSourceErrorBoundary` covers zero bytes, partial line,
  partial header and partial framed body from a failing reader. Public errors
  neither retain nor unwrap/evaluate the source; no partial response is emitted.
  Normal EOF remains successful. Existing sink/budget/ID tests remain separate.
* `TestSecurityOutputCLIMCPReadFailure` follows those read failures through the
  actual CLI adapter, checking exit 2, empty stdout, fixed stderr and zero error
  method calls. These fixtures contain no real credentials or external target.

The existing canaries cover fictional SQL/PII/tenant/password/token/name/path
values, schema/defaults, Metadata/Evidence and driver/provider/parser errors.
Provider refusal and typed execution spies remain in PR1 and
`orm/query/{authorization_execution,public_view}_test.go`; public display does
not evaluate a provider or replace private authorization material. The fixed
diagnostic code, count, position and allowed ID assertions prevent treating every
observable value as a leak.

## Exceptions and limits

Source JSON (`ToJSON`, `WriteJSON`, source plan/report/manifest serializers),
Build/Dump/RawSQL, generated repository source, internal artifacts and returned
runtime errors are sensitive data APIs, even where exported. They are not silently
masked or made incompatible. Explicit local exports, including preflight source,
retain their existing restrictions; mode/path checks are not proof of physical
confidentiality. Validated JSON-RPC ID echo is allowed only in top-level `id`.

Custom drivers/executors, Raw callers, external loggers and application callbacks
can independently disclose their own inputs. Display does not sandbox them.
Observation Drain invokes the explicitly supplied hook; ordinary formatting does
not. Methodless casts, extracted fields, caller-added sibling payloads, concurrent
mutation and writer panics/independent effects remain outside the contract.
Unknown opaque contents are omitted, not recursively redacted or string-replaced.
Public fields convey structural claims, not execution authority, live freshness
or a complete CI decision. Full source reports still gate tails beyond display
limits; private typed values and seals remain execution inputs.

This finite review does not audit general process/FS resource boundaries (SEC03),
all custom integrations, all possible formatter combinations, or production
systems. No private vulnerability disclosure was needed because no concrete
serious vulnerability was reproduced. No new public guarantee is proposed.

## Validation and disposition

Actual toolchain: Go 1.26.10 linux/amd64, explicitly selected, GOTOOLCHAIN=local.
Local test commands use cached dependencies (GOPROXY=off). Raw diagnostics are
captured under an owner-only `/tmp` directory outside the checkout/analysis root;
they are not uploaded. The existing CI capture wrapper is unchanged and its
regression is exercised. The final source identity and exact-head checks are recorded in the PR/controller
report. Local results on the changed tree:

| Actual command (under restricted capture) | Result |
| --- | --- |
| `go test ./orm/query ./orm/review ./orm/operation ./orm/mcp ./orm/migration ./orm/publicoutput ./orm/driver ./orm/internal/observe ./tests/contracts ./cmd/goquent -run 'Test(Public|MigrationPublic|DiagnosticsNoOpaque|DiagnosticWire|OperationDiagnostic|PreflightPublic|PreflightCLI|RepositoryCheckPublic|ReviewCI|SARIF|PerformanceJSON|ExplicitLocal)' -count=1` | Exit 0, ten packages, baseline before additions |
| `go test ./orm/review ./orm/mcp ./cmd/goquent -run '^TestSecurityOutput' -count=1` | Exit 0, all three added regressions |
| `go test -race ./orm/review ./orm/mcp ./cmd/goquent ./orm/internal/observe -run 'Test(SecurityOutput|BudgetConcurrent|FinishRace|SlowDrain)' -count=1` | Exit 0, four packages |
| `make test-integration` (`go test ./... -count=1`, explicit local MySQL/PostgreSQL fixture DSNs) | Exit 0, 26 passing packages. Non-verbose output does not enumerate individual skips; no zero-skip claim. |
| `go vet ./...` | Exit 0 |
| `sh scripts/test-public-output-check.sh` | Exit 0, existing capture-wrapper scenarios |
| `PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s scripts/security -p 'test_*.py'` | Exit 0, harmless scanner process fixtures; not a fresh vulnerability DB scan |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit 0 |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1, 1510 findings, suppressed list 0, truncated public display |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit 1; schema/policy ok, generated_code stale, database skipped |
| `git diff --check` | Exit 0 |

Sandbox Docker socket access was denied; the approved daemon/service diagnostic
succeeded, then the separately approved integration command succeeded against the
existing local containers. Connectivity alone was not counted as integration.
No approval was denied. No Windows tooling or uncached dependency acquisition was
needed. No fresh native census, docs regeneration or weekly schedule was run.

The finite exit condition is inventory/source trace, reuse of existing boundary
tests, the three missing consumer/error regressions, relevant local validation
and exact-PR-head CI/Security results. It is not an endless search for every attack.

| Classification | Owner / remaining closure evidence |
| --- | --- |
| Actual defect | None reproduced within this finite publication contract; future concrete reproductions require separate maintainer triage. |
| Missing evidence | PR maintainer records exact-head checks and any unavailable/skipped tests. First actual weekly schedule remains a separate SEC01 item; no waiting or substitute dispatch for it. |
| Guarantee limit | Application/integration owners control sensitive source APIs, callbacks, physical DB identity and publication outside the defined sinks. No universal redaction guarantee. |
| Historical cause unknown | Prior SQL review: 1510 findings/exit 1; manifest generated_code stale/database skipped; historical native unknowns and release blockers remain. This PR's passing tests do not establish their causes or close them. |
| Candidate/release judgement | No runtime repair candidate or new contract is proposed. Release owners retain all release decisions; module-only x/mod 2 and historical graph-only 15 remain inherited findings, scanner v1.9.0 unchanged. |

Implementation (tests/docs), verification (executed evidence), integration
(draft PR and review), and release approval are separate states. This work does
not mark ready, merge, close Issue 133, approve overall acceptance, publish a
release/tag/package, or begin SEC03/REF01.
