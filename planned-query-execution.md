# Planned Query execution (GQ-AI-04/PR1)

[Issue #66](https://github.com/recoweft/goquent/issues/66), PR1 of 3.
This inventory was checked against the implementation on main
`e81b8b4e93beab7104059b1539980a55c2349b0d` and this change. It supplements the
historical inventory in [contracts v3](../specs/goquent_ai_contracts_v3.md).
PR2 and PR3 require separate work after their predecessor merges.

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

There is no public Query `PlanUpsert`, Query `Returning`, or generic nonexecuting
write-plan API added here. Query `InsertBatch`/slice-based Upsert each render one
statement and are covered here. No parent/child or multi-statement batch lifecycle
is introduced. SQL is rendered once per plan, not again during dispatch.

## Derived entries and deliberately separate boundaries

| Entry family | PR1 status / later owner |
| --- | --- |
| `PlanSelectBy`, `PlanUpdateBy`, `PlanDeleteBy` | Apply scopes and call the corresponding Query plan; DB-free |
| `UpdateBy`, `DeleteBy` | Delegate to Query terminals and therefore use the common execution path |
| `SelectOneBy`, `SelectAllBy` | Existing Query inspection followed by public SQL/params and a separately supplied DB raw gate. PR2; not claimed as integrated. Strict raw DB still refuses |
| `UpdateByReturning`, `UpdateByReturningWithOptions` | Existing Query inspection then separately supplied DB and appended projection. PR2; not a private Query execution path |
| Generic `Insert`, `Update`, `Upsert`, typed `InsertReturning`, `UpdateReturning`, `UpsertReturning`, write `Returning` option | Existing independent builders/trusted dispatch and result options. PR2; no new safety claim |
| `SelectOne`, `SelectAll`, compatibility `SelectStruct`, `SelectStructs`, `DB.SelectMap`, `DB.SelectMaps` | Existing raw gate and generic/bool scanning; PR2 integration boundary |
| `InsertMany`, `UpsertMany`, `InsertManyReturning`, `UpsertManyReturning`, `InsertOnceReturning` | Existing generic/batch/lookup paths. PR2 generic substrate, PR3 compound correspondence; unchanged |
| `ReplaceNestedCollection`, `ReplaceNestedCollectionTx`, `RunIdempotentCommand` | Existing constituent paths; PR3 compound integration, no parent/child plan guarantee here |
| `RunTransactionWithHooks`, `InsertHook`, `InsertManyHook`, `NewTransactionHook` | Callbacks retain their selected helper path. Query writes inherit PR1; generic hook writes are not newly protected. PR3 |
| `DB.RawPlan`, `query.NewRawPlan`, `NewRawPlanWithSettings`, DB Query/Exec and Context variants, `QueryRowE` | Existing raw inspection/gate. Strict unverified Raw rejection unchanged; `TouchedTables` and reasons do not prove safety. PR3 boundary completion |
| `DB.QueryRow`, `QueryRowContext` | Legacy canceled harmless-query error transport may call executor on refusal. Not covered by PR1's Query-terminal zero-dispatch assertion; PR3 |
| `SQLDB`, underlying sql.DB/sql.Tx, driver and direct Executor | Outside ORM interception; no guarantee added |
| `orm/operation.Compile`, review, manifest, MCP | OperationSpec builds a Query SELECT plan using its rejecting executor and merges diagnostics; remains nonexecuting and supplies no application trust. No new write surface |

No `extractPlanSQL`-to-Raw adapter is introduced as the standard execution path.
The existing generic/scoped paths above are recorded rather than silently
included in PR1's coverage. DB settings, executor wrappers and transaction
constructors are unchanged: immutable Settings continue to propagate through
DB derivations and parent transactions; standalone wrappers need explicit
settings. A Query is not safe for concurrent mutation.

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
under that gate; tests do not prove safety of unparsed cases. PR1 does not make
Strict universal across generic CRUD, compound operations or raw escape hatches,
and does not complete Issue #66's overall acceptance criteria.
