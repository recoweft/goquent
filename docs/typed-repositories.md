# Typed repository generation (GQ-AI-08 PR1)

Issue [#79](https://github.com/recoweft/goquent/issues/79), PR1 of 2.
This extends `manifest.GenerateRepositorySkeleton` and
`GenerateRepositorySkeletonForTable`. It adds models, nominal key inputs,
model-specific column predicates and ordering, named result projections, and an
application SELECT adapter. The dynamic APIs remain available.

## Opt-in and compatibility

`RepositorySkeletonOptions.Typed` defaults to false. Existing generation retains
its legacy signatures and type guesses. Set `Typed: true` to adopt the new
signatures explicitly. For the manifest entry, `Manifest.Dialect` must be mysql
or postgres; a conflicting options dialect refuses. The table-only entry requires
`RepositorySkeletonOptions.Dialect`. Missing/unknown dialects are not guessed.

Options add `Projections []RepositoryProjection`, whose `Name` and ordered
`Columns` declare a result. The CLI accepts `--typed` and repeated
`--projection Name=column1,column2` on all three existing aliases (`repository`,
`repo`, `skeleton`). Projection options without `--typed` refuse. There is no
implicit raw expression, join, aggregate, or star projection. Empty, duplicate,
undeclared, forbidden or colliding projection definitions refuse. An existing
column with an unknown type can be selected, but its later operation is blocked.

Use keyed option/Column literals: adding fields changes unkeyed struct literal
source compatibility. Existing function signatures and source manifest version
`"1"` remain; this is not a promise of complete source compatibility. Typed
regeneration changes generated Go types/methods and requires updating callers.
The old generator's `any` IDs, scopes, Insert and map update/delete helpers remain
in legacy mode. Typed mode adds read methods; it does not claim typed write inputs
or complete update patches.

Generation returns source bytes through the existing sensitive source API. CLI
stdout remains a fixed omission message. `--unsafe-local-output` saves only a new
local artifact with the existing O_EXCL/0600 checks and CI prohibition. Generate
to a new file and review its diff; handwritten files are never overwritten.
`ORMImportPath` still specifies an API-compatible ORM package and child packages;
this is not a mechanism for guessing arbitrary external import APIs.

Typed SQL targets currently use single ASCII identifiers (letters/underscore,
then letters/digits/underscore). SQL paths, punctuation, expressions, and other
identifier spellings refuse with `ErrRepositoryGeneration`. Go identifiers are
sanitized using the existing naming helpers; reserved field names and collisions
receive deterministic numeric suffixes. Conflicting top-level names refuse.
Generated fixtures exercise Go keyword, field, import and nominal-type collisions.
This limited identifier subset does not change dynamic Query naming support.

## Generated surface

For a `RecordRow`, the generator emits `RecordRepository`, `NewRecordRepository`,
`RecordRead`, `RecordPredicate`, `RecordOrder`, and `RecordColumns()`.
Each column has its own reference type. Its private token prevents an external
zero reference from becoming a valid target. Predicates/orders cannot be mixed
between models. Zero predicates/orders refuse through the existing validator.

Known scalar columns expose Eq/Ne/In, applicable comparisons, string Like, and
IsNull/IsNotNull. Unknown columns have ordering and NULL tests but no typed value
predicate. These still require runtime declaration checks. Tenant columns expose
only `CurrentTenant()` for predicates, resolving the reserved application binding;
a literal or values map cannot supply tenant authority. PII access reasons,
required filters and soft-delete handling remain in the shared validator/planner.

`RecordRead` supplies typed Filters, OrderBy, optional `*int64` Limit, and
AccessReason. Omitted and explicit zero limits remain different. The shared
input budgets and IN/limit constraints still apply. These APIs are a small typed
front end to single-model OperationSpec, not a new condition language.

A named Summary projection emits `RecordSummaryRow`, `PlanSummary` and
`SelectSummary`. The Plan method returns the existing plan, DiagnosticView, and
error, and never executes SQL. SELECT and scanner field order follow the supplied
projection order. The result uses literal db tags and the existing generic scanner;
`ApplyProjection` and its trusted RawSQL expression contract remain unchanged.

Known supplied primary columns emit dedicated defined column key types. A single
column's `RecordKey` is a dedicated defined scalar type, never an alias for a
built-in integer/string. Composite `RecordKey` is a struct of distinct component types,
sorted by the original SQL column name before Go-name conversion. All components
must have a supported scalar representation; unknown/forbidden key components
omit key operations. This order is an API/SQL determinism rule, not physical index
order. No new PrimaryKeyColumns wire field is introduced.

`FindByKey` and `PlanFindByKey` append these component predicates in that order.
They explicitly select all non-forbidden manifest columns into RecordRow, so an
unknown column in that row can block the operation; use a narrow named projection
when appropriate. Find returns the first scanned row, or `orm.ErrNotFound` for an
empty result. The slice-based adapter scans the result before this first-row
selection; it does not add a uniqueness, affected-row, or single-row SQL guarantee.
A declaration named id does not create a key. Zero scalar values remain valid
values, and omitted composite fields cannot be distinguished from explicit zero.

A tenant key component is an opaque marker obtained with
`RecordCurrentTenantKey()`, not a scalar supplied and ignored by the generator.
Its zero marker refuses. Actual tenant values always come from the current DB's
application context, in automatic and manual tenant modes.

Implicit assignment of another table's key, another component's key, a typed
built-in variable, another model's predicate/order, or a different enum type is a
compile error. Untyped constants and explicit Go conversions can still express
semantically incorrect inputs. Compile success is not authorization.

## Type and result contract

The generator and OperationSpec share the SQL declaration parser under
`orm/internal/sqltype`. Legacy substring mapping remains confined to legacy mode.
The validator's grammar, fixed errors, missing facts and diagnostic coverage are
unchanged. See [typed validation](typed-operation-validation.md).

| Declaration | Typed predicate input | Generated result |
| --- | --- | --- |
| Supported signed/unsigned SQL integers | Corresponding Go width/sign, including nominal PK types | Corresponding built-in integer |
| MySQL mediumint | int32/uint32; 24-bit bound checked at runtime | int32/uint32 |
| bool/boolean | bool | bool; nullable uses sql.NullBool |
| text/char/varchar/UUID | string | string |
| Supported decimal(p,s)/numeric(p,s) | json.Number, original lexeme | Exact driver text as string |
| Supported date/timestamp/datetime | Existing ISO/RFC3339 string subset | time.Time |
| Supported time | Existing clock string subset | Generated named TimeText Scanner |
| Explicit EnumValues on supported string type | Column-specific defined string, Choice1..N constants in supplied order | string |
| Unknown source/type, Go declarations, unsupported SQL types/arrays | No typed value predicate or guaranteed key | any |

Result and predicate representations intentionally differ. Enum constants use
ordinals so arbitrary member spellings cannot inject or collide as identifiers.
Membership, UUID syntax, character length, decimal precision/scale, temporal
validity/precision and driver/private binding constraints are runtime checks.
Nominal values and IN elements convert explicitly to their declared built-in
scalar before the validator. No arbitrary Valuer/Stringer/Marshaler/reflection
conversion is added; native widths, large integers and decimal lexemes survive.
Decimal inputs never change to string/float64 to evade private binding checks.

Nullable or nullability-unknown known result types use `sql.Null[T]`; bool uses
`sql.NullBool` to retain the generic scanner's BoolStrict/Compat/Lenient behavior.
Unknown nullability is never changed to a supplied nonnullable declaration.
IsNull/IsNotNull remain independently validated. This is result NULL handling,
not PR2's unchanged/NULL/value update state.

MySQL integration supplies parseTime=true. The built-in PostgreSQL driver returns
TIME as time.Time; MySQL returns clock text. Generated TimeText.Scan accepts
string/bytes unchanged and formats the PostgreSQL clock with nanosecond precision
without converting through a float. PostgreSQL's special 24:00 next-day value
refuses rather than silently becoming 00:00; unsupported scanner payloads refuse.
No driver argument adapter is added. Result text is not automatically valid as a
predicate: MySQL durations and other out-of-subset values remain outside input
validation. Temporal text/time.Time roundtrips, sessions/timezones, collations,
custom drivers' storage semantics and physical identity are not certified.

Fractional/exponent decimal private binding remains unavailable; compatibility
plans are blocked and Strict refuses. Unsupported arrays, unsigned driver ranges,
MySQL timestamp session semantics and unknown/native DB enum/domain declarations
remain unknown/refused as documented by GQ-AI-07. A column's typed generation does
not override these facts.

## Shared planning and application execution

`orm.SelectOperationBy[T](ctx, db, spec, opts)` returns a result slice and error.
It forces the actual DB's immutable Settings, Dialect and Executor, ignoring
caller attempts to replace those options. It is application-only SELECT;
Compile/Validate/CLI/MCP remain nonexecuting. No write spec or generic public
executor/callback/handle surface is added.

The shared compiler validates once, records diagnostics, constructs one Query,
finalizes it once, and gives its private sealed SELECT to the existing dispatcher.
It does not compile then rebuild a second Query, read public SQL/Params back into
Raw, or execute display output. Rejection and blocked plans dispatch zero calls.
The same generic result scanner retains DB bool policies and tags, transaction
ownership, registered drivers, context cancellation and existing error behavior.
Direct library execution/scanner errors retain their existing source error
contract; there is no new CLI/MCP execution route publishing them.

This immediate read retains the private owner/seal/context/gate/consumption path.
It does not claim the additional explicit current-input, expiry and CAS contract
of ValidateSelect/ExecuteValidatedSelect. Those APIs and their existing canonical
support limits remain unchanged. Public plan tampering cannot change captured
SQL/typed args, and a prepared private read cannot be consumed twice.

## Metadata and PR2 boundary

Source headers record fixed `typed-repository-v1`, supplied schema/policy
fingerprints, and snapshot kind `manifest` or `table`. Caller GeneratorVersion
cannot impersonate the implementation version. A table-only snapshot's locally
computed fingerprint does not fill a missing whole-manifest fingerprint.
No fresh generation timestamp is introduced. The private source snapshot preserves
TypeSource, NullableKnown, constraints and policy metadata and is detached anew
for each operation. These are supplied declarations, not live evidence or permits.

Manifest Column adds `Readonly` / `readonly,omitempty`. Explicit model db tags
readonly/generated and supplied Generated survive merging without weakening SQL
type/nullability/constraints. Both flags generate readonly row tags. No DDL,
introspection, or physical generated-column fact is inferred. Missing/false
readonly preserves old JSON; true changes schema fingerprints and is rejected by
older closed readers. Upgrade readers and regenerate/reverify supplied snapshots;
old fingerprints do not establish the new metadata guarantee.

Generated rows, keys, enum values, references, predicates, orders, read inputs,
repositories and clock types use fixed omitted String/Format/JSON output. Their
public source fields and explicit conversions are still application data, not a
security boundary against deliberate extraction or methodless casts. Artifacts
and existing source APIs remain sensitive. No new public fingerprint getter,
source error disclosure or CI artifact exception is introduced.

PR2 owns three-state update patches, exclusion of readonly/generated update
candidates, nonnullable SetNull rejection, regeneration CI and the completed
workflow. Existing dynamic map updates gain none of those guarantees here.
The version/header/snapshot and readonly metadata provide the extension points.
Fingerprint comparison and deterministic generator tests are not completed
regeneration CI. Resolving that CI workflow with local artifact restrictions
remains PR2 work. This PR does not close Issue #79.
