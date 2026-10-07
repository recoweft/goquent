# Typed read-only operations (GQ-AI-07 PR1)

OperationSpec remains a single-model SELECT language. It does not accept joins,
raw SQL, writes, or an execution permission. `operation.Validate` and `Compile`
share the same DB-free validator and planner. Detailed public diagnostic locations,
evidence and repair suggestions belong to PR2.

## Application entry points

`operation.Options.Settings` is an optional `*query.Settings`. Nil snapshots the
legacy defaults once per call; a pointer to zero Settings imports no defaults.
`DB.CompileOperation(ctx, spec, opts)` and `DB.ValidateOperation(spec, opts)` always
use that DB's current immutable Settings and Dialect, overriding caller options.
The existing free functions and root aliases remain available. None executes SQL
or obtains a ValidatedPlan. Lower-level packages do not import the ORM facade.

`Settings.IsStrict()` reports the existing opt-in profile.
`Settings.ApplicationTenantContext()` returns the existing application context
only when private application provenance, tenant presence and a non-nil value are
present. Its Input getter detaches data. These getters do not authenticate a caller,
attest a database, or authorize execution.

A tenant column must agree between manifest policy metadata and Settings policy.
Only equality (`=` or `eq`) to the exact reserved reference `current_tenant` is
accepted. The value comes from the application context, never Values. A Values
key that equals this reference after case/whitespace normalization is rejected,
even if unused or equal to the application value. Tenant literals, other refs,
IN and null predicates cannot supply tenant authority. Missing private context
refuses in both profiles. Automatic tenant settings reuse the planner's mandatory
predicate; manual mode requires the explicit reserved equality. Application
schema, dialect, database, typed tenant, final inspection and Strict gates still
apply. An auto predicate does not rescue a forbidden literal.

CLI and MCP gain no trusted settings input. Their old tenant JSON examples now
refuse. The runnable `examples/ai-safe-orm` example uses an explicit application
schema, signed integer tenant and DB settings. Its checked-in legacy JSON files
remain unchanged as refusal/migration fixtures. A reason, Fresh, low risk,
public diagnostic or matching Values entry never creates application provenance.

## Input contract

The logical input is `{"spec": ..., "values": ...}` with a shared budget of
1,048,576 UTF-8 bytes, depth 32 (wrapper root = 0), and 16,384 value nodes including
containers. Keys consume bytes but not nodes. Settings and manifest declarations
are outside this request budget. Raw JSON is also byte bounded. CLI files share
one raw wrapper budget. Spec method-level decoding retains the bytes it receives. The standard JSON
decoder removes outer whitespace before calling that method, and a Go map has no
recoverable pre-decode whitespace history. Callers using external JSON decoders
must bound their entire raw document as well as the shared logical input. Library
CLI and MCP entry points bound the full received input.

The library counts its own representation before inspecting values: nil, bool,
valid UTF-8 string, json.Number, built-in integer/unsigned/finite float types,
map[string]any and slices of built-in bool/string/numeric types or []any. Opaque named
values, pointers, structs, custom Valuer/Marshaler/Stringer/Error implementations,
cycles and excessive nesting refuse without calling methods. Slices (including
[]byte) count as logical lists; nil stays nil. Accepted SQL arguments retain the
original Go scalar type and json.Number lexeme. Accepting a value into the input
budget does not make it valid for a column. In particular floats never satisfy
an integer or decimal column. MCP's existing envelope/direct-primitive restrictions
remain separate and can reject an input that fits the operation budget. Direct
MCP numeric arguments are not promoted to exact integers by JSON serialization.

Duplicate and case-ambiguous keys, trailing JSON and unknown filter/order fields
refuse. `FilterSpec.ValuePresent` is a Go-only presence flag: nil plus true means
explicit NULL; nil plus false means missing. Non-nil false, zero, empty strings
and empty lists are present. JSON records `value` and `value_ref` presence even
for null/empty declarations; both at once refuse. Missing fields remain missing
on source serialization. An empty ref is invalid. `is_null` and `is_not_null`
take no value/ref; all other operators take exactly one. Resolved nil and NULL
IN elements refuse. IN is a scalar set of 1..1000 values, not array-column binding.

