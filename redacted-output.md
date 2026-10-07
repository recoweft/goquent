# Public output migration (GQ-AI-06 PR2)

Issue [#73](https://github.com/recoweft/goquent/issues/73), PR2 of 2. This document
records the published revision-2 output boundary. It supplements
[the PR1 field policy](public-plan-views.md); it does not expand its execution or
arbitrary-formatter guarantees. Whole-Issue acceptance and merge are separate.

## Boundary and explicit exceptions

Default CLI stdout/stderr, review GitHub annotations, MCP results/errors/resources/
prompts and generated responses omit SQL (including generated SQL), identifiers,
values, reasons, fingerprints, paths and arbitrary diagnostic text. Projection
never writes back into a plan, Query, settings, handle, approval or executor.
Original complete reports/plans continue to decide thresholds, freshness gates,
approval and apply. A truncated display is not a complete report or safety proof.

Three exceptions are explicit, not access controls:

1. Exported **internal data APIs** below retain their sensitive wire/values. Calling
   them and publishing their results directly is outside the public-display claim.
2. Explicit local artifact files may contain sensitive data. They are never public
   stdout output, and the program cannot establish their physical confidentiality.
3. JSON-RPC echoes a validated correlation ID only in the response's top-level
   `id`. A secret supplied as a string ID therefore appears there. No other field,
   message, data, trace or log inherits this exception.

Caller field extraction, methodless casts, arbitrary sibling payloads, concurrent
mutation and sink panics/independent side effects remain outside PR1's guarantees.
Registered custom drivers, external executors, application callbacks and callers
can independently log their own data; Goquent does not intercept their effects.
Internal driver/runtime error identities and causes remain available to library
callers, who must use a public boundary before logging. No public adapter retains
an underlying error or caller input.

## API and consumer inventory

| Surface | Default public treatment | Internal data / compatibility |
| --- | --- | --- |
| QueryPlan.String | PlanView JSON text; fixed source error for nil/unsupported version | ToJSON/MarshalJSON keep diagnostic envelope/version/number lexemes and sensitive data |
| Query Build/Dump/RawSQL; predicate and OperationSpec serializers | No default CLI/MCP consumer serializes these as output | Existing signatures, SQL, ordered typed args, source wire and interpolation remain data APIs |
| Review WritePretty/WriteGitHub | Project to ReportView; fixed codes, numeric positions, no paths/Evidence | WriteJSON and ReviewReport JSON remain internal diagnostic wire |
| CLI review pretty/json/github | ReportView and public writers; config/discovery/parser/write failures are fixed text | Whole source report drives fail-on, precision and manifest exit codes, including findings after item 256 |
| CLI operation compile | PlanView; fixed parse/compile/IO errors | Compile/spec/values and four diagnostic envelopes unchanged; operation schema is a fixed library contract |
| MigrationPlan.String / migration WritePretty | MigrationPlanView; fixed source error for nil | ToJSON/WriteJSON retain sensitive migration data; EnsureExecutable/Apply unchanged |
| Migration status/schema/drift Pretty writers | SummaryView with structural counts/booleans only | JSON writers retain internal data wire |
| CLI migrate plan/dry-run/apply | MigrationPlanView; original risk and approval decisions | Human deployment operation retained; original SQL dispatched |
| CLI migrate status/schema/drift | SummaryView; driver/IO/runtime errors fixed | Live reads and source comparison unchanged; schema export requires explicit local file |
| Manifest Pretty / VerificationPretty | SummaryView; supplied presence/fresh claim/check vocabulary | Manifest ToJSON/WriteJSON/WriteVerificationJSON, version string `1`, fingerprints and loaders unchanged |
| CLI manifest generate/verify/diff/doctor | SummaryView or fixed skipped result | Source manifest/schema cannot be reconstructed from default JSON; original verification determines exit |
| CLI manifest repository | Fixed omission result; source output only in explicit local file | GenerateRepositorySkeleton remains an internal artifact API |
| MCP explain_query/review_query/generate_query_plan/compile_operation_spec | PlanView, fixed public errors | Read-only planning; no DB write or restored execution handle |
| MCP review_migration | MigrationPlanView | No apply tool |
| MCP get_schema/get_manifest/get_manifest_status | Same redacted resource views | No internal manifest/schema discovery or reusable source JSON |
| MCP schema/manifest/models/relations/policies/query-examples | Per-table structural count summaries, first 128 tables | All names, defaults, enum values, relations, policy targets, examples and fingerprints omitted |
| MCP manifest-status | Known/present booleans and supplied fresh claim/checks | Missing verification is unknown, never filled with fresh=true; fresh is not live evidence |
| MCP migrations/review-rules; tools/resources/prompts lists | Library-owned fixed capability descriptions, names/URIs and rule definitions | Caller allowlists only select fixed entries |
| MCP prompts | Fixed library text; caller model/method/SQL not interpolated | Arguments are not used for personalization |
| MCP propose_repository_method | Fixed FindRows teaching skeleton; no caller name | Not caller-specific code or execution permission |
| MCP generate_test_fixture | Fixed fictional teaching fixture; no caller/manifest dependency | Not a redacted execution plan or authorization |
| MCP JSON-RPC / Serve | Envelope/ID validation, bounded input, fixed errors, short-write detection | ProtocolVersion stays 2024-11-05; no batch support added |
| Examples quickstart/ai-safe-orm | Fixed error text, row counts, explicit PlanView/MigrationPlanView | Fictional source fixtures remain unchanged; values are not printed |
| Built-in MySQL automatic diagnostic logs | Per-connection fixed logger ignores payloads; no global SetLogger | Standard driver's connector, SQL/args/errors preserved; global custom logger is no longer inherited by Goquent's built-in MySQL opener |
| MySQL/PostgreSQL JSON predicate builder error logs | Fixed error text, no arbitrary Error formatting | Existing JSON binding and error behavior unchanged |
| CI test command / database failure logs | Fixed result/exit; ephemeral restricted test capture is deleted, never uploaded; raw DB logs are not dumped | Detailed CI failure diagnostics intentionally unavailable; reproduce locally, preserving test exit status |
| Internal profiling log | Elapsed execution duration only | No SQL/values/errors logged |

Default CLI JSON is intentionally **not** the old source wire. Pretty output for
manifest/migration is now the same compact, labeled JSON summary as its public
JSON output. Do not feed it to old loaders or rely on old headings. No unsafe
String/Pretty fallback was added. `WriteJSON` is not a public writer merely because
it is exported. Use public views explicitly in application publication code.

## Additional views and budgets

`migration.MigrationPlanView` (`goquent.migration_plan_view`, integer version 1)
contains fixed risk/precision, approval/blocked booleans, step/warning counts,
`StepView` entries and `query.WarningView` entries. Step types are the existing
built-in migration types; unknown types become `unknown`, never a safe verdict.
Steps and each warning list are capped at 128; root plus steps plus warnings are
capped at 1024 structural elements across the entire document. Counts refer to
source lists; omitted tails set truncation. All SQL, preflight strings, names,
Metadata and source objects are absent. Projection never walks opaque payloads.

`publicoutput.SummaryView` and `ItemView` serve these fixed kinds:
`goquent.manifest_view`, `goquent.schema_view`, `goquent.models_view`,
`goquent.relations_view`, `goquent.policies_view`, `goquent.query_examples_view`,
`goquent.verification_view`, `goquent.migration_status_view`, `goquent.drift_view`.
Resource items describe tables by local ordinal and child-list counts, not child
identities. Verification items allow only manifest/schema/policy/generated_code/
database check names and ok/stale/skipped statuses (otherwise unknown).
Missing verification is distinguished from an attached claim. Summary lists are
capped at 128 (129 structural elements including the root); no nested lists or
opaque fields exist. Drift reports expose drift/count only, not executable steps.

Both view families reapply allowlists and bounds in ToJSON/MarshalJSON/String/
Format and their public writers. `DecodeMigrationPlanView`,
`DecodeSummaryView` and UnmarshalJSON require exact supported kind and literal
integer version 1, reject unknown/uppercase/duplicate fields, nulls and trailing
JSON, with 1 MiB/depth 32 limits. These readers create no source/handle. Serializer
output stays below the reader size budget. Wrong kind/version and write failures
use fixed PUBLIC_VIEW errors, not original sink errors. No new value-derived hash,
caller classification callback or identifier declassification was added.

## Errors and MCP input

CLI parser output is discarded before fixed errors are printed, including unknown
flag names/values and paths. Public MCP direct methods and Serve return
`publicoutput.ErrOutput` or fixed protocol errors, without formatting or retaining
causes. Internal parse/compile/runtime errors still exist behind this boundary.

Direct HandleJSONRPC and both line/framed Serve input cap payloads at 1 MiB;
line/header accumulation also has a 1 MiB budget. JSON depth is at most 32 and
validation visits at most 16384 values. Duplicate or case-ambiguous keys and
unknown envelope fields are refused before dispatch. Only object params are
supported. Valid ID-less notifications produce no response; malformed envelopes
are refused. Strings in correlation IDs are UTF-8 with at most 1024 decoded bytes.
Numeric IDs are mathematical integers within ±9007199254740991, including valid
`1.0` and `1e3`, validated without float64 precision loss. Object/array/bool/null,
oversized or ambiguous IDs yield id:null and a fixed error without dispatch.

Direct tool/prompt argument maps accept only built-in JSON primitive/map/list
values, bounded by depth/value/encoded-byte budgets before generic serialization.
Unknown/custom types, cyclic/deep/huge data are refused without calling their
Valuer/Stringer/Marshaler/Error methods. json.Number lexemes remain intact. These
bounds may reject payloads smaller than 1 MiB when escaping or structural budgets
would exceed the bound. They do not impose new limits on internal execution APIs.

## Explicit local data export

Only `manifest` generation, `manifest repository` and `migrate schema` accept
`--unsafe-local-output <file>`. It is a command-line choice, never inherited from
configuration or an environment variable. stdout/stderr only report a safe
summary/result. MCP has no such mode. The target must be a new regular file,
created exclusively with 0600 permissions; existing files/symlinks, directories,
FIFO/device/stdio aliases and `-` are refused. Failure does not expose paths or OS
errors. A failed write can leave a partial sensitive file; no overwrite/retry
fallback is used. Choose a fresh path for a deliberate subsequent export.

CI, GITHUB_ACTIONS, GITLAB_CI, TF_BUILD, BUILDKITE, CIRCLECI, TRAVIS, JENKINS_URL
and TEAMCITY_VERSION markers refuse export when nonempty other than false/0.
This is defensive detection, not a security boundary against environment spoofing.
Do not save these files in public repositories or shared locations. Parent path
resolution and file modes do not prove physical confidentiality or prevent a
caller from publishing a file. CI and runnable examples use only public output;
tests exercise local mode with fictional data in temporary directories.

## Execution and acceptance limits

No runtime typed value is masked; no public-view-to-Raw/permit/ValidatedPlan API
exists. Current settings, inspection, private seals, key scope, expiry, CAS,
one-use, Strict rejection and ordinary CRUD/generic/scoped/RETURNING paths retain
their prior contracts. Existing unsupported binding families and external-effect
atomicity limits are unchanged. Source JSON, local artifacts and correlation IDs
are explicit revision-2 AC exceptions; claims of zero canary bytes apply to the
remaining public payload, not these exceptions. Tests/CI/PR creation are evidence,
not whole-Issue acceptance, merge, live database identity or user completion.

CI runs `sh scripts/test-public-output.sh`, which executes `go test ./... -count=1`
behind an ephemeral 0600 diagnostic capture. It publishes only a fixed pass/fail
message and preserves the test process exit status. It does not report test skip
counts. Local validation records count tests separately. No additional runtime or
production dependency is introduced; the wrapper uses POSIX shell/mktemp already
available on the CI runner. Raw database failure logs are no longer published.
