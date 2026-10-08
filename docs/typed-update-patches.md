# Typed update patches and repository checks

GQ-AI-08 PR2 (2/2), [Issue #79](https://github.com/recoweft/goquent/issues/79).
This extends the existing opt-in repository generator. Dynamic APIs and legacy
generation remain available. See [typed repositories](typed-repositories.md)
for the supported input types, nominal keys, projections and read contracts.

## Three explicit states

A generated `RecordPatch` has a separate nominal field type for each supported
update candidate. State and stored value are private. For example:

```go
columns := RecordColumns()
patch := RecordPatch{
    Active: columns.Active.Set(false),
    Note:   columns.Note.SetNull(),
}
result, err := repository.UpdateByKey(ctx, key, patch, RecordUpdate{})
```

The zero field is unchanged and does not enter SET. `Set(false)`, `Set(0)` and
`Set("")` are present values. `SetNull()` explicitly binds SQL NULL in SET;
it does not omit the assignment. The existing builder uses a placeholder with
a nil argument, preserving the dynamic update's SQL/argument contract.
Assigning a field's zero patch value clears that assignment. The last field
assignment wins. An entirely unchanged patch returns `operation.ErrEmptyPatch`
from planning and execution, with no executor call. It is not a successful no-op.

Only supported, explicitly nullable SQL columns expose `SetNull`. Nonnullable
and unknown-nullability columns have no such method. A known type with unknown
nullability can express a value, but its plan is blocked in compatibility mode
and rejected in Strict. Unknown/Go/unsupported declarations expose no any setter.
Runtime checks remain necessary even after a successful Go compilation.

Readonly, generated, forbidden, PII/protected, tenant, soft-delete and supplied
primary columns are excluded from patch candidates. Manifest policy entries for
protected/immutable/readonly/generated targets also exclude them. Actual DB
settings, immutable columns and existing write/tenant checks still apply at each
operation. The manifest does not discover live column facts. Separate legacy
update/delete APIs keep their contracts; this patch API adds no key reassignment
or soft-delete command.

Different models' patches, different columns' field patches and different enum
or key types cannot be mixed implicitly. Untyped constants and explicit Go
conversions can still express wrong inputs. Zero column references produce an
invalid assignment, not an allowed target or an unchanged field. A tenant key
component must be the existing opaque current-tenant marker; its zero value
refuses. Literal values and a `current_tenant` Values-map entry cannot supply
tenant authority.

Generated `RecordUpdate` contains model-specific Filters and AccessReason. It
has no read ordering or LIMIT. `PlanUpdateByKey` returns the existing plan,
DiagnosticView and error. `UpdateByKey` returns the existing sql.Result/error.
Only a supported supplied primary key produces these methods; no inferred id
or any key fills missing facts. Named projection `Summary` also generates
`PlanUpdateSummaryByKey` and `UpdateSummaryByKey`. PostgreSQL RETURNING uses the
existing one-row scanner and bool policy; MySQL RETURNING refuses before dispatch.
First-row scanning does not establish exactly one affected row or roll back an
autocommitted write. External transactions remain caller-owned.

## Application source adapter

`operation.UpdateSpec` is a separate, closed version-1 source interface.
OperationSpec, its CLI and MCP remain SELECT-only. An assignment has Column,
State (`unchanged`, `null`, `value`), Value and a Go-only ValuePresent flag.
Value false/zero/empty string is present. Value nil/JSON null, missing value,
value on a null/unchanged assignment, unknown state, duplicate normalized column,
unknown field and forbidden field refuse. Unchanged records still undergo target,
declaration, presence and budget checks. Source JSON retains explicit presence
and json.Number lexemes; it is sensitive data, not public display output.
Generated patch String/Format/JSON output remains fixed omitted text.

`operation.CompileUpdateWithDiagnostics` is DB-free. The DB method with that
name forces the actual immutable Settings and Dialect. Application-only
`orm.UpdateOperationBy` and `orm.UpdateOperationReturningBy[T]` also force the
actual Executor. They share the existing budgets, declaration/scalar validation,
tenant resolution, diagnostics and private Query write planner. Final RETURNING
is inspected and sealed on that same Query. Public plan SQL/params are never fed
back into Raw; execution does not rebuild a second Query. Public mutations cannot
change the captured statement, and a prepared private write cannot be replayed.
This immediate lifecycle does not add the separate ValidatedPlan current-input,
expiry and CAS protocol to UpdateSpec.

Assignments use the generic builder's sorted literal-column rendering. Unqualified
equality predicates use its literal equality path. Generic Update now sorts its
column/value pairs and primary predicates together, removing previous map-order
variation without coercing values. Equivalent generated and dynamic UpdateSpec
operations have the same SQL, ordered native arguments, condition tree and
diagnostics. The same structural generic input shares the SQL/policy substrate.
Older Query-DSL rendering can have its existing whitespace differences. Ordinary
dynamic map updates do not acquire the new manifest type/NULL guarantees.

Native widths/sign, enum membership and original decimal lexemes are retained.
No Valuer/Stringer/Marshaler conversion, decimal-to-string adapter, driver or
private canonical domain is added. Fractional/exponent json.Number remains
blocked/refused. In addition, existing Strict write inspection accepts a narrower
scalar domain: it rejects json.Number assignments even with an integer lexeme.
Compatibility can execute an integer-lexeme decimal assignment when its existing
gates allow it. There is no automatic result/input roundtrip guarantee. Arrays,
unsigned driver domains, MySQL timestamp sessions, unknown collation/live facts
and external authorization limits remain unchanged.

## Read-only regeneration in CI

```sh
go run ./cmd/goquent manifest check-repositories --config repositories.json
```

The version-1 registry is sensitive source configuration. Its directory is the
fixed base for all relative paths. The checked-in example is
`tests/typedfixture/repositories.json`. Every registry/entry/option/projection
member is explicit; unknown, ambiguous and misspelled members refuse. A registry
has nonempty managed_roots and entries. Each entry records snapshot_kind (`manifest`
or `table`), input, target, generator_version, code_paths and complete options:
package_name, table_name, row_type_name, repository_type_name, orm_import_path,
typed, dialect and ordered projections (name/columns). Empty code_paths means the
snapshot contains no generated-code fingerprint. Typed must be true.

Inputs are fixed supplied snapshots, not instructions to call Manifest.Generate
again with current time or global registered policies. GeneratedAt, verification
and other supplied facts remain intact. Manifest schema/policy fingerprints are
recomputed using the existing algorithms and supplied ordering. A table input is
`RepositoryTableSnapshot`: version 1, snapshot_kind table, dialect, table and its
table-scoped schema/policy fingerprints. Table columns use the generator's sorted
order. Table fingerprints never certify a whole manifest. No live DB is read.

Code_paths, when present, are an explicit list of regular source files relative
to the registry directory, using the original fingerprintPaths name/content
algorithm. They cannot include a managed output, the snapshot itself or the
registry. Directory expansion and self-referential generated-source hashes are
not supported. Establish this scope when creating the fixed snapshot; do not
delete schema/policy/verification facts to obtain a passing check.

Targets must be regular Go files inside the managed roots and begin with the
exact dedicated ownership header. An embedded copy of the header in handwritten
source does not establish ownership. Every owned file under the roots must have
exactly one entry, and every entry must have an owned target. Handwritten files
are neither targets nor automatically overwritten. Missing roots/input/targets,
unregistered owned files, duplicates, symlinks and escaping paths refuse.
Registry and input JSON are limited to 1 MiB and the existing depth/node budgets;
there are at most 256 entries/roots, 4,096 visited filesystem entries, depth 32,
4 MiB per source file and 64 MiB total reads. Oversized input fails closed.

The checker invokes the real existing generator in memory and compares complete
source bytes. Header or code-hash agreement alone is insufficient. Schema/policy,
version, options, projection or source drift returns nonzero. Missing/invalid
input and stale output have fixed classifications without names, paths, values,
fingerprints or raw errors. The checker creates no file, invokes no compiler and
does not update the registry or snapshot. A current result means only equality
to supplied inputs. It is not freshness against a physical DB or authorization.
Deleting both an entry and its artifact cannot be distinguished from an intended
scope removal; changes to the required registry/root/fixture contract need review.

CI runs this command inside `scripts/test-public-output.sh`, before tests.
Build, parse and read errors stay in a private temporary log which is deleted,
never printed or uploaded. Fixed results preserve the command's nonzero exit.
Canary tests cover checker and test/build failures as well as success. Generated
Go compilation is separate fixture testing, with compiler output captured too.
The wrapper explicitly supplies TEST_DB_DSN from TEST_MYSQL_DSN when absent.

## Local update and migration

1. Review the fixed input snapshot, its fingerprint scope and registry options.
   Use an explicit stable generation time when creating a new manifest. Keep
   generated outputs outside its original code fingerprint scope.
2. Outside CI, run the existing `manifest repository --typed --manifest ...`
   command with the registry's table/package/row-type/repository-type/projections
   and `--unsafe-local-output` pointing to a **new** local file. For table inputs
   or a custom ORM import path, use the existing sensitive Go source generator
   API with matching options. Never publish generated bytes or compiler output.
3. Review the local diff and compile the combined application package, including
   handwritten declarations. Manually replace only the confirmed owned generated
   target, then commit the fixed input, options and generated artifact together.
4. Run the read-only check and application tests. For stale/invalid results,
   inspect inputs/diffs locally; do not enable unsafe output in CI or suppress
   the checker. Restore a missing input/target or explicitly review scope changes.

The local CLI still uses O_EXCL, mode 0600, existing path checks and the CI
prohibition. It does not overwrite an existing target or handwritten source.
Typed output records implementation version `typed-repository-v2`; a caller's
version string cannot impersonate it. Regenerating v1 adds types, fields, methods
and the ownership header, so update callers and registry together. Use keyed
literals and check package-wide collisions. Preserved v1 fixtures are compiled
separately for source compatibility and are not certified as current v2 output.
Legacy generation retains its existing behavior and byte fixtures.
