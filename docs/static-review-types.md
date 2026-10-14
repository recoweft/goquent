# Typed static review (GQ-AI-09 / PR1 of 3)

Refs #82: https://github.com/recoweft/goquent/issues/82.

Go source review resolves declaration packages and receiver types with
`go/types`, including generic origins, selections, import aliases and inferred
instantiations. A spelling such as `db`, `Get` or `Update` is not type evidence.
Unrelated methods are excluded. `database/sql` DB, Tx, Conn and Stmt methods,
Goquent DB/Executor raw methods, Query operations and generic CRUD are identified.
Prepared statement execution remains partial because statement SQL is not tracked.
Literal raw SQL with dynamic bindings also retains partial evidence.
Function and method values assigned locally, aliases, and method expressions are
recognized; an unavailable receiver remains partial. Returning a type-confirmed
DB callable, storing it in a struct/slice/map literal or a field/index assignment,
or sending it through a channel retains a fixed partial escape marker at the
escape site. Container contents and calls through them are not reconstructed;
the marker preserves the loss of tracking without interprocedural analysis.
Direct calls and callable escapes share the same declared raw-API inventory,
including SelectMap/SelectMaps and known Executor interfaces. Arbitrary dispatch through
interfaces declared by applications and function parameters is outside this
bounded analysis. This is not a complete whole-program call graph.

## Loading and dependency review

`golang.org/x/tools v0.43.0` supplies `go/packages`: module/build/test-variant
selection and export-data loading cannot be obtained by parsing imports alone.
It requires Go 1.25.0, compatible with this module's Go 1.25.0 minimum and tested
Go 1.26.4 toolchain. The downloaded module's LICENSE is BSD-3-Clause. Its direct
runtime dependencies used here are `x/mod v0.34.0` and `x/sync v0.20.0` (also
BSD-3-Clause). The versions and checksums are pinned in go.mod/go.sum. Review
does not import the root ORM facade or CLI, and exposes no internal builder.

Each Run loads selected package directories grouped by module root, with test
variants enabled and `-mod=readonly`. Types and helper declarations are reused
only within that Run and package variant. No persistent type cache is shared
between different source inputs. Normal files, internal/external tests, explicit
files and explicitly discovered testdata are best effort. Excluded build-tag
files, broken packages, module-external files and missing imports can remain
unsupported. An explicit file still needs its surrounding buildable package.
Dependencies must already be available to the Go toolchain. Go workspace and
build settings follow the invoking environment.

Budgets are 4,096 files, 2 MiB per file, 64 MiB parsed input, 256 package
directories and a 60-second loading deadline. An optional `Options.Context`
cancels loading earlier. Package loading uses these parse budgets even for
additional syntax requested by the loader. Budget or loading failures retain a
file-level `STATIC_REVIEW_UNSUPPORTED` finding; they do not mean "no database
operations." Type-confirmed calls and unresolved candidate locations survive
partial loads. Compiler/loader diagnostics are discarded, not included in public
errors, messages or evidence. The external Go process and its dependency export
loading are time-bounded; this is not an OS memory sandbox for the Go toolchain.

## Tracking boundaries

Query objects are mutable. Assignments and aliases share their state, keyed by
`types.Object`; different declarations and shadowed names do not share it.
Straight-line modifications are ordered. Branches inspect both arms and retain
uncertainty after joining; they never combine a predicate from one arm with a
limit from another into a precise plan. Untouched queries retain their state.
Branch snapshots preserve receiver identity. Rebinding a Query alias cannot
transfer another receiver's predicates or limits into precise evidence. Both
arms replacing a method receiver also retain uncertainty. Loops join the
zero-iteration and body states, preserving a possible DB callable even when the
body assigns an unrelated method. Loops, closure capture, escapes and opaque
calls make affected state partial.
Planning and execution terminals do not manufacture runtime evidence.

Known `ApplyScopes`, `ComposeScopes` and `TenantScope` declarations are followed.
`RequirePredicates` and `RequireTenantScope` retain a partial guard boundary:
requiring a predicate is not proving it exists or proving its tenant value.
Opaque callbacks and variadic scope slices stay partial. Scope aliases can be
tracked locally; arbitrary closures are not expanded as trusted scopes.

