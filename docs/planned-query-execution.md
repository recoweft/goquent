# Planned Query execution (GQ-AI-04/PR3)

[Issue #66](https://github.com/recoweft/goquent/issues/66), **GQ-AI-04/PR3 (3/3)**.
PR1 (#67) and PR2 (#68) are merged. The implementation base for PR3 is
`c30f8dc1e984ea63005eec0dc57ed700acef06d0`. The tables below describe current
coverage; [contracts v3](../specs/goquent_ai_contracts_v3.md) retains its historical
baseline. These conditional contracts do not establish Issue-wide completion.

## Query terminal inventory

`DB.Table`, `DB.TablePath`, `DB.Model`, `query.New` and
`query.NewWithSettings` construct the same Query implementation. Context,
projection, predicate, join, aggregate, cursor, ordering, locking and other
chain modifiers feed the terminal operation; they do not execute independently.

| Public entry | Before PR1 | PR1 path and result |
| --- | --- | --- |
| `Query.Plan` | SELECT BuildSnapshot and finalizer | Same DB-free planning, private execution snapshot retained |
| `PlanInsert`, `PlanInsertBatch` | INSERT Build plus separately assembled target/column metadata | INSERT BuildSnapshot supplies SQL, arguments, target, columns and batch/conflict metadata; no executor call |
| `PlanUpdate`, `PlanDelete` | Rendered condition snapshot, separately assembled target/assignments | Rendered statement snapshot also supplies target and assignment columns; no executor call |
| `First`, `FirstMap`, `Get`, `GetMaps` | Repeated Plan/gate/Query/scan blocks | Common private bind and Query/QueryContext dispatch; existing struct/map scanners, row closure and errors |
| `Count` | Copied aggregate builder, local gate and QueryRow dispatch | Same aggregate plan, common private bind and QueryRow/QueryRowContext with int64 scan |
| `Insert`, `InsertBatch`, `InsertOrIgnore` | Local gate then Exec | Common private bind and Exec/ExecContext; original sql.Result retained |
| `Upsert`, `UpdateOrInsert` | Private insert plan, local gate then Exec | Same supported insert/conflict operation, frozen builder metadata and common dispatch |
| `InsertUsing` | Private INSERT SELECT plan; strict refusal | Common dispatch in compatibility mode; strict refusal unchanged |
| `Update`, `Delete` | Local gate then Exec | Common private bind and Exec/ExecContext; original sql.Result retained |
| `InsertGetId` (MySQL) | Insert then LastInsertId | Common Insert dispatch, unchanged driver LastInsertId semantics |
| `InsertGetId` (PostgreSQL) | Insert plan, private RETURNING check/reseal, local QueryRow | Extend private original; inspect projection and re-finalize final SQL; common QueryRow and ID scan |
| `Build`, `Dump`, `RawSQL` | SQL/debug rendering only | Unchanged; no semantic gate and no executable evidence |

There is no public Query `PlanUpsert`, Query `Returning`, or execute-a-plan API.
The generic diagnostic plan APIs added in PR2 are listed below. Query `InsertBatch`/slice-based Upsert each render one
statement and are covered here. These Query terminals themselves do not introduce
a parent/child or multi-statement batch lifecycle. SQL is rendered once per plan, not again during dispatch.

## Generic, scoped and boundary inventory

| Public entries | Current path and remaining boundary |
| --- | --- |
| `PlanInsert`, `PlanUpdate`, `PlanUpsert`, `PlanInsertMany`, `PlanUpsertMany` in `orm` | New DB-free diagnostic APIs; same input extraction, builders, policy/risk, final RETURNING inspection and seal as execution; no dispatch, EXPLAIN or schema reads |
| `Insert`, `Update`, `Upsert` | Generic struct/map mapping and effective WriteOpt fields feed the common Query write planner; private Exec dispatch and existing sql.Result/affected-row checks |
| `InsertReturning`, `UpdateReturning`, `UpsertReturning` | Same planner; infer projection from result type unless explicit Returning is nonempty; re-inspect final projection/SQL before private Query dispatch and generic one-row scanner |
| Write `Returning` option on result helpers | Same final projection inspection; count returned rows for the existing result/ExpectAffected/NoRowsAs behavior; PostgreSQL only |
| `InsertMany`, `UpsertMany`, `InsertManyReturning`, `UpsertManyReturning` | All candidates of **one statement** use the same substrate. No public splitting API. Nested generated-ID collection uses private per-row statements (below). Empty-input errors remain |
| `PlanSelectBy`, `PlanUpdateBy`, `PlanDeleteBy` | Existing DB-free plans use the supplied base Query and scopes. They accept no separate destination DB and do not attest a later helper's destination |
| `UpdateBy`, `DeleteBy` | Existing Query terminals and their original Query executor/settings |
| `SelectOneBy`, `SelectAllBy` | Apply scopes, copy private builder state, then re-plan with the separately supplied DB's settings, dialect, execution context and executor. Private Query dispatch; generic struct/map/scalar/bool scanner |
| `UpdateByReturning`, `UpdateByReturningWithOptions` | Same destination rebinding, scoped UPDATE/data inspection and final RETURNING inspection. Only Returning and NoRowsAs are effective options; see below |
| `SelectOne`, `SelectAll`, `SelectStruct`, `SelectStructs`, `DB.SelectMap`, `DB.SelectMaps` | SQL-string inputs remain **Raw**. Existing error-returning raw gate then generic scanning. Conditional strict settings refuse them before executor calls; TouchedTables and reasons are not semantic proof |
| `InsertOnceReturning` | Private ordered insert and conditional lookup plans from the same extracted input; both checked before insert dispatch. Final RETURNING and lookup projection checked. Scan/result-dependent branch; no concurrency or replay guarantee |
| `ReplaceNestedCollection`, `ReplaceNestedCollectionTx` | Known parent/delete/children inputs checked before dispatch/Begin; private ordered statement/range records. Strict rejects opaque scopes and ID callbacks; compatibility resolves dynamic slots at the original callback position |
| `RunIdempotentCommand` | Strict rejects before LookupExisting/Apply/LookupAfterConflict/Begin. Compatibility preserves existing lookup/transaction/conflict order; ORM helpers retain their gates |
| `RunTransactionWithHooks`, `InsertHook`, `InsertManyHook`, `NewTransactionHook` | Strict recipe entry refuses opaque Apply/hooks before Begin or callbacks. Compatibility retains Apply → ordered hooks → commit/rollback. Standalone hook.Run retains constituent helper gates; public mutable Run is not safety evidence |
| `DB.RawPlan`, `query.NewRawPlan`, `NewRawPlanWithSettings` | DB-free Raw diagnostics; conditional Strict refuses unknown Raw regardless of reasons, TouchedTables, risk overrides or public verdict edits |
| `DB.Query`, `QueryContext`, `Exec`, `ExecContext`, `QueryRowE` | One Raw inspection followed by the shared private seal/bind/dispatch; rejection calls no Executor method. Raw read/scanning aliases inherit this path |
| `DB.QueryRow`, `QueryRowContext` | Return *orm.Row in every mode. Refusal is stored without DB/Executor calls; Scan/Err retain original error identity. Allowed rows delegate to *sql.Row |
| `SQLDB`, underlying sql.DB/sql.Tx, driver and direct Executor | Outside ORM interception |
| `orm/operation.Compile`, review, manifest, MCP | Existing nonexecuting/read-only contracts; no application trust or new write API |

No entry accepting public QueryPlan/JSON is added. `extractPlanSQL` followed by
Raw is not an execution adapter. The old generic independent SQL renderers and
trusted dispatch paths are removed. Raw input also enters the shared private bind/dispatch after its Raw gate; it is never upgraded to semantic SQL.

### Scoped write options

`UpdateByReturningWithOptions` continues to evaluate its WriteOpt functions,
but only `Returning` and `NoRowsAs` fields affect SQL/results. The last values
win; an absent or empty Returning infers the result struct's columns. Map and
untyped return values still need explicit columns.

`Table`, `TablePath`, `SchemaName`, `Columns`, `Omit`, `PK`, `WherePK`, `SetRaw`,
`SetExpr`, `SetColumn`, `Increment`, `ConflictColumns`, `ConflictWhere`,
`ConflictConstraint`, `ConflictTargetRaw`, `UpdateColumns`, `ConflictDoNothing`
and `ExpectAffected` remain nonapplicable to this scoped helper. They neither
change the actual table/data/conditions nor remove assignments from inspection.
No new error is introduced merely for supplying one of those options. This
compatibility rule does **not** disable effective options on ordinary generic
Update/UpdateReturning or other helpers.

`NoRowsAs` maps only the existing not-found outcome to `RowsAffectedError`.
Policy/approval refusals, driver failures and scan errors retain their identities.
Typed one-row RETURNING still does not enforce ExpectAffected or reject extra
rows; typed Many RETURNING still does not apply ExpectAffected/NoRowsAs. Result
helpers retain their existing affected-row options. No new cardinality, automatic
rollback, resend, permit or artifact-version contract is introduced.

## Internal lifecycle and inspection contract

Builders render a detached input. SELECT/UPDATE/DELETE retain the rendered
condition trees; INSERT retains the rendered candidate/target/conflict shape.
The Query finalizer applies the existing policy, required predicates, risk,
write evidence and conditional tenant inspection. It seals an unexported
execution record with the statement, detached supported arguments, structural
inspection, gate result, executor/context and approval expiry. All Query terminal
modes bind that record, reject before dispatch, and consume it once. Reusing a
Query terminal builds a fresh plan; it does not replay an old execution record.

Public QueryPlan fields, JSON, masked display data and public verdicts are never
execution inputs to this dispatcher. Changing public fields cannot change its
statement or remove its captured rejection. JSON has no private lifecycle and
cannot enter it. This does not turn the legacy public `EnsurePlanExecutable`
helper into a general validator for arbitrary artifacts. There is no new public
execute-a-plan API, signed artifact, external permit or live executor attestation.
Unsupported opaque custom values retain compatibility-mode driver semantics;
that is not an immutable-value or safety proof. Strict still refuses values it
cannot compare, without invoking application Valuer/Stringer/marshaler methods.

PostgreSQL ID RETURNING extends the private original, checks the actual returned
column, and re-runs finalization over the final SQL before sealing a new record.
A rejected extension leaves the original unchanged. Projection metadata records
`returning_columns`; that public annotation itself is not evidence. Builder or
condition changes cause fresh planning. Planning performs neither executor calls,
live schema reads nor EXPLAIN. Such database reads remain separate explicit work.

The dispatcher retains context/noncontext custom Executor methods and sql.Tx
behavior. Struct/map/bool scanners, sql.Row scan errors, sql.Result/RowsAffected
and MySQL LastInsertId remain their existing contracts. Query writes have no
new expected-cardinality option or automatic rollback after an affected-row
observation. Result scanning cannot replace pre-execution rejection.

## Compatibility and conditional strict inspection

Existing compatibility settings remain the default; legacy reason-based risk
handling is unchanged there. Applications using `WithTenantPolicy` retain the
PR65 conditional gate documented in [tenant-policy.md](tenant-policy.md):
application-authenticated tenant supply, asserted schema/PlainTable, signed SQL
integer domain, all-branch binding, per-alias supported JOIN checks, outer-AND
soft deletion, all INSERT candidates, protected assignments, dialect-specific
unique conflict coverage, PII/RETURNING refusal and missing external high-risk
permit refusal. No reason, suppression, RiskLow or precise result overrides this.

The ORM does not authenticate tenants or verify live schema freshness. Unknown
conditions, opaque/raw SQL and unsupported schema/write forms remain refusals
under that gate; tests do not prove safety of unparsed cases. The supported generic/scoped and compound entries above inherit this gate. Opaque
compound recipes are explicitly unsupported, not statically proven. Direct escape
hatches remain outside interception; Issue #66 is not declared complete.

## Internal adapter and ownership

`orm/internal/querybridge` is a package-private dependency inversion point:
Query installs its adapter during package initialization; the facade already
imports Query. Only ORM-internal structural requests can reach it. Its opaque
Base/Settings identities are checked as concrete Query/Settings values. It has
no SQL-input, public verdict or serialized-artifact execution interface and does
not expose internal builders in public signatures. No new dependency is added.

Generic field/tag/option extraction produces rows, literal identifier paths,
ordered columns, key equalities and conflict/assignment inputs. Both Query and
generic writes use `planInsertRows`/`planUpdateValues`, the existing frozen
builder snapshots, tenant inspection, finalizer and `plannedExecution` dispatch.
Generic column ordering and literal identifier quoting are retained; Query's
existing path/JSON assignment interpretation stays distinct. Query PrimaryKey is
a literal identifier, while generic Returning retains identifier-path quoting. Expression and
named/raw/partial conflict forms preserve compatibility rendering but are not
strict evidence. Unsupported literal key identifiers are opaque to scope proof.

The facade receives internal scanner/result closures bound to the sealed record;
the diagnostic view is not consumed by those closures. Mutable supported payloads
are detached by the existing bounded copier. Opaque custom values preserve driver
semantics in compatibility mode and are never compared by invoking Valuer,
Stringer or marshaler methods. Conditional strict writes reject unsupported
payloads. TableName mapping and WriteOpt evaluation retain their ordinary API
behavior; planning is not a promise to avoid every possible application callback.

Scoped destination rebinding copies the private builder and retains the requested
operation/scopes, but replaces settings, policy lookup, dialect, tenant/execution
context and executor with the destination DB's values. Source write-key assertions
are not transferred to another destination. Mandatory soft-delete predicates are
now derived on detached builders, including compatibility mode, so prior source
planning cannot inject generated source policy into a later destination operation.
Explicit caller predicates remain part of the requested operation.

DB settings still propagate through parent transactions, WrapTx and WrapExecutor.
Standalone external wrappers need explicit settings. No physical DB identity,
executor honesty, schema freshness, authorization or transaction ownership is
independently attested by these tests or wrappers.

## Migration examples and error contract

Compatibility remains the default settings mode, not a new profile API. Generic
writes now receive the **same configured policy/risk gate as Query writes**;
previously bypassed operations can fail. RETURNING now receives projection policy
inspection, including PII modes in compatibility and unconditional PII/forbidden
refusal under conditional strict settings. Retain the existing sentinel checks
with `errors.Is`: `ErrBlockedOperation`, `ErrApprovalRequired`,
`ErrApprovalReasonRequired` and `ErrAccessReasonRequired`. Strict refusal messages
include the same `tenant_policy/...` reason codes. Shape/dialect errors remain
ordinary errors; not every driver failure is normalized.

After application authentication/authorization and schema assertion, configure
`requestDB` as in [tenant-policy.md](tenant-policy.md). Generic insert and its
nonexecuting preview can then use the same options:

```go
values := map[string]any{"id": orderID, "score": score}
opts := []orm.WriteOpt{orm.Table("orders")}
preview, err := orm.PlanInsert(ctx, requestDB, values, opts...)
if err != nil { return err }
// preview is diagnostic only. Do not feed preview.SQL/Params to Raw.
_ = preview
_, err = orm.Insert(ctx, requestDB, values, opts...)
```

With automatic tenant binding, only missing INSERT tenant values are filled;
supplied mismatches are refused. For updates, use explicit PK/WherePK (map) or
model PK tags and select only intended assignment columns: updating a full model
containing tenant/immutable/PII fields is not allowed merely because the values
are unchanged. Upserts require tenant-compatible unique constraints and update
columns; MySQL checks every possible unique conflict, PostgreSQL the target.
Named/raw/partial conflict targets and expression assignments are refused under
conditional strict settings. Ordinary unsupported forms are not silently made safe.

To inspect final PostgreSQL RETURNING without execution, use the corresponding
PlanInsert/PlanUpdate/PlanUpsert (or Many) with explicit `Returning("id", ...)`.
Typed execution can infer that same projection from its result struct. There is
no public execution handle, typed returning-plan family or automatic live lookup.

Replace raw-shaped reads with structural scoped reads when strict inspection is
required, retaining the generic scanner:

```go
// Raw-shaped SelectOne(ctx, requestDB, "SELECT ...", ...) is refused in strict.
q := requestDB.Table("orders").Select("id", "score").Where("id", orderID)
row, err := orm.SelectOneBy[OrderSummary](ctx, requestDB, q)
```

A separately supplied destination DB is the final authority for configuration,
not a prior PlanSelectBy result. There is no new destination parameter on the
existing scoped plan helpers. Re-plan through the actual execution helper; a
preview cannot attest a future executor. Reasons/TouchedTables cannot upgrade
an unparsed SQL string to a semantic SELECT, and bool/scalar/map scanning does
not inspect SQL meaning.

See [PR3 validation](gq-ai-04-pr3-validation.md) for commands and limitations.

## Compound support and ordered correspondence

The unexported compound plan records phase, input start/end range, detached
structural input and the corresponding sealed statement closures in execution
order. It is not a new public artifact, OperationSpec or bulk split API. Ordinary
InsertMany/UpsertMany and typed Many RETURNING still issue one statement. Private
compound preparation calls no DB/Executor/Begin; diagnostics are never read back
as execution inputs. All known constituent gates are checked before initial
execution, then each statement binds again (including expiry/context checks).
Transaction wrappers first prepare on the original DB, then begin, re-plan all
known inputs using the actual transaction executor/settings/context/dialect,
and dispatch. No source DB permission transfers to another destination.

Supported conditional Strict nested shapes have no Grandchildren, AssignChildID,
or nonnil Scope anywhere in DeleteBefore. The rejection runs before TableName,
WriteOpt, Scope, parent, deletion, child or Begin. SkipParent/empty Children do not
waive it, and built-in scope factories are not exceptions. ErrUnsupportedCompound
and ErrBlockedOperation both match with errors.Is. Nil-only/empty scopes receive
ordinary DELETE inspection, not permission; missing tenant/filter/risk evidence
refuses the whole known plan. No cleanup step is silently omitted. The current
NestedDelete API has no structural condition input: useful Strict support is
primarily Parent/Children without cleanup, not complete collection replacement.
Existing automatic-tenant policy behavior is retained, not an unconditional
cleanup authorization.

In compatibility mode, Parent executes before each DeleteBefore scope is evaluated
at that deletion's original position. A scope is evaluated once, in array order;
nil is ignored and a nil return keeps the current Query. An alternate Query is
copied and re-planned on the actual destination. Unknown delete slots remain
explicitly unresolved until that point. Known children are prechecked; later delete inputs are re-planned after a scope has run. If scopes
can change captured inputs, children are freshly extracted with the already
evaluated options and re-inspected after scopes. Dynamic grandchildren are
constructed after child IDs/AssignChildID, then receive fresh ordered plans and
the normal gate. Opaque callback effects themselves cannot be enumerated or
intercepted. A callback or later refusal may follow earlier valid statements;
rollback does not mean those statements or external effects never happened.
TableName/WriteOpt remain ordinary application construction callbacks, not a
promise of zero effects from every possible application callback.

Generated-ID collection uses one private INSERT per input row on **both** dialects.
MySQL reads each result.LastInsertId; it no longer guesses firstID+index. PostgreSQL
uses final inspected RETURNING per row, checks one returned ID, and scans it with
the existing converter. This avoids assuming arbitrary multi-row RETURNING order.
The prior MySQL implementation used one multirow insert and contiguous-ID guessing;
this is an intentional statement granularity change, with possible partial writes
without a caller-owned transaction. WriteOpt is evaluated once for the input batch.
MySQL ExpectAffected/NoRowsAs apply to the aggregate affected count, not each row;
PG's prior typed-Many option nonapplication remains. Driver/result failures stop
processing, preserve errors and leave already executed statements executed. There
is no retry, new transaction ownership or custom-executor correctness guarantee.

InsertOnceReturning builds insert/conflict/final projection and lookup from one
extraction and inspects both branches before dispatch even when the insert would
succeed. A lookup construction/policy error can therefore now prevent an insert.
The lookup is still conditional on ErrNoRows; it is not an atomic concurrency or
exactly-once contract. Explicit tenant-compatible conflict coverage is checked for
generic DO NOTHING as well as updating UPSERT. MySQL RETURNING stays unsupported.

RunTransactionWithHooks and RunIdempotentCommand are unsupported at their Strict
recipe entry, including empty Hooks: Apply/Lookup remain opaque. Compatibility
retains existing callback/commit/rollback behavior and all migrated statement
gates. NewTransactionHook, InsertHook/InsertManyHook provenance or public Run
replacement is never a static proof. Ordinary Transaction/TransactionContext,
Begin/BeginTx, external caller-owned transactions and standalone hook.Run are not
whole-recipe interception APIs. Direct SQLDB/driver/Tx/Executor and external effects
remain outside the ORM guarantee. An external Executor does not acquire transaction
ownership or Begin capability by being wrapped. Switching to compatibility does
not retain Strict guarantees.

## Row source compatibility and migration

DB.QueryRow and DB.QueryRowContext return ***orm.Row**, with Scan/Err. This change
applies in compatibility mode too. Chained Scan and Scan/Err-only interfaces keep
working. Explicit *sql.Row assignments, functions/interfaces requiring that return
type, and passing *orm.DB itself as an Executor are source incompatible. Executor's
six method signatures and sql.DB/sql.Tx/custom executors are unchanged; Query's
internal executor also remains unchanged.

```go
// Old concrete-type use no longer compiles:
// var row *sql.Row = db.QueryRow(sqlText, args...)
row, err := db.QueryRowE(ctx, sqlText, args...)
if err != nil { return err }
return row.Scan(&value)
```

Do not replace a rejected operation with SQLDB/direct Executor as a safety
migration. Use structural scoped reads for Strict. A rejected Row returns the
original error from both Scan and Err (errors.Is/As preserved), never fills a
destination, and performs no SQL, driver access or Begin. Nil/zero wrappers return
an explicit uninitialized-row error without panic. Allowed Row delegates Scan/Err
to the original sql.Row without further dispatch; initial query timing, driver
errors, no-row and scan behavior remain database/sql's. No replay/cardinality or
new scan-success guarantee is added. No unsafe/reflection/private sql.Row fields,
custom driver or cancelled sentinel SELECT is used for refusal transport.