Operation limit is an integer in 0..10000. Omitted limit stays omitted and retains
the missing-limit warning. Zero renders LIMIT 0; it does not bypass policy or DB
execution gates. Select/filter/order counts have no separate limit beyond the
shared budget. JSON Schema describes the limit, arity and literal IN bounds;
byte/depth, resolved refs and manifest-dependent checks require the validator.
Legacy source version 0/missing and version 1 remain supported. Existing operator
aliases/normalization remain compatibility inputs; the Schema lists canonical
spellings and aliases. Schema alone is not a complete validation oracle.

## Type declarations and exactness

`manifest.Column.TypeSource` is a string: `sql` or `go`. Absence is unknown;
explicit JSON empty/null, other spellings and invalid Go nonempty values refuse.
It declares the namespace of Type, not live evidence or a security verdict.
Known built-in dialects and matching manifest declarations are required to check
SQL types. Unknown or conflicting dialects are not verified as MySQL.

| SQL type grammar | Checked input |
| --- | --- |
| MySQL tinyint, smallint, mediumint, int/integer, bigint, optionally ` unsigned` | Built-in int/uint or JSON integer lexeme, exact declared sign/width |
| PostgreSQL smallint/int2, integer/int/int4, bigint/int8 | Exact signed 16/32/64-bit integer |
| decimal(p,s), numeric(p,s) | Built-in integer or json.Number, exact decimal fit without rounding; p > 0, 0 <= s <= p; MySQL p <= 65 and s <= 30; PostgreSQL p <= 1000 |
| char(n), varchar(n), text | Valid UTF-8 string; declared n counts Unicode codepoints |
| bool, boolean | Go/JSON boolean; equality, inequality and IN |
| PostgreSQL uuid | 36-character hyphenated hexadecimal string, either letter case |
| date | YYYY-MM-DD and valid calendar date; positive four-digit year, MySQL year >= 1000 |
| time[(f)] | HH:MM:SS[.fraction], f <= 6, no duration/24h/leap-second forms |
| timestamp[(f)] [with/without time zone] (PostgreSQL) | RFC3339 for with-zone, otherwise YYYY-MM-DDTHH:MM:SS[.fraction] |
| datetime[(f)] (MySQL) | Timezone-free ISO date/time subset |
| timestamp[(f)] (MySQL) | Syntax inspected; session/range semantics remain unverified |
| PostgreSQL element[] | One-dimensional type recognition; value-based array-column compilation always returns ErrArrayBinding |

Type names are case-insensitive; whitespace and modifiers outside the listed
forms are not inferred. Defaults for temporal fraction precision are MySQL 0 and
PostgreSQL 6. LIKE requires a known string type. EnumValues are exact string
matches without trimming/case folding. NULL predicates need explicit SQL
nullability information in Strict. A nonnullable column can still be tested with
IS NULL; the compiler does not synthesize a result.

No Go-to-SQL width mapping is inferred. PostgreSQL SQL int8 means 64 bits; Go int8
is a Go declaration with unknown DB width. Go int/uint/string, unknown aliases,
float/binary/JSON types, PG unsigned/domains/custom enums, tinyint(1) as bool,
complex modifiers, unconstrained numeric and nonportable scales remain unverified.
Multi-dimensional/custom-element/NULL-element array binding is not introduced.
Integer 1.0/1e3, numeric strings and float32/64 refuse for integer columns.
Decimal exponent checks do not expand powers of ten; redundant zeroes may be
accepted when the value is unchanged. NaN/Infinity refuse.

Primitive validation, driver binding, actual DB comparison/storage, and private
binding availability are separate facts. There is no CAST injection, decimal
adapter, new typed public value or canonical/HMAC domain change. Fractional or
exponent json.Number decimals remain outside the existing private jsonint domain:
compatibility returns a blocked partial diagnostic plan; Strict refuses. An
unsigned value outside a driver's domain is not a verified binding. Collation,
charset, CHAR padding, live constraints, session settings and physical database
identity are not proved by TypeSource or a successful string constraint check.

