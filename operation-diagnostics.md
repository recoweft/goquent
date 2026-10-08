# Structured operation diagnostics (GQ-AI-07 PR2)

Issue [#76](https://github.com/recoweft/goquent/issues/76), PR2 of 2.
PR1 [#77](https://github.com/recoweft/goquent/pull/77) merged at
471b24a69282668744fe3f035da435ff4399d250. This change keeps single-model,
read-only SELECT and the existing typed validator, settings, gates and planner.
It adds no SQL execution, write spec, join, driver adapter or dependency.
See the [validation record](operation-diagnostics-validation.md) for measured results.

## API and single-pass behavior

| Entry | Result |
| --- | --- |
| operation.CompileWithDiagnostics(ctx, spec, opts) | (*query.QueryPlan, DiagnosticView, error) |
| operation.ValidateWithDiagnostics(spec, opts) | ([]query.Warning, DiagnosticView, error) |
| orm.CompileOperationSpecWithDiagnostics / ValidateOperationSpecWithDiagnostics | Corresponding free wrappers |
| DB.CompileOperationWithDiagnostics / ValidateOperationWithDiagnostics | Same results, with DB Settings and Dialect forced |

Existing Compile/Validate/free/DB signatures and known errors.Is identities
remain. Missing required-filter diagnostics now use sorted field order rather
than map iteration, without changing acceptance. New private failure wrappers
use fixed Error/Format text; callers that parsed old detailed error strings must
migrate to errors.Is and the structured code. Each entry makes one call through
the same compiler. Validate includes
the existing DB-free planning gates. Diagnostic collection does not recompile,
render another SQL statement, query a DB or call value/error methods. Original
SQL and ordered native/JSON-number arguments are retained, including LimitExact(0).

The view is available on both success and rejection. Inspect the returned error
and original plan for the existing semantics; never use a display field to grant
permission. Existing internal errors may be unwrapped by library callers.
The new private failure wrapper formats as a fixed message even with %#v; it
retains known error identities and private history, not a public cause adapter.

Internal history is accessible only inside operation/querybridge/query. There
is no new sensitive getter, diagnostic source JSON, unsafe mode, value hash,
execution conversion or local artifact exception. Existing source data APIs
remain sensitive and retain their separate contracts.

## Internal facts and public omissions

Internal records retain actual table/qualifier/field, logical position, supplied
type declaration/source, width/sign/precision/scale/length/fraction, nullability
presence, input value/ref presence, checked constraint, missing fact, evidence
basis and fixed repair action. They do not copy argument values, enum member
values, reasons, errors or arbitrary metadata. A qualifier exists only when
actually supplied. Records stop at the first refusal; later checks are not
reported as completed. Declared facts are never live observations.

| Internal information | Public representation |
| --- | --- |
| Input budget, operator, type/nullability/enum checks | Fixed CHECK code and checked status; no source values |
| Declaration, target table/qualifier/field, constraint parameters | Omitted; logical section and indexes only |
| Missing type/source/nullability/dialect or unsupported declaration | TYPE_UNVERIFIED; missing reason and supply-supported-declarations action remain internal |
| Value does not fit declared scalar constraints | TYPE_MISMATCH; declared constraints and supply-matching-value action remain internal |
| IN arity or value/ref presence conflict | FILTER_INVALID / ARITY_INVALID; position without actual payload |
| Absent referenced value | VALUE_REF_MISSING; reference text omitted |
| Reserved application binding misuse/missing context | RESERVED_BINDING; trusted-context repair action, name and value omitted |
| Unsupported array binding | ARRAY_UNSUPPORTED; use-supported-scalar action retained internally |
| Private fractional/exponent numeric domain | UNVERIFIED_BINDING, separately from successful decimal validation |
| Unknown dialect or unsupported builtin driver unsigned domain | UNVERIFIED_DRIVER, separately from type validation |
| Session/range/collation/live constraints/physical identity | UNVERIFIED_LIVE general limitation; no new refusal gate |
| Arbitrary planner/settings error | Fixed PLANNER_REFUSED / SETTINGS_INVALID; original error not evaluated for classification |
| Warnings/reasons/metadata/evidence | Only the existing fixed warning-code vocabulary; all source strings omitted |

CHECK_TYPE for select/order checks declaration grammar only. For scalar predicates
it means the implemented scalar constraints completed; CHECK_ENUM additionally
means exact membership in the supplied enum was checked. CHECK_NULLABILITY
means supplied SQL nullability presence, not the result of a NULL predicate.
CHECK_BINDING covers only the existing numeric canonical exclusion and builtin
driver unsigned-domain check. It is not a ValidatedPlan or proof of all driver,
session or private execution conditions. See [typed validation](typed-operation-validation.md)
for the precise subset.

## Independent public wire

DiagnosticView uses kind goquent.operation_diagnostics and literal integer
version 1. It does not alter PlanView's existing kind/version/reader.

Root fields: kind, version, outcome, coverage, diagnostic_count, diagnostics,
truncated, details_omitted, and optional plan. Only a compiled result includes
the existing detached PlanView. Rejected/unknown outcomes omit plan; null is
not a valid supplied field. Compiled includes plans with Blocked=true.

| Coverage | Meaning |
| --- | --- |
| checked_subset | Compilation completed without relevant unknowns from the shared declared-type validator; live and private execution guarantees remain absent |
| partial | A relevant type/driver/binding fact was unknown, or rejection occurred after typed inspection was reached |
| unavailable | Rejection occurred before typed inspection |
| unknown | Unrecognized supplied display classification |

Plan precision is a separate planner claim. Neither precise nor checked_subset
is full type/DB safety, authorization, freshness or a CI verdict. General live
uncertainty does not change any existing gate.

Entry fields: ordinal, code, status, message, location. Status is one of checked,
refused, unverified, warning, unknown. Codes are closed below. Message is always
CODE followed by ": diagnostic details omitted.", or
"Unknown diagnostic; details omitted." Unknown codes also force unknown status.
No caller string is interpolated. Unknown future data is unknown/omitted.
CHECK codes permit checked only; UNVERIFIED codes permit unverified only;
TYPE_UNVERIFIED permits unverified/refused/warning; inherited warning codes
permit warning only; other OPERATION codes permit refused only. A mismatched
status normalizes to unknown. These display claims still confer no authority.

Location fields: source (spec/values/manifest/settings/planner/unknown),
section (root/version/operation/model/select/filters/order_by/limit/implicit/unknown),
member (field/op/value/value_ref/direction/unknown), index_known/index,
element_known/element, origin (json/go/implicit/unknown).
Indexes are zero-based logical list and IN-element positions, never byte/line/
column offsets. Unknown/negative indexes normalize to false/zero. The combined input-budget failure has unknown source because it cannot identify
which part of the spec+values wrapper caused the excess. Values-map
keys and implicit predicates have no fabricated list position. No raw path,
table/field name, limit value, tenant or ValueRef text appears.

Origin labels the acquisition route, not trusted provenance. Decoded source
objects retain JSON acquisition even if later modified; locations always refer
to their current logical structure, not immutable original source spans. Direct
MCP maps are labeled go after structural decoding; JSON-RPC maps are labeled
json through a private adapter. JSON re-encoding does not coerce native values.
Manifest/settings/planner source locations have unknown origin. Implicit
predicates use implicit origin.

## Ordering, bounds and independent readers

Internal records retain validation order. Projection moves the first refusal to
the front, then keeps the other records in their original order, at most 128.
Diagnostic_count is the full pre-projection count. Shared validation warnings
keep each occurrence. A planner warning whose code was already recorded as a
validation warning is not duplicated; other planner warnings keep their source
occurrences. A type-unverified record and a planner warning describe different
checks and may have the same code with different statuses.

The structural budget counts root=1, each entry=1, each location=1, optional
plan=1, each plan warning=1. Plan warning lists each keep their existing 128 cap:
normalized output has at most 514 structural elements, below the 1024 ceiling.
Readers enforce the ceiling on raw lists before nested truncation, plus 1 MiB,
depth 32 and 16384 JSON value nodes. Scalar fields count as nodes, not structural
objects. Counts preserve omitted tails, truncated is sticky, details_omitted is
always true. Exported-field tampering is normalized on every output. Original
refusal/Strict/exit and CI decisions never consult the bounded view.

ToJSON/MarshalJSON/String/Format and entry/location standalone formatting apply
the same policy for values, pointers and nested objects. WriteDiagnosticJSON and
WriteDiagnosticPretty use fixed errors. DecodeDiagnosticView requires exact kind
and literal version 1 and rejects unknown/case-variant/duplicate fields, nulls,
bad types, trailing documents and excessive budgets. Unknown enum/code strings
normalize safely. Missing legacy versions are not accepted for this new wire.
Source OperationSpec missing/0/1 version compatibility remains separate.

Wrong kind/version/serialization/decoding uses PUBLIC_VIEW_INVALID; sink errors
and short writes use PUBLIC_VIEW_WRITE without retaining causes. Dedicated
decode returns a fresh zero view on error; method decode clears the visited
receiver. The standard JSON decoder can reject syntax before invoking a method,
so use a fresh receiver or the dedicated decoder. No alias-wide invalidation is
promised. Standard-library wrappers can create their own errors; use dedicated
entry points for fixed errors. Nil-pointer JSON's standard null behavior is not
accepted by the reader. Concurrent caller mutation, methodless casts, field
extraction, arbitrary sibling payloads and sink panics/side effects remain outside
the existing display guarantee.

## CLI and MCP migration

CLI operation compile success stdout changes from PlanView to DiagnosticView.
Compile refusal stderr now carries the same diagnostic envelope. JSON is a JSON
document; pretty uses a fixed label and the same JSON information. Exit 0/1/2
retains success/compile-refusal/input-or-output-failure meanings. Precompile
parser/file/decode failures retain fixed generic messages and their existing
exit categories, without pretending to know typed failure positions. A failed
writer uses a fixed fallback and never retries raw diagnostics.

MCP compile_operation_spec and the Spec branch of generate_query_plan use the
same view. Direct compilation refusal returns IsError=true, a safe envelope and
non-nil fixed ErrOutput. A private result adapter retains only library-generated
public operation diagnostics across CallTool and JSON-RPC tools/call. It rebuilds
content from that view and never trusts arbitrary error-associated ToolResult
text. Invalid adapter data/serialization and unrelated tool errors fall back to
the existing fixed error. JSON-RPC and line/framed Serve retain the same result;
Serve outputs framed responses for either input style. Raw SQL planning remains
PlanView. Transport/decode failures retain existing fixed protocol errors.
Only the validated top-level RPC id has the existing correlation exception.

Consumers must dispatch on the new kind and unwrap optional plan for old
PlanView display logic. Refusal consumers must handle the structured stderr/tool
payload while retaining exit/IsError checks. No source loader, other command,
public PlanView contract or database write endpoint changes.

## Schema and fixture scope

The Schema describes canonical generated input, not every legacy input and not
execution permission. The shared operation_diagnostics_v1.json fixture covers
positive/negative/boundary cases for missing vs NULL, value/ref conflict, scalar,
operators and aliases, IN 0/1/1000/1001 and NULL elements, limit -1/0/10000/10001,
required targets, type constraints, reserved bindings and unknown declarations.
CLI JSON/pretty, API/root/Validate, MCP direct/RPC and both Serve input formats
run these same cases. The test-only schema interpreter evaluates the vocabulary
actually emitted and panics on unimplemented keywords; it is not a general JSON
Schema conformance certification.

Legacy empty/missing operation and normalized operators/directions may compile
outside that canonical Schema. JSON Schema's mathematical integer differs from
literal version and numeric lexeme rules; duplicate keys disappear from its data
model. The separate wire-difference fixture records these cases. Schema does not
know manifest-dependent type/forbidden fields, value-ref resolution, trusted
settings, native Go scalar identity, byte/depth/node budgets or driver/private
binding availability. Passing Schema does not imply compile success; compiled
does not imply execution permission. Native Go floats are not coerced into exact
integer input by MCP structural serialization.

No PR1 precision, NULL presence, manifest fingerprint, reserved-context,
LimitExact, private binding/Strict, executor/transaction/scanning/bool guarantee
is weakened. Supplied schema/policy comparison is not live freshness; generated
code/database checks can remain skipped. Tests and PR creation do not close
the two-PR Issue or establish physical identity or external-effect atomicity.

## Closed code catalog

Operation codes (exact spelling):

- OPERATION_INPUT_INVALID
- OPERATION_INPUT_LIMIT
- OPERATION_VERSION_UNSUPPORTED
- OPERATION_MANIFEST_REQUIRED
- OPERATION_MANIFEST_INVALID
- OPERATION_MANIFEST_STALE
- OPERATION_UNSUPPORTED
- OPERATION_MODEL_REQUIRED
- OPERATION_MODEL_UNKNOWN
- OPERATION_SELECT_REQUIRED
- OPERATION_FIELD_UNKNOWN
- OPERATION_FIELD_FORBIDDEN
- OPERATION_FILTER_INVALID
- OPERATION_OPERATOR_UNSUPPORTED
- OPERATION_ARITY_INVALID
- OPERATION_ORDER_INVALID
- OPERATION_VALUE_REF_MISSING
- OPERATION_REQUIRED_FILTER_MISSING
- OPERATION_ACCESS_REASON_REQUIRED
- OPERATION_TYPE_MISMATCH
- OPERATION_TYPE_UNVERIFIED
- OPERATION_ARRAY_UNSUPPORTED
- OPERATION_RESERVED_BINDING
- OPERATION_SETTINGS_INVALID
- OPERATION_PLANNER_REFUSED
- OPERATION_CHECK_TYPE
- OPERATION_CHECK_NULLABILITY
- OPERATION_CHECK_ENUM
- OPERATION_CHECK_INPUT
- OPERATION_CHECK_OPERATOR
- OPERATION_CHECK_BINDING
- OPERATION_UNVERIFIED_DRIVER
- OPERATION_UNVERIFIED_BINDING
- OPERATION_UNVERIFIED_LIVE

Inherited warning codes (unchanged PlanView allowlist):

- BULK_DELETE_DETECTED
- BULK_UPDATE_DETECTED
- DELETE_WITHOUT_WHERE
- DESTRUCTIVE_SQL_DETECTED
- LIMIT_MISSING
- MANIFEST_REQUIRED
- MANIFEST_STALE
- MANIFEST_UNREADABLE
- MANIFEST_UNVERIFIED
- MIGRATION_ADD_INDEX_NON_CONCURRENT
- MIGRATION_ADD_NOT_NULL_COLUMN
- MIGRATION_ALTER_COLUMN_TYPE
- MIGRATION_BACKFILL_REVIEW
- MIGRATION_DROP_COLUMN
- MIGRATION_DROP_INDEX
- MIGRATION_DROP_TABLE
- MIGRATION_RENAME_COLUMN
- MIGRATION_SET_NOT_NULL
- MIGRATION_TYPE_NARROWING
- MIGRATION_UNSUPPORTED
- OPERATION_SPEC_PII_SELECTED
- OPERATION_SPEC_REQUIRED_FILTER_MISSING
- PII_COLUMN_SELECTED
- RAW_SQL_USED
- REQUIRED_FILTER_MISSING
- REQUIRED_PREDICATE_MISSING
- SELECT_STAR_USED
- SOFT_DELETE_FILTER_MISSING
- STATIC_REVIEW_PARTIAL
- STATIC_REVIEW_UNSUPPORTED
- SUPPRESSION_CONFIG_INVALID
- SUPPRESSION_EXPIRED
- SUPPRESSION_NOT_ALLOWED
- SUPPRESSION_OVERBROAD
- SUPPRESSION_OWNER_MISSING
- SUPPRESSION_REASON_WEAK
- SUPPRESSION_UNUSED
- TENANT_FILTER_MISSING
- TENANT_POLICY_REJECTED
- UNSUPPORTED_RISK_ENGINE
- UPDATE_WITHOUT_WHERE
- WEAK_PREDICATE