One same-package nonrecursive helper may be expanded if it has exactly one Query
parameter and one Query result, only Query modifier expression statements and a
final return. It can be in another file of that package. Calls to another helper,
assignments, loops, closures, escapes or other Query objects reject expansion.
Unsupported helpers retain unknown state; this is not general interprocedural
analysis. The six Query Validate* and six ExecuteValidated* operation families retain
partial markers; static review does not validate their private runtime handles.
Generic CRUD/composite operations retain a partial operation marker;
their SQL is not reconstructed. Complex DSL modifiers, Model-derived tables,
dynamic tables/columns/bindings and missing receiver state remain partial.

Literal supported chains still produce existing risk codes at the terminal's
physical selector position (ignoring `//line` remapping). Values and policy
metadata do not authorize an operation. Static soft-delete registration is no
longer inferred from a method spelling: registration-time/global behavior is
outside this tracker. Runtime Strict, tenant binding and private plan seals are
unchanged.

## External QueryPlan JSON

Source wire/version formats and original fixture inputs are unchanged. A readable
external QueryPlan now has at least partial precision and an independent
`STATIC_REVIEW_PARTIAL` marker with the fixed source message "external query plan
has no verified runtime correspondence". Known unsupported precision, invalid
plan fields/operation, inconsistent operation/SQL prefix, or missing SQL produce
`STATIC_REVIEW_UNSUPPORTED`. Identifiable malformed plan envelopes retain that
marker; unsupported versions additionally retain their error identity. Ordinary
JSON and OperationSpec documents without plan fields are not plans.

Existing risk checks still run. Supplied warnings cannot replace recomputed
warnings. Final findings, including supplied suppressed warnings, cannot regain
precise status through write-scope evidence. The independent marker is not taken
from input warnings, locations, approval or verification claims. JSON strings
are never interpreted as inline suppression directives. Explicit application
rule/config suppression remains supported under the existing expiry rules;
suppression remains a review choice, not evidence or authorization.

This changes precision gates, including previously warning-free serialized
plans. It does not change QueryPlan serialization, source JSON APIs, runtime
execution or public-view redaction. MigrationPlan JSON and manifest freshness
semantics remain outside this change. No runtime-plan matching, signatures or
provenance registry are implemented here. PR2 owns correspondence and multi-axis
precision; PR3 owns SARIF, changed-line review and bounded baselines.

## Fixture migration

The original invalid `db any` / `q any` snippets remain verbatim in test source.
Tests retain copies as `.legacy.txt` and use a separate buildable module fixture
with actual DB/Query types for the original code/location/suppression assertions.
The adapter adds imports on the package line to preserve physical line numbers.
It only repairs the explicitly listed legacy fixtures below; production review
never rewrites input or falls back to name-based identification.

| Case | Previous assumption | New expectation and retained coverage |
| --- | --- | --- |
| GoSourcePreciseAndSuppressed | `any.Table` was precise | Invalid original is unsupported; typed counterpart preserves UPDATE_WITHOUT_WHERE at line 5 and LIMIT_MISSING suppression |
| GoSourcePartialAndUnsupported | `any.Update` was partial | Invalid original is unsupported; typed Query variable remains partial, dynamic DB SQL unsupported |
| GoSourceWithManifestPolicies | `any.Table` supplied policy structure | Typed counterpart retains tenant/soft-delete/PII/bulk codes; manifest is not authorization |
| RunLintsConfigSuppressions | Name matching supplied the suppressed operation | Typed counterpart retains unused/expiry/owner/reason lint behavior |
| RunGoSuppressionRegression | `any.Exec` was raw SQL | Original parser fixtures remain; typed counterparts retain actual comment/string distinctions and physical locations |
| DynamicTableWriteScopeIsUnknown | Undeclared `db` was a database | Original text retained; typed DB counterpart retains unknown write-scope evidence |
| SharedPlanSerializationAndReview C01 | Warning-free serialized SELECT had zero findings | Exact original SQL/params/wire checks remain; one partial correspondence marker now trips the partial gate |
| SuppressedWarningsFromQueryPlanJSON | Supplied suppression yielded zero visible findings | Supplied warning remains suppressed; independent partial marker remains visible |

`TestTypedIdentityAndTracking`, failure/cancellation/privacy tests and JSON/API/CLI
regressions exercise the new boundaries. Existing SQL, expiry and source JSON
round-trip tests remain. No thresholds or bulk suppressions were relaxed.

Run `go test ./...`, `go test -race ./orm/review ./cmd/goquent`, `go vet ./...`,
`sh scripts/test-public-output-check.sh`, and the repository integration/check
commands. `go run ./cmd/goquent review --fail-on high ./...` can still exit 1;
counts and precision limits must be reported rather than hidden.