Known mismatches, arity/budget/reserved violations refuse in every profile. Unknown
required type/constraint information produces a blocked partial diagnostic plan in
compatibility and a fixed sentinel refusal in Strict. Referenced projections,
orders, predicates and implicit tenant/soft-delete columns are checked; unrelated
columns are not certified. Internal records retain checked types, missing reasons,
positions, presence and resolution provenance for PR2. Source plan analysis is not
reinterpreted as complete type verification. Public views and CLI/MCP errors retain
GQ-AI-06's bounded omission and no-cause rules.

Stable errors support errors.Is: ErrInputLimit, ErrTypeMismatch, ErrTypeUnverified,
ErrArrayBinding, ErrReservedBinding and ErrInvalidManifest, alongside existing
operation errors. Their strings contain no input data; rejected internal checks
are not a new public detailed diagnostic API.

## Manifest and migration source compatibility

Schema generation writes TypeSource=sql; model-only generation writes go;
policy-only columns remain unknown. SQL type, nullability, default and other SQL
constraints take precedence when merging a model. Model policy annotations still
propagate; a model primary annotation does not create a physical SQL key.
Duplicate explicit schema table/column declarations refuse instead of last-wins.

Both manifest.Column and migration.ColumnSchema add Go-only NullableKnown.
JSON nullable true/false is known, omission unknown, null/nonboolean/ambiguous keys
invalid. Go callers must set NullableKnown for either bool value. Unknown Marshal
omits nullable and does not preserve an undeclared internal bool through roundtrip.
ReadSchema marks only observed YES/NO tokens as known; unknown tokens remain
unknown, while scan/connection errors remain errors. No new introspection occurs.
Model pointer/nonpointer declarations do not fill missing SQL nullability.

Manifest string version 1 remains and the shared compiler checks it, including
for direct Go manifests. Missing or unsupported manifest versions refuse. New readers load legacy missing type_source
as unknown, requiring regeneration for relevant Strict operations. New source is
not guaranteed readable by an old closed JSON Schema. TypeSource and SQL-preserving
merges naturally change SchemaFingerprint. Nullable omission for unknown migration
columns can separately change DatabaseFingerprint. Known nullable source keeps its
existing bool field layout. Hash algorithms are unchanged; neither Load nor Fresh
silently repairs stored fingerprints. Mixed schema/model generation now retains model policy flags previously dropped
by the merge, so those same inputs can also change PolicyFingerprint. Other
policy inputs retain their existing hash rules; generated-code/database skipped
checks remain skipped.

Migration: establish explicit schema declarations, regenerate with the new
library, reverify each fingerprint comparison, and update consumers together.
Model-only regeneration still supplies no DB type evidence. Already-normalized old
nullable:false files cannot reveal whether their original input omitted nullable.

DiffSchemas, CompareSchemaDrift and Apply rules are unchanged. Diff/drift still
consume Nullable bool and do not make safe unknown-aware DDL decisions. Presence
alone changes no in-process DDL step; source roundtrips can change undeclared bools.
Typed operation validation is not migration safety. Apply is not run by this work.

## Explicit Query limits

`Query.LimitExact(n int) *Query` adds a chainable SELECT setter over 0..max int
(architecture dependent). Negative input records sticky `query.ErrInvalidLimit`;
Plan/Build/execution refuse without interpolating the number. Valid later setters
do not clear an error. Existing Limit/Take retain their previous positive/zero/
negative behavior. A valid last setter wins: Limit(0) clears exact presence,
LimitExact(0) keeps it. Operation validates 0..10000 before converting int64 to int.

The same state flows through snapshots, copies, policy predicates, SQL, inspection,
Plan.Limit and private execution. No SQL or public plan is patched after sealing.
An Exact state on a write destination refuses before dispatch; no new write-limit
syntax is promised. A supported INSERT SELECT source can retain its own SELECT
limit. Count/First/Get keep ordinary scanner behavior, including no-row errors;
there is no synthetic zero count. Validated binding detects 0-to-positive and
0-to-omitted changes. Public Limit mutation never changes private execution.

PR2 retains detailed structured diagnostic projection and cross-surface diagnostic
formatting. Array binding, new driver adapters, canonical expansion and unsupported
DB types are continuing limits, not an implicit PR2 commitment.
