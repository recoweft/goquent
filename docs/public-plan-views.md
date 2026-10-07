# Redacted public views (GQ-AI-06 PR1)

PR2 update: [current output inventory and migration](redacted-output.md) supersedes
the historical PR2-work column and legacy-display statements below. PR1 view and
source JSON contracts remain unchanged.

[Issue #73](https://github.com/recoweft/goquent/issues/73), PR1 of 2. These additive
APIs implement the published revision-2 fixed-policy contract. CLI/CI/MCP output
switching belongs to PR2 after PR1 merges. Existing diagnostics can still expose
values. This PR does not establish whole-Issue redaction or change execution.

## API inventory and migration

| Package | New API | Contract |
| --- | --- | --- |
| query | `(*QueryPlan).PublicView() (PlanView, error)` | Accepts diagnostic version 0 (legacy) or 1; nil/other versions return a fixed source error |
| query | `PlanViewKind`, `PlanViewVersion`, `MaxPlanViewWarnings` | `goquent.plan_view`, integer 1, 128 per warning list |
| query | `PlanView`, `WarningView` | Detached display fields; no SQL, params, names, any payload, source-plan pointer or handle |
| query | `PlanView.ToJSON() ([]byte, error)`, `MarshalJSON() ([]byte, error)`, `String() string`, `Format(fmt.State, rune)` | Value receivers cover values and pointers; every provided output reapplies the fixed policy |
| query | `DecodePlanView([]byte) (PlanView, error)`, `(*PlanView).UnmarshalJSON([]byte) error` | Strict display-only reader; no execution conversion |
| query | `WarningView.MarshalJSON`, `String`, `Format` | Also sanitize individual elements outside their containing view |
| review | `(ReviewReport).PublicView() (ReportView, error)` | Accepts diagnostic version 0 or 1; does not read Evidence or Summary contents |
| review | `ReportViewKind`, `ReportViewVersion`, `MaxReportViewFindings` | `goquent.report_view`, integer 1, 256 per finding list |
| review | `ReportView`, `FindingView` | Detached display fields; no nested source plans or payload references |
| review | `ReportView.ToJSON`, `MarshalJSON`, `String`, `Format` | Same signatures and safe display rules as PlanView |
| review | `DecodeReportView([]byte) (ReportView, error)`, `(*ReportView).UnmarshalJSON([]byte) error` | Strict display-only reader |
| review | `FindingView.MarshalJSON`, `String`, `Format` | Also sanitize individual elements |
| review | `WritePublicJSON(io.Writer, ReportView) error`, `WritePublicPretty(io.Writer, ReportView) error`, `WritePublicGitHub(io.Writer, ReportView) error` | Require an explicit view; fixed errors, no source paths or arbitrary messages |

For a new public consumer, call `PublicView`, check its error, then call `ToJSON`
or a public writer. Keep the original Query/diagnostic/validated handle on the
execution side. Do not replace runtime arguments with view fields. The older
`QueryPlan.ToJSON/String` and `review.WriteJSON/WritePretty/WriteGitHub` retain
exactly their prior meanings in PR1. Their defaults are not safe public output.
No dependency, classification callback, identifier mapping or declassification
API is added. Internal display helpers import neither root ORM nor CLI.

## Fixed field policy

| Classification | Source fields | Projection |
| --- | --- | --- |
| structural | operation, risk, precision, built-in code | Exact fixed vocabulary; unknown becomes `unknown`, never low/precise |
| structural | RequiredApproval/Blocked; warning Suppressible/RequiresReason; finding Suppressed | Boolean claims, not authority |
| structural | Tables/Columns/Joins/Predicates list lengths; warning/finding list lengths | Nonnegative counts only; no recursive tree traversal |
| structural | warning/finding positions | Numeric Line/Column; local one-based ordinal regenerated within each output list |
| sensitive | SQL including generated SQL, table/column/alias/expression/function names, Params, typed values, tenant, ValueRef, NamedValues, predicate SQL/Raw/Values | Entirely omitted; no parsing, literal replacement or value-derived public hash |
| sensitive | approval/reason/suppression text, owner/time, Message/Hint, source/path/error/diagnostic strings | Omitted; displayed Message regenerated from fixed Code |
| opaque | Metadata, Evidence (including Key), nested plans, condition trees, tenant/write results, private execution/identity/key/current | Contents entirely omitted, without invoking Valuer/Stringer/Marshaler/error methods or application callbacks |
| unknown | custom enum/code/value and future fields | Unknown strings normalize to `unknown`; unknown payloads omitted |

`Limit`, `Offset`, `EstimatedRows`, timestamps and business values are not
structural counts and are omitted. `UsesIndex`, manifest status and source Summary
are also omitted. PII/Forbidden labels are not prerequisites for hiding values.
`DetailsOmitted` is always true, including after exported-field tampering. The
view is deliberately unsuitable for SQL/name debugging. Counts, booleans and
positions disclose structure; this is not zero-information output or protection
for a caller deliberately encoding secrets as numbers or allowed vocabulary.

PlanView exposes Operation/Risk/Precision, RequiredApproval/Blocked,
TableCount/ColumnCount/JoinCount/PredicateCount, WarningCount/
SuppressedWarningCount, Warnings/SuppressedWarnings, Truncated and DetailsOmitted,
in addition to Kind/Version. WarningView exposes Ordinal/Code/Level/Message/
Line/Column/Suppressible/RequiresReason. ReportView exposes FindingCount/
SuppressedFindingCount, Findings/SuppressedFindings, Truncated/DetailsOmitted and
Kind/Version. FindingView exposes Ordinal/Code/Level/Precision/Message/Line/Column/
Suppressed. JSON names use snake_case as declared in the source types.

The enum vocabularies are operations `select/insert/update/delete/raw`, risks
`low/medium/high/destructive/blocked`, precisions `precise/partial/unsupported`,
and the fallback `unknown`. For a recognized code, Message is the code followed
by `: diagnostic details omitted.`; otherwise it is
`Unknown diagnostic; details omitted.` Source text is never substituted.

### Fixed code vocabulary

The same vocabulary applies to plan warnings and report findings, including
review suppression lint, manifest, migration and OperationSpec codes. It is a
closed display list, not automatic registration of future/custom rules:

- `BULK_DELETE_DETECTED`
- `BULK_UPDATE_DETECTED`
- `DELETE_WITHOUT_WHERE`
- `DESTRUCTIVE_SQL_DETECTED`
- `LIMIT_MISSING`
- `MANIFEST_REQUIRED`
- `MANIFEST_STALE`
- `MANIFEST_UNREADABLE`
- `MANIFEST_UNVERIFIED`
- `MIGRATION_ADD_INDEX_NON_CONCURRENT`
- `MIGRATION_ADD_NOT_NULL_COLUMN`
- `MIGRATION_ALTER_COLUMN_TYPE`
- `MIGRATION_BACKFILL_REVIEW`
- `MIGRATION_DROP_COLUMN`
- `MIGRATION_DROP_INDEX`
- `MIGRATION_DROP_TABLE`
- `MIGRATION_RENAME_COLUMN`
- `MIGRATION_SET_NOT_NULL`
- `MIGRATION_TYPE_NARROWING`
- `MIGRATION_UNSUPPORTED`
- `OPERATION_SPEC_PII_SELECTED`
- `OPERATION_SPEC_REQUIRED_FILTER_MISSING`
- `PII_COLUMN_SELECTED`
- `RAW_SQL_USED`
- `REQUIRED_FILTER_MISSING`
- `REQUIRED_PREDICATE_MISSING`
- `SELECT_STAR_USED`
- `SOFT_DELETE_FILTER_MISSING`
- `STATIC_REVIEW_PARTIAL`
- `STATIC_REVIEW_UNSUPPORTED`
- `SUPPRESSION_CONFIG_INVALID`
- `SUPPRESSION_EXPIRED`
- `SUPPRESSION_NOT_ALLOWED`
- `SUPPRESSION_OVERBROAD`
- `SUPPRESSION_OWNER_MISSING`
- `SUPPRESSION_REASON_WEAK`
- `SUPPRESSION_UNUSED`
- `TENANT_FILTER_MISSING`
- `TENANT_POLICY_REJECTED`
- `UNSUPPORTED_RISK_ENGINE`
- `UPDATE_WITHOUT_WHERE`
- `WEAK_PREDICATE`

## Bounds, serialization and errors

Projection processes at most 128 active plus 128 suppressed plan warnings, or
256 findings plus 256 suppressed report findings. It uses slice lengths for
whole-list counts and never walks opaque payloads (including cyclic, deep or huge
Metadata/Evidence). Each list's ordinal is independent. Counts refer to the
entire corresponding source list; they do not deduplicate the overlap produced
by review ShowSuppressed. There is no copied ByLevel map or misleading aggregate
highest risk. Truncated is true when any source list was cut, and is preserved
on subsequent serialization. Modified counts are raised to at least supplied
list lengths; a missing tail also marks truncation. Serializers bound modified
arrays again. Pretty/GitHub output always labels omitted details and truncation,
including empty reports; it never says "No findings" for an incomplete report.
GitHub annotations omit paths and render unknown risk as an error annotation;
this is display, not a new CI threshold or approval rule.

New readers require exact kind and literal integer version `1`. Missing/0/unknown,
string/fraction/exponent/null versions, wrong kinds, malformed types, duplicate
keys, uppercase keys, unknown fields, null fields and trailing JSON documents
are rejected. Input is bounded to 1 MiB and nesting to 32. Integer fields decode
directly to native int, with overflow/fractions rejected; no float64 conversion.
New readers do not accept old diagnostic JSON. Old missing/0 diagnostic envelopes
can still be projected after the existing reader accepts them. Existing
UseNumber/native-width/JSON-lexeme and four-envelope/manifest contracts remain
unchanged. The new kinds are unrelated to manifest string version "1".

Dedicated decoders always return a fresh zero view on error. Method-level
UnmarshalJSON clears its receiver first; direct encoding/json can reject syntax
before calling a method, so external inputs should use the dedicated decoder,
or a fresh receiver discarded on error. No alias-wide clearing is promised.
ToJSON/MarshalJSON reject wrong Kind/Version rather than laundering them. The
zero view is not a valid wire object. Standard encoding/json's nil-pointer rule
can produce `null`; a dedicated reader rejects it.

PublicView returns only `PUBLIC_VIEW_SOURCE: unsupported diagnostic source` on
invalid sources. Dedicated decode/serialize and invalid-view writers return
`PUBLIC_VIEW_INVALID: invalid public view`. Sink errors and short writes become
`PUBLIC_VIEW_WRITE: output failed`; original errors are not wrapped or retained.
String/Format return the fixed invalid message when serialization fails. An
io.Writer is intentionally called to deliver output; its returned errors are
sanitized, not its panics or independent side effects. Similarly, standard
encoding/json and fmt used directly may produce their own library errors; these
are not promised to be fixed public error messages. Use dedicated entry points
for the fixed-error guarantee.

Value/pointer JSON, nested view JSON, String and fmt (including `%+v`/`%#v`) are
filtered, even after strings in exported fields are modified. The guarantee does
not extend to caller code extracting fields, casting to a distinct methodless
type, formatting arbitrary old source structs, or adding arbitrary sibling
payloads around a view. Concurrent caller mutation is not supported.

## Execution separation

Projection reads only the documented source fields and allocates independent
bounded output slices. It does not mutate QueryPlan, Query, typed args, conditions,
inspection, Settings/current, seals, gate, canonical identity or ValidatedPlan.
There is no view-to-Raw/ValidatedPlan/permit API. Feeding its JSON to an old
permissive diagnostic decoder cannot restore private evidence, SQL or args.
Formatting equality is neither execution equality nor authorization.

The existing private plannedExecution still dispatches captured SQL and ordered
typed args; validated binding still performs its existing current-input/fresh
reinspection and single-attempt checks. No second SQL generation is introduced
by views. Strict, expiry, owner/context/executor, tenant/key/scope-generation,
CRUD, external transaction and scanning contracts are unchanged. RiskLow,
precise, reasons, column names and manifest Fresh are not permission. Physical
identity, caller truth, live DB state and external-effect atomicity remain outside
these guarantees.

## Output inventory and PR2 work

| Current source/route | Existing exposure | PR1 treatment | PR2 migration |
| --- | --- | --- | --- |
| `orm/query/plan.go`: ToJSON, MarshalJSON, String; Query Build/Dump/RawSQL | SQL, params, names, predicates, Metadata, reasons and warnings; RawSQL interpolates | Add separate PlanView; legacy behavior retained | Switch public consumers; decide explicit local debug handling without CI/MCP inheritance |
| `orm/predicate`: Values/NamedValues/Raw/SQL/ValueColumn; operation FilterSpec.ValueRef and Options.Values | Bound/reference values and names; opaque wrappers are not a universal redaction boundary | Entire predicate/Evidence/Metadata payloads omitted from views | Audit every direct source/condition serializer |
| `orm/review/format.go`: JSON/pretty/GitHub; ReviewReport Findings/SuppressedFindings | Arbitrary Message/Hint/path/suppression/Evidence, including nested plans; arbitrary Summary keys | Separate ReportView and public writers; old writers unchanged | Replace CLI/CI writers, review input/error output; preserve risk/precision decisions using source report |
| `cmd/goquent/main.go`: review formats, thresholds, stderr | Old writers and raw errors/paths | Inventory only | Select public writers, sanitize errors independently; never use truncated view for CI decisions |
| `cmd/goquent/operation.go` | Plan.ToJSON/String; model/field/ValueRef/access reasons and parser/compile errors | New view available for compiled plans; tested Values/ValueRef omission | Switch output plus dedicated safe error adapter |
| `orm/mcp/server.go`: explain_query/review_query/generate_query_plan/compile_operation_spec | Plan.ToJSON and free-form tool/input errors | Inventory only; still read-only | Project plan outputs and sanitize protocol errors |
| MCP resources get_schema/get_manifest/status/models/relations/policies/query-examples; prompts | Manifest names/defaults/enums/query examples/fingerprints, path/errors, caller model names | Inventory only | Define safe resource projections; audit prompt interpolation and error builders |
| MCP propose_repository_method / generate_test_fixture | Interpolated method name; current fixture has static schema/model/ValueRef text | Inventory only; fixture is not permission or executable view | Review generated content, fixture values and errors; no claim all generated output is safe |
| `orm/manifest/manifest.go`, CLI manifest/doctor | Full models/columns/defaults/enums/PII metadata, fingerprints, verification paths/messages | Manifest wire/fingerprints retained; no new value hash; manifest codes allowed in views | Audit public manifest/context/error exports separately |
| migration plan/String/JSON, CLI migrate and MCP review_migration | SQL/steps/reasons/source/error text | Existing APIs unchanged; report finding codes can be shown safely | Separate migration/public output treatment; no parser expansion or apply change in PR1 |
| generated code, examples/fixtures, docs, runtime/driver errors | Names, values, source/DSN/error strings can escape independent of Plan/Report | Existing fixtures preserved; new fictional canary tests exercise new routes | Complete all-route fixed-canary regression; sanitize publication/CI logs without printing real credentials |

The inventory distinguishes located paths from proof of complete coverage. PR1
has no runtime error adapter for all DB/CLI/MCP consumers. Legacy outputs in the
compatibility regression deliberately still contain the fictional canary. No
production secret is used in tests. All-Issue acceptance remains pending PR2.

## Regression evidence

`orm/query/public_view_test.go` covers SQL/name/value/opaque omission, callbacks,
cycles/depth/large payloads, bounded lists, fmt and nested JSON tampering, legacy
projection/JSON numeric lexemes, strict wire failures and private seal dispatch.
`orm/review/public_view_test.go` covers nested evidence, summary/path/suppression,
all public writers, I/O failure/short writes, vocabularies and bounds.
`tests/contracts/testdata/public_views_v1.json` is shared accepted/rejected wire
input; `tests/contracts/public_views_test.go` covers OperationSpec value references,
nested report plans, unchanged legacy exposure and handle reconstruction refusal.
Existing binding six-method tests now exercise view operations before dispatch;
both-DB binding integration projects/mutates views before every operation and
external Tx execution, checks stored fictional string values and rollback/bool
scanning. Full-suite results, static review deltas and CI are recorded separately
in the validation record and PR; test descriptions alone are not execution evidence.
